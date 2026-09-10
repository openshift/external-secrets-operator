package external_secrets

import (
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
	"unsafe"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core"
	corevalidation "k8s.io/kubernetes/pkg/apis/core/validation"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

// operandArgsEnvSeparator splits OPERAND_*_ARGS on commas that introduce the next
// --flag, so values may themselves contain commas (e.g. --tls-ciphers=A,B).
var operandArgsEnvSeparator = regexp.MustCompile(`,+\s*--`)

// createOrApplyDeployments ensures required Deployment resources exist and are correctly configured.
func (r *Reconciler) createOrApplyDeployments(esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata common.ResourceMetadata, externalSecretsConfigCreateRecon bool) error {
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
			condition: !isCertManagerConfigEnabled(esc),
		},
		{
			assetName: bitwardenDeploymentAssetName,
			condition: isBitwardenConfigEnabled(esc),
		},
	}

	// Apply deployments based on the specified conditions.
	for _, d := range deployments {
		if !d.condition {
			continue
		}
		if err := r.createOrApplyDeploymentFromAsset(esc, d.assetName, resourceMetadata, externalSecretsConfigCreateRecon); err != nil {
			return err
		}
	}

	if err := r.updateImageInStatus(esc); err != nil {
		return common.FromClientError(err, "failed to update %s/%s status with image info", esc.GetNamespace(), esc.GetName())
	}

	return nil
}

func (r *Reconciler) createOrApplyDeploymentFromAsset(esc *operatorv1alpha1.ExternalSecretsConfig, assetName string, resourceMetadata common.ResourceMetadata,
	externalSecretsConfigCreateRecon bool,
) error {
	deployment, trustedCAErr := r.getDeploymentObject(assetName, esc, resourceMetadata)
	// trustedCABundle may fail in getDeploymentObject (e.g. missing ConfigMap) while still
	// returning a deployment with the stale user-ca-bundle mount removed. Apply that spec
	// first, then return the error so status becomes Degraded and the reconcile requeues.
	if deployment == nil {
		return trustedCAErr
	}

	deploymentName := fmt.Sprintf("%s/%s", deployment.GetNamespace(), deployment.GetName())
	fetched := &appsv1.Deployment{}
	exist, err := r.Exists(r.ctx, client.ObjectKeyFromObject(deployment), fetched)
	if err != nil {
		return common.FromClientError(err, "failed to check %s deployment resource already exists", deploymentName)
	}

	if !exist {
		if err := r.createWithFallback(deployment, resourceMetadata, deploymentName, esc); err != nil {
			return err
		}
		return trustedCAErr
	}

	if externalSecretsConfigCreateRecon {
		r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s deployment resource already exists", deploymentName)
	}

	if !common.HasObjectChanged(deployment, fetched, &resourceMetadata) {
		r.log.V(4).Info("deployment resource already exists and is in expected state", "name", deploymentName)
		return trustedCAErr
	}

	r.log.V(1).Info("deployment has been modified, updating to desired state", "name", deploymentName)
	common.RemoveObsoleteAnnotations(deployment, resourceMetadata)
	if err := r.UpdateWithRetry(r.ctx, deployment); err != nil {
		return common.FromClientError(err, "failed to update %s deployment resource", deploymentName)
	}
	r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "deployment resource %s updated", deploymentName)

	return trustedCAErr
}

