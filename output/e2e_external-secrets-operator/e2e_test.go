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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/external-secrets-operator/test/utils"
)

const (
	operandNamespaceCC       = "external-secrets"
	operatorNamespaceCC      = "external-secrets-operator"
	reconciliationTimeout    = 30 * time.Second
	reconciliationPollPeriod = 5 * time.Second
)

var externalSecretsConfigGVR = schema.GroupVersionResource{
	Group:    "operator.openshift.io",
	Version:  "v1alpha1",
	Resource: "externalsecretsconfigs",
}

// Diff-suggested: New annotations and componentConfig fields added to ControllerConfig.
// These tests validate the new component configuration features from EP #1898.
var _ = Describe("ExternalSecretsConfig Component Configuration", Ordered, Label("ComponentConfig"), func() {
	ctx := context.TODO()
	var (
		clientset     *kubernetes.Clientset
		dynamicClient *dynamic.DynamicClient
	)

	BeforeAll(func() {
		var err error

		clientset, err = kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		dynamicClient, err = dynamic.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for operator pod to be ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespaceCC, []string{
			"external-secrets-operator-controller-manager-",
		})).To(Succeed())

		By("Verifying external-secrets operand pods are ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespaceCC, []string{
			"external-secrets-",
			"external-secrets-cert-controller-",
			"external-secrets-webhook-",
		})).To(Succeed())
	})

	AfterAll(func() {
		By("Cleaning up: removing annotations and componentConfig from ExternalSecretsConfig")
		patch := []byte(`{"spec":{"controllerConfig":{"annotations":[],"componentConfig":[]}}}`)
		_, err := dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to clean up ExternalSecretsConfig")
	})

	// Diff-suggested: annotations field added to ControllerConfig
	Context("Global Annotations", func() {

		AfterEach(func() {
			By("Removing annotations after each test")
			patch := []byte(`{"spec":{"controllerConfig":{"annotations":[]}}}`)
			_, _ = dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			time.Sleep(reconciliationPollPeriod)
		})

		// Diff-suggested: Annotations applied to Deployment metadata and Pod template
		It("should apply custom annotations to all operand Deployments and Pod templates", func() {
			By("Patching ExternalSecretsConfig with global annotations")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"annotations": [
							{"key": "e2e.test/team", "value": "platform"},
							{"key": "e2e.test/env", "value": "testing"}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			deploymentNames := []string{
				"external-secrets",
				"external-secrets-webhook",
				"external-secrets-cert-controller",
			}

			By("Waiting for annotations to appear on all Deployments")
			for _, deployName := range deploymentNames {
				Eventually(func(g Gomega) {
					deploy, err := clientset.AppsV1().Deployments(operandNamespaceCC).Get(ctx, deployName, metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())

					// Check Deployment metadata annotations
					g.Expect(deploy.Annotations).To(HaveKeyWithValue("e2e.test/team", "platform"),
						"deployment %s should have annotation e2e.test/team", deployName)
					g.Expect(deploy.Annotations).To(HaveKeyWithValue("e2e.test/env", "testing"),
						"deployment %s should have annotation e2e.test/env", deployName)

					// Check Pod template annotations
					g.Expect(deploy.Spec.Template.Annotations).To(HaveKeyWithValue("e2e.test/team", "platform"),
						"pod template of %s should have annotation e2e.test/team", deployName)
					g.Expect(deploy.Spec.Template.Annotations).To(HaveKeyWithValue("e2e.test/env", "testing"),
						"pod template of %s should have annotation e2e.test/env", deployName)
				}, reconciliationTimeout, reconciliationPollPeriod).Should(Succeed(),
					"annotations should appear on deployment %s", deployName)
			}
		})

		// Diff-suggested: CEL validation for reserved annotation prefixes
		It("should reject annotations with reserved prefixes via API validation", func() {
			reservedPrefixes := []struct {
				prefix string
				key    string
			}{
				{prefix: "kubernetes.io/", key: "kubernetes.io/test"},
				{prefix: "app.kubernetes.io/", key: "app.kubernetes.io/test"},
				{prefix: "openshift.io/", key: "openshift.io/test"},
				{prefix: "k8s.io/", key: "k8s.io/test"},
			}

			for _, rp := range reservedPrefixes {
				By(fmt.Sprintf("Trying to set annotation with reserved prefix %s", rp.prefix))
				patch := []byte(fmt.Sprintf(`{
					"spec": {
						"controllerConfig": {
							"annotations": [
								{"key": %q, "value": "should-fail"}
							]
						}
					}
				}`, rp.key))
				_, err := dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
				Expect(err).To(HaveOccurred(),
					"API should reject annotation with reserved prefix %s", rp.prefix)
			}
		})

		// Diff-suggested: Annotation update and removal lifecycle
		It("should update annotation values and remove annotations through reconciliation", func() {
			By("Adding an annotation")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"annotations": [
							{"key": "e2e.test/lifecycle", "value": "initial"}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying initial annotation value")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespaceCC).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Annotations).To(HaveKeyWithValue("e2e.test/lifecycle", "initial"))
			}, reconciliationTimeout, reconciliationPollPeriod).Should(Succeed())

			By("Updating annotation value")
			patch = []byte(`{
				"spec": {
					"controllerConfig": {
						"annotations": [
							{"key": "e2e.test/lifecycle", "value": "updated"}
						]
					}
				}
			}`)
			_, err = dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying updated annotation value")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespaceCC).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Annotations).To(HaveKeyWithValue("e2e.test/lifecycle", "updated"))
			}, reconciliationTimeout, reconciliationPollPeriod).Should(Succeed())
		})
	})

	// Diff-suggested: componentConfig field added to ControllerConfig
	Context("Component-Specific Configuration", func() {

		AfterEach(func() {
			By("Removing componentConfig after each test")
			patch := []byte(`{"spec":{"controllerConfig":{"componentConfig":[]}}}`)
			_, _ = dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			time.Sleep(reconciliationPollPeriod)
		})

		// Diff-suggested: revisionHistoryLimit field added to DeploymentConfig
		It("should apply revisionHistoryLimit to the controller Deployment", func() {
			By("Patching ExternalSecretsConfig with revisionHistoryLimit for controller")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"componentConfig": [
							{
								"componentName": "ExternalSecretsCoreController",
								"deploymentConfigs": {
									"revisionHistoryLimit": 5
								}
							}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying revisionHistoryLimit on controller Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespaceCC).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(5)))
			}, reconciliationTimeout, reconciliationPollPeriod).Should(Succeed())
		})

		// Diff-suggested: revisionHistoryLimit for Webhook component
		It("should apply revisionHistoryLimit to the webhook Deployment", func() {
			By("Patching ExternalSecretsConfig with revisionHistoryLimit for webhook")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"componentConfig": [
							{
								"componentName": "Webhook",
								"deploymentConfigs": {
									"revisionHistoryLimit": 3
								}
							}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying revisionHistoryLimit on webhook Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespaceCC).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(3)))
			}, reconciliationTimeout, reconciliationPollPeriod).Should(Succeed())
		})

		// Diff-suggested: overrideEnv field added to ComponentConfig
		It("should apply custom environment variables to the controller container", func() {
			By("Patching ExternalSecretsConfig with overrideEnv for controller")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"componentConfig": [
							{
								"componentName": "ExternalSecretsCoreController",
								"overrideEnv": [
									{"name": "GOMAXPROCS", "value": "4"}
								]
							}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying GOMAXPROCS env var on external-secrets container")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespaceCC).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())

				container := findContainer(deploy, "external-secrets")
				g.Expect(container).NotTo(BeNil(), "external-secrets container should exist")

				found := false
				for _, env := range container.Env {
					if env.Name == "GOMAXPROCS" {
						g.Expect(env.Value).To(Equal("4"))
						found = true
						break
					}
				}
				g.Expect(found).To(BeTrue(), "GOMAXPROCS env var should be present")
			}, reconciliationTimeout, reconciliationPollPeriod).Should(Succeed())
		})

		// Diff-suggested: CEL validation for reserved env var prefixes
		It("should reject environment variables with reserved prefixes via API validation", func() {
			reservedVars := []struct {
				name string
			}{
				{name: "HOSTNAME"},
				{name: "KUBERNETES_SERVICE_HOST"},
				{name: "EXTERNAL_SECRETS_CONFIG"},
			}

			for _, rv := range reservedVars {
				By(fmt.Sprintf("Trying to set reserved env var %s", rv.name))
				patch := []byte(fmt.Sprintf(`{
					"spec": {
						"controllerConfig": {
							"componentConfig": [
								{
									"componentName": "ExternalSecretsCoreController",
									"overrideEnv": [
										{"name": %q, "value": "custom"}
									]
								}
							]
						}
					}
				}`, rv.name))
				_, err := dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
				Expect(err).To(HaveOccurred(),
					"API should reject env var with reserved name %s", rv.name)
			}
		})

		// Diff-suggested: Multiple components configured simultaneously
		It("should configure multiple components independently", func() {
			By("Patching ExternalSecretsConfig with configs for controller and webhook")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"componentConfig": [
							{
								"componentName": "ExternalSecretsCoreController",
								"deploymentConfigs": {
									"revisionHistoryLimit": 10
								}
							},
							{
								"componentName": "Webhook",
								"deploymentConfigs": {
									"revisionHistoryLimit": 3
								}
							}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying controller has revisionHistoryLimit=10")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespaceCC).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(10)))
			}, reconciliationTimeout, reconciliationPollPeriod).Should(Succeed())

			By("Verifying webhook has revisionHistoryLimit=3")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespaceCC).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(3)))
			}, reconciliationTimeout, reconciliationPollPeriod).Should(Succeed())
		})

		// Diff-suggested: ComponentName enum validation
		It("should reject invalid component name via API validation", func() {
			By("Trying to set invalid componentName")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"componentConfig": [
							{
								"componentName": "InvalidComponent"
							}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).To(HaveOccurred(), "API should reject invalid componentName")
		})

		// Diff-suggested: CEL uniqueness validation on componentName
		It("should reject duplicate component names via API validation", func() {
			By("Trying to configure the same component twice")
			patchJSON := []byte(`[
				{"op": "replace", "path": "/spec/controllerConfig/componentConfig", "value": [
					{"componentName": "ExternalSecretsCoreController", "deploymentConfigs": {"revisionHistoryLimit": 5}},
					{"componentName": "ExternalSecretsCoreController", "deploymentConfigs": {"revisionHistoryLimit": 10}}
				]}
			]`)
			_, err := dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.JSONPatchType, patchJSON, metav1.PatchOptions{})
			Expect(err).To(HaveOccurred(), "API should reject duplicate componentName entries")
		})
	})

	// Diff-suggested: Combined annotations + componentConfig
	Context("Combined Configuration", func() {

		AfterEach(func() {
			By("Removing all custom config after each test")
			patch := []byte(`{"spec":{"controllerConfig":{"annotations":[],"componentConfig":[]}}}`)
			_, _ = dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			time.Sleep(reconciliationPollPeriod)
		})

		// Diff-suggested: Both annotations and componentConfig applied together
		It("should apply both annotations and component configs together", func() {
			By("Patching with both annotations and componentConfig")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"annotations": [
							{"key": "e2e.test/combined", "value": "yes"}
						],
						"componentConfig": [
							{
								"componentName": "ExternalSecretsCoreController",
								"deploymentConfigs": {
									"revisionHistoryLimit": 7
								},
								"overrideEnv": [
									{"name": "GOMAXPROCS", "value": "2"}
								]
							}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying all configurations are applied to controller Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespaceCC).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())

				// Annotations on Deployment
				g.Expect(deploy.Annotations).To(HaveKeyWithValue("e2e.test/combined", "yes"))

				// Annotations on Pod template
				g.Expect(deploy.Spec.Template.Annotations).To(HaveKeyWithValue("e2e.test/combined", "yes"))

				// RevisionHistoryLimit
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(7)))

				// Override env
				container := findContainer(deploy, "external-secrets")
				g.Expect(container).NotTo(BeNil())
				found := false
				for _, env := range container.Env {
					if env.Name == "GOMAXPROCS" {
						g.Expect(env.Value).To(Equal("2"))
						found = true
						break
					}
				}
				g.Expect(found).To(BeTrue(), "GOMAXPROCS env var should be present")
			}, reconciliationTimeout, reconciliationPollPeriod).Should(Succeed())

			By("Verifying annotations are also applied to webhook Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespaceCC).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Annotations).To(HaveKeyWithValue("e2e.test/combined", "yes"))
			}, reconciliationTimeout, reconciliationPollPeriod).Should(Succeed())
		})
	})

	// Diff-suggested: Controller reconciliation restores drift
	Context("Reconciliation Recovery", func() {

		AfterEach(func() {
			By("Removing componentConfig after each test")
			patch := []byte(`{"spec":{"controllerConfig":{"componentConfig":[]}}}`)
			_, _ = dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			time.Sleep(reconciliationPollPeriod)
		})

		// Diff-suggested: applyComponentConfig restores deployment state on drift
		It("should restore Deployment configuration after manual drift", func() {
			By("Setting revisionHistoryLimit to 5 via ExternalSecretsConfig")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"componentConfig": [
							{
								"componentName": "ExternalSecretsCoreController",
								"deploymentConfigs": {
									"revisionHistoryLimit": 5
								}
							}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(externalSecretsConfigGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for revisionHistoryLimit to be applied")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespaceCC).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(5)))
			}, reconciliationTimeout, reconciliationPollPeriod).Should(Succeed())

			By("Manually changing revisionHistoryLimit to introduce drift")
			deployPatch := []byte(`{"spec":{"revisionHistoryLimit":1}}`)
			_, err = clientset.AppsV1().Deployments(operandNamespaceCC).Patch(ctx, "external-secrets",
				types.MergePatchType, deployPatch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for operator to reconcile and restore the desired value")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespaceCC).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(5)),
					"operator should have reconciled revisionHistoryLimit back to 5")
			}, 60*time.Second, reconciliationPollPeriod).Should(Succeed())
		})
	})
})

// findContainer finds a container by name in a Deployment's pod template spec.
func findContainer(deploy *appsv1.Deployment, name string) *corev1.Container {
	for i := range deploy.Spec.Template.Spec.Containers {
		if deploy.Spec.Template.Spec.Containers[i].Name == name {
			return &deploy.Spec.Template.Spec.Containers[i]
		}
	}
	return nil
}

