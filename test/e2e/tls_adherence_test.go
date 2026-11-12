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
	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/test/utils"
)

var _ = Describe("Centralized TLS Adherence", Ordered, Label("Platform:Generic", "TechPreview", "Feature:TLSAdherence"), func() {
	ctx := context.Background()

	var (
		originalAPIServer         *configv1.APIServer
		originalBitwardenProvider *operatorv1alpha1.BitwardenSecretManagerProvider
		bitwardenCaptured         bool
	)

	BeforeAll(func() {
		Expect(suiteClientset).NotTo(BeNil())
		Expect(suiteDynamicClient).NotTo(BeNil())
		Expect(suiteRuntimeClient).NotTo(BeNil())
		requireOpenSSLAndOC()

		By("Capturing original APIServer TLS configuration")
		apiServer, err := getClusterAPIServer(ctx)
		Expect(err).NotTo(HaveOccurred(), "get apiserver.config.openshift.io/cluster (requires OpenShift + TLSAdherence feature gate / TechPreview)")
		originalAPIServer = apiServer.DeepCopy()
		// Restore even if AfterAll Bitwarden/ESC cleanup fails.
		toRestore := originalAPIServer
		DeferCleanup(func() {
			if toRestore == nil {
				return
			}
			By("Restoring original APIServer TLS configuration")
			Expect(restoreAPIServerTLS(ctx, toRestore)).To(Succeed())
			By("Waiting for operator and operands to be healthy after restore")
			waitOperatorAndOperandsHealthy(ctx)
		})

		By("Ensuring ExternalSecretsConfig and bitwarden-sdk-server are ready")
		Expect(ensureExternalSecretsConfigReady(ctx)).To(Succeed())

		esc := &operatorv1alpha1.ExternalSecretsConfig{}
		Expect(suiteRuntimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc)).To(Succeed())
		if esc.Spec.Plugins.BitwardenSecretManagerProvider != nil {
			originalBitwardenProvider = esc.Spec.Plugins.BitwardenSecretManagerProvider.DeepCopy()
		}
		bitwardenCaptured = true

		Expect(ensureBitwardenOperandReady(ctx, nil)).To(Succeed())
		waitOperatorAndOperandsHealthy(ctx)
	})

	AfterAll(func() {
		if !bitwardenCaptured {
			return
		}
		By("Reverting ExternalSecretsConfig Bitwarden plugin to pre-suite state")
		Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			if err := suiteRuntimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc); err != nil {
				return err
			}
			esc.Spec.Plugins.BitwardenSecretManagerProvider = originalBitwardenProvider
			return suiteRuntimeClient.Update(ctx, esc)
		})).To(Succeed())
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, suiteDynamicClient, common.ExternalSecretsConfigObjectName, 3*time.Minute)).To(Succeed())
	})

	It("does not propagate TLS args when tlsAdherence does not require honoring the profile", func() {
		By("Setting tlsAdherence=LegacyAdheringComponentsOnly")
		Expect(updateAPIServerTLS(ctx, configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly, originalAPIServer.Spec.TLSSecurityProfile)).To(Succeed())
		waitOperatorAndOperandsHealthy(ctx)

		By("Asserting operand Deployments have no --tls-min-version/--tls-ciphers args")
		assertOperandTLSArgsAbsent(ctx)
	})

	It("applies Intermediate defaults when StrictAllComponents is set with an empty TLS profile", func() {
		By("Setting tlsAdherence=StrictAllComponents with empty tlsSecurityProfile")
		Expect(updateAPIServerTLS(ctx, configv1.TLSAdherencePolicyStrictAllComponents, nil)).To(Succeed())
		waitOperatorAndOperandsHealthy(ctx)

		By("Asserting Intermediate operand TLS args")
		assertOperandTLSArgsMatch(ctx, "1.2", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", true)

		By("Probing TLS endpoints with openssl (Intermediate expectations)")
		assertOpenSSLProbes(ctx, intermediateOpenSSLExpectation)
	})

	It("applies an explicit Intermediate TLS profile under StrictAllComponents", func() {
		By("Setting tlsSecurityProfile.type=Intermediate")
		Expect(updateAPIServerTLS(ctx, configv1.TLSAdherencePolicyStrictAllComponents, intermediateTLSProfile())).To(Succeed())
		waitOperatorAndOperandsHealthy(ctx)

		By("Asserting Intermediate operand TLS args")
		assertOperandTLSArgsMatch(ctx, "1.2", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", true)

		By("Probing TLS endpoints with openssl (Intermediate expectations)")
		assertOpenSSLProbes(ctx, intermediateOpenSSLExpectation)
	})

	It("applies a Modern TLS profile under StrictAllComponents", func() {
		By("Setting tlsSecurityProfile.type=Modern")
		Expect(updateAPIServerTLS(ctx, configv1.TLSAdherencePolicyStrictAllComponents, modernTLSProfile())).To(Succeed())
		waitOperatorAndOperandsHealthy(ctx)

		By("Asserting Modern operand TLS args (min 1.3, no --tls-ciphers)")
		assertOperandTLSArgsMatch(ctx, "1.3", "", false)

		By("Probing TLS endpoints with openssl (TLS 1.3 only)")
		assertOpenSSLProbes(ctx, tls13OnlyOpenSSLExpectation)
	})

	It("applies a Custom TLS 1.3 profile under StrictAllComponents", func() {
		By("Setting tlsSecurityProfile.type=Custom (minTLSVersion VersionTLS13)")
		Expect(updateAPIServerTLS(ctx, configv1.TLSAdherencePolicyStrictAllComponents, customTLS13Profile())).To(Succeed())
		waitOperatorAndOperandsHealthy(ctx)

		By("Asserting Custom TLS 1.3 operand args (min 1.3, no --tls-ciphers)")
		assertOperandTLSArgsMatch(ctx, "1.3", "", false)

		By("Probing TLS endpoints with openssl (TLS 1.3 only)")
		assertOpenSSLProbes(ctx, tls13OnlyOpenSSLExpectation)
	})
})
