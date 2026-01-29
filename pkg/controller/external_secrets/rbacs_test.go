package external_secrets

import (
	"context"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

func TestCreateOrApplyRBACResource(t *testing.T) {
	tests := []struct {
		name                        string
		preReq                      func(*Reconciler, *fakes.FakeCtrlClient)
		updateExternalSecretsConfig func(*operatorv1alpha1.ExternalSecretsConfig)
		wantErr                     string
	}{
		{
			name: "clusterrole reconciliation fails while checking if exists",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch obj.(type) {
					case *rbacv1.ClusterRole:
						return false, commontest.TestClientError
					}
					return true, nil
				})
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec = operatorv1alpha1.ExternalSecretsConfigSpec{
					ControllerConfig: operatorv1alpha1.ControllerConfig{},
				}
			},
			wantErr: `failed to check external-secrets-controller clusterrole resource already exists: test client error`,
		},
		{
			name: "clusterrolebindings reconciliation fails while checking if exists",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch obj.(type) {
					case *rbacv1.ClusterRoleBinding:
						return false, commontest.TestClientError
					}
					return true, nil
				})
			},
			wantErr: `failed to check external-secrets-controller clusterrolebinding resource already exists: test client error`,
		},
		{
			name: "role reconciliation fails while checking if exists",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch obj.(type) {
					case *rbacv1.Role:
						return false, commontest.TestClientError
					}
					return true, nil
				})
			},
			wantErr: `failed to check external-secrets/external-secrets-leaderelection role resource already exists: test client error`,
		},
		{
			name: "rolebindings reconciliation fails while checking if exists",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch obj.(type) {
					case *rbacv1.RoleBinding:
						return false, commontest.TestClientError
					}
					return true, nil
				})
			},
			wantErr: `failed to check external-secrets/external-secrets-leaderelection rolebinding resource already exists: test client error`,
		},
		{
			name: "clusterrolebindings reconciliation updating to desired state fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, object client.Object) (bool, error) {
					switch o := object.(type) {
					case *rbacv1.ClusterRoleBinding:
						clusterRoleBinding := testClusterRoleBinding(controllerClusterRoleBindingAssetName)
						clusterRoleBinding.Labels = nil
						clusterRoleBinding.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					switch obj.(type) {
					case *rbacv1.ClusterRoleBinding:
						return commontest.TestClientError
					}
					return nil
				})
			},
			wantErr: `failed to update external-secrets-controller clusterrolebinding resource: test client error`,
		},
		{
			name: "clusterrolebindings reconciliation updating to desired state successful",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, object client.Object) (bool, error) {
					switch o := object.(type) {
					case *rbacv1.ClusterRoleBinding:
						clusterRoleBinding := testClusterRoleBinding(controllerClusterRoleBindingAssetName)
						clusterRoleBinding.Labels = nil
						clusterRoleBinding.DeepCopyInto(o)
					}
					return true, nil
				})
			},
		},
		{
			name: "cert-controller clusterrolebindings reconciliation creation fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, object client.Object) (bool, error) {
					switch object.(type) {
					case *rbacv1.ClusterRoleBinding:
						return false, nil
					}
					return true, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					switch obj.(type) {
					case *rbacv1.ClusterRoleBinding:
						if obj.GetName() == testClusterRoleBinding(certControllerClusterRoleBindingAssetName).GetName() {
							return commontest.TestClientError
						}
					}
					return nil
				})
			},
			wantErr: `failed to create external-secrets-cert-controller clusterrolebinding resource: test client error`,
		},
		{
			name: "clusterrole reconciliation updating to desired state fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, object client.Object) (bool, error) {
					switch o := object.(type) {
					case *rbacv1.ClusterRole:
						clusterRole := testClusterRole(controllerClusterRoleAssetName)
						clusterRole.Labels = nil
						clusterRole.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					switch obj.(type) {
					case *rbacv1.ClusterRole:
						return commontest.TestClientError
					}
					return nil
				})
			},
			wantErr: `failed to update external-secrets-controller clusterrole resource: test client error`,
		},
		{
			name: "cert-controller clusterrole reconciliation creation fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, object client.Object) (bool, error) {
					switch object.(type) {
					case *rbacv1.ClusterRole:
						return false, nil
					}
					return true, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					switch obj.(type) {
					case *rbacv1.ClusterRole:
						if obj.GetName() == testClusterRoleBinding(certControllerClusterRoleBindingAssetName).GetName() {
							return commontest.TestClientError
						}
					}
					return nil
				})
			},
			wantErr: `failed to create external-secrets-cert-controller clusterrole resource: test client error`,
		},
		{
			name: "role reconciliation updating to desired state fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, object client.Object) (bool, error) {
					switch o := object.(type) {
					case *rbacv1.Role:
						role := testRole(controllerRoleLeaderElectionAssetName)
						role.Labels = nil
						role.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					switch obj.(type) {
					case *rbacv1.Role:
						return commontest.TestClientError
					}
					return nil
				})
			},
			wantErr: `failed to update external-secrets/external-secrets-leaderelection role resource: test client error`,
		},
		{
			name: "role reconciliation creation fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, object client.Object) (bool, error) {
					switch object.(type) {
					case *rbacv1.Role:
						return false, nil
					}
					return true, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					switch obj.(type) {
					case *rbacv1.Role:
						return commontest.TestClientError
					}
					return nil
				})
			},
			wantErr: `failed to create external-secrets/external-secrets-leaderelection role resource: test client error`,
		},
		{
			name: "rolebindings reconciliation updating to desired state fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, object client.Object) (bool, error) {
					switch o := object.(type) {
					case *rbacv1.RoleBinding:
						role := testRoleBinding(controllerRoleBindingLeaderElectionAssetName)
						role.Labels = nil
						role.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					switch obj.(type) {
					case *rbacv1.RoleBinding:
						return commontest.TestClientError
					}
					return nil
				})
			},
			wantErr: `failed to update external-secrets/external-secrets-leaderelection rolebinding resource: test client error`,
		},
		{
			name: "rolebindings reconciliation creation fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, object client.Object) (bool, error) {
					switch object.(type) {
					case *rbacv1.RoleBinding:
						return false, nil
					}
					return true, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					switch obj.(type) {
					case *rbacv1.RoleBinding:
						return commontest.TestClientError
					}
					return nil
				})
			},
			wantErr: `failed to create external-secrets/external-secrets-leaderelection rolebinding resource: test client error`,
		},
		{
			name: "clusterroles creation successful",
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
			name: "clusterrolebindings creation successful",
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
			name: "roles creation successful",
		},
		{
			name: "rolebindings creation successful",
		},
		{
			name: "clusterrole with custom annotations applied successfully",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if cr, ok := obj.(*rbacv1.ClusterRole); ok {
						// Verify annotations are applied
						if cr.Annotations != nil {
							if cr.Annotations["rbac/managed-by"] != "operator" {
								t.Errorf("expected annotation 'rbac/managed-by'='operator', got '%s'",
									cr.Annotations["rbac/managed-by"])
							}
						}
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.Annotations = map[string]string{
					"rbac/managed-by": "operator",
					"team/owner":      "platform",
				}
			},
		},
		{
			name: "clusterrolebinding with custom annotations applied successfully",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if crb, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
						// Verify annotations are applied
						if crb.Annotations != nil {
							if crb.Annotations["binding/type"] != "cluster" {
								t.Errorf("expected annotation 'binding/type'='cluster'")
							}
						}
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.Annotations = map[string]string{
					"binding/type": "cluster",
				}
			},
		},
		{
			name: "role with custom annotations applied successfully",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if role, ok := obj.(*rbacv1.Role); ok {
						// Verify annotations are applied
						if role.Annotations != nil {
							if role.Annotations["role/purpose"] != "leader-election" {
								t.Errorf("expected annotation 'role/purpose'='leader-election'")
							}
						}
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.Annotations = map[string]string{
					"role/purpose": "leader-election",
				}
			},
		},
		{
			name: "rolebinding with custom annotations applied successfully",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if rb, ok := obj.(*rbacv1.RoleBinding); ok {
						// Verify annotations are applied
						if rb.Annotations != nil {
							if rb.Annotations["binding/scope"] != "namespace" {
								t.Errorf("expected annotation 'binding/scope'='namespace'")
							}
						}
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.Annotations = map[string]string{
					"binding/scope": "namespace",
				}
			},
		},
		{
			name: "rbac resources filter reserved annotation prefixes",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					switch obj := obj.(type) {
					case *rbacv1.ClusterRole, *rbacv1.ClusterRoleBinding, *rbacv1.Role, *rbacv1.RoleBinding:
						// Verify reserved prefixes were filtered
						metaObj := obj.(client.Object)
						annotations := metaObj.GetAnnotations()
						if _, exists := annotations["openshift.io/test"]; exists {
							t.Error("reserved prefix 'openshift.io/' should have been filtered")
						}
						if annotations["allowed-rbac"] != "value" {
							t.Errorf("expected 'allowed-rbac' annotation")
						}
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.Annotations = map[string]string{
					"allowed-rbac":      "value",
					"openshift.io/test": "filtered",
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)

			mock := &fakes.FakeCtrlClient{}
			if tt.preReq != nil {
				tt.preReq(r, mock)
			}
			r.CtrlClient = mock

			esc := commontest.TestExternalSecretsConfig()
			if tt.updateExternalSecretsConfig != nil {
				tt.updateExternalSecretsConfig(esc)
			}

			err := r.createOrApplyRBACResource(esc, controllerDefaultResourceLabels, true)
			if (tt.wantErr != "" || err != nil) && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("createOrApplyRBACResource() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}
