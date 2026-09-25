package external_secrets

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

const (
	testAffinityValue               = "test"
	bitwardenSDKServerContainerName = "bitwarden-sdk-server"
)

// Helper function to create an ExistsCalls mock that returns false.
func doesNotExist() func(context.Context, types.NamespacedName, client.Object) (bool, error) {
	return func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
		return false, nil
	}
}

// Helper function to set up mock for deployment creation.
func setupDeploymentCreate(m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment, deploymentName string) {
	m.ExistsCalls(doesNotExist())
	m.CreateCalls(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) error {
		switch o := obj.(type) {
		case *appsv1.Deployment:
			if o.Name == deploymentName {
				*capturedDeployment = o.DeepCopy()
			}
		}
		return nil
	})
}

// Helper function to validate revision history limit.
func validateRevisionHistory(expectedLimit int32) func(*testing.T, *appsv1.Deployment) {
	return func(t *testing.T, deployment *appsv1.Deployment) {
		if deployment == nil {
			t.Error("deployment should not be nil")
			return
		}
		if deployment.Spec.RevisionHistoryLimit == nil {
			t.Error("revisionHistoryLimit should be set")
			return
		}
		if *deployment.Spec.RevisionHistoryLimit != expectedLimit {
			t.Errorf("revisionHistoryLimit = %d, want %d", *deployment.Spec.RevisionHistoryLimit, expectedLimit)
		}
	}
}

// Helper to create component config with revision history limit.
func componentConfigWithRevisionLimit(name v1alpha1.ComponentName, limit *int32) v1alpha1.ComponentConfig {
	return v1alpha1.ComponentConfig{
		ComponentName:     name,
		DeploymentConfigs: &v1alpha1.DeploymentConfig{RevisionHistoryLimit: limit},
	}
}

// Helper to create ESC update function with component configs.
func escWithComponentConfigs(configs ...v1alpha1.ComponentConfig) func(*v1alpha1.ExternalSecretsConfig) {
	return func(esc *v1alpha1.ExternalSecretsConfig) {
		esc.Status.ExternalSecretsImage = commontest.TestExternalSecretsImageName
		esc.Spec.ControllerConfig.ComponentConfigs = configs
	}
}

func containerArgs(deployment *appsv1.Deployment, containerName string) []string {
	if deployment == nil {
		return nil
	}
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			return container.Args
		}
	}
	return nil
}

func validateContainerHasArg(containerName, arg string) func(*testing.T, *appsv1.Deployment) {
	return func(t *testing.T, deployment *appsv1.Deployment) {
		t.Helper()
		args := containerArgs(deployment, containerName)
		if args == nil {
			t.Fatalf("container %q not found in deployment", containerName)
		}
		if !slices.Contains(args, arg) {
			t.Errorf("container %q args = %v, want to contain %q", containerName, args, arg)
		}
	}
}

func validateContainerMissingArg(containerName, arg string) func(*testing.T, *appsv1.Deployment) {
	return func(t *testing.T, deployment *appsv1.Deployment) {
		t.Helper()
		args := containerArgs(deployment, containerName)
		if args == nil {
			t.Fatalf("container %q not found in deployment", containerName)
		}
		if slices.Contains(args, arg) {
			t.Errorf("container %q args = %v, should not contain %q", containerName, args, arg)
		}
	}
}

func TestUpdateOptionalFeatures(t *testing.T) {
	enabledESM := &v1alpha1.ExternalSecretsManager{
		Spec: v1alpha1.ExternalSecretsManagerSpec{
			Features: []v1alpha1.Feature{
				{Name: v1alpha1.UnsafeAllowGenericTargets, Mode: v1alpha1.Enabled},
			},
		},
	}
	disabledESM := &v1alpha1.ExternalSecretsManager{
		Spec: v1alpha1.ExternalSecretsManagerSpec{
			Features: []v1alpha1.Feature{
				{Name: v1alpha1.UnsafeAllowGenericTargets, Mode: v1alpha1.Disabled},
			},
		},
	}

	tests := []struct {
		name              string
		esm               *v1alpha1.ExternalSecretsManager
		supportedFeatures []v1alpha1.FeatureName
		wantArg           bool
	}{
		{name: "nil esm", esm: nil, supportedFeatures: []v1alpha1.FeatureName{v1alpha1.UnsafeAllowGenericTargets}, wantArg: false},
		{name: "empty features", esm: &v1alpha1.ExternalSecretsManager{}, supportedFeatures: []v1alpha1.FeatureName{v1alpha1.UnsafeAllowGenericTargets}, wantArg: false},
		{name: "enabled feature supported by deployment", esm: enabledESM, supportedFeatures: []v1alpha1.FeatureName{v1alpha1.UnsafeAllowGenericTargets}, wantArg: true},
		{name: "disabled feature", esm: disabledESM, supportedFeatures: []v1alpha1.FeatureName{v1alpha1.UnsafeAllowGenericTargets}, wantArg: false},
		{name: "enabled feature not supported by deployment", esm: enabledESM, supportedFeatures: nil, wantArg: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"--existing-arg=true"}
			r := testReconciler(t)
			r.esm = tt.esm
			r.updateOptionalFeatures(&args, tt.supportedFeatures)

			hasArg := slices.Contains(args, UnsafeAllowGenericTargetsArg)
			if hasArg != tt.wantArg {
				t.Errorf("updateOptionalFeatures() arg present = %v, want %v (args=%v)", hasArg, tt.wantArg, args)
			}
		})
	}
}

