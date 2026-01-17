package external_secrets

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

// createOrApplyNetworkPolicies handles creation of both static network policies from manifests
// and custom network policies configured in the ExternalSecretsConfig API.
func (r *Reconciler) createOrApplyNetworkPolicies(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string, externalSecretsConfigCreateRecon bool) error {
	// First, apply the static deny-all network policy
	if err := r.createOrApplyStaticNetworkPolicies(esc, resourceLabels, externalSecretsConfigCreateRecon); err != nil {
		return err
	}

	// Then, apply custom network policies from the API spec
	if err := r.createOrApplyCustomNetworkPolicies(esc, resourceLabels, externalSecretsConfigCreateRecon); err != nil {
		return err
	}

	return nil
}

// createOrApplyStaticNetworkPolicies applies the static network policy manifests from bindata.
func (r *Reconciler) createOrApplyStaticNetworkPolicies(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string, externalSecretsConfigCreateRecon bool) error {
	// Define static network policy assets to apply
	staticNetworkPolicies := []struct {
		assetName string
		condition bool
	}{
		{
			assetName: denyAllNetworkPolicyAssetName,
			condition: true, // Always apply deny-all as the base policy
		},
		{
			assetName: allowMainControllerTrafficAssetName,
			condition: true, // Always apply for main controller
		},
		{
			assetName: allowWebhookTrafficAssetName,
			condition: true, // Always apply for webhook
		},
		{
			assetName: allowCertControllerTrafficAssetName,
			condition: !isCertManagerConfigEnabled(esc), // Only if cert-controller is enabled
		},
		{
			assetName: allowBitwardenServerTrafficAssetName,
			condition: isBitwardenConfigEnabled(esc), // Only if bitwarden is enabled
		},
		{
			assetName: allowDnsTrafficAsserName,
			condition: true,
		},
	}

	// Apply static network policies based on conditions
	for _, np := range staticNetworkPolicies {
		if !np.condition {
			continue
		}
		if err := r.createOrApplyNetworkPolicyFromAsset(esc, np.assetName, resourceLabels, externalSecretsConfigCreateRecon); err != nil {
			return err
		}
	}

	return nil
}

// createOrApplyCustomNetworkPolicies applies custom network policies defined in the ExternalSecretsConfig spec.
func (r *Reconciler) createOrApplyCustomNetworkPolicies(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string, externalSecretsConfigCreateRecon bool) error {
	if esc.Spec.ControllerConfig.NetworkPolicies == nil {
		r.log.V(4).Info("No custom network policies configured in ControllerConfig")
		return nil
	}

	for _, npConfig := range esc.Spec.ControllerConfig.NetworkPolicies {
		if err := r.createOrApplyCustomNetworkPolicy(esc, npConfig, resourceLabels, externalSecretsConfigCreateRecon); err != nil {
			return err
		}
	}

	return nil
}

