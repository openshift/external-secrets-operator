package external_secrets

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

func TestApplyAdvancedOverrides(t *testing.T) {
	t.Parallel()

	baseDeployment := func() *appsv1.Deployment {
		return &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: OperandCoreControllerContainer,
								Args: []string{"--concurrent=1", "--metrics-addr=:8080", "--loglevel=info"},
							},
						},
					},
				},
			},
		}
	}

	rawJSON := func(v any) *runtime.RawExtension {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}
		return &runtime.RawExtension{Raw: data}
	}

	tests := []struct {
		name          string
		deployment    *appsv1.Deployment
		overrides     *runtime.RawExtension
		componentName v1alpha1.ComponentName
		containerName string
		wantErr       bool
		errSubstring  string
		assertDeploy  func(t *testing.T, d *appsv1.Deployment)
	}{
		{
			name:          "nil overrides is no-op",
			deployment:    baseDeployment(),
			overrides:     nil,
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				if len(d.Spec.Template.Spec.Containers[0].Args) != 3 {
					t.Errorf("expected 3 args unchanged, got %v", d.Spec.Template.Spec.Containers[0].Args)
				}
			},
		},
		{
			name:       "valid args override replaces args list",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{"name": OperandCoreControllerContainer, "args": []string{"--concurrent=20", "--client-burst=200"}},
							},
						},
					},
				},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				args := d.Spec.Template.Spec.Containers[0].Args
				if !slices.Contains(args, "--concurrent=20") {
					t.Errorf("expected --concurrent=20 in args, got %v", args)
				}
				if !slices.Contains(args, "--client-burst=200") {
					t.Errorf("expected --client-burst=200 in args, got %v", args)
				}
				if slices.Contains(args, "--concurrent=1") {
					t.Errorf("expected --concurrent=1 to be replaced, got %v", args)
				}
				if slices.Contains(args, "--metrics-addr=:8080") {
					t.Errorf("expected --metrics-addr=:8080 to be dropped, got %v", args)
				}
			},
		},
		{
			name: "valid args override with replicas>1 re-asserts leader election",
			deployment: func() *appsv1.Deployment {
				d := baseDeployment()
				d.Spec.Replicas = ptr.To(int32(2))
				d.Spec.Template.Spec.Containers[0].Args = append(d.Spec.Template.Spec.Containers[0].Args, LeaderElectionArg)
				return d
			}(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{"name": OperandCoreControllerContainer, "args": []string{"--concurrent=20"}},
							},
						},
					},
				},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				args := d.Spec.Template.Spec.Containers[0].Args
				if !slices.Contains(args, LeaderElectionArg) {
					t.Errorf("expected leader election arg to be re-asserted when replicas=2, got %v", args)
				}
				if !slices.Contains(args, "--concurrent=20") {
					t.Errorf("expected --concurrent=20 in args, got %v", args)
				}
			},
		},
		{
			name:       "valid args override with replicas=1 does not re-assert leader election",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{"name": OperandCoreControllerContainer, "args": []string{"--concurrent=20"}},
							},
						},
					},
				},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				args := d.Spec.Template.Spec.Containers[0].Args
				if slices.Contains(args, LeaderElectionArg) {
					t.Errorf("expected no leader election arg when replicas=1, got %v", args)
				}
			},
		},
		{
			name:       "valid affinity override",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"affinity": map[string]any{
								"podAntiAffinity": map[string]any{
									"preferredDuringSchedulingIgnoredDuringExecution": []map[string]any{
										{"weight": 100, "podAffinityTerm": map[string]any{
											"topologyKey": "kubernetes.io/hostname",
											"labelSelector": map[string]any{
												"matchLabels": map[string]string{"app": "external-secrets"},
											},
										}},
									},
								},
							},
						},
					},
				},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				if d.Spec.Template.Spec.Affinity == nil || d.Spec.Template.Spec.Affinity.PodAntiAffinity == nil {
					t.Fatal("expected pod anti-affinity to be set")
				}
			},
		},
		{
			name:       "valid tolerations override",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"tolerations": []map[string]any{
								{"key": "node-role.kubernetes.io/infra", "operator": "Exists", "effect": "NoSchedule"},
							},
						},
					},
				},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				if len(d.Spec.Template.Spec.Tolerations) != 1 {
					t.Fatalf("expected 1 toleration, got %d", len(d.Spec.Template.Spec.Tolerations))
				}
				if d.Spec.Template.Spec.Tolerations[0].Key != "node-role.kubernetes.io/infra" {
					t.Errorf("unexpected toleration key: %s", d.Spec.Template.Spec.Tolerations[0].Key)
				}
			},
		},
		{
			name:       "valid nodeSelector override",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"nodeSelector": map[string]string{"disktype": "ssd"},
						},
					},
				},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				if d.Spec.Template.Spec.NodeSelector["disktype"] != "ssd" {
					t.Errorf("expected nodeSelector disktype=ssd, got %v", d.Spec.Template.Spec.NodeSelector)
				}
			},
		},
		{
			name:       "valid topologySpreadConstraints override",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"topologySpreadConstraints": []map[string]any{
								{"maxSkew": 1, "topologyKey": "topology.kubernetes.io/zone", "whenUnsatisfiable": "ScheduleAnyway"},
							},
						},
					},
				},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				if len(d.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
					t.Fatalf("expected 1 topology constraint, got %d", len(d.Spec.Template.Spec.TopologySpreadConstraints))
				}
			},
		},
		{
			name:       "valid resources override",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{"name": OperandCoreControllerContainer, "resources": map[string]any{
									"requests": map[string]string{"cpu": "100m", "memory": "128Mi"},
									"limits":   map[string]string{"cpu": "500m", "memory": "512Mi"},
								}},
							},
						},
					},
				},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				res := d.Spec.Template.Spec.Containers[0].Resources
				if res.Requests.Cpu().Cmp(resource.MustParse("100m")) != 0 {
					t.Errorf("expected CPU request 100m, got %v", res.Requests.Cpu())
				}
			},
		},
		{
			name: "webhook component does not re-assert leader election",
			deployment: func() *appsv1.Deployment {
				d := baseDeployment()
				d.Spec.Replicas = ptr.To(int32(2))
				d.Spec.Template.Spec.Containers[0].Name = OperandWebhookContainer
				return d
			}(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{"name": OperandWebhookContainer, "args": []string{"webhook", "--port=10250"}},
							},
						},
					},
				},
			}),
			componentName: v1alpha1.Webhook,
			containerName: OperandWebhookContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				args := d.Spec.Template.Spec.Containers[0].Args
				if slices.Contains(args, LeaderElectionArg) {
					t.Errorf("webhook should not have leader election arg, got %v", args)
				}
			},
		},

		// --- Disallowed paths ---
		{
			name:       "reject spec.replicas",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"replicas": 3},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "spec.replicas",
		},
		{
			name:       "reject spec.selector",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"selector": map[string]any{"matchLabels": map[string]string{"app": "evil"}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "spec.selector",
		},
		{
			name:       "reject spec.revisionHistoryLimit",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"revisionHistoryLimit": 5},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "revisionHistoryLimit",
		},
		{
			name:       "reject spec.template.metadata",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"metadata": map[string]any{"labels": map[string]string{"evil": "true"}}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "spec.template.metadata",
		},
		{
			name:       "reject volumes",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"volumes": []map[string]any{{"name": "evil"}},
				}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "volumes",
		},
		{
			name:       "reject initContainers",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"initContainers": []map[string]any{{"name": "evil", "image": "evil:latest"}},
				}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "initContainers",
		},
		{
			name:       "reject container env",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"name": OperandCoreControllerContainer, "env": []map[string]any{{"name": "EVIL", "value": "true"}}},
					},
				}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "containers[].env",
		},
		{
			name:       "reject container image",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"name": OperandCoreControllerContainer, "image": "evil:latest"},
					},
				}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "containers[].image",
		},
		{
			name:       "reject container ports",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"name": OperandCoreControllerContainer, "ports": []map[string]any{{"containerPort": 9999}}},
					},
				}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "containers[].ports",
		},
		{
			name:       "reject container securityContext",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"name": OperandCoreControllerContainer, "securityContext": map[string]any{"privileged": true}},
					},
				}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "containers[].securityContext",
		},
		{
			name:       "reject container volumeMounts",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"name": OperandCoreControllerContainer, "volumeMounts": []map[string]any{{"name": "evil", "mountPath": "/evil"}}},
					},
				}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "containers[].volumeMounts",
		},
		{
			name:       "reject container command",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"name": OperandCoreControllerContainer, "command": []string{"/bin/sh", "-c", "evil"}},
					},
				}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "containers[].command",
		},
		{
			name:       "reject wrong container name",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"name": "evil-sidecar", "args": []string{"--hack"}},
					},
				}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "not a valid target",
		},
		{
			name:       "reject container without name",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"args": []string{"--concurrent=20"}},
					},
				}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "container name is required",
		},
		{
			name:          "reject invalid JSON",
			deployment:    baseDeployment(),
			overrides:     &runtime.RawExtension{Raw: []byte(`{not valid json}`)},
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "invalid JSON",
		},
		{
			name:       "reject pod-level serviceAccountName",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"serviceAccountName": "evil",
				}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "serviceAccountName",
		},
		{
			name:       "reject ephemeralContainers",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"ephemeralContainers": []map[string]any{{"name": "debug", "image": "busybox"}},
				}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "ephemeralContainers",
		},
		{
			name:       "reject invalid fields",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"bar": map[string]any{"foo": map[string]any{"spec": map[string]any{
					"foo": "evil:latest",
				}}},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "unknown field",
		},
		{
			name:       "comprehensive valid override with all allowlisted fields",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"affinity": map[string]any{
								"nodeAffinity": map[string]any{
									"requiredDuringSchedulingIgnoredDuringExecution": map[string]any{
										"nodeSelectorTerms": []map[string]any{
											{"matchExpressions": []map[string]any{
												{"key": "kubernetes.io/arch", "operator": "In", "values": []string{"amd64"}},
											}},
										},
									},
								},
							},
							"tolerations": []map[string]any{
								{"key": "dedicated", "operator": "Equal", "value": "eso", "effect": "NoSchedule"},
							},
							"nodeSelector": map[string]string{"tier": "eso"},
							"topologySpreadConstraints": []map[string]any{
								{"maxSkew": 1, "topologyKey": "topology.kubernetes.io/zone", "whenUnsatisfiable": "DoNotSchedule"},
							},
							"containers": []map[string]any{
								{
									"name": OperandCoreControllerContainer,
									"args": []string{"--concurrent=20", "--client-burst=200", "--client-qps=100"},
									"resources": map[string]any{
										"requests": map[string]string{"cpu": "200m", "memory": "256Mi"},
									},
								},
							},
						},
					},
				},
			}),
			componentName: v1alpha1.CoreController,
			containerName: OperandCoreControllerContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				if d.Spec.Template.Spec.Affinity == nil {
					t.Error("expected affinity to be set")
				}
				if len(d.Spec.Template.Spec.Tolerations) != 1 {
					t.Errorf("expected 1 toleration, got %d", len(d.Spec.Template.Spec.Tolerations))
				}
				if d.Spec.Template.Spec.NodeSelector["tier"] != "eso" {
					t.Errorf("expected nodeSelector tier=eso, got %v", d.Spec.Template.Spec.NodeSelector)
				}
				if len(d.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
					t.Errorf("expected 1 topology constraint, got %d", len(d.Spec.Template.Spec.TopologySpreadConstraints))
				}
				c := d.Spec.Template.Spec.Containers[0]
				if !slices.Contains(c.Args, "--concurrent=20") {
					t.Errorf("expected --concurrent=20 in args, got %v", c.Args)
				}
				if c.Resources.Requests.Cpu().Cmp(resource.MustParse("200m")) != 0 {
					t.Errorf("expected CPU request 200m, got %v", c.Resources.Requests.Cpu())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := applyAdvancedOverrides(tt.deployment, tt.overrides, tt.componentName, tt.containerName)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !common.IsUserConfigurationError(err) {
					t.Errorf("expected UserConfigurationError, got %v", err)
				}
				if tt.errSubstring != "" && !strings.Contains(err.Error(), tt.errSubstring) {
					t.Errorf("expected error containing %q, got %q", tt.errSubstring, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.assertDeploy != nil {
				tt.assertDeploy(t, tt.deployment)
			}
		})
	}
}

