package external_secrets

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

func TestValidateExternalSecretsConfig(t *testing.T) {
	tests := []struct {
		name                        string
		setupReconciler             func(*Reconciler)
		updateExternalSecretsConfig func(*operatorv1alpha1.ExternalSecretsConfig)
		wantErr                     string
	}{
		{
			name: "valid config with no network policies",
		},
		{
			name: "valid config with CoreController network policy",
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.NetworkPolicies = []operatorv1alpha1.NetworkPolicy{
					{
						Name:          "allow-egress",
						ComponentName: operatorv1alpha1.CoreController,
						Egress:        []networkingv1.NetworkPolicyEgressRule{{}},
					},
				}
			},
		},
		{
			name: "valid config with BitwardenSDKServer network policy and bitwarden enabled",
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.Plugins.BitwardenSecretManagerProvider = &operatorv1alpha1.BitwardenSecretManagerProvider{
					Mode: operatorv1alpha1.Enabled,
				}
				esc.Spec.ControllerConfig.NetworkPolicies = []operatorv1alpha1.NetworkPolicy{
					{
						Name:          "allow-bitwarden-egress",
						ComponentName: operatorv1alpha1.BitwardenSDKServer,
						Egress:        []networkingv1.NetworkPolicyEgressRule{{}},
					},
				}
			},
		},
		{
			name: "warn when BitwardenSDKServer network policy configured but bitwarden disabled",
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.NetworkPolicies = []operatorv1alpha1.NetworkPolicy{
					{
						Name:          "allow-bitwarden-egress",
						ComponentName: operatorv1alpha1.BitwardenSDKServer,
						Egress:        []networkingv1.NetworkPolicyEgressRule{{}},
					},
				}
			},
			// No error expected - this is a warning only
		},
		{
			name: "cert-manager mode enabled but not installed",
			setupReconciler: func(r *Reconciler) {
				// optionalResourcesList does NOT contain certificateCRDGKV
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider = &operatorv1alpha1.CertProvidersConfig{
					CertManager: &operatorv1alpha1.CertManagerConfig{
						Mode: operatorv1alpha1.Enabled,
					},
				}
			},
			wantErr: "spec.controllerConfig.certProvider.certManager.mode is set, but cert-manager is not installed",
		},
		{
			name: "cert-manager mode enabled and installed",
			setupReconciler: func(r *Reconciler) {
				r.optionalResourcesList[certificateCRDGKV] = struct{}{}
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider = &operatorv1alpha1.CertProvidersConfig{
					CertManager: &operatorv1alpha1.CertManagerConfig{
						Mode: operatorv1alpha1.Enabled,
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)
			if tt.setupReconciler != nil {
				tt.setupReconciler(r)
			}

			esc := commontest.TestExternalSecretsConfig()
			if tt.updateExternalSecretsConfig != nil {
				tt.updateExternalSecretsConfig(esc)
			}

			err := r.validateExternalSecretsConfig(esc)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Errorf("Expected error: %v, got: %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
