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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/external-secrets-operator/test/utils"
)

const (
	componentConfigTestTimeout  = 2 * time.Minute
	componentConfigTestInterval = 5 * time.Second
)

var escGVR = schema.GroupVersionResource{
	Group:    "operator.openshift.io",
	Version:  "v1alpha1",
	Resource: "externalsecretsconfigs",
}

// Diff-suggested: EP-1898 introduces annotations, componentConfig, overrideEnv, and new ComponentName values.
// These tests verify end-to-end behavior of the new controller configuration fields.
var _ = Describe("ExternalSecretsConfig Component Configuration (EP-1898)", Ordered, func() {
	ctx := context.TODO()
	var (
		clientset     *kubernetes.Clientset
		dynamicClient *dynamic.DynamicClient
		escName       = "cluster"
	)

	BeforeAll(func() {
		var err error
		clientset, err = kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		dynamicClient, err = dynamic.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for operator pod to be ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespace, []string{
			operatorPodPrefix,
		})).To(Succeed())

		By("Creating ExternalSecretsConfig CR with network policies")
		esc := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "operator.openshift.io/v1alpha1",
				"kind":       "ExternalSecretsConfig",
				"metadata": map[string]interface{}{
					"name": escName,
				},
				"spec": map[string]interface{}{
					"controllerConfig": map[string]interface{}{
						"networkPolicies": []interface{}{
							map[string]interface{}{
								"name":          "allow-core-egress",
								"componentName": "ExternalSecretsCoreController",
								"egress": []interface{}{
									map[string]interface{}{
										"ports": []interface{}{
											map[string]interface{}{"protocol": "TCP", "port": 6443},
											map[string]interface{}{"protocol": "TCP", "port": 443},
										},
									},
								},
							},
							map[string]interface{}{
								"name":          "allow-webhook-egress",
								"componentName": "Webhook",
								"egress": []interface{}{
									map[string]interface{}{
										"ports": []interface{}{
											map[string]interface{}{"protocol": "TCP", "port": 6443},
											map[string]interface{}{"protocol": "TCP", "port": 443},
										},
									},
								},
							},
							map[string]interface{}{
								"name":          "allow-cert-controller-egress",
								"componentName": "CertController",
								"egress": []interface{}{
									map[string]interface{}{
										"ports": []interface{}{
											map[string]interface{}{"protocol": "TCP", "port": 6443},
											map[string]interface{}{"protocol": "TCP", "port": 443},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		_, err = dynamicClient.Resource(escGVR).Create(ctx, esc, metav1.CreateOptions{})
		if err != nil {
			// CR may already exist, try to get it
			_, getErr := dynamicClient.Resource(escGVR).Get(ctx, escName, metav1.GetOptions{})
			Expect(getErr).NotTo(HaveOccurred(), "ExternalSecretsConfig CR should either be created or already exist")
		}

		By("Waiting for operand pods to become ready")
		Eventually(func() error {
			return utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
				operandCoreControllerPodPrefix,
				operandCertControllerPodPrefix,
				operandWebhookPodPrefix,
			})
		}, componentConfigTestTimeout, componentConfigTestInterval).Should(Succeed())
	})

	AfterAll(func() {
		By("Cleaning up ExternalSecretsConfig CR")
		_ = dynamicClient.Resource(escGVR).Delete(ctx, escName, metav1.DeleteOptions{})
	})

	// Diff-suggested: New field controllerConfig.annotations (EP-1898)
	Context("Custom Annotations (TC-01, TC-10)", func() {
		It("should propagate custom annotations to all operand Deployments and Pod templates", func() {
			By("Patching ExternalSecretsConfig with a custom annotation")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"annotations": [
							{"key": "example.com/test-annotation", "value": "e2e-value"}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(escGVR).Patch(ctx, escName, types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying annotations are applied to all operand Deployments")
			deploymentNames := []string{"external-secrets", "external-secrets-webhook", "external-secrets-cert-controller"}
			for _, deployName := range deploymentNames {
				Eventually(func(g Gomega) {
					deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, deployName, metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())

					g.Expect(deploy.Annotations).To(HaveKeyWithValue("example.com/test-annotation", "e2e-value"),
						"Deployment %s should have the custom annotation", deployName)
					g.Expect(deploy.Spec.Template.Annotations).To(HaveKeyWithValue("example.com/test-annotation", "e2e-value"),
						"Pod template of Deployment %s should have the custom annotation", deployName)
				}, componentConfigTestTimeout, componentConfigTestInterval).Should(Succeed(),
					"Deployment %s should have the custom annotation", deployName)
			}
		})

		It("should update annotations after initial creation", func() {
			By("Updating the annotation value and adding a second annotation")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"annotations": [
							{"key": "example.com/test-annotation", "value": "updated-value"},
							{"key": "example.com/second-annotation", "value": "new-value"}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(escGVR).Patch(ctx, escName, types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying updated annotations on all operand Deployments")
			deploymentNames := []string{"external-secrets", "external-secrets-webhook", "external-secrets-cert-controller"}
			for _, deployName := range deploymentNames {
				Eventually(func(g Gomega) {
					deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, deployName, metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())

					g.Expect(deploy.Annotations).To(HaveKeyWithValue("example.com/test-annotation", "updated-value"))
					g.Expect(deploy.Annotations).To(HaveKeyWithValue("example.com/second-annotation", "new-value"))
					g.Expect(deploy.Spec.Template.Annotations).To(HaveKeyWithValue("example.com/test-annotation", "updated-value"))
					g.Expect(deploy.Spec.Template.Annotations).To(HaveKeyWithValue("example.com/second-annotation", "new-value"))
				}, componentConfigTestTimeout, componentConfigTestInterval).Should(Succeed(),
					"Deployment %s should have updated annotations", deployName)
			}
		})
	})

	// Diff-suggested: CEL validation rule for reserved annotation prefixes (EP-1898)
	Context("Reserved Annotation Prefix Rejection (TC-02)", func() {
		DescribeTable("should reject annotations with reserved prefixes",
			func(annotationKey string) {
				patch := []byte(fmt.Sprintf(`{
					"spec": {
						"controllerConfig": {
							"annotations": [
								{"key": %q, "value": "forbidden"}
							]
						}
					}
				}`, annotationKey))
				_, err := dynamicClient.Resource(escGVR).Patch(ctx, escName, types.MergePatchType, patch, metav1.PatchOptions{})
				Expect(err).To(HaveOccurred(), "API server should reject annotations with reserved prefix %s", annotationKey)
				Expect(err.Error()).To(ContainSubstring("reserved"))
			},
			Entry("kubernetes.io/ prefix", "kubernetes.io/custom"),
			Entry("app.kubernetes.io/ prefix", "app.kubernetes.io/name"),
			Entry("openshift.io/ prefix", "openshift.io/test"),
			Entry("k8s.io/ prefix", "k8s.io/test"),
		)
	})

	// Diff-suggested: New field componentConfig.deploymentConfigs.revisionHistoryLimit (EP-1898)
	Context("ComponentConfig revisionHistoryLimit (TC-03, TC-06)", func() {
		It("should apply revisionHistoryLimit to the targeted component's Deployment", func() {
			By("Setting revisionHistoryLimit=5 for ExternalSecretsCoreController")
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
			_, err := dynamicClient.Resource(escGVR).Patch(ctx, escName, types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying revisionHistoryLimit on the external-secrets Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(5)))
			}, componentConfigTestTimeout, componentConfigTestInterval).Should(Succeed())
		})

		It("should apply different revisionHistoryLimit values to multiple components", func() {
			By("Setting revisionHistoryLimit=10 for CoreController and revisionHistoryLimit=3 for Webhook")
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
			_, err := dynamicClient.Resource(escGVR).Patch(ctx, escName, types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying revisionHistoryLimit on the external-secrets Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(10)))
			}, componentConfigTestTimeout, componentConfigTestInterval).Should(Succeed())

			By("Verifying revisionHistoryLimit on the external-secrets-webhook Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(3)))
			}, componentConfigTestTimeout, componentConfigTestInterval).Should(Succeed())
		})
	})

	// Diff-suggested: New field componentConfig.overrideEnv (EP-1898)
	Context("ComponentConfig overrideEnv (TC-04)", func() {
		It("should apply custom environment variables to the targeted component's container", func() {
			By("Setting GOMAXPROCS=4 for the Webhook component")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"componentConfig": [
							{
								"componentName": "Webhook",
								"overrideEnv": [
									{"name": "GOMAXPROCS", "value": "4"}
								]
							}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(escGVR).Patch(ctx, escName, types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the environment variable on the webhook Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())

				found := findEnvVar(deploy, "webhook", "GOMAXPROCS", "4")
				g.Expect(found).To(BeTrue(), "GOMAXPROCS=4 should be present in webhook container env vars")
			}, componentConfigTestTimeout, componentConfigTestInterval).Should(Succeed())
		})
	})

	// Diff-suggested: CEL validation rule for reserved env var prefixes (EP-1898)
	Context("Reserved Env Var Prefix Rejection (TC-05)", func() {
		DescribeTable("should reject env vars with reserved prefixes",
			func(envVarName string) {
				patch := []byte(fmt.Sprintf(`{
					"spec": {
						"controllerConfig": {
							"componentConfig": [
								{
									"componentName": "ExternalSecretsCoreController",
									"overrideEnv": [
										{"name": %q, "value": "test-value"}
									]
								}
							]
						}
					}
				}`, envVarName))
				_, err := dynamicClient.Resource(escGVR).Patch(ctx, escName, types.MergePatchType, patch, metav1.PatchOptions{})
				Expect(err).To(HaveOccurred(), "API server should reject env vars with reserved prefix %s", envVarName)
				Expect(err.Error()).To(ContainSubstring("reserved"))
			},
			Entry("KUBERNETES_ prefix", "KUBERNETES_SERVICE_HOST"),
			Entry("EXTERNAL_SECRETS_ prefix", "EXTERNAL_SECRETS_FOO"),
			Entry("HOSTNAME prefix", "HOSTNAME"),
		)
	})

	// Diff-suggested: CEL validation for duplicate componentName entries (EP-1898)
	Context("Duplicate ComponentName Rejection (TC-07)", func() {
		It("should reject duplicate componentName entries in componentConfig", func() {
			// We need to use an Apply patch or direct unstructured update to send duplicates.
			// MergePatch may deduplicate. Use a full object update instead.
			esc, err := dynamicClient.Resource(escGVR).Get(ctx, escName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			// Attempt to set duplicate componentName entries via raw JSON
			componentConfig := []interface{}{
				map[string]interface{}{
					"componentName":     "ExternalSecretsCoreController",
					"deploymentConfigs": map[string]interface{}{"revisionHistoryLimit": 5},
				},
				map[string]interface{}{
					"componentName":     "ExternalSecretsCoreController",
					"deploymentConfigs": map[string]interface{}{"revisionHistoryLimit": 3},
				},
			}

			spec, ok := esc.Object["spec"].(map[string]interface{})
			if !ok {
				spec = map[string]interface{}{}
			}
			controllerConfig, ok := spec["controllerConfig"].(map[string]interface{})
			if !ok {
				controllerConfig = map[string]interface{}{}
			}
			controllerConfig["componentConfig"] = componentConfig
			spec["controllerConfig"] = controllerConfig
			esc.Object["spec"] = spec

			_, err = dynamicClient.Resource(escGVR).Update(ctx, esc, metav1.UpdateOptions{})
			Expect(err).To(HaveOccurred(), "API server should reject duplicate componentName entries")
			Expect(err.Error()).To(ContainSubstring("unique"))
		})
	})

	// Diff-suggested: New ComponentName values Webhook and CertController for NetworkPolicy (EP-1898)
	Context("Network Policy with New Component Names (TC-08, TC-09)", func() {
		It("should create NetworkPolicy with correct pod selector for Webhook component", func() {
			By("Verifying NetworkPolicy exists for Webhook component")
			Eventually(func(g Gomega) {
				npList, err := clientset.NetworkingV1().NetworkPolicies(operandNamespace).List(ctx, metav1.ListOptions{})
				g.Expect(err).NotTo(HaveOccurred())

				found := false
				for _, np := range npList.Items {
					if selectorValue, ok := np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"]; ok {
						if selectorValue == "external-secrets-webhook" {
							found = true
							break
						}
					}
				}
				g.Expect(found).To(BeTrue(), "NetworkPolicy with pod selector for external-secrets-webhook should exist")
			}, componentConfigTestTimeout, componentConfigTestInterval).Should(Succeed())
		})

		It("should create NetworkPolicy with correct pod selector for CertController component", func() {
			By("Verifying NetworkPolicy exists for CertController component")
			Eventually(func(g Gomega) {
				npList, err := clientset.NetworkingV1().NetworkPolicies(operandNamespace).List(ctx, metav1.ListOptions{})
				g.Expect(err).NotTo(HaveOccurred())

				found := false
				for _, np := range npList.Items {
					if selectorValue, ok := np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"]; ok {
						if selectorValue == "external-secrets-cert-controller" {
							found = true
							break
						}
					}
				}
				g.Expect(found).To(BeTrue(), "NetworkPolicy with pod selector for external-secrets-cert-controller should exist")
			}, componentConfigTestTimeout, componentConfigTestInterval).Should(Succeed())
		})
	})

	// Diff-suggested: Combined configuration test (EP-1898)
	Context("Reconciliation with Combined Config (TC-11)", func() {
		It("should successfully reconcile with annotations, componentConfig, and networkPolicies configured together", func() {
			By("Setting up combined configuration")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"annotations": [
							{"key": "example.com/combined-test", "value": "active"}
						],
						"componentConfig": [
							{
								"componentName": "ExternalSecretsCoreController",
								"deploymentConfigs": {"revisionHistoryLimit": 7},
								"overrideEnv": [{"name": "CUSTOM_E2E_VAR", "value": "combined-test"}]
							},
							{
								"componentName": "Webhook",
								"deploymentConfigs": {"revisionHistoryLimit": 4}
							},
							{
								"componentName": "CertController",
								"overrideEnv": [{"name": "CERT_E2E_VAR", "value": "cert-test"}]
							}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(escGVR).Patch(ctx, escName, types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying all operand pods are ready after combined update")
			Eventually(func() error {
				return utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
					operandCoreControllerPodPrefix,
					operandCertControllerPodPrefix,
					operandWebhookPodPrefix,
				})
			}, componentConfigTestTimeout, componentConfigTestInterval).Should(Succeed())

			By("Verifying annotation and revisionHistoryLimit on core controller Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Annotations).To(HaveKeyWithValue("example.com/combined-test", "active"))
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(7)))
			}, componentConfigTestTimeout, componentConfigTestInterval).Should(Succeed())

			By("Verifying revisionHistoryLimit on webhook Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(4)))
			}, componentConfigTestTimeout, componentConfigTestInterval).Should(Succeed())

			By("Verifying overrideEnv on cert-controller Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-cert-controller", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())

				found := findEnvVar(deploy, "cert-controller", "CERT_E2E_VAR", "cert-test")
				g.Expect(found).To(BeTrue(), "CERT_E2E_VAR=cert-test should be in cert-controller container")
			}, componentConfigTestTimeout, componentConfigTestInterval).Should(Succeed())
		})
	})
})

// findEnvVar checks if an environment variable exists in a deployment's container.
func findEnvVar(deploy *appsv1.Deployment, containerName, envName, envValue string) bool {
	for _, container := range deploy.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			for _, env := range container.Env {
				if env.Name == envName && env.Value == envValue {
					return true
				}
			}
		}
	}
	return false
}
