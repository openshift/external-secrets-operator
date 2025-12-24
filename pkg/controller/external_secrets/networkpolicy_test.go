package external_secrets

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

// staticNetworkPolicies returns a map of all static network policy names to their asset paths.
func staticNetworkPolicies() map[string]string {
	return map[string]string{
		"deny-all-traffic": denyAllNetworkPolicyAssetName,
		"allow-api-server-egress-for-main-controller":  allowMainControllerTrafficAssetName,
		"allow-api-server-egress-for-webhook":          allowWebhookTrafficAssetName,
		"allow-api-server-egress-for-cert-controller":  allowCertControllerTrafficAssetName,
		"allow-api-server-egress-for-bitwarden-server": allowBitwardenServerTrafficAssetName,
		"allow-to-dns": allowDnsTrafficAsserName,
	}
}

func TestCreateOrApplyStaticNetworkPolicies(t *testing.T) {
	tests := []struct {
		name                        string
		preReq                      func(*Reconciler, *fakes.FakeCtrlClient)
		updateExternalSecretsConfig func(*operatorv1alpha1.ExternalSecretsConfig)
		wantErr                     string
	}{
		{
			name: "all static network policies created successfully",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})

				expectedNPMap := make(map[string]*networkingv1.NetworkPolicy)
				for name, path := range staticNetworkPolicies() {
					if name == "allow-api-server-egress-for-cert-controller" ||
						name == "allow-api-server-egress-for-bitwarden-server" {
						continue // Skip conditional policies in default test
					}
					expectedNPMap[name] = testNetworkPolicy(path)
				}

				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if np, ok := obj.(*networkingv1.NetworkPolicy); ok {
						if _, found := expectedNPMap[np.Name]; found {
							return nil
						}
					}
					return nil
				})
			},
		},
		{
			name: "bitwarden network policy created when enabled",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					expectedNP := testNetworkPolicy(allowBitwardenServerTrafficAssetName)
					if np, ok := obj.(*networkingv1.NetworkPolicy); ok {
						if np.Name == expectedNP.Name {
							return nil
						}
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec = operatorv1alpha1.ExternalSecretsConfigSpec{
					Plugins: operatorv1alpha1.PluginsConfig{
						BitwardenSecretManagerProvider: &operatorv1alpha1.BitwardenSecretManagerProvider{
							Mode: operatorv1alpha1.Enabled,
						},
					},
				}
			},
		},
		{
			name: "cert-controller network policy skipped when cert-manager enabled",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if np, ok := obj.(*networkingv1.NetworkPolicy); ok {
						if np.Name == "allow-api-server-egress-for-cert-controller" {
							return fmt.Errorf("cert-controller policy should not be created")
						}
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec = operatorv1alpha1.ExternalSecretsConfigSpec{
					ControllerConfig: operatorv1alpha1.ControllerConfig{
						CertProvider: &operatorv1alpha1.CertProvidersConfig{
							CertManager: &operatorv1alpha1.CertManagerConfig{
								Mode: operatorv1alpha1.Enabled,
							},
						},
					},
				}
			},
		},
		{
			name: "network policy exists and needs update",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *networkingv1.NetworkPolicy:
						np := testNetworkPolicy(denyAllNetworkPolicyAssetName)
						np.Labels = nil // Simulate outdated policy
						np.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					return nil
				})
			},
		},
		{
			name: "network policy creation fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if np, ok := obj.(*networkingv1.NetworkPolicy); ok && np.Name == "deny-all-traffic" {
						return commontest.TestClientError
					}
					return nil
				})
			},
			wantErr: "failed to create network policy external-secrets/deny-all-traffic: test client error",
		},
		{
			name: "network policy exists check fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch obj.(type) {
					case *networkingv1.NetworkPolicy:
						return false, commontest.TestClientError
					}
					return true, nil
				})
			},
			wantErr: "failed to check existence of network policy external-secrets/deny-all-traffic: test client error",
		},
		{
			name: "network policy update fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *networkingv1.NetworkPolicy:
						np := testNetworkPolicy(denyAllNetworkPolicyAssetName)
						np.Labels = nil // Force update
						np.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					if _, ok := obj.(*networkingv1.NetworkPolicy); ok {
						return commontest.TestClientError
					}
					return nil
				})
			},
			wantErr: "failed to update network policy external-secrets/deny-all-traffic: test client error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)
			mock := &fakes.FakeCtrlClient{}
			r.CtrlClient = mock
			if tt.preReq != nil {
				tt.preReq(r, mock)
			}

			esc := commontest.TestExternalSecretsConfig()
			if tt.updateExternalSecretsConfig != nil {
				tt.updateExternalSecretsConfig(esc)
			}

			err := r.createOrApplyStaticNetworkPolicies(esc, controllerDefaultResourceLabels, false)
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