func TestCreateOrApplyDeployments(t *testing.T) {
	tests := []struct {
		name                        string
		preReq                      func(*Reconciler, *fakes.FakeCtrlClient, **appsv1.Deployment)
		updateExternalSecretsConfig func(*v1alpha1.ExternalSecretsConfig)
		externalSecretsManager      *v1alpha1.ExternalSecretsManager
		validateDeployment          func(*testing.T, *appsv1.Deployment)
		skipEnvVar                  bool
		wantErr                     string
	}{
		{
			name: "deployment reconciliation successful",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if o, ok := obj.(*appsv1.Deployment); ok {
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) error {
					if o, ok := obj.(*appsv1.Deployment); ok {
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
				if len(deployment.Labels) == 0 {
					t.Error("deployment should have labels")
				}
			},
		},
		{
			name: "deployment reconciliation fails as image env var is empty",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if o, ok := obj.(*appsv1.Deployment); ok {
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
					if _, ok := obj.(*appsv1.Deployment); ok {
						return false, commontest.ErrTestClient
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
					if o, ok := obj.(*appsv1.Deployment); ok {
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.Labels = nil
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) error {
					if _, ok := obj.(*appsv1.Deployment); ok {
						return commontest.ErrTestClient
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
					if o, ok := obj.(*appsv1.Deployment); ok {
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) error {
					if o, ok := obj.(*appsv1.Deployment); ok {
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
											Values:   []string{testAffinityValue},
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
											Values:   []string{testAffinityValue},
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
												Values:   []string{testAffinityValue},
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
						term.MatchExpressions[0].Values[0] != testAffinityValue {
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
				} else if podSpec.NodeSelector["type"] != "test" {
					t.Errorf("NodeSelector should have type=test, got: %v", podSpec.NodeSelector)
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
					if _, ok := obj.(*appsv1.Deployment); ok {
						return false, nil
					}
					return true, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if o, ok := obj.(*appsv1.Deployment); ok {
						*capturedDeployment = o.DeepCopy()
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return nil
				})
				m.StatusUpdateCalls(func(ctx context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
					if _, ok := obj.(*v1alpha1.ExternalSecretsConfig); ok {
						return commontest.ErrTestClient
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
					if o, ok := obj.(*appsv1.Deployment); ok {
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
					if o, ok := obj.(*appsv1.Deployment); ok {
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
			},
			updateExternalSecretsConfig: func(i *v1alpha1.ExternalSecretsConfig) {
				i.Spec.ApplicationConfig.NodeSelector = map[string]string{"node/Label/2": "value2"}
			},
			wantErr: `failed to update node selector: spec.nodeSelector: Invalid value: "node/Label/2": a valid label key must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]') with an optional DNS subdomain prefix and '/' (e.g. 'example.com/MyName')`,
		},
		{
			name: "deployment reconciliation with invalid affinity configuration",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if o, ok := obj.(*appsv1.Deployment); ok {
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
											Values:   []string{testAffinityValue},
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
												Values:   []string{testAffinityValue},
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
					if o, ok := obj.(*appsv1.Deployment); ok {
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
			wantErr: `failed to update resource requirements: invalid resource requirements: [spec.resources.requests[test]: Invalid value: "test": must be a standard resource type or fully qualified, spec.resources.requests[test]: Invalid value: "test": must be a standard resource for containers]`,
		},
		{
			name: "bitwarden is enabled with secretRef for certificates",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if o, ok := obj.(*appsv1.Deployment); ok {
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
					if o, ok := obj.(*appsv1.Deployment); ok {
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
					if container.Name == bitwardenSDKServerContainerName {
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
		{
			name: "deployment with custom revisionHistoryLimit from componentConfig",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, d **appsv1.Deployment) {
				setupDeploymentCreate(m, d, "external-secrets")
			},
			updateExternalSecretsConfig: escWithComponentConfigs(componentConfigWithRevisionLimit(v1alpha1.CoreController, ptr.To(int32(5)))),
			validateDeployment:          validateRevisionHistory(5),
		},
		{
			name: "deployment without revisionHistoryLimit should use default",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, d **appsv1.Deployment) {
				setupDeploymentCreate(m, d, "external-secrets")
			},
			updateExternalSecretsConfig: escWithComponentConfigs(componentConfigWithRevisionLimit(v1alpha1.CoreController, nil)),
			validateDeployment:          validateRevisionHistory(10),
		},
		{
			name: "deployment with nil DeploymentConfigs should not panic",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, d **appsv1.Deployment) {
				setupDeploymentCreate(m, d, "external-secrets")
			},
			updateExternalSecretsConfig: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Status.ExternalSecretsImage = commontest.TestExternalSecretsImageName
				esc.Spec.ControllerConfig.ComponentConfigs = []v1alpha1.ComponentConfig{{ComponentName: v1alpha1.CoreController, DeploymentConfigs: nil}}
			},
			validateDeployment: validateRevisionHistory(10),
		},
		{
			name: "multiple components with mixed revisionHistoryLimit configurations",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, d **appsv1.Deployment) {
				m.ExistsCalls(doesNotExist())
				m.CreateCalls(func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
					if dep, ok := obj.(*appsv1.Deployment); ok && dep.Name == "external-secrets-webhook" {
						*d = dep.DeepCopy()
					}
					return nil
				})
			},
			updateExternalSecretsConfig: escWithComponentConfigs(
				componentConfigWithRevisionLimit(v1alpha1.CoreController, ptr.To(int32(3))),
				componentConfigWithRevisionLimit(v1alpha1.Webhook, ptr.To(int32(7))),
				componentConfigWithRevisionLimit(v1alpha1.CertController, nil),
			),
			validateDeployment: func(t *testing.T, d *appsv1.Deployment) {
				if d == nil || d.Name != "external-secrets-webhook" {
					t.Errorf("expected webhook deployment, got %v", d)
					return
				}
				validateRevisionHistory(7)(t, d)
			},
		},
		{
			name: "deployment includes unsafe allow generic targets arg when feature enabled",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if o, ok := obj.(*appsv1.Deployment); ok {
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) error {
					if o, ok := obj.(*appsv1.Deployment); ok && o.Name == externalsecretsCommonName {
						*capturedDeployment = o.DeepCopy()
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(i *v1alpha1.ExternalSecretsConfig) {
				i.Status.ExternalSecretsImage = commontest.TestExternalSecretsImageName
			},
			externalSecretsManager: &v1alpha1.ExternalSecretsManager{
				Spec: v1alpha1.ExternalSecretsManagerSpec{
					Features: []v1alpha1.Feature{
						{Name: v1alpha1.UnsafeAllowGenericTargets, Mode: v1alpha1.Enabled},
					},
				},
			},
			validateDeployment: validateContainerHasArg(OperandCoreControllerContainer, UnsafeAllowGenericTargetsArg),
		},
		{
			name: "deployment omits unsafe allow generic targets arg when feature disabled",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient, capturedDeployment **appsv1.Deployment) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if o, ok := obj.(*appsv1.Deployment); ok {
						deployment := testDeployment(controllerDeploymentAssetName)
						deployment.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) error {
					if o, ok := obj.(*appsv1.Deployment); ok && o.Name == externalsecretsCommonName {
						*capturedDeployment = o.DeepCopy()
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(i *v1alpha1.ExternalSecretsConfig) {
				i.Status.ExternalSecretsImage = commontest.TestExternalSecretsImageName
			},
			externalSecretsManager: &v1alpha1.ExternalSecretsManager{
				Spec: v1alpha1.ExternalSecretsManagerSpec{
					Features: []v1alpha1.Feature{
						{Name: v1alpha1.UnsafeAllowGenericTargets, Mode: v1alpha1.Disabled},
					},
				},
			},
			validateDeployment: validateContainerMissingArg(OperandCoreControllerContainer, UnsafeAllowGenericTargetsArg),
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
			if tt.externalSecretsManager != nil {
				r.esm = tt.externalSecretsManager
			}
			if !tt.skipEnvVar {
				t.Setenv("RELATED_IMAGE_EXTERNAL_SECRETS", commontest.TestExternalSecretsImageName)
			}
			t.Setenv("RELATED_IMAGE_BITWARDEN_SDK_SERVER", commontest.TestBitwardenImageName)

			err := r.createOrApplyDeployments(externalsecrets, testResourceMetadata(externalsecrets), false)
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

func TestUpdateProxyConfiguration(t *testing.T) {
	// Expected trusted CA bundle volume
	expectedTrustedCAVolume := corev1.Volume{
		Name: "trusted-ca-bundle",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: ProxyTrustedCABundleConfigMapName,
				},
				DefaultMode: ptr.To(int32(420)),
			},
		},
	}

	tests := []struct {
		name                     string
		deployment               *appsv1.Deployment
		externalSecretsConfig    *v1alpha1.ExternalSecretsConfig
		externalSecretsManager   *v1alpha1.ExternalSecretsManager
		olmEnvVars               map[string]string
		expectedContainerEnvVars map[string][]corev1.EnvVar      // container name -> env vars
		expectedVolumes          []corev1.Volume                 // expected volumes in the deployment
		expectedVolumeMounts     map[string][]corev1.VolumeMount // container name -> volume mounts
	}{
		{
			name: "ExternalSecretsConfig proxy takes precedence",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{
								{Name: "init-migration"},
							},
							Containers: []corev1.Container{
								{Name: "external-secrets"},
								{Name: "webhook"},
							},
						},
					},
				},
			},
			externalSecretsConfig: &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{
					ApplicationConfig: v1alpha1.ApplicationConfig{
						CommonConfigs: v1alpha1.CommonConfigs{
							Proxy: &v1alpha1.ProxyConfig{
								HTTPProxy:  "http://esc-proxy:8080",
								HTTPSProxy: "https://esc-proxy:8443",
								NoProxy:    "esc.local",
							},
						},
					},
				},
			},
			externalSecretsManager: &v1alpha1.ExternalSecretsManager{
				Spec: v1alpha1.ExternalSecretsManagerSpec{
					GlobalConfig: &v1alpha1.GlobalConfig{
						CommonConfigs: v1alpha1.CommonConfigs{
							Proxy: &v1alpha1.ProxyConfig{
								HTTPProxy:  "http://esm-proxy:8080",
								HTTPSProxy: "https://esm-proxy:8443",
								NoProxy:    "esm.local",
							},
						},
					},
				},
			},
			olmEnvVars: map[string]string{
				"HTTP_PROXY":  "http://olm-proxy:8080",
				"HTTPS_PROXY": "https://olm-proxy:8443",
				"NO_PROXY":    "olm.local",
			},
			expectedContainerEnvVars: map[string][]corev1.EnvVar{
				"init-migration": {
					{Name: "HTTP_PROXY", Value: "http://esc-proxy:8080"},
					{Name: "HTTPS_PROXY", Value: "https://esc-proxy:8443"},
					{Name: "NO_PROXY", Value: "esc.local"},
					{Name: "http_proxy", Value: "http://esc-proxy:8080"},
					{Name: "https_proxy", Value: "https://esc-proxy:8443"},
					{Name: "no_proxy", Value: "esc.local"},
				},
				"external-secrets": {
					{Name: "HTTP_PROXY", Value: "http://esc-proxy:8080"},
					{Name: "HTTPS_PROXY", Value: "https://esc-proxy:8443"},
					{Name: "NO_PROXY", Value: "esc.local"},
					{Name: "http_proxy", Value: "http://esc-proxy:8080"},
					{Name: "https_proxy", Value: "https://esc-proxy:8443"},
					{Name: "no_proxy", Value: "esc.local"},
				},
				"webhook": {
					{Name: "HTTP_PROXY", Value: "http://esc-proxy:8080"},
					{Name: "HTTPS_PROXY", Value: "https://esc-proxy:8443"},
					{Name: "NO_PROXY", Value: "esc.local"},
					{Name: "http_proxy", Value: "http://esc-proxy:8080"},
					{Name: "https_proxy", Value: "https://esc-proxy:8443"},
					{Name: "no_proxy", Value: "esc.local"},
				},
			},
			expectedVolumes: []corev1.Volume{expectedTrustedCAVolume},
			expectedVolumeMounts: map[string][]corev1.VolumeMount{
				"init-migration": {
					{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
				},
				"external-secrets": {
					{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
				},
				"webhook": {
					{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
				},
			},
		},
		{
			name: "ExternalSecretsManager proxy when ESC has no proxy",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "external-secrets"},
								{Name: "webhook"},
							},
						},
					},
				},
			},
			externalSecretsConfig: &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{
					ApplicationConfig: v1alpha1.ApplicationConfig{
						CommonConfigs: v1alpha1.CommonConfigs{
							// No proxy config
						},
					},
				},
			},
			externalSecretsManager: &v1alpha1.ExternalSecretsManager{
				Spec: v1alpha1.ExternalSecretsManagerSpec{
					GlobalConfig: &v1alpha1.GlobalConfig{
						CommonConfigs: v1alpha1.CommonConfigs{
							Proxy: &v1alpha1.ProxyConfig{
								HTTPProxy:  "http://esm-proxy:8080",
								HTTPSProxy: "https://esm-proxy:8443",
								NoProxy:    "esm.local",
							},
						},
					},
				},
			},
			olmEnvVars: map[string]string{
				"HTTP_PROXY":  "http://olm-proxy:8080",
				"HTTPS_PROXY": "https://olm-proxy:8443",
				"NO_PROXY":    "olm.local",
			},
			expectedContainerEnvVars: map[string][]corev1.EnvVar{
				"external-secrets": {
					{Name: "HTTP_PROXY", Value: "http://esm-proxy:8080"},
					{Name: "HTTPS_PROXY", Value: "https://esm-proxy:8443"},
					{Name: "NO_PROXY", Value: "esm.local"},
					{Name: "http_proxy", Value: "http://esm-proxy:8080"},
					{Name: "https_proxy", Value: "https://esm-proxy:8443"},
					{Name: "no_proxy", Value: "esm.local"},
				},
				"webhook": {
					{Name: "HTTP_PROXY", Value: "http://esm-proxy:8080"},
					{Name: "HTTPS_PROXY", Value: "https://esm-proxy:8443"},
					{Name: "NO_PROXY", Value: "esm.local"},
					{Name: "http_proxy", Value: "http://esm-proxy:8080"},
					{Name: "https_proxy", Value: "https://esm-proxy:8443"},
					{Name: "no_proxy", Value: "esm.local"},
				},
			},
			expectedVolumes: []corev1.Volume{expectedTrustedCAVolume},
			expectedVolumeMounts: map[string][]corev1.VolumeMount{
				"external-secrets": {
					{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
				},
				"webhook": {
					{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
				},
			},
		},
		{
			name: "OLM environment variables used when no config proxy",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "external-secrets"},
							},
						},
					},
				},
			},
			externalSecretsConfig:  &v1alpha1.ExternalSecretsConfig{},
			externalSecretsManager: &v1alpha1.ExternalSecretsManager{},
			olmEnvVars: map[string]string{
				"HTTP_PROXY":  "http://olm-proxy:8080",
				"HTTPS_PROXY": "https://olm-proxy:8443",
				"NO_PROXY":    "olm.local",
			},
			expectedContainerEnvVars: map[string][]corev1.EnvVar{
				"external-secrets": {
					{Name: "HTTP_PROXY", Value: "http://olm-proxy:8080"},
					{Name: "HTTPS_PROXY", Value: "https://olm-proxy:8443"},
					{Name: "NO_PROXY", Value: "olm.local"},
					{Name: "http_proxy", Value: "http://olm-proxy:8080"},
					{Name: "https_proxy", Value: "https://olm-proxy:8443"},
					{Name: "no_proxy", Value: "olm.local"},
				},
			},
			expectedVolumes: []corev1.Volume{expectedTrustedCAVolume},
			expectedVolumeMounts: map[string][]corev1.VolumeMount{
				"external-secrets": {
					{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
				},
			},
		},
		{
			name: "Partial proxy configuration",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "external-secrets"},
							},
						},
					},
				},
			},
			externalSecretsConfig: &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{
					ApplicationConfig: v1alpha1.ApplicationConfig{
						CommonConfigs: v1alpha1.CommonConfigs{
							Proxy: &v1alpha1.ProxyConfig{
								HTTPProxy: "http://esc-proxy:8080",
								// HTTPSProxy and NoProxy are empty
							},
						},
					},
				},
			},
			externalSecretsManager: &v1alpha1.ExternalSecretsManager{},
			olmEnvVars:             map[string]string{},
			expectedContainerEnvVars: map[string][]corev1.EnvVar{
				"external-secrets": {
					{Name: "HTTP_PROXY", Value: "http://esc-proxy:8080"},
					{Name: "http_proxy", Value: "http://esc-proxy:8080"},
				},
			},
			expectedVolumes: []corev1.Volume{expectedTrustedCAVolume},
			expectedVolumeMounts: map[string][]corev1.VolumeMount{
				"external-secrets": {
					{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
				},
			},
		},
		{
			name: "Update existing proxy environment variables",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "external-secrets",
									Env: []corev1.EnvVar{
										{Name: "HTTP_PROXY", Value: "http://old-proxy:8080"},
										{Name: "EXISTING_VAR", Value: "existing-value"},
									},
								},
							},
						},
					},
				},
			},
			externalSecretsConfig: &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{
					ApplicationConfig: v1alpha1.ApplicationConfig{
						CommonConfigs: v1alpha1.CommonConfigs{
							Proxy: &v1alpha1.ProxyConfig{
								HTTPProxy:  "http://new-proxy:8080",
								HTTPSProxy: "https://new-proxy:8443",
								NoProxy:    "localhost",
							},
						},
					},
				},
			},
			externalSecretsManager: &v1alpha1.ExternalSecretsManager{},
			olmEnvVars:             map[string]string{},
			expectedContainerEnvVars: map[string][]corev1.EnvVar{
				"external-secrets": {
					{Name: "HTTP_PROXY", Value: "http://new-proxy:8080"},
					{Name: "EXISTING_VAR", Value: "existing-value"},
					{Name: "HTTPS_PROXY", Value: "https://new-proxy:8443"},
					{Name: "NO_PROXY", Value: "localhost"},
					{Name: "http_proxy", Value: "http://new-proxy:8080"},
					{Name: "https_proxy", Value: "https://new-proxy:8443"},
					{Name: "no_proxy", Value: "localhost"},
				},
			},
			expectedVolumes: []corev1.Volume{expectedTrustedCAVolume},
			expectedVolumeMounts: map[string][]corev1.VolumeMount{
				"external-secrets": {
					{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
				},
			},
		},
		{
			name: "No proxy configuration results in no changes",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "external-secrets",
									Env: []corev1.EnvVar{
										{Name: "EXISTING_VAR", Value: "existing-value"},
									},
								},
							},
						},
					},
				},
			},
			externalSecretsConfig:  &v1alpha1.ExternalSecretsConfig{},
			externalSecretsManager: &v1alpha1.ExternalSecretsManager{},
			olmEnvVars:             map[string]string{},
			expectedContainerEnvVars: map[string][]corev1.EnvVar{
				"external-secrets": {
					{Name: "EXISTING_VAR", Value: "existing-value"},
				},
			},
			expectedVolumes:      []corev1.Volume{},
			expectedVolumeMounts: map[string][]corev1.VolumeMount{},
		},
		{
			name: "Proxy configuration applied to init containers",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{
								{Name: "init-setup"},
							},
							Containers: []corev1.Container{
								{Name: "external-secrets"},
							},
						},
					},
				},
			},
			externalSecretsConfig: &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{
					ApplicationConfig: v1alpha1.ApplicationConfig{
						CommonConfigs: v1alpha1.CommonConfigs{
							Proxy: &v1alpha1.ProxyConfig{
								HTTPProxy:  "http://esc-proxy:8080",
								HTTPSProxy: "https://esc-proxy:8443",
								NoProxy:    "esc.local",
							},
						},
					},
				},
			},
			externalSecretsManager: &v1alpha1.ExternalSecretsManager{},
			olmEnvVars:             map[string]string{},
			expectedContainerEnvVars: map[string][]corev1.EnvVar{
				"init-setup": {
					{Name: "HTTP_PROXY", Value: "http://esc-proxy:8080"},
					{Name: "HTTPS_PROXY", Value: "https://esc-proxy:8443"},
					{Name: "NO_PROXY", Value: "esc.local"},
					{Name: "http_proxy", Value: "http://esc-proxy:8080"},
					{Name: "https_proxy", Value: "https://esc-proxy:8443"},
					{Name: "no_proxy", Value: "esc.local"},
				},
				"external-secrets": {
					{Name: "HTTP_PROXY", Value: "http://esc-proxy:8080"},
					{Name: "HTTPS_PROXY", Value: "https://esc-proxy:8443"},
					{Name: "NO_PROXY", Value: "esc.local"},
					{Name: "http_proxy", Value: "http://esc-proxy:8080"},
					{Name: "https_proxy", Value: "https://esc-proxy:8443"},
					{Name: "no_proxy", Value: "esc.local"},
				},
			},
			expectedVolumes: []corev1.Volume{expectedTrustedCAVolume},
			expectedVolumeMounts: map[string][]corev1.VolumeMount{
				"init-setup": {
					{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
				},
				"external-secrets": {
					{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
				},
			},
		},
		{
			name: "Proxy with only networkPolicyProvisioning Unmanaged keeps OLM proxy env",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "external-secrets",
									Env: []corev1.EnvVar{
										{Name: "HTTP_PROXY", Value: "http://old-proxy:8080"},
										{Name: "HTTPS_PROXY", Value: "https://old-proxy:8443"},
										{Name: "NO_PROXY", Value: "old.local"},
										{Name: "http_proxy", Value: "http://old-proxy:8080"},
										{Name: "https_proxy", Value: "https://old-proxy:8443"},
										{Name: "no_proxy", Value: "old.local"},
										{Name: "KEEP_THIS_VAR", Value: "keep-value"},
									},
									VolumeMounts: []corev1.VolumeMount{
										{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "trusted-ca-bundle",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: ProxyTrustedCABundleConfigMapName,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			externalSecretsConfig: &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{
					ApplicationConfig: v1alpha1.ApplicationConfig{
						CommonConfigs: v1alpha1.CommonConfigs{
							Proxy: &v1alpha1.ProxyConfig{
								NetworkPolicyProvisioning: v1alpha1.ManagementStateUnmanaged,
							},
						},
					},
				},
			},
			externalSecretsManager: &v1alpha1.ExternalSecretsManager{},
			olmEnvVars: map[string]string{
				httpProxyEnvVar:  "http://olm-proxy:8080",
				httpsProxyEnvVar: "https://olm-proxy:8443",
				noProxyEnvVar:    "olm.local",
			},
			expectedContainerEnvVars: map[string][]corev1.EnvVar{
				"external-secrets": {
					{Name: "HTTP_PROXY", Value: "http://olm-proxy:8080"},
					{Name: "HTTPS_PROXY", Value: "https://olm-proxy:8443"},
					{Name: "NO_PROXY", Value: "olm.local"},
					{Name: "http_proxy", Value: "http://olm-proxy:8080"},
					{Name: "https_proxy", Value: "https://olm-proxy:8443"},
					{Name: "no_proxy", Value: "olm.local"},
					{Name: "KEEP_THIS_VAR", Value: "keep-value"},
				},
			},
			expectedVolumes: []corev1.Volume{expectedTrustedCAVolume},
			expectedVolumeMounts: map[string][]corev1.VolumeMount{
				"external-secrets": {
					{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
				},
			},
		},
		{
			name: "Proxy configuration removal cleans up environment variables and volumes",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{
								{
									Name: "init-setup",
									Env: []corev1.EnvVar{
										{Name: "HTTP_PROXY", Value: "http://old-proxy:8080"},
										{Name: "HTTPS_PROXY", Value: "https://old-proxy:8443"},
										{Name: "NO_PROXY", Value: "old.local"},
										{Name: "http_proxy", Value: "http://old-proxy:8080"},
										{Name: "https_proxy", Value: "https://old-proxy:8443"},
										{Name: "no_proxy", Value: "old.local"},
										{Name: "KEEP_THIS_VAR", Value: "keep-value"},
									},
									VolumeMounts: []corev1.VolumeMount{
										{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
										{Name: "other-volume", MountPath: "/other", ReadOnly: true},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Name: "external-secrets",
									Env: []corev1.EnvVar{
										{Name: "HTTP_PROXY", Value: "http://old-proxy:8080"},
										{Name: "HTTPS_PROXY", Value: "https://old-proxy:8443"},
										{Name: "NO_PROXY", Value: "old.local"},
										{Name: "http_proxy", Value: "http://old-proxy:8080"},
										{Name: "https_proxy", Value: "https://old-proxy:8443"},
										{Name: "no_proxy", Value: "old.local"},
										{Name: "KEEP_THIS_VAR", Value: "keep-value"},
									},
									VolumeMounts: []corev1.VolumeMount{
										{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
										{Name: "other-volume", MountPath: "/other", ReadOnly: true},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "trusted-ca-bundle",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: ProxyTrustedCABundleConfigMapName,
											},
										},
									},
								},
								{
									Name: "other-volume",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
							},
						},
					},
				},
			},
			externalSecretsConfig:  &v1alpha1.ExternalSecretsConfig{}, // No proxy configuration
			externalSecretsManager: &v1alpha1.ExternalSecretsManager{},
			olmEnvVars:             map[string]string{},
			expectedContainerEnvVars: map[string][]corev1.EnvVar{
				"init-setup": {
					{Name: "KEEP_THIS_VAR", Value: "keep-value"},
				},
				"external-secrets": {
					{Name: "KEEP_THIS_VAR", Value: "keep-value"},
				},
			},
			expectedVolumes: []corev1.Volume{
				{
					Name: "other-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			expectedVolumeMounts: map[string][]corev1.VolumeMount{
				"init-setup": {
					{Name: "other-volume", MountPath: "/other", ReadOnly: true},
				},
				"external-secrets": {
					{Name: "other-volume", MountPath: "/other", ReadOnly: true},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{
				httpProxyEnvVar, httpsProxyEnvVar, noProxyEnvVar,
				httpProxyEnvVarLowercase, httpsProxyEnvVarLowercase, noProxyEnvVarLowercase,
			} {
				t.Setenv(key, "")
			}
			for key, value := range tt.olmEnvVars {
				t.Setenv(key, value)
			}

			r := &Reconciler{
				esm: tt.externalSecretsManager,
			}
			if err := r.validateExternalSecretsConfig(tt.externalSecretsConfig); err != nil {
				t.Fatalf("validateExternalSecretsConfig() error = %v", err)
			}

			r.updateProxyConfiguration(tt.deployment)

			validateEnvironmentVariables(t, tt.deployment, tt.expectedContainerEnvVars)
			validateVolumes(t, tt.deployment, tt.expectedVolumes)
			validateVolumeMounts(t, tt.deployment, tt.expectedVolumeMounts)
		})
	}
}

// validateEnvironmentVariables validates that containers have expected environment variables.
func validateEnvironmentVariables(t *testing.T, deployment *appsv1.Deployment, expectedContainerEnvVars map[string][]corev1.EnvVar) {
	for containerName, expectedEnvVars := range expectedContainerEnvVars {
		container := findContainer(deployment, containerName)
		if container == nil {
			t.Errorf("Container %s not found in deployment", containerName)
			return
		}
		actualEnv := append([]corev1.EnvVar(nil), container.Env...)
		expectedEnv := append([]corev1.EnvVar(nil), expectedEnvVars...)
		slices.SortFunc(actualEnv, func(a, b corev1.EnvVar) int { return strings.Compare(a.Name, b.Name) })
		slices.SortFunc(expectedEnv, func(a, b corev1.EnvVar) int { return strings.Compare(a.Name, b.Name) })
		if !reflect.DeepEqual(actualEnv, expectedEnv) {
			t.Errorf("Container %s environment variables mismatch.\nExpected: %+v\nActual: %+v",
				containerName, expectedEnv, actualEnv)
		}
	}
}

// validateVolumes validates that deployment has expected volumes.
func validateVolumes(t *testing.T, deployment *appsv1.Deployment, expectedVolumes []corev1.Volume) {
	if len(expectedVolumes) == 0 {
		// Verify no trusted CA bundle volume was added
		for _, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.Name == ProxyTrustedCABundleVolumeName {
				t.Errorf("Expected no trusted CA bundle volume, but found one: %+v", volume)
			}
		}
		return
	}

	// Verify expected volumes exist and match exactly
	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Volumes, expectedVolumes) {
		t.Errorf("Volumes mismatch.\nExpected: %+v\nActual: %+v",
			expectedVolumes, deployment.Spec.Template.Spec.Volumes)
	}
}

