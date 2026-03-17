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

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/external-secrets-operator/test/utils"
)

var _ = Describe("Network Policy E2E Tests", Ordered, func() {
	ctx := context.TODO()
	var (
		clientset     *kubernetes.Clientset
		dynamicClient *dynamic.DynamicClient
	)

	escGVR := schema.GroupVersionResource{
		Group:    "operator.openshift.io",
		Version:  "v1alpha1",
		Resource: "externalsecretsconfigs",
	}

	BeforeAll(func() {
		var err error
		clientset, err = kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		dynamicClient, err = dynamic.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for operator pod to be ready")
		Eventually(func(g Gomega) {
			pods, err := clientset.CoreV1().Pods(operatorNamespace).List(ctx, metav1.ListOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			found := false
			for _, pod := range pods.Items {
				if len(pod.Name) >= len(operatorPodPrefix) && pod.Name[:len(operatorPodPrefix)] == operatorPodPrefix {
					g.Expect(string(pod.Status.Phase)).To(Equal("Running"))
					found = true
				}
			}
			g.Expect(found).To(BeTrue(), "operator pod should exist")
		}, 2*time.Minute, 10*time.Second).Should(Succeed())
	})

	AfterAll(func() {
		By("Cleaning up ExternalSecretsConfig custom network policies")
		esc, err := dynamicClient.Resource(escGVR).Get(ctx, "cluster", metav1.GetOptions{})
		if err == nil {
			// Remove custom network policies by clearing the field
			unstructured.RemoveNestedField(esc.Object,
				"spec", "controllerConfig", "networkPolicies")
			_, _ = dynamicClient.Resource(escGVR).Update(ctx, esc, metav1.UpdateOptions{})
		}
	})

	// Diff-suggested: network policy static assets are created during reconciliation
	Context("Static Network Policies", func() {
		It("should create deny-all network policy in operand namespace", func() {
			By("Verifying deny-all-traffic NetworkPolicy exists")
			Eventually(func(g Gomega) {
				np, err := clientset.NetworkingV1().NetworkPolicies(operandNamespace).
					Get(ctx, "deny-all-traffic", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred(), "deny-all-traffic NetworkPolicy should exist")
				g.Expect(np.Spec.PodSelector).To(Equal(metav1.LabelSelector{}),
					"deny-all should select all pods")
				g.Expect(np.Spec.PolicyTypes).To(ContainElements(
					networkingv1.PolicyTypeIngress,
					networkingv1.PolicyTypeEgress,
				))
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})

		// Diff-suggested: static allow policies for main controller, webhook, cert-controller
		It("should create allow traffic policies for operand components", func() {
			expectedPolicies := []string{
				"allow-api-server-egress-for-main-controller-traffic",
				"allow-api-server-egress-for-webhook",
				"allow-to-dns",
			}

			for _, policyName := range expectedPolicies {
				By(fmt.Sprintf("Verifying %s NetworkPolicy exists", policyName))
				Eventually(func(g Gomega) {
					_, err := clientset.NetworkingV1().NetworkPolicies(operandNamespace).
						Get(ctx, policyName, metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred(),
						fmt.Sprintf("%s NetworkPolicy should exist", policyName))
				}, 2*time.Minute, 10*time.Second).Should(Succeed())
			}
		})

		// Diff-suggested: operator namespace should have its own network policy (shipped via OLM bundle)
		It("should have a network policy in the operator namespace", func() {
			By("Listing NetworkPolicies in operator namespace")
			Eventually(func(g Gomega) {
				npList, err := clientset.NetworkingV1().NetworkPolicies(operatorNamespace).
					List(ctx, metav1.ListOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(npList.Items)).To(BeNumerically(">", 0),
					"at least one NetworkPolicy should exist in operator namespace")
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: custom network policies from ExternalSecretsConfig API
	Context("Custom Network Policies", func() {
		It("should create a custom network policy from ExternalSecretsConfig spec", func() {
			customPolicyName := fmt.Sprintf("e2e-custom-np-%s", utils.GetRandomString(5))

			By("Updating ExternalSecretsConfig with a custom network policy")
			esc, err := dynamicClient.Resource(escGVR).Get(ctx, "cluster", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "should get ExternalSecretsConfig")

			networkPolicies := []interface{}{
				map[string]interface{}{
					"name":          customPolicyName,
					"componentName": "ExternalSecretsCoreController",
					"egress": []interface{}{
						map[string]interface{}{},
					},
				},
			}

			err = unstructured.SetNestedSlice(esc.Object, networkPolicies,
				"spec", "controllerConfig", "networkPolicies")
			Expect(err).NotTo(HaveOccurred())

			_, err = dynamicClient.Resource(escGVR).Update(ctx, esc, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred(), "should update ExternalSecretsConfig")

			By("Waiting for custom NetworkPolicy to be created")
			Eventually(func(g Gomega) {
				np, err := clientset.NetworkingV1().NetworkPolicies(operandNamespace).
					Get(ctx, customPolicyName, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred(),
					"custom NetworkPolicy should be created")
				g.Expect(np.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue(
					"app.kubernetes.io/name", "external-secrets"),
					"should target CoreController pods")
				g.Expect(np.Spec.PolicyTypes).To(ContainElement(networkingv1.PolicyTypeEgress))
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})

		// Diff-suggested: custom network policy egress update
		It("should update custom network policy egress rules when spec changes", func() {
			customPolicyName := fmt.Sprintf("e2e-update-np-%s", utils.GetRandomString(5))

			By("Creating ExternalSecretsConfig with initial custom network policy")
			esc, err := dynamicClient.Resource(escGVR).Get(ctx, "cluster", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			// Get existing network policies to preserve them (immutability rule)
			existingNPs, _, _ := unstructured.NestedSlice(esc.Object,
				"spec", "controllerConfig", "networkPolicies")

			newNP := map[string]interface{}{
				"name":          customPolicyName,
				"componentName": "ExternalSecretsCoreController",
				"egress": []interface{}{
					map[string]interface{}{
						"ports": []interface{}{
							map[string]interface{}{
								"protocol": "TCP",
								"port":     int64(443),
							},
						},
					},
				},
			}
			existingNPs = append(existingNPs, newNP)

			err = unstructured.SetNestedSlice(esc.Object, existingNPs,
				"spec", "controllerConfig", "networkPolicies")
			Expect(err).NotTo(HaveOccurred())

			_, err = dynamicClient.Resource(escGVR).Update(ctx, esc, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for initial custom NetworkPolicy")
			Eventually(func(g Gomega) {
				_, err := clientset.NetworkingV1().NetworkPolicies(operandNamespace).
					Get(ctx, customPolicyName, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
			}, 2*time.Minute, 10*time.Second).Should(Succeed())

			By("Updating egress rules to add port 8443")
			esc, err = dynamicClient.Resource(escGVR).Get(ctx, "cluster", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			nps, _, _ := unstructured.NestedSlice(esc.Object,
				"spec", "controllerConfig", "networkPolicies")

			for i, np := range nps {
				npMap, ok := np.(map[string]interface{})
				if !ok {
					continue
				}
				if npMap["name"] == customPolicyName {
					nps[i] = map[string]interface{}{
						"name":          customPolicyName,
						"componentName": "ExternalSecretsCoreController",
						"egress": []interface{}{
							map[string]interface{}{
								"ports": []interface{}{
									map[string]interface{}{
										"protocol": "TCP",
										"port":     int64(443),
									},
								},
							},
							map[string]interface{}{
								"ports": []interface{}{
									map[string]interface{}{
										"protocol": "TCP",
										"port":     int64(8443),
									},
								},
							},
						},
					}
				}
			}

			err = unstructured.SetNestedSlice(esc.Object, nps,
				"spec", "controllerConfig", "networkPolicies")
			Expect(err).NotTo(HaveOccurred())

			_, err = dynamicClient.Resource(escGVR).Update(ctx, esc, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the NetworkPolicy egress was updated")
			Eventually(func(g Gomega) {
				np, err := clientset.NetworkingV1().NetworkPolicies(operandNamespace).
					Get(ctx, customPolicyName, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(np.Spec.Egress)).To(Equal(2),
					"should have 2 egress rules after update")
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: runtime validation warning for misconfigured BitwardenSDKServer policies
	Context("Network Policy Misconfiguration Warning", func() {
		It("should emit a warning event when BitwardenSDKServer policy is configured without plugin enabled", func() {
			bwPolicyName := fmt.Sprintf("e2e-bw-np-%s", utils.GetRandomString(5))

			By("Updating ExternalSecretsConfig with BitwardenSDKServer network policy (plugin not enabled)")
			esc, err := dynamicClient.Resource(escGVR).Get(ctx, "cluster", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			existingNPs, _, _ := unstructured.NestedSlice(esc.Object,
				"spec", "controllerConfig", "networkPolicies")

			bwNP := map[string]interface{}{
				"name":          bwPolicyName,
				"componentName": "BitwardenSDKServer",
				"egress": []interface{}{
					map[string]interface{}{
						"ports": []interface{}{
							map[string]interface{}{
								"protocol": "TCP",
								"port":     int64(6443),
							},
						},
					},
				},
			}
			existingNPs = append(existingNPs, bwNP)

			err = unstructured.SetNestedSlice(esc.Object, existingNPs,
				"spec", "controllerConfig", "networkPolicies")
			Expect(err).NotTo(HaveOccurred())

			_, err = dynamicClient.Resource(escGVR).Update(ctx, esc, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for reconciliation and checking for warning event")
			Eventually(func(g Gomega) {
				events, err := clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{
					FieldSelector: "involvedObject.name=cluster,reason=NetworkPolicyMisconfiguration",
				})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(events.Items)).To(BeNumerically(">", 0),
					"should have a NetworkPolicyMisconfiguration warning event")
				g.Expect(events.Items[0].Type).To(Equal("Warning"))
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: verify ExternalSecretsConfig Ready condition after network policy changes
	Context("ExternalSecretsConfig Status", func() {
		It("should report Ready condition after network policies are reconciled", func() {
			By("Checking ExternalSecretsConfig status conditions")
			Eventually(func(g Gomega) {
				esc, err := dynamicClient.Resource(escGVR).Get(ctx, "cluster", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())

				conditions, found, err := unstructured.NestedSlice(esc.Object, "status", "conditions")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue(), "status.conditions should exist")

				readyFound := false
				for _, c := range conditions {
					cond, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					if cond["type"] == "Ready" && cond["status"] == "True" {
						readyFound = true
					}
				}
				g.Expect(readyFound).To(BeTrue(), "ExternalSecretsConfig should have Ready=True condition")
			}, 3*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	// Diff-suggested: webhook accessibility through network policies
	Context("Webhook Accessibility", func() {
		It("should allow webhook to receive admission requests through network policies", func() {
			By("Creating a test resource to trigger webhook validation")
			testNS := fmt.Sprintf("e2e-np-webhook-%s", utils.GetRandomString(5))

			// Create test namespace using dynamic client
			nsObj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Namespace",
					"metadata": map[string]interface{}{
						"name": testNS,
					},
				},
			}
			nsGVR := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
			_, err := dynamicClient.Resource(nsGVR).Create(ctx, nsObj, metav1.CreateOptions{})
			if err != nil {
				Skip(fmt.Sprintf("Could not create test namespace: %v", err))
			}

			defer func() {
				_ = dynamicClient.Resource(nsGVR).Delete(ctx, testNS, metav1.DeleteOptions{})
			}()

			// Attempt to create a SecretStore - this will go through the webhook
			ssGVR := schema.GroupVersionResource{
				Group:    "external-secrets.io",
				Version:  "v1beta1",
				Resource: "secretstores",
			}
			ss := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "external-secrets.io/v1beta1",
					"kind":       "SecretStore",
					"metadata": map[string]interface{}{
						"name":      "e2e-webhook-test",
						"namespace": testNS,
					},
					"spec": map[string]interface{}{
						"provider": map[string]interface{}{
							"fake": map[string]interface{}{
								"data": []interface{}{},
							},
						},
					},
				},
			}

			By("Verifying webhook processes the admission request")
			// The creation may succeed or fail with a validation error,
			// but should NOT fail with a connection/timeout error
			_, err = dynamicClient.Resource(ssGVR).Namespace(testNS).Create(ctx, ss, metav1.CreateOptions{})
			// We don't check for success - only that the webhook responded
			// A timeout or connection refused would indicate network policy issues
			if err != nil {
				Expect(err.Error()).NotTo(ContainSubstring("connection refused"),
					"webhook should be accessible through network policies")
				Expect(err.Error()).NotTo(ContainSubstring("context deadline exceeded"),
					"webhook should respond within timeout")
			}

			// Cleanup
			_ = dynamicClient.Resource(ssGVR).Namespace(testNS).Delete(ctx, "e2e-webhook-test", metav1.DeleteOptions{})
		})
	})
})