func (r *Reconciler) getDeploymentObject(assetName string, esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata common.ResourceMetadata) (*appsv1.Deployment, error) {
	deployment := common.DecodeDeploymentObjBytes(assets.MustAsset(assetName))
	updateNamespace(deployment, esc)
	common.ApplyResourceMetadata(deployment, resourceMetadata)
	updatePodTemplateLabels(deployment, resourceMetadata.Labels)
	updatePodTemplateAnnotations(deployment, resourceMetadata.Annotations)

	image := os.Getenv(externalsecretsImageEnvVarName)
	if image == "" {
		return nil, common.NewIrrecoverableError(fmt.Errorf("%s environment variable with externalsecrets image not set", externalsecretsImageEnvVarName), "failed to update image in %s deployment object", deployment.GetName())
	}
	bitwardenImage := os.Getenv(bitwardenImageEnvVarName)
	if bitwardenImage == "" {
		return nil, common.NewIrrecoverableError(fmt.Errorf("%s environment variable with bitwarden-sdk-server image not set", bitwardenImageEnvVarName), "failed to update image in %s deployment object", deployment.GetName())
	}
	logLevel := getLogLevel(esc, r.esm)

	switch assetName {
	case controllerDeploymentAssetName:
		r.updateContainerSpec(deployment, esc, image, logLevel)
		if err := applyOperandArgsFromEnv(deployment, OperandCoreControllerContainer, OperandExternalSecretsArgsEnvVar); err != nil {
			return nil, fmt.Errorf("failed to apply operand args from env: %w", err)
		}
		if err := r.applyUserCABundleConfig(deployment, esc); err != nil {
			wrapped := fmt.Errorf("failed to apply user CA bundle config: %w", err)
			// When the referenced ConfigMap is missing, the deployment spec is updated to remove
			// the user CA volume so the operand does not retain a stale mount reference.
			if common.IsUserConfigurationNotFound(err) {
				return deployment, wrapped
			}
			return nil, wrapped
		}
	case webhookDeploymentAssetName:
		checkInterval := normalizeDurationArg("5m")
		if esc.Spec.ApplicationConfig.WebhookConfig != nil &&
			esc.Spec.ApplicationConfig.WebhookConfig.CertificateCheckInterval != nil {
			checkInterval = normalizeDurationArg(esc.Spec.ApplicationConfig.WebhookConfig.CertificateCheckInterval.Duration.String())
		}
		updateWebhookContainerSpec(deployment, image, logLevel, checkInterval)
		if err := applyOperandArgsFromEnv(deployment, OperandWebhookContainer, OperandWebhookArgsEnvVar); err != nil {
			return nil, fmt.Errorf("failed to apply operand args from env: %w", err)
		}
		updateWebhookVolumeConfig(deployment, esc)
	case certControllerDeploymentAssetName:
		updateCertControllerContainerSpec(deployment, image, logLevel)
		if err := applyOperandArgsFromEnv(deployment, OperandCertControllerContainer, OperandCertControllerArgsEnvVar); err != nil {
			return nil, fmt.Errorf("failed to apply operand args from env: %w", err)
		}
	case bitwardenDeploymentAssetName:
		deployment.Labels["app.kubernetes.io/version"] = os.Getenv(bitwardenImageVersionEnvVarName)
		updateBitwardenServerContainerSpec(deployment, bitwardenImage)
		if err := applyOperandArgsFromEnv(deployment, OperandBitwardenContainer, OperandBitwardenSDKServerArgsEnvVar); err != nil {
			return nil, fmt.Errorf("failed to apply operand args from env: %w", err)
		}
		updateBitwardenVolumeConfig(deployment, esc)
	}

	if err := r.updateResourceRequirement(deployment, esc); err != nil {
		return nil, fmt.Errorf("failed to update resource requirements: %w", err)
	}
	if err := r.updateAffinityRules(deployment, esc); err != nil {
		return nil, fmt.Errorf("failed to update affinity rules: %w", err)
	}
	if err := r.updatePodTolerations(deployment, esc); err != nil {
		return nil, fmt.Errorf("failed to update pod tolerations: %w", err)
	}
	if err := r.updateNodeSelector(deployment, esc); err != nil {
		return nil, fmt.Errorf("failed to update node selector: %w", err)
	}
	if err := r.applyUserDeploymentConfigs(deployment, esc, assetName); err != nil {
		return nil, fmt.Errorf("failed to apply user deployment configuration: %w", err)
	}
	r.updateProxyConfiguration(deployment)

	return deployment, nil
}

// normalizeDurationArg parses a duration string and returns its canonical Go form
// (e.g. "5m" → "5m0s") so container args match values persisted by the API server.
func normalizeDurationArg(raw string) string {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return raw
	}
	return d.String()
}

// updatePodTemplateLabels sets labels on the pod template spec.
func updatePodTemplateLabels(deployment *appsv1.Deployment, labels map[string]string) {
	if len(labels) == 0 {
		return
	}

	l := deployment.Spec.Template.GetLabels()
	if len(l) == 0 {
		l = make(map[string]string)
	}

	maps.Copy(l, labels)
	deployment.Spec.Template.SetLabels(l)
}