// validateVolumeMounts validates that containers have expected volume mounts.
func validateVolumeMounts(t *testing.T, deployment *appsv1.Deployment, expectedVolumeMounts map[string][]corev1.VolumeMount) {
	if len(expectedVolumeMounts) == 0 {
		// Verify no trusted CA bundle volume mounts exist in any container
		for _, container := range deployment.Spec.Template.Spec.Containers {
			trustedCAMounts := filterTrustedCAMounts(container.VolumeMounts)
			if len(trustedCAMounts) > 0 {
				t.Errorf("Expected no trusted CA bundle volume mount in container %s, but found: %+v",
					container.Name, trustedCAMounts)
			}
		}
		return
	}

	// Verify expected volume mounts exist
	for containerName, expectedMounts := range expectedVolumeMounts {
		container := findContainer(deployment, containerName)
		if container == nil {
			t.Errorf("Container %s not found for volume mount validation", containerName)
			continue
		}

		// Determine if we're testing for trusted CA mounts or non-trusted CA mounts
		var actualMounts []corev1.VolumeMount
		if len(expectedMounts) > 0 && expectedMounts[0].Name == ProxyTrustedCABundleVolumeName {
			// Testing for trusted CA mounts
			actualMounts = filterTrustedCAMounts(container.VolumeMounts)
		} else {
			// Testing for non-trusted CA mounts (e.g., in removal scenarios)
			actualMounts = filterNonTrustedCAMounts(container.VolumeMounts)
		}

		if !reflect.DeepEqual(actualMounts, expectedMounts) {
			t.Errorf("Container %s volume mounts mismatch.\nExpected: %+v\nActual: %+v",
				containerName, expectedMounts, actualMounts)
		}
	}
}

// findContainer finds a container by name in the deployment.
func findContainer(deployment *appsv1.Deployment, containerName string) *corev1.Container {
	// Search regular containers first
	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			return &deployment.Spec.Template.Spec.Containers[i]
		}
	}
	// Search init containers
	for i, container := range deployment.Spec.Template.Spec.InitContainers {
		if container.Name == containerName {
			return &deployment.Spec.Template.Spec.InitContainers[i]
		}
	}
	return nil
}

// filterTrustedCAMounts filters volume mounts to only include trusted CA bundle mounts.
func filterTrustedCAMounts(volumeMounts []corev1.VolumeMount) []corev1.VolumeMount {
	var trustedCAMounts []corev1.VolumeMount
	for _, mount := range volumeMounts {
		if mount.Name == ProxyTrustedCABundleVolumeName {
			trustedCAMounts = append(trustedCAMounts, mount)
		}
	}
	return trustedCAMounts
}

// filterNonTrustedCAMounts filters volume mounts to exclude trusted CA bundle mounts.
func filterNonTrustedCAMounts(volumeMounts []corev1.VolumeMount) []corev1.VolumeMount {
	var nonTrustedCAMounts []corev1.VolumeMount
	for _, mount := range volumeMounts {
		if mount.Name != ProxyTrustedCABundleVolumeName {
			nonTrustedCAMounts = append(nonTrustedCAMounts, mount)
		}
	}
	return nonTrustedCAMounts
}

func TestGetComponentNameFromAsset(t *testing.T) {
	tests := []struct {
		name          string
		assetName     string
		wantComponent v1alpha1.ComponentName
		wantContainer string
		wantErr       bool
		errContains   string
	}{
		{
			name:          "valid controller deployment asset",
			assetName:     controllerDeploymentAssetName,
			wantComponent: v1alpha1.CoreController,
			wantContainer: "external-secrets",
			wantErr:       false,
		},
		{
			name:          "valid webhook deployment asset",
			assetName:     webhookDeploymentAssetName,
			wantComponent: v1alpha1.Webhook,
			wantContainer: "webhook",
			wantErr:       false,
		},
		{
			name:          "valid cert controller deployment asset",
			assetName:     certControllerDeploymentAssetName,
			wantComponent: v1alpha1.CertController,
			wantContainer: "cert-controller",
			wantErr:       false,
		},
		{
			name:          "valid bitwarden deployment asset",
			assetName:     bitwardenDeploymentAssetName,
			wantComponent: v1alpha1.BitwardenSDKServer,
			wantContainer: "bitwarden-sdk-server",
			wantErr:       false,
		},
		{
			name:          "invalid asset name returns error",
			assetName:     "invalid-asset-name.yml",
			wantComponent: "",
			wantContainer: "",
			wantErr:       true,
			errContains:   "unknown deployment asset name",
		},
		{
			name:          "empty asset name returns error",
			assetName:     "",
			wantComponent: "",
			wantContainer: "",
			wantErr:       true,
			errContains:   "unknown deployment asset name",
		},
		{
			name:          "random string returns error",
			assetName:     "some-random-deployment.yml",
			wantComponent: "",
			wantContainer: "",
			wantErr:       true,
			errContains:   "unknown deployment asset name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotComponent, gotContainer, err := getComponentNameFromAsset(tt.assetName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("getComponentNameFromAsset() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("getComponentNameFromAsset() error = %v, should contain %q", err, tt.errContains)
				}
				if gotComponent != "" {
					t.Errorf("getComponentNameFromAsset() on error should return empty component, got %v", gotComponent)
				}
				if gotContainer != "" {
					t.Errorf("getComponentNameFromAsset() on error should return empty container, got %v", gotContainer)
				}
			} else {
				if err != nil {
					t.Errorf("getComponentNameFromAsset() unexpected error = %v", err)
					return
				}
				if gotComponent != tt.wantComponent {
					t.Errorf("getComponentNameFromAsset() component = %v, want %v", gotComponent, tt.wantComponent)
				}
				if gotContainer != tt.wantContainer {
					t.Errorf("getComponentNameFromAsset() container = %v, want %v", gotContainer, tt.wantContainer)
				}
			}
		})
	}
}

