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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/external-secrets-operator/test/utils"
)

var (
	escGVR = schema.GroupVersionResource{
		Group:    "operator.openshift.io",
		Version:  "v1alpha1",
		Resource: "externalsecretsconfigs",
	}
)

var _ = Describe("Component Configuration Overrides (EP-1898)", Ordered, func() {
	ctx := context.TODO()
	var (
		clientset     *kubernetes.Clientset
		dynamicClient dynamic.Interface
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
	})

	// Diff-suggested: New annotations field in ControllerConfig — verify annotations are applied to Deployments
	Context("Global Annotations", func() {
		const annotationKey = "example.com/e2e-component-config-test"
		const annotationValue = "test-value"

		AfterAll(func() {
			By("Cleaning up ExternalSecretsConfig")
			_ = dynamicClient.Resource(escGVR).Delete(ctx, "cluster", metav1.DeleteOptions{})
			// Wait for cleanup to complete
			time.Sleep(5 * time.Second)
		})

		It("should apply custom annotations to all operand Deployments and Pod templates", func() {
			By("Creating ExternalSecretsConfig with global annotations")
			esc := buildESCWithAnnotations(annotationKey, annotationValue)
			_, err := dynamicClient.Resource(escGVR).Create(ctx, esc, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for ExternalSecretsConfig to become Ready")
			Expect(waitForESCReady(ctx, dynamicClient, 2*time.Minute)).To(Succeed())

			By("Waiting for operand pods to be ready")
			Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
				operandCoreControllerPodPrefix,
				operandCertControllerPodPrefix,
				operandWebhookPodPrefix,
			})).To(Succeed())

			By("Verifying annotations on controller Deployment")
			verifyDeploymentAnnotation(ctx, clientset, operandNamespace, "external-secrets", annotationKey, annotationValue)

			By("Verifying annotations on webhook Deployment")
			verifyDeploymentAnnotation(ctx, clientset, operandNamespace, "external-secrets-webhook", annotationKey, annotationValue)

			By("Verifying annotations on cert-controller Deployment")
			verifyDeploymentAnnotation(ctx, clientset, operandNamespace, "external-secrets-cert-controller", annotationKey, annotationValue)
		})
	})

	// Diff-suggested: New componentConfigs field — verify revisionHistoryLimit is applied
	Context("Per-Component revisionHistoryLimit", func() {
		AfterAll(func() {
			By("Cleaning up ExternalSecretsConfig")
			_ = dynamicClient.Resource(escGVR).Delete(ctx, "cluster", metav1.DeleteOptions{})
			time.Sleep(5 * time.Second)
		})

		It("should apply revisionHistoryLimit to the specified component Deployment", func() {
			By("Creating ExternalSecretsConfig with revisionHistoryLimit for controller")
			esc := buildESCWithRevisionHistoryLimit("ExternalSecretsCoreController", 5)
			_, err := dynamicClient.Resource(escGVR).Create(ctx, esc, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for ExternalSecretsConfig to become Ready")
			Expect(waitForESCReady(ctx, dynamicClient, 2*time.Minute)).To(Succeed())

			By("Waiting for operand pods to be ready")
			Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
				operandCoreControllerPodPrefix,
				operandCertControllerPodPrefix,
				operandWebhookPodPrefix,
			})).To(Succeed())

			By("Verifying revisionHistoryLimit on controller Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(5)))
			}, time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: New overrideEnv field — verify env vars are merged into containers
	Context("Per-Component overrideEnv", func() {
		AfterAll(func() {
			By("Cleaning up ExternalSecretsConfig")
			_ = dynamicClient.Resource(escGVR).Delete(ctx, "cluster", metav1.DeleteOptions{})
			time.Sleep(5 * time.Second)
		})

		It("should apply custom environment variables to the specified component container", func() {
			By("Creating ExternalSecretsConfig with overrideEnv for controller")
			esc := buildESCWithOverrideEnv("ExternalSecretsCoreController", "GOMAXPROCS", "4")
			_, err := dynamicClient.Resource(escGVR).Create(ctx, esc, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for ExternalSecretsConfig to become Ready")
			Expect(waitForESCReady(ctx, dynamicClient, 2*time.Minute)).To(Succeed())

			By("Waiting for operand pods to be ready")
			Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
				operandCoreControllerPodPrefix,
				operandCertControllerPodPrefix,
				operandWebhookPodPrefix,
			})).To(Succeed())

			By("Verifying GOMAXPROCS env var on controller Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.Template.Spec.Containers).NotTo(BeEmpty())

				found := false
				for _, env := range deploy.Spec.Template.Spec.Containers[0].Env {
					if env.Name == "GOMAXPROCS" && env.Value == "4" {
						found = true
						break
					}
				}
				g.Expect(found).To(BeTrue(), "GOMAXPROCS=4 not found in controller container env vars")
			}, time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: Combined annotations + componentConfig — verify both work together
	Context("Combined Annotations and ComponentConfig", func() {
		AfterAll(func() {
			By("Cleaning up ExternalSecretsConfig")
			_ = dynamicClient.Resource(escGVR).Delete(ctx, "cluster", metav1.DeleteOptions{})
			time.Sleep(5 * time.Second)
		})

		It("should apply both global annotations and per-component configs simultaneously", func() {
			By("Creating ExternalSecretsConfig with annotations and componentConfig")
			esc := buildESCCombined()
			_, err := dynamicClient.Resource(escGVR).Create(ctx, esc, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for ExternalSecretsConfig to become Ready")
			Expect(waitForESCReady(ctx, dynamicClient, 2*time.Minute)).To(Succeed())

			By("Waiting for operand pods to be ready")
			Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
				operandCoreControllerPodPrefix,
				operandCertControllerPodPrefix,
				operandWebhookPodPrefix,
			})).To(Succeed())

			By("Verifying annotations on controller Deployment")
			verifyDeploymentAnnotation(ctx, clientset, operandNamespace, "external-secrets", "example.com/combined-test", "combined-value")

			By("Verifying revisionHistoryLimit on controller Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(10)))
			}, time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying revisionHistoryLimit on webhook Deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(3)))
			}, time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: Update scenario — verify that updating componentConfig triggers re-reconciliation
	Context("Update revisionHistoryLimit", func() {
		AfterAll(func() {
			By("Cleaning up ExternalSecretsConfig")
			_ = dynamicClient.Resource(escGVR).Delete(ctx, "cluster", metav1.DeleteOptions{})
			time.Sleep(5 * time.Second)
		})

		It("should update the Deployment when revisionHistoryLimit is changed", func() {
			By("Creating ExternalSecretsConfig with initial revisionHistoryLimit")
			esc := buildESCWithRevisionHistoryLimit("ExternalSecretsCoreController", 5)
			_, err := dynamicClient.Resource(escGVR).Create(ctx, esc, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for ExternalSecretsConfig to become Ready")
			Expect(waitForESCReady(ctx, dynamicClient, 2*time.Minute)).To(Succeed())

			By("Verifying initial revisionHistoryLimit")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(5)))
			}, time.Minute, 5*time.Second).Should(Succeed())

			By("Updating revisionHistoryLimit to 10")
			updatedESC := buildESCWithRevisionHistoryLimit("ExternalSecretsCoreController", 10)
			_, err = dynamicClient.Resource(escGVR).Update(ctx, updatedESC, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for re-reconciliation")
			Expect(waitForESCReady(ctx, dynamicClient, 2*time.Minute)).To(Succeed())

			By("Verifying updated revisionHistoryLimit")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(10)))
			}, time.Minute, 5*time.Second).Should(Succeed())
		})
	})
})

