package controller

import (
	"fmt"
	"os"
	"reflect"
	"unsafe"

	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
	corevalidation "k8s.io/kubernetes/pkg/apis/core/validation"
)

// createOrApplyDeployments ensures required Deployment resources exist and are correctly configured.
func (r *ExternalSecretsReconciler) createOrApplyDeployments(externalsecrets *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string, externalsecretsCreateRecon bool) error {
	// Define all Deployment assets to apply based on conditions.
	deployments := []struct {
		assetName  string
		argUpdater func(*appsv1.Deployment, *operatorv1alpha1.ExternalSecrets)
		condition  bool
	}{
		{
			assetName:  controllerDeploymentAssetName,
			argUpdater: updateArgList,
			condition:  true,
		},
		{
			assetName:  webhookDeploymentAssetName,
			argUpdater: updateWebhookArgs,
			condition:  true,
		},
		{
			assetName:  certControllerDeploymentAssetName,
			argUpdater: updateCertControllerArgs,
			condition:  externalsecrets.Spec.ExternalSecretsConfig.WebhookConfig.CertManagerConfig.Enabled == "false",
		},
		{
			assetName:  bitwardenDeploymentAssetName,
			argUpdater: nil,
			condition:  externalsecrets.Spec.ExternalSecretsConfig.BitwardenSecretManagerProvider.Enabled == "true",
		},
	}

	// Apply deployments based on the specified conditions.
	for _, d := range deployments {
		if !d.condition {
			continue
		}
		if err := r.createOrApplyDeploymentFromAsset(externalsecrets, d.assetName, resourceLabels, d.argUpdater, externalsecretsCreateRecon); err != nil {
			return err
		}
	}

	return nil
}

func (r *ExternalSecretsReconciler) createOrApplyDeploymentFromAsset(externalsecrets *operatorv1alpha1.ExternalSecrets, assetName string, resourceLabels map[string]string,
	argUpdater func(*appsv1.Deployment, *operatorv1alpha1.ExternalSecrets),
	externalsecretsCreateRecon bool,
) error {
	deployment := decodeDeploymentObjBytes(assets.MustAsset(assetName))

	updateNamespace(deployment, externalsecrets.GetNamespace())
	updateResourceLabels(deployment, resourceLabels)
	updatePodTemplateLabels(deployment, resourceLabels)

	if argUpdater != nil {
		argUpdater(deployment, externalsecrets)
	}

	if err := updateResourceRequirement(deployment, externalsecrets); err != nil {
		return fmt.Errorf("failed to update resource requirements: %w", err)
	}
	if err := updateAffinityRules(deployment, externalsecrets); err != nil {
		return fmt.Errorf("failed to update affinity rules: %w", err)
	}
	if err := updatePodTolerations(deployment, externalsecrets); err != nil {
		return fmt.Errorf("failed to update pod tolerations: %w", err)
	}
	if err := updateNodeSelector(deployment, externalsecrets); err != nil {
		return fmt.Errorf("failed to update node selector: %w", err)
	}
	if err := r.updateImage(deployment); err != nil {
		return NewIrrecoverableError(err, "failed to update image %s/%s", externalsecrets.GetNamespace(), externalsecrets.GetName())
	}

	deploymentName := fmt.Sprintf("%s/%s", deployment.GetNamespace(), deployment.GetName())
	key := types.NamespacedName{
		Name:      deployment.GetName(),
		Namespace: deployment.GetNamespace(),
	}
	fetched := &appsv1.Deployment{}
	exist, err := r.Exists(r.ctx, key, fetched)
	if err != nil {
		return FromClientError(err, "failed to check %s deployment resource already exists", deploymentName)
	}
	if exist && externalsecretsCreateRecon {
		r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s deployment resource already exists", deploymentName)
	}
	if exist && hasObjectChanged(deployment, fetched) {
		r.log.V(1).Info("deployment has been modified, updating to desired state", "name", deploymentName)
		if err := r.UpdateWithRetry(r.ctx, deployment); err != nil {
			return FromClientError(err, "failed to update %s deployment resource", deploymentName)
		}
		r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeNormal, "Reconciled", "deployment resource %s updated", deploymentName)
	} else if !exist {
		if err := r.Create(r.ctx, deployment); err != nil {
			return FromClientError(err, "failed to create %s deployment resource", deploymentName)
		}
		r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeNormal, "Reconciled", "deployment resource %s created", deploymentName)
	} else {
		r.log.V(4).Info("deployment resource already exists and is in expected state", "name", deploymentName)
	}

	if err := r.updateImageInStatus(externalsecrets, deployment); err != nil {
		return FromClientError(err, "failed to update %s/%s status with image info", externalsecrets.GetNamespace(), externalsecrets.GetName())
	}
	return nil
}

