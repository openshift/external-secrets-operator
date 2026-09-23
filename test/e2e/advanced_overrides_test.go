//go:build e2e
// +build e2e

/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"context"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	externalsecrets "github.com/openshift/external-secrets-operator/pkg/controller/external_secrets"
	"github.com/openshift/external-secrets-operator/test/utils"
)

var _ = Describe("Advanced Overrides", Ordered, Label("Platform:Generic", "Feature:AdvancedOverrides"), func() {
	ctx := context.Background()

	var (
		clientset     *kubernetes.Clientset
		dynamicClient *dynamic.DynamicClient
		runtimeClient client.Client
	)

	BeforeAll(func() {
		clientset = suiteClientset
		dynamicClient = suiteDynamicClient
		runtimeClient = suiteRuntimeClient
		Expect(clientset).NotTo(BeNil())
		Expect(dynamicClient).NotTo(BeNil())
		Expect(runtimeClient).NotTo(BeNil())

		By("Ensuring ExternalSecretsConfig is Ready")
		Expect(ensureExternalSecretsConfigReady(ctx)).To(Succeed())

		esc := &operatorv1alpha1.ExternalSecretsConfig{}
		Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc)).To(Succeed())

		By("Waiting for operand pods to be ready")
		Expect(utils.VerifyOperandPodsReady(ctx, clientset, operandNamespace, esc)).To(Succeed())
	})

	AfterEach(func() {
		By("Clearing advancedOverrides from all components")
		clearAdvancedOverrides(ctx, runtimeClient)

		By("Waiting for ExternalSecretsConfig to recover to Ready")
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 3*time.Minute)).To(Succeed())
	})

	Context("Valid scheduling overrides on the core controller", func() {
		It("should apply affinity from advancedOverrides to the core controller Deployment", func() {
			By("Setting advancedOverrides with pod anti-affinity on the core controller")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"affinity": map[string]any{
								"podAntiAffinity": map[string]any{
									"preferredDuringSchedulingIgnoredDuringExecution": []map[string]any{
										{
											"weight": 100,
											"podAffinityTerm": map[string]any{
												"topologyKey": "kubernetes.io/hostname",
												"labelSelector": map[string]any{
													"matchLabels": map[string]string{"app": "external-secrets"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to be Ready")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

			By("Verifying pod anti-affinity is set on the core controller Deployment")
			Eventually(func(g Gomega) {
				dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
				g.Expect(dep.Spec.Template.Spec.Affinity).NotTo(BeNil(), "affinity should be set")
				g.Expect(dep.Spec.Template.Spec.Affinity.PodAntiAffinity).NotTo(BeNil(), "podAntiAffinity should be set")
				g.Expect(dep.Spec.Template.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution).
					To(HaveLen(1), "should have 1 preferred anti-affinity rule")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should apply tolerations from advancedOverrides to the core controller Deployment", func() {
			By("Setting advancedOverrides with tolerations on the core controller")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"tolerations": []map[string]any{
								{"key": "node-role.kubernetes.io/infra", "operator": "Exists", "effect": "NoSchedule"},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to be Ready")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

			By("Verifying tolerations are set on the core controller Deployment")
			Eventually(func(g Gomega) {
				dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
				g.Expect(dep.Spec.Template.Spec.Tolerations).To(ContainElement(
					SatisfyAll(
						HaveField("Key", "node-role.kubernetes.io/infra"),
						HaveField("Operator", corev1.TolerationOperator("Exists")),
						HaveField("Effect", corev1.TaintEffect("NoSchedule")),
					),
				))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should apply nodeSelector from advancedOverrides to the core controller Deployment", func() {
			By("Setting advancedOverrides with nodeSelector on the core controller")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"nodeSelector": map[string]string{"disktype": "ssd"},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to be Ready")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

			By("Verifying nodeSelector is set on the core controller Deployment")
			Eventually(func(g Gomega) {
				dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
				g.Expect(dep.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("disktype", "ssd"))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should apply topologySpreadConstraints from advancedOverrides to the core controller Deployment", func() {
			By("Setting advancedOverrides with topologySpreadConstraints on the core controller")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"topologySpreadConstraints": []map[string]any{
								{
									"maxSkew":           1,
									"topologyKey":       "topology.kubernetes.io/zone",
									"whenUnsatisfiable": "ScheduleAnyway",
								},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to be Ready")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

			By("Verifying topologySpreadConstraints are set on the core controller Deployment")
			Eventually(func(g Gomega) {
				dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
				g.Expect(dep.Spec.Template.Spec.TopologySpreadConstraints).To(HaveLen(1))
				g.Expect(dep.Spec.Template.Spec.TopologySpreadConstraints[0].TopologyKey).
					To(Equal("topology.kubernetes.io/zone"))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	Context("Valid container args and resources overrides", func() {
		It("should apply container args override via advancedOverrides to the core controller", func() {
			By("Setting advancedOverrides with custom --concurrent on the core controller")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{
									"name": externalsecrets.OperandCoreControllerContainer,
									"args": []string{"--concurrent=20", "--client-burst=200", "--client-qps=100"},
								},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to be Ready")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

			By("Verifying the core controller has the overridden args")
			Eventually(func(g Gomega) {
				dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
				c := containerByName(dep, externalsecrets.OperandCoreControllerContainer)
				g.Expect(c).NotTo(BeNil(), "core controller container should exist")
				g.Expect(c.Args).To(ContainElement("--concurrent=20"), "should have --concurrent=20")
				g.Expect(c.Args).To(ContainElement("--client-burst=200"), "should have --client-burst=200")
				g.Expect(c.Args).To(ContainElement("--client-qps=100"), "should have --client-qps=100")
				g.Expect(c.Args).NotTo(ContainElement("--concurrent=1"), "default --concurrent=1 should be replaced")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should apply container resources override via advancedOverrides", func() {
			By("Setting advancedOverrides with custom resources on the core controller")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{
									"name": externalsecrets.OperandCoreControllerContainer,
									"resources": map[string]any{
										"requests": map[string]string{"cpu": "200m", "memory": "256Mi"},
										"limits":   map[string]string{"cpu": "1", "memory": "512Mi"},
									},
								},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to be Ready")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

			By("Verifying the core controller container has the overridden resources")
			Eventually(func(g Gomega) {
				dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
				c := containerByName(dep, externalsecrets.OperandCoreControllerContainer)
				g.Expect(c).NotTo(BeNil(), "core controller container should exist")
				g.Expect(c.Resources.Requests.Cpu().String()).To(Equal("200m"))
				g.Expect(c.Resources.Requests.Memory().String()).To(Equal("256Mi"))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	Context("Forbidden paths set Degraded condition", func() {
		It("should set Degraded when advancedOverrides targets spec.replicas", func() {
			By("Setting advancedOverrides targeting spec.replicas")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{"replicas": 3},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded when advancedOverrides targets spec.replicas")
				msg := externalSecretsConfigDegradedMessage(ctx)
				g.Expect(msg).To(ContainSubstring("replicas"),
					"Degraded message should mention replicas")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying the core controller Deployment is not changed")
			dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
			Expect(dep.Spec.Replicas == nil || *dep.Spec.Replicas != 3).To(BeTrue(),
				"replicas should not be set to 3 by advancedOverrides")
		})

		It("should set Degraded when advancedOverrides targets containers[].image", func() {
			By("Setting advancedOverrides targeting container image")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{
									"name":  externalsecrets.OperandCoreControllerContainer,
									"image": "evil:latest",
								},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded when advancedOverrides targets image")
				msg := externalSecretsConfigDegradedMessage(ctx)
				g.Expect(msg).To(ContainSubstring("image"),
					"Degraded message should mention image")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying the core controller container image is not changed")
			dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
			c := containerByName(dep, externalsecrets.OperandCoreControllerContainer)
			Expect(c).NotTo(BeNil())
			Expect(c.Image).NotTo(Equal("evil:latest"), "container image should not be overridden")
		})

		It("should set Degraded when advancedOverrides targets containers[].env", func() {
			By("Setting advancedOverrides targeting container env")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{
									"name": externalsecrets.OperandCoreControllerContainer,
									"env":  []map[string]any{{"name": "EVIL", "value": "true"}},
								},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded when advancedOverrides targets env")
				msg := externalSecretsConfigDegradedMessage(ctx)
				g.Expect(msg).To(ContainSubstring("env"),
					"Degraded message should mention env")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should set Degraded when advancedOverrides targets volumes", func() {
			By("Setting advancedOverrides targeting volumes")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"volumes": []map[string]any{{"name": "evil-vol"}},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded when advancedOverrides targets volumes")
				msg := externalSecretsConfigDegradedMessage(ctx)
				g.Expect(msg).To(ContainSubstring("volumes"),
					"Degraded message should mention volumes")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should set Degraded when advancedOverrides targets initContainers", func() {
			By("Setting advancedOverrides targeting initContainers")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"initContainers": []map[string]any{
								{"name": "evil-init", "image": "busybox"},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded when advancedOverrides targets initContainers")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should set Degraded when advancedOverrides targets containers[].command", func() {
			By("Setting advancedOverrides targeting container command")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{
									"name":    externalsecrets.OperandCoreControllerContainer,
									"command": []string{"/bin/sh", "-c", "evil"},
								},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded when advancedOverrides targets command")
				msg := externalSecretsConfigDegradedMessage(ctx)
				g.Expect(msg).To(ContainSubstring("command"),
					"Degraded message should mention command")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should set Degraded when advancedOverrides targets containers[].securityContext", func() {
			By("Setting advancedOverrides targeting container securityContext")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{
									"name":            externalsecrets.OperandCoreControllerContainer,
									"securityContext": map[string]any{"privileged": true},
								},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded when advancedOverrides targets securityContext")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should set Degraded when advancedOverrides targets containers[].ports", func() {
			By("Setting advancedOverrides targeting container ports")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{
									"name":  externalsecrets.OperandCoreControllerContainer,
									"ports": []map[string]any{{"containerPort": 9999}},
								},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded when advancedOverrides targets ports")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should set Degraded when advancedOverrides targets containers[].volumeMounts", func() {
			By("Setting advancedOverrides targeting container volumeMounts")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{
									"name":         externalsecrets.OperandCoreControllerContainer,
									"volumeMounts": []map[string]any{{"name": "evil", "mountPath": "/evil"}},
								},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded when advancedOverrides targets volumeMounts")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should set Degraded when advancedOverrides targets spec.selector", func() {
			By("Setting advancedOverrides targeting spec.selector")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"selector": map[string]any{
						"matchLabels": map[string]string{"app": "evil"},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded when advancedOverrides targets spec.selector")
				msg := externalSecretsConfigDegradedMessage(ctx)
				g.Expect(msg).To(ContainSubstring("selector"),
					"Degraded message should mention selector")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should set Degraded when advancedOverrides targets spec.template.metadata", func() {
			By("Setting advancedOverrides targeting template metadata")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"metadata": map[string]any{
							"labels": map[string]string{"evil": "true"},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded when advancedOverrides targets template metadata")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should set Degraded when advancedOverrides targets serviceAccountName", func() {
			By("Setting advancedOverrides targeting serviceAccountName")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"serviceAccountName": "evil",
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded when advancedOverrides targets serviceAccountName")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should set Degraded when advancedOverrides references a wrong container name", func() {
			By("Setting advancedOverrides with an incorrect container name")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{
									"name": "evil-sidecar",
									"args": []string{"--hack"},
								},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded when container name is invalid")
				msg := externalSecretsConfigDegradedMessage(ctx)
				g.Expect(msg).To(ContainSubstring("not a valid target"),
					"Degraded message should indicate the container name is not valid")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	Context("Malformed advancedOverrides sets Degraded", func() {

		It("should set Degraded when advancedOverrides contains invalid deployment fields", func() {
			By("Setting advancedOverrides with unknown fields")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"bar": map[string]any{"foo": "evil"},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded for unknown fields in advancedOverrides")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	Context("Recovery from Degraded after fixing advancedOverrides", func() {
		It("should recover from Degraded when invalid advancedOverrides are corrected", func() {
			By("Setting an invalid advancedOverrides (forbidden path: volumes)")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"volumes": []map[string]any{{"name": "evil-vol"}},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
					"ExternalSecretsConfig should be Degraded for volumes in advancedOverrides")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("Correcting advancedOverrides to a valid nodeSelector patch")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"nodeSelector": map[string]string{"disktype": "ssd"},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to recover to Ready")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 3*time.Minute)).To(Succeed())

			By("Verifying the correction was applied")
			Eventually(func(g Gomega) {
				dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
				g.Expect(dep.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("disktype", "ssd"))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should recover from Degraded when advancedOverrides are cleared", func() {
			By("Setting an invalid advancedOverrides (forbidden path: initContainers)")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"initContainers": []map[string]any{
								{"name": "evil-init", "image": "busybox"},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue())
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("Clearing advancedOverrides")
			clearAdvancedOverrides(ctx, runtimeClient)

			By("Waiting for ExternalSecretsConfig to recover to Ready")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 3*time.Minute)).To(Succeed())
		})
	})

	Context("Multi-component isolation", func() {
		It("should apply advancedOverrides to the webhook without affecting the core controller", func() {
			By("Setting advancedOverrides with tolerations on the webhook component only")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.Webhook, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"tolerations": []map[string]any{
								{"key": "webhook-only", "operator": "Exists", "effect": "NoSchedule"},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to be Ready")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

			By("Verifying tolerations are applied to the webhook Deployment")
			Eventually(func(g Gomega) {
				dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandWebhookDeployment)
				g.Expect(dep.Spec.Template.Spec.Tolerations).To(ContainElement(
					HaveField("Key", "webhook-only"),
				))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying the core controller Deployment does NOT have the webhook-only toleration")
			dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
			for _, t := range dep.Spec.Template.Spec.Tolerations {
				Expect(t.Key).NotTo(Equal("webhook-only"),
					"core controller should not have webhook-only toleration")
			}
		})

		It("should apply advancedOverrides to the cert-controller without affecting other components", func() {
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc)).To(Succeed())
			if !utils.IsCertControllerExpected(esc) {
				Skip("cert-controller Deployment is not expected with current ExternalSecretsConfig (cert-manager enabled)")
			}

			By("Setting advancedOverrides with nodeSelector on the cert-controller component")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CertController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"nodeSelector": map[string]string{"tier": "cert-controller-only"},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to be Ready")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

			By("Verifying nodeSelector is applied to the cert-controller Deployment")
			Eventually(func(g Gomega) {
				dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCertControllerDeployment)
				g.Expect(dep.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("tier", "cert-controller-only"))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying the core controller Deployment does NOT have the cert-controller nodeSelector")
			dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
			Expect(dep.Spec.Template.Spec.NodeSelector).NotTo(HaveKey("tier"),
				"core controller should not have tier nodeSelector from cert-controller overrides")
		})
	})

	Context("Comprehensive combined override", func() {
		It("should apply a comprehensive advancedOverrides with all allowlisted fields simultaneously", func() {
			By("Setting advancedOverrides with affinity, tolerations, nodeSelector, topologySpreadConstraints, args, and resources")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
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
								{"maxSkew": 1, "topologyKey": "topology.kubernetes.io/zone", "whenUnsatisfiable": "ScheduleAnyway"},
							},
							"containers": []map[string]any{
								{
									"name": externalsecrets.OperandCoreControllerContainer,
									"args": []string{"--concurrent=20", "--client-burst=200", "--client-qps=100"},
									"resources": map[string]any{
										"requests": map[string]string{"cpu": "200m", "memory": "256Mi"},
									},
								},
							},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to be Ready")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

			By("Verifying all allowlisted fields are applied to the core controller Deployment")
			Eventually(func(g Gomega) {
				dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
				g.Expect(dep.Spec.Template.Spec.Affinity).NotTo(BeNil(), "affinity should be set")
				g.Expect(dep.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil(), "nodeAffinity should be set")
				g.Expect(dep.Spec.Template.Spec.Tolerations).To(ContainElement(
					HaveField("Key", "dedicated"),
				))
				g.Expect(dep.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("tier", "eso"))
				g.Expect(dep.Spec.Template.Spec.TopologySpreadConstraints).To(HaveLen(1))
				g.Expect(dep.Spec.Template.Spec.TopologySpreadConstraints[0].TopologyKey).
					To(Equal("topology.kubernetes.io/zone"))

				c := containerByName(dep, externalsecrets.OperandCoreControllerContainer)
				g.Expect(c).NotTo(BeNil(), "core controller container should exist")
				g.Expect(c.Args).To(ContainElement("--concurrent=20"))
				g.Expect(c.Args).To(ContainElement("--client-burst=200"))
				g.Expect(c.Args).To(ContainElement("--client-qps=100"))
				g.Expect(c.Resources.Requests.Cpu().String()).To(Equal("200m"))
				g.Expect(c.Resources.Requests.Memory().String()).To(Equal("256Mi"))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying operator baseline args are preserved alongside overrides")
			Eventually(func(g Gomega) {
				dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
				c := containerByName(dep, externalsecrets.OperandCoreControllerContainer)
				g.Expect(c).NotTo(BeNil())
				g.Expect(slices.ContainsFunc(c.Args, func(a string) bool {
					return a == "--metrics-addr=:8080" || a == "--zap-time-encoding=epoch"
				})).To(BeFalse(), "strategic merge replaces the args list; baseline defaults come from the override")
			}, time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	Context("Leader election re-assertion after advancedOverrides", func() {
		It("should re-assert --enable-leader-election=true on the core controller when replicas > 1 after advancedOverrides", func() {

			By("Setting advancedOverrides with custom args (which replaces the args list)")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]any{
								{
									"name": externalsecrets.OperandCoreControllerContainer,
									"args": []string{"--concurrent=20"},
								},
							},
						},
					},
				},
			}), ptr.To(int32(2)))

			By("Waiting for ExternalSecretsConfig to be Ready")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

			By("Verifying replicas > 1 and leader election arg is re-asserted")
			Eventually(func(g Gomega) {
				dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
				g.Expect(dep.Spec.Replicas).NotTo(BeNil(), "replicas should be set")
				g.Expect(*dep.Spec.Replicas).To(BeNumerically(">", 1), "Core controller replicas must be > 1")
				c := containerByName(dep, externalsecrets.OperandCoreControllerContainer)
				g.Expect(c).NotTo(BeNil())
				g.Expect(c.Args).To(ContainElement(externalsecrets.LeaderElectionArg),
					"leader election should be re-asserted when replicas > 1")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	Context("Rapid consecutive updates", func() {
		It("should handle rapid valid-then-invalid-then-valid advancedOverrides updates without crashing", func() {
			By("Applying a valid advancedOverrides patch")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"nodeSelector": map[string]string{"phase": "one"},
						},
					},
				},
			}), nil)

			By("Immediately applying an invalid advancedOverrides patch")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"volumes": []map[string]any{{"name": "evil"}},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to become Degraded")
			Eventually(func(g Gomega) {
				g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue())
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("Immediately applying a valid advancedOverrides patch to recover")
			setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"nodeSelector": map[string]string{"phase": "three"},
						},
					},
				},
			}), nil)

			By("Waiting for ExternalSecretsConfig to recover to Ready")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 3*time.Minute)).To(Succeed())

			By("Verifying the final valid patch was applied")
			Eventually(func(g Gomega) {
				dep := getOperandDeployment(ctx, clientset, externalsecrets.OperandCoreControllerDeployment)
				g.Expect(dep.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("phase", "three"))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	Context("Disallowed container-level fields batch", func() {
		type forbiddenCase struct {
			name       string
			field      string
			fieldValue any
		}

		cases := []forbiddenCase{
			{name: "env", field: "env", fieldValue: []map[string]any{{"name": "EVIL", "value": "true"}}},
			{name: "ports", field: "ports", fieldValue: []map[string]any{{"containerPort": 9999}}},
			{name: "volumeMounts", field: "volumeMounts", fieldValue: []map[string]any{{"name": "v", "mountPath": "/m"}}},
			{name: "image", field: "image", fieldValue: "evil:latest"},
			{name: "command", field: "command", fieldValue: []string{"/bin/sh"}},
			{name: "securityContext", field: "securityContext", fieldValue: map[string]any{"privileged": true}},
		}

		for _, tc := range cases {
			tc := tc
			It("should reject advancedOverrides targeting containers[]."+tc.name, func() {
				setAdvancedOverrides(ctx, runtimeClient, operatorv1alpha1.CoreController, rawJSON(map[string]any{
					"spec": map[string]any{
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []map[string]any{
									{
										"name":   externalsecrets.OperandCoreControllerContainer,
										tc.field: tc.fieldValue,
									},
								},
							},
						},
					},
				}), nil)

				Eventually(func(g Gomega) {
					g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
						"should be Degraded for forbidden containers[].%s", tc.name)
				}, 2*time.Minute, 5*time.Second).Should(Succeed())

				clearAdvancedOverrides(ctx, runtimeClient)
				Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 3*time.Minute)).To(Succeed())
			})
		}
	})
})
