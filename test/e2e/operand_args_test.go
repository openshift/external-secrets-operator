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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	externalsecrets "github.com/openshift/external-secrets-operator/pkg/controller/external_secrets"
	"github.com/openshift/external-secrets-operator/test/utils"
)

var _ = Describe("Operand Args Env Overrides", Ordered, Label("Platform:Generic", "Feature:OverrideOperandArgs"), func() {
	ctx := context.Background()

	const (
		controllerArg     = "--concurrent=2"
		webhookArg        = "--check-interval=10m0s"
		certControllerArg = "--crd-requeue-interval=10m"
		bitwardenArg      = "--key-file=/certs/key.pem"

		// Real webhook flags in the messy parse format: boolean + junk, repeated
		// commas/tab, comma-separated --tls-ciphers value, then another flag.
		messyValidWebhookArgs = "--enable-http2,arg2,,,\t,--tls-ciphers=TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,--loglevel=debug"
		messyTLSCiphersArg    = "--tls-ciphers=TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256"
		// Same value with a leading non-flag token — must Degrade until removed.
		messyInvalidWebhookArgs = "not-a-flag," + messyValidWebhookArgs
	)

	var (
		clientset     *kubernetes.Clientset
		dynamicClient *dynamic.DynamicClient
		runtimeClient client.Client

		originalBitwardenProvider *operatorv1alpha1.BitwardenSecretManagerProvider

		operandArgsEnv = map[string]string{
			externalsecrets.OperandExternalSecretsArgsEnvVar:    controllerArg,
			externalsecrets.OperandWebhookArgsEnvVar:            webhookArg,
			externalsecrets.OperandCertControllerArgsEnvVar:     certControllerArg,
			externalsecrets.OperandBitwardenSDKServerArgsEnvVar: bitwardenArg,
		}
		operandArgsEnvKeys = []string{
			externalsecrets.OperandExternalSecretsArgsEnvVar,
			externalsecrets.OperandWebhookArgsEnvVar,
			externalsecrets.OperandCertControllerArgsEnvVar,
			externalsecrets.OperandBitwardenSDKServerArgsEnvVar,
		}
	)

	waitForOperandArg := func(deploymentName, containerName, arg string, present bool) {
		Eventually(func(g Gomega) {
			deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, deploymentName, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred(), "should get %s deployment", deploymentName)
			hasArg, found := deploymentContainerHasArg(deployment, containerName, arg)
			g.Expect(found).To(BeTrue(), "%s container should exist in %s", containerName, deploymentName)
			if present {
				g.Expect(hasArg).To(BeTrue(), "%s/%s should include %q", deploymentName, containerName, arg)
			} else {
				g.Expect(hasArg).To(BeFalse(), "%s/%s should not include %q", deploymentName, containerName, arg)
			}
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
	}

	BeforeAll(func() {
		clientset = suiteClientset
		dynamicClient = suiteDynamicClient
		runtimeClient = suiteRuntimeClient
		Expect(clientset).NotTo(BeNil())
		Expect(dynamicClient).NotTo(BeNil())
		Expect(runtimeClient).NotTo(BeNil())

		// Snapshot Bitwarden plugin state before ensureBitwardenOperandReady creates/updates ESC.
		esc := &operatorv1alpha1.ExternalSecretsConfig{}
		if err := runtimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc); err != nil {
			Expect(k8serrors.IsNotFound(err)).To(BeTrue(), "unexpected error getting ExternalSecretsConfig: %v", err)
		} else if esc.Spec.Plugins.BitwardenSecretManagerProvider != nil {
			originalBitwardenProvider = esc.Spec.Plugins.BitwardenSecretManagerProvider.DeepCopy()
		}

		By("Ensuring ExternalSecretsConfig exists and bitwarden-sdk-server is ready")
		Expect(ensureBitwardenOperandReady(ctx)).To(Succeed())

		By("Setting OPERAND_*_ARGS on the operator manager")
		Expect(setOperatorManagerEnv(ctx, clientset, dynamicClient, operandArgsEnv)).To(Succeed())

		By("Waiting for operator pod to be ready after env update")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespace, []string{operatorPodPrefix})).To(Succeed())
	})

	AfterAll(func() {
		By("Clearing OPERAND_*_ARGS from the operator manager")
		Expect(unsetOperatorManagerEnv(ctx, clientset, dynamicClient, operandArgsEnvKeys)).To(Succeed())

		By("Waiting for operator pod to be ready after env cleanup")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespace, []string{operatorPodPrefix})).To(Succeed())

		By("Reverting ExternalSecretsConfig Bitwarden plugin to pre-suite state")
		Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			if err := runtimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc); err != nil {
				return err
			}
			esc.Spec.Plugins.BitwardenSecretManagerProvider = originalBitwardenProvider
			return runtimeClient.Update(ctx, esc)
		})).To(Succeed())
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 3*time.Minute)).To(Succeed())
	})

	It("should override args on the core controller Deployment", func() {
		By("Verifying controller args override and default concurrent flag is replaced")
		waitForOperandArg(externalsecrets.OperandCoreControllerDeployment, externalsecrets.OperandCoreControllerContainer, controllerArg, true)
		Eventually(func(g Gomega) {
			deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandCoreControllerDeployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			hasDefault, found := deploymentContainerHasArg(deployment, externalsecrets.OperandCoreControllerContainer, "--concurrent=1")
			g.Expect(found).To(BeTrue())
			g.Expect(hasDefault).To(BeFalse(), "default --concurrent=1 should be replaced")
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should override args on the webhook Deployment", func() {
		waitForOperandArg(externalsecrets.OperandWebhookDeployment, externalsecrets.OperandWebhookContainer, webhookArg, true)
		Eventually(func(g Gomega) {
			deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandWebhookDeployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			args, found := getDeploymentContainerArgs(deployment, externalsecrets.OperandWebhookContainer)
			g.Expect(found).To(BeTrue())
			g.Expect(args).NotTo(BeEmpty())
			g.Expect(args[0]).To(Equal("webhook"), "webhook positional token should be preserved")
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should override args on the cert-controller Deployment", func() {
		waitForOperandArg(externalsecrets.OperandCertControllerDeployment, externalsecrets.OperandCertControllerContainer, certControllerArg, true)
		Eventually(func(g Gomega) {
			deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandCertControllerDeployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			args, found := getDeploymentContainerArgs(deployment, externalsecrets.OperandCertControllerContainer)
			g.Expect(found).To(BeTrue())
			g.Expect(args).NotTo(BeEmpty())
			g.Expect(args[0]).To(Equal("certcontroller"), "certcontroller positional token should be preserved")
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should apply args on the bitwarden-sdk-server Deployment", func() {
		waitForOperandArg(externalsecrets.OperandBitwardenSDKServerDeployment, externalsecrets.OperandBitwardenContainer, bitwardenArg, true)
	})

	It("should mark ExternalSecretsConfig Degraded for invalid OPERAND_*_ARGS", func() {
		By("Setting a messy invalid webhook args override")
		Expect(setOperatorManagerEnv(ctx, clientset, dynamicClient, map[string]string{
			externalsecrets.OperandWebhookArgsEnvVar: messyInvalidWebhookArgs,
		})).To(Succeed())
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespace, []string{operatorPodPrefix})).To(Succeed())

		By("Waiting for ExternalSecretsConfig to become Degraded with a user-configuration message")
		Eventually(func(g Gomega) {
			g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
				"ExternalSecretsConfig should be Degraded for invalid OPERAND_*_ARGS")
			msg := externalSecretsConfigDegradedMessage(ctx)
			g.Expect(msg).To(ContainSubstring("invalid custom arg override"))
			g.Expect(msg).To(ContainSubstring(`argument "not-a-flag" must start with --`))
			g.Expect(msg).To(ContainSubstring(externalsecrets.OperandWebhookArgsEnvVar))
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("Verifying no tokens from the messy value were applied to the webhook Deployment")
		Eventually(func(g Gomega) {
			deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandWebhookDeployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			for _, arg := range []string{"not-a-flag", "--enable-http2", messyTLSCiphersArg, "--loglevel=debug"} {
				hasArg, found := deploymentContainerHasArg(deployment, externalsecrets.OperandWebhookContainer, arg)
				g.Expect(found).To(BeTrue())
				g.Expect(hasArg).To(BeFalse(), "%q must not be present on webhook container", arg)
			}
		}, time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should recover from Degraded when not-a-flag is removed from the messy value", func() {
		By("Ensuring ExternalSecretsConfig is currently Degraded from the messy invalid args")
		Eventually(func(g Gomega) {
			g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue())
		}, time.Minute, 5*time.Second).Should(Succeed())

		By("Correcting OPERAND_WEBHOOK_ARGS by removing only not-a-flag")
		Expect(setOperatorManagerEnv(ctx, clientset, dynamicClient, map[string]string{
			externalsecrets.OperandWebhookArgsEnvVar: messyValidWebhookArgs,
		})).To(Succeed())
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespace, []string{operatorPodPrefix})).To(Succeed())

		By("Waiting for ExternalSecretsConfig to recover from Degraded")
		Eventually(func(g Gomega) {
			g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeFalse(),
				"ExternalSecretsConfig should leave Degraded after not-a-flag is removed")
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

		By("Verifying the messy valid webhook flags were applied after recovery")
		waitForOperandArg(externalsecrets.OperandWebhookDeployment, externalsecrets.OperandWebhookContainer, "--enable-http2", true)
		waitForOperandArg(externalsecrets.OperandWebhookDeployment, externalsecrets.OperandWebhookContainer, messyTLSCiphersArg, true)
		waitForOperandArg(externalsecrets.OperandWebhookDeployment, externalsecrets.OperandWebhookContainer, "--loglevel=debug", true)
		waitForOperandArg(externalsecrets.OperandWebhookDeployment, externalsecrets.OperandWebhookContainer, "arg2", false)
		Eventually(func(g Gomega) {
			deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandWebhookDeployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			args, found := getDeploymentContainerArgs(deployment, externalsecrets.OperandWebhookContainer)
			g.Expect(found).To(BeTrue())
			g.Expect(args).NotTo(BeEmpty())
			g.Expect(args[0]).To(Equal("webhook"), "webhook positional token should be preserved")
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should mark ExternalSecretsConfig Degraded for positional OPERAND_WEBHOOK_ARGS", func() {
		By("Setting a positional webhook args override so webhook invalidation is the only failure")
		Expect(setOperatorManagerEnv(ctx, clientset, dynamicClient, map[string]string{
			externalsecrets.OperandWebhookArgsEnvVar: "webhook,--port=10251",
		})).To(Succeed())
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespace, []string{operatorPodPrefix})).To(Succeed())

		By("Waiting for ExternalSecretsConfig to become Degraded")
		Eventually(func(g Gomega) {
			g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
				"ExternalSecretsConfig should be Degraded for positional webhook args")
			msg := externalSecretsConfigDegradedMessage(ctx)
			g.Expect(msg).To(ContainSubstring("invalid custom arg override"))
			g.Expect(msg).To(ContainSubstring(externalsecrets.OperandWebhookArgsEnvVar))
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should recover from Degraded when invalid OPERAND_*_ARGS are corrected", func() {
		By("Ensuring ExternalSecretsConfig is currently Degraded from prior invalid args")
		Eventually(func(g Gomega) {
			g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue())
		}, time.Minute, 5*time.Second).Should(Succeed())

		By("Correcting OPERAND_*_ARGS back to valid overrides")
		Expect(setOperatorManagerEnv(ctx, clientset, dynamicClient, operandArgsEnv)).To(Succeed())
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespace, []string{operatorPodPrefix})).To(Succeed())

		By("Waiting for ExternalSecretsConfig to recover from Degraded")
		Eventually(func(g Gomega) {
			g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeFalse(),
				"ExternalSecretsConfig should leave Degraded after args are corrected")
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

		By("Verifying valid overrides are applied after recovery")
		waitForOperandArg(externalsecrets.OperandCoreControllerDeployment, externalsecrets.OperandCoreControllerContainer, controllerArg, true)
		waitForOperandArg(externalsecrets.OperandWebhookDeployment, externalsecrets.OperandWebhookContainer, webhookArg, true)
	})

	It("should restore default operand args when OPERAND_*_ARGS are cleared", func() {
		By("Clearing OPERAND_*_ARGS to verify restoration")
		Expect(unsetOperatorManagerEnv(ctx, clientset, dynamicClient, operandArgsEnvKeys)).To(Succeed())
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespace, []string{operatorPodPrefix})).To(Succeed())

		waitForOperandArg(externalsecrets.OperandCoreControllerDeployment, externalsecrets.OperandCoreControllerContainer, controllerArg, false)
		Eventually(func(g Gomega) {
			deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandCoreControllerDeployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			hasDefault, found := deploymentContainerHasArg(deployment, externalsecrets.OperandCoreControllerContainer, "--concurrent=1")
			g.Expect(found).To(BeTrue())
			g.Expect(hasDefault).To(BeTrue(), "default --concurrent=1 should be restored")
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		waitForOperandArg(externalsecrets.OperandWebhookDeployment, externalsecrets.OperandWebhookContainer, webhookArg, false)
		waitForOperandArg(externalsecrets.OperandCertControllerDeployment, externalsecrets.OperandCertControllerContainer, certControllerArg, false)
		waitForOperandArg(externalsecrets.OperandBitwardenSDKServerDeployment, externalsecrets.OperandBitwardenContainer, bitwardenArg, false)
	})
})
