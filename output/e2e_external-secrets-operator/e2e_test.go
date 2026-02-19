//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	escGVR = schema.GroupVersionResource{
		Group:    "operator.openshift.io",
		Version:  "v1alpha1",
		Resource: "externalsecretsconfigs",
	}
	networkPolicyGVR = schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "networkpolicies",
	}
)

// Diff-suggested: NetworkPolicy API changes from EP #1834
var _ = Describe("NetworkPolicy E2E Tests for ExternalSecretsConfig", Ordered, func() {
	const (
		operandNamespace = "external-secrets"
		escName          = "cluster"
		timeout          = 60 * time.Second
		interval         = 2 * time.Second
	)

	// getESC retrieves the ExternalSecretsConfig cluster singleton
	getESC := func(ctx context.Context) (*unstructured.Unstructured, error) {
		return dynamicClient.Resource(escGVR).Get(ctx, escName, metav1.GetOptions{})
	}

	// getNetworkPolicy retrieves a NetworkPolicy from the operand namespace
	getNetworkPolicy := func(ctx context.Context, name string) (*unstructured.Unstructured, error) {
		return dynamicClient.Resource(networkPolicyGVR).Namespace(operandNamespace).Get(ctx, name, metav1.GetOptions{})
	}

	// patchESCNetworkPolicies patches the ExternalSecretsConfig with network policies
	patchESCNetworkPolicies := func(ctx context.Context, networkPolicies []interface{}) error {
		esc, err := getESC(ctx)
		if err != nil {
			return err
		}

		spec, _, _ := unstructured.NestedMap(esc.Object, "spec")
		if spec == nil {
			spec = map[string]interface{}{}
		}

		controllerConfig, _, _ := unstructured.NestedMap(spec, "controllerConfig")
		if controllerConfig == nil {
			controllerConfig = map[string]interface{}{}
		}

		controllerConfig["networkPolicies"] = networkPolicies
		spec["controllerConfig"] = controllerConfig
		esc.Object["spec"] = spec

		_, err = dynamicClient.Resource(escGVR).Update(ctx, esc, metav1.UpdateOptions{})
		return err
	}

	// waitForReady waits for the ESC to reach Ready condition
	waitForReady := func(ctx context.Context) {
		Eventually(func() bool {
			esc, err := getESC(ctx)
			if err != nil {
				return false
			}
			conditions, _, _ := unstructured.NestedSlice(esc.Object, "status", "conditions")
			for _, c := range conditions {
				cond, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				if cond["type"] == "Ready" && cond["status"] == "True" {
					return true
				}
			}
			return false
		}, timeout, interval).Should(BeTrue(), "ExternalSecretsConfig should become Ready")
	}

	BeforeAll(func() {
		By("Verifying ExternalSecretsConfig exists and is Ready")
		ctx := context.Background()
		waitForReady(ctx)
	})

	AfterAll(func() {
		By("Cleaning up custom network policies")
		ctx := context.Background()
		_ = patchESCNetworkPolicies(ctx, []interface{}{})
		time.Sleep(5 * time.Second)
	})

	// Diff-suggested: Static NetworkPolicy verification (baseline)
	Context("Static NetworkPolicies", func() {
		It("should have deny-all network policy in operand namespace", func() {
			By("Checking for deny-all-traffic NetworkPolicy")
			ctx := context.Background()
			np, err := getNetworkPolicy(ctx, "deny-all-traffic")
			Expect(err).NotTo(HaveOccurred(), "deny-all-traffic NetworkPolicy should exist")

			By("Verifying deny-all selects all pods")
			podSelector, _, _ := unstructured.NestedMap(np.Object, "spec", "podSelector")
			Expect(podSelector).To(BeEmpty(), "deny-all should have empty podSelector to match all pods")
		})

		It("should have DNS allow policy in operand namespace", func() {
			By("Checking for allow-to-dns NetworkPolicy")
			ctx := context.Background()
			np, err := getNetworkPolicy(ctx, "allow-to-dns")
			Expect(err).NotTo(HaveOccurred(), "allow-to-dns NetworkPolicy should exist")

			By("Verifying DNS policy has egress rules")
			egress, _, _ := unstructured.NestedSlice(np.Object, "spec", "egress")
			Expect(egress).NotTo(BeEmpty(), "DNS policy should have egress rules")
		})

		It("should have main controller allow policy", func() {
			By("Checking for main controller NetworkPolicy")
			ctx := context.Background()
			_, err := getNetworkPolicy(ctx, "allow-api-server-egress-for-main-controller")
			Expect(err).NotTo(HaveOccurred(), "main controller NetworkPolicy should exist")
		})

		It("should have webhook allow policy", func() {
			By("Checking for webhook NetworkPolicy")
			ctx := context.Background()
			_, err := getNetworkPolicy(ctx, "allow-api-server-and-webhook-traffic")
			Expect(err).NotTo(HaveOccurred(), "webhook NetworkPolicy should exist")
		})
	})

	// Diff-suggested: Custom NetworkPolicy lifecycle (new API field from EP #1834)
	Context("Custom NetworkPolicies", func() {
		It("should create a custom NetworkPolicy for ExternalSecretsCoreController", func() {
			ctx := context.Background()

			By("Patching ESC with a custom network policy")
			networkPolicies := []interface{}{
				map[string]interface{}{
					"name":          "allow-core-egress",
					"componentName": "ExternalSecretsCoreController",
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
				},
			}
			err := patchESCNetworkPolicies(ctx, networkPolicies)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for reconciliation")
			waitForReady(ctx)

			By("Verifying custom NetworkPolicy was created")
			Eventually(func() error {
				_, err := getNetworkPolicy(ctx, "allow-core-egress")
				return err
			}, timeout, interval).Should(Succeed(), "Custom NetworkPolicy should be created")

			By("Verifying podSelector targets external-secrets pods")
			np, err := getNetworkPolicy(ctx, "allow-core-egress")
			Expect(err).NotTo(HaveOccurred())
			matchLabels, _, _ := unstructured.NestedStringMap(np.Object, "spec", "podSelector", "matchLabels")
			Expect(matchLabels).To(HaveKeyWithValue("app.kubernetes.io/name", "external-secrets"))

			By("Verifying egress rules")
			egress, _, _ := unstructured.NestedSlice(np.Object, "spec", "egress")
			Expect(egress).To(HaveLen(1))
		})

		It("should create a custom NetworkPolicy with empty egress for deny-all", func() {
			ctx := context.Background()

			By("Patching ESC with empty egress for deny-all")
			networkPolicies := []interface{}{
				map[string]interface{}{
					"name":          "deny-core-egress",
					"componentName": "ExternalSecretsCoreController",
					"egress":        []interface{}{},
				},
			}
			err := patchESCNetworkPolicies(ctx, networkPolicies)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for reconciliation")
			waitForReady(ctx)

			By("Verifying deny-all custom NetworkPolicy was created")
			Eventually(func() error {
				_, err := getNetworkPolicy(ctx, "deny-core-egress")
				return err
			}, timeout, interval).Should(Succeed())

			By("Verifying empty egress rules")
			np, err := getNetworkPolicy(ctx, "deny-core-egress")
			Expect(err).NotTo(HaveOccurred())
			egress, found, _ := unstructured.NestedSlice(np.Object, "spec", "egress")
			if found {
				Expect(egress).To(BeEmpty(), "Egress should be empty for deny-all")
			}

			By("Verifying policyTypes includes Egress")
			policyTypes, _, _ := unstructured.NestedStringSlice(np.Object, "spec", "policyTypes")
			Expect(policyTypes).To(ContainElement(string(networkingv1.PolicyTypeEgress)))
		})

		It("should clean up stale custom NetworkPolicies when removed from spec", func() {
			ctx := context.Background()

			By("Creating a custom network policy first")
			networkPolicies := []interface{}{
				map[string]interface{}{
					"name":          "temp-policy",
					"componentName": "ExternalSecretsCoreController",
					"egress": []interface{}{
						map[string]interface{}{
							"ports": []interface{}{
								map[string]interface{}{"protocol": "TCP", "port": int64(6443)},
							},
						},
					},
				},
			}
			err := patchESCNetworkPolicies(ctx, networkPolicies)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for NetworkPolicy to be created")
			Eventually(func() error {
				_, err := getNetworkPolicy(ctx, "temp-policy")
				return err
			}, timeout, interval).Should(Succeed())

			By("Removing the network policy from spec")
			err = patchESCNetworkPolicies(ctx, []interface{}{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the stale NetworkPolicy is deleted")
			Eventually(func() bool {
				_, err := getNetworkPolicy(ctx, "temp-policy")
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue(), "Stale NetworkPolicy should be deleted")
		})
	})

	// Diff-suggested: DNS name pattern validation (new pattern validation from EP #1834)
	Context("NetworkPolicy Name Validation", func() {
		It("should reject names with uppercase characters", func() {
			ctx := context.Background()

			By("Attempting to create policy with uppercase name")
			networkPolicies := []interface{}{
				map[string]interface{}{
					"name":          "Allow-Egress",
					"componentName": "ExternalSecretsCoreController",
					"egress":        []interface{}{},
				},
			}
			err := patchESCNetworkPolicies(ctx, networkPolicies)
			Expect(err).To(HaveOccurred(), "Uppercase name should be rejected")
			Expect(err.Error()).To(ContainSubstring("Invalid value"))
		})

		It("should reject names with underscores", func() {
			ctx := context.Background()

			By("Attempting to create policy with underscore name")
			networkPolicies := []interface{}{
				map[string]interface{}{
					"name":          "allow_egress",
					"componentName": "ExternalSecretsCoreController",
					"egress":        []interface{}{},
				},
			}
			err := patchESCNetworkPolicies(ctx, networkPolicies)
			Expect(err).To(HaveOccurred(), "Underscore name should be rejected")
		})

		It("should reject names starting with hyphen", func() {
			ctx := context.Background()

			By("Attempting to create policy with leading hyphen")
			networkPolicies := []interface{}{
				map[string]interface{}{
					"name":          "-allow-egress",
					"componentName": "ExternalSecretsCoreController",
					"egress":        []interface{}{},
				},
			}
			err := patchESCNetworkPolicies(ctx, networkPolicies)
			Expect(err).To(HaveOccurred(), "Leading hyphen name should be rejected")
		})

		It("should reject invalid componentName", func() {
			ctx := context.Background()

			By("Attempting to create policy with invalid componentName")
			networkPolicies := []interface{}{
				map[string]interface{}{
					"name":          "test-policy",
					"componentName": "InvalidComponent",
					"egress":        []interface{}{},
				},
			}
			err := patchESCNetworkPolicies(ctx, networkPolicies)
			Expect(err).To(HaveOccurred(), "Invalid componentName should be rejected")
			Expect(err.Error()).To(ContainSubstring("Unsupported value"))
		})

		It("should accept valid DNS subdomain names", func() {
			ctx := context.Background()

			By("Creating policy with valid DNS subdomain name")
			networkPolicies := []interface{}{
				map[string]interface{}{
					"name":          "allow-core-controller-egress-to-api",
					"componentName": "ExternalSecretsCoreController",
					"egress": []interface{}{
						map[string]interface{}{
							"ports": []interface{}{
								map[string]interface{}{"protocol": "TCP", "port": int64(6443)},
							},
						},
					},
				},
			}
			err := patchESCNetworkPolicies(ctx, networkPolicies)
			Expect(err).NotTo(HaveOccurred(), "Valid DNS subdomain name should be accepted")

			waitForReady(ctx)

			By("Cleaning up")
			_ = patchESCNetworkPolicies(ctx, []interface{}{})
		})
	})
})
