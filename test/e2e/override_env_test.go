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

// Tests for ESO-334 / PR-106: OverrideEnv per-component environment variable injection.
//
// Covered scenarios (all absent from the existing e2e suite):
//   TC-001  Container isolation   – env vars set for CoreController must not leak into
//                                   the webhook or cert-controller containers.
//   TC-002  Incremental add       – adding a second env var must preserve the first.
//   TC-003  Partial remove        – removing one env var of several must leave the rest.
//   TC-004  Value update          – changing a value must be reflected in the deployment.
//   TC-005  Reserved prefix       – KUBERNETES_*, HOSTNAME, EXTERNAL_SECRETS_* must be
//                                   rejected by the CRD CEL validation rule.
//   TC-006  Single component scope– updating one component must not roll out sibling
//                                   deployments whose spec has not changed.

package e2e

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/test/utils"
)

var _ = Describe("OverrideEnv: per-component environment variable injection", Ordered, func() {
	var (
		ctx           context.Context
		clientset     *kubernetes.Clientset
		dynamicClient *dynamic.DynamicClient
		runtimeClient client.Client
	)

	BeforeAll(func() {
		ctx = context.Background()
		clientset = suiteClientset
		dynamicClient = suiteDynamicClient
		runtimeClient = suiteRuntimeClient

		By("Waiting for operator pod to be ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespace, []string{
			operatorPodPrefix,
		})).To(Succeed())

		By("Waiting for ExternalSecretsConfig to reach Ready state")
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, "cluster", 2*time.Minute)).To(Succeed())
	})

	BeforeEach(func() {
		By("Verifying all operand pods are Ready before the spec")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
			operandCoreControllerPodPrefix,
			operandCertControllerPodPrefix,
			operandWebhookPodPrefix,
		})).To(Succeed())
	})

	// ── TC-001 ─────────────────────────────────────────────────────────────────
	// Verify that `applyUserDeploymentConfigs` + `getComponentNameFromAsset` target
	// only the primary container of the named component and do not inject env vars
	// into sibling deployments.
	Context("TC-001: container isolation", func() {
		It("should inject env vars only into the targeted component's container, not into sibling containers",
			Label("reconciliation", "controller-manager"), func() {

				isolationVar := corev1.EnvVar{
					Name:  fmt.Sprintf("ESO_ISOLATION_TEST_%s", utils.GetRandomString(5)),
					Value: "isolation-value",
				}

				By("Setting OverrideEnv only for ExternalSecretsCoreController")
				Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
					esc := &operatorv1alpha1.ExternalSecretsConfig{}
					if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
						return err
					}
					esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
						{
							ComponentName: operatorv1alpha1.CoreController,
							OverrideEnv:   []corev1.EnvVar{isolationVar},
						},
					}
					return runtimeClient.Update(ctx, esc)
				})).To(Succeed())

				DeferCleanup(func() {
					_ = retry.RetryOnConflict(retry.DefaultRetry, func() error {
						esc := &operatorv1alpha1.ExternalSecretsConfig{}
						if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
							return err
						}
						esc.Spec.ControllerConfig.ComponentConfigs = nil
						return runtimeClient.Update(ctx, esc)
					})
				})

				By("Waiting for core controller pod to roll out")
				Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
					operandCoreControllerPodPrefix,
				})).To(Succeed())

				By("Verifying the env var IS present in the external-secrets container")
				Eventually(func(g Gomega) {
					d, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					c := overrideEnvFindContainer(d.Spec.Template.Spec.Containers, "external-secrets")
					g.Expect(c).NotTo(BeNil(), "external-secrets container not found in deployment")
					g.Expect(overrideEnvHasEnvVar(c.Env, isolationVar.Name, isolationVar.Value)).To(BeTrue(),
						"env var %s=%s should be present in external-secrets container", isolationVar.Name, isolationVar.Value)
				}, 2*time.Minute, 5*time.Second).Should(Succeed())

				By("Verifying the env var is NOT present in the webhook container")
				Eventually(func(g Gomega) {
					d, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					c := overrideEnvFindContainer(d.Spec.Template.Spec.Containers, "webhook")
					g.Expect(c).NotTo(BeNil(), "webhook container not found in deployment")
					g.Expect(overrideEnvHasEnvVarName(c.Env, isolationVar.Name)).To(BeFalse(),
						"env var %s must NOT leak into the webhook container", isolationVar.Name)
				}, time.Minute, 5*time.Second).Should(Succeed())

				By("Verifying the env var is NOT present in the cert-controller container")
				Eventually(func(g Gomega) {
					d, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-cert-controller", metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					c := overrideEnvFindContainer(d.Spec.Template.Spec.Containers, "cert-controller")
					g.Expect(c).NotTo(BeNil(), "cert-controller container not found in deployment")
					g.Expect(overrideEnvHasEnvVarName(c.Env, isolationVar.Name)).To(BeFalse(),
						"env var %s must NOT leak into the cert-controller container", isolationVar.Name)
				}, time.Minute, 5*time.Second).Should(Succeed())
			})
	})

	// ── TC-002 ─────────────────────────────────────────────────────────────────
	// Adding a second OverrideEnv entry to a component must not evict the first.
	Context("TC-002: incremental env var addition", func() {
		It("should preserve existing OverrideEnv entries when a new one is added to the same component",
			Label("reconciliation", "controller-manager"), func() {

				suffix := utils.GetRandomString(5)
				varA := corev1.EnvVar{Name: fmt.Sprintf("ESO_INC_A_%s", suffix), Value: "value-a"}
				varB := corev1.EnvVar{Name: fmt.Sprintf("ESO_INC_B_%s", suffix), Value: "value-b"}

				DeferCleanup(func() {
					_ = retry.RetryOnConflict(retry.DefaultRetry, func() error {
						esc := &operatorv1alpha1.ExternalSecretsConfig{}
						if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
							return err
						}
						esc.Spec.ControllerConfig.ComponentConfigs = nil
						return runtimeClient.Update(ctx, esc)
					})
				})

				By("Setting one env var for ExternalSecretsCoreController")
				Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
					esc := &operatorv1alpha1.ExternalSecretsConfig{}
					if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
						return err
					}
					esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
						{ComponentName: operatorv1alpha1.CoreController, OverrideEnv: []corev1.EnvVar{varA}},
					}
					return runtimeClient.Update(ctx, esc)
				})).To(Succeed())

				Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
					operandCoreControllerPodPrefix,
				})).To(Succeed())

				Eventually(func(g Gomega) {
					d, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					c := overrideEnvFindContainer(d.Spec.Template.Spec.Containers, "external-secrets")
					g.Expect(overrideEnvHasEnvVar(c.Env, varA.Name, varA.Value)).To(BeTrue(),
						"varA should be present after the first update")
				}, 2*time.Minute, 5*time.Second).Should(Succeed())

				By("Adding a second env var while keeping the first")
				Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
					esc := &operatorv1alpha1.ExternalSecretsConfig{}
					if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
						return err
					}
					esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
						{ComponentName: operatorv1alpha1.CoreController, OverrideEnv: []corev1.EnvVar{varA, varB}},
					}
					return runtimeClient.Update(ctx, esc)
				})).To(Succeed())

				Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
					operandCoreControllerPodPrefix,
				})).To(Succeed())

				By("Verifying both env vars are present after the incremental update")
				Eventually(func(g Gomega) {
					d, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					c := overrideEnvFindContainer(d.Spec.Template.Spec.Containers, "external-secrets")
					g.Expect(overrideEnvHasEnvVar(c.Env, varA.Name, varA.Value)).To(BeTrue(),
						"varA should still be present after adding varB")
					g.Expect(overrideEnvHasEnvVar(c.Env, varB.Name, varB.Value)).To(BeTrue(),
						"varB should be present after the incremental add")
				}, 2*time.Minute, 5*time.Second).Should(Succeed())
			})
	})

	// ── TC-003 ─────────────────────────────────────────────────────────────────
	// Removing one OverrideEnv entry of several must leave the remaining ones intact.
	Context("TC-003: partial env var removal", func() {
		It("should remove only the deleted env var entry and preserve the remaining ones",
			Label("reconciliation", "controller-manager"), func() {

				suffix := utils.GetRandomString(5)
				varX := corev1.EnvVar{Name: fmt.Sprintf("ESO_REM_X_%s", suffix), Value: "value-x"}
				varY := corev1.EnvVar{Name: fmt.Sprintf("ESO_REM_Y_%s", suffix), Value: "value-y"}

				DeferCleanup(func() {
					_ = retry.RetryOnConflict(retry.DefaultRetry, func() error {
						esc := &operatorv1alpha1.ExternalSecretsConfig{}
						if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
							return err
						}
						esc.Spec.ControllerConfig.ComponentConfigs = nil
						return runtimeClient.Update(ctx, esc)
					})
				})

				By("Setting two env vars for ExternalSecretsCoreController")
				Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
					esc := &operatorv1alpha1.ExternalSecretsConfig{}
					if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
						return err
					}
					esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
						{ComponentName: operatorv1alpha1.CoreController, OverrideEnv: []corev1.EnvVar{varX, varY}},
					}
					return runtimeClient.Update(ctx, esc)
				})).To(Succeed())

				Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
					operandCoreControllerPodPrefix,
				})).To(Succeed())

				Eventually(func(g Gomega) {
					d, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					c := overrideEnvFindContainer(d.Spec.Template.Spec.Containers, "external-secrets")
					g.Expect(overrideEnvHasEnvVar(c.Env, varX.Name, varX.Value)).To(BeTrue())
					g.Expect(overrideEnvHasEnvVar(c.Env, varY.Name, varY.Value)).To(BeTrue())
				}, 2*time.Minute, 5*time.Second).Should(Succeed())

				By("Removing varX while keeping varY in the ComponentConfig")
				Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
					esc := &operatorv1alpha1.ExternalSecretsConfig{}
					if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
						return err
					}
					esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
						{ComponentName: operatorv1alpha1.CoreController, OverrideEnv: []corev1.EnvVar{varY}},
					}
					return runtimeClient.Update(ctx, esc)
				})).To(Succeed())

				Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
					operandCoreControllerPodPrefix,
				})).To(Succeed())

				By("Verifying varY is still present and varX has been removed")
				Eventually(func(g Gomega) {
					d, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					c := overrideEnvFindContainer(d.Spec.Template.Spec.Containers, "external-secrets")
					g.Expect(overrideEnvHasEnvVar(c.Env, varY.Name, varY.Value)).To(BeTrue(),
						"varY should remain after partial removal")
					g.Expect(overrideEnvHasEnvVarName(c.Env, varX.Name)).To(BeFalse(),
						"varX should be gone after being removed from OverrideEnv")
				}, 2*time.Minute, 5*time.Second).Should(Succeed())
			})
	})

	// ── TC-004 ─────────────────────────────────────────────────────────────────
	// Changing the value of an existing OverrideEnv entry must propagate to the
	// deployment; the stale value must be replaced.
	Context("TC-004: env var value update propagation", func() {
		It("should reflect the new env var value in the deployment after an OverrideEnv value change",
			Label("reconciliation", "controller-manager"), func() {

				varName := fmt.Sprintf("ESO_UPDATE_TEST_%s", utils.GetRandomString(5))
				initial := corev1.EnvVar{Name: varName, Value: "initial-value"}
				updated := corev1.EnvVar{Name: varName, Value: "updated-value"}

				DeferCleanup(func() {
					_ = retry.RetryOnConflict(retry.DefaultRetry, func() error {
						esc := &operatorv1alpha1.ExternalSecretsConfig{}
						if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
							return err
						}
						esc.Spec.ControllerConfig.ComponentConfigs = nil
						return runtimeClient.Update(ctx, esc)
					})
				})

				By("Setting env var with the initial value on the Webhook component")
				Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
					esc := &operatorv1alpha1.ExternalSecretsConfig{}
					if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
						return err
					}
					esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
						{ComponentName: operatorv1alpha1.Webhook, OverrideEnv: []corev1.EnvVar{initial}},
					}
					return runtimeClient.Update(ctx, esc)
				})).To(Succeed())

				Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
					operandWebhookPodPrefix,
				})).To(Succeed())

				Eventually(func(g Gomega) {
					d, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					c := overrideEnvFindContainer(d.Spec.Template.Spec.Containers, "webhook")
					g.Expect(overrideEnvHasEnvVar(c.Env, varName, initial.Value)).To(BeTrue(),
						"initial value should be set in webhook container")
				}, 2*time.Minute, 5*time.Second).Should(Succeed())

				By("Updating the env var to a new value")
				Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
					esc := &operatorv1alpha1.ExternalSecretsConfig{}
					if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
						return err
					}
					esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
						{ComponentName: operatorv1alpha1.Webhook, OverrideEnv: []corev1.EnvVar{updated}},
					}
					return runtimeClient.Update(ctx, esc)
				})).To(Succeed())

				Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
					operandWebhookPodPrefix,
				})).To(Succeed())

				By("Verifying the updated value is reflected and the initial value is gone")
				Eventually(func(g Gomega) {
					d, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					c := overrideEnvFindContainer(d.Spec.Template.Spec.Containers, "webhook")
					g.Expect(overrideEnvHasEnvVar(c.Env, varName, updated.Value)).To(BeTrue(),
						"updated value should be reflected in webhook container")
					g.Expect(overrideEnvHasEnvVar(c.Env, varName, initial.Value)).To(BeFalse(),
						"initial value should no longer be present after the update")
				}, 2*time.Minute, 5*time.Second).Should(Succeed())
			})
	})

	// ── TC-005 ─────────────────────────────────────────────────────────────────
	// The CRD carries a CEL rule that rejects env var names starting with
	// `HOSTNAME`, `KUBERNETES_`, or `EXTERNAL_SECRETS_`.  Verify the gate at the
	// API level so the restriction is not accidentally removed.
	Context("TC-005: reserved env var prefix validation", func() {
		It("should reject OverrideEnv entries whose names start with reserved prefixes",
			Label("negative-input-validation"), func() {

				reservedCases := []struct {
					desc string
					env  corev1.EnvVar
				}{
					{"KUBERNETES_ prefix", corev1.EnvVar{Name: "KUBERNETES_SERVICE_HOST", Value: "10.0.0.1"}},
					{"HOSTNAME prefix", corev1.EnvVar{Name: "HOSTNAME", Value: "test-host"}},
					{"EXTERNAL_SECRETS_ prefix", corev1.EnvVar{Name: "EXTERNAL_SECRETS_CUSTOM", Value: "val"}},
				}

				for _, tc := range reservedCases {
					By(fmt.Sprintf("Attempting to set env var with %s", tc.desc))

					// Capture loop variable for use in the closure.
					reservedEnv := tc.env

					err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
						esc := &operatorv1alpha1.ExternalSecretsConfig{}
						if getErr := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); getErr != nil {
							return getErr
						}
						esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
							{
								ComponentName: operatorv1alpha1.CoreController,
								OverrideEnv:   []corev1.EnvVar{reservedEnv},
							},
						}
						return runtimeClient.Update(ctx, esc)
					})

					Expect(err).To(HaveOccurred(),
						"API should reject env var %q with %s", reservedEnv.Name, tc.desc)
					Expect(k8serrors.IsInvalid(err)).To(BeTrue(),
						"expected HTTP 422 Invalid for %q, got: %v", reservedEnv.Name, err)
					Expect(err.Error()).To(ContainSubstring("reserved"),
						"error message should mention reserved prefixes for %q", reservedEnv.Name)
				}
			})
	})

	// ── TC-006 ─────────────────────────────────────────────────────────────────
	// Updating OverrideEnv for a single component must only mutate that component's
	// deployment.  Sibling deployments whose spec has not changed must keep the same
	// Generation so that no unnecessary pod rollouts are triggered.
	Context("TC-006: single component update scope", func() {
		It("should not increase the Generation of unaffected component deployments",
			Label("reconciliation", "controller-manager"), func() {

				By("Recording the current Generation of webhook and cert-controller deployments")
				webhookDeploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				certDeploy, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-cert-controller", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				webhookGen := webhookDeploy.Generation
				certGen := certDeploy.Generation

				envVar := corev1.EnvVar{
					Name:  fmt.Sprintf("ESO_SCOPE_TEST_%s", utils.GetRandomString(5)),
					Value: "scoped-value",
				}

				DeferCleanup(func() {
					_ = retry.RetryOnConflict(retry.DefaultRetry, func() error {
						esc := &operatorv1alpha1.ExternalSecretsConfig{}
						if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
							return err
						}
						esc.Spec.ControllerConfig.ComponentConfigs = nil
						return runtimeClient.Update(ctx, esc)
					})
				})

				By("Setting OverrideEnv only for ExternalSecretsCoreController")
				Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
					esc := &operatorv1alpha1.ExternalSecretsConfig{}
					if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
						return err
					}
					esc.Spec.ControllerConfig.ComponentConfigs = []operatorv1alpha1.ComponentConfig{
						{ComponentName: operatorv1alpha1.CoreController, OverrideEnv: []corev1.EnvVar{envVar}},
					}
					return runtimeClient.Update(ctx, esc)
				})).To(Succeed())

				By("Waiting for core controller to roll out")
				Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
					operandCoreControllerPodPrefix,
				})).To(Succeed())

				By("Verifying webhook deployment Generation has not changed")
				Eventually(func(g Gomega) {
					d, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(d.Generation).To(Equal(webhookGen),
						"webhook deployment must not be updated when only CoreController OverrideEnv changed")
				}, time.Minute, 5*time.Second).Should(Succeed())

				By("Verifying cert-controller deployment Generation has not changed")
				Eventually(func(g Gomega) {
					d, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-cert-controller", metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(d.Generation).To(Equal(certGen),
						"cert-controller deployment must not be updated when only CoreController OverrideEnv changed")
				}, time.Minute, 5*time.Second).Should(Succeed())
			})
	})
})

// ── package-private helpers ────────────────────────────────────────────────────

// overrideEnvFindContainer returns a pointer to the named container or nil.
func overrideEnvFindContainer(containers []corev1.Container, name string) *corev1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}
	return nil
}

// overrideEnvHasEnvVar reports whether name=value is present in the env list.
func overrideEnvHasEnvVar(envVars []corev1.EnvVar, name, value string) bool {
	for _, e := range envVars {
		if e.Name == name {
			return e.Value == value
		}
	}
	return false
}

// overrideEnvHasEnvVarName reports whether an env var with the given name exists,
// regardless of its value.
func overrideEnvHasEnvVarName(envVars []corev1.EnvVar, name string) bool {
	for _, e := range envVars {
		if e.Name == name {
			return true
		}
	}
	return false
}