// createOrApplyCustomNetworkPolicy creates or updates a custom network policy based on API configuration.
func (r *Reconciler) createOrApplyCustomNetworkPolicy(esc *operatorv1alpha1.ExternalSecretsConfig, npConfig operatorv1alpha1.NetworkPolicy, resourceLabels map[string]string, externalSecretsConfigCreateRecon bool) error {
	// Build the NetworkPolicy object from the API spec
	networkPolicy, err := r.buildNetworkPolicyFromConfig(esc, npConfig, resourceLabels)
	if err != nil {
		return err
	}

	networkPolicyName := fmt.Sprintf("%s/%s", networkPolicy.GetNamespace(), networkPolicy.GetName())
	r.log.V(4).Info("Reconciling custom network policy", "name", networkPolicyName, "component", npConfig.ComponentName)

	fetched := &networkingv1.NetworkPolicy{}
	exists, err := r.Exists(r.ctx, client.ObjectKeyFromObject(networkPolicy), fetched)
	if err != nil {
		return common.FromClientError(err, "failed to check existence of network policy %s", networkPolicyName)
	}

	if exists && externalSecretsConfigCreateRecon {
		r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "NetworkPolicy %s already exists", networkPolicyName)
	}

	switch {
	case exists && common.HasObjectChanged(networkPolicy, fetched):
		r.log.V(1).Info("NetworkPolicy modified, updating", "name", networkPolicyName)
		if err := r.UpdateWithRetry(r.ctx, networkPolicy); err != nil {
			return common.FromClientError(err, "failed to update network policy %s", networkPolicyName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "NetworkPolicy %s updated", networkPolicyName)
	case !exists:
		if err := r.Create(r.ctx, networkPolicy); err != nil {
			return common.FromClientError(err, "failed to create network policy %s", networkPolicyName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "NetworkPolicy %s created", networkPolicyName)
	default:
		r.log.V(4).Info("NetworkPolicy already up-to-date", "name", networkPolicyName)
	}

	return nil
}

// createOrApplyNetworkPolicyFromAsset decodes a NetworkPolicy YAML asset and ensures it exists in the cluster.
func (r *Reconciler) createOrApplyNetworkPolicyFromAsset(esc *operatorv1alpha1.ExternalSecretsConfig, assetName string, resourceLabels map[string]string, externalSecretsConfigCreateRecon bool) error {
	networkPolicy := common.DecodeNetworkPolicyObjBytes(assets.MustAsset(assetName))
	updateNamespace(networkPolicy, esc)
	common.UpdateResourceLabels(networkPolicy, resourceLabels)

	networkPolicyName := fmt.Sprintf("%s/%s", networkPolicy.GetNamespace(), networkPolicy.GetName())
	r.log.V(4).Info("Reconciling static network policy", "name", networkPolicyName)

	fetched := &networkingv1.NetworkPolicy{}
	exists, err := r.Exists(r.ctx, client.ObjectKeyFromObject(networkPolicy), fetched)
	if err != nil {
		return common.FromClientError(err, "failed to check existence of network policy %s", networkPolicyName)
	}

	if exists && externalSecretsConfigCreateRecon {
		r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "NetworkPolicy %s already exists", networkPolicyName)
	}

	switch {
	case exists && common.HasObjectChanged(networkPolicy, fetched):
		r.log.V(1).Info("NetworkPolicy modified, updating", "name", networkPolicyName)
		if err := r.UpdateWithRetry(r.ctx, networkPolicy); err != nil {
			return common.FromClientError(err, "failed to update network policy %s", networkPolicyName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "NetworkPolicy %s updated", networkPolicyName)
	case !exists:
		if err := r.Create(r.ctx, networkPolicy); err != nil {
			return common.FromClientError(err, "failed to create network policy %s", networkPolicyName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "NetworkPolicy %s created", networkPolicyName)
	default:
		r.log.V(4).Info("NetworkPolicy already up-to-date", "name", networkPolicyName)
	}

	return nil
}

// buildNetworkPolicyFromConfig constructs a NetworkPolicy object from the API configuration.
func (r *Reconciler) buildNetworkPolicyFromConfig(esc *operatorv1alpha1.ExternalSecretsConfig, npConfig operatorv1alpha1.NetworkPolicy, resourceLabels map[string]string) (*networkingv1.NetworkPolicy, error) {
	namespace := getNamespace(esc)

	// Determine pod selector based on component name
	podSelector, err := r.getPodSelectorForComponent(npConfig.ComponentName)
	if err != nil {
		return nil, fmt.Errorf("failed to determine pod selector for network policy %s: %w", npConfig.Name, err)
	}

	// Build the NetworkPolicy object
	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      npConfig.Name,
			Namespace: namespace,
			Labels:    resourceLabels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: podSelector,
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
			Egress: npConfig.Egress,
		},
	}

	return networkPolicy, nil
}

// getPodSelectorForComponent returns the appropriate pod selector for the given component.
func (r *Reconciler) getPodSelectorForComponent(componentName operatorv1alpha1.ComponentName) (metav1.LabelSelector, error) {
	switch componentName {
	case operatorv1alpha1.CoreController:
		return metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name": "external-secrets",
			},
		}, nil
	case operatorv1alpha1.BitwardenSDKServer:
		return metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name": "bitwarden-sdk-server",
			},
		}, nil
	default:
		return metav1.LabelSelector{}, fmt.Errorf("unknown component name: %s", componentName)
	}
}
