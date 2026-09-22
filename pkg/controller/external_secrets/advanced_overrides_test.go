package external_secrets

import (
	"encoding/json"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

func rawJSON(v interface{}) *runtime.RawExtension {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return &runtime.RawExtension{Raw: data}
}

func TestValidateAndApplyAdvancedOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		overrides     *runtime.RawExtension
		containerName string
		wantErr       bool
		isUserErr     bool
		validate      func(t *testing.T, dep *appsv1.Deployment)
	}{
		{
			name:          "nil overrides is a no-op",
			overrides:     nil,
			containerName: OperandCoreControllerContainer,
			wantErr:       false,
		},
		{
			name:          "empty raw is a no-op",
			overrides:     &runtime.RawExtension{Raw: []byte{}},
			containerName: OperandCoreControllerContainer,
			wantErr:       false,
		},
		{
			name: "valid affinity override",
			overrides: rawJSON(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"affinity": map[string]interface{}{
								"podAntiAffinity": map[string]interface{}{
									"preferredDuringSchedulingIgnoredDuringExecution": []map[string]interface{}{
										{
											"weight": 100,
											"podAffinityTerm": map[string]interface{}{
												"topologyKey": "kubernetes.io/hostname",
											},
										},
									},
								},
							},
						},
					},
				},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       false,
			validate: func(t *testing.T, dep *appsv1.Deployment) {
				if dep.Spec.Template.Spec.Affinity == nil {
					t.Error("expected affinity to be set")
				}
				if dep.Spec.Template.Spec.Affinity.PodAntiAffinity == nil {
					t.Error("expected podAntiAffinity to be set")
				}
			},
		},
		{
			name: "valid nodeSelector override",
			overrides: rawJSON(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"nodeSelector": map[string]string{
								"zone": "us-east-1a",
							},
						},
					},
				},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       false,
			validate: func(t *testing.T, dep *appsv1.Deployment) {
				if dep.Spec.Template.Spec.NodeSelector == nil {
					t.Fatal("expected nodeSelector to be set")
				}
				if dep.Spec.Template.Spec.NodeSelector["zone"] != "us-east-1a" {
					t.Errorf("expected zone=us-east-1a, got %s", dep.Spec.Template.Spec.NodeSelector["zone"])
				}
			},
		},
		{
			name: "valid tolerations override",
			overrides: rawJSON(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"tolerations": []map[string]interface{}{
								{
									"key":      "test-taint",
									"operator": "Exists",
									"effect":   "NoSchedule",
								},
							},
						},
					},
				},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       false,
			validate: func(t *testing.T, dep *appsv1.Deployment) {
				if len(dep.Spec.Template.Spec.Tolerations) != 1 {
					t.Fatalf("expected 1 toleration, got %d", len(dep.Spec.Template.Spec.Tolerations))
				}
				if dep.Spec.Template.Spec.Tolerations[0].Key != "test-taint" {
					t.Errorf("expected key=test-taint, got %s", dep.Spec.Template.Spec.Tolerations[0].Key)
				}
			},
		},
		{
			name: "valid container args override",
			overrides: rawJSON(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{
									"name": OperandCoreControllerContainer,
									"args": []string{"--concurrent=20", "--client-burst=200"},
								},
							},
						},
					},
				},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       false,
			validate: func(t *testing.T, dep *appsv1.Deployment) {
				for _, c := range dep.Spec.Template.Spec.Containers {
					if c.Name == OperandCoreControllerContainer {
						if len(c.Args) != 2 {
							t.Fatalf("expected 2 args, got %d: %v", len(c.Args), c.Args)
						}
						if c.Args[0] != "--concurrent=20" {
							t.Errorf("expected --concurrent=20, got %s", c.Args[0])
						}
						return
					}
				}
				t.Error("core controller container not found")
			},
		},
		{
			name: "valid container resources override",
			overrides: rawJSON(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{
									"name": OperandCoreControllerContainer,
									"resources": map[string]interface{}{
										"requests": map[string]string{
											"cpu":    "200m",
											"memory": "256Mi",
										},
									},
								},
							},
						},
					},
				},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       false,
			validate: func(t *testing.T, dep *appsv1.Deployment) {
				for _, c := range dep.Spec.Template.Spec.Containers {
					if c.Name == OperandCoreControllerContainer {
						if c.Resources.Requests.Cpu().Cmp(resource.MustParse("200m")) != 0 {
							t.Errorf("expected cpu request 200m, got %s", c.Resources.Requests.Cpu().String())
						}
						return
					}
				}
				t.Error("core controller container not found")
			},
		},
		{
			name: "disallowed path: volumes",
			overrides: rawJSON(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"volumes": []map[string]interface{}{
								{"name": "evil-volume", "emptyDir": map[string]interface{}{}},
							},
						},
					},
				},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			isUserErr:     true,
		},
		{
			name: "disallowed path: initContainers",
			overrides: rawJSON(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"initContainers": []map[string]interface{}{
								{"name": "evil-init", "image": "busybox"},
							},
						},
					},
				},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			isUserErr:     true,
		},
		{
			name: "disallowed path: spec.replicas",
			overrides: rawJSON(map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			isUserErr:     true,
		},
		{
			name: "disallowed path: spec.template.metadata",
			overrides: rawJSON(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]string{"evil": "label"},
						},
					},
				},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			isUserErr:     true,
		},
		{
			name: "disallowed container field: env",
			overrides: rawJSON(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{
									"name": OperandCoreControllerContainer,
									"env":  []map[string]interface{}{{"name": "BAD", "value": "val"}},
								},
							},
						},
					},
				},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			isUserErr:     true,
		},
		{
			name: "disallowed container field: image",
			overrides: rawJSON(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{
									"name":  OperandCoreControllerContainer,
									"image": "evil-image:latest",
								},
							},
						},
					},
				},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			isUserErr:     true,
		},
		{
			name: "disallowed container field: ports",
			overrides: rawJSON(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{
									"name":  OperandCoreControllerContainer,
									"ports": []map[string]interface{}{{"containerPort": 9999}},
								},
							},
						},
					},
				},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			isUserErr:     true,
		},
		{
			name:          "malformed JSON",
			overrides:     &runtime.RawExtension{Raw: []byte(`{not valid json}`)},
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			isUserErr:     true,
		},
		{
			name: "disallowed pod spec: serviceAccountName",
			overrides: rawJSON(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"serviceAccountName": "evil-sa",
						},
					},
				},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			isUserErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dep := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: OperandCoreControllerContainer,
									Args: []string{"--concurrent=1"},
								},
							},
						},
					},
				},
			}

			err := validateAndApplyAdvancedOverrides(dep, tt.overrides, tt.containerName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAndApplyAdvancedOverrides() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.isUserErr && err != nil {
				if !common.IsUserConfigurationError(err) {
					t.Errorf("expected UserConfigurationError, got %T: %v", err, err)
				}
			}
			if tt.validate != nil && err == nil {
				tt.validate(t, dep)
			}
		})
	}
}

