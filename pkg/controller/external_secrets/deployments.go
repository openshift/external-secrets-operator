package external_secrets

import (
	"fmt"
	"os"
	"unsafe"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core"
	corevalidation "k8s.io/kubernetes/pkg/apis/core/validation"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

// createOrApplyDeployments ensures required Deployment resources exist and are correctly configured.
func (r *Reconciler) createOrApplyDeployments(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string, externalSecretsConfigCreateRecon bool) error {
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
		if err := r.createOrApplyDeploymentFromAsset(esc, d.assetName, resourceLabels, externalSecretsConfigCreateRecon); err != nil {
			return err
		}
	}

	if err := r.updateImageInStatus(esc); err != nil {
		return common.FromClientError(err, "failed to update %s/%s status with image info", esc.GetNamespace(), esc.GetName())
	}

	return nil
}

func (r *Reconciler) createOrApplyDeploymentFromAsset(esc *operatorv1alpha1.ExternalSecretsConfig, assetName string, resourceLabels map[string]string,
	externalSecretsConfigCreateRecon bool,
) error {

	deployment, err := r.getDeploymentObject(assetName, esc, resourceLabels)
	if err != nil {
		return err
	}

	deploymentName := fmt.Sprintf("%s/%s", deployment.GetNamespace(), deployment.GetName())
	fetched := &appsv1.Deployment{}
	exist, err := r.Exists(r.ctx, client.ObjectKeyFromObject(deployment), fetched)
	if err != nil {
		return common.FromClientError(err, "failed to check %s deployment resource already exists", deploymentName)
	}
	if exist && externalSecretsConfigCreateRecon {
		r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s deployment resource already exists", deploymentName)
	}
	if exist && common.HasObjectChanged(deployment, fetched) {
		r.log.V(1).Info("deployment has been modified, updating to desired state", "name", deploymentName)
		if err := r.UpdateWithRetry(r.ctx, deployment); err != nil {
			return common.FromClientError(err, "failed to update %s deployment resource", deploymentName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "deployment resource %s updated", deploymentName)
	} else if !exist {
		if err := r.Create(r.ctx, deployment); err != nil {
			return common.FromClientError(err, "failed to create %s deployment resource", deploymentName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "deployment resource %s created", deploymentName)
	} else {
		r.log.V(4).Info("deployment resource already exists and is in expected state", "name", deploymentName)
	}

	return nil
}

func (r *Reconciler) getDeploymentObject(assetName string, esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string) (*appsv1.Deployment, error) {
	deployment := common.DecodeDeploymentObjBytes(assets.MustAsset(assetName))
	updateNamespace(deployment, esc)
	common.UpdateResourceLabels(deployment, resourceLabels)
	updatePodTemplateLabels(deployment, resourceLabels)

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
		updateContainerSpec(deployment, esc, image, logLevel)
	case webhookDeploymentAssetName:
		checkInterval := "5m"
		if esc.Spec.ApplicationConfig.WebhookConfig != nil &&
			esc.Spec.ApplicationConfig.WebhookConfig.CertificateCheckInterval != nil {
			checkInterval = esc.Spec.ApplicationConfig.WebhookConfig.CertificateCheckInterval.Duration.String()
		}
		updateWebhookContainerSpec(deployment, image, logLevel, checkInterval)
		updateWebhookVolumeConfig(deployment, esc)
	case certControllerDeploymentAssetName:
		updateCertControllerContainerSpec(deployment, image, logLevel)
	case bitwardenDeploymentAssetName:
		deployment.Labels["app.kubernetes.io/version"] = os.Getenv(bitwardenImageVersionEnvVarName)
		updateBitwardenServerContainerSpec(deployment, bitwardenImage)
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
	if err := r.updateProxyConfiguration(deployment, esc); err != nil {
		return nil, fmt.Errorf("failed to update proxy configuration: %w", err)
	}
	if err := r.applyUserDeploymentConfigs(deployment, esc, assetName); err != nil {
		return nil, fmt.Errorf("failed to apply user deployment configuration: %w", err)
	}

	return deployment, nil
}

// updatePodTemplateLabels sets labels on the pod template spec.
func updatePodTemplateLabels(deployment *appsv1.Deployment, labels map[string]string) {
	l := deployment.Spec.Template.GetLabels()
	for k, v := range labels {
		l[k] = v
	}
	deployment.Spec.Template.SetLabels(l)
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
	if esc.Spec.ApplicationConfig.Resources != nil {
		esc.Spec.ApplicationConfig.Resources.DeepCopyInto(&rscReqs)
	} else if r.esm.Spec.GlobalConfig != nil && r.esm.Spec.GlobalConfig.Resources != nil {
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
	return corevalidation.ValidateTolerations(convTolerations, fldPath.Child("tolerations")).ToAggregate()
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

// argument list for external-secrets deployment resource
func updateContainerSpec(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig, image, logLevel string) {
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

// updateProxyConfiguration applies all proxy-related configuration to the deployment.
func (r *Reconciler) updateProxyConfiguration(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig) error {
	proxyConfig := r.getProxyConfiguration(esc)

	if err := r.updateProxyEnvironmentVariables(deployment, proxyConfig); err != nil {
		return fmt.Errorf("failed to update proxy environment variables: %w", err)
	}
	if err := r.updateTrustedCABundleVolumes(deployment, proxyConfig); err != nil {
		return fmt.Errorf("failed to update trusted CA bundle volumes: %w", err)
	}

	return nil
}

// updateProxyEnvironmentVariables sets or removes proxy environment variables on all containers and init containers in the deployment.
func (r *Reconciler) updateProxyEnvironmentVariables(deployment *appsv1.Deployment, proxyConfig *operatorv1alpha1.ProxyConfig) error {
	// Apply proxy environment variables to all containers
	for i := range deployment.Spec.Template.Spec.Containers {
		container := &deployment.Spec.Template.Spec.Containers[i]
		if proxyConfig != nil {
			r.setProxyEnvVars(container, proxyConfig)
		} else {
			r.removeProxyEnvVars(container)
		}
	}

	// Apply proxy environment variables to all init containers
	for i := range deployment.Spec.Template.Spec.InitContainers {
		initContainer := &deployment.Spec.Template.Spec.InitContainers[i]
		if proxyConfig != nil {
			r.setProxyEnvVars(initContainer, proxyConfig)
		} else {
			r.removeProxyEnvVars(initContainer)
		}
	}

	return nil
}

// setProxyEnvVars sets proxy environment variables on a container.
func (r *Reconciler) setProxyEnvVars(container *corev1.Container, proxyConfig *operatorv1alpha1.ProxyConfig) {
	if proxyConfig == nil {
		return
	}
	if container.Env == nil {
		container.Env = []corev1.EnvVar{}
	}

	setEnvVar := func(name, value string) {
		if value == "" {
			return
		}

		// Check if the environment variable already exists
		for i, env := range container.Env {
			if env.Name == name {
				container.Env[i].Value = value
				return
			}
		}

		// Add new environment variable if it doesn't exist
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  name,
			Value: value,
		})
	}

	// Set proxy environment variables
	setEnvVar(httpProxyEnvVar, proxyConfig.HTTPProxy)
	setEnvVar(httpsProxyEnvVar, proxyConfig.HTTPSProxy)
	setEnvVar(noProxyEnvVar, proxyConfig.NoProxy)

	setEnvVar(httpProxyEnvVarLowercase, proxyConfig.HTTPProxy)
	setEnvVar(httpsProxyEnvVarLowercase, proxyConfig.HTTPSProxy)
	setEnvVar(noProxyEnvVarLowercase, proxyConfig.NoProxy)
}

// removeProxyEnvVars removes proxy environment variables from a container.
func (r *Reconciler) removeProxyEnvVars(container *corev1.Container) {
	if len(container.Env) == 0 {
		return
	}

	// Helper function to check if an env var name is a proxy variable
	isProxyEnvVar := func(name string) bool {
		switch name {
		case httpProxyEnvVar, httpsProxyEnvVar, noProxyEnvVar,
			httpProxyEnvVarLowercase, httpsProxyEnvVarLowercase, noProxyEnvVarLowercase:
			return true
		default:
			return false
		}
	}

	// Filter out proxy environment variables
	filteredEnv := make([]corev1.EnvVar, 0, len(container.Env))
	for _, env := range container.Env {
		if !isProxyEnvVar(env.Name) {
			filteredEnv = append(filteredEnv, env)
		}
	}
	container.Env = filteredEnv
}

// updateTrustedCABundleVolumes adds or removes trusted CA bundle volume and volume mounts to/from the deployment
// based on proxy configuration presence.
func (r *Reconciler) updateTrustedCABundleVolumes(deployment *appsv1.Deployment, proxyConfig *operatorv1alpha1.ProxyConfig) error {

	if proxyConfig != nil {
		// Add trusted CA bundle volume and volume mounts
		return r.addTrustedCABundleVolumes(deployment)
	} else {
		// Remove trusted CA bundle volume and volume mounts
		return r.removeTrustedCABundleVolumes(deployment)
	}
}

// addTrustedCABundleVolumes adds trusted CA bundle volume and volume mounts to the deployment.
func (r *Reconciler) addTrustedCABundleVolumes(deployment *appsv1.Deployment) error {
	// Add the trusted CA bundle volume to the pod spec
	trustedCAVolume := corev1.Volume{
		Name: trustedCABundleVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: trustedCABundleConfigMapName,
				},
			},
		},
	}

	// Check if the volume already exists, if not add it
	volumeExists := false
	for i, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Name == trustedCABundleVolumeName {
			deployment.Spec.Template.Spec.Volumes[i] = trustedCAVolume
			volumeExists = true
			break
		}
	}
	if !volumeExists {
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, trustedCAVolume)
	}

	// Add volume mounts to all containers and init containers
	trustedCAVolumeMount := corev1.VolumeMount{
		Name:      trustedCABundleVolumeName,
		MountPath: trustedCABundleMountPath,
		ReadOnly:  true,
	}

	// Add volume mount to all containers
	for i := range deployment.Spec.Template.Spec.Containers {
		container := &deployment.Spec.Template.Spec.Containers[i]
		r.addTrustedCAVolumeMount(container, trustedCAVolumeMount)
	}

	// Add volume mount to all init containers
	for i := range deployment.Spec.Template.Spec.InitContainers {
		initContainer := &deployment.Spec.Template.Spec.InitContainers[i]
		r.addTrustedCAVolumeMount(initContainer, trustedCAVolumeMount)
	}

	return nil
}

