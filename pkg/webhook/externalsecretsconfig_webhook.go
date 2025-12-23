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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	ctrlClient "github.com/openshift/external-secrets-operator/pkg/controller/client"
)

var (
	log = ctrl.Log.WithName("webhook").WithName("ExternalSecretsConfig")
)

// ExternalSecretsConfigValidator validates ExternalSecretsConfig resources
type ExternalSecretsConfigValidator struct {
	Client         ctrlClient.CtrlClient
	CacheReader    cache.Cache // Direct cache access for indexed queries
	CacheSyncCheck func(context.Context) bool
}

// isBitwardenBeingDisabled checks if the Bitwarden provider is being disabled.
func isBitwardenBeingDisabled(oldConfig, newConfig *operatorv1alpha1.ExternalSecretsConfig) bool {
	// Check if old config had Bitwarden enabled
	oldEnabled := oldConfig.Spec.Plugins.BitwardenSecretManagerProvider != nil &&
		oldConfig.Spec.Plugins.BitwardenSecretManagerProvider.Mode == operatorv1alpha1.Enabled

	// Check if new config has Bitwarden disabled
	newDisabled := newConfig.Spec.Plugins.BitwardenSecretManagerProvider == nil ||
		newConfig.Spec.Plugins.BitwardenSecretManagerProvider.Mode == operatorv1alpha1.Disabled

	return oldEnabled && newDisabled
}

// isBitwardenProviderInUse checks if any SecretStore or ClusterSecretStore is using the Bitwarden provider
// This method uses dynamic client to avoid importing external-secrets APIs
func (v *ExternalSecretsConfigValidator) isBitwardenProviderInUse(ctx context.Context) (bool, string, error) {
	// Use indexed implementation for optimal performance
	// Indexes are now set up correctly in cache builder with proper CRD name
	inUse, details, err := v.isBitwardenProviderInUseIndexed(ctx)
	if err != nil {
		// If indexed query fails, fall back to dynamic
		log.V(1).Info("indexed query failed, falling back to dynamic query", "error", err.Error())
		return v.isBitwardenProviderInUseDynamic(ctx)
	}
	return inUse, details, nil
}

// SetupWebhookWithManager sets up the webhook with the Manager
func (v *ExternalSecretsConfigValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&operatorv1alpha1.ExternalSecretsConfig{}).
		WithValidator(v).
		Complete()
}

// ValidateCreate implements webhook.Validator
func (v *ExternalSecretsConfigValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for CREATE operations
	return nil, nil
}

// ValidateUpdate implements webhook.Validator
func (v *ExternalSecretsConfigValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldConfig, ok := oldObj.(*operatorv1alpha1.ExternalSecretsConfig)
	if !ok {
		return nil, fmt.Errorf("expected ExternalSecretsConfig but got %T", oldObj)
	}

	newConfig, ok := newObj.(*operatorv1alpha1.ExternalSecretsConfig)
	if !ok {
		return nil, fmt.Errorf("expected ExternalSecretsConfig but got %T", newObj)
	}

	// Check if Bitwarden provider is being disabled
	if isBitwardenBeingDisabled(oldConfig, newConfig) {
		log.Info("detected attempt to disable Bitwarden provider, checking for existing stores")

		// Check if any SecretStore or ClusterSecretStore is using Bitwarden
		inUse, resourceDetails, err := v.isBitwardenProviderInUse(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to check if Bitwarden provider is in use: %w", err)
		}

		if inUse {
			return nil, fmt.Errorf(
				"cannot disable bitwardenSecretManagerProvider: it is currently being used by the following resources: %s. "+
					"Please remove or update these resources before disabling the provider",
				resourceDetails,
			)
		}
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator
func (v *ExternalSecretsConfigValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for DELETE operations
	return nil, nil
}