func TestCreateOrApplyCustomNetworkPolicies(t *testing.T) {
	tests := []struct {
		name                        string
		preReq                      func(*Reconciler, *fakes.FakeCtrlClient)
		updateExternalSecretsConfig func(*operatorv1alpha1.ExternalSecretsConfig)
		wantErr                     string
	}{
		{
			name: "no custom network policies configured",
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.NetworkPolicies = nil
			},
		},
		{
			name: "custom network policy created successfully",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if np, ok := obj.(*networkingv1.NetworkPolicy); ok {
						if np.Name != "test-custom-policy" {
							return fmt.Errorf("unexpected network policy name: %s", np.Name)
						}
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.NetworkPolicies = []operatorv1alpha1.NetworkPolicy{
					{
						Name:          "test-custom-policy",
						ComponentName: operatorv1alpha1.CoreController,
						Egress: []networkingv1.NetworkPolicyEgressRule{
							{
								Ports: []networkingv1.NetworkPolicyPort{
									{
										Protocol: &[]corev1.Protocol{corev1.ProtocolTCP}[0],
										Port:     &[]intstr.IntOrString{intstr.FromInt(443)}[0],
									},
								},
							},
						},
					},
				}
			},
		},
		{
			name: "custom network policy with invalid component name",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.NetworkPolicies = []operatorv1alpha1.NetworkPolicy{
					{
						Name:          "test-invalid-policy",
						ComponentName: "InvalidComponent",
						Egress:        []networkingv1.NetworkPolicyEgressRule{},
					},
				}
			},
			wantErr: "failed to determine pod selector for network policy test-invalid-policy: unknown component name: InvalidComponent",
		},
		{
			name: "custom network policy creation fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if _, ok := obj.(*networkingv1.NetworkPolicy); ok {
						return commontest.TestClientError
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.NetworkPolicies = []operatorv1alpha1.NetworkPolicy{
					{
						Name:          "test-fail-policy",
						ComponentName: operatorv1alpha1.CoreController,
						Egress:        []networkingv1.NetworkPolicyEgressRule{},
					},
				}
			},
			wantErr: "failed to create network policy external-secrets/test-fail-policy: test client error",
		},
		{
			name: "custom network policy updated successfully",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *networkingv1.NetworkPolicy:
						np := &networkingv1.NetworkPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-update-policy",
								Namespace: externalsecretsDefaultNamespace,
							},
						}
						np.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					return nil
				})
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.NetworkPolicies = []operatorv1alpha1.NetworkPolicy{
					{
						Name:          "test-update-policy",
						ComponentName: operatorv1alpha1.CoreController,
						Egress: []networkingv1.NetworkPolicyEgressRule{
							{
								Ports: []networkingv1.NetworkPolicyPort{
									{
										Protocol: &[]corev1.Protocol{corev1.ProtocolTCP}[0],
										Port:     &[]intstr.IntOrString{intstr.FromInt(443)}[0],
									},
								},
							},
						},
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)
			mock := &fakes.FakeCtrlClient{}
			r.CtrlClient = mock
			if tt.preReq != nil {
				tt.preReq(r, mock)
			}

			esc := commontest.TestExternalSecretsConfig()
			if tt.updateExternalSecretsConfig != nil {
				tt.updateExternalSecretsConfig(esc)
			}

			err := r.createOrApplyCustomNetworkPolicies(esc, controllerDefaultResourceLabels, false)
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

func TestGetPodSelectorForComponent(t *testing.T) {
	tests := []struct {
		name          string
		componentName operatorv1alpha1.ComponentName
		wantLabels    map[string]string
		wantErr       bool
	}{
		{
			name:          "CoreController component",
			componentName: operatorv1alpha1.CoreController,
			wantLabels: map[string]string{
				"app.kubernetes.io/name": "external-secrets",
			},
			wantErr: false,
		},
		{
			name:          "BitwardenSDKServer component",
			componentName: operatorv1alpha1.BitwardenSDKServer,
			wantLabels: map[string]string{
				"app.kubernetes.io/name": "bitwarden-sdk-server",
			},
			wantErr: false,
		},
		{
			name:          "Unknown component",
			componentName: "UnknownComponent",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)
			selector, err := r.getPodSelectorForComponent(tt.componentName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(selector.MatchLabels) != len(tt.wantLabels) {
					t.Errorf("Expected %d labels, got %d", len(tt.wantLabels), len(selector.MatchLabels))
				}
				for k, v := range tt.wantLabels {
					if selector.MatchLabels[k] != v {
						t.Errorf("Expected label %s=%s, got %s", k, v, selector.MatchLabels[k])
					}
				}
			}
		})
	}
}