func updatePodTemplateAnnotations(deployment *appsv1.Deployment, annotations map[string]string) {
	if len(annotations) == 0 {
		return
	}

	l := deployment.Spec.Template.GetAnnotations()
	if len(l) == 0 {
		l = make(map[string]string)
	}

	maps.Copy(l, annotations)
	deployment.Spec.Template.SetAnnotations(l)
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
func (r *Reconciler) updateResourceRequirement(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig) error {
	rscReqs := corev1.ResourceRequirements{}
	switch {
	case esc.Spec.ApplicationConfig.Resources != nil:
		esc.Spec.ApplicationConfig.Resources.DeepCopyInto(&rscReqs)
	case r.esm.Spec.GlobalConfig != nil && r.esm.Spec.GlobalConfig.Resources != nil:
		r.esm.Spec.GlobalConfig.Resources.DeepCopyInto(&rscReqs)
	default:
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
func (r *Reconciler) updateNodeSelector(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig) error {
	var nodeSelector map[string]string

	if esc.Spec.ApplicationConfig.NodeSelector != nil {
		nodeSelector = esc.Spec.ApplicationConfig.NodeSelector
	} else if r.esm.Spec.GlobalConfig != nil && r.esm.Spec.GlobalConfig.NodeSelector != nil {
		nodeSelector = r.esm.Spec.GlobalConfig.NodeSelector
	}

	if len(nodeSelector) == 0 {
		return nil
	}

	if err := validateNodeSelectorConfig(nodeSelector, field.NewPath("spec")); err != nil {
		return err
	}

	deployment.Spec.Template.Spec.NodeSelector = nodeSelector
	return nil
}

// updateAffinityRules sets and validates pod affinity/anti-affinity rules.
func (r *Reconciler) updateAffinityRules(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig) error {
	var affinity *corev1.Affinity

	if esc.Spec.ApplicationConfig.Affinity != nil {
		affinity = esc.Spec.ApplicationConfig.Affinity
	} else if r.esm.Spec.GlobalConfig != nil && r.esm.Spec.GlobalConfig.Affinity != nil {
		affinity = r.esm.Spec.GlobalConfig.Affinity
	}

	if affinity == nil {
		return nil
	}

	if err := validateAffinityRules(affinity, field.NewPath("spec", "affinity")); err != nil {
		return err
	}

	deployment.Spec.Template.Spec.Affinity = affinity
	return nil
}

// updatePodTolerations sets and validates pod tolerations.
func (r *Reconciler) updatePodTolerations(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig) error {
	var tolerations []corev1.Toleration

	if esc.Spec.ApplicationConfig.Tolerations != nil {
		tolerations = esc.Spec.ApplicationConfig.Tolerations
	} else if r.esm.Spec.GlobalConfig != nil && r.esm.Spec.GlobalConfig.Tolerations != nil {
		tolerations = r.esm.Spec.GlobalConfig.Tolerations
	}

	if len(tolerations) == 0 {
		return nil
	}

	if err := validateTolerationsConfig(tolerations, field.NewPath("spec", "tolerations")); err != nil {
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
	return common.ValidateAffinity(convAffinity, corevalidation.PodValidationOptions{}, fldPath.Child("affinity")).ToAggregate()
}

// validateTolerationsConfig validates the toleration configuration.
func validateTolerationsConfig(tolerations []corev1.Toleration, fldPath *field.Path) error {
	// convert corev1.Tolerations to core.Tolerations, required for validation.
	convTolerations := *(*[]core.Toleration)(unsafe.Pointer(&tolerations))
	return corevalidation.ValidateTolerations(convTolerations, fldPath.Child("tolerations"), corevalidation.PodValidationOptions{}).ToAggregate()
}

func (r *Reconciler) updateImageInStatus(esc *operatorv1alpha1.ExternalSecretsConfig) error {
	externalSecretsImage := os.Getenv(externalsecretsImageEnvVarName)
	bitwardenImage := os.Getenv(bitwardenImageEnvVarName)
	if esc.Status.ExternalSecretsImage != externalSecretsImage || esc.Status.BitwardenSDKServerImage != bitwardenImage {
		esc.Status.ExternalSecretsImage = externalSecretsImage
		esc.Status.BitwardenSDKServerImage = bitwardenImage
		return r.updateStatus(r.ctx, esc)
	}
	return nil
}

// argument list for external-secrets deployment resource.
func (r *Reconciler) updateContainerSpec(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig, image, logLevel string) {
	var (
		enableClusterStoreArgFmt           = "--enable-cluster-store-reconciler=%s"
		enableClusterExternalSecretsArgFmt = "--enable-cluster-external-secret-reconciler=%s"
	)

	args := []string{
		"--concurrent=1",
		"--metrics-addr=:8080",
		fmt.Sprintf("--loglevel=%s", logLevel),
		"--zap-time-encoding=epoch",
		"--enable-leader-election=true",
		"--enable-push-secret-reconciler=true",
	}

	// when spec.appConfig.operatingNamespace is configured, which is for restricting the
	// external-secrets custom resource reconcile scope to specified namespace, the reconciliation
	// of cluster scoped custom resources must also be disabled.
	namespace := getOperatingNamespace(esc)
	if namespace != "" {
		args = append(args, fmt.Sprintf("--namespace=%s", namespace),
			fmt.Sprintf(enableClusterStoreArgFmt, "false"),
			fmt.Sprintf(enableClusterExternalSecretsArgFmt, "false"))
	} else {
		args = append(args, fmt.Sprintf(enableClusterStoreArgFmt, "true"),
			fmt.Sprintf(enableClusterExternalSecretsArgFmt, "true"))
	}

	r.updateOptionalFeatures(&args, []operatorv1alpha1.FeatureName{operatorv1alpha1.UnsafeAllowGenericTargets})

	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == OperandCoreControllerContainer {
			deployment.Spec.Template.Spec.Containers[i].Args = args
			deployment.Spec.Template.Spec.Containers[i].Image = image
			updateContainerSecurityContext(&deployment.Spec.Template.Spec.Containers[i])
			break
		}
	}
}

// argument list for webhook deployment resource.
func updateWebhookContainerSpec(deployment *appsv1.Deployment, image, logLevel, checkInterval string) {
	args := []string{
		"webhook",
		fmt.Sprintf("--dns-name=external-secrets-webhook.%s.svc", deployment.GetNamespace()),
		"--port=10250",
		"--cert-dir=/tmp/certs",
		fmt.Sprintf("--check-interval=%s", checkInterval),
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

// argument list for cert controller deployment resource.
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

// updateBitwardenServerContainerSpec is for updating the primary container spec in bitwarden-sdk-server
// deployment object.
func updateBitwardenServerContainerSpec(deployment *appsv1.Deployment, image string) {
	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "bitwarden-sdk-server" {
			deployment.Spec.Template.Spec.Containers[i].Image = image
			updateContainerSecurityContext(&deployment.Spec.Template.Spec.Containers[i])
			break
		}
	}
}

func updateBitwardenVolumeConfig(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig) {
	if esc.Spec.Plugins.BitwardenSecretManagerProvider.SecretRef != nil &&
		esc.Spec.Plugins.BitwardenSecretManagerProvider.SecretRef.Name != "" {
		secretName := esc.Spec.Plugins.BitwardenSecretManagerProvider.SecretRef.Name
		updateSecretVolumeConfig(deployment, "bitwarden-tls-certs", secretName)
	}
}

func updateWebhookVolumeConfig(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig) {
	if isCertManagerConfigEnabled(esc) {
		updateSecretVolumeConfig(deployment, "certs", certmanagerTLSSecretWebhook)
	}
}

func updateSecretVolumeConfig(deployment *appsv1.Deployment, volumeName, secretName string) {
	for i := range deployment.Spec.Template.Spec.Volumes {
		if deployment.Spec.Template.Spec.Volumes[i].Name == volumeName {
			if deployment.Spec.Template.Spec.Volumes[i].Secret == nil {
				deployment.Spec.Template.Spec.Volumes[i].Secret = &corev1.SecretVolumeSource{}
			}
			deployment.Spec.Template.Spec.Volumes[i].Secret.SecretName = secretName
			return
		}
	}

	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	})
}

// updateProxyConfiguration applies or removes all proxy-related deployment configuration
// (environment variables and trusted CA bundle volume/mounts) based on proxy configuration.
func (r *Reconciler) updateProxyConfiguration(deployment *appsv1.Deployment) {
	if !r.isProxyEnabled() {
		removeProxyEnvironmentVariables(deployment)
		removeTrustedCABundleVolumes(deployment)
		return
	}

	applyProxyEnvironmentVariables(deployment, r.proxyConfig)
	applyTrustedCABundleVolumes(deployment)
}

// applyProxyEnvironmentVariables sets proxy environment variables on all containers and init containers.
func applyProxyEnvironmentVariables(deployment *appsv1.Deployment, proxyConfig *operatorv1alpha1.ProxyConfig) {
	for i := range deployment.Spec.Template.Spec.Containers {
		setProxyEnvVars(&deployment.Spec.Template.Spec.Containers[i], proxyConfig)
	}
	for i := range deployment.Spec.Template.Spec.InitContainers {
		setProxyEnvVars(&deployment.Spec.Template.Spec.InitContainers[i], proxyConfig)
	}
}

// removeProxyEnvironmentVariables removes proxy environment variables from all containers and init containers.
func removeProxyEnvironmentVariables(deployment *appsv1.Deployment) {
	for i := range deployment.Spec.Template.Spec.Containers {
		removeProxyEnvVars(&deployment.Spec.Template.Spec.Containers[i])
	}
	for i := range deployment.Spec.Template.Spec.InitContainers {
		removeProxyEnvVars(&deployment.Spec.Template.Spec.InitContainers[i])
	}
}

// mergeContainerEnvVars updates, adds, or removes environment variables in the target container
// based on the provided map. Managed keys from envVars are applied in sorted name order so
// container.Env stays stable across reconciles. Keys with empty string values are omitted.
// Unmanaged environment variables from the container are appended after managed keys.
func mergeContainerEnvVars(container *corev1.Container, envVars map[string]string) {
	if envVars == nil {
		return
	}

	newEnv := make([]corev1.EnvVar, 0, len(container.Env)+len(envVars))

	names := make([]string, 0, len(envVars))
	for name := range envVars {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		if value := envVars[name]; value != "" {
			newEnv = append(newEnv, corev1.EnvVar{
				Name:  name,
				Value: value,
			})
		}
	}

	for _, env := range container.Env {
		if _, exists := envVars[env.Name]; !exists {
			newEnv = append(newEnv, env)
		}
	}

	container.Env = newEnv
}

// pruneContainerEnvVars filters out specified environment variables from the target container.
// It removes any variable whose name matches an entry in the target slice, effectively
// cleaning up configurations that are no longer required by the controller lifecycle.
// Variables not present in the target slice remain entirely unaffected.
func pruneContainerEnvVars(container *corev1.Container, envVars []string) {
	if len(container.Env) == 0 || len(envVars) == 0 {
		return
	}

	filteredEnv := make([]corev1.EnvVar, 0, len(container.Env))
	for _, env := range container.Env {
		if slices.Contains(envVars, env.Name) {
			continue
		}
		filteredEnv = append(filteredEnv, env)
	}

	container.Env = filteredEnv
}

// setProxyEnvVars sets proxy environment variables on a container.
func setProxyEnvVars(container *corev1.Container, proxyConfig *operatorv1alpha1.ProxyConfig) {
	if proxyConfig == nil {
		return
	}

	envVars := map[string]string{
		httpProxyEnvVar:           proxyConfig.HTTPProxy,
		httpsProxyEnvVar:          proxyConfig.HTTPSProxy,
		noProxyEnvVar:             proxyConfig.NoProxy,
		httpProxyEnvVarLowercase:  proxyConfig.HTTPProxy,
		httpsProxyEnvVarLowercase: proxyConfig.HTTPSProxy,
		noProxyEnvVarLowercase:    proxyConfig.NoProxy,
	}

	mergeContainerEnvVars(container, envVars)
}

// removeProxyEnvVars removes proxy environment variables from a container.
func removeProxyEnvVars(container *corev1.Container) {
	proxyEnvVars := []string{httpProxyEnvVar, httpsProxyEnvVar, noProxyEnvVar,
		httpProxyEnvVarLowercase, httpsProxyEnvVarLowercase, noProxyEnvVarLowercase}
	pruneContainerEnvVars(container, proxyEnvVars)
}

func updateVolumeMount(container *corev1.Container, volumeName, volumeMountPath string) {
	mount := corev1.VolumeMount{
		Name:      volumeName,
		MountPath: volumeMountPath,
		ReadOnly:  true,
	}

	for i, existing := range container.VolumeMounts {
		if existing.Name == volumeName {
			container.VolumeMounts[i] = mount
			return
		}
	}
	container.VolumeMounts = append(container.VolumeMounts, mount)
}

// upsertConfigMapVolume adds or updates a ConfigMap volume on the deployment pod spec.
func upsertConfigMapVolume(deployment *appsv1.Deployment, configMapName, configMapKeyName, volumeName, volumeKeyPath string) {
	volume := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
				DefaultMode: ptr.To(int32(420)),
			},
		},
	}

	if configMapKeyName != "" && volumeKeyPath != "" {
		volume.ConfigMap.Items = append(volume.VolumeSource.ConfigMap.Items, corev1.KeyToPath{
			Key:  configMapKeyName,
			Path: volumeKeyPath,
		})
	}

	for i, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == volumeName {
			deployment.Spec.Template.Spec.Volumes[i] = volume
			return
		}
	}
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, volume)
}