// Helper functions

func waitForESCReady(ctx context.Context, client dynamic.Interface, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		u, err := client.Resource(escGVR).Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		conds, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
		if err != nil || !found {
			return false, nil
		}

		for _, c := range conds {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if cond["type"] == "Ready" && cond["status"] == "True" {
				return true, nil
			}
		}
		return false, nil
	})
}

func verifyDeploymentAnnotation(ctx context.Context, clientset *kubernetes.Clientset, namespace, name, annotationKey, annotationValue string) {
	Eventually(func(g Gomega) {
		deploy, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())

		// Check deployment metadata annotations
		g.Expect(deploy.Annotations).To(HaveKeyWithValue(annotationKey, annotationValue),
			fmt.Sprintf("Deployment %s should have annotation %s=%s on metadata", name, annotationKey, annotationValue))

		// Check pod template annotations
		podAnnotations := deploy.Spec.Template.GetAnnotations()
		g.Expect(podAnnotations).To(HaveKeyWithValue(annotationKey, annotationValue),
			fmt.Sprintf("Deployment %s should have annotation %s=%s on pod template", name, annotationKey, annotationValue))
	}, time.Minute, 5*time.Second).Should(Succeed())
}

func buildESCWithAnnotations(key, value string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operator.openshift.io/v1alpha1",
			"kind":       "ExternalSecretsConfig",
			"metadata": map[string]interface{}{
				"name": "cluster",
			},
			"spec": map[string]interface{}{
				"controllerConfig": map[string]interface{}{
					"annotations": []interface{}{
						map[string]interface{}{
							"key":   key,
							"value": value,
						},
					},
					"networkPolicies": defaultNetworkPolicies(),
				},
			},
		},
	}
}

