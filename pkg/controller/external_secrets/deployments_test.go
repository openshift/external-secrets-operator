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
		preReq                      func(*Reconciler, *fakes.FakeCtrlClient, **appsv1.Deployment)
		updateExternalSecretsConfig func(*v1alpha1.ExternalSecretsConfig)
		validateDeployment          func(*testing.T, *appsv1.Deployment)
		skipEnvVar                  bool
		wantErr                     string
	}{
		{
			name: "deployment reconciliation successful",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) error {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						*capturedDeployment = o.DeepCopy()
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(i *v1alpha1.ExternalSecretsConfig) {
				i.Status.ExternalSecretsImage = commontest.TestExternalSecretsImageName
			},
			validateDeployment: func(t *testing.T, deployment *appsv1.Deployment) {
				// Validate basic deployment structure
				if deployment == nil {
					t.Error("deployment should not be nil")
					return
				}
				// Validate container image is updated
				if len(deployment.Spec.Template.Spec.Containers) > 0 {
					container := deployment.Spec.Template.Spec.Containers[0]
					if container.Image == "" {
						t.Error("container image should be set")
					}
				}
				// Validate labels are preserved
				if deployment.Labels == nil || len(deployment.Labels) == 0 {
					t.Error("deployment should have labels")
				}
			},
		},
		{
			name: "deployment reconciliation fails as image env var is empty",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
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
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
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
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
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
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) error {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						*capturedDeployment = o.DeepCopy()
					}
					return nil
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
			validateDeployment: func(t *testing.T, deployment *appsv1.Deployment) {
				if deployment == nil {
					t.Error("deployment should not be nil")
					return
				}

				podSpec := &deployment.Spec.Template.Spec

				// Validate Affinity
				if podSpec.Affinity == nil {
					t.Error("Affinity should be set")
					return
				}

				// Validate NodeAffinity
				if podSpec.Affinity.NodeAffinity == nil ||
					podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil ||
					len(podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) == 0 {
					t.Error("NodeAffinity should be properly configured")
				} else {
					term := podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0]
					if len(term.MatchExpressions) == 0 ||
						term.MatchExpressions[0].Key != "node" ||
						term.MatchExpressions[0].Operator != corev1.NodeSelectorOpIn ||
						len(term.MatchExpressions[0].Values) == 0 ||
						term.MatchExpressions[0].Values[0] != "test" {
						t.Error("NodeAffinity match expressions should be configured correctly")
					}
				}

				// Validate PodAffinity
				if podSpec.Affinity.PodAffinity == nil ||
					len(podSpec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution) == 0 {
					t.Error("PodAffinity should be configured")
				} else {
					podAffinityTerm := podSpec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0]
					if podAffinityTerm.TopologyKey != "topology.kubernetes.io/zone" {
						t.Errorf("PodAffinity TopologyKey should be 'topology.kubernetes.io/zone', got: %s", podAffinityTerm.TopologyKey)
					}
				}

				// Validate PodAntiAffinity
				if podSpec.Affinity.PodAntiAffinity == nil ||
					len(podSpec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution) == 0 {
					t.Error("PodAntiAffinity should be configured")
				} else {
					weightedTerm := podSpec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0]
					if weightedTerm.Weight != 100 {
						t.Errorf("PodAntiAffinity weight should be 100, got: %d", weightedTerm.Weight)
					}
					if weightedTerm.PodAffinityTerm.TopologyKey != "topology.kubernetes.io/zone" {
						t.Errorf("PodAntiAffinity TopologyKey should be 'topology.kubernetes.io/zone', got: %s", weightedTerm.PodAffinityTerm.TopologyKey)
					}
				}

				// Validate Tolerations
				if len(podSpec.Tolerations) == 0 {
					t.Error("Tolerations should be set")
				} else {
					toleration := podSpec.Tolerations[0]
					if toleration.Key != "type" ||
						toleration.Operator != corev1.TolerationOpEqual ||
						toleration.Value != "test" ||
						toleration.Effect != corev1.TaintEffectNoSchedule {
						t.Errorf("Toleration configuration is incorrect: %+v", toleration)
					}
				}

				// Validate NodeSelector
				if podSpec.NodeSelector == nil {
					t.Error("NodeSelector should be set")
				} else {
					if podSpec.NodeSelector["type"] != "test" {
						t.Errorf("NodeSelector should have type=test, got: %v", podSpec.NodeSelector)
					}
				}

				// Validate Resources
				if len(podSpec.Containers) == 0 {
					t.Error("Containers should be present")
					return
				}
				container := podSpec.Containers[0]

				expectedCPU := resource.MustParse("100m")
				expectedMemory := resource.MustParse("100Mi")

				if !container.Resources.Requests[corev1.ResourceCPU].Equal(expectedCPU) {
					t.Errorf("CPU request should be %v, got: %v", expectedCPU, container.Resources.Requests[corev1.ResourceCPU])
				}
				if !container.Resources.Requests[corev1.ResourceMemory].Equal(expectedMemory) {
					t.Errorf("Memory request should be %v, got: %v", expectedMemory, container.Resources.Requests[corev1.ResourceMemory])
				}
				if !container.Resources.Limits[corev1.ResourceCPU].Equal(expectedCPU) {
					t.Errorf("CPU limit should be %v, got: %v", expectedCPU, container.Resources.Limits[corev1.ResourceCPU])
				}
				if !container.Resources.Limits[corev1.ResourceMemory].Equal(expectedMemory) {
					t.Errorf("Memory limit should be %v, got: %v", expectedMemory, container.Resources.Limits[corev1.ResourceMemory])
				}
			},
		},
		{
			name: "deployment reconciliation fails while updating image in externalsecrets status",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
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
						*capturedDeployment = o.DeepCopy()
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
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
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
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
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
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
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
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
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
		{
			name: "bitwarden is enabled with secretRef for certificates",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						// Create a deployment with bitwarden-tls-certs volume to test volume update
						deployment := testDeployment(bitwardenDeploymentAssetName)
						deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
							{
								Name: "bitwarden-tls-certs",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "initial-secret-name", // This should be updated by reconciler
									},
								},
							},
						}
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) error {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						*capturedDeployment = o.DeepCopy()
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(i *v1alpha1.ExternalSecretsConfig) {
				if i.Spec.Plugins.BitwardenSecretManagerProvider == nil {
					i.Spec.Plugins.BitwardenSecretManagerProvider = &v1alpha1.BitwardenSecretManagerProvider{
						Mode: v1alpha1.Enabled,
						SecretRef: &v1alpha1.SecretReference{
							Name: "bitwarden-certs",
						},
					}
				}
				if i.Spec.ControllerConfig.CertProvider == nil {
					i.Spec.ControllerConfig.CertProvider = &v1alpha1.CertProvidersConfig{
						CertManager: &v1alpha1.CertManagerConfig{
							Mode: v1alpha1.Enabled,
						},
					}
				}
			},
			validateDeployment: func(t *testing.T, deployment *appsv1.Deployment) {
				if deployment == nil {
					t.Error("deployment should not be nil")
					return
				}

				// Validate that bitwarden-tls-certs volume secret name was updated
				foundVolume := false
				for _, volume := range deployment.Spec.Template.Spec.Volumes {
					if volume.Name == "bitwarden-tls-certs" {
						foundVolume = true
						if volume.Secret == nil {
							t.Error("bitwarden-tls-certs volume should have a secret")
						} else if volume.Secret.SecretName != "bitwarden-certs" {
							t.Errorf("bitwarden-tls-certs volume secret name should be updated to 'bitwarden-certs', got: %s", volume.Secret.SecretName)
						}
						break
					}
				}
				if !foundVolume {
					t.Error("bitwarden-tls-certs volume should exist in deployment")
				}

				// Validate that bitwarden-sdk-server container image was updated
				foundContainer := false
				for _, container := range deployment.Spec.Template.Spec.Containers {
					if container.Name == "bitwarden-sdk-server" {
						foundContainer = true
						if container.Image != commontest.TestBitwardenImageName {
							t.Errorf("bitwarden-sdk-server container image should be %s, got: %s", commontest.TestBitwardenImageName, container.Image)
						}
						break
					}
				}
				if !foundContainer {
					t.Error("bitwarden-sdk-server container should exist in deployment")
				}

				// Validate basic deployment structure
				if len(deployment.Spec.Template.Spec.Containers) == 0 {
					t.Error("deployment should have at least one container")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)
			mock := &fakes.FakeCtrlClient{}
			var capturedDeployment *appsv1.Deployment

			if tt.preReq != nil {
				tt.preReq(r, mock, &capturedDeployment)
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
				t.Errorf("createOrApplyDeployments() got image in status: %v, want: %v", externalsecrets.Status.ExternalSecretsImage, commontest.TestExternalSecretsImageName)
			}

			// Validate deployment changes if validation function is provided
			if tt.validateDeployment != nil && capturedDeployment != nil {
				tt.validateDeployment(t, capturedDeployment)
			}
		})
	}
}