func TestReassertLeaderElection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		replicas   int32
		initialArg bool
		expectArg  bool
	}{
		{
			name:       "replicas=2, no initial arg → adds leader election",
			replicas:   2,
			initialArg: false,
			expectArg:  true,
		},
		{
			name:       "replicas=2, already has arg → keeps it",
			replicas:   2,
			initialArg: true,
			expectArg:  true,
		},
		{
			name:       "replicas=1, has arg → removes it",
			replicas:   1,
			initialArg: true,
			expectArg:  false,
		},
		{
			name:       "replicas=1, no arg → stays absent",
			replicas:   1,
			initialArg: false,
			expectArg:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := []string{"--concurrent=1", "--metrics-addr=:8080"}
			if tt.initialArg {
				args = append(args, LeaderElectionArg)
			}
			dep := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: OperandCoreControllerContainer, Args: args},
							},
						},
					},
				},
			}

			reassertLeaderElection(dep, OperandCoreControllerContainer, tt.replicas)

			foundArg := false
			for _, a := range dep.Spec.Template.Spec.Containers[0].Args {
				if a == LeaderElectionArg {
					foundArg = true
				}
			}
			if foundArg != tt.expectArg {
				t.Errorf("reassertLeaderElection() leader election arg present = %v, want %v; args = %v",
					foundArg, tt.expectArg, dep.Spec.Template.Spec.Containers[0].Args)
			}
		})
	}
}