func pruneVolumeMount(container *corev1.Container, volumeName string) {
	var filteredVolumeMounts []corev1.VolumeMount
	for _, volumeMount := range container.VolumeMounts {
		if volumeMount.Name != volumeName {
			filteredVolumeMounts = append(filteredVolumeMounts, volumeMount)
		}
	}
	container.VolumeMounts = filteredVolumeMounts
}

// prunePodVolume removes a volume from the deployment pod spec.
func prunePodVolume(deployment *appsv1.Deployment, volumeName string) {
	var filteredVolumes []corev1.Volume
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Name != volumeName {
			filteredVolumes = append(filteredVolumes, volume)
		}
	}
	deployment.Spec.Template.Spec.Volumes = filteredVolumes
}

// applyTrustedCABundleVolumes adds the operator-managed trusted CA bundle volume and mounts it on all containers and init containers.
func applyTrustedCABundleVolumes(deployment *appsv1.Deployment) {
	upsertConfigMapVolume(deployment, ProxyTrustedCABundleConfigMapName, "", ProxyTrustedCABundleVolumeName, "")
	for i := range deployment.Spec.Template.Spec.Containers {
		updateVolumeMount(&deployment.Spec.Template.Spec.Containers[i], ProxyTrustedCABundleVolumeName, ProxyTrustedCABundleMountPath)
	}
	for i := range deployment.Spec.Template.Spec.InitContainers {
		updateVolumeMount(&deployment.Spec.Template.Spec.InitContainers[i], ProxyTrustedCABundleVolumeName, ProxyTrustedCABundleMountPath)
	}
}