// removeTrustedCABundleVolumes removes trusted CA bundle volume and volume mounts from the deployment.
func (r *Reconciler) removeTrustedCABundleVolumes(deployment *appsv1.Deployment) error {
	// Remove the trusted CA bundle volume from the pod spec
	var filteredVolumes []corev1.Volume
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Name != trustedCABundleVolumeName {
			filteredVolumes = append(filteredVolumes, volume)
		}
	}
	deployment.Spec.Template.Spec.Volumes = filteredVolumes

	// Remove volume mounts from all containers
	for i := range deployment.Spec.Template.Spec.Containers {
		container := &deployment.Spec.Template.Spec.Containers[i]
		r.removeTrustedCAVolumeMount(container)
	}

	// Remove volume mounts from all init containers
	for i := range deployment.Spec.Template.Spec.InitContainers {
		initContainer := &deployment.Spec.Template.Spec.InitContainers[i]
		r.removeTrustedCAVolumeMount(initContainer)
	}

	return nil
}

// addTrustedCAVolumeMount adds the trusted CA bundle volume mount to a container if it doesn't already exist.
func (r *Reconciler) addTrustedCAVolumeMount(container *corev1.Container, trustedCAVolumeMount corev1.VolumeMount) {
	// Check if the volume mount already exists, if not add it
	volumeMountExists := false
	for j, volumeMount := range container.VolumeMounts {
		if volumeMount.Name == trustedCABundleVolumeName {
			container.VolumeMounts[j] = trustedCAVolumeMount
			volumeMountExists = true
			break
		}
	}
	if !volumeMountExists {
		container.VolumeMounts = append(container.VolumeMounts, trustedCAVolumeMount)
	}
}

