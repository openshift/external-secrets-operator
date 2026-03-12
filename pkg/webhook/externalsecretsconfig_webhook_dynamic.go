/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// isBitwardenProviderInUseDynamic checks if any SecretStore or ClusterSecretStore is using the Bitwarden provider
// using dynamic client to avoid importing external-secrets APIs
func (v *ExternalSecretsConfigValidator) isBitwardenProviderInUseDynamic(ctx context.Context) (bool, string, error) {
	// Check if cache is synced (only relevant if using cached client)
	if v.CacheSyncCheck != nil && !v.CacheSyncCheck(ctx) {
		log.V(1).Info("cache not yet synced, returning temporary error")
		return false, "", fmt.Errorf("cache not synced yet, please retry")
	}

	var resourceDetails []string

	// Check SecretStores
	secretStoreList := &unstructured.UnstructuredList{}
	secretStoreList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "external-secrets.io",
		Version: "v1",
		Kind:    "SecretStoreList",
	})

	if err := v.Client.List(ctx, secretStoreList); err != nil {
		// If CRD doesn't exist or resource not found, ignore the error
		if !errors.IsNotFound(err) {
			return false, "", fmt.Errorf("failed to list SecretStores: %w", err)
		}
		log.V(2).Info("SecretStore CRD not found, skipping SecretStore check")
	} else {
		log.V(2).Info("listed SecretStores from cache", "count", len(secretStoreList.Items))
		for _, item := range secretStoreList.Items {
			if hasBitwardenProvider(&item) {
				namespace := item.GetNamespace()
				name := item.GetName()
				resourceDetails = append(resourceDetails,
					fmt.Sprintf("SecretStore '%s/%s'", namespace, name))
			}
		}
	}

	// Check ClusterSecretStores
	clusterSecretStoreList := &unstructured.UnstructuredList{}
	clusterSecretStoreList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "external-secrets.io",
		Version: "v1",
		Kind:    "ClusterSecretStoreList",
	})

	if err := v.Client.List(ctx, clusterSecretStoreList); err != nil {
		// If CRD doesn't exist or resource not found, ignore the error
		if !errors.IsNotFound(err) {
			return false, "", fmt.Errorf("failed to list ClusterSecretStores: %w", err)
		}
		log.V(2).Info("ClusterSecretStore CRD not found, skipping ClusterSecretStore check")
	} else {
		log.V(2).Info("listed ClusterSecretStores from cache", "count", len(clusterSecretStoreList.Items))
		for _, item := range clusterSecretStoreList.Items {
			if hasBitwardenProvider(&item) {
				name := item.GetName()
				resourceDetails = append(resourceDetails,
					fmt.Sprintf("ClusterSecretStore '%s'", name))
			}
		}
	}

	if len(resourceDetails) > 0 {
		return true, formatResourceList(resourceDetails), nil
	}

	return false, "", nil
}

// hasBitwardenProvider checks if an unstructured object has a Bitwarden provider configured
func hasBitwardenProvider(obj *unstructured.Unstructured) bool {
	// Navigate to spec.provider.bitwardensecretsmanager
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if !found || err != nil {
		return false
	}

	provider, found, err := unstructured.NestedMap(spec, "provider")
	if !found || err != nil {
		return false
	}

	// Check if bitwardensecretsmanager field exists
	_, found, err = unstructured.NestedMap(provider, "bitwardensecretsmanager")
	return found && err == nil
}

// formatResourceList formats the list of resources for display
func formatResourceList(resources []string) string {
	if len(resources) == 0 {
		return ""
	}
	if len(resources) == 1 {
		return resources[0]
	}
	if len(resources) <= 5 {
		result := ""
		for i, r := range resources {
			if i > 0 {
				result += ", "
			}
			result += r
		}
		return result
	}
	// Show first 5 and indicate there are more
	result := ""
	for i := 0; i < 5; i++ {
		if i > 0 {
			result += ", "
		}
		result += resources[i]
	}
	return fmt.Sprintf("%s, and %d more", result, len(resources)-5)
}