func TestMergeEnvVars(t *testing.T) {
	tests := []struct {
		name        string
		existingEnv []corev1.EnvVar
		overrideEnv []corev1.EnvVar
		expectedEnv []corev1.EnvVar
	}{
		{
			name:        "empty container env, add new env vars",
			existingEnv: nil,
			overrideEnv: []corev1.EnvVar{
				{Name: "LOG_LEVEL", Value: "debug"},
				{Name: "TIMEOUT", Value: "30s"},
			},
			expectedEnv: []corev1.EnvVar{
				{Name: "LOG_LEVEL", Value: "debug"},
				{Name: "TIMEOUT", Value: "30s"},
			},
		},
		{
			name: "override existing env var",
			existingEnv: []corev1.EnvVar{
				{Name: "LOG_LEVEL", Value: "info"},
				{Name: "OTHER_VAR", Value: "value"},
			},
			overrideEnv: []corev1.EnvVar{
				{Name: "LOG_LEVEL", Value: "debug"},
			},
			expectedEnv: []corev1.EnvVar{
				{Name: "LOG_LEVEL", Value: "debug"},
				{Name: "OTHER_VAR", Value: "value"},
			},
		},
		{
			name: "add new env var to existing ones",
			existingEnv: []corev1.EnvVar{
				{Name: "EXISTING_VAR", Value: "existing"},
			},
			overrideEnv: []corev1.EnvVar{
				{Name: "NEW_VAR", Value: "new"},
			},
			expectedEnv: []corev1.EnvVar{
				{Name: "EXISTING_VAR", Value: "existing"},
				{Name: "NEW_VAR", Value: "new"},
			},
		},
		{
			name: "mix of override and new env vars",
			existingEnv: []corev1.EnvVar{
				{Name: "VAR_A", Value: "old_a"},
				{Name: "VAR_B", Value: "old_b"},
			},
			overrideEnv: []corev1.EnvVar{
				{Name: "VAR_A", Value: "new_a"},
				{Name: "VAR_C", Value: "new_c"},
			},
			expectedEnv: []corev1.EnvVar{
				{Name: "VAR_A", Value: "new_a"},
				{Name: "VAR_B", Value: "old_b"},
				{Name: "VAR_C", Value: "new_c"},
			},
		},
		{
			name: "empty override env vars does nothing",
			existingEnv: []corev1.EnvVar{
				{Name: "EXISTING", Value: "value"},
			},
			overrideEnv: []corev1.EnvVar{},
			expectedEnv: []corev1.EnvVar{
				{Name: "EXISTING", Value: "value"},
			},
		},
		{
			name: "override env var with ValueFrom",
			existingEnv: []corev1.EnvVar{
				{Name: "SECRET_VAR", Value: "plaintext"},
			},
			overrideEnv: []corev1.EnvVar{
				{
					Name: "SECRET_VAR",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
							Key:                  "password",
						},
					},
				},
			},
			expectedEnv: []corev1.EnvVar{
				{
					Name: "SECRET_VAR",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
							Key:                  "password",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &corev1.Container{
				Name: "test-container",
				Env:  tt.existingEnv,
			}

			mergeUserEnvVars(container, tt.overrideEnv)

			if len(container.Env) != len(tt.expectedEnv) {
				t.Errorf("mergeEnvVars() got %d env vars, want %d", len(container.Env), len(tt.expectedEnv))
				return
			}

			for i, expected := range tt.expectedEnv {
				found := false
				for _, actual := range container.Env {
					if actual.Name == expected.Name {
						found = true
						if expected.ValueFrom != nil {
							if actual.ValueFrom == nil {
								t.Errorf("mergeEnvVars() env var %s expected ValueFrom but got Value", expected.Name)
							}
						} else if actual.Value != expected.Value {
							t.Errorf("mergeEnvVars() env var %s = %v, want %v", expected.Name, actual.Value, expected.Value)
						}
						break
					}
				}
				if !found {
					t.Errorf("mergeEnvVars() missing env var %s at index %d", expected.Name, i)
				}
			}
		})
	}
}

func TestMergeContainerEnvVars(t *testing.T) {
	t.Parallel()

	t.Run("managed env vars are sorted by name", func(t *testing.T) {
		t.Parallel()
		container := &corev1.Container{
			Env: []corev1.EnvVar{{Name: "UNMANAGED", Value: "keep"}},
		}
		mergeContainerEnvVars(container, map[string]string{
			httpsProxyEnvVar:          "https://proxy:8080",
			httpProxyEnvVar:           "http://proxy:8080",
			noProxyEnvVar:             ".cluster.local",
			httpProxyEnvVarLowercase:  "http://proxy:8080",
			httpsProxyEnvVarLowercase: "https://proxy:8080",
			noProxyEnvVarLowercase:    ".cluster.local",
		})

		wantManaged := []string{
			httpsProxyEnvVar,
			httpProxyEnvVar,
			noProxyEnvVar,
			httpProxyEnvVarLowercase,
			httpsProxyEnvVarLowercase,
			noProxyEnvVarLowercase,
		}
		if len(container.Env) != len(wantManaged)+1 {
			t.Fatalf("got %d env vars, want %d", len(container.Env), len(wantManaged)+1)
		}
		for i, name := range wantManaged {
			if container.Env[i].Name != name {
				t.Fatalf("managed env[%d].Name = %q, want %q", i, container.Env[i].Name, name)
			}
		}
		if container.Env[len(wantManaged)].Name != "UNMANAGED" {
			t.Fatalf("unmanaged env = %q, want UNMANAGED", container.Env[len(wantManaged)].Name)
		}
	})

	t.Run("repeated merge produces identical env order", func(t *testing.T) {
		t.Parallel()
		envVars := map[string]string{
			httpsProxyEnvVar: "https://proxy:8080",
			httpProxyEnvVar:  "http://proxy:8080",
			noProxyEnvVar:    ".cluster.local",
		}
		first := &corev1.Container{
			Env: []corev1.EnvVar{{Name: "LOG_LEVEL", Value: "info"}},
		}
		second := &corev1.Container{
			Env: []corev1.EnvVar{{Name: "LOG_LEVEL", Value: "info"}},
		}

		for range 10 {
			mergeContainerEnvVars(first, envVars)
		}
		for range 10 {
			mergeContainerEnvVars(second, envVars)
		}

		if !reflect.DeepEqual(first.Env, second.Env) {
			t.Fatalf("env order not stable across merges:\nfirst:  %#v\nsecond: %#v", first.Env, second.Env)
		}
	})

	t.Run("empty value omits managed key", func(t *testing.T) {
		t.Parallel()
		container := &corev1.Container{
			Env: []corev1.EnvVar{
				{Name: httpProxyEnvVar, Value: "old"},
				{Name: "OTHER", Value: "x"},
			},
		}
		mergeContainerEnvVars(container, map[string]string{
			httpProxyEnvVar: "",
		})
		for _, env := range container.Env {
			if env.Name == httpProxyEnvVar {
				t.Fatal("expected http proxy env var to be removed")
			}
		}
		if len(container.Env) != 1 || container.Env[0].Name != "OTHER" {
			t.Fatalf("unexpected env: %#v", container.Env)
		}
	})
}

func TestApplyUserDeploymentConfigsWithOverrideEnv(t *testing.T) {
	tests := []struct {
		name            string
		assetName       string
		containerName   string
		componentConfig v1alpha1.ComponentConfig
		existingEnv     []corev1.EnvVar
		expectedEnv     []corev1.EnvVar
	}{
		{
			name:          "apply override env to core controller",
			assetName:     controllerDeploymentAssetName,
			containerName: "external-secrets",
			componentConfig: v1alpha1.ComponentConfig{
				ComponentName: v1alpha1.CoreController,
				OverrideEnv: []corev1.EnvVar{
					{Name: "LOG_LEVEL", Value: "debug"},
				},
			},
			existingEnv: []corev1.EnvVar{},
			expectedEnv: []corev1.EnvVar{
				{Name: "LOG_LEVEL", Value: "debug"},
			},
		},
		{
			name:          "apply override env to webhook",
			assetName:     webhookDeploymentAssetName,
			containerName: "webhook",
			componentConfig: v1alpha1.ComponentConfig{
				ComponentName: v1alpha1.Webhook,
				OverrideEnv: []corev1.EnvVar{
					{Name: "TIMEOUT", Value: "60s"},
				},
			},
			existingEnv: []corev1.EnvVar{
				{Name: "EXISTING", Value: "value"},
			},
			expectedEnv: []corev1.EnvVar{
				{Name: "EXISTING", Value: "value"},
				{Name: "TIMEOUT", Value: "60s"},
			},
		},
		{
			name:          "apply override env to cert controller",
			assetName:     certControllerDeploymentAssetName,
			containerName: "cert-controller",
			componentConfig: v1alpha1.ComponentConfig{
				ComponentName: v1alpha1.CertController,
				OverrideEnv: []corev1.EnvVar{
					{Name: "CERT_DURATION", Value: "8760h"},
				},
			},
			existingEnv: []corev1.EnvVar{},
			expectedEnv: []corev1.EnvVar{
				{Name: "CERT_DURATION", Value: "8760h"},
			},
		},
		{
			name:          "apply override env to bitwarden",
			assetName:     bitwardenDeploymentAssetName,
			containerName: "bitwarden-sdk-server",
			componentConfig: v1alpha1.ComponentConfig{
				ComponentName: v1alpha1.BitwardenSDKServer,
				OverrideEnv: []corev1.EnvVar{
					{Name: "API_URL", Value: "https://api.bitwarden.com"},
				},
			},
			existingEnv: []corev1.EnvVar{},
			expectedEnv: []corev1.EnvVar{
				{Name: "API_URL", Value: "https://api.bitwarden.com"},
			},
		},
		{
			name:          "no override env does not modify container",
			assetName:     controllerDeploymentAssetName,
			containerName: "external-secrets",
			componentConfig: v1alpha1.ComponentConfig{
				ComponentName: v1alpha1.CoreController,
				OverrideEnv:   nil,
			},
			existingEnv: []corev1.EnvVar{
				{Name: "EXISTING", Value: "value"},
			},
			expectedEnv: []corev1.EnvVar{
				{Name: "EXISTING", Value: "value"},
			},
		},
		{
			name:          "both revision history and override env",
			assetName:     controllerDeploymentAssetName,
			containerName: "external-secrets",
			componentConfig: v1alpha1.ComponentConfig{
				ComponentName: v1alpha1.CoreController,
				DeploymentConfigs: &v1alpha1.DeploymentConfig{
					RevisionHistoryLimit: ptr.To(int32(5)),
				},
				OverrideEnv: []corev1.EnvVar{
					{Name: "LOG_LEVEL", Value: "debug"},
				},
			},
			existingEnv: []corev1.EnvVar{},
			expectedEnv: []corev1.EnvVar{
				{Name: "LOG_LEVEL", Value: "debug"},
			},
		},
		{
			name:          "override env does not apply to init containers",
			assetName:     controllerDeploymentAssetName,
			containerName: "external-secrets",
			componentConfig: v1alpha1.ComponentConfig{
				ComponentName: v1alpha1.CoreController,
				OverrideEnv: []corev1.EnvVar{
					{Name: "LOG_LEVEL", Value: "debug"},
					{Name: "TIMEOUT", Value: "30s"},
				},
			},
			existingEnv: []corev1.EnvVar{},
			expectedEnv: []corev1.EnvVar{
				{Name: "LOG_LEVEL", Value: "debug"},
				{Name: "TIMEOUT", Value: "30s"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)
			initEnv := []corev1.EnvVar{{Name: "INIT_ONLY_VAR", Value: "init-value"}}
			deployment := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{
								{Name: "init-setup", Env: initEnv},
							},
							Containers: []corev1.Container{
								{
									Name: tt.containerName,
									Env:  tt.existingEnv,
								},
							},
						},
					},
				},
			}

			esc := &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{
					ControllerConfig: v1alpha1.ControllerConfig{
						ComponentConfigs: []v1alpha1.ComponentConfig{tt.componentConfig},
					},
				},
			}

			err := r.applyUserDeploymentConfigs(deployment, esc, tt.assetName)
			if err != nil {
				t.Errorf("applyUserDeploymentConfigs() unexpected error: %v", err)
				return
			}

			container := &deployment.Spec.Template.Spec.Containers[0]
			if len(container.Env) != len(tt.expectedEnv) {
				t.Errorf("applyUserDeploymentConfigs() got %d env vars, want %d. Got: %v", len(container.Env), len(tt.expectedEnv), container.Env)
				return
			}

			for _, expected := range tt.expectedEnv {
				found := false
				for _, actual := range container.Env {
					if actual.Name == expected.Name && actual.Value == expected.Value {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("applyUserDeploymentConfigs() missing env var %s=%s", expected.Name, expected.Value)
				}
			}

			// Verify init containers are NOT modified by OverrideEnv
			if len(deployment.Spec.Template.Spec.InitContainers) > 0 {
				initContainer := &deployment.Spec.Template.Spec.InitContainers[0]
				if !reflect.DeepEqual(initContainer.Env, initEnv) {
					t.Errorf("applyUserDeploymentConfigs() should not modify init container env.\nExpected: %+v\nActual: %+v",
						initEnv, initContainer.Env)
				}
			}

			// Verify revision history limit if set
			if tt.componentConfig.DeploymentConfigs != nil && tt.componentConfig.DeploymentConfigs.RevisionHistoryLimit != nil {
				if deployment.Spec.RevisionHistoryLimit == nil {
					t.Error("applyUserDeploymentConfigs() RevisionHistoryLimit should be set")
				} else if *deployment.Spec.RevisionHistoryLimit != *tt.componentConfig.DeploymentConfigs.RevisionHistoryLimit {
					t.Errorf("applyUserDeploymentConfigs() RevisionHistoryLimit = %d, want %d",
						*deployment.Spec.RevisionHistoryLimit, *tt.componentConfig.DeploymentConfigs.RevisionHistoryLimit)
				}
			}
		})
	}
}

