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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	externalsecrets "github.com/openshift/external-secrets-operator/pkg/controller/external_secrets"
	"github.com/openshift/external-secrets-operator/test/utils"
)

const (
	trustedCABundleTestCMName = "e2e-test-user-ca-bundle"
)

// testCACertPEM returns a minimal valid PEM CA certificate for e2e ConfigMaps.
func testCACertPEM() string {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "e2e-test-ca",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:      true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// testLeafCertPEM returns a valid PEM end-entity certificate (not a CA) for negative validation tests.
func testLeafCertPEM() string {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "e2e-test-leaf",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		IsCA:      false,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

var _ = Describe("Trusted CA Bundle", Ordered, Label("Platform:Generic", "Feature:TrustedCABundle"), func() {
	var (
		ctx context.Context
		esc *operatorv1alpha1.ExternalSecretsConfig
	)

	BeforeAll(func() {
		ctx = context.Background()

		By("Waiting for operator pod to be ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, suiteClientset, operatorNamespace, []string{
			operatorPodPrefix,
		})).To(Succeed())

		By("Ensuring ExternalSecretsConfig cluster CR exists and is Ready")
		Expect(ensureExternalSecretsConfigReady(ctx)).To(Succeed())

		esc = &operatorv1alpha1.ExternalSecretsConfig{}
		Expect(suiteRuntimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc)).To(Succeed())

		By("Waiting for operand pods to be ready before trusted CA bundle tests")
		Expect(utils.VerifyOperandPodsReady(ctx, suiteClientset, operandNamespace, esc)).To(Succeed())
	})

	AfterEach(func() {
		By("Clearing trustedCABundle from ExternalSecretsConfig")
		_ = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			if err := suiteRuntimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc); err != nil {
				return err
			}
			esc.Spec.ControllerConfig.TrustedCABundle = nil
			return suiteRuntimeClient.Update(ctx, esc)
		})

		By("Deleting the test CA ConfigMap (best-effort)")
		_ = suiteClientset.CoreV1().ConfigMaps(operandNamespace).Delete(ctx, trustedCABundleTestCMName, metav1.DeleteOptions{})
	})

	It("should mount user-ca-bundle volume and set SSL_CERT_DIR on the core controller deployment only", func() {
		By("Creating a ConfigMap with a valid test CA certificate")
		createTestCAConfigMap(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath, testCACertPEM(), nil)

		By("Setting trustedCABundle on ExternalSecretsConfig")
		setTrustedCABundle(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath)

		By("Waiting for ExternalSecretsConfig to be Ready")
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, suiteDynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

		By("Verifying the core controller deployment has the user-ca-bundle volume and SSL_CERT_DIR")
		Eventually(func(g Gomega) {
			deployment, err := suiteClientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandCoreControllerDeployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred(), "should get %s deployment", externalsecrets.OperandCoreControllerDeployment)

			assertHasUserCABundleVolume(g, deployment.Spec.Template.Spec.Volumes, trustedCABundleTestCMName)
			assertHasUserCABundleMount(g, deployment.Spec.Template.Spec.Containers, externalsecrets.OperandCoreControllerContainer)
			assertHasSSLCertDir(g, deployment.Spec.Template.Spec.Containers, externalsecrets.OperandCoreControllerContainer)
		}, time.Minute, 5*time.Second).Should(Succeed(), "core controller should have user CA bundle configured")

		By("Verifying the operator applied the watch label on the referenced ConfigMap")
		Eventually(func(g Gomega) {
			cm, err := suiteClientset.CoreV1().ConfigMaps(operandNamespace).Get(ctx, trustedCABundleTestCMName, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cm.Labels).To(HaveKeyWithValue(externalsecrets.WatchedResourceLabelKey, externalsecrets.WatchedResourceLabelValue))
		}, time.Minute, 5*time.Second).Should(Succeed())

		By("Verifying the webhook deployment does NOT have the user-ca-bundle volume or SSL_CERT_DIR")
		Eventually(func(g Gomega) {
			deployment, err := suiteClientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandWebhookDeployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred(), "should get %s deployment", externalsecrets.OperandWebhookDeployment)

			assertDoesNotHaveUserCABundleVolume(g, deployment.Spec.Template.Spec.Volumes)
			assertDoesNotHaveUserCABundleMount(g, deployment.Spec.Template.Spec.Containers)
			assertDoesNotHaveSSLCertDir(g, deployment.Spec.Template.Spec.Containers)
		}, time.Minute, 5*time.Second).Should(Succeed(), "webhook deployment should not have user CA bundle")

		if utils.IsCertControllerExpected(esc) {
			By("Verifying the cert-controller deployment does NOT have the user-ca-bundle volume or SSL_CERT_DIR")
			Eventually(func(g Gomega) {
				deployment, err := suiteClientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandCertControllerDeployment, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred(), "should get %s deployment", externalsecrets.OperandCertControllerDeployment)

				assertDoesNotHaveUserCABundleVolume(g, deployment.Spec.Template.Spec.Volumes)
				assertDoesNotHaveUserCABundleMount(g, deployment.Spec.Template.Spec.Containers)
				assertDoesNotHaveSSLCertDir(g, deployment.Spec.Template.Spec.Containers)
			}, time.Minute, 5*time.Second).Should(Succeed(), "cert-controller deployment should not have user CA bundle")
		}
	})

	It("should remove user-ca-bundle volume and SSL_CERT_DIR when trustedCABundle is cleared", func() {
		By("Creating a ConfigMap and configuring trustedCABundle")
		createTestCAConfigMap(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath, testCACertPEM(), nil)
		setTrustedCABundle(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath)

		By("Waiting for ExternalSecretsConfig to be Ready with volume mounted")
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, suiteDynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())
		Eventually(func(g Gomega) {
			deployment, err := suiteClientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandCoreControllerDeployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			assertHasUserCABundleVolume(g, deployment.Spec.Template.Spec.Volumes, trustedCABundleTestCMName)
		}, time.Minute, 5*time.Second).Should(Succeed(), "volume should be mounted before clearing")

		By("Clearing trustedCABundle from ExternalSecretsConfig")
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			if err := suiteRuntimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc); err != nil {
				return err
			}
			esc.Spec.ControllerConfig.TrustedCABundle = nil
			return suiteRuntimeClient.Update(ctx, esc)
		})
		Expect(err).NotTo(HaveOccurred(), "should clear trustedCABundle from ExternalSecretsConfig")

		By("Waiting for ExternalSecretsConfig to be Ready after clearing")
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, suiteDynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

		By("Verifying user-ca-bundle volume and SSL_CERT_DIR are removed from the core controller deployment")
		Eventually(func(g Gomega) {
			deployment, err := suiteClientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandCoreControllerDeployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred(), "should get %s deployment", externalsecrets.OperandCoreControllerDeployment)

			assertDoesNotHaveUserCABundleVolume(g, deployment.Spec.Template.Spec.Volumes)
			assertDoesNotHaveUserCABundleMount(g, deployment.Spec.Template.Spec.Containers)
			assertDoesNotHaveSSLCertDir(g, deployment.Spec.Template.Spec.Containers)
		}, time.Minute, 5*time.Second).Should(Succeed(), "core controller should have user CA bundle removed after clearing")
	})

	It("should restore the watch label on the trustedCABundle ConfigMap after external removal", func() {
		By("Creating a ConfigMap and configuring trustedCABundle")
		createTestCAConfigMap(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath, testCACertPEM(), nil)
		setTrustedCABundle(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath)

		By("Waiting for ExternalSecretsConfig to be Ready")
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, suiteDynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

		By("Verifying the operator applied the watch label on the referenced ConfigMap")
		Eventually(func(g Gomega) {
			cm, err := suiteClientset.CoreV1().ConfigMaps(operandNamespace).Get(ctx, trustedCABundleTestCMName, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cm.Labels).To(HaveKeyWithValue(externalsecrets.WatchedResourceLabelKey, externalsecrets.WatchedResourceLabelValue))
		}, time.Minute, 5*time.Second).Should(Succeed())

		By("Removing the watch label from the trustedCABundle ConfigMap")
		Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
			cm, err := suiteClientset.CoreV1().ConfigMaps(operandNamespace).Get(ctx, trustedCABundleTestCMName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if cm.Labels == nil {
				return nil
			}
			delete(cm.Labels, externalsecrets.WatchedResourceLabelKey)
			_, err = suiteClientset.CoreV1().ConfigMaps(operandNamespace).Update(ctx, cm, metav1.UpdateOptions{})
			return err
		})).To(Succeed(), "should remove the watch label")

		By("Waiting for the operator to restore the watch label")
		Eventually(func(g Gomega) {
			cm, err := suiteClientset.CoreV1().ConfigMaps(operandNamespace).Get(ctx, trustedCABundleTestCMName, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cm.Labels).To(HaveKeyWithValue(externalsecrets.WatchedResourceLabelKey, externalsecrets.WatchedResourceLabelValue),
				"operator should restore %s=%s on trustedCABundle ConfigMap %s",
				externalsecrets.WatchedResourceLabelKey, externalsecrets.WatchedResourceLabelValue, trustedCABundleTestCMName)
		}, 2*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should set ExternalSecretsConfig to Degraded when ConfigMap does not exist", func() {
		By("Setting trustedCABundle pointing to a non-existent ConfigMap")
		setTrustedCABundle(ctx, "does-not-exist-ca-bundle", externalsecrets.UserCABundleKeyPath)

		By("Waiting for ExternalSecretsConfig to become Degraded")
		Eventually(func(g Gomega) {
			g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
				"ExternalSecretsConfig should be Degraded when ConfigMap is missing")
		}, time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should set ExternalSecretsConfig to Degraded when the key is absent from ConfigMap", func() {
		By("Creating a ConfigMap without the expected key")
		createTestCAConfigMap(ctx, trustedCABundleTestCMName, "wrong-key", testCACertPEM(), nil)

		By("Setting trustedCABundle referencing a key that does not exist in the ConfigMap")
		setTrustedCABundle(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath)

		By("Waiting for ExternalSecretsConfig to become Degraded")
		Eventually(func(g Gomega) {
			g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
				"ExternalSecretsConfig should be Degraded when key is missing from ConfigMap")
		}, time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should set ExternalSecretsConfig to Degraded when ConfigMap contains a non-CA certificate", func() {
		By("Creating a ConfigMap with a leaf (non-CA) certificate")
		createTestCAConfigMap(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath, testLeafCertPEM(), nil)

		By("Setting trustedCABundle on ExternalSecretsConfig")
		setTrustedCABundle(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath)

		By("Waiting for ExternalSecretsConfig to become Degraded")
		Eventually(func(g Gomega) {
			g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
				"ExternalSecretsConfig should be Degraded when certificate is not a CA")
		}, time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should set ExternalSecretsConfig to Degraded when ConfigMap data is not valid PEM", func() {
		By("Creating a ConfigMap with invalid PEM at the referenced key")
		createTestCAConfigMap(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath, "# not a certificate", nil)

		By("Setting trustedCABundle on ExternalSecretsConfig")
		setTrustedCABundle(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath)

		By("Waiting for ExternalSecretsConfig to become Degraded")
		Eventually(func(g Gomega) {
			g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
				"ExternalSecretsConfig should be Degraded when PEM is invalid")
		}, time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should recover from Degraded when invalid PEM is fixed in the ConfigMap without changing ExternalSecretsConfig", func() {
		By("Creating a ConfigMap with invalid PEM")
		createTestCAConfigMap(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath, "# not a certificate", nil)
		setTrustedCABundle(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath)

		By("Waiting for ExternalSecretsConfig to become Degraded")
		Eventually(func(g Gomega) {
			g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue())
		}, time.Minute, 5*time.Second).Should(Succeed())

		By("Fixing ConfigMap data only (ConfigMap watch should trigger reconciliation)")
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			cm, err := suiteClientset.CoreV1().ConfigMaps(operandNamespace).Get(ctx, trustedCABundleTestCMName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			cm.Data[externalsecrets.UserCABundleKeyPath] = testCACertPEM()
			_, err = suiteClientset.CoreV1().ConfigMaps(operandNamespace).Update(ctx, cm, metav1.UpdateOptions{})
			return err
		})
		Expect(err).NotTo(HaveOccurred(), "should update ConfigMap with valid PEM")

		By("Waiting for ExternalSecretsConfig to recover without modifying the spec")
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, suiteDynamicClient, common.ExternalSecretsConfigObjectName, 3*time.Minute)).To(Succeed())

		Eventually(func(g Gomega) {
			deployment, err := suiteClientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandCoreControllerDeployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			assertHasUserCABundleVolume(g, deployment.Spec.Template.Spec.Volumes, trustedCABundleTestCMName)
		}, time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should recover from Degraded when the missing ConfigMap is created after initial reconciliation", func() {
		By("Setting trustedCABundle before the ConfigMap exists")
		setTrustedCABundle(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath)

		By("Waiting for ExternalSecretsConfig to become Degraded")
		Eventually(func(g Gomega) {
			g.Expect(isExternalSecretsConfigDegraded(ctx)).To(BeTrue(),
				"ExternalSecretsConfig should be Degraded when ConfigMap is missing")
		}, time.Minute, 5*time.Second).Should(Succeed())

		By("Creating the missing ConfigMap with valid PEM")
		createTestCAConfigMap(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath, testCACertPEM(), nil)

		By("Waiting for ExternalSecretsConfig to recover to Ready")
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, suiteDynamicClient, common.ExternalSecretsConfigObjectName, 3*time.Minute)).To(Succeed(),
			"ExternalSecretsConfig should recover from Degraded when ConfigMap is created")

		By("Verifying the core controller deployment has the user-ca-bundle volume after recovery")
		Eventually(func(g Gomega) {
			deployment, err := suiteClientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandCoreControllerDeployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			assertHasUserCABundleVolume(g, deployment.Spec.Template.Spec.Volumes, trustedCABundleTestCMName)
			assertHasSSLCertDir(g, deployment.Spec.Template.Spec.Containers, externalsecrets.OperandCoreControllerContainer)
		}, time.Minute, 5*time.Second).Should(Succeed(), "core controller should have user CA bundle after recovery")
	})

	It("should handle trustedCABundle ConfigMap with CNO inject label based on cluster proxy", func() {
		clusterProxyConfigured := isOpenShiftClusterProxyConfigured(ctx)

		By("Creating a ConfigMap with the CNO inject label")
		createTestCAConfigMap(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath, testCACertPEM(), map[string]string{
			externalsecrets.TrustedCABundleInjectLabel: "true",
		})

		By("Setting trustedCABundle on ExternalSecretsConfig (no proxy in ESC spec)")
		setTrustedCABundle(ctx, trustedCABundleTestCMName, externalsecrets.UserCABundleKeyPath)

		By("Waiting for ExternalSecretsConfig to be Ready")
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, suiteDynamicClient, common.ExternalSecretsConfigObjectName, 2*time.Minute)).To(Succeed())

		if clusterProxyConfigured {
			By("Verifying user CA mount is skipped and operator-managed trusted-ca-bundle is used when cluster proxy is configured")
			Eventually(func(g Gomega) {
				deployment, err := suiteClientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandCoreControllerDeployment, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				assertDoesNotHaveUserCABundleVolume(g, deployment.Spec.Template.Spec.Volumes)
				assertDoesNotHaveUserCABundleMount(g, deployment.Spec.Template.Spec.Containers)
				assertDoesNotHaveSSLCertDir(g, deployment.Spec.Template.Spec.Containers)
				assertHasProxyTrustedCABundleVolume(g, deployment.Spec.Template.Spec.Volumes)
			}, time.Minute, 5*time.Second).Should(Succeed(),
				"CNO-managed ConfigMap with cluster proxy should use operator trusted-ca-bundle, not user-ca-bundle")
			return
		}

		By("Verifying the core controller deployment has the user-ca-bundle volume when cluster proxy is not configured")
		Eventually(func(g Gomega) {
			deployment, err := suiteClientset.AppsV1().Deployments(operandNamespace).Get(ctx, externalsecrets.OperandCoreControllerDeployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			assertHasUserCABundleVolume(g, deployment.Spec.Template.Spec.Volumes, trustedCABundleTestCMName)
			assertHasSSLCertDir(g, deployment.Spec.Template.Spec.Containers, externalsecrets.OperandCoreControllerContainer)
		}, time.Minute, 5*time.Second).Should(Succeed(),
			"volume should be mounted when CNO label is present but cluster proxy is not configured")
	})
})

