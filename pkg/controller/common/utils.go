package common

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"sort"
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

	"github.com/go-logr/logr"

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

// ResourceMetadata holds the labels and annotations to apply to all managed resources.
type ResourceMetadata struct {
	Labels      map[string]string
	Annotations map[string]string

	DeletedAnnotationKeys []string
}

// ApplyResourceMetadata applies both labels and managed annotations to a resource object.
func ApplyResourceMetadata(obj client.Object, metadata ResourceMetadata) {
	UpdateResourceLabels(obj, metadata.Labels)
	UpdateResourceAnnotations(obj, metadata.Annotations)
}

func UpdateResourceLabels(obj client.Object, labels map[string]string) {
	l := obj.GetLabels()
	if l == nil {
		l = make(map[string]string, len(labels))
	}
	maps.Copy(l, labels)
	obj.SetLabels(l)
}

// UpdateResourceAnnotations merges userAnnotations into the object's existing annotations.
// The tracking of which keys are managed is handled separately on the ExternalSecretsConfig CR
// via UpdateResourceAnnotations.
func UpdateResourceAnnotations(obj client.Object, userAnnotations map[string]string) {
	a := obj.GetAnnotations()
	if a == nil {
		a = make(map[string]string)
	}

	// Merge user annotations
	maps.Copy(a, userAnnotations)
	obj.SetAnnotations(a)
}

// GetManagedAnnotationKeys reads the ManagedAnnotationsKey tracking annotation from the
// ExternalSecretsConfig CR, base64-decodes and JSON-unmarshals it, and returns the list
// of managed annotation. Returns nil if the annotation is missing or cannot be decoded.
func GetManagedAnnotationKeys(esc *operatorv1alpha1.ExternalSecretsConfig) []string {
	annotations := esc.GetAnnotations()
	if annotations == nil {
		return nil
	}
	encoded, ok := annotations[ManagedAnnotationsKey]
	if !ok || encoded == "" {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}
	var keys []string
	if err := json.Unmarshal(decoded, &keys); err != nil {
		return nil
	}
	return keys
}