// updatePodTemplateLabels sets labels on the pod template spec.
func updatePodTemplateLabels(deployment *appsv1.Deployment, resourceLabels map[string]string) {
	deployment.Spec.Template.ObjectMeta.Labels = resourceLabels
}

// updateResourceRequirement sets validated resource requirements to all containers.
func updateResourceRequirement(deployment *appsv1.Deployment, externalsecrets *operatorv1alpha1.ExternalSecrets) error {
	if reflect.ValueOf(externalsecrets.Spec.ExternalSecretsConfig.Resources).IsZero() {
		return nil
	}
	if err := validateResourceRequirements(externalsecrets.Spec.ExternalSecretsConfig.Resources,
		field.NewPath("spec", "externalsecretsConfig")); err != nil {
		return err
	}
	for i := range deployment.Spec.Template.Spec.Containers {
		deployment.Spec.Template.Spec.Containers[i].Resources = externalsecrets.Spec.ExternalSecretsConfig.Resources
	}
	return nil
}

// validateResourceRequirements validates the resource request/limit configuration.
func validateResourceRequirements(requirements corev1.ResourceRequirements, fldPath *field.Path) error {
	// convert corev1.ResourceRequirements to core.ResourceRequirements, required for validation.
	convRequirements := *(*core.ResourceRequirements)(unsafe.Pointer(&requirements))
	return corevalidation.ValidateContainerResourceRequirements(&convRequirements, nil, fldPath.Child("resources"), corevalidation.PodValidationOptions{}).ToAggregate()
}

// updateNodeSelector sets and validates node selector constraints.
func updateNodeSelector(deployment *appsv1.Deployment, externalsecrets *operatorv1alpha1.ExternalSecrets) error {
	if externalsecrets.Spec.ExternalSecretsConfig.NodeSelector == nil {
		return nil
	}
	if err := validateNodeSelectorConfig(externalsecrets.Spec.ExternalSecretsConfig.NodeSelector,
		field.NewPath("spec", "externalsecretsConfig")); err != nil {
		return err
	}
	deployment.Spec.Template.Spec.NodeSelector = externalsecrets.Spec.ExternalSecretsConfig.NodeSelector
	return nil
}

// updateAffinityRules sets and validates pod affinity/anti-affinity rules.
func updateAffinityRules(deployment *appsv1.Deployment, externalsecrets *operatorv1alpha1.ExternalSecrets) error {
	if externalsecrets.Spec.ExternalSecretsConfig.Affinity == nil {
		return nil
	}
	if err := validateAffinityRules(externalsecrets.Spec.ExternalSecretsConfig.Affinity,
		field.NewPath("spec", "istioCSRConfig")); err != nil {
		return err
	}
	deployment.Spec.Template.Spec.Affinity = externalsecrets.Spec.ExternalSecretsConfig.Affinity
	return nil
}

// updatePodTolerations sets and validates pod tolerations.
func updatePodTolerations(deployment *appsv1.Deployment, externalsecrets *operatorv1alpha1.ExternalSecrets) error {
	if externalsecrets.Spec.ExternalSecretsConfig.Tolerations == nil {
		return nil
	}
	if err := validateTolerationsConfig(externalsecrets.Spec.ExternalSecretsConfig.Tolerations,
		field.NewPath("spec", "istioCSRConfig")); err != nil {
		return err
	}
	deployment.Spec.Template.Spec.Tolerations = externalsecrets.Spec.ExternalSecretsConfig.Tolerations
	return nil
}

// validateNodeSelectorConfig validates the NodeSelector configuration.
func validateNodeSelectorConfig(nodeSelector map[string]string, fldPath *field.Path) error {
	return metav1validation.ValidateLabels(nodeSelector, fldPath.Child("nodeSelector")).ToAggregate()
}

// validateAffinityRules validates the Affinity configuration.
func validateAffinityRules(affinity *corev1.Affinity, fldPath *field.Path) error {
	// convert corev1.Affinity to core.Affinity, required for validation.
	convAffinity := (*core.Affinity)(unsafe.Pointer(affinity))
	return validateAffinity(convAffinity, corevalidation.PodValidationOptions{}, fldPath.Child("affinity")).ToAggregate()
}