func TestApplyUserDeploymentConfigsWithReplicas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                     string
		assetName                string
		containerName            string
		componentName            v1alpha1.ComponentName
		replicas                 *int32
		existingArgs             []string
		expectedReplicas         *int32
		expectLeaderElectionArg  bool
		expectNoLeaderElectionIn bool
	}{
		{
			name:                    "replicas=3 on core controller sets replicas and injects leader election",
			assetName:               controllerDeploymentAssetName,
			containerName:           OperandCoreControllerContainer,
			componentName:           v1alpha1.CoreController,
			replicas:                ptr.To(int32(3)),
			expectedReplicas:        ptr.To(int32(3)),
			expectLeaderElectionArg: true,
		},
		{
			name:                    "replicas=1 on core controller sets replicas and does NOT inject leader election",
			assetName:               controllerDeploymentAssetName,
			containerName:           OperandCoreControllerContainer,
			componentName:           v1alpha1.CoreController,
			replicas:                ptr.To(int32(1)),
			expectedReplicas:        ptr.To(int32(1)),
			expectLeaderElectionArg: false,
		},
		{
			name:                    "replicas=nil on core controller leaves replicas unchanged, no leader election",
			assetName:               controllerDeploymentAssetName,
			containerName:           OperandCoreControllerContainer,
			componentName:           v1alpha1.CoreController,
			replicas:                nil,
			expectedReplicas:        nil,
			expectLeaderElectionArg: false,
		},
		{
			name:                     "replicas=2 on webhook sets replicas, no leader election",
			assetName:                webhookDeploymentAssetName,
			containerName:            OperandWebhookContainer,
			componentName:            v1alpha1.Webhook,
			replicas:                 ptr.To(int32(2)),
			expectedReplicas:         ptr.To(int32(2)),
			expectLeaderElectionArg:  false,
			expectNoLeaderElectionIn: true,
		},
		{
			name:                     "replicas=2 on cert-controller sets replicas, no leader election",
			assetName:                certControllerDeploymentAssetName,
			containerName:            OperandCertControllerContainer,
			componentName:            v1alpha1.CertController,
			replicas:                 ptr.To(int32(2)),
			expectedReplicas:         ptr.To(int32(2)),
			expectLeaderElectionArg:  false,
			expectNoLeaderElectionIn: true,
		},
		{
			name:                     "replicas=2 on bitwarden sets replicas, no leader election",
			assetName:                bitwardenDeploymentAssetName,
			containerName:            OperandBitwardenContainer,
			componentName:            v1alpha1.BitwardenSDKServer,
			replicas:                 ptr.To(int32(2)),
			expectedReplicas:         ptr.To(int32(2)),
			expectLeaderElectionArg:  false,
			expectNoLeaderElectionIn: true,
		},
		{
			name:                    "replicas=1 on core controller removes pre-existing leader election arg",
			assetName:               controllerDeploymentAssetName,
			containerName:           OperandCoreControllerContainer,
			componentName:           v1alpha1.CoreController,
			replicas:                ptr.To(int32(1)),
			existingArgs:            []string{"--some-flag=value", LeaderElectionArg},
			expectedReplicas:        ptr.To(int32(1)),
			expectLeaderElectionArg: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := testReconciler(t)
			deployment := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: tt.containerName,
									Args: tt.existingArgs,
								},
							},
						},
					},
				},
			}

			componentConfig := v1alpha1.ComponentConfig{
				ComponentName: tt.componentName,
			}
			if tt.replicas != nil {
				componentConfig.DeploymentConfigs = &v1alpha1.DeploymentConfig{
					Replicas: tt.replicas,
				}
			}

			esc := &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{
					ControllerConfig: v1alpha1.ControllerConfig{
						ComponentConfigs: []v1alpha1.ComponentConfig{componentConfig},
					},
				},
			}

			if err := r.applyUserDeploymentConfigs(deployment, esc, tt.assetName); err != nil {
				t.Fatalf("applyUserDeploymentConfigs() unexpected error: %v", err)
			}

			if tt.expectedReplicas != nil {
				if deployment.Spec.Replicas == nil {
					t.Errorf("expected replicas=%d, got nil", *tt.expectedReplicas)
				} else if *deployment.Spec.Replicas != *tt.expectedReplicas {
					t.Errorf("expected replicas=%d, got %d", *tt.expectedReplicas, *deployment.Spec.Replicas)
				}
			} else if deployment.Spec.Replicas != nil {
				t.Errorf("expected replicas=nil, got %d", *deployment.Spec.Replicas)
			}

			container := &deployment.Spec.Template.Spec.Containers[0]
			hasLeaderElection := slices.Contains(container.Args, LeaderElectionArg)

			if tt.expectLeaderElectionArg && !hasLeaderElection {
				t.Errorf("expected %s in container args, got: %v", LeaderElectionArg, container.Args)
			}
			if !tt.expectLeaderElectionArg && hasLeaderElection {
				t.Errorf("did not expect %s in container args, got: %v", LeaderElectionArg, container.Args)
			}
			if tt.expectNoLeaderElectionIn {
				for _, arg := range container.Args {
					if strings.Contains(arg, "--enable-leader-election") {
						t.Errorf("component %s must NOT have --enable-leader-election arg, got: %v", tt.componentName, container.Args)
					}
				}
			}
		})
	}
}

func TestApplyUserDeploymentConfigsReplicasRegression(t *testing.T) {
	t.Parallel()

	t.Run("replicas does not break existing revisionHistoryLimit and overrideEnv", func(t *testing.T) {
		t.Parallel()
		r := testReconciler(t)
		deployment := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: OperandCoreControllerContainer},
						},
					},
				},
			},
		}

		esc := &v1alpha1.ExternalSecretsConfig{
			Spec: v1alpha1.ExternalSecretsConfigSpec{
				ControllerConfig: v1alpha1.ControllerConfig{
					ComponentConfigs: []v1alpha1.ComponentConfig{
						{
							ComponentName: v1alpha1.CoreController,
							DeploymentConfigs: &v1alpha1.DeploymentConfig{
								RevisionHistoryLimit: ptr.To(int32(5)),
								Replicas:             ptr.To(int32(3)),
							},
							OverrideEnv: []corev1.EnvVar{
								{Name: "LOG_LEVEL", Value: "debug"},
							},
						},
					},
				},
			},
		}

		if err := r.applyUserDeploymentConfigs(deployment, esc, controllerDeploymentAssetName); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if deployment.Spec.RevisionHistoryLimit == nil || *deployment.Spec.RevisionHistoryLimit != 5 {
			t.Errorf("expected RevisionHistoryLimit=5, got %v", deployment.Spec.RevisionHistoryLimit)
		}
		if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 3 {
			t.Errorf("expected Replicas=3, got %v", deployment.Spec.Replicas)
		}
		container := &deployment.Spec.Template.Spec.Containers[0]
		if !slices.Contains(container.Args, LeaderElectionArg) {
			t.Errorf("expected leader election arg for replicas=3, got args: %v", container.Args)
		}
		envFound := false
		for _, env := range container.Env {
			if env.Name == "LOG_LEVEL" && env.Value == "debug" {
				envFound = true
			}
		}
		if !envFound {
			t.Error("expected LOG_LEVEL=debug env var to be applied")
		}
	})

	t.Run("valid replicas value does not set Degraded condition", func(t *testing.T) {
		t.Parallel()
		r := testReconciler(t)
		deployment := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: OperandCoreControllerContainer},
						},
					},
				},
			},
		}

		for _, replicas := range []int32{1, 2, 5, 10} {
			esc := &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{
					ControllerConfig: v1alpha1.ControllerConfig{
						ComponentConfigs: []v1alpha1.ComponentConfig{
							{
								ComponentName: v1alpha1.CoreController,
								DeploymentConfigs: &v1alpha1.DeploymentConfig{
									Replicas: ptr.To(replicas),
								},
							},
						},
					},
				},
			}

			err := r.applyUserDeploymentConfigs(deployment, esc, controllerDeploymentAssetName)
			if err != nil {
				t.Errorf("replicas=%d: unexpected error (would set Degraded): %v", replicas, err)
			}
		}
	})
}

func TestApplyLeaderElection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		replicas     *int32
		existingArgs []string
		expectedArgs []string
	}{
		{
			name:         "injects leader election when replicas > 1",
			replicas:     ptr.To(int32(3)),
			existingArgs: []string{"--some-flag=value"},
			expectedArgs: []string{"--some-flag=value", LeaderElectionArg},
		},
		{
			name:         "removes leader election when replicas = 1",
			replicas:     ptr.To(int32(1)),
			existingArgs: []string{"--some-flag=value", LeaderElectionArg},
			expectedArgs: []string{"--some-flag=value"},
		},
		{
			name:         "removes leader election when replicas = nil",
			replicas:     nil,
			existingArgs: []string{LeaderElectionArg, "--other=true"},
			expectedArgs: []string{"--other=true"},
		},
		{
			name:         "no-op when replicas = 1 and no leader election arg present",
			replicas:     ptr.To(int32(1)),
			existingArgs: []string{"--some-flag=value"},
			expectedArgs: []string{"--some-flag=value"},
		},
		{
			name:         "idempotent injection when already present and replicas > 1",
			replicas:     ptr.To(int32(2)),
			existingArgs: []string{LeaderElectionArg, "--other=true"},
			expectedArgs: []string{LeaderElectionArg, "--other=true"},
		},
		{
			name:         "empty args with replicas > 1",
			replicas:     ptr.To(int32(5)),
			existingArgs: nil,
			expectedArgs: []string{LeaderElectionArg},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			container := &corev1.Container{
				Args: slices.Clone(tt.existingArgs),
			}
			applyLeaderElection(container, tt.replicas)
			if !slices.Equal(container.Args, tt.expectedArgs) {
				t.Errorf("applyLeaderElection() args = %v, want %v", container.Args, tt.expectedArgs)
			}
		})
	}
}

func TestDeploymentSpecModifiedWithReplicas(t *testing.T) {
	t.Parallel()

	t.Run("drift detection — replicas changed from 3 to 1 triggers modified", func(t *testing.T) {
		t.Parallel()
		desired := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(3)),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: OperandCoreControllerContainer,
							Args: []string{LeaderElectionArg},
						}},
					},
				},
			},
		}
		fetched := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: OperandCoreControllerContainer,
							Args: []string{LeaderElectionArg},
						}},
					},
				},
			},
		}
		meta := &common.ResourceMetadata{}
		if !common.HasObjectChanged(desired, fetched, meta) {
			t.Error("expected HasObjectChanged=true when replicas differ (3 vs 1)")
		}
	})

	t.Run("drift detection — leader election arg removed triggers modified", func(t *testing.T) {
		t.Parallel()
		desired := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(3)),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: OperandCoreControllerContainer,
							Args: []string{"--some-flag=value", LeaderElectionArg},
						}},
					},
				},
			},
		}
		fetched := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(3)),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: OperandCoreControllerContainer,
							Args: []string{"--some-flag=value"},
						}},
					},
				},
			},
		}
		meta := &common.ResourceMetadata{}
		if !common.HasObjectChanged(desired, fetched, meta) {
			t.Error("expected HasObjectChanged=true when leader election arg is missing from fetched")
		}
	})

	t.Run("recovery without CR edit — external actor change detected", func(t *testing.T) {
		t.Parallel()
		desired := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(3)),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: OperandCoreControllerContainer,
							Args: []string{LeaderElectionArg},
						}},
					},
				},
			},
		}
		fetched := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: OperandCoreControllerContainer,
							Args: []string{LeaderElectionArg},
						}},
					},
				},
			},
		}
		meta := &common.ResourceMetadata{}
		if !common.HasObjectChanged(desired, fetched, meta) {
			t.Error("expected drift detection when external actor modifies replicas")
		}
	})
}