func buildESCWithRevisionHistoryLimit(componentName string, limit int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operator.openshift.io/v1alpha1",
			"kind":       "ExternalSecretsConfig",
			"metadata": map[string]interface{}{
				"name": "cluster",
			},
			"spec": map[string]interface{}{
				"controllerConfig": map[string]interface{}{
					"componentConfig": []interface{}{
						map[string]interface{}{
							"componentName": componentName,
							"deploymentConfigs": map[string]interface{}{
								"revisionHistoryLimit": limit,
							},
						},
					},
					"networkPolicies": defaultNetworkPolicies(),
				},
			},
		},
	}
}

func buildESCWithOverrideEnv(componentName, envName, envValue string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operator.openshift.io/v1alpha1",
			"kind":       "ExternalSecretsConfig",
			"metadata": map[string]interface{}{
				"name": "cluster",
			},
			"spec": map[string]interface{}{
				"controllerConfig": map[string]interface{}{
					"componentConfig": []interface{}{
						map[string]interface{}{
							"componentName": componentName,
							"overrideEnv": []interface{}{
								map[string]interface{}{
									"name":  envName,
									"value": envValue,
								},
							},
						},
					},
					"networkPolicies": defaultNetworkPolicies(),
				},
			},
		},
	}
}

func buildESCCombined() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operator.openshift.io/v1alpha1",
			"kind":       "ExternalSecretsConfig",
			"metadata": map[string]interface{}{
				"name": "cluster",
			},
			"spec": map[string]interface{}{
				"controllerConfig": map[string]interface{}{
					"annotations": []interface{}{
						map[string]interface{}{
							"key":   "example.com/combined-test",
							"value": "combined-value",
						},
					},
					"componentConfig": []interface{}{
						map[string]interface{}{
							"componentName": "ExternalSecretsCoreController",
							"deploymentConfigs": map[string]interface{}{
								"revisionHistoryLimit": int64(10),
							},
							"overrideEnv": []interface{}{
								map[string]interface{}{
									"name":  "GOMAXPROCS",
									"value": "4",
								},
							},
						},
						map[string]interface{}{
							"componentName": "Webhook",
							"deploymentConfigs": map[string]interface{}{
								"revisionHistoryLimit": int64(3),
							},
						},
					},
					"networkPolicies": defaultNetworkPolicies(),
				},
			},
		},
	}
}

func defaultNetworkPolicies() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"name":          "allow-external-secrets-egress",
			"componentName": "ExternalSecretsCoreController",
			"egress": []interface{}{
				map[string]interface{}{
					"to": []interface{}{},
					"ports": []interface{}{
						map[string]interface{}{"protocol": "TCP", "port": int64(6443)},
						map[string]interface{}{"protocol": "TCP", "port": int64(443)},
						map[string]interface{}{"protocol": "TCP", "port": int64(5353)},
						map[string]interface{}{"protocol": "UDP", "port": int64(5353)},
					},
				},
			},
		},
	}
}
