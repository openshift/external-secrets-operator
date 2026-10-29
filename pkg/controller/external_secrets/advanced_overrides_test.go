package external_secrets

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
		containerName string
		wantErr       bool
		errSubstring  string
		assertDeploy  func(t *testing.T, d *appsv1.Deployment)
	}{
		{
			name:          "nil overrides is no-op",
			deployment:    baseDeployment(),
			overrides:     nil,
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
			containerName: OperandCoreControllerContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				res := d.Spec.Template.Spec.Containers[0].Resources
				if res.Requests.Cpu().Cmp(resource.MustParse("100m")) != 0 {
					t.Errorf("expected CPU request 100m, got %v", res.Requests.Cpu())
				}
			},
		},
		// --- Disallowed top-level fields ---
		{
			name:       "reject metadata at top level",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"metadata": map[string]any{"name": "evil"},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "metadata",
		},
		{
			name:       "reject status at top level",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"status": map[string]any{},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "status",
		},

		// --- Disallowed paths ---
		{
			name:       "reject spec.replicas",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"replicas": 3},
			}),
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
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "containers[0].env",
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
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "containers[0].image",
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
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "containers[0].ports",
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
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "containers[0].securityContext",
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
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "containers[0].volumeMounts",
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
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "containers[0].command",
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
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "must match the component container",
		},
		{
			// The container loop must inspect every entry, not stop after the first match:
			// a valid entry for the component container followed by an entry for an unrelated
			// container must still be rejected.
			name:       "reject second container entry with mismatched name",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"name": OperandCoreControllerContainer, "args": []string{"--concurrent=20"}},
						{"name": "evil-sidecar", "args": []string{"--hack"}},
					},
				}}},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "must match the component container",
		},
		{
			// Strategic-merge-patch directive keys ($patch, $retainKeys, $setElementOrder/*) are
			// never valid field names on any Kubernetes API type, so they're rejected by the
			// strict DisallowUnknownFields decode at the top of validateOverridePaths before the
			// allowlist walk even runs. This pins that down explicitly rather than relying on it
			// as an accidental side effect.
			name:       "reject $patch directive key in container entry",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"name": OperandCoreControllerContainer, "$patch": "replace", "args": []string{"--concurrent=20"}},
					},
				}}},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "$patch",
		},
		{
			name:       "reject $setElementOrder directive key at pod-spec level",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"$setElementOrder/containers": []map[string]any{{"name": OperandCoreControllerContainer}},
				}}},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "$setElementOrder/containers",
		},
		{
			name:       "auto-inject container name when omitted",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"args": []string{"--concurrent=20"}},
					},
				}}},
			}),
			containerName: OperandCoreControllerContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				args := d.Spec.Template.Spec.Containers[0].Args
				if !slices.Contains(args, "--concurrent=20") {
					t.Errorf("expected --concurrent=20 in args after auto-inject, got %v", args)
				}
			},
		},
		{
			name:       "auto-inject container name with resources override",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"resources": map[string]any{
							"requests": map[string]string{"cpu": "100m", "memory": "128Mi"},
						}},
					},
				}}},
			}),
			containerName: OperandCoreControllerContainer,
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				res := d.Spec.Template.Spec.Containers[0].Resources
				if res.Requests.Cpu().Cmp(resource.MustParse("100m")) != 0 {
					t.Errorf("expected CPU request 100m after auto-inject, got %v", res.Requests.Cpu())
				}
			},
		},
		{
			name:       "reject explicit empty container name",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"name": "", "args": []string{"--concurrent=20"}},
					},
				}}},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "container name is required",
		},
		{
			name:          "reject invalid JSON",
			deployment:    baseDeployment(),
			overrides:     &runtime.RawExtension{Raw: []byte(`{not valid json}`)},
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
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "unknown field",
		},
		{
			// Regression test: a strategic merge patch treats an explicit JSON null as
			// "delete this field". Before the fix, unstructured.NestedMap's error for a
			// null "spec" value was conflated with "spec absent", so this patch would
			// silently skip all allowlist checks and wipe out deployment.Spec entirely.
			name:       "reject null spec",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": nil,
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "spec",
		},
		{
			// Regression test: same conflation as above, but for the "containers" key,
			// which is already allowlisted at the pod-spec level, making this reachable
			// without even tripping the top-level/pod-spec allowlist checks.
			name:       "reject null containers",
			deployment: baseDeployment(),
			overrides: rawJSON(map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": nil,
				}}},
			}),
			containerName: OperandCoreControllerContainer,
			wantErr:       true,
			errSubstring:  "containers",
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
			err := applyAdvancedOverrides(tt.deployment, tt.overrides, tt.containerName)
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