func createTestCAConfigMap(ctx context.Context, name, key, pemData string, extraLabels map[string]string) {
	GinkgoHelper()
	labels := map[string]string{}
	for k, v := range extraLabels {
		labels[k] = v
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: operandNamespace,
			Labels:    labels,
		},
		Data: map[string]string{
			key: pemData,
		},
	}
	existing, err := suiteClientset.CoreV1().ConfigMaps(operandNamespace).Get(ctx, name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		_, err = suiteClientset.CoreV1().ConfigMaps(operandNamespace).Create(ctx, cm, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred(), "should create test CA ConfigMap %s", name)
		return
	}
	Expect(err).NotTo(HaveOccurred(), "should check existence of test CA ConfigMap %s", name)
	existing.Data = cm.Data
	existing.Labels = cm.Labels
	_, err = suiteClientset.CoreV1().ConfigMaps(operandNamespace).Update(ctx, existing, metav1.UpdateOptions{})
	Expect(err).NotTo(HaveOccurred(), "should update existing test CA ConfigMap %s", name)
}

func setTrustedCABundle(ctx context.Context, cmName, key string) {
	GinkgoHelper()
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		esc := &operatorv1alpha1.ExternalSecretsConfig{}
		if err := suiteRuntimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc); err != nil {
			return err
		}
		esc.Spec.ControllerConfig.TrustedCABundle = &operatorv1alpha1.ConfigMapKeyReference{
			Name: cmName,
			Key:  key,
		}
		return suiteRuntimeClient.Update(ctx, esc)
	})
	Expect(err).NotTo(HaveOccurred(), "should update ExternalSecretsConfig with trustedCABundle name=%s key=%s", cmName, key)
}