func TestNormalizeDurationArg(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "5m", want: "5m0s"},
		{in: "5m0s", want: "5m0s"},
		{in: "15m0s", want: "15m0s"},
		{in: "1h", want: "1h0m0s"},
		{in: "not-a-duration", want: "not-a-duration"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := normalizeDurationArg(tt.in); got != tt.want {
				t.Errorf("normalizeDurationArg(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestApplyUserCABundleConfig(t *testing.T) {
	t.Parallel()

	validPEM := mustPEMCert(t, true)
	cmName := "user-ca-bundle"

	baseESC := &v1alpha1.ExternalSecretsConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.ExternalSecretsConfigSpec{
			ControllerConfig: v1alpha1.ControllerConfig{
				TrustedCABundle: &v1alpha1.ConfigMapKeyReference{
					Name: cmName,
					Key:  UserCABundleKeyPath,
				},
			},
		},
	}

	baseDeployment := func() *appsv1.Deployment {
		return &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: OperandCoreControllerContainer}},
					},
				},
			},
		}
	}

	multiContainerDeployment := func() *appsv1.Deployment {
		return &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: OperandCoreControllerContainer},
							{Name: "sidecar"},
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name         string
		esc          *v1alpha1.ExternalSecretsConfig
		cm           *corev1.ConfigMap
		cmNotFound   bool
		deployment   *appsv1.Deployment
		proxyConfig  *v1alpha1.ProxyConfig
		wantErr      bool
		wantTrusted  bool
		assertEvents func(t *testing.T, r *Reconciler)
		assertDeploy func(t *testing.T, deployment *appsv1.Deployment)
	}{
		{
			name:        "mounts volume and SSL_CERT_DIR when valid",
			esc:         baseESC,
			cm:          testUserCAConfigMap(cmName, validPEM, nil),
			deployment:  baseDeployment(),
			wantTrusted: true,
			assertDeploy: func(t *testing.T, deployment *appsv1.Deployment) {
				t.Helper()
				foundVolume := false
				for _, vol := range deployment.Spec.Template.Spec.Volumes {
					if vol.Name == UserCABundleVolumeName && vol.ConfigMap != nil && vol.ConfigMap.Name == cmName {
						foundVolume = true
					}
				}
				if !foundVolume {
					t.Fatal("expected user-ca-bundle volume")
				}
				container := findContainer(deployment, OperandCoreControllerContainer)
				if container == nil {
					t.Fatal("controller container not found")
				}
				hasUserCAMount := false
				for _, mount := range container.VolumeMounts {
					if mount.Name == UserCABundleVolumeName {
						hasUserCAMount = true
					}
				}
				if !hasUserCAMount {
					t.Fatal("expected user-ca-bundle volume mount on controller container")
				}
				hasSSLCertDir := false
				for _, env := range container.Env {
					if env.Name == SSLCertDirEnvVar && env.Value == SSLCertDirValue {
						hasSSLCertDir = true
					}
				}
				if !hasSSLCertDir {
					t.Fatal("expected SSL_CERT_DIR on controller container")
				}
			},
		},
		{
			name:        "mounts only on controller container when multiple containers exist",
			esc:         baseESC,
			cm:          testUserCAConfigMap(cmName, validPEM, nil),
			deployment:  multiContainerDeployment(),
			wantTrusted: true,
			assertDeploy: func(t *testing.T, deployment *appsv1.Deployment) {
				t.Helper()
				controller := findContainer(deployment, OperandCoreControllerContainer)
				if controller == nil {
					t.Fatal("controller container not found")
				}
				hasUserCAMount := false
				for _, mount := range controller.VolumeMounts {
					if mount.Name == UserCABundleVolumeName {
						hasUserCAMount = true
						break
					}
				}
				if !hasUserCAMount {
					t.Fatal("expected user-ca-bundle volume mount on controller container")
				}
				sidecar := findContainer(deployment, "sidecar")
				if sidecar == nil {
					t.Fatal("sidecar container not found")
				}
				for _, mount := range sidecar.VolumeMounts {
					if mount.Name == UserCABundleVolumeName {
						t.Fatal("user-ca-bundle volume mount must not be added to non-controller containers")
					}
				}
			},
		},
		{
			name:        "returns TrustedCABundleError for invalid PEM",
			esc:         baseESC,
			cm:          testUserCAConfigMap(cmName, "not pem", nil),
			deployment:  baseDeployment(),
			wantErr:     true,
			wantTrusted: true,
		},
		{
			name: "clears config when trustedCABundle is nil",
			esc: func() *v1alpha1.ExternalSecretsConfig {
				esc := baseESC.DeepCopy()
				esc.Spec.ControllerConfig.TrustedCABundle = nil
				return esc
			}(),
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{{Name: UserCABundleVolumeName}},
							Containers: []corev1.Container{{
								Name: OperandCoreControllerContainer,
								Env:  []corev1.EnvVar{{Name: SSLCertDirEnvVar, Value: SSLCertDirValue}},
								VolumeMounts: []corev1.VolumeMount{{
									Name:      UserCABundleVolumeName,
									MountPath: UserCABundleMountPath,
									ReadOnly:  true,
								}},
							}},
						},
					},
				},
			},
			assertDeploy: func(t *testing.T, deployment *appsv1.Deployment) {
				t.Helper()
				for _, vol := range deployment.Spec.Template.Spec.Volumes {
					if vol.Name == UserCABundleVolumeName {
						t.Fatal("user-ca-bundle volume should be removed")
					}
				}
				controller := findContainer(deployment, OperandCoreControllerContainer)
				if controller == nil {
					t.Fatal("controller container not found")
				}
				for _, mount := range controller.VolumeMounts {
					if mount.Name == UserCABundleVolumeName {
						t.Fatal("user-ca-bundle volume mount should be removed from controller container")
					}
				}
			},
		},
		{
			name:        "skips mount when CNO label and proxy enabled",
			esc:         baseESC,
			cm:          testUserCAConfigMap(cmName, validPEM, map[string]string{TrustedCABundleInjectLabel: "true"}),
			deployment:  baseDeployment(),
			proxyConfig: &v1alpha1.ProxyConfig{HTTPProxy: "http://proxy:8080"},
			assertDeploy: func(t *testing.T, deployment *appsv1.Deployment) {
				t.Helper()
				for _, vol := range deployment.Spec.Template.Spec.Volumes {
					if vol.Name == UserCABundleVolumeName {
						t.Fatal("user-ca-bundle volume should not be mounted when CNO inject label and proxy are set")
					}
				}
			},
			assertEvents: func(t *testing.T, r *Reconciler) {
				t.Helper()
				assertRecorderNormalEvent(t, r, trustedCABundleEventSkippedCNOProxy)
			},
		},
		{
			name:       "removes user CA config when ConfigMap is missing",
			esc:        baseESC,
			cmNotFound: true,
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{{Name: UserCABundleVolumeName}},
							Containers: []corev1.Container{{
								Name: OperandCoreControllerContainer,
								Env:  []corev1.EnvVar{{Name: SSLCertDirEnvVar, Value: SSLCertDirValue}},
								VolumeMounts: []corev1.VolumeMount{{
									Name:      UserCABundleVolumeName,
									MountPath: UserCABundleMountPath,
									ReadOnly:  true,
								}},
							}},
						},
					},
				},
			},
			wantErr:     true,
			wantTrusted: true,
			assertEvents: func(t *testing.T, r *Reconciler) {
				t.Helper()
				assertNoRecorderEvent(t, r)
			},
			assertDeploy: func(t *testing.T, deployment *appsv1.Deployment) {
				t.Helper()
				for _, vol := range deployment.Spec.Template.Spec.Volumes {
					if vol.Name == UserCABundleVolumeName {
						t.Fatal("user-ca-bundle volume should be removed when ConfigMap is missing")
					}
				}
				controller := findContainer(deployment, OperandCoreControllerContainer)
				if controller == nil {
					t.Fatal("controller container not found")
				}
				for _, mount := range controller.VolumeMounts {
					if mount.Name == UserCABundleVolumeName {
						t.Fatal("user-ca-bundle volume mount should be removed when ConfigMap is missing")
					}
				}
				for _, env := range controller.Env {
					if env.Name == SSLCertDirEnvVar {
						t.Fatal("SSL_CERT_DIR should be removed when ConfigMap is missing")
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var cached, uncached *fakes.FakeCtrlClient
			switch {
			case tt.cm != nil:
				cached, uncached = setupConfigMapClients(t, tt.cm)
			case tt.cmNotFound:
				cached = &fakes.FakeCtrlClient{}
				uncached = &fakes.FakeCtrlClient{}
				notFound := func(_ context.Context, ns types.NamespacedName, _ client.Object) error {
					return apierrors.NewNotFound(corev1.Resource("configmaps"), ns.Name)
				}
				cached.GetCalls(notFound)
				uncached.GetCalls(notFound)
			}
			r := &Reconciler{
				ctx:            context.Background(),
				CtrlClient:     cached,
				UncachedClient: uncached,
				eventRecorder:  record.NewFakeRecorder(10),
				now:            &common.Now{},
				proxyConfig:    tt.proxyConfig,
			}

			err := r.applyUserCABundleConfig(tt.deployment, tt.esc)
			if (err != nil) != tt.wantErr {
				t.Fatalf("applyUserCABundleConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantTrusted && err != nil && !common.IsUserConfigurationError(err) {
				t.Fatalf("expected TrustedCABundleError, got %v", err)
			}
			if tt.assertDeploy != nil {
				tt.assertDeploy(t, tt.deployment)
			}
			switch {
			case tt.assertEvents != nil:
				tt.assertEvents(t, r)
			case tt.wantErr:
				assertRecorderWarningEvent(t, r, trustedCABundleEventInvalidPEM)
			default:
				assertNoRecorderEvent(t, r)
			}
		})
	}
}

func TestCreateOrApplyDeploymentFromAssetReturnsTrustedCAError(t *testing.T) {
	esc := commontest.TestExternalSecretsConfig()
	esc.Spec.ControllerConfig.TrustedCABundle = &v1alpha1.ConfigMapKeyReference{
		Name: "user-ca-bundle",
		Key:  UserCABundleKeyPath,
	}
	resourceMetadata := testResourceMetadata(esc)

	t.Setenv("RELATED_IMAGE_EXTERNAL_SECRETS", commontest.TestExternalSecretsImageName)
	t.Setenv("RELATED_IMAGE_BITWARDEN_SDK_SERVER", commontest.TestBitwardenImageName)

	setupMissingUserCAClients := func() (*fakes.FakeCtrlClient, *fakes.FakeCtrlClient) {
		cached := &fakes.FakeCtrlClient{}
		uncached := &fakes.FakeCtrlClient{}
		notFound := func(_ context.Context, ns types.NamespacedName, _ client.Object) error {
			return apierrors.NewNotFound(corev1.Resource("configmaps"), ns.Name)
		}
		cached.GetCalls(notFound)
		uncached.GetCalls(notFound)
		return cached, uncached
	}

	newReconciler := func(cached, uncached *fakes.FakeCtrlClient) *Reconciler {
		r := testReconciler(t)
		r.CtrlClient = cached
		r.UncachedClient = uncached
		return r
	}

	t.Run("returns TrustedCABundleError when deployment is unchanged", func(t *testing.T) {
		t.Parallel()

		cached, uncached := setupMissingUserCAClients()
		r := newReconciler(cached, uncached)

		desired, trustedCAErr := r.getDeploymentObject(controllerDeploymentAssetName, esc, resourceMetadata)
		if trustedCAErr == nil {
			t.Fatal("expected trustedCAErr from missing ConfigMap")
		}
		if !common.IsUserConfigurationNotFound(trustedCAErr) {
			t.Fatalf("expected NotFound user configuration error, got %v", trustedCAErr)
		}

		cached.ExistsCalls(func(_ context.Context, _ types.NamespacedName, obj client.Object) (bool, error) {
			desired.DeepCopyInto(obj.(*appsv1.Deployment))
			return true, nil
		})

		err := r.createOrApplyDeploymentFromAsset(esc, controllerDeploymentAssetName, resourceMetadata, false)
		if err == nil {
			t.Fatal("createOrApplyDeploymentFromAsset() error = nil, want TrustedCABundleError")
		}
		if !common.IsUserConfigurationNotFound(err) {
			t.Fatalf("expected NotFound user configuration error, got %v", err)
		}
	})

	t.Run("returns TrustedCABundleError after create", func(t *testing.T) {
		t.Parallel()

		cached, uncached := setupMissingUserCAClients()
		r := newReconciler(cached, uncached)

		cached.ExistsCalls(doesNotExist())
		cached.CreateCalls(func(context.Context, client.Object, ...client.CreateOption) error {
			return nil
		})

		err := r.createOrApplyDeploymentFromAsset(esc, controllerDeploymentAssetName, resourceMetadata, false)
		if err == nil {
			t.Fatal("createOrApplyDeploymentFromAsset() error = nil, want TrustedCABundleError")
		}
		if !common.IsUserConfigurationNotFound(err) {
			t.Fatalf("expected NotFound user configuration error, got %v", err)
		}
	})
}

