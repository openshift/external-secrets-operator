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
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/external-secrets-operator/test/utils"
)

//go:embed testdata/*
var testassets embed.FS

const (
	// test bindata
	externalSecretsFile     = "testdata/external_secret.yaml"
	expectedSecretValueFile = "testdata/expected_value.yaml"
)

const (
	// test resource names
	operatorNamespace              = "external-secrets-operator"
	operandNamespace               = "external-secrets"
	operatorPodPrefix              = "external-secrets-operator-controller-manager-"
	operandCoreControllerPodPrefix = "external-secrets-"
	operandCertControllerPodPrefix = "external-secrets-cert-controller-"
	operandWebhookPodPrefix        = "external-secrets-webhook-"
	testNamespacePrefix            = "external-secrets-e2e-test-"
)

const (
	externalSecretsGroupName = "external-secrets.io"
	v1APIVersion             = "v1"
	v1alpha1APIVersion       = "v1alpha1"
	clusterSecretStoresKind  = "clustersecretstores"
	PushSecretsKind          = "pushsecrets"
	externalSecretsKind      = "externalsecrets"
	awsSecretRegionName      = "ap-south-1"
)

var _ = Describe("External Secrets Operator End-to-End test scenarios", Ordered, func() {
	ctx := context.TODO()
	var (
		clientset     *kubernetes.Clientset
		dynamicClient *dynamic.DynamicClient
		loader        utils.DynamicResourceLoader
		awsSecretName string
		testNamespace string
	)

	BeforeAll(func() {
		var err error
		loader = utils.NewDynamicResourceLoader(ctx, &testing.T{})

		clientset, err = kubernetes.NewForConfig(cfg)
		Expect(err).Should(BeNil())

		dynamicClient, err = dynamic.NewForConfig(cfg)
		Expect(err).Should(BeNil())

		awsSecretName = fmt.Sprintf("eso-e2e-secret-%s", utils.GetRandomString(5))

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"e2e-test": "true",
					"operator": "openshift-external-secrets-operator",
				},
				GenerateName: testNamespacePrefix,
			},
		}
		By("Creating the test namespace")
		got, err := clientset.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to create test namespace")
		testNamespace = got.GetName()

		By("Waiting for operator pod to be ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespace, []string{
			operatorPodPrefix,
		})).To(Succeed())

		By("Creating the externalsecrets.openshift.operator.io/cluster CR")
		loader.CreateFromFile(testassets.ReadFile, externalSecretsFile, "")
	})

	AfterAll(func() {
		By("Deleting the externalsecrets.openshift.operator.io/cluster CR")
		loader.DeleteFromFile(testassets.ReadFile, externalSecretsFile, "")

		By("Deleting the test namespace")
		Expect(clientset.CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})).
			NotTo(HaveOccurred(), "failed to delete test namespace")
	})

	BeforeEach(func() {
		By("Verifying external-secrets operand pods are ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
			operandCoreControllerPodPrefix,
			operandCertControllerPodPrefix,
			operandWebhookPodPrefix,
		})).To(Succeed())
	})

	Context("AWS Secret Manager", Label("Platform:AWS"), func() {
		const (
			clusterSecretStoreFile           = "testdata/aws_secret_store.yaml"
			externalSecretFile               = "testdata/aws_external_secret.yaml"
			pushSecretFile                   = "testdata/push_secret.yaml"
			awsSecretToPushFile              = "testdata/aws_k8s_push_secret.yaml"
			awsSecretNamePattern             = "${AWS_SECRET_KEY_NAME}"
			awsSecretValuePattern            = "${SECRET_VALUE}"
			awsClusterSecretStoreNamePattern = "${CLUSTERSECRETSTORE_NAME}"
		)

		AfterAll(func() {
			By("Deleting the AWS secret")
			Expect(utils.DeleteAWSSecret(ctx, clientset, awsSecretName, awsSecretRegionName)).
				NotTo(HaveOccurred(), "failed to delete AWS secret test/e2e")
		})

		It("should create secrets mentioned in ExternalSecret using the referenced ClusterSecretStore", func() {
			var (
				clusterSecretStoreResourceName = fmt.Sprintf("aws-secret-store-%s", utils.GetRandomString(5))
				pushSecretResourceName         = "aws-push-secret"
				externalSecretResourceName     = "aws-external-secret"
				secretResourceName             = "aws-secret"
				keyNameInSecret                = "aws_secret_access_key"
			)

			defer func() {
				Expect(utils.DeleteAWSSecret(ctx, clientset, awsSecretName, awsSecretRegionName)).
					NotTo(HaveOccurred(), "failed to delete AWS secret test/e2e")
			}()

			expectedSecretValue, err := utils.ReadExpectedSecretValue(expectedSecretValueFile)
			Expect(err).To(Succeed())

			By("Creating kubernetes secret to be used in PushSecret")
			secretsAssetFunc := utils.ReplacePatternInAsset(awsSecretValuePattern, base64.StdEncoding.EncodeToString(expectedSecretValue))
			loader.CreateFromFile(secretsAssetFunc, awsSecretToPushFile, testNamespace)
			defer loader.DeleteFromFile(testassets.ReadFile, awsSecretToPushFile, testNamespace)

			By("Creating ClusterSecretStore")
			cssAssetFunc := utils.ReplacePatternInAsset(
				awsClusterSecretStoreNamePattern, clusterSecretStoreResourceName,
				"${AWS_REGION}", awsSecretRegionName,
			)
			loader.CreateFromFile(cssAssetFunc, clusterSecretStoreFile, testNamespace)
			defer loader.DeleteFromFile(cssAssetFunc, clusterSecretStoreFile, testNamespace)

			By("Waiting for ClusterSecretStore to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: clusterSecretStoresKind,
				},
				"", clusterSecretStoreResourceName, time.Minute,
			)).To(Succeed())

			By("Creating PushSecret")
			assetFunc := utils.ReplacePatternInAsset(awsSecretNamePattern, awsSecretName,
				awsClusterSecretStoreNamePattern, clusterSecretStoreResourceName)
			loader.CreateFromFile(assetFunc, pushSecretFile, testNamespace)
			defer loader.DeleteFromFile(testassets.ReadFile, pushSecretFile, testNamespace)

			By("Waiting for PushSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1alpha1APIVersion,
					Resource: PushSecretsKind,
				},
				testNamespace, pushSecretResourceName, time.Minute,
			)).To(Succeed())

			By("Creating ExternalSecret")
			loader.CreateFromFile(assetFunc, externalSecretFile, testNamespace)
			defer loader.DeleteFromFile(testassets.ReadFile, externalSecretFile, testNamespace)

			By("Waiting for ExternalSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: externalSecretsKind,
				},
				testNamespace, externalSecretResourceName, time.Minute,
			)).To(Succeed())

			By("Waiting for target secret to be created with expected data")
			Eventually(func(g Gomega) {
				secret, err := loader.KubeClient.CoreV1().Secrets(testNamespace).Get(ctx, secretResourceName, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred(), "should get %s from namespace %s", secretResourceName, testNamespace)

				val, ok := secret.Data[keyNameInSecret]
				g.Expect(ok).To(BeTrue(), "%s should be present in secret %s", keyNameInSecret, secret.Name)

				g.Expect(val).To(Equal(expectedSecretValue), "%s does not match expected value", keyNameInSecret)
			}, time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	Context("Basic Operations", Label("Platform:AWS"), func() {
		It("should work with namespace-scoped SecretStore", func() {
			secretStoreName := fmt.Sprintf("aws-secret-store-ns-%s", utils.GetRandomString(5))
			awsSecretName := fmt.Sprintf("eso-e2e-secret-ns-%s", utils.GetRandomString(5))
			secretValue := `{"value":"test-namespace-scoped-secret"}`

			defer func() {
				By("Cleaning up AWS secret")
				Expect(utils.DeleteAWSSecret(ctx, clientset, awsSecretName, awsSecretRegionName)).
					NotTo(HaveOccurred())
			}()

			By("Creating AWS secret")
			Expect(utils.CreateAWSSecret(ctx, clientset, awsSecretName, secretValue, awsSecretRegionName)).
				NotTo(HaveOccurred())

			By("Copying AWS credentials to test namespace")
			awsCreds, err := clientset.CoreV1().Secrets("kube-system").Get(ctx, "aws-creds", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			awsCredsSecretName := fmt.Sprintf("aws-creds-ns-%s", utils.GetRandomString(5))
			namespacedCreds := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      awsCredsSecretName,
					Namespace: testNamespace,
				},
				Data: awsCreds.Data,
			}
			_, err = clientset.CoreV1().Secrets(testNamespace).Create(ctx, namespacedCreds, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			defer clientset.CoreV1().Secrets(testNamespace).Delete(ctx, awsCredsSecretName, metav1.DeleteOptions{})

			By("Creating namespace-scoped SecretStore")
			storeAssetFunc := utils.ReplacePatternInAsset(
				"${SECRETSTORE_NAME}", secretStoreName,
				"${SECRETNAME}", awsCredsSecretName,
				"${AWS_REGION}", awsSecretRegionName,
			)
			loader.CreateFromFile(storeAssetFunc, "testdata/aws_secret_store_namespace.yaml", testNamespace)
			defer loader.DeleteFromFile(storeAssetFunc, "testdata/aws_secret_store_namespace.yaml", testNamespace)

			By("Waiting for SecretStore to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: "secretstores",
				},
				testNamespace, secretStoreName, time.Minute,
			)).To(Succeed())

			By("Creating ExternalSecret")
			esAssetFunc := utils.ReplacePatternInAsset(
				"${SECRETSTORE_NAME}", secretStoreName,
				"${SECRETSTORE_KIND}", "SecretStore",
				"${AWS_SECRET_KEY_NAME}", awsSecretName,
			)
			loader.CreateFromFile(esAssetFunc, "testdata/aws_external_secret_datafrom.yaml", testNamespace)
			defer loader.DeleteFromFile(testassets.ReadFile, "testdata/aws_external_secret_datafrom.yaml", testNamespace)

			By("Waiting for ExternalSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: externalSecretsKind,
				},
				testNamespace, "aws-external-secret-datafrom", time.Minute,
			)).To(Succeed())

			By("Verifying target secret contains the data")
			Eventually(func(g Gomega) {
				secret, err := loader.KubeClient.CoreV1().Secrets(testNamespace).Get(ctx, "aws-secret-datafrom", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(secret.Data).To(HaveKey("value"))
				g.Expect(string(secret.Data["value"])).To(Equal("test-namespace-scoped-secret"))
			}, time.Minute, 10*time.Second).Should(Succeed())
		})

		It("should handle binary data correctly", func() {
			awsSecretName := fmt.Sprintf("eso-e2e-secret-binary-%s", utils.GetRandomString(5))
			secretStoreName := fmt.Sprintf("aws-secret-store-%s", utils.GetRandomString(5))
			binaryData := []byte("binary-test-data-12345")
			encodedData := base64.StdEncoding.EncodeToString(binaryData)
			secretValue := fmt.Sprintf(`{"binary_data":"%s"}`, encodedData)

			defer func() {
				By("Cleaning up AWS secret")
				Expect(utils.DeleteAWSSecret(ctx, clientset, awsSecretName, awsSecretRegionName)).
					NotTo(HaveOccurred())
			}()

			By("Creating AWS secret with binary data")
			Expect(utils.CreateAWSSecret(ctx, clientset, awsSecretName, secretValue, awsSecretRegionName)).
				NotTo(HaveOccurred())

			By("Creating ClusterSecretStore")
			cssAssetFunc := utils.ReplacePatternInAsset(
				"${CLUSTERSECRETSTORE_NAME}", secretStoreName,
				"${AWS_REGION}", awsSecretRegionName,
			)
			loader.CreateFromFile(cssAssetFunc, "testdata/aws_secret_store.yaml", testNamespace)
			defer loader.DeleteFromFile(cssAssetFunc, "testdata/aws_secret_store.yaml", testNamespace)

			By("Waiting for ClusterSecretStore to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: clusterSecretStoresKind,
				},
				"", secretStoreName, time.Minute,
			)).To(Succeed())

			By("Creating ExternalSecret for binary data")
			esAssetFunc := utils.ReplacePatternInAsset(
				"${SECRETSTORE_NAME}", secretStoreName,
				"${SECRETSTORE_KIND}", "ClusterSecretStore",
				"${AWS_SECRET_KEY_NAME}", awsSecretName,
			)
			loader.CreateFromFile(esAssetFunc, "testdata/aws_external_secret_binary.yaml", testNamespace)
			defer loader.DeleteFromFile(testassets.ReadFile, "testdata/aws_external_secret_binary.yaml", testNamespace)

			By("Waiting for ExternalSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: externalSecretsKind,
				},
				testNamespace, "aws-external-secret-binary", time.Minute,
			)).To(Succeed())

			By("Verifying binary data is correctly decoded")
			Eventually(func(g Gomega) {
				secret, err := loader.KubeClient.CoreV1().Secrets(testNamespace).Get(ctx, "aws-secret-binary", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(secret.Data).To(HaveKey("binary_data"))
				g.Expect(string(secret.Data["binary_data"])).To(Equal(encodedData))
			}, time.Minute, 10*time.Second).Should(Succeed())
		})
	})

	Context("Advanced Features", Label("Platform:AWS"), func() {
		It("should refresh secret when AWS secret is updated", func() {
			awsSecretName := fmt.Sprintf("eso-e2e-secret-refresh-%s", utils.GetRandomString(5))
			secretStoreName := fmt.Sprintf("aws-secret-store-%s", utils.GetRandomString(5))
			initialValue := `{"value":"initial-value"}`
			updatedValue := `{"value":"updated-value"}`

			defer func() {
				By("Cleaning up AWS secret")
				Expect(utils.DeleteAWSSecret(ctx, clientset, awsSecretName, awsSecretRegionName)).
					NotTo(HaveOccurred())
			}()

			By("Creating AWS secret with initial value")
			Expect(utils.CreateAWSSecret(ctx, clientset, awsSecretName, initialValue, awsSecretRegionName)).
				NotTo(HaveOccurred())

			By("Creating ClusterSecretStore")
			cssAssetFunc := utils.ReplacePatternInAsset(
				"${CLUSTERSECRETSTORE_NAME}", secretStoreName,
				"${AWS_REGION}", awsSecretRegionName,
			)
			loader.CreateFromFile(cssAssetFunc, "testdata/aws_secret_store.yaml", testNamespace)
			defer loader.DeleteFromFile(cssAssetFunc, "testdata/aws_secret_store.yaml", testNamespace)

			By("Waiting for ClusterSecretStore to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: clusterSecretStoresKind,
				},
				"", secretStoreName, time.Minute,
			)).To(Succeed())

			By("Creating ExternalSecret with short refresh interval")
			esAssetFunc := utils.ReplacePatternInAsset(
				"${SECRETSTORE_NAME}", secretStoreName,
				"${SECRETSTORE_KIND}", "ClusterSecretStore",
				"${AWS_SECRET_KEY_NAME}", awsSecretName,
			)
			loader.CreateFromFile(esAssetFunc, "testdata/aws_external_secret_refresh.yaml", testNamespace)
			defer loader.DeleteFromFile(testassets.ReadFile, "testdata/aws_external_secret_refresh.yaml", testNamespace)

			By("Waiting for ExternalSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: externalSecretsKind,
				},
				testNamespace, "aws-external-secret-refresh", time.Minute,
			)).To(Succeed())

			By("Verifying initial value")
			Eventually(func(g Gomega) {
				secret, err := loader.KubeClient.CoreV1().Secrets(testNamespace).Get(ctx, "aws-secret-refresh", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(secret.Data).To(HaveKey("refresh_value"))
				g.Expect(string(secret.Data["refresh_value"])).To(Equal("initial-value"))
			}, time.Minute, 10*time.Second).Should(Succeed())

			By("Updating AWS secret")
			Expect(utils.UpdateAWSSecret(ctx, clientset, awsSecretName, updatedValue, awsSecretRegionName)).
				NotTo(HaveOccurred())

			By("Waiting for secret to be refreshed (30s refresh interval + buffer)")
			Eventually(func(g Gomega) {
				secret, err := loader.KubeClient.CoreV1().Secrets(testNamespace).Get(ctx, "aws-secret-refresh", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(secret.Data).To(HaveKey("refresh_value"))
				g.Expect(string(secret.Data["refresh_value"])).To(Equal("updated-value"))
			}, 2*time.Minute, 10*time.Second).Should(Succeed())
		})

		It("should apply template transformation to secret data", func() {
			awsSecretName := fmt.Sprintf("eso-e2e-secret-template-%s", utils.GetRandomString(5))
			secretStoreName := fmt.Sprintf("aws-secret-store-%s", utils.GetRandomString(5))
			secretValue := `{"username":"testuser","password":"testpass123"}`

			defer func() {
				By("Cleaning up AWS secret")
				Expect(utils.DeleteAWSSecret(ctx, clientset, awsSecretName, awsSecretRegionName)).
					NotTo(HaveOccurred())
			}()

			By("Creating AWS secret")
			Expect(utils.CreateAWSSecret(ctx, clientset, awsSecretName, secretValue, awsSecretRegionName)).
				NotTo(HaveOccurred())

			By("Creating ClusterSecretStore")
			cssAssetFunc := utils.ReplacePatternInAsset(
				"${CLUSTERSECRETSTORE_NAME}", secretStoreName,
				"${AWS_REGION}", awsSecretRegionName,
			)
			loader.CreateFromFile(cssAssetFunc, "testdata/aws_secret_store.yaml", testNamespace)
			defer loader.DeleteFromFile(cssAssetFunc, "testdata/aws_secret_store.yaml", testNamespace)

			By("Waiting for ClusterSecretStore to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: clusterSecretStoresKind,
				},
				"", secretStoreName, time.Minute,
			)).To(Succeed())

			By("Creating ExternalSecret with template")
			esAssetFunc := utils.ReplacePatternInAsset(
				"${SECRETSTORE_NAME}", secretStoreName,
				"${SECRETSTORE_KIND}", "ClusterSecretStore",
				"${AWS_SECRET_KEY_NAME}", awsSecretName,
			)
			loader.CreateFromFile(esAssetFunc, "testdata/aws_external_secret_template.yaml", testNamespace)
			defer loader.DeleteFromFile(testassets.ReadFile, "testdata/aws_external_secret_template.yaml", testNamespace)

			By("Waiting for ExternalSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: externalSecretsKind,
				},
				testNamespace, "aws-external-secret-template", time.Minute,
			)).To(Succeed())

			By("Verifying template transformation applied")
			expectedConfig := "database:\n  username: testuser\n  password: testpass123\n"
			Eventually(func(g Gomega) {
				secret, err := loader.KubeClient.CoreV1().Secrets(testNamespace).Get(ctx, "aws-secret-template", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(secret.Data).To(HaveKey("config.yaml"))
				g.Expect(string(secret.Data["config.yaml"])).To(Equal(expectedConfig))
			}, time.Minute, 10*time.Second).Should(Succeed())
		})

		It("should fetch entire secret using dataFrom", func() {
			awsSecretName := fmt.Sprintf("eso-e2e-secret-datafrom-%s", utils.GetRandomString(5))
			secretStoreName := fmt.Sprintf("aws-secret-store-%s", utils.GetRandomString(5))
			secretValue := `{"api_key":"key123","api_secret":"secret456","endpoint":"https://api.example.com"}`

			defer func() {
				By("Cleaning up AWS secret")
				Expect(utils.DeleteAWSSecret(ctx, clientset, awsSecretName, awsSecretRegionName)).
					NotTo(HaveOccurred())
			}()

			By("Creating AWS secret with multiple fields")
			Expect(utils.CreateAWSSecret(ctx, clientset, awsSecretName, secretValue, awsSecretRegionName)).
				NotTo(HaveOccurred())

			By("Creating ClusterSecretStore")
			cssAssetFunc := utils.ReplacePatternInAsset(
				"${CLUSTERSECRETSTORE_NAME}", secretStoreName,
				"${AWS_REGION}", awsSecretRegionName,
			)
			loader.CreateFromFile(cssAssetFunc, "testdata/aws_secret_store.yaml", testNamespace)
			defer loader.DeleteFromFile(cssAssetFunc, "testdata/aws_secret_store.yaml", testNamespace)

			By("Waiting for ClusterSecretStore to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: clusterSecretStoresKind,
				},
				"", secretStoreName, time.Minute,
			)).To(Succeed())

			By("Creating ExternalSecret with dataFrom")
			esAssetFunc := utils.ReplacePatternInAsset(
				"${SECRETSTORE_NAME}", secretStoreName,
				"${SECRETSTORE_KIND}", "ClusterSecretStore",
				"${AWS_SECRET_KEY_NAME}", awsSecretName,
			)
			loader.CreateFromFile(esAssetFunc, "testdata/aws_external_secret_datafrom.yaml", testNamespace)
			defer loader.DeleteFromFile(testassets.ReadFile, "testdata/aws_external_secret_datafrom.yaml", testNamespace)

			By("Waiting for ExternalSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: externalSecretsKind,
				},
				testNamespace, "aws-external-secret-datafrom", time.Minute,
			)).To(Succeed())

			By("Verifying all keys imported without explicit mapping")
			Eventually(func(g Gomega) {
				secret, err := loader.KubeClient.CoreV1().Secrets(testNamespace).Get(ctx, "aws-secret-datafrom", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(secret.Data).To(HaveKey("api_key"))
				g.Expect(secret.Data).To(HaveKey("api_secret"))
				g.Expect(secret.Data).To(HaveKey("endpoint"))
				g.Expect(string(secret.Data["api_key"])).To(Equal("key123"))
				g.Expect(string(secret.Data["api_secret"])).To(Equal("secret456"))
				g.Expect(string(secret.Data["endpoint"])).To(Equal("https://api.example.com"))
			}, time.Minute, 10*time.Second).Should(Succeed())
		})

		It("should extract nested JSON values using property path", func() {
			awsSecretName := fmt.Sprintf("eso-e2e-secret-json-%s", utils.GetRandomString(5))
			secretStoreName := fmt.Sprintf("aws-secret-store-%s", utils.GetRandomString(5))
			nestedSecret := map[string]interface{}{
				"db": map[string]interface{}{
					"credentials": map[string]interface{}{
						"password": "nested-password-123",
					},
				},
			}
			secretBytes, _ := json.Marshal(nestedSecret)
			secretValue := string(secretBytes)

			defer func() {
				By("Cleaning up AWS secret")
				Expect(utils.DeleteAWSSecret(ctx, clientset, awsSecretName, awsSecretRegionName)).
					NotTo(HaveOccurred())
			}()

			By("Creating AWS secret with nested JSON")
			Expect(utils.CreateAWSSecret(ctx, clientset, awsSecretName, secretValue, awsSecretRegionName)).
				NotTo(HaveOccurred())

			By("Creating ClusterSecretStore")
			cssAssetFunc := utils.ReplacePatternInAsset(
				"${CLUSTERSECRETSTORE_NAME}", secretStoreName,
				"${AWS_REGION}", awsSecretRegionName,
			)
			loader.CreateFromFile(cssAssetFunc, "testdata/aws_secret_store.yaml", testNamespace)
			defer loader.DeleteFromFile(cssAssetFunc, "testdata/aws_secret_store.yaml", testNamespace)

			By("Waiting for ClusterSecretStore to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: clusterSecretStoresKind,
				},
				"", secretStoreName, time.Minute,
			)).To(Succeed())

			By("Creating ExternalSecret with JSON path extraction")
			esAssetFunc := utils.ReplacePatternInAsset(
				"${SECRETSTORE_NAME}", secretStoreName,
				"${SECRETSTORE_KIND}", "ClusterSecretStore",
				"${AWS_SECRET_KEY_NAME}", awsSecretName,
			)
			loader.CreateFromFile(esAssetFunc, "testdata/aws_external_secret_jsonpath.yaml", testNamespace)
			defer loader.DeleteFromFile(testassets.ReadFile, "testdata/aws_external_secret_jsonpath.yaml", testNamespace)

			By("Waiting for ExternalSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: externalSecretsKind,
				},
				testNamespace, "aws-external-secret-jsonpath", time.Minute,
			)).To(Succeed())

			By("Verifying nested value extracted correctly")
			Eventually(func(g Gomega) {
				secret, err := loader.KubeClient.CoreV1().Secrets(testNamespace).Get(ctx, "aws-secret-jsonpath", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(secret.Data).To(HaveKey("nested_value"))
				g.Expect(string(secret.Data["nested_value"])).To(Equal("nested-password-123"))
			}, time.Minute, 10*time.Second).Should(Succeed())
		})
	})

})