func isExternalSecretsConfigDegraded(ctx context.Context) bool {
	esc := &operatorv1alpha1.ExternalSecretsConfig{}
	if err := suiteRuntimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc); err != nil {
		return false
	}
	for _, cond := range esc.Status.Conditions {
		if cond.Type == operatorv1alpha1.Degraded && cond.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func assertHasUserCABundleVolume(g Gomega, volumes []corev1.Volume, cmName string) {
	g.Expect(volumes).To(ContainElement(
		SatisfyAll(
			HaveField("Name", externalsecrets.UserCABundleVolumeName),
			HaveField("VolumeSource.ConfigMap", Not(BeNil())),
			HaveField("VolumeSource.ConfigMap.LocalObjectReference.Name", cmName),
		),
	), "volumes should contain %s sourced from ConfigMap %s", externalsecrets.UserCABundleVolumeName, cmName)
}

func assertDoesNotHaveUserCABundleVolume(g Gomega, volumes []corev1.Volume) {
	for _, v := range volumes {
		g.Expect(v.Name).NotTo(Equal(externalsecrets.UserCABundleVolumeName),
			"volumes should not contain %s", externalsecrets.UserCABundleVolumeName)
	}
}

func assertHasProxyTrustedCABundleVolume(g Gomega, volumes []corev1.Volume) {
	g.Expect(volumes).To(ContainElement(
		SatisfyAll(
			HaveField("Name", externalsecrets.ProxyTrustedCABundleVolumeName),
			HaveField("VolumeSource.ConfigMap", Not(BeNil())),
			HaveField("VolumeSource.ConfigMap.LocalObjectReference.Name", externalsecrets.ProxyTrustedCABundleConfigMapName),
		),
	), "volumes should contain %s sourced from ConfigMap %s",
		externalsecrets.ProxyTrustedCABundleVolumeName, externalsecrets.ProxyTrustedCABundleConfigMapName)
}

func isOpenShiftClusterProxyConfigured(ctx context.Context) bool {
	proxyGVR := schema.GroupVersionResource{
		Group:    "config.openshift.io",
		Version:  "v1",
		Resource: "proxies",
	}
	proxyCR, err := suiteDynamicClient.Resource(proxyGVR).Get(ctx, common.ExternalSecretsConfigObjectName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	for _, section := range []string{"status", "spec"} {
		httpProxy, _, _ := unstructured.NestedString(proxyCR.Object, section, "httpProxy")
		httpsProxy, _, _ := unstructured.NestedString(proxyCR.Object, section, "httpsProxy")
		noProxy, _, _ := unstructured.NestedString(proxyCR.Object, section, "noProxy")
		if httpProxy != "" || httpsProxy != "" || noProxy != "" {
			return true
		}
	}
	return false
}

func assertHasUserCABundleMount(g Gomega, containers []corev1.Container, containerName string) {
	for _, c := range containers {
		if c.Name != containerName {
			continue
		}
		g.Expect(c.VolumeMounts).To(ContainElement(
			SatisfyAll(
				HaveField("Name", externalsecrets.UserCABundleVolumeName),
				HaveField("MountPath", externalsecrets.UserCABundleMountPath),
				HaveField("ReadOnly", BeTrue()),
			),
		), "container %s should have %s volume mount at %s (ReadOnly)", containerName, externalsecrets.UserCABundleVolumeName, externalsecrets.UserCABundleMountPath)
		return
	}
	Fail("container " + containerName + " not found in deployment")
}

func assertDoesNotHaveUserCABundleMount(g Gomega, containers []corev1.Container) {
	for _, c := range containers {
		for _, vm := range c.VolumeMounts {
			g.Expect(vm.Name).NotTo(Equal(externalsecrets.UserCABundleVolumeName),
				"container %s should not have %s volume mount", c.Name, externalsecrets.UserCABundleVolumeName)
		}
	}
}

func assertHasSSLCertDir(g Gomega, containers []corev1.Container, containerName string) {
	for _, c := range containers {
		if c.Name != containerName {
			continue
		}
		g.Expect(c.Env).To(ContainElement(
			SatisfyAll(
				HaveField("Name", externalsecrets.SSLCertDirEnvVar),
				HaveField("Value", externalsecrets.SSLCertDirValue),
			),
		), "container %s should have %s=%s", containerName, externalsecrets.SSLCertDirEnvVar, externalsecrets.SSLCertDirValue)
		return
	}
	Fail("container " + containerName + " not found in deployment")
}

func assertDoesNotHaveSSLCertDir(g Gomega, containers []corev1.Container) {
	for _, c := range containers {
		for _, env := range c.Env {
			g.Expect(env.Name).NotTo(Equal(externalsecrets.SSLCertDirEnvVar),
				"container %s should not have %s env var", c.Name, externalsecrets.SSLCertDirEnvVar)
		}
	}
}
