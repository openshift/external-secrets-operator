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
	externalSecretsFile                 = "testdata/external_secret.yaml"
	externalSecretsFileWithRevisionLimit = "testdata/external_secret_with_revision_limits.yaml"
	expectedSecretValueFile             = "testdata/expected_value.yaml"
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
			awsSecretRegionName              = "ap-south-1"
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
			cssAssetFunc := utils.ReplacePatternInAsset(awsClusterSecretStoreNamePattern, clusterSecretStoreResourceName)
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

	Context("Deployment Revision History Limit", func() {
		It("should use default revisionHistoryLimit when not configured", func() {
			By("Verifying default revisionHistoryLimit (10) for ExternalSecretsCoreController deployment")
			Eventually(func(g Gomega) {
				deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred(), "should get external-secrets deployment")
				g.Expect(deployment.Spec.RevisionHistoryLimit).NotTo(BeNil(), "revisionHistoryLimit should be set")
				g.Expect(*deployment.Spec.RevisionHistoryLimit).To(Equal(int32(10)), "revisionHistoryLimit should default to 10 when not configured")
			}, time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying default revisionHistoryLimit (10) for Webhook deployment")
			Eventually(func(g Gomega) {
				deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred(), "should get external-secrets-webhook deployment")
				g.Expect(deployment.Spec.RevisionHistoryLimit).NotTo(BeNil(), "revisionHistoryLimit should be set")
				g.Expect(*deployment.Spec.RevisionHistoryLimit).To(Equal(int32(10)), "revisionHistoryLimit should default to 10 when not configured")
			}, time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying default revisionHistoryLimit (10) for CertController deployment")
			Eventually(func(g Gomega) {
				deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-cert-controller", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred(), "should get external-secrets-cert-controller deployment")
				g.Expect(deployment.Spec.RevisionHistoryLimit).NotTo(BeNil(), "revisionHistoryLimit should be set")
				g.Expect(*deployment.Spec.RevisionHistoryLimit).To(Equal(int32(10)), "revisionHistoryLimit should default to 10 when not configured")
			}, time.Minute, 5*time.Second).Should(Succeed())
		})

		It("should set custom revisionHistoryLimit for all component deployments", func() {
			const (
				controllerLimit     = int32(3)
				webhookLimit        = int32(5)
				certControllerLimit = int32(2)
			)

			By("Updating the ExternalSecretsConfig with custom revision history limits")
			loader.DeleteFromFile(testassets.ReadFile, externalSecretsFile, "")
			loader.CreateFromFile(testassets.ReadFile, externalSecretsFileWithRevisionLimit, "")
			defer func() {
				loader.DeleteFromFile(testassets.ReadFile, externalSecretsFileWithRevisionLimit, "")
				loader.CreateFromFile(testassets.ReadFile, externalSecretsFile, "")
			}()

			By("Waiting for pods to be ready after config update")
			Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
				operandCoreControllerPodPrefix,
				operandCertControllerPodPrefix,
				operandWebhookPodPrefix,
			})).To(Succeed())

			By("Verifying custom revisionHistoryLimit (3) for ExternalSecretsCoreController deployment")
			Eventually(func(g Gomega) {
				deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred(), "should get external-secrets deployment")
				g.Expect(deployment.Spec.RevisionHistoryLimit).NotTo(BeNil(), "revisionHistoryLimit should be set")
				g.Expect(*deployment.Spec.RevisionHistoryLimit).To(Equal(controllerLimit), "revisionHistoryLimit should be %d for controller", controllerLimit)
			}, time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying custom revisionHistoryLimit (5) for Webhook deployment")
			Eventually(func(g Gomega) {
				deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-webhook", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred(), "should get external-secrets-webhook deployment")
				g.Expect(deployment.Spec.RevisionHistoryLimit).NotTo(BeNil(), "revisionHistoryLimit should be set")
				g.Expect(*deployment.Spec.RevisionHistoryLimit).To(Equal(webhookLimit), "revisionHistoryLimit should be %d for webhook", webhookLimit)
			}, time.Minute, 5*time.Second).Should(Succeed())

			By("Verifying custom revisionHistoryLimit (2) for CertController deployment")
			Eventually(func(g Gomega) {
				deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, "external-secrets-cert-controller", metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred(), "should get external-secrets-cert-controller deployment")
				g.Expect(deployment.Spec.RevisionHistoryLimit).NotTo(BeNil(), "revisionHistoryLimit should be set")
				g.Expect(*deployment.Spec.RevisionHistoryLimit).To(Equal(certControllerLimit), "revisionHistoryLimit should be %d for cert-controller", certControllerLimit)
			}, time.Minute, 5*time.Second).Should(Succeed())
		})
	})
})