func TestApplyAdvancedOverridesViaReconciler(t *testing.T) {
	t.Parallel()

	t.Run("advancedOverrides applied through applyUserDeploymentConfigs", func(t *testing.T) {
		t.Parallel()
		r := testReconciler(t)
		deployment := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: OperandCoreControllerContainer,
								Args: []string{"--concurrent=1", "--metrics-addr=:8080"},
							},
						},
					},
				},
			},
		}

		overrideJSON, _ := json.Marshal(map[string]any{
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"nodeSelector": map[string]string{"disktype": "ssd"},
					},
				},
			},
		})

		esc := &v1alpha1.ExternalSecretsConfig{
			Spec: v1alpha1.ExternalSecretsConfigSpec{
				ControllerConfig: v1alpha1.ControllerConfig{
					ComponentConfigs: []v1alpha1.ComponentConfig{
						{
							ComponentName:     v1alpha1.CoreController,
							AdvancedOverrides: &runtime.RawExtension{Raw: overrideJSON},
						},
					},
				},
			},
		}

		if err := r.applyUserDeploymentConfigs(deployment, esc, controllerDeploymentAssetName); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if deployment.Spec.Template.Spec.NodeSelector["disktype"] != "ssd" {
			t.Errorf("expected nodeSelector to be applied, got %v", deployment.Spec.Template.Spec.NodeSelector)
		}
	})

	t.Run("invalid advancedOverrides returns error through applyUserDeploymentConfigs", func(t *testing.T) {
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

		overrideJSON, _ := json.Marshal(map[string]any{
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"volumes": []map[string]any{{"name": "evil"}},
					},
				},
			},
		})

		esc := &v1alpha1.ExternalSecretsConfig{
			Spec: v1alpha1.ExternalSecretsConfigSpec{
				ControllerConfig: v1alpha1.ControllerConfig{
					ComponentConfigs: []v1alpha1.ComponentConfig{
						{
							ComponentName:     v1alpha1.CoreController,
							AdvancedOverrides: &runtime.RawExtension{Raw: overrideJSON},
						},
					},
				},
			},
		}

		err := r.applyUserDeploymentConfigs(deployment, esc, controllerDeploymentAssetName)
		if err == nil {
			t.Fatal("expected error for disallowed volumes path")
		}
		if !common.IsUserConfigurationError(err) {
			t.Errorf("expected UserConfigurationError, got %v", err)
		}
	})
}
