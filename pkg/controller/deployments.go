package controller

import (
	"fmt"
	"os"
	"reflect"
	"unsafe"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/utils/ptr"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
	corevalidation "k8s.io/kubernetes/pkg/apis/core/validation"
)

// createOrApplyDeployments ensures required Deployment resources exist and are correctly configured.
func (r *ExternalSecretsReconciler) createOrApplyDeployments(externalsecrets *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string, externalsecretsCreateRecon bool) error {
	// Define all Deployment assets to apply based on conditions.
	deployments := []struct {
		assetName string
		condition bool
	}{
		{
			assetName: controllerDeploymentAssetName,
			condition: true,
		},
		{
			assetName: webhookDeploymentAssetName,
			condition: true,
		},
		{
			assetName: certControllerDeploymentAssetName,
			condition: !isCertManagerConfigEnabled(externalsecrets),
		},
		{
			assetName: bitwardenDeploymentAssetName,
			condition: isBitwardenConfigEnabled(externalsecrets),
		},
	}

	// Apply deployments based on the specified conditions.
	for _, d := range deployments {
		if !d.condition {
			continue
		}
		if err := r.createOrApplyDeploymentFromAsset(externalsecrets, d.assetName, resourceLabels, externalsecretsCreateRecon); err != nil {
			return err
		}
	}

	if err := r.updateImageInStatus(externalsecrets); err != nil {
		return FromClientError(err, "failed to update %s/%s status with image info", externalsecrets.GetNamespace(), externalsecrets.GetName())
	}

	return nil
}