func TestInjectContainerName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		raw           map[string]any
		containerName string
		wantInjected  bool
		wantName      string
	}{
		{
			name:          "inject name when missing",
			raw:           map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"args": []any{"--concurrent=20"}}}}}}},
			containerName: OperandCoreControllerContainer,
			wantInjected:  true,
			wantName:      OperandCoreControllerContainer,
		},
		{
			name:          "do not overwrite existing name",
			raw:           map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"name": "webhook", "args": []any{"--concurrent=20"}}}}}}},
			containerName: OperandCoreControllerContainer,
			wantInjected:  false,
			wantName:      "webhook",
		},
		{
			name:          "do not overwrite explicit empty name",
			raw:           map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"name": "", "args": []any{"--concurrent=20"}}}}}}},
			containerName: OperandCoreControllerContainer,
			wantInjected:  false,
			wantName:      "",
		},
		{
			name:          "no containers path is no-op",
			raw:           map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"nodeSelector": map[string]any{"tier": "eso"}}}}},
			containerName: OperandCoreControllerContainer,
			wantInjected:  false,
		},
		{
			name:          "empty containers list is no-op",
			raw:           map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{}}}}},
			containerName: OperandCoreControllerContainer,
			wantInjected:  false,
		},
		{
			name:          "nil raw is no-op",
			raw:           nil,
			containerName: OperandCoreControllerContainer,
			wantInjected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := injectContainerName(tt.raw, tt.containerName)
			if got != tt.wantInjected {
				t.Errorf("injectContainerName() = %v, want %v", got, tt.wantInjected)
			}
			if tt.wantName != "" || tt.wantInjected {
				containers, exists, err := unstructured.NestedSlice(tt.raw, "spec", "template", "spec", "containers")
				if err != nil || !exists {
					t.Fatal("expected containers path to exist after injection")
				}
				cMap := containers[0].(map[string]any)
				name, _ := cMap["name"].(string)
				if name != tt.wantName {
					t.Errorf("container name = %q, want %q", name, tt.wantName)
				}
			}
		})
	}
}

func TestApplyAdvancedOverridesViaReconciler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// initialArgs seeds the container's Args before advancedOverrides is applied, simulating
		// whatever first-class/upstream logic (e.g. the OPERAND_EXTERNAL_SECRETS_ARGS env-var
		// escape hatch, applied earlier in getDeploymentObject) already ran.
		initialArgs  []string
		overrides    map[string]any
		wantErr      bool
		assertDeploy func(t *testing.T, d *appsv1.Deployment)
	}{
		{
			name:        "advancedOverrides applied through applyUserDeploymentConfigs",
			initialArgs: []string{"--concurrent=1", "--metrics-addr=:8080"},
			overrides: map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"nodeSelector": map[string]string{"disktype": "ssd"},
						},
					},
				},
			},
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				if d.Spec.Template.Spec.NodeSelector["disktype"] != "ssd" {
					t.Errorf("expected nodeSelector to be applied, got %v", d.Spec.Template.Spec.NodeSelector)
				}
			},
		},
		{
			name: "invalid advancedOverrides returns error through applyUserDeploymentConfigs",
			overrides: map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"volumes": []map[string]any{{"name": "evil"}},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			// Both applyOperandArgsFromEnv (the temporary v1.1/1.2 escape hatch, see its TODO)
			// and advancedOverrides (its replacement) can set container args for the same
			// component. getDeploymentObject calls the env-var hatch first and advancedOverrides
			// last, so advancedOverrides must win for any flag both of them touch. initialArgs
			// below is seeded via mergeContainerArgs (what the env-var hatch calls internally)
			// rather than applyOperandArgsFromEnv + t.Setenv, since t.Setenv panics under any
			// parallel ancestor test, and os.Environ is process-global.
			name:        "advancedOverrides args take precedence over the OPERAND_EXTERNAL_SECRETS_ARGS env-var escape hatch",
			initialArgs: mustMergeContainerArgs(t, []string{"--concurrent=1", "--metrics-addr=:8080"}, "--concurrent=20,--loglevel=debug"),
			overrides: map[string]any{
				"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
					"containers": []map[string]any{
						{"name": OperandCoreControllerContainer, "args": []string{"--concurrent=99"}},
					},
				}}},
			},
			assertDeploy: func(t *testing.T, d *appsv1.Deployment) {
				got := d.Spec.Template.Spec.Containers[0].Args
				if !slices.Equal(got, []string{"--concurrent=99"}) {
					t.Errorf("expected advancedOverrides args to fully replace the env-var hatch's args, got %v", got)
				}
			},
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
								{Name: OperandCoreControllerContainer, Args: tt.initialArgs},
							},
						},
					},
				},
			}

			overrideJSON, err := json.Marshal(tt.overrides)
			if err != nil {
				t.Fatalf("failed to marshal overrides: %v", err)
			}
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

			err = r.applyUserDeploymentConfigs(deployment, esc, controllerDeploymentAssetName)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !common.IsUserConfigurationError(err) {
					t.Errorf("expected UserConfigurationError, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.assertDeploy != nil {
				tt.assertDeploy(t, deployment)
			}
		})
	}
}