// removeTrustedCABundleVolumes removes the operator-managed trusted CA bundle volume and mounts from all containers and init containers.
func removeTrustedCABundleVolumes(deployment *appsv1.Deployment) {
	prunePodVolume(deployment, ProxyTrustedCABundleVolumeName)
	for i := range deployment.Spec.Template.Spec.Containers {
		pruneVolumeMount(&deployment.Spec.Template.Spec.Containers[i], ProxyTrustedCABundleVolumeName)
	}
	for i := range deployment.Spec.Template.Spec.InitContainers {
		pruneVolumeMount(&deployment.Spec.Template.Spec.InitContainers[i], ProxyTrustedCABundleVolumeName)
	}
}

// applyUserCABundleConfig validates the user-specified trustedCABundle ConfigMap and, when valid,
// mounts it into the controller deployment and sets SSL_CERT_DIR on the core controller container.
// Skip and error conditions:
//   - trustedCABundle is nil → remove any existing user CA bundle config, no-op
//   - CM NotFound → remove user CA config from deployment, TrustedCABundleError (Degraded + requeue)
//   - CM key missing or invalid PEM → fail closed (keep existing mount), TrustedCABundleError
//     (Degraded; recovery via ConfigMap watch)
//   - CM has TrustedCABundleInjectLabel AND proxy is configured → skip user mount; CNO reconciles
//     this ConfigMap and the proxy trusted CA bundle is already mounted at /etc/pki/tls/certs
func (r *Reconciler) applyUserCABundleConfig(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig) error {
	ref := esc.Spec.ControllerConfig.TrustedCABundle
	if ref == nil {
		r.removeUserCABundleConfig(deployment)
		r.now.Reset()
		return nil
	}

	namespacedName := types.NamespacedName{Name: ref.Name, Namespace: getNamespace(esc)}
	cm := &corev1.ConfigMap{}
	if err := r.getWithCacheFallback(namespacedName, cm); err != nil {
		if apierrors.IsNotFound(err) {
			r.removeUserCABundleConfig(deployment)
			return common.NewUserConfigurationError(err, "trustedCABundle ConfigMap %q not found", namespacedName)
		}
		return common.FromClientError(err, "failed to fetch trustedCABundle ConfigMap %q", namespacedName)
	}

	// Label before validation so administrators can recover from invalid PEM via resource watch.
	if err := r.updateWatchLabel(namespacedName, &corev1.ConfigMap{}); err != nil {
		return common.FromClientError(err, "failed to patch watch label on trustedCABundle ConfigMap %q", namespacedName)
	}

	// Skip mounting when the CM is CNO-managed (inject-trusted-cabundle label) and proxy is
	// configured in cluster: the operator-managed ConfigMap already receives the same cluster CAs from CNO
	// and is mounted at /etc/pki/tls/certs by the existing proxy CA bundle mechanism.
	if cm.Labels[TrustedCABundleInjectLabel] == "true" && r.isProxyEnabled() {
		r.log.V(1).Info("trustedCABundle ConfigMap is CNO-managed and proxy is configured, skipping user CA bundle mount", "name", namespacedName)
		r.removeUserCABundleConfig(deployment)
		r.eventRecorder.Eventf(
			esc,
			corev1.EventTypeNormal,
			trustedCABundleEventSkippedCNOProxy,
			"trustedCABundle ConfigMap %q is CNO-managed and proxy is configured; user CA mount skipped because cluster trusted CA bundle is already mounted for proxy",
			namespacedName,
		)
		return nil
	}

	key := ref.Key
	if key == "" {
		key = UserCABundleKeyPath
	}
	data, ok := cm.Data[key]
	if !ok {
		return common.NewUserConfigurationError(
			fmt.Errorf("key %q not found", key),
			"trustedCABundle ConfigMap %q does not contain key %q",
			namespacedName, key,
		)
	}

	if err := r.validateTrustedCABundleData(esc, namespacedName, key, data); err != nil {
		return common.NewUserConfigurationError(err, "trustedCABundle ConfigMap %q key %q has invalid PEM", namespacedName, key)
	}

	// Add or replace the user CA bundle volume and mount it on the core controller container only.
	upsertConfigMapVolume(deployment, ref.Name, key, UserCABundleVolumeName, UserCABundleKeyPath)
	for i := range deployment.Spec.Template.Spec.Containers {
		container := &deployment.Spec.Template.Spec.Containers[i]
		if container.Name == OperandCoreControllerContainer {
			updateVolumeMount(container, UserCABundleVolumeName, UserCABundleMountPath)
			mergeContainerEnvVars(container, map[string]string{SSLCertDirEnvVar: SSLCertDirValue})
			break
		}
	}
	return nil
}