func DecodeCertificateObjBytes(objBytes []byte) *certmanagerv1.Certificate {
	obj, err := runtime.Decode(codecs.UniversalDecoder(certmanagerv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	result, ok := obj.(*certmanagerv1.Certificate)
	if !ok {
		panic(fmt.Sprintf("decoded object is not a Certificate: %T", obj))
	}
	return result
}

func DecodeClusterRoleObjBytes(objBytes []byte) *rbacv1.ClusterRole {
	obj, err := runtime.Decode(codecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	result, ok := obj.(*rbacv1.ClusterRole)
	if !ok {
		panic(fmt.Sprintf("decoded object is not a ClusterRole: %T", obj))
	}
	return result
}

func DecodeClusterRoleBindingObjBytes(objBytes []byte) *rbacv1.ClusterRoleBinding {
	obj, err := runtime.Decode(codecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	result, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok {
		panic(fmt.Sprintf("decoded object is not a ClusterRoleBinding: %T", obj))
	}
	return result
}

func DecodeDeploymentObjBytes(objBytes []byte) *appsv1.Deployment {
	obj, err := runtime.Decode(codecs.UniversalDecoder(appsv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	result, ok := obj.(*appsv1.Deployment)
	if !ok {
		panic(fmt.Sprintf("decoded object is not a Deployment: %T", obj))
	}
	return result
}

func DecodeRoleObjBytes(objBytes []byte) *rbacv1.Role {
	obj, err := runtime.Decode(codecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	result, ok := obj.(*rbacv1.Role)
	if !ok {
		panic(fmt.Sprintf("decoded object is not a Role: %T", obj))
	}
	return result
}

func DecodeRoleBindingObjBytes(objBytes []byte) *rbacv1.RoleBinding {
	obj, err := runtime.Decode(codecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	result, ok := obj.(*rbacv1.RoleBinding)
	if !ok {
		panic(fmt.Sprintf("decoded object is not a RoleBinding: %T", obj))
	}
	return result
}

func DecodeSecretObjBytes(objBytes []byte) *corev1.Secret {
	obj, err := runtime.Decode(codecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	result, ok := obj.(*corev1.Secret)
	if !ok {
		panic(fmt.Sprintf("decoded object is not a Secret: %T", obj))
	}
	return result
}

func DecodeServiceObjBytes(objBytes []byte) *corev1.Service {
	obj, err := runtime.Decode(codecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	result, ok := obj.(*corev1.Service)
	if !ok {
		panic(fmt.Sprintf("decoded object is not a Service: %T", obj))
	}
	return result
}

func DecodeServiceAccountObjBytes(objBytes []byte) *corev1.ServiceAccount {
	obj, err := runtime.Decode(codecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	result, ok := obj.(*corev1.ServiceAccount)
	if !ok {
		panic(fmt.Sprintf("decoded object is not a ServiceAccount: %T", obj))
	}
	return result
}

func DecodeValidatingWebhookConfigurationObjBytes(objBytes []byte) *webhook.ValidatingWebhookConfiguration {
	obj, err := runtime.Decode(codecs.UniversalDecoder(webhook.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	result, ok := obj.(*webhook.ValidatingWebhookConfiguration)
	if !ok {
		panic(fmt.Sprintf("decoded object is not a ValidatingWebhookConfiguration: %T", obj))
	}
	return result
}

func DecodeNetworkPolicyObjBytes(objBytes []byte) *networkingv1.NetworkPolicy {
	obj, err := runtime.Decode(codecs.UniversalDecoder(networkingv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	result, ok := obj.(*networkingv1.NetworkPolicy)
	if !ok {
		panic(fmt.Sprintf("decoded object is not a NetworkPolicy: %T", obj))
	}
	return result
}

func HasObjectChanged(desired, fetched client.Object, metaState *ResourceMetadata) bool {
	if reflect.TypeOf(desired) != reflect.TypeOf(fetched) {
		panic("both objects to be compared must be of same type")
	}

	var objectModified bool
	//nolint:forcetypeassert // Type assertion for fetched is safe - type equality checked the first before switch.
	switch d := desired.(type) {
	case *certmanagerv1.Certificate:
		f := fetched.(*certmanagerv1.Certificate)
		objectModified = certificateSpecModified(d, f)
	case *rbacv1.ClusterRole:
		f := fetched.(*rbacv1.ClusterRole)
		objectModified = rbacRoleRulesModified[*rbacv1.ClusterRole](d, f)
	case *rbacv1.ClusterRoleBinding:
		f := fetched.(*rbacv1.ClusterRoleBinding)
		objectModified = rbacRoleBindingRefModified[*rbacv1.ClusterRoleBinding](d, f) ||
			rbacRoleBindingSubjectsModified[*rbacv1.ClusterRoleBinding](d, f)
	case *appsv1.Deployment:
		f := fetched.(*appsv1.Deployment)
		objectModified = deploymentSpecModified(d, f, metaState)
	case *rbacv1.Role:
		f := fetched.(*rbacv1.Role)
		objectModified = rbacRoleRulesModified[*rbacv1.Role](d, f)
	case *rbacv1.RoleBinding:
		f := fetched.(*rbacv1.RoleBinding)
		objectModified = rbacRoleBindingRefModified[*rbacv1.RoleBinding](d, f) ||
			rbacRoleBindingSubjectsModified[*rbacv1.RoleBinding](d, f)
	case *corev1.Service:
		f := fetched.(*corev1.Service)
		objectModified = serviceSpecModified(d, f)
	case *networkingv1.NetworkPolicy:
		f := fetched.(*networkingv1.NetworkPolicy)
		objectModified = networkPolicySpecModified(d, f)
	case *webhook.ValidatingWebhookConfiguration:
		f := fetched.(*webhook.ValidatingWebhookConfiguration)
		objectModified = validatingWebHookSpecModified(d, f)
	case *corev1.ServiceAccount:
		objectModified = false
	default:
		panic(fmt.Sprintf("unsupported object type: %T", desired))
	}

	return objectModified || ObjectMetadataModified(desired, fetched, metaState)
}

func ObjectMetadataModified(desired, fetched client.Object, metaState *ResourceMetadata) bool {
	// Check if labels have changed
	if !reflect.DeepEqual(desired.GetLabels(), fetched.GetLabels()) {
		return true
	}

	// Compare only managed annotation keys to avoid infinite reconcile loops caused by
	// annotations managed by external controllers.
	return annotationMapsModified(desired.GetAnnotations(), fetched.GetAnnotations(), metaState.DeletedAnnotationKeys)
}

// annotationMapsModified checks whether managed annotation keys differ between two annotation maps.
// It returns true if any desired key is missing or has a different value in fetchedAnnotations, or if any
// previously-managed key still exists in fetchedAnnotations but is no longer in the current set.
// Extra annotations in fetchedAnnotations that are not in desiredAnnotations (e.g. added by
// external controllers like deployment.kubernetes.io/revision) are intentionally ignored.
func annotationMapsModified(desiredAnnotations, fetchedAnnotations map[string]string, deletedAnnotationKeys []string) bool {
	for k, v := range desiredAnnotations {
		if fetchedAnnotations[k] != v {
			return true
		}
	}
	for _, k := range deletedAnnotationKeys {
		if _, exists := fetchedAnnotations[k]; exists {
			return true
		}
	}
	return false
}

func certificateSpecModified(desired, fetched *certmanagerv1.Certificate) bool {
	return !reflect.DeepEqual(desired.Spec, fetched.Spec)
}

func deploymentSpecModified(desired, fetched *appsv1.Deployment, metaState *ResourceMetadata) bool {
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

	// Check pod template annotations using managed-keys comparison
	if annotationMapsModified(desired.Spec.Template.Annotations, fetched.Spec.Template.Annotations, metaState.DeletedAnnotationKeys) {
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
	//nolint:forcetypeassert // Type assertion is safe - generic constraint guarantees type match
	switch any(desired).(type) {
	case *rbacv1.ClusterRole:
		return !reflect.DeepEqual(any(desired).(*rbacv1.ClusterRole).Rules, any(fetched).(*rbacv1.ClusterRole).Rules)
	case *rbacv1.Role:
		return !reflect.DeepEqual(any(desired).(*rbacv1.Role).Rules, any(fetched).(*rbacv1.Role).Rules)
	default:
		panic(fmt.Sprintf("unsupported object type %T", desired))
	}
}

func rbacRoleBindingRefModified[Object *rbacv1.RoleBinding | *rbacv1.ClusterRoleBinding](desired, fetched Object) bool {
	//nolint:forcetypeassert // Type assertion is safe - generic constraint guarantees type match
	switch any(desired).(type) {
	case *rbacv1.ClusterRoleBinding:
		return !reflect.DeepEqual(any(desired).(*rbacv1.ClusterRoleBinding).RoleRef, any(fetched).(*rbacv1.ClusterRoleBinding).RoleRef)
	case *rbacv1.RoleBinding:
		return !reflect.DeepEqual(any(desired).(*rbacv1.RoleBinding).RoleRef, any(fetched).(*rbacv1.RoleBinding).RoleRef)
	default:
		panic(fmt.Sprintf("unsupported object type %T", desired))
	}
}

func rbacRoleBindingSubjectsModified[Object *rbacv1.RoleBinding | *rbacv1.ClusterRoleBinding](desired, fetched Object) bool {
	//nolint:forcetypeassert // Type assertion is safe - generic constraint guarantees type match
	switch any(desired).(type) {
	case *rbacv1.ClusterRoleBinding:
		return !reflect.DeepEqual(any(desired).(*rbacv1.ClusterRoleBinding).Subjects, any(fetched).(*rbacv1.ClusterRoleBinding).Subjects)
	case *rbacv1.RoleBinding:
		return !reflect.DeepEqual(any(desired).(*rbacv1.RoleBinding).Subjects, any(fetched).(*rbacv1.RoleBinding).Subjects)
	default:
		panic(fmt.Sprintf("unsupported object type %T", desired))
	}
}

func validatingWebHookSpecModified(desired, fetched *webhook.ValidatingWebhookConfiguration) bool {
	if len(desired.Webhooks) != len(fetched.Webhooks) {
		return true
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
// Do calls f() only once until Reset is called, similar to sync.Once.Do.
// Uses double-checked locking to ensure thread-safety.
func (n *Now) Do(f func()) {
	if n.done.Load() == 0 {
		n.Lock()
		defer n.Unlock()
		if n.done.Load() == 0 {
			defer n.done.Store(1)
			f()
		}
	}
}

// Reset is for allowing the Do method to call the func f again.
func (n *Now) Reset() {
	n.Lock()
	defer n.Unlock()

	n.done.Store(0)
}

func RemoveObsoleteAnnotations(obj client.Object, resourceMetadata ResourceMetadata) {
	if len(resourceMetadata.DeletedAnnotationKeys) == 0 {
		return
	}
	annotations := obj.GetAnnotations()
	obj.SetAnnotations(removeObsoleteAnnotations(annotations, resourceMetadata.DeletedAnnotationKeys))
}

func RemoveObsoleteAnnotationsInPodSpec(obj *appsv1.Deployment, resourceMetadata ResourceMetadata) {
	if len(resourceMetadata.DeletedAnnotationKeys) == 0 {
		return
	}
	obj.Spec.Template = obj.Spec.DeepCopy().Template
	updatedAnnotations := removeObsoleteAnnotations(obj.Spec.Template.Annotations, resourceMetadata.DeletedAnnotationKeys)
	obj.Spec.Template.Annotations = updatedAnnotations
}

func removeObsoleteAnnotations(annotations map[string]string, deletedAnnots []string) map[string]string {
	for _, k := range deletedAnnots {
		delete(annotations, k)
	}
	return annotations
}

// GetResourceMetadata builds the labels and annotations to apply to all operand resources,
// and computes which annotation keys were previously managed but are no longer in spec
// (DeletedAnnotationKeys) so downstream logic can remove them from resources.
func GetResourceMetadata(log logr.Logger, esm *operatorv1alpha1.ExternalSecretsManager, esc *operatorv1alpha1.ExternalSecretsConfig, defaultLabels map[string]string) (ResourceMetadata, error) {
	resourceMetadata := ResourceMetadata{}
	GetResourceLabels(log, esm, esc, &resourceMetadata, defaultLabels)
	if err := GetResourceAnnotations(esc, &resourceMetadata); err != nil {
		return resourceMetadata, err
	}
	return resourceMetadata, nil
}

// GetResourceLabels builds the labels map for all operand resources. Labels defined in
// ExternalSecretsManager have the lowest priority, followed by ExternalSecretsConfig labels,
// and defaultLabels have the highest priority. Labels matching DisallowedLabelMatcher are skipped.
func GetResourceLabels(log logr.Logger, esm *operatorv1alpha1.ExternalSecretsManager, esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata *ResourceMetadata, defaultLabels map[string]string) {
	resourceMetadata.Labels = make(map[string]string)
	if !IsESMSpecEmpty(esm) && esm != nil && esm.Spec.GlobalConfig != nil {
		for k, v := range esm.Spec.GlobalConfig.Labels {
			if DisallowedLabelMatcher.MatchString(k) {
				log.V(1).Info("skip adding unallowed label configured in externalsecretsmanagers.operator.openshift.io", "label", k, "value", v)
				continue
			}
			resourceMetadata.Labels[k] = v
		}
	}
	if len(esc.Spec.ControllerConfig.Labels) != 0 {
		for k, v := range esc.Spec.ControllerConfig.Labels {
			if DisallowedLabelMatcher.MatchString(k) {
				log.V(1).Info("skip adding unallowed label configured in externalsecretsconfig.operator.openshift.io", "label", k, "value", v)
				continue
			}
			resourceMetadata.Labels[k] = v
		}
	}
	maps.Copy(resourceMetadata.Labels, defaultLabels)
}

// GetResourceAnnotations copies the current ControllerConfig.Annotations into resourceMetadata
// and populates DeletedAnnotationKeys with any keys that were previously managed (stored in
// the CR's ManagedAnnotationsKey annotation) but are no longer in spec, so callers can remove
// them from child resources.
func GetResourceAnnotations(esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata *ResourceMetadata) error {
	resourceMetadata.Annotations = make(map[string]string)
	if esc.Spec.ControllerConfig.Annotations != nil {
		for k, v := range esc.Spec.ControllerConfig.Annotations {
			resourceMetadata.Annotations[k] = v
		}
	}
	previousAnnotations, err := GetPreviouslyAppliedAnnotationKeys(esc.GetAnnotations())
	if err != nil {
		return err
	}
	for _, k := range previousAnnotations {
		if _, ok := resourceMetadata.Annotations[k]; !ok {
			resourceMetadata.DeletedAnnotationKeys = append(resourceMetadata.DeletedAnnotationKeys, k)
		}
	}
	return nil
}

// AddManagedMetadataAnnotation updates the CR's ManagedAnnotationsKey annotation to the
// current list of ControllerConfig.Annotations keys (base64-encoded JSON). It reads the
// previously stored keys to detect changes, then writes the new keys and returns whether
// the CR needs to be updated (so the caller can call UpdateWithRetry at the end of reconcile).
func AddManagedMetadataAnnotation(esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata ResourceMetadata) (bool, error) {
	a := esc.GetAnnotations()
	if a == nil {
		a = make(map[string]string)
	}

	userAnnotations := esc.Spec.ControllerConfig.Annotations
	keys := make([]string, 0, len(userAnnotations))
	for k := range userAnnotations {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	existingAnnotationKeys, err := GetPreviouslyAppliedAnnotationKeys(a)
	if err != nil {
		return false, err
	}

	a[ManagedAnnotationsKey], err = EncodeDataToB64Json(keys)
	if err != nil {
		return false, err
	}
	esc.SetAnnotations(a)

	needsUpdate := len(existingAnnotationKeys) != len(userAnnotations) || len(resourceMetadata.DeletedAnnotationKeys) != 0
	return needsUpdate, nil
}

// GetPreviouslyAppliedAnnotationKeys returns the list of annotation keys stored in the CR's
// ManagedAnnotationsKey annotation (base64-encoded JSON) which were applied in previous
// reconciliation. Returns nil, nil when the annotation is missing or empty.
func GetPreviouslyAppliedAnnotationKeys(annotations map[string]string) ([]string, error) {
	if annotations == nil {
		return nil, nil
	}

	val, ok := annotations[ManagedAnnotationsKey]
	if !ok || val == "" {
		return nil, nil
	}
	data, err := DecodeDataFromB64Json([]byte(val))
	if err != nil {
		return nil, err
	}
	var keys []string
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// EncodeDataToB64Json marshals the given value to JSON and returns its base64-encoded string.
// Used for the ManagedAnnotationsKey annotation value (list of managed annotation keys).
func EncodeDataToB64Json(data any) (string, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(jsonBytes), nil
}

// DecodeDataFromB64Json decodes base64-encoded data and returns the raw bytes. Used to read the
// ManagedAnnotationsKey annotation value before JSON-unmarshaling the list of keys.
// The destination slice must be pre-allocated with at least DecodedLen(len(data)) bytes
// so base64.Decode can write into it; otherwise Decode writes nothing and returns empty.
func DecodeDataFromB64Json(data []byte) ([]byte, error) {
	decodedData := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
	n, err := base64.StdEncoding.Decode(decodedData, data)
	if err != nil {
		return nil, err
	}
	return decodedData[:n], nil
}