func TestParseOperandArgsEnv(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		want       []string
		wantErr    bool
		wantErrSub string // optional; defaults to "must start with --" when wantErr
	}{
		// Empty / whitespace
		{
			name: "empty",
			raw:  "",
			want: []string{},
		},
		{
			name: "whitespace only",
			raw:  "  \t  ",
			want: []string{},
		},
		{
			name: "commas only",
			raw:  ",,,",
			want: []string{},
		},

		// Happy path
		{
			name: "single flag",
			raw:  "--concurrent=5",
			want: []string{"--concurrent=5"},
		},
		{
			name: "boolean flag without equals",
			raw:  "--enable-foo",
			want: []string{"--enable-foo"},
		},
		{
			name: "boolean then valued flags",
			raw:  "--enable-foo,--enable-bar=true",
			want: []string{"--enable-foo", "--enable-bar=true"},
		},
		{
			name: "many flags",
			raw:  "--a=1,--b=2,--c=3,--d=4",
			want: []string{"--a=1", "--b=2", "--c=3", "--d=4"},
		},
		{
			name: "surrounding whitespace trimmed",
			raw:  "  --concurrent=5,--loglevel=debug  ",
			want: []string{"--concurrent=5", "--loglevel=debug"},
		},
		{
			name: "space after comma before next flag",
			raw:  "--concurrent=5, --loglevel=debug",
			want: []string{"--concurrent=5", "--loglevel=debug"},
		},
		{
			name: "tab after comma before next flag",
			raw:  "--a=1,\t--b=2",
			want: []string{"--a=1", "--b=2"},
		},
		{
			name: "newline after comma before next flag",
			raw:  "--foo=bar,\n--baz=1",
			want: []string{"--foo=bar", "--baz=1"},
		},
		{
			name: "repeated commas between flags skipped",
			raw:  "--concurrent=5,,--loglevel=debug,",
			want: []string{"--concurrent=5", "--loglevel=debug"},
		},
		{
			name: "messy whitespace and repeated commas",
			raw:  "--arg1,   --arg2,, --arg3,--arg4,",
			want: []string{"--arg1", "--arg2", "--arg3", "--arg4"},
		},
		{
			name: "leading separator tolerated",
			raw:  ",--concurrent=5",
			want: []string{"--concurrent=5"},
		},
		{
			name: "leading comma and spaces before first flag",
			raw:  "  ,  --foo=1",
			want: []string{"--foo=1"},
		},

		// Commas inside values (the reason for ,-- splitting)
		{
			name: "commas inside single flag value preserved",
			raw:  "--tls-ciphers=TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
			want: []string{
				"--tls-ciphers=TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
			},
		},
		{
			name: "commas inside value then another flag",
			raw:  "--tls-ciphers=TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,--loglevel=debug",
			want: []string{
				"--tls-ciphers=TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
				"--loglevel=debug",
			},
		},
		{
			name: "boolean flag then comma-containing value",
			raw:  "--enable-http2,--tls-ciphers=A,B,C",
			want: []string{"--enable-http2", "--tls-ciphers=A,B,C"},
		},
		{
			name: "trailing comma on single flag value stripped",
			raw:  "--tls-ciphers=A,B,",
			want: []string{"--tls-ciphers=A,B"},
		},
		{
			name: "comma without following flag stays in value",
			raw:  "--concurrent=5,not-a-flag,--loglevel=debug",
			want: []string{"--concurrent=5,not-a-flag", "--loglevel=debug"},
		},
		{
			name: "single-dash fragment stays in prior value",
			raw:  "--concurrent=5,-v,--loglevel=debug",
			want: []string{"--concurrent=5,-v", "--loglevel=debug"},
		},
		{
			name: "positional-looking text after comma stays in value",
			raw:  "--port=10250,webhook",
			want: []string{"--port=10250,webhook"},
		},
		{
			name: "equals signs inside value preserved",
			raw:  "--dns-name=foo=bar,--loglevel=debug",
			want: []string{"--dns-name=foo=bar", "--loglevel=debug"},
		},
		{
			name: "empty value after equals",
			raw:  "--concurrent=,--loglevel=debug",
			want: []string{"--concurrent=", "--loglevel=debug"},
		},
		{
			// Space-separated "--flag value" is not supported; the whole token is kept.
			name: "space inside token kept as single flag",
			raw:  "--concurrent 5",
			want: []string{"--concurrent 5"},
		},
		{
			// Known delimiter limitation: a value that itself contains ",--" is split.
			name: "value containing comma-dash-dash splits at delimiter",
			raw:  "--cert-dir=/tmp/foo,--bar/baz,--loglevel=debug",
			want: []string{"--cert-dir=/tmp/foo", "--bar/baz", "--loglevel=debug"},
		},
		{
			// Leading separator, junk after a boolean flag (dropped), real tab before
			// the next --flag, commas inside = values, and a quoted bool value.
			name: "messy mixed separators and comma values",
			raw:  ",--ARG1,arg2,,,\t,--arg3=1,2,3,--arg4=\"true\"",
			want: []string{"--ARG1", "--arg3=1,2,3", "--arg4=\"true\""},
		},
		{
			name: "junk after boolean flag dropped",
			raw:  "--enable-foo,junk,--loglevel=debug",
			want: []string{"--enable-foo", "--loglevel=debug"},
		},

		// Rejected inputs
		{
			name:    "missing dashes",
			raw:     "concurrent=5",
			wantErr: true,
		},
		{
			name:    "single dash rejected",
			raw:     "-concurrent=5",
			wantErr: true,
		},
		{
			name:    "positional token rejected",
			raw:     "webhook,--port=10250",
			wantErr: true,
		},
		{name: "positional only rejected",
			raw:     "certcontroller",
			wantErr: true,
		},
		{
			name:    "bare equals rejected",
			raw:     "=value",
			wantErr: true,
		},
		{
			name:    "bare double-dash alone rejected",
			raw:     "--",
			wantErr: true,
		},
		{
			name:    "bare double-dash mid-list rejected",
			raw:     "--concurrent=5,--,--loglevel=debug",
			wantErr: true,
		},
		{
			name:    "trailing bare double-dash rejected",
			raw:     "--concurrent=5,--",
			wantErr: true,
		},
		{
			name:    "trailing bare double-dash with spaces rejected",
			raw:     "--foo=bar, --",
			wantErr: true,
		},
		{
			name:    "only separators rejected",
			raw:     ",--,--",
			wantErr: true,
		},
		{
			name:       "empty flag name rejected",
			raw:        "--=value",
			wantErr:    true,
			wantErrSub: "must include a flag name after --",
		},
		{
			name:       "empty flag name with empty value rejected",
			raw:        "--=",
			wantErr:    true,
			wantErrSub: "must include a flag name after --",
		},
		{
			name:       "empty flag name mid-list rejected",
			raw:        "--concurrent=5,--=value,--loglevel=debug",
			wantErr:    true,
			wantErrSub: "must include a flag name after --",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOperandArgsEnv(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseOperandArgsEnv(%q) error = nil, want error", tt.raw)
				}
				if !common.IsUserConfigurationError(err) {
					t.Fatalf("parseOperandArgsEnv(%q) error = %v, want UserConfigurationError", tt.raw, err)
				}
				if !strings.Contains(err.Error(), "invalid custom arg override") {
					t.Fatalf("parseOperandArgsEnv(%q) error = %v, want message %q", tt.raw, err, "invalid custom arg override")
				}
				errSub := tt.wantErrSub
				if errSub == "" {
					errSub = "must start with --"
				}
				if !strings.Contains(err.Error(), errSub) {
					t.Fatalf("parseOperandArgsEnv(%q) error = %v, want cause mentioning %q", tt.raw, err, errSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseOperandArgsEnv(%q) unexpected error: %v", tt.raw, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseOperandArgsEnv(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestMergeContainerArgs(t *testing.T) {
	tests := []struct {
		name      string
		base      []string
		overrides []string
		want      []string
	}{
		{
			name:      "nil overrides unchanged",
			base:      []string{"--concurrent=1"},
			overrides: nil,
			want:      []string{"--concurrent=1"},
		},
		{
			name:      "override existing key",
			base:      []string{"--concurrent=1", "--metrics-addr=:8080"},
			overrides: []string{"--concurrent=5"},
			want:      []string{"--concurrent=5", "--metrics-addr=:8080"},
		},
		{
			name:      "append new key",
			base:      []string{"--concurrent=1"},
			overrides: []string{"--enable-foo=true"},
			want:      []string{"--concurrent=1", "--enable-foo=true"},
		},
		{
			name:      "boolean token replaces valued flag with same key",
			base:      []string{"--enable-foo=true", "--loglevel=info"},
			overrides: []string{"--enable-foo"},
			want:      []string{"--enable-foo", "--loglevel=info"},
		},
		{
			name:      "preserve leading positional token",
			base:      []string{"webhook", "--port=10250", "--loglevel=info"},
			overrides: []string{"--loglevel=debug", "--metrics-addr=:9090"},
			want:      []string{"webhook", "--port=10250", "--loglevel=debug", "--metrics-addr=:9090"},
		},
		{
			name:      "preserve certcontroller positional token",
			base:      []string{"certcontroller", "--crd-requeue-interval=5m"},
			overrides: []string{"--crd-requeue-interval=10m"},
			want:      []string{"certcontroller", "--crd-requeue-interval=10m"},
		},
		{
			name:      "skip non-flag overrides",
			base:      []string{"webhook", "--port=10250"},
			overrides: []string{"webhook", "--port=10251"},
			want:      []string{"webhook", "--port=10251"},
		},
		{
			name:      "all non-flag overrides leave base unchanged",
			base:      []string{"webhook", "--port=10250"},
			overrides: []string{"webhook", "certcontroller", "-v"},
			want:      []string{"webhook", "--port=10250"},
		},
		{
			name:      "empty override tokens skipped",
			base:      []string{"--concurrent=1"},
			overrides: []string{"", "--concurrent=2", ""},
			want:      []string{"--concurrent=2"},
		},
		{
			name:      "single-dash override skipped",
			base:      []string{"--loglevel=info"},
			overrides: []string{"-loglevel=debug", "--metrics-addr=:9090"},
			want:      []string{"--loglevel=info", "--metrics-addr=:9090"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeContainerArgs(tt.base, tt.overrides)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeContainerArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestApplyOperandArgsFromEnv(t *testing.T) {
	deploymentWithContainer := func(name string, args []string) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: name, Args: args}},
					},
				},
			},
		}
	}

	t.Run("unset env is no-op", func(t *testing.T) {
		dep := deploymentWithContainer(OperandCoreControllerContainer, []string{"--concurrent=1"})
		if err := applyOperandArgsFromEnv(dep, OperandCoreControllerContainer, OperandExternalSecretsArgsEnvVar); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"--concurrent=1"}
		if !reflect.DeepEqual(dep.Spec.Template.Spec.Containers[0].Args, want) {
			t.Errorf("Args = %#v, want %#v", dep.Spec.Template.Spec.Containers[0].Args, want)
		}
	})

	t.Run("empty env is no-op", func(t *testing.T) {
		t.Setenv(OperandExternalSecretsArgsEnvVar, "  ")
		dep := deploymentWithContainer(OperandCoreControllerContainer, []string{"--concurrent=1"})
		if err := applyOperandArgsFromEnv(dep, OperandCoreControllerContainer, OperandExternalSecretsArgsEnvVar); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"--concurrent=1"}
		if !reflect.DeepEqual(dep.Spec.Template.Spec.Containers[0].Args, want) {
			t.Errorf("Args = %#v, want %#v", dep.Spec.Template.Spec.Containers[0].Args, want)
		}
	})

	t.Run("overrides and appends", func(t *testing.T) {
		t.Setenv(OperandExternalSecretsArgsEnvVar, "--concurrent=5,--enable-foo=true")
		dep := deploymentWithContainer(OperandCoreControllerContainer, []string{"--concurrent=1", "--metrics-addr=:8080"})
		if err := applyOperandArgsFromEnv(dep, OperandCoreControllerContainer, OperandExternalSecretsArgsEnvVar); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"--concurrent=5", "--metrics-addr=:8080", "--enable-foo=true"}
		if !reflect.DeepEqual(dep.Spec.Template.Spec.Containers[0].Args, want) {
			t.Errorf("Args = %#v, want %#v", dep.Spec.Template.Spec.Containers[0].Args, want)
		}
	})

	t.Run("invalid entry fails", func(t *testing.T) {
		t.Setenv(OperandWebhookArgsEnvVar, "not-a-flag")
		dep := deploymentWithContainer(OperandWebhookContainer, []string{"webhook", "--port=10250"})
		err := applyOperandArgsFromEnv(dep, OperandWebhookContainer, OperandWebhookArgsEnvVar)
		if err == nil {
			t.Fatal("expected error for invalid args env value")
		}
		if !common.IsUserConfigurationError(err) {
			t.Fatalf("error = %v, want UserConfigurationError", err)
		}
		wantSubstrings := []string{
			OperandWebhookArgsEnvVar,
			"invalid custom arg override",
			`argument "not-a-flag" must start with --`,
		}
		for _, sub := range wantSubstrings {
			if !strings.Contains(err.Error(), sub) {
				t.Fatalf("error = %v, want substring %q", err, sub)
			}
		}
	})

	t.Run("positional env value fails without mutating args", func(t *testing.T) {
		t.Setenv(OperandWebhookArgsEnvVar, "webhook,--port=10251")
		original := []string{"webhook", "--port=10250"}
		dep := deploymentWithContainer(OperandWebhookContainer, append([]string(nil), original...))
		err := applyOperandArgsFromEnv(dep, OperandWebhookContainer, OperandWebhookArgsEnvVar)
		if err == nil {
			t.Fatal("expected error for positional env value")
		}
		if !common.IsUserConfigurationError(err) {
			t.Fatalf("error = %v, want UserConfigurationError", err)
		}
		if !reflect.DeepEqual(dep.Spec.Template.Spec.Containers[0].Args, original) {
			t.Errorf("Args mutated on error: %#v, want %#v", dep.Spec.Template.Spec.Containers[0].Args, original)
		}
	})

	t.Run("single dash env value fails", func(t *testing.T) {
		t.Setenv(OperandExternalSecretsArgsEnvVar, "-concurrent=5")
		dep := deploymentWithContainer(OperandCoreControllerContainer, []string{"--concurrent=1"})
		err := applyOperandArgsFromEnv(dep, OperandCoreControllerContainer, OperandExternalSecretsArgsEnvVar)
		if err == nil {
			t.Fatal("expected error for single-dash env value")
		}
		if !common.IsUserConfigurationError(err) {
			t.Fatalf("error = %v, want UserConfigurationError", err)
		}
	})

	t.Run("comma without following flag stays in value", func(t *testing.T) {
		// Commas only separate flags when they introduce the next --flag, so
		// ",bad" remains part of --concurrent's value rather than a second token.
		t.Setenv(OperandExternalSecretsArgsEnvVar, "--concurrent=5,bad")
		dep := deploymentWithContainer(OperandCoreControllerContainer, []string{"--concurrent=1"})
		if err := applyOperandArgsFromEnv(dep, OperandCoreControllerContainer, OperandExternalSecretsArgsEnvVar); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"--concurrent=5,bad"}
		if !reflect.DeepEqual(dep.Spec.Template.Spec.Containers[0].Args, want) {
			t.Errorf("Args = %#v, want %#v", dep.Spec.Template.Spec.Containers[0].Args, want)
		}
	})

	t.Run("comma-containing tls-ciphers value merges with other flags", func(t *testing.T) {
		t.Setenv(OperandWebhookArgsEnvVar, "--tls-ciphers=A,B,--loglevel=debug")
		dep := deploymentWithContainer(OperandWebhookContainer, []string{
			"webhook",
			"--port=10250",
			"--loglevel=info",
		})
		if err := applyOperandArgsFromEnv(dep, OperandWebhookContainer, OperandWebhookArgsEnvVar); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{
			"webhook",
			"--port=10250",
			"--loglevel=debug",
			"--tls-ciphers=A,B",
		}
		if !reflect.DeepEqual(dep.Spec.Template.Spec.Containers[0].Args, want) {
			t.Errorf("Args = %#v, want %#v", dep.Spec.Template.Spec.Containers[0].Args, want)
		}
	})

	t.Run("bare double-dash separator fails without mutating args", func(t *testing.T) {
		t.Setenv(OperandExternalSecretsArgsEnvVar, "--concurrent=5,--,--loglevel=debug")
		original := []string{"--concurrent=1"}
		dep := deploymentWithContainer(OperandCoreControllerContainer, append([]string(nil), original...))
		err := applyOperandArgsFromEnv(dep, OperandCoreControllerContainer, OperandExternalSecretsArgsEnvVar)
		if err == nil {
			t.Fatal("expected error for bare -- token")
		}
		if !common.IsUserConfigurationError(err) {
			t.Fatalf("error = %v, want UserConfigurationError", err)
		}
		if !reflect.DeepEqual(dep.Spec.Template.Spec.Containers[0].Args, original) {
			t.Errorf("Args mutated on error: %#v, want %#v", dep.Spec.Template.Spec.Containers[0].Args, original)
		}
	})

	t.Run("empty flag name fails without mutating args", func(t *testing.T) {
		t.Setenv(OperandExternalSecretsArgsEnvVar, "--=value")
		original := []string{"--concurrent=1"}
		dep := deploymentWithContainer(OperandCoreControllerContainer, append([]string(nil), original...))
		err := applyOperandArgsFromEnv(dep, OperandCoreControllerContainer, OperandExternalSecretsArgsEnvVar)
		if err == nil {
			t.Fatal("expected error for empty flag name")
		}
		if !common.IsUserConfigurationError(err) {
			t.Fatalf("error = %v, want UserConfigurationError", err)
		}
		if !strings.Contains(err.Error(), "must include a flag name after --") {
			t.Fatalf("error = %v, want empty flag name message", err)
		}
		if !reflect.DeepEqual(dep.Spec.Template.Spec.Containers[0].Args, original) {
			t.Errorf("Args mutated on error: %#v, want %#v", dep.Spec.Template.Spec.Containers[0].Args, original)
		}
	})

	t.Run("missing container fails", func(t *testing.T) {
		t.Setenv(OperandExternalSecretsArgsEnvVar, "--concurrent=5")
		dep := deploymentWithContainer("other", []string{"--concurrent=1"})
		if err := applyOperandArgsFromEnv(dep, OperandCoreControllerContainer, OperandExternalSecretsArgsEnvVar); err == nil {
			t.Fatal("expected error for missing container")
		}
	})

	t.Run("empty containers list fails", func(t *testing.T) {
		t.Setenv(OperandExternalSecretsArgsEnvVar, "--concurrent=5")
		dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		if err := applyOperandArgsFromEnv(dep, OperandCoreControllerContainer, OperandExternalSecretsArgsEnvVar); err == nil {
			t.Fatal("expected error for empty containers list")
		}
	})

	t.Run("bitwarden args applied onto empty base", func(t *testing.T) {
		t.Setenv(OperandBitwardenSDKServerArgsEnvVar, "--port=9999")
		dep := deploymentWithContainer(OperandBitwardenContainer, nil)
		if err := applyOperandArgsFromEnv(dep, OperandBitwardenContainer, OperandBitwardenSDKServerArgsEnvVar); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"--port=9999"}
		if !reflect.DeepEqual(dep.Spec.Template.Spec.Containers[0].Args, want) {
			t.Errorf("Args = %#v, want %#v", dep.Spec.Template.Spec.Containers[0].Args, want)
		}
	})
}

