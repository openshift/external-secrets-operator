package common

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	webhook "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	operatorclient "github.com/openshift/external-secrets-operator/pkg/controller/client"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

// Now is a rip-off of golang's sync.Once functionality but extended to
// support reset.
type Now struct {
	sync.Mutex
	done atomic.Uint32
}

func init() {
	if err := appsv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := networkingv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := certmanagerv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := webhook.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := crdv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
}

func UpdateResourceLabels(obj client.Object, labels map[string]string) {
	l := obj.GetLabels()
	if l == nil {
		l = make(map[string]string, len(labels))
	}
	for k, v := range labels {
		l[k] = v
	}
	obj.SetLabels(l)
}

func DecodeCertificateObjBytes(objBytes []byte) *certmanagerv1.Certificate {
	obj, err := runtime.Decode(codecs.UniversalDecoder(certmanagerv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*certmanagerv1.Certificate)
}

func DecodeClusterRoleObjBytes(objBytes []byte) *rbacv1.ClusterRole {
	obj, err := runtime.Decode(codecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*rbacv1.ClusterRole)
}

func DecodeClusterRoleBindingObjBytes(objBytes []byte) *rbacv1.ClusterRoleBinding {
	obj, err := runtime.Decode(codecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*rbacv1.ClusterRoleBinding)
}

func DecodeDeploymentObjBytes(objBytes []byte) *appsv1.Deployment {
	obj, err := runtime.Decode(codecs.UniversalDecoder(appsv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*appsv1.Deployment)
}

func DecodeRoleObjBytes(objBytes []byte) *rbacv1.Role {
	obj, err := runtime.Decode(codecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*rbacv1.Role)
}

func DecodeRoleBindingObjBytes(objBytes []byte) *rbacv1.RoleBinding {
	obj, err := runtime.Decode(codecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*rbacv1.RoleBinding)
}

func DecodeSecretObjBytes(objBytes []byte) *corev1.Secret {
	obj, err := runtime.Decode(codecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*corev1.Secret)
}

func DecodeServiceObjBytes(objBytes []byte) *corev1.Service {
	obj, err := runtime.Decode(codecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*corev1.Service)
}

func DecodeServiceAccountObjBytes(objBytes []byte) *corev1.ServiceAccount {
	obj, err := runtime.Decode(codecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*corev1.ServiceAccount)
}

func DecodeValidatingWebhookConfigurationObjBytes(objBytes []byte) *webhook.ValidatingWebhookConfiguration {
	obj, err := runtime.Decode(codecs.UniversalDecoder(webhook.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*webhook.ValidatingWebhookConfiguration)
}

func DecodeNetworkPolicyObjBytes(objBytes []byte) *networkingv1.NetworkPolicy {
	obj, err := runtime.Decode(codecs.UniversalDecoder(networkingv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*networkingv1.NetworkPolicy)
}

func HasObjectChanged(desired, fetched client.Object) bool {
	if reflect.TypeOf(desired) != reflect.TypeOf(fetched) {
		panic("both objects to be compared must be of same type")
	}

	var objectModified bool
	switch desired.(type) {
	case *certmanagerv1.Certificate:
		objectModified = certificateSpecModified(desired.(*certmanagerv1.Certificate), fetched.(*certmanagerv1.Certificate))
	case *rbacv1.ClusterRole:
		objectModified = rbacRoleRulesModified[*rbacv1.ClusterRole](desired.(*rbacv1.ClusterRole), fetched.(*rbacv1.ClusterRole))
	case *rbacv1.ClusterRoleBinding:
		objectModified = rbacRoleBindingRefModified[*rbacv1.ClusterRoleBinding](desired.(*rbacv1.ClusterRoleBinding), fetched.(*rbacv1.ClusterRoleBinding)) ||
			rbacRoleBindingSubjectsModified[*rbacv1.ClusterRoleBinding](desired.(*rbacv1.ClusterRoleBinding), fetched.(*rbacv1.ClusterRoleBinding))
	case *appsv1.Deployment:
		objectModified = deploymentSpecModified(desired.(*appsv1.Deployment), fetched.(*appsv1.Deployment))
	case *rbacv1.Role:
		objectModified = rbacRoleRulesModified[*rbacv1.Role](desired.(*rbacv1.Role), fetched.(*rbacv1.Role))
	case *rbacv1.RoleBinding:
		objectModified = rbacRoleBindingRefModified[*rbacv1.RoleBinding](desired.(*rbacv1.RoleBinding), fetched.(*rbacv1.RoleBinding)) ||
			rbacRoleBindingSubjectsModified[*rbacv1.RoleBinding](desired.(*rbacv1.RoleBinding), fetched.(*rbacv1.RoleBinding))
	case *corev1.Service:
		objectModified = serviceSpecModified(desired.(*corev1.Service), fetched.(*corev1.Service))
	case *networkingv1.NetworkPolicy:
		objectModified = networkPolicySpecModified(desired.(*networkingv1.NetworkPolicy), fetched.(*networkingv1.NetworkPolicy))
	case *webhook.ValidatingWebhookConfiguration:
		objectModified = validatingWebHookSpecModified(desired.(*webhook.ValidatingWebhookConfiguration), fetched.(*webhook.ValidatingWebhookConfiguration))
	default:
		panic(fmt.Sprintf("unsupported object type: %T", desired))
	}
	return objectModified || ObjectMetadataModified(desired, fetched)
}

func ObjectMetadataModified(desired, fetched client.Object) bool {
	return !reflect.DeepEqual(desired.GetLabels(), fetched.GetLabels())
}

func certificateSpecModified(desired, fetched *certmanagerv1.Certificate) bool {
	return !reflect.DeepEqual(desired.Spec, fetched.Spec)
}

func deploymentSpecModified(desired, fetched *appsv1.Deployment) bool {
	if desired.Spec.Replicas != nil && !reflect.DeepEqual(desired.Spec.Replicas, fetched.Spec.Replicas) {
		return true
	}

	if desired.Spec.Template.Spec.ServiceAccountName != fetched.Spec.Template.Spec.ServiceAccountName ||
		desired.Spec.Template.Spec.AutomountServiceAccountToken != nil {
		if !reflect.DeepEqual(desired.Spec.Template.Spec.AutomountServiceAccountToken, fetched.Spec.Template.Spec.AutomountServiceAccountToken) {
			return true
		}
	}

	if desired.Spec.Template.Spec.DNSPolicy != "" && desired.Spec.Template.Spec.DNSPolicy != fetched.Spec.Template.Spec.DNSPolicy {
		return true
	}

	if desired.Spec.Template.Labels != nil && !reflect.DeepEqual(desired.Spec.Template.Labels, fetched.Spec.Template.Labels) {
		return true
	}

	// Check volumes
	if !volumesEqual(desired.Spec.Template.Spec.Volumes, fetched.Spec.Template.Spec.Volumes) {
		return true
	}

	if desired.Spec.Template.Spec.NodeSelector != nil && !reflect.DeepEqual(desired.Spec.Template.Spec.NodeSelector, fetched.Spec.Template.Spec.NodeSelector) {
		return true
	}

	if desired.Spec.Template.Spec.Affinity != nil && !reflect.DeepEqual(desired.Spec.Template.Spec.Affinity, fetched.Spec.Template.Spec.Affinity) {
		return true
	}

	if desired.Spec.Template.Spec.Tolerations != nil && !reflect.DeepEqual(desired.Spec.Template.Spec.Tolerations, fetched.Spec.Template.Spec.Tolerations) {
		return true
	}

	if desired.Spec.RevisionHistoryLimit != nil && !reflect.DeepEqual(desired.Spec.RevisionHistoryLimit, fetched.Spec.RevisionHistoryLimit) {
		return true
	}

	// Check regular containers
	if len(desired.Spec.Template.Spec.Containers) != len(fetched.Spec.Template.Spec.Containers) {
		return true
	}
	fetchedContainers := make(map[string]*corev1.Container)
	for i := range fetched.Spec.Template.Spec.Containers {
		fetchedContainers[fetched.Spec.Template.Spec.Containers[i].Name] = &fetched.Spec.Template.Spec.Containers[i]
	}
	for i := range desired.Spec.Template.Spec.Containers {
		desiredContainer := &desired.Spec.Template.Spec.Containers[i]
		fetchedContainer, exists := fetchedContainers[desiredContainer.Name]
		if !exists {
			return true
		}
		if containerSpecModified(desiredContainer, fetchedContainer) {
			return true
		}
	}

	// Check init containers
	if len(desired.Spec.Template.Spec.InitContainers) != len(fetched.Spec.Template.Spec.InitContainers) {
		return true
	}
	fetchedInitContainers := make(map[string]*corev1.Container)
	for i := range fetched.Spec.Template.Spec.InitContainers {
		fetchedInitContainers[fetched.Spec.Template.Spec.InitContainers[i].Name] = &fetched.Spec.Template.Spec.InitContainers[i]
	}
	for i := range desired.Spec.Template.Spec.InitContainers {
		desiredInitContainer := &desired.Spec.Template.Spec.InitContainers[i]
		fetchedInitContainer, exists := fetchedInitContainers[desiredInitContainer.Name]
		if !exists {
			return true
		}
		if containerSpecModified(desiredInitContainer, fetchedInitContainer) {
			return true
		}
	}

	return false
}

func containerSpecModified(desiredContainer, fetchedContainer *corev1.Container) bool {
	// Check basic container properties
	if !reflect.DeepEqual(desiredContainer.Args, fetchedContainer.Args) ||
		desiredContainer.Name != fetchedContainer.Name ||
		desiredContainer.Image != fetchedContainer.Image ||
		desiredContainer.ImagePullPolicy != fetchedContainer.ImagePullPolicy {
		return true
	}

	// Check environment variables
	if !reflect.DeepEqual(desiredContainer.Env, fetchedContainer.Env) {
		return true
	}

	// Check ports
	if len(desiredContainer.Ports) != len(fetchedContainer.Ports) {
		return true
	}
	for _, fetchedPort := range fetchedContainer.Ports {
		matched := false
		for _, desiredPort := range desiredContainer.Ports {
			if fetchedPort.ContainerPort == desiredPort.ContainerPort {
				matched = true
				break
			}
		}
		if !matched {
			return true
		}
	}

	// ReadinessProbe nil checks
	if (desiredContainer.ReadinessProbe == nil) != (fetchedContainer.ReadinessProbe == nil) {
		return true
	}
	if desiredContainer.ReadinessProbe != nil && fetchedContainer.ReadinessProbe != nil {
		if (desiredContainer.ReadinessProbe.HTTPGet == nil) != (fetchedContainer.ReadinessProbe.HTTPGet == nil) {
			return true
		}
		if desiredContainer.ReadinessProbe.HTTPGet != nil && fetchedContainer.ReadinessProbe.HTTPGet != nil {
			if desiredContainer.ReadinessProbe.HTTPGet.Path != fetchedContainer.ReadinessProbe.HTTPGet.Path {
				return true
			}
		}
	}

	// SecurityContext nil check
	if desiredContainer.SecurityContext != nil && !reflect.DeepEqual(*desiredContainer.SecurityContext, *fetchedContainer.SecurityContext) {
		return true
	}

	// Check volume mounts
	if !reflect.DeepEqual(desiredContainer.VolumeMounts, fetchedContainer.VolumeMounts) {
		return true
	}

	// Check resources
	if !reflect.DeepEqual(desiredContainer.Resources, fetchedContainer.Resources) {
		return true
	}

	return false
}

func volumesEqual(desired, fetched []corev1.Volume) bool {
	if len(desired) == 0 && len(fetched) == 0 {
		return true
	}
	if len(desired) != len(fetched) {
		return false
	}

	// Create a map of fetched volumes by name for easier lookup
	fetchedMap := make(map[string]corev1.Volume)
	for _, v := range fetched {
		fetchedMap[v.Name] = v
	}

	// Check each desired volume exists and matches in fetched
	for _, desiredVol := range desired {
		fetchedVol, exists := fetchedMap[desiredVol.Name]
		if !exists {
			return false
		}

		// Compare volume sources
		// Check ConfigMap volume
		if desiredVol.ConfigMap != nil {
			if fetchedVol.ConfigMap == nil {
				return false
			}
			if desiredVol.ConfigMap.Name != fetchedVol.ConfigMap.Name {
				return false
			}
		}

		// Check Secret volume
		if desiredVol.Secret != nil {
			if fetchedVol.Secret == nil {
				return false
			}
			if desiredVol.Secret.SecretName != fetchedVol.Secret.SecretName {
				return false
			}
			if !reflect.DeepEqual(desiredVol.Secret.Items, fetchedVol.Secret.Items) {
				return false
			}
		}

		// Check EmptyDir volume
		if desiredVol.EmptyDir != nil {
			if fetchedVol.EmptyDir == nil {
				return false
			}
		}

		// Add other volume types as needed (PVC, HostPath, etc.)
	}

	return true
}

func serviceSpecModified(desired, fetched *corev1.Service) bool {
	if desired.Spec.Type != fetched.Spec.Type ||
		!reflect.DeepEqual(desired.Spec.Ports, fetched.Spec.Ports) ||
		!reflect.DeepEqual(desired.Spec.Selector, fetched.Spec.Selector) {
		return true
	}

	return false
}

func networkPolicySpecModified(desired, fetched *networkingv1.NetworkPolicy) bool {
	if !reflect.DeepEqual(desired.Spec.PodSelector, fetched.Spec.PodSelector) ||
		!reflect.DeepEqual(desired.Spec.PolicyTypes, fetched.Spec.PolicyTypes) ||
		!reflect.DeepEqual(desired.Spec.Ingress, fetched.Spec.Ingress) ||
		!reflect.DeepEqual(desired.Spec.Egress, fetched.Spec.Egress) {
		return true
	}

	return false
}

func rbacRoleRulesModified[Object *rbacv1.Role | *rbacv1.ClusterRole](desired, fetched Object) bool {
	switch typ := any(desired).(type) {
	case *rbacv1.ClusterRole:
		return !reflect.DeepEqual(any(desired).(*rbacv1.ClusterRole).Rules, any(fetched).(*rbacv1.ClusterRole).Rules)
	case *rbacv1.Role:
		return !reflect.DeepEqual(any(desired).(*rbacv1.Role).Rules, any(fetched).(*rbacv1.Role).Rules)
	default:
		panic(fmt.Sprintf("unsupported object type %v", typ))
	}
}

func rbacRoleBindingRefModified[Object *rbacv1.RoleBinding | *rbacv1.ClusterRoleBinding](desired, fetched Object) bool {
	switch typ := any(desired).(type) {
	case *rbacv1.ClusterRoleBinding:
		return !reflect.DeepEqual(any(desired).(*rbacv1.ClusterRoleBinding).RoleRef, any(fetched).(*rbacv1.ClusterRoleBinding).RoleRef)
	case *rbacv1.RoleBinding:
		return !reflect.DeepEqual(any(desired).(*rbacv1.RoleBinding).RoleRef, any(fetched).(*rbacv1.RoleBinding).RoleRef)
	default:
		panic(fmt.Sprintf("unsupported object type %v", typ))
	}
}

func rbacRoleBindingSubjectsModified[Object *rbacv1.RoleBinding | *rbacv1.ClusterRoleBinding](desired, fetched Object) bool {
	switch typ := any(desired).(type) {
	case *rbacv1.ClusterRoleBinding:
		return !reflect.DeepEqual(any(desired).(*rbacv1.ClusterRoleBinding).Subjects, any(fetched).(*rbacv1.ClusterRoleBinding).Subjects)
	case *rbacv1.RoleBinding:
		return !reflect.DeepEqual(any(desired).(*rbacv1.RoleBinding).Subjects, any(fetched).(*rbacv1.RoleBinding).Subjects)
	default:
		panic(fmt.Sprintf("unsupported object type %v", typ))
	}
}

func validatingWebHookSpecModified(desired, fetched *webhook.ValidatingWebhookConfiguration) bool {
	if len(desired.Webhooks) != len(fetched.Webhooks) {
		return true
	}

	if desiredVal, exists := desired.GetAnnotations()[CertManagerInjectCAFromAnnotation]; exists {
		if desiredVal != fetched.GetAnnotations()[CertManagerInjectCAFromAnnotation] {
			return true
		}
	}

	fetchedWebhooksMap := make(map[string]webhook.ValidatingWebhook)
	for _, wh := range fetched.Webhooks {
		fetchedWebhooksMap[wh.Name] = wh
	}

	for _, desiredWh := range desired.Webhooks {
		fetchedWh, ok := fetchedWebhooksMap[desiredWh.Name]
		if !ok {
			return true
		}

		if !reflect.DeepEqual(desiredWh.SideEffects, fetchedWh.SideEffects) ||
			!reflect.DeepEqual(desiredWh.TimeoutSeconds, fetchedWh.TimeoutSeconds) ||
			!reflect.DeepEqual(desiredWh.AdmissionReviewVersions, fetchedWh.AdmissionReviewVersions) ||
			!reflect.DeepEqual(desiredWh.ClientConfig.Service.Name, fetchedWh.ClientConfig.Service.Name) ||
			!reflect.DeepEqual(desiredWh.ClientConfig.Service.Path, fetchedWh.ClientConfig.Service.Path) ||
			!reflect.DeepEqual(desiredWh.Rules, fetchedWh.Rules) {
			return true
		}
	}

	return false
}

// ParseBool is for parsing a string value as a boolean value. This is very specific to the values
// read from CR which allows only `true` or `false` as values.
func ParseBool(val string) bool {
	return val == "true"
}

// EvalMode is for evaluating the Mode values and return a boolean. This is very specific to the values
// read from CR which allows only `Enabled`, `Disabled` or `DisabledAndCleanup` as values. Returns
// true when has `Enabled` and false for every other value.
func EvalMode(val operatorv1alpha1.Mode) bool {
	return val == operatorv1alpha1.Enabled
}

// IsESMSpecEmpty returns whether ExternalSecretsManager CR Spec is empty.
func IsESMSpecEmpty(esm *operatorv1alpha1.ExternalSecretsManager) bool {
	return esm != nil && !reflect.DeepEqual(esm.Spec, operatorv1alpha1.ExternalSecretsManagerSpec{})
}

// IsInjectCertManagerAnnotationEnabled is for check if add cert-manager annotation is enabled.
func IsInjectCertManagerAnnotationEnabled(esc *operatorv1alpha1.ExternalSecretsConfig) bool {
	return esc.Spec.ControllerConfig.CertProvider != nil &&
		esc.Spec.ControllerConfig.CertProvider.CertManager != nil &&
		ParseBool(esc.Spec.ControllerConfig.CertProvider.CertManager.InjectAnnotations)
}

// AddFinalizer adds finalizer to the passed resource object.
func AddFinalizer(ctx context.Context, obj client.Object, opClient operatorclient.CtrlClient, finalizer string) error {
	namespacedName := client.ObjectKeyFromObject(obj)
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		if !controllerutil.AddFinalizer(obj, finalizer) {
			return fmt.Errorf("failed to create %q object with finalizers added", namespacedName)
		}

		if err := opClient.UpdateWithRetry(ctx, obj); err != nil {
			return fmt.Errorf("failed to add finalizers on %q with %w", namespacedName, err)
		}

		switch o := obj.(type) {
		case *operatorv1alpha1.ExternalSecretsManager:
			updated := &operatorv1alpha1.ExternalSecretsManager{}
			if err := opClient.Get(ctx, namespacedName, updated); err != nil {
				return fmt.Errorf("failed to fetch %q after updating finalizers: %w", namespacedName, err)
			}
			updated.DeepCopyInto(o)
		case *operatorv1alpha1.ExternalSecretsConfig:
			updated := &operatorv1alpha1.ExternalSecretsConfig{}
			if err := opClient.Get(ctx, namespacedName, updated); err != nil {
				return fmt.Errorf("failed to fetch %q after updating finalizers: %w", namespacedName, err)
			}
			updated.DeepCopyInto(o)
		default:
			return fmt.Errorf("adding finalizer to %T object not handled", obj)
		}
		return nil
	}
	return nil
}

// RemoveFinalizer removes finalizers added from the passed resource object.
func RemoveFinalizer(ctx context.Context, obj client.Object, opClient operatorclient.CtrlClient, finalizer string) error {
	namespacedName := client.ObjectKeyFromObject(obj)
	if controllerutil.ContainsFinalizer(obj, finalizer) {
		if !controllerutil.RemoveFinalizer(obj, finalizer) {
			return fmt.Errorf("failed to remove finalizers on %q", namespacedName)
		}

		if err := opClient.UpdateWithRetry(ctx, obj); err != nil {
			return fmt.Errorf("update failed to remove finalizers on %q: %w", namespacedName, err)
		}
		return nil
	}
	return nil
}

// Do is same as sync.Once.Do, which calls the passed func f only once
// until Now is reset.
func (n *Now) Do(f func()) {
	n.done.Load()
	if n.done.Load() == 0 {
		n.Lock()
		defer n.Unlock()

		defer n.done.Store(1)
		f()
	}
}

// Reset is for allowing the Do method to call the func f again.
func (n *Now) Reset() {
	n.Lock()
	defer n.Unlock()

	n.done.Store(0)
}