// removeUserCABundleConfig removes the user CA bundle volume, volume mount, and SSL_CERT_DIR
// env var from the controller deployment when trustedCABundle is no longer configured or valid.
func (r *Reconciler) removeUserCABundleConfig(deployment *appsv1.Deployment) {
	prunePodVolume(deployment, UserCABundleVolumeName)
	for i := range deployment.Spec.Template.Spec.Containers {
		container := &deployment.Spec.Template.Spec.Containers[i]
		if container.Name == OperandCoreControllerContainer {
			pruneVolumeMount(container, UserCABundleVolumeName)
			pruneContainerEnvVars(container, []string{SSLCertDirEnvVar})
			break
		}
	}
}

// argFlagKey returns the flag key for a container arg (everything before the first '='),
// or the whole token when there is no '='. Positional args without a leading "--" return "".
func argFlagKey(arg string) string {
	if !strings.HasPrefix(arg, "--") {
		return ""
	}
	if i := strings.IndexByte(arg, '='); i >= 0 {
		return arg[:i]
	}
	return arg
}

// parseOperandArgsEnv parses a list of full CLI flags separated by a comma that
// introduces the next --flag (e.g. "--concurrent=5,--loglevel=debug").
// Commas inside --key=value values are preserved (e.g. "--tls-ciphers=A,B,--loglevel=debug").
// Boolean-style flags (--key with no '=') cannot carry a value, so any ",..." after
// the flag name is dropped (e.g. "--enable-foo,junk,--bar=1" → "--enable-foo","--bar=1").
// Whitespace around separators, repeated commas between flags, and a trailing
// comma are tolerated. Callers should treat a fully empty/whitespace env value
// as a no-op before calling this helper.
func parseOperandArgsEnv(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}, nil
	}

	// Delimiter consumes the next flag's leading "--", so re-prefix on later parts.
	parts := operandArgsEnvSeparator.Split(raw, -1)
	args := make([]string, 0, len(parts))
	for i, part := range parts {
		if i > 0 {
			part = "--" + part
		}
		// Trim surrounding whitespace, strip trailing separator commas, then
		// trim again so a comma preceded by spaces (e.g. "--foo=1 ,") is removed.
		part = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(part), ","))
		if part == "" {
			continue
		}
		if !strings.HasPrefix(part, "--") || part == "--" {
			return nil, common.NewUserConfigurationError(
				fmt.Errorf("argument %q must start with --", part),
				"invalid custom arg override",
			)
		}
		// Boolean-style flags have no value; discard ",junk" after the flag name.
		if !strings.Contains(part, "=") {
			if j := strings.IndexByte(part, ','); j >= 0 {
				part = strings.TrimSpace(part[:j])
				if part == "--" {
					return nil, common.NewUserConfigurationError(
						fmt.Errorf("argument %q must start with --", part),
						"invalid custom arg override",
					)
				}
			}
		}
		// Reject empty flag names such as "--=value" (argFlagKey is "--").
		if argFlagKey(part) == "--" {
			return nil, common.NewUserConfigurationError(
				fmt.Errorf("argument %q must include a flag name after --", part),
				"invalid custom arg override",
			)
		}
		args = append(args, part)
	}
	return args, nil
}