func (r *ExternalSecretsReconciler) createOrApplyDeploymentFromAsset(externalsecrets *operatorv1alpha1.ExternalSecrets, assetName string, resourceLabels map[string]string,
	externalsecretsCreateRecon bool,
) error {

	deployment, err := r.getDeploymentObject(assetName, externalsecrets, resourceLabels)
	if err != nil {
		return err
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

	return nil
}

func (r *ExternalSecretsReconciler) getDeploymentObject(assetName string, externalsecrets *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string) (*appsv1.Deployment, error) {
	deployment := decodeDeploymentObjBytes(assets.MustAsset(assetName))
	updateNamespace(deployment, externalsecrets)
	updateResourceLabels(deployment, resourceLabels)
	updatePodTemplateLabels(deployment, resourceLabels)

	image := os.Getenv(externalsecretsImageEnvVarName)
	if image == "" {
		return nil, NewIrrecoverableError(fmt.Errorf("%s environment variable with externalsecrets image not set", externalsecretsImageEnvVarName), "failed to update image in %s deployment object", deployment.GetName())
	}
	logLevel := getLogLevel(externalsecrets.Spec.ExternalSecretsConfig)

	switch assetName {
	case controllerDeploymentAssetName:
		updateContainerSpec(deployment, externalsecrets, image, logLevel)
	case webhookDeploymentAssetName:
		updateWebhookContainerSpec(deployment, image, logLevel)
	case certControllerDeploymentAssetName:
		updateCertControllerContainerSpec(deployment, image, logLevel)
	}

	if err := r.updateResourceRequirement(deployment, externalsecrets); err != nil {
		return nil, fmt.Errorf("failed to update resource requirements: %w", err)
	}
	if err := r.updateAffinityRules(deployment, externalsecrets); err != nil {
		return nil, fmt.Errorf("failed to update affinity rules: %w", err)
	}
	if err := r.updatePodTolerations(deployment, externalsecrets); err != nil {
		return nil, fmt.Errorf("failed to update pod tolerations: %w", err)
	}
	if err := r.updateNodeSelector(deployment, externalsecrets); err != nil {
		return nil, fmt.Errorf("failed to update node selector: %w", err)
	}

	return deployment, nil
}

// updatePodTemplateLabels sets labels on the pod template spec.
func updatePodTemplateLabels(deployment *appsv1.Deployment, labels map[string]string) {
	l := deployment.Spec.Template.ObjectMeta.GetLabels()
	for k, v := range labels {
		l[k] = v
	}
	deployment.Spec.Template.ObjectMeta.SetLabels(l)
}

func updateContainerSecurityContext(container *corev1.Container) {
	container.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
		},
		ReadOnlyRootFilesystem: ptr.To(true),
		RunAsNonRoot:           ptr.To(true),
		RunAsUser:              nil,
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// updateResourceRequirement sets validated resource requirements to all containers.
func (r *ExternalSecretsReconciler) updateResourceRequirement(deployment *appsv1.Deployment, externalsecrets *operatorv1alpha1.ExternalSecrets) error {
	rscReqs := corev1.ResourceRequirements{}
	if externalsecrets.Spec.ExternalSecretsConfig != nil && !reflect.ValueOf(externalsecrets.Spec.ExternalSecretsConfig.Resources).IsZero() {
		externalsecrets.Spec.ExternalSecretsConfig.Resources.DeepCopyInto(&rscReqs)
	} else if r.esm.Spec.GlobalConfig != nil && !reflect.ValueOf(r.esm.Spec.GlobalConfig.Resources).IsZero() {
		r.esm.Spec.GlobalConfig.Resources.DeepCopyInto(&rscReqs)
	} else {
		return nil
	}

	// Validate the resource requirements
	if err := validateResourceRequirements(rscReqs, field.NewPath("spec")); err != nil {
		return fmt.Errorf("invalid resource requirements: %w", err)
	}

	// Apply the resource requirements to all containers in the pod template
	for i := range deployment.Spec.Template.Spec.Containers {
		deployment.Spec.Template.Spec.Containers[i].Resources = rscReqs
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
func (r *ExternalSecretsReconciler) updateNodeSelector(deployment *appsv1.Deployment, externalsecrets *operatorv1alpha1.ExternalSecrets) error {
	var nodeSelector map[string]string

	if externalsecrets.Spec.ExternalSecretsConfig != nil && externalsecrets.Spec.ExternalSecretsConfig.NodeSelector != nil {
		nodeSelector = externalsecrets.Spec.ExternalSecretsConfig.NodeSelector
	} else if r.esm.Spec.GlobalConfig != nil && r.esm.Spec.GlobalConfig.NodeSelector != nil {
		nodeSelector = r.esm.Spec.GlobalConfig.NodeSelector
	}

	if len(nodeSelector) == 0 {
		return nil
	}

	if err := validateNodeSelectorConfig(nodeSelector, field.NewPath("spec", "externalSecretsConfig")); err != nil {
		return err
	}

	deployment.Spec.Template.Spec.NodeSelector = nodeSelector
	return nil
}

// updateAffinityRules sets and validates pod affinity/anti-affinity rules.
func (r *ExternalSecretsReconciler) updateAffinityRules(deployment *appsv1.Deployment, externalsecrets *operatorv1alpha1.ExternalSecrets) error {
	var affinity *corev1.Affinity

	if externalsecrets.Spec.ExternalSecretsConfig != nil && externalsecrets.Spec.ExternalSecretsConfig.Affinity != nil {
		affinity = externalsecrets.Spec.ExternalSecretsConfig.Affinity
	} else if r.esm.Spec.GlobalConfig != nil && r.esm.Spec.GlobalConfig.Affinity != nil {
		affinity = r.esm.Spec.GlobalConfig.Affinity
	}

	if affinity == nil {
		return nil
	}

	if err := validateAffinityRules(affinity, field.NewPath("spec", "externalSecretsConfig", "affinity")); err != nil {
		return err
	}

	deployment.Spec.Template.Spec.Affinity = affinity
	return nil
}

// updatePodTolerations sets and validates pod tolerations.
func (r *ExternalSecretsReconciler) updatePodTolerations(deployment *appsv1.Deployment, externalsecrets *operatorv1alpha1.ExternalSecrets) error {
	var tolerations []corev1.Toleration

	if externalsecrets.Spec.ExternalSecretsConfig != nil && externalsecrets.Spec.ExternalSecretsConfig.Tolerations != nil {
		tolerations = externalsecrets.Spec.ExternalSecretsConfig.Tolerations
	} else if r.esm.Spec.GlobalConfig != nil && r.esm.Spec.GlobalConfig.Tolerations != nil {
		tolerations = r.esm.Spec.GlobalConfig.Tolerations
	}

	if len(tolerations) == 0 {
		return nil
	}

	if err := validateTolerationsConfig(tolerations, field.NewPath("spec", "externalSecretsConfig", "tolerations")); err != nil {
		return err
	}

	deployment.Spec.Template.Spec.Tolerations = tolerations
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

func (r *ExternalSecretsReconciler) updateImageInStatus(externalsecrets *operatorv1alpha1.ExternalSecrets) error {
	image := os.Getenv(externalsecretsImageEnvVarName)
	if externalsecrets.Status.ExternalSecretsImage != image {
		externalsecrets.Status.ExternalSecretsImage = image
		return r.updateStatus(r.ctx, externalsecrets)
	}
	return nil
}

// argument list for external-secrets deployment resource
func updateContainerSpec(deployment *appsv1.Deployment, externalsecrets *operatorv1alpha1.ExternalSecrets, image, logLevel string) {
	namespace := getOperatingNamespace(externalsecrets)
	args := []string{
		"--concurrent=1",
		"--metrics-addr=:8080",
		fmt.Sprintf("--loglevel=%s", logLevel),
		"--zap-time-encoding=epoch",
		"--enable-leader-election=true",
		"--enable-cluster-store-reconciler=true",
		"--enable-cluster-external-secret-reconciler=true",
		"--enable-push-secret-reconciler=true",
	}

	if namespace != "" {
		args = append(args, fmt.Sprintf("--namespace=%s", namespace))
	}

	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "external-secrets" {
			deployment.Spec.Template.Spec.Containers[i].Args = args
			deployment.Spec.Template.Spec.Containers[i].Image = image
			updateContainerSecurityContext(&deployment.Spec.Template.Spec.Containers[i])
			break
		}
	}
}

// argument list for webhook deployment resource
func updateWebhookContainerSpec(deployment *appsv1.Deployment, image, logLevel string) {
	args := []string{
		"webhook",
		fmt.Sprintf("--dns-name=external-secrets-webhook.%s.svc", deployment.GetNamespace()),
		"--port=10250",
		"--cert-dir=/tmp/certs",
		"--check-interval=5m",
		"--metrics-addr=:8080",
		"--healthz-addr=:8081",
		fmt.Sprintf("--loglevel=%s", logLevel),
		"--zap-time-encoding=epoch",
	}

	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "webhook" {
			deployment.Spec.Template.Spec.Containers[i].Args = args
			deployment.Spec.Template.Spec.Containers[i].Image = image
			updateContainerSecurityContext(&deployment.Spec.Template.Spec.Containers[i])
			break
		}
	}
}

// argument list for cert controller deployment resource
func updateCertControllerContainerSpec(deployment *appsv1.Deployment, image, logLevel string) {
	namespace := deployment.GetNamespace()
	args := []string{
		"certcontroller",
		"--crd-requeue-interval=5m",
		"--service-name=external-secrets-webhook",
		fmt.Sprintf("--service-namespace=%s", namespace),
		"--secret-name=external-secrets-webhook",
		fmt.Sprintf("--secret-namespace=%s", namespace),
		"--metrics-addr=:8080",
		"--healthz-addr=:8081",
		fmt.Sprintf("--loglevel=%s", logLevel),
		"--zap-time-encoding=epoch",
		"--enable-partial-cache=true",
	}

	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "cert-controller" {
			deployment.Spec.Template.Spec.Containers[i].Args = args
			deployment.Spec.Template.Spec.Containers[i].Image = image
			updateContainerSecurityContext(&deployment.Spec.Template.Spec.Containers[i])
			break
		}
	}
}