// validateTolerationsConfig validates the toleration configuration.
func validateTolerationsConfig(tolerations []corev1.Toleration, fldPath *field.Path) error {
	// convert corev1.Tolerations to core.Tolerations, required for validation.
	convTolerations := *(*[]core.Toleration)(unsafe.Pointer(&tolerations))
	return corevalidation.ValidateTolerations(convTolerations, fldPath.Child("tolerations")).ToAggregate()
}

func (r *ExternalSecretsReconciler) updateImage(deployment *appsv1.Deployment) error {
	image := os.Getenv(externalsecretsImageEnvVarName)
	if image == "" {
		return fmt.Errorf("%s environment variable with externalsecrets image not set", externalsecretsImageEnvVarName)
	}
	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == externalsecretsCommonName {
			deployment.Spec.Template.Spec.Containers[i].Image = image
		}
	}
	return nil
}

func (r *ExternalSecretsReconciler) updateImageInStatus(externalsecrets *operatorv1alpha1.ExternalSecrets, deployment *appsv1.Deployment) error {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == externalsecretsCommonName {
			if externalsecrets.Status.ExternalSecretsImage == container.Image {
				return nil
			}
			externalsecrets.Status.ExternalSecretsImage = container.Image
		}
	}
	return r.updateStatus(r.ctx, externalsecrets)
}

// argument list for external-secrets deployment resource
func updateArgList(deployment *appsv1.Deployment, externalsecrets *operatorv1alpha1.ExternalSecrets) {
	externalsecretsConfigs := externalsecrets.Spec.ExternalSecretsConfig
	logLevelInt := externalsecretsConfigs.LogLevel
	level := zapcore.Level(logLevelInt)
	levelStr := level.String()

	args := []string{
		"--concurrent=1",
		"--metrics-port=9402",
		fmt.Sprintf("--loglevel=%s", levelStr),
		"--zap-time-encoding=epoch",
		"--enable-leader-election=false", "--enable-cluster-store-reconciler=false", "--enable-cluster-external-secret-reconciler=false",
		"--enable-push-secret-reconciler=false",
	}

	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == externalsecretsCommonName {
			deployment.Spec.Template.Spec.Containers[i].Args = args
			break
		}
	}
}

// argument list for webhook deployment resource
func updateWebhookArgs(deployment *appsv1.Deployment, externalsecrets *operatorv1alpha1.ExternalSecrets) {
	externalsecretsConfigs := externalsecrets.Spec.ExternalSecretsConfig
	logLevelInt := externalsecretsConfigs.LogLevel
	level := zapcore.Level(logLevelInt)
	levelStr := level.String()

	args := []string{
		"webhook",
		fmt.Sprintf("--dns-name=external-secrets-webhook.%s.svc", externalsecrets.GetNamespace()),
		"--port=10250",
		"--cert-dir=/tmp/certs",
		"--check-interval=5m",
		"--metrics-addr=:8080",
		"--healthz-addr=:8081",
		fmt.Sprintf("--loglevel=%s", levelStr),
		"--zap-time-encoding=epoch",
	}

	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == externalsecretsCommonName {
			deployment.Spec.Template.Spec.Containers[i].Args = args
			break
		}
	}
}

// argument list for cert controller deployment resource
func updateCertControllerArgs(deployment *appsv1.Deployment, externalsecrets *operatorv1alpha1.ExternalSecrets) {
	externalsecretsConfigs := externalsecrets.Spec.ExternalSecretsConfig
	nameSpace := externalsecrets.GetNamespace()
	logLevelInt := externalsecretsConfigs.LogLevel
	level := zapcore.Level(logLevelInt)
	levelStr := level.String()

	args := []string{
		"certcontroller",
		"--crd-requeue-interval=5m",
		"--service-name=external-secrets-webhook",
		fmt.Sprintf("--service-namespace=%s", nameSpace),
		"--secret-name=external-secrets-webhook",
		fmt.Sprintf("--secret-namespace=%s", nameSpace),
		"--metrics-addr=:8080",
		"--healthz-addr=:8081",
		fmt.Sprintf("--loglevel=%s", levelStr),
		"--zap-time-encoding=epoch",
		"--enable-partial-cache=true",
	}

	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == externalsecretsCommonName {
			deployment.Spec.Template.Spec.Containers[i].Args = args
			break
		}
	}
}