func TestGetCoreControllerReplicas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		esc      *v1alpha1.ExternalSecretsConfig
		expected int32
	}{
		{
			name: "no componentConfigs",
			esc: &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{},
			},
			expected: 1,
		},
		{
			name: "componentConfigs without core controller",
			esc: &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{
					ControllerConfig: v1alpha1.ControllerConfig{
						ComponentConfigs: []v1alpha1.ComponentConfig{
							{ComponentName: v1alpha1.Webhook},
						},
					},
				},
			},
			expected: 1,
		},
		{
			name: "core controller with replicas=3",
			esc: &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{
					ControllerConfig: v1alpha1.ControllerConfig{
						ComponentConfigs: []v1alpha1.ComponentConfig{
							{
								ComponentName: v1alpha1.CoreController,
								DeploymentConfigs: &v1alpha1.DeploymentConfig{
									Replicas: ptr.To(int32(3)),
								},
							},
						},
					},
				},
			},
			expected: 3,
		},
		{
			name: "core controller with nil replicas",
			esc: &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{
					ControllerConfig: v1alpha1.ControllerConfig{
						ComponentConfigs: []v1alpha1.ComponentConfig{
							{
								ComponentName:     v1alpha1.CoreController,
								DeploymentConfigs: &v1alpha1.DeploymentConfig{},
							},
						},
					},
				},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getCoreControllerReplicas(tt.esc)
			if got != tt.expected {
				t.Errorf("getCoreControllerReplicas() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestApplyUserDeploymentConfigsWithReplicas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		replicas         *int32
		expectedReplicas *int32
	}{
		{
			name:             "replicas=2 sets deployment replicas",
			replicas:         ptr.To(int32(2)),
			expectedReplicas: ptr.To(int32(2)),
		},
		{
			name:             "replicas=nil leaves deployment replicas unchanged",
			replicas:         nil,
			expectedReplicas: nil,
		},
		{
			name:             "replicas=1 sets deployment replicas to 1",
			replicas:         ptr.To(int32(1)),
			expectedReplicas: ptr.To(int32(1)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := testReconciler(t)
			dep := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: OperandCoreControllerContainer, Args: []string{"--concurrent=1"}},
							},
						},
					},
				},
			}

			var deployConfigs *v1alpha1.DeploymentConfig
			if tt.replicas != nil {
				deployConfigs = &v1alpha1.DeploymentConfig{Replicas: tt.replicas}
			}

			esc := &v1alpha1.ExternalSecretsConfig{
				Spec: v1alpha1.ExternalSecretsConfigSpec{
					ControllerConfig: v1alpha1.ControllerConfig{
						ComponentConfigs: []v1alpha1.ComponentConfig{
							{
								ComponentName:     v1alpha1.CoreController,
								DeploymentConfigs: deployConfigs,
							},
						},
					},
				},
			}

			err := r.applyUserDeploymentConfigs(dep, esc, controllerDeploymentAssetName)
			if err != nil {
				t.Fatalf("applyUserDeploymentConfigs() error = %v", err)
			}

			if tt.expectedReplicas == nil {
				if dep.Spec.Replicas != nil {
					t.Errorf("expected nil replicas, got %d", *dep.Spec.Replicas)
				}
			} else {
				if dep.Spec.Replicas == nil {
					t.Errorf("expected replicas=%d, got nil", *tt.expectedReplicas)
				} else if *dep.Spec.Replicas != *tt.expectedReplicas {
					t.Errorf("expected replicas=%d, got %d", *tt.expectedReplicas, *dep.Spec.Replicas)
				}
			}
		})
	}
}