// removeTrustedCAVolumeMount removes the trusted CA bundle volume mount from a container.
func (r *Reconciler) removeTrustedCAVolumeMount(container *corev1.Container) {
	var filteredVolumeMounts []corev1.VolumeMount
	for _, volumeMount := range container.VolumeMounts {
		if volumeMount.Name != trustedCABundleVolumeName {
			filteredVolumeMounts = append(filteredVolumeMounts, volumeMount)
		}
	}
	container.VolumeMounts = filteredVolumeMounts
}

// applyUserDeploymentConfigs updates the deployment resource spec with user specified configurations.
func (r *Reconciler) applyUserDeploymentConfigs(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig, assetName string) error {
	componentName, err := getComponentNameFromAsset(assetName)
	if err != nil {
		return err
	}

	for _, i := range esc.Spec.ControllerConfig.ComponentConfigs {
		if i.ComponentName == componentName {
			// Apply RevisionHistoryLimit if set
			if i.DeploymentConfigs != nil && i.DeploymentConfigs.RevisionHistoryLimit != nil {
				deployment.Spec.RevisionHistoryLimit = i.DeploymentConfigs.RevisionHistoryLimit
			}
			break
		}
	}

	return nil
}

// getComponentNameFromAsset maps asset file names to ComponentName enum values.
func getComponentNameFromAsset(assetName string) (operatorv1alpha1.ComponentName, error) {
	switch assetName {
	case controllerDeploymentAssetName:
		return operatorv1alpha1.CoreController, nil
	case webhookDeploymentAssetName:
		return operatorv1alpha1.Webhook, nil
	case certControllerDeploymentAssetName:
		return operatorv1alpha1.CertController, nil
	case bitwardenDeploymentAssetName:
		return operatorv1alpha1.BitwardenSDKServer, nil
	default:
		return "", fmt.Errorf("unknown deployment asset name: %s", assetName)
	}
}
