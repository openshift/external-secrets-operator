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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

// Diff-suggested: EP-1898 introduces annotations, componentConfigs, deploymentConfig, and overrideEnv fields.
// These tests verify the controller correctly propagates these configurations to operand Deployments.
var _ = Describe("ExternalSecretsConfig Component Overrides", Ordered, Label("EP-1898"), func() {
	ctx := context.TODO()
	var (
		ctrlClient client.Client
	)

	BeforeAll(func() {
		var err error

		By("Building scheme with operator types")
		testScheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(testScheme))
		utilruntime.Must(operatorv1alpha1.AddToScheme(testScheme))

		ctrlClient, err = client.New(cfg, client.Options{Scheme: testScheme})
		Expect(err).NotTo(HaveOccurred())
	})

	// Diff-suggested: annotations field in ControllerConfig propagates to all operand Deployments.
	Context("Custom Annotations", func() {
		const (
			annotationKey   = "example.com/custom-annotation"
			annotationValue = "e2e-test-value"
		)

		AfterEach(func() {
			By("Resetting ExternalSecretsConfig to remove annotations")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())
			esc.Spec.ControllerConfig.Annotations = nil
			esc.Spec.ControllerConfig.ComponentConfigs = nil
			Expect(ctrlClient.Update(ctx, esc)).To(Succeed())
		})

		It("should propagate custom annotations to all operand Deployments and Pod templates", func() {
			By("Updating ExternalSecretsConfig with custom annotations")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.Annotations = []operatorv1alpha1.Annotation{
				{
					KVPair: operatorv1alpha1.KVPair{
						Key:   annotationKey,
						Value: annotationValue,
					},
				},
			}
			Expect(ctrlClient.Update(ctx, esc)).To(Succeed())

			By("Waiting for reconciliation to apply annotations")
			deploymentNames := []string{
				"external-secrets",
				"external-secrets-webhook",
				"external-secrets-cert-controller",
			}

			for _, name := range deploymentNames {
				By(fmt.Sprintf("Verifying annotations on Deployment %s", name))
				Eventually(func(g Gomega) {
					deployment := &appsv1.Deployment{}
					err := ctrlClient.Get(ctx, types.NamespacedName{
						Name:      name,
						Namespace: operandNamespace,
					}, deployment)
					g.Expect(err).NotTo(HaveOccurred())

					// Check Deployment metadata annotations
					g.Expect(deployment.Annotations).To(HaveKeyWithValue(annotationKey, annotationValue),
						"Deployment %s should have custom annotation in metadata", name)

					// Check Pod template annotations
					g.Expect(deployment.Spec.Template.Annotations).To(HaveKeyWithValue(annotationKey, annotationValue),
						"Deployment %s should have custom annotation in pod template", name)
				}, 2*time.Minute, 5*time.Second).Should(Succeed())
			}
		})

		It("should update annotations when ExternalSecretsConfig is modified", func() {
			const updatedValue = "updated-value"

			By("Setting initial annotations")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.Annotations = []operatorv1alpha1.Annotation{
				{
					KVPair: operatorv1alpha1.KVPair{
						Key:   annotationKey,
						Value: annotationValue,
					},
				},
			}
			Expect(ctrlClient.Update(ctx, esc)).To(Succeed())

			By("Waiting for initial annotations to be applied")
			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				err := ctrlClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deployment)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deployment.Annotations).To(HaveKeyWithValue(annotationKey, annotationValue))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("Updating annotations to new value")
			Expect(ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())
			esc.Spec.ControllerConfig.Annotations = []operatorv1alpha1.Annotation{
				{
					KVPair: operatorv1alpha1.KVPair{
						Key:   annotationKey,
						Value: updatedValue,
					},
				},
			}
			Expect(ctrlClient.Update(ctx, esc)).To(Succeed())

			By("Verifying updated annotations")
			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				err := ctrlClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deployment)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deployment.Annotations).To(HaveKeyWithValue(annotationKey, updatedValue))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: componentConfigs with revisionHistoryLimit applies per-component deployment config.
	Context("Component-Specific RevisionHistoryLimit", func() {
		AfterEach(func() {
			By("Resetting ExternalSecretsConfig to remove componentConfigs")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())
			esc.Spec.ControllerConfig.ComponentConfigs = nil
			Expect(ctrlClient.Update(ctx, esc)).To(Succeed())
		})

		It("should set revisionHistoryLimit on the specified component Deployment", func() {
			var revisionHistoryLimit int32 = 5

			By("Setting revisionHistoryLimit for CoreController")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CoreController,
					DeploymentConfig: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(revisionHistoryLimit),
					},
				},
			}
			Expect(ctrlClient.Update(ctx, esc)).To(Succeed())

			By("Verifying revisionHistoryLimit on controller Deployment")
			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				err := ctrlClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deployment)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deployment.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deployment.Spec.RevisionHistoryLimit).To(Equal(revisionHistoryLimit))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should independently configure revisionHistoryLimit for multiple components", func() {
			var (
				controllerLimit int32 = 10
				webhookLimit    int32 = 3
			)

			By("Setting different revisionHistoryLimit for CoreController and Webhook")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CoreController,
					DeploymentConfig: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(controllerLimit),
					},
				},
				{
					ComponentName: operatorv1alpha1.Webhook,
					DeploymentConfig: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(webhookLimit),
					},
				},
			}
			Expect(ctrlClient.Update(ctx, esc)).To(Succeed())

			By("Verifying controller Deployment has limit 10")
			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				err := ctrlClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deployment)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deployment.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deployment.Spec.RevisionHistoryLimit).To(Equal(controllerLimit))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying webhook Deployment has limit 3")
			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				err := ctrlClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets-webhook",
					Namespace: operandNamespace,
				}, deployment)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deployment.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deployment.Spec.RevisionHistoryLimit).To(Equal(webhookLimit))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: overrideEnv allows injecting custom environment variables per component.
	Context("Override Environment Variables", func() {
		AfterEach(func() {
			By("Resetting ExternalSecretsConfig to remove componentConfigs")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())
			esc.Spec.ControllerConfig.ComponentConfigs = nil
			Expect(ctrlClient.Update(ctx, esc)).To(Succeed())
		})

		It("should merge custom env vars into the component container", func() {
			By("Setting overrideEnv for CoreController")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CoreController,
					OverrideEnv: []corev1.EnvVar{
						{
							Name:  "GOMAXPROCS",
							Value: "4",
						},
					},
				},
			}
			Expect(ctrlClient.Update(ctx, esc)).To(Succeed())

			By("Verifying GOMAXPROCS env var in controller Deployment")
			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				err := ctrlClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deployment)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())

				container := deployment.Spec.Template.Spec.Containers[0]
				found := false
				for _, env := range container.Env {
					if env.Name == "GOMAXPROCS" && env.Value == "4" {
						found = true
						break
					}
				}
				g.Expect(found).To(BeTrue(), "GOMAXPROCS=4 should be present in container env")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should apply both revisionHistoryLimit and overrideEnv together", func() {
			var revisionHistoryLimit int32 = 7

			By("Setting both deploymentConfig and overrideEnv for CoreController")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CoreController,
					DeploymentConfig: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(revisionHistoryLimit),
					},
					OverrideEnv: []corev1.EnvVar{
						{
							Name:  "LOG_FORMAT",
							Value: "json",
						},
					},
				},
			}
			Expect(ctrlClient.Update(ctx, esc)).To(Succeed())

			By("Verifying both revisionHistoryLimit and env var are applied")
			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				err := ctrlClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deployment)
				g.Expect(err).NotTo(HaveOccurred())

				// Check revisionHistoryLimit
				g.Expect(deployment.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deployment.Spec.RevisionHistoryLimit).To(Equal(revisionHistoryLimit))

				// Check env var
				g.Expect(deployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
				container := deployment.Spec.Template.Spec.Containers[0]
				found := false
				for _, env := range container.Env {
					if env.Name == "LOG_FORMAT" && env.Value == "json" {
						found = true
						break
					}
				}
				g.Expect(found).To(BeTrue(), "LOG_FORMAT=json should be present in container env")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: Combined annotations and componentConfigs should work together.
	Context("Combined Annotations and Component Configs", func() {
		AfterEach(func() {
			By("Resetting ExternalSecretsConfig to clean state")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())
			esc.Spec.ControllerConfig.Annotations = nil
			esc.Spec.ControllerConfig.ComponentConfigs = nil
			Expect(ctrlClient.Update(ctx, esc)).To(Succeed())
		})

		It("should apply both annotations and component configs simultaneously", func() {
			const (
				annotationKey   = "example.com/combined-test"
				annotationValue = "combined-value"
			)
			var revisionHistoryLimit int32 = 8

			By("Setting both annotations and componentConfigs")
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, esc)).To(Succeed())

			esc.Spec.ControllerConfig.Annotations = []operatorv1alpha1.Annotation{
				{
					KVPair: operatorv1alpha1.KVPair{
						Key:   annotationKey,
						Value: annotationValue,
					},
				},
			}
			esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CoreController,
					DeploymentConfig: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(revisionHistoryLimit),
					},
					OverrideEnv: []corev1.EnvVar{
						{
							Name:  "GOMAXPROCS",
							Value: "2",
						},
					},
				},
			}
			Expect(ctrlClient.Update(ctx, esc)).To(Succeed())

			By("Verifying annotations, revisionHistoryLimit, and env vars are all applied")
			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				err := ctrlClient.Get(ctx, types.NamespacedName{
					Name:      "external-secrets",
					Namespace: operandNamespace,
				}, deployment)
				g.Expect(err).NotTo(HaveOccurred())

				// Check annotation
				g.Expect(deployment.Annotations).To(HaveKeyWithValue(annotationKey, annotationValue))
				g.Expect(deployment.Spec.Template.Annotations).To(HaveKeyWithValue(annotationKey, annotationValue))

				// Check revisionHistoryLimit
				g.Expect(deployment.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deployment.Spec.RevisionHistoryLimit).To(Equal(revisionHistoryLimit))

				// Check env var
				g.Expect(deployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
				container := deployment.Spec.Template.Spec.Containers[0]
				found := false
				for _, env := range container.Env {
					if env.Name == "GOMAXPROCS" && env.Value == "2" {
						found = true
						break
					}
				}
				g.Expect(found).To(BeTrue(), "GOMAXPROCS=2 should be present")
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})
})
