package external_secrets

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

// ensureTrustedCABundleConfigMap creates or ensures the trusted CA bundle ConfigMap exists
// in the operand namespace when proxy configuration is present. The ConfigMap is labeled
// with the injection label required by the Cluster Network Operator (CNO), which watches
// for this label and injects the cluster's trusted CA bundle into the ConfigMap's data.
// This function ensures the correct labels are present so that CNO can manage the CA bundle
// content as expected.
func (r *Reconciler) ensureTrustedCABundleConfigMap(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string) error {
	proxyConfig := r.getProxyConfiguration(esc)

	// Only create ConfigMap if proxy is configured
	if proxyConfig == nil {
		return nil
	}

	namespace := getNamespace(esc)
	expectedLabels := getTrustedCABundleLabels(resourceLabels)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trustedCABundleConfigMapName,
			Namespace: namespace,
			Labels:    expectedLabels,
		},
	}

	// Check if the ConfigMap already exists
	existingConfigMap := &corev1.ConfigMap{}
	exist, err := r.Exists(r.ctx, client.ObjectKeyFromObject(configMap), existingConfigMap)
	if err != nil {
		return fmt.Errorf("failed to check if trusted CA bundle ConfigMap exists: %w", err)
	}

	if !exist {
		// Create the ConfigMap
		if err := r.Create(r.ctx, configMap); err != nil {
			return fmt.Errorf("failed to create trusted CA bundle ConfigMap: %w", err)
		}
		return nil
	}

	// ConfigMap exists, ensure it has the correct labels
	// Do not update the data of the ConfigMap since it is managed by CNO
	if existingConfigMap.Labels == nil {
		existingConfigMap.Labels = make(map[string]string)
	}

	expectedLabels = getTrustedCABundleLabels(resourceLabels)
	needsUpdate := false
	for k, expectedValue := range expectedLabels {
		if existingValue, exists := existingConfigMap.Labels[k]; !exists || existingValue != expectedValue {
			existingConfigMap.Labels[k] = expectedValue
			needsUpdate = true
		}
	}

	// Update the ConfigMap if any labels changed
	if needsUpdate {
		if err := r.Update(r.ctx, existingConfigMap); err != nil {
			return fmt.Errorf("failed to update trusted CA bundle ConfigMap labels: %w", err)
		}
	}

	return nil
}

// getTrustedCABundleLabels merges resource labels with the injection label
func getTrustedCABundleLabels(resourceLabels map[string]string) map[string]string {
	labels := make(map[string]string)
	for k, v := range resourceLabels {
		labels[k] = v
	}
	labels[trustedCABundleInjectLabel] = "true"
	return labels
}
