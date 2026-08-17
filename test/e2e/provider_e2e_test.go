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
	"encoding/base64"
	"fmt"
	"regexp"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/test/utils"
)

const (
	generatorsGroupName   = "generators.external-secrets.io"
	clusterGeneratorsKind = "clustergenerators"
	awsRegion             = "us-east-1"
)

var _ = Describe("ESO Extended Tests Multiple Providers", Label("OpenShiftTestsPrivate"), Ordered, func() {
	ctx := context.Background()
	var (
		providerClientset     *kubernetes.Clientset
		providerDynamicClient *dynamic.DynamicClient
		providerRuntimeClient client.Client
		providerLoader        utils.DynamicResourceLoader
		providerTestNamespace string
	)

	secretStoreGVR := schema.GroupVersionResource{
		Group: externalSecretsGroupName, Version: v1APIVersion, Resource: secretStoresKind,
	}
	externalSecretGVR := schema.GroupVersionResource{
		Group: externalSecretsGroupName, Version: v1APIVersion, Resource: externalSecretsKind,
	}
	pushSecretGVR := schema.GroupVersionResource{
		Group: externalSecretsGroupName, Version: v1alpha1APIVersion, Resource: PushSecretsKind,
	}
	clusterGeneratorGVR := schema.GroupVersionResource{
		Group: generatorsGroupName, Version: v1alpha1APIVersion, Resource: clusterGeneratorsKind,
	}
	BeforeAll(func() {
		providerClientset = suiteClientset
		providerDynamicClient = suiteDynamicClient
		providerRuntimeClient = suiteRuntimeClient
		providerLoader = utils.NewDynamicResourceLoader(ctx, &testing.T{})

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"e2e-test": "true",
					"operator": "openshift-external-secrets-operator",
				},
				GenerateName: testNamespacePrefix,
			},
		}
		By("Creating the provider test namespace")
		got, err := providerClientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		providerTestNamespace = got.GetName()

		By("Waiting for operator pod to be ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, providerClientset, operatorNamespace, []string{
			operatorPodPrefix,
		})).To(Succeed())

		By("Ensuring ExternalSecretsConfig cluster CR exists and is Ready")
		Expect(ensureExternalSecretsConfigReady(ctx)).To(Succeed())
	})

	BeforeEach(func() {
		By("Verifying external-secrets operand pods are ready")
		esc := &operatorv1alpha1.ExternalSecretsConfig{}
		Expect(providerRuntimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc)).To(Succeed())
		Expect(utils.VerifyOperandPodsReady(ctx, providerClientset, operandNamespace, esc)).To(Succeed())
	})

	AfterEach(func() {
		if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
			return
		}
		artifactDir := getTestDir()
		By(fmt.Sprintf("Test failed: dumping logs and resources to %s/e2e-artifacts/", artifactDir))
		if err := utils.DumpE2EArtifacts(ctx, providerClientset, providerDynamicClient, operatorNamespace, operandNamespace, providerTestNamespace, artifactDir); err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "warning: failed to dump e2e artifacts: %v\n", err)
		}
	})

	// ──────────────────────────────────────────────────────────────────
	// Context: AWS Parameter Store
	// ──────────────────────────────────────────────────────────────────

	Context("AWS Parameter Store", Label("Platform:AWS"), func() {
		const awsCredsLocalName = "aws-creds"

		BeforeAll(func() {
			By("Copying AWS credentials into the test namespace")
			Expect(utils.CopyAWSCredsToNamespace(ctx, providerClientset, providerTestNamespace, awsCredsLocalName)).To(Succeed())
		})

		// OCP-80759: Sync from AWS Parameter Store and verify updates
		It("[OCP-80759] should sync and re-sync from AWS Parameter Store", func() {
			var (
				paramName    = fmt.Sprintf("eso-e2e-80759-%s", utils.GetRandomString(5))
				storeName    = "secretstore-80759"
				esName       = "externalsecret-80759"
				targetSecret = "secret-from-ps-80759"
				secretKey    = "value-80759"
				initialValue = utils.GetRandomString(8)
				updatedValue = utils.GetRandomString(8)
			)

			By("Creating an AWS SSM parameter")
			Expect(utils.PutAWSSSMParameter(ctx, providerClientset, paramName, initialValue, awsRegion)).To(Succeed())
			defer func() {
				_ = utils.DeleteAWSSSMParameter(ctx, providerClientset, paramName, awsRegion)
			}()

			By("Creating SecretStore for AWS Parameter Store")
			store := utils.AWSSecretStore(storeName, providerTestNamespace, awsRegion, "ParameterStore", awsCredsLocalName)
			Expect(providerLoader.CreateFromUnstructuredReturnErr(store, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(store, providerTestNamespace)

			By("Waiting for SecretStore to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, secretStoreGVR, providerTestNamespace, storeName, 2*time.Minute)).To(Succeed())

			By("Creating ExternalSecret")
			es := utils.AWSExternalSecret(esName, providerTestNamespace, storeName, targetSecret, "10s", paramName, secretKey, "")
			Expect(providerLoader.CreateFromUnstructuredReturnErr(es, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(es, providerTestNamespace)

			By("Waiting for ExternalSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, externalSecretGVR, providerTestNamespace, esName, 2*time.Minute)).To(Succeed())

			By("Verifying initial parameter value synced")
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(secret.Data[secretKey])).To(Equal(initialValue))
			}, time.Minute, 5*time.Second).Should(Succeed())

			By("Updating the SSM parameter")
			Expect(utils.PutAWSSSMParameter(ctx, providerClientset, paramName, updatedValue, awsRegion)).To(Succeed())

			By("Waiting for k8s secret to reflect updated value")
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(secret.Data[secretKey])).To(Equal(updatedValue))
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})

		// OCP-80569: ESO takes ownership of orphan secrets
		It("[OCP-80569] should take ownership of orphan secrets", func() {
			var (
				awsSecretName   = fmt.Sprintf("eso-e2e-80569-%s", utils.GetRandomString(5))
				storeName       = "secretstore-80569"
				esName          = "externalsecret-80569"
				targetSecret    = fmt.Sprintf("orphan-secret-80569-%s", utils.GetRandomString(4))
				secretKey       = "password-80569"
				unrelatedKey    = "unrelatedkey"
				initialPassword = utils.GetRandomString(8)
			)

			By("Creating an AWS Secrets Manager secret")
			secretJSON := fmt.Sprintf(`{"%s":"%s"}`, secretKey, initialPassword)
			Expect(utils.CreateAWSSecret(ctx, providerClientset, awsSecretName, secretJSON, awsRegion)).To(Succeed())
			defer func() {
				_ = utils.DeleteAWSSecret(ctx, providerClientset, awsSecretName, awsRegion)
			}()

			By("Creating SecretStore")
			store := utils.AWSSecretStore(storeName, providerTestNamespace, awsRegion, "SecretsManager", awsCredsLocalName)
			Expect(providerLoader.CreateFromUnstructuredReturnErr(store, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(store, providerTestNamespace)
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, secretStoreGVR, providerTestNamespace, storeName, 2*time.Minute)).To(Succeed())

			By("Creating an orphan k8s secret")
			orphan := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      targetSecret,
					Namespace: providerTestNamespace,
				},
				StringData: map[string]string{
					secretKey:    "old-value",
					unrelatedKey: "should-be-removed",
				},
			}
			_, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Create(ctx, orphan, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the orphan has no ownerReferences")
			secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.OwnerReferences).To(BeEmpty())

			By("Creating ExternalSecret with creationPolicy Owner targeting the same secret name")
			es := utils.AWSExternalSecretWithPolicy(esName, providerTestNamespace, storeName, targetSecret, "10s", awsSecretName, secretKey, secretKey, "Owner")
			Expect(providerLoader.CreateFromUnstructuredReturnErr(es, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(es, providerTestNamespace)
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, externalSecretGVR, providerTestNamespace, esName, 2*time.Minute)).To(Succeed())

			By("Verifying ESO set ownerReferences on the secret")
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(secret.OwnerReferences).NotTo(BeEmpty())
				found := false
				for _, ref := range secret.OwnerReferences {
					if ref.Kind == "ExternalSecret" {
						found = true
						g.Expect(ref.Controller).NotTo(BeNil())
						g.Expect(*ref.Controller).To(BeTrue())
					}
				}
				g.Expect(found).To(BeTrue(), "expected ExternalSecret ownerReference")
			}, time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying the unrelated key was removed")
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				_, exists := secret.Data[unrelatedKey]
				g.Expect(exists).To(BeFalse(), "unrelated key should be removed by ESO")
			}, time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying managed key has the correct value from remote")
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(secret.Data[secretKey])).To(Equal(initialPassword))
			}, time.Minute, 5*time.Second).Should(Succeed())

			By("Creating a second ExternalSecret targeting the same secret - should fail with ownership conflict")
			es2Name := esName + "-vie"
			es2 := utils.AWSExternalSecretWithPolicy(es2Name, providerTestNamespace, storeName, targetSecret, "10s", awsSecretName, secretKey, secretKey, "Owner")
			Expect(providerLoader.CreateFromUnstructuredReturnErr(es2, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(es2, providerTestNamespace)

			Eventually(func(g Gomega) {
				u, err := providerDynamicClient.Resource(externalSecretGVR).Namespace(providerTestNamespace).Get(ctx, es2Name, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
				for _, c := range conds {
					cond, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					if cond["type"] == "Ready" {
						msg, _ := cond["message"].(string)
						g.Expect(msg).To(ContainSubstring("owned by another ExternalSecret"))
						return
					}
				}
				g.Expect(false).To(BeTrue(), "expected Ready condition with ownership error")
			}, time.Minute, 5*time.Second).Should(Succeed())
		})

		// OCP-81666: Check ESO decoding strategies
		It("[OCP-81666] should handle decoding strategies", func() {
			var (
				awsSecretName = fmt.Sprintf("eso-e2e-81666-%s", utils.GetRandomString(5))
				storeName     = "secretstore-81666"
				esName        = "externalsecret-81666"
				targetSecret  = "secret-from-awssm-81666"
				secretKey     = "password-81666"
				secretValue   = utils.GetRandomString(8)
			)

			By("Creating an AWS Secrets Manager secret")
			secretJSON := fmt.Sprintf(`{"%s":"%s"}`, secretKey, secretValue)
			Expect(utils.CreateAWSSecret(ctx, providerClientset, awsSecretName, secretJSON, awsRegion)).To(Succeed())
			defer func() {
				_ = utils.DeleteAWSSecret(ctx, providerClientset, awsSecretName, awsRegion)
			}()

			By("Creating SecretStore")
			store := utils.AWSSecretStore(storeName, providerTestNamespace, awsRegion, "SecretsManager", awsCredsLocalName)
			Expect(providerLoader.CreateFromUnstructuredReturnErr(store, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(store, providerTestNamespace)
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, secretStoreGVR, providerTestNamespace, storeName, 2*time.Minute)).To(Succeed())

			By("Creating ExternalSecret with dataFrom extract")
			es := utils.AWSExternalSecretWithDataFrom(esName, providerTestNamespace, storeName, targetSecret, "10s", awsSecretName)
			Expect(providerLoader.CreateFromUnstructuredReturnErr(es, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(es, providerTestNamespace)
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, externalSecretGVR, providerTestNamespace, esName, 2*time.Minute)).To(Succeed())

			By("Verifying initial synced value")
			var initialData string
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				data, exists := secret.Data[secretKey]
				g.Expect(exists).To(BeTrue())
				g.Expect(data).NotTo(BeEmpty())
				initialData = string(data)
			}, time.Minute, 5*time.Second).Should(Succeed())

			By("Patching ExternalSecret to enable decodingStrategy Auto")
			patch := []byte(`{"spec":{"dataFrom":[{"extract":{"key":"` + awsSecretName + `","decodingStrategy":"Auto"}}]}}`)
			_, err := providerDynamicClient.Resource(externalSecretGVR).Namespace(providerTestNamespace).Patch(
				ctx, esName, "application/merge-patch+json", patch, metav1.PatchOptions{},
			)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the decoded value")
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				data := secret.Data[secretKey]
				g.Expect(data).NotTo(BeEmpty())
				g.Expect(string(data)).To(Equal(initialData), "decoded value should match")
			}, time.Minute, 5*time.Second).Should(Succeed())
		})

		// OCP-81708: Push key value to AWS Secrets Manager
		It("[OCP-81708] should push key value to AWS Secrets Manager", func() {
			var (
				smSecretName = fmt.Sprintf("eso-e2e-81708-%s", utils.GetRandomString(5))
				storeName    = "secretstore-81708"
				psName       = "pushsecret-81708"
				esName       = "externalsecret-81708"
				targetSecret = "target-secret-81708"
				sourceSecret = "source-secret-81708"
				secretKey    = "secret-access-key"
				initialValue = utils.GetRandomString(8)
				updatedValue = utils.GetRandomString(8)
			)

			By("Creating SecretStore for AWS Secrets Manager")
			store := utils.AWSSecretStore(storeName, providerTestNamespace, awsRegion, "SecretsManager", awsCredsLocalName)
			Expect(providerLoader.CreateFromUnstructuredReturnErr(store, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(store, providerTestNamespace)
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, secretStoreGVR, providerTestNamespace, storeName, 2*time.Minute)).To(Succeed())

			By("Creating source k8s secret")
			src := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: sourceSecret, Namespace: providerTestNamespace},
				StringData: map[string]string{secretKey: initialValue},
			}
			_, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Create(ctx, src, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = providerClientset.CoreV1().Secrets(providerTestNamespace).Delete(ctx, sourceSecret, metav1.DeleteOptions{})
			}()

			By("Creating PushSecret to push specific key to AWSSM")
			ps := utils.AWSPushSecretKey(psName, providerTestNamespace, storeName, sourceSecret, secretKey, smSecretName, "10s")
			Expect(providerLoader.CreateFromUnstructuredReturnErr(ps, providerTestNamespace)).To(Succeed())
			defer func() {
				providerLoader.DeleteFromUnstructured(ps, providerTestNamespace)
				_ = utils.DeleteAWSSecret(ctx, providerClientset, smSecretName, awsRegion)
			}()

			By("Waiting for PushSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, pushSecretGVR, providerTestNamespace, psName, 2*time.Minute)).To(Succeed())

			By("Creating ExternalSecret to pull the pushed value back")
			es := utils.AWSExternalSecret(esName, providerTestNamespace, storeName, targetSecret, "10s", smSecretName, secretKey, "")
			Expect(providerLoader.CreateFromUnstructuredReturnErr(es, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(es, providerTestNamespace)
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, externalSecretGVR, providerTestNamespace, esName, 2*time.Minute)).To(Succeed())

			By("Verifying pushed value via ExternalSecret round-trip")
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(secret.Data[secretKey])).To(ContainSubstring(initialValue))
			}, time.Minute, 10*time.Second).Should(Succeed())

			By("Updating the source k8s secret")
			existing, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, sourceSecret, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			existing.Data[secretKey] = []byte(updatedValue)
			_, err = providerClientset.CoreV1().Secrets(providerTestNamespace).Update(ctx, existing, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying updated value via ExternalSecret round-trip")
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(secret.Data[secretKey])).To(ContainSubstring(updatedValue))
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})

		// OCP-81709: Push entire secret to AWS Parameter Store
		It("[OCP-81709] should push entire secret to AWS Parameter Store", func() {
			var (
				paramName    = fmt.Sprintf("eso-e2e-81709-%s", utils.GetRandomString(5))
				storeName    = "secretstore-81709"
				psName       = "pushsecret-81709"
				esName       = "externalsecret-81709"
				targetSecret = "target-secret-81709"
				sourceSecret = "source-secret-81709"
				secretKey    = "secret-access-key"
				initialValue = utils.GetRandomString(8)
				updatedValue = utils.GetRandomString(8)
			)

			By("Creating SecretStore for AWS Parameter Store")
			store := utils.AWSSecretStore(storeName, providerTestNamespace, awsRegion, "ParameterStore", awsCredsLocalName)
			Expect(providerLoader.CreateFromUnstructuredReturnErr(store, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(store, providerTestNamespace)
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, secretStoreGVR, providerTestNamespace, storeName, 2*time.Minute)).To(Succeed())

			By("Creating source k8s secret")
			src := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: sourceSecret, Namespace: providerTestNamespace},
				StringData: map[string]string{secretKey: initialValue},
			}
			_, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Create(ctx, src, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = providerClientset.CoreV1().Secrets(providerTestNamespace).Delete(ctx, sourceSecret, metav1.DeleteOptions{})
			}()

			By("Creating PushSecret to push entire secret to AWS PS")
			ps := utils.AWSPushSecretEntire(psName, providerTestNamespace, storeName, sourceSecret, paramName, "10s")
			Expect(providerLoader.CreateFromUnstructuredReturnErr(ps, providerTestNamespace)).To(Succeed())
			defer func() {
				providerLoader.DeleteFromUnstructured(ps, providerTestNamespace)
				_ = utils.DeleteAWSSSMParameter(ctx, providerClientset, paramName, awsRegion)
			}()

			By("Waiting for PushSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, pushSecretGVR, providerTestNamespace, psName, 2*time.Minute)).To(Succeed())

			By("Creating ExternalSecret to pull the pushed value back")
			es := utils.AWSExternalSecret(esName, providerTestNamespace, storeName, targetSecret, "10s", paramName, secretKey, "")
			Expect(providerLoader.CreateFromUnstructuredReturnErr(es, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(es, providerTestNamespace)
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, externalSecretGVR, providerTestNamespace, esName, 2*time.Minute)).To(Succeed())

			By("Verifying pushed value via ExternalSecret round-trip")
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(secret.Data[secretKey])).To(ContainSubstring(base64.StdEncoding.EncodeToString([]byte(initialValue))))
			}, time.Minute, 10*time.Second).Should(Succeed())

			By("Updating the source k8s secret")
			existing, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, sourceSecret, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			existing.Data[secretKey] = []byte(updatedValue)
			_, err = providerClientset.CoreV1().Secrets(providerTestNamespace).Update(ctx, existing, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying updated value via ExternalSecret round-trip")
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(secret.Data[secretKey])).To(ContainSubstring(base64.StdEncoding.EncodeToString([]byte(updatedValue))))
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	// ──────────────────────────────────────────────────────────────────
	// Context: ESO Generator
	// ──────────────────────────────────────────────────────────────────

	Context("ESO Generator", Label("Platform:AWS"), func() {
		var clusterGeneratorNames []string

		AfterAll(func() {
			By("Cleaning up ClusterGenerators")
			for _, name := range clusterGeneratorNames {
				_ = providerDynamicClient.Resource(clusterGeneratorGVR).Delete(ctx, name, metav1.DeleteOptions{})
			}
		})

		// OCP-81695: Generate random passwords
		It("[OCP-81695] should generate random passwords with ClusterGenerator and namespaced Generator", func() {
			var (
				cgName         = fmt.Sprintf("cg-password-81695-%s", utils.GetRandomString(4))
				esName         = "externalsecret-cg-81695"
				targetSecret   = "secret-from-cg-81695"
				genName        = fmt.Sprintf("gen-password-81695-%s", utils.GetRandomString(4))
				esNameNS       = "externalsecret-gen-81695"
				targetSecretNS = "secret-from-gen-81695"
				secretKey      = "password"
			)

			By("Creating ClusterGenerator (Password, length=32, digits=5, symbols=5)")
			cg := utils.PasswordClusterGenerator(cgName, 32, 5, 5, "-_$@!", false)
			Expect(providerLoader.CreateFromUnstructuredReturnErr(cg, "")).To(Succeed())
			clusterGeneratorNames = append(clusterGeneratorNames, cgName)

			By("Creating ExternalSecret referencing ClusterGenerator")
			es := utils.ExternalSecretWithGenerator(esName, providerTestNamespace, targetSecret, cgName, "ClusterGenerator", "30s")
			Expect(providerLoader.CreateFromUnstructuredReturnErr(es, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(es, providerTestNamespace)

			By("Waiting for ExternalSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, externalSecretGVR, providerTestNamespace, esName, 2*time.Minute)).To(Succeed())

			By("Verifying generated password format")
			var firstPassword string
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				raw, exists := secret.Data[secretKey]
				g.Expect(exists).To(BeTrue())
				firstPassword = string(raw)
				g.Expect(len(firstPassword)).To(Equal(32), "expected password length 32")
				g.Expect(regexp.MustCompile(`[0-9]`).FindAllString(firstPassword, -1)).To(HaveLen(5))
				g.Expect(regexp.MustCompile(`[-_\$@!]`).FindAllString(firstPassword, -1)).To(HaveLen(5))
				g.Expect(regexp.MustCompile(`[A-Z]`).MatchString(firstPassword)).To(BeTrue())
				g.Expect(regexp.MustCompile(`[a-z]`).MatchString(firstPassword)).To(BeTrue())
			}, time.Minute, 5*time.Second).Should(Succeed())

			By("Waiting for password to regenerate on refresh")
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				newPassword := string(secret.Data[secretKey])
				g.Expect(newPassword).NotTo(Equal(firstPassword), "password should change on refresh")
			}, 2*time.Minute, 10*time.Second).Should(Succeed())

			By("Creating namespace-scoped Password Generator")
			gen := utils.PasswordNamespacedGenerator(genName, providerTestNamespace, 32, 5, 5, "-_$@!", false)
			Expect(providerLoader.CreateFromUnstructuredReturnErr(gen, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(gen, providerTestNamespace)

			By("Creating ExternalSecret referencing namespaced Generator")
			esNS := utils.ExternalSecretWithGenerator(esNameNS, providerTestNamespace, targetSecretNS, genName, "Password", "30s")
			Expect(providerLoader.CreateFromUnstructuredReturnErr(esNS, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(esNS, providerTestNamespace)

			By("Waiting for namespaced ExternalSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, externalSecretGVR, providerTestNamespace, esNameNS, 2*time.Minute)).To(Succeed())

			By("Verifying namespaced generator password format")
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecretNS, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				pw := string(secret.Data[secretKey])
				g.Expect(len(pw)).To(Equal(32))
				g.Expect(regexp.MustCompile(`[0-9]`).FindAllString(pw, -1)).To(HaveLen(5))
				g.Expect(regexp.MustCompile(`[-_\$@!]`).FindAllString(pw, -1)).To(HaveLen(5))
				g.Expect(regexp.MustCompile(`[A-Z]`).MatchString(pw)).To(BeTrue())
				g.Expect(regexp.MustCompile(`[a-z]`).MatchString(pw)).To(BeTrue())
			}, time.Minute, 5*time.Second).Should(Succeed())
		})

		// OCP-81813: Generator + push to AWS Parameter Store
		It("[OCP-81813] should generate and push password to AWS Parameter Store", Label("Platform:AWS"), func() {
			const awsCredsLocalName = "aws-creds-gen"
			var (
				cgName       = fmt.Sprintf("cg-password-81813-%s", utils.GetRandomString(4))
				esName       = "externalsecret-cg-81813"
				targetSecret = "secret-from-cg-81813"
				secretKey    = "password"
				storeName    = "secretstore-81813"
				psName       = "pushsecret-81813"
				paramName    = fmt.Sprintf("eso-e2e-81813-%s", utils.GetRandomString(5))
			)

			By("Copying AWS credentials")
			_ = utils.CopyAWSCredsToNamespace(ctx, providerClientset, providerTestNamespace, awsCredsLocalName)

			By("Creating ClusterGenerator (Password, length=16)")
			cg := utils.PasswordClusterGenerator(cgName, 16, 0, 0, "", true)
			Expect(providerLoader.CreateFromUnstructuredReturnErr(cg, "")).To(Succeed())
			clusterGeneratorNames = append(clusterGeneratorNames, cgName)

			By("Creating ExternalSecret to generate password")
			es := utils.ExternalSecretWithGenerator(esName, providerTestNamespace, targetSecret, cgName, "ClusterGenerator", "30s")
			Expect(providerLoader.CreateFromUnstructuredReturnErr(es, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(es, providerTestNamespace)
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, externalSecretGVR, providerTestNamespace, esName, 2*time.Minute)).To(Succeed())

			By("Verifying password was generated")
			var generatedPassword string
			Eventually(func(g Gomega) {
				secret, err := providerClientset.CoreV1().Secrets(providerTestNamespace).Get(ctx, targetSecret, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				raw, exists := secret.Data[secretKey]
				g.Expect(exists).To(BeTrue())
				generatedPassword = string(raw)
				g.Expect(len(generatedPassword)).To(Equal(16))
			}, time.Minute, 5*time.Second).Should(Succeed())

			By("Creating AWS Parameter Store SecretStore")
			awsStore := utils.AWSSecretStore(storeName, providerTestNamespace, awsRegion, "ParameterStore", awsCredsLocalName)
			Expect(providerLoader.CreateFromUnstructuredReturnErr(awsStore, providerTestNamespace)).To(Succeed())
			defer providerLoader.DeleteFromUnstructured(awsStore, providerTestNamespace)
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, secretStoreGVR, providerTestNamespace, storeName, 2*time.Minute)).To(Succeed())

			By("Creating PushSecret to push generated password to AWS PS")
			ps := utils.AWSPushSecretKey(psName, providerTestNamespace, storeName, targetSecret, secretKey, paramName, "10s")
			Expect(providerLoader.CreateFromUnstructuredReturnErr(ps, providerTestNamespace)).To(Succeed())
			defer func() {
				providerLoader.DeleteFromUnstructured(ps, providerTestNamespace)
				_ = utils.DeleteAWSSSMParameter(ctx, providerClientset, paramName, awsRegion)
			}()

			By("Waiting for PushSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, providerDynamicClient, pushSecretGVR, providerTestNamespace, psName, 2*time.Minute)).To(Succeed())

			By("Verifying the generated password was pushed to AWS Parameter Store")
			Eventually(func(g Gomega) {
				val, err := utils.GetAWSSSMParameter(ctx, providerClientset, paramName, awsRegion)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(val).To(ContainSubstring(generatedPassword))
			}, time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	AfterAll(func() {
		if providerTestNamespace != "" {
			By("Deleting the provider test namespace")
			_ = providerClientset.CoreV1().Namespaces().Delete(ctx, providerTestNamespace, metav1.DeleteOptions{})
		}
	})
})