func TestGetDeploymentObjectOperandArgsFromEnv(t *testing.T) {
	t.Setenv(externalsecretsImageEnvVarName, commontest.TestExternalSecretsImageName)
	t.Setenv(bitwardenImageEnvVarName, commontest.TestBitwardenImageName)

	esc := commontest.TestExternalSecretsConfig()
	resourceMetadata := testResourceMetadata(esc)
	r := testReconciler(t)

	t.Run("controller overrides concurrent", func(t *testing.T) {
		t.Setenv(OperandExternalSecretsArgsEnvVar, "--concurrent=5")
		dep, err := r.getDeploymentObject(controllerDeploymentAssetName, esc, resourceMetadata)
		if err != nil {
			t.Fatalf("getDeploymentObject() unexpected error: %v", err)
		}
		args := containerArgsByName(dep, OperandCoreControllerContainer)
		if !slices.Contains(args, "--concurrent=5") {
			t.Errorf("expected --concurrent=5 in args, got %#v", args)
		}
		if slices.Contains(args, "--concurrent=1") {
			t.Errorf("default --concurrent=1 should have been overridden, got %#v", args)
		}
	})

	t.Run("webhook preserves positional token", func(t *testing.T) {
		t.Setenv(OperandWebhookArgsEnvVar, "--loglevel=debug")
		dep, err := r.getDeploymentObject(webhookDeploymentAssetName, esc, resourceMetadata)
		if err != nil {
			t.Fatalf("getDeploymentObject() unexpected error: %v", err)
		}
		args := containerArgsByName(dep, OperandWebhookContainer)
		if len(args) == 0 || args[0] != "webhook" {
			t.Errorf("expected leading webhook token, got %#v", args)
		}
		if !slices.Contains(args, "--loglevel=debug") {
			t.Errorf("expected --loglevel=debug in args, got %#v", args)
		}
	})

	t.Run("cert controller preserves positional token", func(t *testing.T) {
		t.Setenv(OperandCertControllerArgsEnvVar, "--crd-requeue-interval=10m")
		dep, err := r.getDeploymentObject(certControllerDeploymentAssetName, esc, resourceMetadata)
		if err != nil {
			t.Fatalf("getDeploymentObject() unexpected error: %v", err)
		}
		args := containerArgsByName(dep, OperandCertControllerContainer)
		if len(args) == 0 || args[0] != "certcontroller" {
			t.Errorf("expected leading certcontroller token, got %#v", args)
		}
		if !slices.Contains(args, "--crd-requeue-interval=10m") {
			t.Errorf("expected overridden interval in args, got %#v", args)
		}
	})

	t.Run("bitwarden applies args", func(t *testing.T) {
		escWithBW := esc.DeepCopy()
		escWithBW.Spec.Plugins.BitwardenSecretManagerProvider = &v1alpha1.BitwardenSecretManagerProvider{
			Mode: v1alpha1.Enabled,
			SecretRef: &v1alpha1.SecretReference{
				Name: "bitwarden-certs",
			},
		}
		t.Setenv(OperandBitwardenSDKServerArgsEnvVar, "--enable-debug=true")
		dep, err := r.getDeploymentObject(bitwardenDeploymentAssetName, escWithBW, resourceMetadata)
		if err != nil {
			t.Fatalf("getDeploymentObject() unexpected error: %v", err)
		}
		args := containerArgsByName(dep, OperandBitwardenContainer)
		if !reflect.DeepEqual(args, []string{"--enable-debug=true"}) {
			t.Errorf("Args = %#v, want [--enable-debug=true]", args)
		}
	})

	t.Run("invalid env fails", func(t *testing.T) {
		t.Setenv(OperandExternalSecretsArgsEnvVar, "bad-arg")
		_, err := r.getDeploymentObject(controllerDeploymentAssetName, esc, resourceMetadata)
		if err == nil {
			t.Fatal("expected error for invalid operand args env")
		}
		if !common.IsUserConfigurationError(err) {
			t.Fatalf("error = %v, want UserConfigurationError", err)
		}
		wantSubstrings := []string{
			OperandExternalSecretsArgsEnvVar,
			"invalid custom arg override",
			`argument "bad-arg" must start with --`,
		}
		for _, sub := range wantSubstrings {
			if !strings.Contains(err.Error(), sub) {
				t.Fatalf("error = %v, want substring %q", err, sub)
			}
		}
	})

	t.Run("webhook positional env fails", func(t *testing.T) {
		t.Setenv(OperandWebhookArgsEnvVar, "webhook,--port=10251")
		_, err := r.getDeploymentObject(webhookDeploymentAssetName, esc, resourceMetadata)
		if err == nil {
			t.Fatal("expected error for positional webhook env value")
		}
		if !common.IsUserConfigurationError(err) {
			t.Fatalf("error = %v, want UserConfigurationError", err)
		}
	})

	t.Run("cert controller single dash env fails", func(t *testing.T) {
		t.Setenv(OperandCertControllerArgsEnvVar, "-crd-requeue-interval=10m")
		_, err := r.getDeploymentObject(certControllerDeploymentAssetName, esc, resourceMetadata)
		if err == nil {
			t.Fatal("expected error for single-dash cert-controller env value")
		}
		if !common.IsUserConfigurationError(err) {
			t.Fatalf("error = %v, want UserConfigurationError", err)
		}
	})
}

func containerArgsByName(dep *appsv1.Deployment, name string) []string {
	for i := range dep.Spec.Template.Spec.Containers {
		if dep.Spec.Template.Spec.Containers[i].Name == name {
			return dep.Spec.Template.Spec.Containers[i].Args
		}
	}
	return nil
}