// mergeContainerArgs overrides matching --flag keys in base with overrides and appends
// unknown keys. Positional args (no leading "--") in base are preserved in place;
// non-flag overrides are skipped (parseOperandArgsEnv already rejects them for the env path).
func mergeContainerArgs(base []string, overrides []string) []string {
	if len(overrides) == 0 {
		return base
	}

	keyIndex := make(map[string]int, len(base))
	for i, arg := range base {
		if key := argFlagKey(arg); key != "" {
			keyIndex[key] = i
		}
	}

	result := slices.Clone(base)
	for _, override := range overrides {
		key := argFlagKey(override)
		if key == "" {
			continue
		}
		if idx, ok := keyIndex[key]; ok {
			result[idx] = override
			continue
		}
		result = append(result, override)
		keyIndex[key] = len(result) - 1
	}
	return result
}

// applyOperandArgsFromEnv reads envVarName and merges its --key[=value] flags into
// the named container's Args. Flags are separated by a comma before the next --flag
// so values may contain commas. Unset or empty env is a no-op.
//
// TODO: Remove in v1.4.0. Backported to 1.1/1.2 as a temporary escape hatch;
// v1.3.0 adds ExternalSecretsConfig advancedOverrides for per-component Deployment
// overrides and is the migration window before this env-based path is removed.
func applyOperandArgsFromEnv(deployment *appsv1.Deployment, containerName, envVarName string) error {
	raw := strings.TrimSpace(os.Getenv(envVarName))
	if raw == "" {
		return nil
	}

	overrides, err := parseOperandArgsEnv(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", envVarName, err)
	}

	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name != containerName {
			continue
		}
		deployment.Spec.Template.Spec.Containers[i].Args = mergeContainerArgs(
			deployment.Spec.Template.Spec.Containers[i].Args, overrides)
		return nil
	}
	return fmt.Errorf("container %s not found in deployment %s", containerName, deployment.GetName())
}

