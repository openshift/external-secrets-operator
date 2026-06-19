package webhook

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Index name for provider type field - used by controller's setupWebhookIndexes
	ProviderTypeIndexField = "spec.provider.type"

	// Provider type values
	ProviderTypeBitwarden = "bitwarden"
)

// IndexedListBitwardenSecretStores lists only SecretStores using BitWarden provider
// This is MUCH more efficient than listing all stores and filtering
// Note: Must use cache.Cache directly for indexed queries to work
func IndexedListBitwardenSecretStores(ctx context.Context, c client.Reader) (*unstructured.UnstructuredList, error) {
	secretStoreList := &unstructured.UnstructuredList{}
	secretStoreList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "external-secrets.io",
		Version: "v1",
		Kind:    "SecretStoreList",
	})

	// Use the index to only get BitWarden SecretStores
	if err := c.List(ctx, secretStoreList, client.MatchingFields{
		ProviderTypeIndexField: ProviderTypeBitwarden,
	}); err != nil {
		return nil, fmt.Errorf("failed to list BitWarden SecretStores: %w", err)
	}

	return secretStoreList, nil
}

// IndexedListBitwardenClusterSecretStores lists only ClusterSecretStores using BitWarden provider
// Note: Must use cache.Cache directly for indexed queries to work
func IndexedListBitwardenClusterSecretStores(ctx context.Context, c client.Reader) (*unstructured.UnstructuredList, error) {
	clusterSecretStoreList := &unstructured.UnstructuredList{}
	clusterSecretStoreList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "external-secrets.io",
		Version: "v1",
		Kind:    "ClusterSecretStoreList",
	})

	// Use the index to only get BitWarden ClusterSecretStores
	if err := c.List(ctx, clusterSecretStoreList, client.MatchingFields{
		ProviderTypeIndexField: ProviderTypeBitwarden,
	}); err != nil {
		return nil, fmt.Errorf("failed to list BitWarden ClusterSecretStores: %w", err)
	}

	return clusterSecretStoreList, nil
}

// isBitwardenProviderInUseIndexed checks using indexed cache (MUCH more efficient)
func (v *ExternalSecretsConfigValidator) isBitwardenProviderInUseIndexed(ctx context.Context) (bool, string, error) {
	log := log.WithName("isBitwardenProviderInUseIndexed")
	log.Info("🚀 Using indexed cache for BitWarden provider check")

	// Check if cache is synced
	if v.CacheSyncCheck != nil && !v.CacheSyncCheck(ctx) {
		log.V(1).Info("cache not yet synced, returning temporary error")
		return false, "", fmt.Errorf("cache not synced yet, please retry")
	}

	var resourceDetails []string

	// List only BitWarden SecretStores (indexed query)
	// Use v.CacheReader (cache) instead of v.Client for indexed queries to work!
	secretStoreList, err := IndexedListBitwardenSecretStores(ctx, v.CacheReader)
	if err != nil {
		// If CRD doesn't exist or resource not found, ignore the error
		if !errors.IsNotFound(err) {
			return false, "", fmt.Errorf("failed to list BitWarden SecretStores: %w", err)
		}
		log.V(2).Info("SecretStore CRD not found, skipping SecretStore check")
	} else {
		log.Info("✅ Indexed cache query succeeded for SecretStores", "bitwardenCount", len(secretStoreList.Items))

		// All items in this list are BitWarden stores (index guarantees this)
		for _, item := range secretStoreList.Items {
			namespace := item.GetNamespace()
			name := item.GetName()
			resourceDetails = append(resourceDetails,
				fmt.Sprintf("SecretStore '%s/%s'", namespace, name))
		}
	}

	// List only BitWarden ClusterSecretStores (indexed query)
	// Use v.CacheReader (cache) instead of v.Client for indexed queries to work!
	clusterSecretStoreList, err := IndexedListBitwardenClusterSecretStores(ctx, v.CacheReader)
	if err != nil {
		// If CRD doesn't exist or resource not found, ignore the error
		if !errors.IsNotFound(err) {
			return false, "", fmt.Errorf("failed to list BitWarden ClusterSecretStores: %w", err)
		}
		log.V(2).Info("ClusterSecretStore CRD not found, skipping ClusterSecretStore check")
	} else {
		log.Info("✅ Indexed cache query succeeded for ClusterSecretStores", "bitwardenCount", len(clusterSecretStoreList.Items))

		// All items in this list are BitWarden stores (index guarantees this)
		for _, item := range clusterSecretStoreList.Items {
			name := item.GetName()
			resourceDetails = append(resourceDetails,
				fmt.Sprintf("ClusterSecretStore '%s'", name))
		}
	}

	if len(resourceDetails) > 0 {
		return true, fmt.Sprintf("%d resource(s): %v", len(resourceDetails), resourceDetails), nil
	}

	return false, "", nil
}
