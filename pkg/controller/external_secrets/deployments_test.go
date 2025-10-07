package external_secrets

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

func TestCreateOrApplyDeployments(t *testing.T) {
	tests := []struct {
		name                        string
		preReq                      func(*Reconciler, *fakes.FakeCtrlClient)
		updateExternalSecretsConfig func(*v1alpha1.ExternalSecretsConfig)
		skipEnvVar                  bool
		wantErr                     string
	}{
		{
			name: "deployment reconciliation successful",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
			},
			updateExternalSecretsConfig: func(i *v1alpha1.ExternalSecretsConfig) {
				i.Status.ExternalSecretsImage = commontest.TestExternalSecretsImageName
			},
		},
		{
			name: "deployment reconciliation fails as image env var is empty",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
			},
			skipEnvVar: true,
			wantErr:    `failed to update image in external-secrets deployment object: RELATED_IMAGE_EXTERNAL_SECRETS environment variable with externalsecrets image not set`,
		},
		{
			name: "deployment reconciliation fails while checking if exists",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch obj.(type) {
					case *appsv1.Deployment:
						return false, commontest.TestClientError
					}
					return true, nil
				})
			},
			wantErr: `failed to check external-secrets/external-secrets deployment resource already exists: test client error`,
		},
		{
			name: "deployment reconciliation failed while restoring to desired state",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.Labels = nil
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) error {
					switch obj.(type) {
					case *appsv1.Deployment:
						return commontest.TestClientError
					}
					return nil
				})
			},
			wantErr: `failed to update external-secrets/external-secrets deployment resource: test client error`,
		},
		{
			name: "deployment reconciliation with user custom config successful",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
			},
			updateExternalSecretsConfig: func(i *v1alpha1.ExternalSecretsConfig) {
				i.Spec.ApplicationConfig.Affinity = &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "node",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"test"},
										},
									},
								},
							},
						},
					},
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "test",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"test"},
										},
									},
								},
								TopologyKey: "topology.kubernetes.io/zone",
							},
						},
					},
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
							{
								Weight: 100,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "test",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"test"},
											},
										},
									},
									TopologyKey: "topology.kubernetes.io/zone",
								},
							},
						},
					},
				}
				i.Spec.ApplicationConfig.Tolerations = []corev1.Toleration{
					{
						Key:      "type",
						Operator: corev1.TolerationOpEqual,
						Value:    "test",
						Effect:   corev1.TaintEffectNoSchedule,
					},
				}
				i.Spec.ApplicationConfig.NodeSelector = map[string]string{"type": "test"}
				i.Spec.ApplicationConfig.Resources = &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				}
			},
		},
		{
			name: "deployment reconciliation fails while updating image in externalsecrets status",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch obj.(type) {
					case *appsv1.Deployment:
						return false, nil
					}
					return true, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return nil
				})
				m.StatusUpdateCalls(func(ctx context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
					switch obj.(type) {
					case *v1alpha1.ExternalSecretsConfig:
						return commontest.TestClientError
					}
					return nil
				})
			},
			wantErr: `failed to update /cluster status with image info: failed to update externalsecretsconfigs.operator.openshift.io "/cluster" status: test client error`,
		},
		{
			name: "deployment reconciliation with invalid toleration configuration",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						deployment := testDeployment("external-secrets-controller")
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
			},
			updateExternalSecretsConfig: func(i *v1alpha1.ExternalSecretsConfig) {
				i.Spec.ApplicationConfig.Tolerations = []corev1.Toleration{
					{
						Operator: corev1.TolerationOpExists,
						Value:    "test",
						Effect:   corev1.TaintEffectNoSchedule,
					},
				}
			},
			wantErr: "failed to update pod tolerations: spec.tolerations.tolerations[0].operator: Invalid value: \"test\": value must be empty when `operator` is 'Exists'",
		},
		{
			name: "deployment reconciliation with invalid nodeSelector configuration",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
			},
			updateExternalSecretsConfig: func(i *v1alpha1.ExternalSecretsConfig) {
				i.Spec.ApplicationConfig.NodeSelector = map[string]string{"node/Label/2": "value2"}
			},
			wantErr: `failed to update node selector: spec.nodeSelector: Invalid value: "node/Label/2": a qualified name must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]') with an optional DNS subdomain prefix and '/' (e.g. 'example.com/MyName')`,
		},
		{
			name: "deployment reconciliation with invalid affinity configuration",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						deployment := testDeployment("external-secrets-controller")
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
			},
			updateExternalSecretsConfig: func(i *v1alpha1.ExternalSecretsConfig) {
				i.Spec.ApplicationConfig.Affinity = &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "node",
											Operator: corev1.NodeSelectorOpIn,
										},
									},
								},
							},
						},
					},
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "test",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"test"},
										},
									},
								},
							},
						},
					},
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
							{
								Weight: 100,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "test",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"test"},
											},
										},
									},
								},
							},
						},
					},
				}
			},
			wantErr: "failed to update affinity rules: [spec.affinity.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].values: Required value: must be specified when `operator` is 'In' or 'NotIn', spec.affinity.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey: Required value: can not be empty, spec.affinity.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey: Invalid value: \"\": name part must be non-empty, spec.affinity.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey: Invalid value: \"\": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.affinity.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey: Required value: can not be empty, spec.affinity.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey: Invalid value: \"\": name part must be non-empty, spec.affinity.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey: Invalid value: \"\": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')]",
		},
		{
			name: "deployment reconciliation with invalid resource requirement configuration",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
			},
			updateExternalSecretsConfig: func(i *v1alpha1.ExternalSecretsConfig) {
				i.Spec.ApplicationConfig.Resources = &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
						"test":                resource.MustParse("100.0"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				}
			},
			wantErr: `failed to update resource requirements: invalid resource requirements: [spec.resources.requests[test]: Invalid value: test: must be a standard resource type or fully qualified, spec.resources.requests[test]: Invalid value: test: must be a standard resource for containers]`,
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
			externalsecrets := commontest.TestExternalSecretsConfig()

			if tt.updateExternalSecretsConfig != nil {
				tt.updateExternalSecretsConfig(externalsecrets)
			}
			if !tt.skipEnvVar {
				t.Setenv("RELATED_IMAGE_EXTERNAL_SECRETS", commontest.TestExternalSecretsImageName)
			}
			t.Setenv("RELATED_IMAGE_BITWARDEN_SDK_SERVER", commontest.TestBitwardenImageName)

			err := r.createOrApplyDeployments(externalsecrets, controllerDefaultResourceLabels, false)
			if (tt.wantErr != "" || err != nil) && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("createOrApplyDeployments() err: %v, wantErr: %v", err, tt.wantErr)
			}
			if tt.wantErr == "" && externalsecrets.Status.ExternalSecretsImage != commontest.TestExternalSecretsImageName {
				t.Errorf("createOrApplyDeployments() got image in status: %v, want: %v", externalsecrets.Status.ExternalSecretsImage, "test-image")
			}
		})
	}
}
