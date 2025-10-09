package external_secrets

import (
	"context"
	"reflect"
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

func TestUpdateProxyConfiguration(t *testing.T) {
	// Expected trusted CA bundle volume
	expectedTrustedCAVolume := corev1.Volume{
		Name: "trusted-ca-bundle",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "external-secrets-trusted-ca-bundle",
				},
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
												Name: "external-secrets-trusted-ca-bundle",
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
			// Set up environment variables
			for key, value := range tt.olmEnvVars {
				t.Setenv(key, value)
			}

			r := &Reconciler{
				esm: tt.externalSecretsManager,
			}

			err := r.updateProxyConfiguration(tt.deployment, tt.externalSecretsConfig)
			if err != nil {
				t.Errorf("updateProxyConfiguration() error = %v", err)
				return
			}

			validateEnvironmentVariables(t, tt.deployment, tt.expectedContainerEnvVars)
			validateVolumes(t, tt.deployment, tt.expectedVolumes)
			validateVolumeMounts(t, tt.deployment, tt.expectedVolumeMounts)
		})
	}
}

// validateEnvironmentVariables validates that containers have expected environment variables
func validateEnvironmentVariables(t *testing.T, deployment *appsv1.Deployment, expectedContainerEnvVars map[string][]corev1.EnvVar) {
	for containerName, expectedEnvVars := range expectedContainerEnvVars {
		container := findContainer(deployment, containerName)
		if container == nil {
			t.Errorf("Container %s not found in deployment", containerName)
			return
		}
		if !reflect.DeepEqual(container.Env, expectedEnvVars) {
			t.Errorf("Container %s environment variables mismatch.\nExpected: %+v\nActual: %+v",
				containerName, expectedEnvVars, container.Env)
		}
	}
}

// validateVolumes validates that deployment has expected volumes
func validateVolumes(t *testing.T, deployment *appsv1.Deployment, expectedVolumes []corev1.Volume) {
	if len(expectedVolumes) == 0 {
		// Verify no trusted CA bundle volume was added
		for _, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.Name == trustedCABundleVolumeName {
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

// validateVolumeMounts validates that containers have expected volume mounts
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
		if len(expectedMounts) > 0 && expectedMounts[0].Name == trustedCABundleVolumeName {
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

// findContainer finds a container by name in the deployment
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

// filterTrustedCAMounts filters volume mounts to only include trusted CA bundle mounts
func filterTrustedCAMounts(volumeMounts []corev1.VolumeMount) []corev1.VolumeMount {
	var trustedCAMounts []corev1.VolumeMount
	for _, mount := range volumeMounts {
		if mount.Name == trustedCABundleVolumeName {
			trustedCAMounts = append(trustedCAMounts, mount)
		}
	}
	return trustedCAMounts
}

// filterNonTrustedCAMounts filters volume mounts to exclude trusted CA bundle mounts
func filterNonTrustedCAMounts(volumeMounts []corev1.VolumeMount) []corev1.VolumeMount {
	var nonTrustedCAMounts []corev1.VolumeMount
	for _, mount := range volumeMounts {
		if mount.Name != trustedCABundleVolumeName {
			nonTrustedCAMounts = append(nonTrustedCAMounts, mount)
		}
	}
	return nonTrustedCAMounts
}