// applyUserDeploymentConfigs updates the deployment resource spec with user specified configurations.
func (r *Reconciler) applyUserDeploymentConfigs(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig, assetName string) error {
	componentName, containerName, err := getComponentNameFromAsset(assetName)
	if err != nil {
		return err
	}

	for _, i := range esc.Spec.ControllerConfig.ComponentConfigs {
		if i.ComponentName == componentName {
			// Apply RevisionHistoryLimit if set
			if i.DeploymentConfigs != nil && i.DeploymentConfigs.RevisionHistoryLimit != nil {
				deployment.Spec.RevisionHistoryLimit = i.DeploymentConfigs.RevisionHistoryLimit
			}

			// Apply Replicas if set
			if i.DeploymentConfigs != nil && i.DeploymentConfigs.Replicas != nil {
				deployment.Spec.Replicas = i.DeploymentConfigs.Replicas
			}

			// Inject or remove leader election arg for the core controller based on replica count.
			if componentName == operatorv1alpha1.CoreController {
				for j := range deployment.Spec.Template.Spec.Containers {
					if deployment.Spec.Template.Spec.Containers[j].Name == containerName {
						applyLeaderElection(&deployment.Spec.Template.Spec.Containers[j], deployment.Spec.Replicas)
						break
					}
				}
			}

			// Apply OverrideEnv only to the target component container.
			if len(i.OverrideEnv) > 0 {
				for j := range deployment.Spec.Template.Spec.Containers {
					if deployment.Spec.Template.Spec.Containers[j].Name == containerName {
						mergeUserEnvVars(&deployment.Spec.Template.Spec.Containers[j], i.OverrideEnv)
						break
					}
				}
			}
			break
		}
	}

	return nil
}

// applyLeaderElection injects --enable-leader-election=true when replicas > 1 and
// removes it when replicas <= 1 (or nil) to ensure single-replica deployments do not
// pay the leader election overhead.
func applyLeaderElection(container *corev1.Container, replicas *int32) {
	wantLeaderElection := replicas != nil && *replicas > 1

	if wantLeaderElection {
		container.Args = mergeContainerArgs(container.Args, []string{LeaderElectionArg})
		return
	}

	// Remove the leader election arg if present
	filtered := make([]string, 0, len(container.Args))
	for _, arg := range container.Args {
		if argFlagKey(arg) != argFlagKey(LeaderElectionArg) {
			filtered = append(filtered, arg)
		}
	}
	container.Args = filtered
}

// mergeUserEnvVars merges user-defined environment variables into a container.
// User-defined values take precedence over existing values.
func mergeUserEnvVars(container *corev1.Container, overrideEnv []corev1.EnvVar) {
	if container.Env == nil {
		container.Env = []corev1.EnvVar{}
	}

	for _, override := range overrideEnv {
		found := false
		for i, existing := range container.Env {
			if existing.Name == override.Name {
				container.Env[i] = override // User-defined value takes precedence
				found = true
				break
			}
		}
		if !found {
			container.Env = append(container.Env, override)
		}
	}
}

// getComponentNameFromAsset maps asset file names to ComponentName enum values and container names.
func getComponentNameFromAsset(assetName string) (operatorv1alpha1.ComponentName, string, error) {
	switch assetName {
	case controllerDeploymentAssetName:
		return operatorv1alpha1.CoreController, OperandCoreControllerContainer, nil
	case webhookDeploymentAssetName:
		return operatorv1alpha1.Webhook, OperandWebhookContainer, nil
	case certControllerDeploymentAssetName:
		return operatorv1alpha1.CertController, OperandCertControllerContainer, nil
	case bitwardenDeploymentAssetName:
		return operatorv1alpha1.BitwardenSDKServer, OperandBitwardenContainer, nil
	default:
		return "", "", fmt.Errorf("unknown deployment asset name: %s", assetName)
	}
}

// updateOptionalFeatures appends container args for enabled ESM features that are
// supported by the target deployment. supportedFeatures declares which features
// this deployment accepts; features enabled in ESM but not listed here are ignored.
func (r *Reconciler) updateOptionalFeatures(containerArgs *[]string, supportedFeatures []operatorv1alpha1.FeatureName) {
	for _, featureName := range supportedFeatures {
		if !common.IsFeatureEnabled(r.esm, featureName) {
			r.log.V(4).Info("feature not active", "feature", featureName)
			continue
		}
		arg, ok := featureContainerArgs[featureName]
		if !ok {
			r.log.V(2).Info("feature enabled but no arg mapping for deployment", "feature", featureName)
			continue
		}
		*containerArgs = append(*containerArgs, arg)
	}
}
