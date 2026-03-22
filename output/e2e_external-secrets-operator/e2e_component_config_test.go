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
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/test/utils"
)

var _ = Describe("Component Configuration Overrides (EP-1898)", Ordered, func() {
	ctx := context.TODO()
	var (
		clientset *kubernetes.Clientset
		k8sClient client.Client
	)

	BeforeAll(func() {
		var err error
		clientset, err = kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		k8sClient, err = client.New(cfg, client.Options{})
		Expect(err).NotTo(HaveOccurred())

		// Ensure operator is running
		By("Verifying operator pod is ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespace, []string{
			operatorPodPrefix,
		})).To(Succeed())

		// Ensure operand pods are running
		By("Verifying external-secrets operand pods are ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
			operandCoreControllerPodPrefix,
			operandWebhookPodPrefix,
		})).To(Succeed())
	})

	AfterAll(func() {
		By("Cleaning up: removing annotations and componentConfigs")
		esc := &operatorv1alpha1.ExternalSecretsConfig{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

		esc.Spec.ControllerConfig.Annotations = nil
		esc.Spec.ControllerConfig.ComponentConfigs = nil
		Expect(k8sClient.Update(ctx, esc)).To(Succeed())
	})

	// Diff-suggested: New annotations field on ControllerConfig (EP-1898)
	Context("Global Annotations", func() {
		It("should apply custom annotations to all operand Deployments and Pod templates", func() {
			By("Patching ExternalSecretsConfig with a custom annotation")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.Annotations = []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/e2e-annotation", Value: "e2e-test-value"}},
			}
			Expect(k8sClient.Update(ctx, esc)).To(Succeed())

			By("Waiting for annotation to appear on controller Deployment")
			Eventually(func(g Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deploy)).To(Succeed())

				g.Expect(deploy.Annotations).To(HaveKeyWithValue("example.com/e2e-annotation", "e2e-test-value"),
					"annotation should be present on Deployment metadata")
				g.Expect(deploy.Spec.Template.Annotations).To(HaveKeyWithValue("example.com/e2e-annotation", "e2e-test-value"),
					"annotation should be present on Pod template metadata")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying annotation is also on webhook Deployment")
			Eventually(func(g Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets-webhook",
					Namespace: operandNamespace,
				}, deploy)).To(Succeed())

				g.Expect(deploy.Annotations).To(HaveKeyWithValue("example.com/e2e-annotation", "e2e-test-value"))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should update annotations when ExternalSecretsConfig is modified", func() {
			By("Adding a second annotation")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.Annotations = []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/e2e-annotation", Value: "e2e-test-value"}},
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/e2e-second", Value: "second-value"}},
			}
			Expect(k8sClient.Update(ctx, esc)).To(Succeed())

			By("Verifying both annotations appear on the Deployment")
			Eventually(func(g Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deploy)).To(Succeed())

				g.Expect(deploy.Annotations).To(HaveKeyWithValue("example.com/e2e-annotation", "e2e-test-value"))
				g.Expect(deploy.Annotations).To(HaveKeyWithValue("example.com/e2e-second", "second-value"))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: New componentConfigs field with deploymentConfig (EP-1898)
	Context("Component Config - RevisionHistoryLimit", func() {
		It("should apply revisionHistoryLimit to the targeted component Deployment", func() {
			By("Setting revisionHistoryLimit=5 for ExternalSecretsCoreController")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CoreController,
					DeploymentConfig: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(int32(5)),
					},
				},
			}
			Expect(k8sClient.Update(ctx, esc)).To(Succeed())

			By("Waiting for revisionHistoryLimit to be applied to the Deployment")
			Eventually(func(g Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deploy)).To(Succeed())

				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil(),
					"revisionHistoryLimit should be set")
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(5)),
					"revisionHistoryLimit should be 5")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should apply different revisionHistoryLimit values to different components", func() {
			By("Setting different limits for Controller and Webhook")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CoreController,
					DeploymentConfig: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(int32(10)),
					},
				},
				{
					ComponentName: operatorv1alpha1.Webhook,
					DeploymentConfig: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(int32(3)),
					},
				},
			}
			Expect(k8sClient.Update(ctx, esc)).To(Succeed())

			By("Verifying controller Deployment")
			Eventually(func(g Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deploy)).To(Succeed())

				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(10)))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying webhook Deployment")
			Eventually(func(g Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets-webhook",
					Namespace: operandNamespace,
				}, deploy)).To(Succeed())

				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(3)))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: New overrideEnv field on ComponentConfig (EP-1898)
	Context("Component Config - Override Environment Variables", func() {
		It("should merge custom environment variables into the component container", func() {
			By("Setting GOMAXPROCS=4 for ExternalSecretsCoreController")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CoreController,
					OverrideEnv: []corev1.EnvVar{
						{Name: "GOMAXPROCS", Value: "4"},
					},
				},
			}
			Expect(k8sClient.Update(ctx, esc)).To(Succeed())

			By("Verifying GOMAXPROCS is present in the container env vars")
			Eventually(func(g Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deploy)).To(Succeed())

				found := false
				for _, container := range deploy.Spec.Template.Spec.Containers {
					if container.Name == "external-secrets" {
						for _, env := range container.Env {
							if env.Name == "GOMAXPROCS" {
								g.Expect(env.Value).To(Equal("4"))
								found = true
								break
							}
						}
						break
					}
				}
				g.Expect(found).To(BeTrue(), "GOMAXPROCS env var should be present in container spec")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: Combined annotations + componentConfigs workflow (EP-1898)
	Context("Combined Configuration", func() {
		It("should apply annotations and component configs together", func() {
			By("Setting annotations and componentConfigs simultaneously")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.Annotations = []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/combined-test", Value: "combined-value"}},
			}
			esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CoreController,
					DeploymentConfig: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(int32(7)),
					},
					OverrideEnv: []corev1.EnvVar{
						{Name: "CUSTOM_VAR", Value: "custom-value"},
					},
				},
			}
			Expect(k8sClient.Update(ctx, esc)).To(Succeed())

			By("Verifying all configurations are applied to the controller Deployment")
			Eventually(func(g Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deploy)).To(Succeed())

				// Check annotation
				g.Expect(deploy.Annotations).To(HaveKeyWithValue("example.com/combined-test", "combined-value"))

				// Check revisionHistoryLimit
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(7)))

				// Check env var
				found := false
				for _, container := range deploy.Spec.Template.Spec.Containers {
					for _, env := range container.Env {
						if env.Name == "CUSTOM_VAR" && env.Value == "custom-value" {
							found = true
							break
						}
					}
				}
				g.Expect(found).To(BeTrue(), "CUSTOM_VAR env var should be present")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: Verify reconciliation after removing overrides (EP-1898)
	Context("Configuration Removal", func() {
		It("should restore defaults when componentConfigs are removed", func() {
			By("First setting a componentConfig")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CoreController,
					DeploymentConfig: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(int32(15)),
					},
				},
			}
			Expect(k8sClient.Update(ctx, esc)).To(Succeed())

			By("Waiting for revisionHistoryLimit to be applied")
			Eventually(func(g Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deploy)).To(Succeed())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(15)))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("Removing componentConfigs")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())
			esc.Spec.ControllerConfig.ComponentConfigs = nil
			esc.Spec.ControllerConfig.Annotations = nil
			Expect(k8sClient.Update(ctx, esc)).To(Succeed())

			By("Verifying ExternalSecretsConfig is reconciled successfully after removal")
			Eventually(func(g Gomega) {
				updatedEsc := &operatorv1alpha1.ExternalSecretsConfig{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, updatedEsc)).To(Succeed())

				// Verify Ready condition is True
				for _, cond := range updatedEsc.Status.Conditions {
					if cond.Type == "Ready" {
						g.Expect(string(cond.Status)).To(Equal("True"),
							fmt.Sprintf("Ready condition should be True, got: %s, message: %s", cond.Status, cond.Message))
					}
				}
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})
})