// mustMergeContainerArgs seeds a table case's initialArgs by merging envArgs (in the
// OPERAND_EXTERNAL_SECRETS_ARGS env-var format) onto base, exactly as applyOperandArgsFromEnv
// would, without needing a live environment variable.
func mustMergeContainerArgs(t *testing.T, base []string, envArgs string) []string {
	t.Helper()
	overrides, err := parseOperandArgsEnv(envArgs)
	if err != nil {
		t.Fatalf("failed to parse env args %q: %v", envArgs, err)
	}
	return mergeContainerArgs(base, overrides)
}

// FuzzApplyAdvancedOverrides asserts an invariant that's cheaper to check by fuzzing than to
// enumerate by hand: for arbitrary attacker/user-controlled JSON, applyAdvancedOverrides must
// either return an error, or leave every field that has no advancedOverrides allowlist entry
// (replicas, selector, container count, and each container's name/image/env) exactly as it was.
// The seed corpus includes the null-value shapes that were found to violate this invariant
// before the fix to validateAllowedFields / the containers NestedSlice check.
func FuzzApplyAdvancedOverrides(f *testing.F) {
	seeds := []string{
		`{"spec": null}`,
		`{"spec": {"template": null}}`,
		`{"spec": {"template": {"spec": null}}}`,
		`{"spec": {"template": {"spec": {"containers": null}}}}`,
		`{"spec": {"template": {"spec": {"affinity": null}}}}`,
		`{"spec": {"replicas": 5}}`,
		`{"spec": {"selector": {"matchLabels": {"app": "evil"}}}}`,
		`{"metadata": {"name": "evil"}}`,
		`{"spec": {"template": {"spec": {"containers": [{"name": "core-controller", "$patch": "replace"}]}}}}`,
		`{"spec": {"template": {"spec": {"containers": [{"name": "core-controller", "image": "evil:latest"}]}}}}`,
		`not json`,
		`null`,
		`42`,
		`[]`,
		`{}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		deployment := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "external-secrets"}},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  OperandCoreControllerContainer,
								Image: "quay.io/example/external-secrets:v1",
								Args:  []string{"--concurrent=1"},
								Env:   []corev1.EnvVar{{Name: "FOO", Value: "bar"}},
							},
						},
					},
				},
			},
		}
		original := deployment.DeepCopy()

		err := applyAdvancedOverrides(deployment, &runtime.RawExtension{Raw: raw}, OperandCoreControllerContainer)
		if err != nil {
			return // rejected: nothing further to check
		}

		// No error means applyAdvancedOverrides considered this input safe. Assert it didn't
		// touch anything outside the documented allowlist.
		if !reflect.DeepEqual(deployment.Spec.Replicas, original.Spec.Replicas) {
			t.Fatalf("replicas changed without error: %v -> %v", original.Spec.Replicas, deployment.Spec.Replicas)
		}
		if !reflect.DeepEqual(deployment.Spec.Selector, original.Spec.Selector) {
			t.Fatalf("selector changed without error: %v -> %v", original.Spec.Selector, deployment.Spec.Selector)
		}
		if len(deployment.Spec.Template.Spec.Containers) != len(original.Spec.Template.Spec.Containers) {
			t.Fatalf("container count changed without error: %d -> %d",
				len(original.Spec.Template.Spec.Containers), len(deployment.Spec.Template.Spec.Containers))
		}
		for i := range original.Spec.Template.Spec.Containers {
			oc := original.Spec.Template.Spec.Containers[i]
			nc := deployment.Spec.Template.Spec.Containers[i]
			if oc.Name != nc.Name {
				t.Fatalf("container[%d] name changed without error: %q -> %q", i, oc.Name, nc.Name)
			}
			if oc.Image != nc.Image {
				t.Fatalf("container[%d] image changed without error: %q -> %q", i, oc.Image, nc.Image)
			}
			if !reflect.DeepEqual(oc.Env, nc.Env) {
				t.Fatalf("container[%d] env changed without error: %+v -> %+v", i, oc.Env, nc.Env)
			}
		}
	})
}
