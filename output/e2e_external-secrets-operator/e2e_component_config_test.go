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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/external-secrets-operator/test/utils"
)

var escGVR = schema.GroupVersionResource{
	Group:    "operator.openshift.io",
	Version:  "v1alpha1",
	Resource: "externalsecretsconfigs",
}

// Diff-suggested: New componentConfig and annotations API fields added in EP#1898
var _ = Describe("ComponentConfig and Annotations E2E", Ordered, func() {
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

	AfterAll(func() {
		By("Cleaning up ExternalSecretsConfig")
		_ = dynamicClient.Resource(escGVR).Delete(ctx, "cluster", metav1.DeleteOptions{})
	})

	// Diff-suggested: annotations field added to ControllerConfig in API types
	Context("Custom Annotations", func() {
		It("should apply custom annotations to all operand deployments", func() {
			By("Creating ExternalSecretsConfig with custom annotations")
			esc := &unstructured.Unstructured{
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
									"key":   "example.com/e2e-annotation",
									"value": "test-value",
								},
							},
						},
					},
				},
			}
			_, err := dynamicClient.Resource(escGVR).Create(ctx, esc, metav1.CreateOptions{})
			if errors.IsAlreadyExists(err) {
				_, err = dynamicClient.Resource(escGVR).Update(ctx, esc, metav1.UpdateOptions{})
			}
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for ExternalSecretsConfig to become Ready")
			waitForESCReady(ctx, dynamicClient)

			By("Waiting for operand pods to be ready")
			Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
				operandCoreControllerPodPrefix,
				operandWebhookPodPrefix,
			})).To(Succeed())

			By("Verifying annotation on core controller deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Annotations).To(HaveKeyWithValue("example.com/e2e-annotation", "test-value"))
				g.Expect(deploy.Spec.Template.Annotations).To(HaveKeyWithValue("example.com/e2e-annotation", "test-value"))
			}, 2*time.Minute, 10*time.Second).Should(Succeed())

			By("Verifying annotation on webhook deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Annotations).To(HaveKeyWithValue("example.com/e2e-annotation", "test-value"))
				g.Expect(deploy.Spec.Template.Annotations).To(HaveKeyWithValue("example.com/e2e-annotation", "test-value"))
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: componentConfigs field added to ControllerConfig in API types
	Context("ComponentConfig RevisionHistoryLimit", func() {
		It("should apply revisionHistoryLimit to the targeted deployment", func() {
			By("Updating ExternalSecretsConfig with componentConfigs for core controller")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"componentConfigs": [
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
			_, err := dynamicClient.Resource(escGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for reconciliation")
			waitForESCReady(ctx, dynamicClient)

			By("Verifying revisionHistoryLimit on core controller deployment")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(5)))
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: overrideEnv field added to ComponentConfig in API types
	Context("ComponentConfig OverrideEnv", func() {
		It("should apply custom environment variables to the targeted container", func() {
			By("Updating ExternalSecretsConfig with overrideEnv for core controller")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"componentConfigs": [
							{
								"componentName": "ExternalSecretsCoreController",
								"deploymentConfigs": {
									"revisionHistoryLimit": 5
								},
								"overrideEnv": [
									{"name": "GOMAXPROCS", "value": "4"}
								]
							}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(escGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for reconciliation")
			waitForESCReady(ctx, dynamicClient)

			By("Verifying GOMAXPROCS env var on core controller container")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())

				container := findContainer(deploy, "external-secrets")
				g.Expect(container).NotTo(BeNil(), "container external-secrets should exist")

				envVal := findEnvValue(container.Env, "GOMAXPROCS")
				g.Expect(envVal).To(Equal("4"), "GOMAXPROCS should be set to 4")
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: Multiple componentConfigs entries with different componentName values
	Context("Multiple ComponentConfigs", func() {
		It("should apply different configurations to different components", func() {
			By("Updating ExternalSecretsConfig with multiple componentConfigs")
			patch := []byte(`{
				"spec": {
					"controllerConfig": {
						"componentConfigs": [
							{
								"componentName": "ExternalSecretsCoreController",
								"deploymentConfigs": {"revisionHistoryLimit": 10}
							},
							{
								"componentName": "Webhook",
								"deploymentConfigs": {"revisionHistoryLimit": 3}
							}
						]
					}
				}
			}`)
			_, err := dynamicClient.Resource(escGVR).Patch(ctx, "cluster", types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for reconciliation")
			waitForESCReady(ctx, dynamicClient)

			By("Verifying revisionHistoryLimit on core controller = 10")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(10)))
			}, 2*time.Minute, 10*time.Second).Should(Succeed())

			By("Verifying revisionHistoryLimit on webhook = 3")
			Eventually(func(g Gomega) {
				deploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
				g.Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(3)))
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})
	})
})

// waitForESCReady waits for ExternalSecretsConfig/cluster to have Ready=True condition.
func waitForESCReady(ctx context.Context, client dynamic.Interface) {
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
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
	Expect(err).NotTo(HaveOccurred(), "ExternalSecretsConfig/cluster should become Ready")
}

// findContainer returns a pointer to the container with the given name, or nil if not found.
func findContainer(deploy *appsv1.Deployment, name string) *corev1.Container {
	for i := range deploy.Spec.Template.Spec.Containers {
		if deploy.Spec.Template.Spec.Containers[i].Name == name {
			return &deploy.Spec.Template.Spec.Containers[i]
		}
	}
	return nil
}

// findEnvValue returns the value of the env var with the given name, or empty string if not found.
func findEnvValue(envVars []corev1.EnvVar, name string) string {
	for _, env := range envVars {
		if env.Name == name {
			return env.Value
		}
	}
	return ""
}