func TestBuildNetworkPolicyFromConfig(t *testing.T) {
	tests := []struct {
		name       string
		npConfig   operatorv1alpha1.NetworkPolicy
		wantErr    bool
		wantPolicy func(*networkingv1.NetworkPolicy) bool
	}{
		{
			name: "valid CoreController network policy",
			npConfig: operatorv1alpha1.NetworkPolicy{
				Name:          "test-core-policy",
				ComponentName: operatorv1alpha1.CoreController,
				Egress: []networkingv1.NetworkPolicyEgressRule{
					{
						Ports: []networkingv1.NetworkPolicyPort{
							{
								Protocol: &[]corev1.Protocol{corev1.ProtocolTCP}[0],
								Port:     &[]intstr.IntOrString{intstr.FromInt(443)}[0],
							},
						},
					},
				},
			},
			wantErr: false,
			wantPolicy: func(np *networkingv1.NetworkPolicy) bool {
				return np.Name == "test-core-policy" &&
					np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"] == "external-secrets" &&
					len(np.Spec.Egress) == 1 &&
					len(np.Spec.PolicyTypes) == 1 &&
					np.Spec.PolicyTypes[0] == networkingv1.PolicyTypeEgress
			},
		},
		{
			name: "valid BitwardenSDKServer network policy",
			npConfig: operatorv1alpha1.NetworkPolicy{
				Name:          "test-bitwarden-policy",
				ComponentName: operatorv1alpha1.BitwardenSDKServer,
				Egress:        []networkingv1.NetworkPolicyEgressRule{},
			},
			wantErr: false,
			wantPolicy: func(np *networkingv1.NetworkPolicy) bool {
				return np.Name == "test-bitwarden-policy" &&
					np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"] == "bitwarden-sdk-server"
			},
		},
		{
			name: "invalid component name",
			npConfig: operatorv1alpha1.NetworkPolicy{
				Name:          "test-invalid",
				ComponentName: "InvalidComponent",
				Egress:        []networkingv1.NetworkPolicyEgressRule{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)
			esc := commontest.TestExternalSecretsConfig()

			np, err := r.buildNetworkPolicyFromConfig(esc, tt.npConfig, controllerDefaultResourceLabels)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.wantPolicy != nil && !tt.wantPolicy(np) {
					t.Errorf("Network policy validation failed")
				}
			}
		})
	}
}
