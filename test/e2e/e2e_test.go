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
	operatorNamespace       = "external-secrets-operator"
	operandNamespace        = "external-secrets"
	secretStoreFile         = "testdata/aws_secret_store.yaml"
	externalSecretFile      = "testdata/aws_external_secret.yaml"
	pushSecretFile          = "testdata/push_secret.yaml"
	externalSecrets         = "testdata/external_secret.yaml"
	expectedSecretValueFile = "testdata/expected_value.yaml"
	awsSecretToPushFile     = "testdata/aws_k8s_push_secret.yaml"
	awsSecretRegionName     = "ap-south-1"
)

var _ = Describe("External Secrets Operator End-to-End test scenarios", Ordered, func() {
	ctx := context.TODO()
	var (
		clientset     *kubernetes.Clientset
		dynamicClient *dynamic.DynamicClient
		loader        utils.DynamicResourceLoader
		awsSecretName string
	)

	BeforeAll(func() {
		var err error
		loader = utils.NewDynamicResourceLoader(ctx, &testing.T{})

		clientset, err = kubernetes.NewForConfig(cfg)
		Expect(err).Should(BeNil())

		dynamicClient, err = dynamic.NewForConfig(cfg)
		Expect(err).Should(BeNil())

		awsSecretName = fmt.Sprintf("eso-e2e-secret-%s", utils.GetRandomString(5))

		By("Waiting for external-secrets-operator controller-manager pod to be ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespace, []string{
			"external-secrets-operator-controller-manager-",
		})).To(Succeed())

		By("Creating the externalsecretsconfig.operator.openshift.io/cluster CR")
		loader.CreateFromFile(testassets.ReadFile, externalSecrets, operatorNamespace)
	})

	AfterAll(func() {
		By("Deleting the externalsecretsconfig.operator.openshift.io/cluster CR")
		loader.DeleteFromFile(testassets.ReadFile, externalSecrets, operatorNamespace)

		err := utils.DeleteAWSSecret(ctx, clientset, awsSecretName, awsSecretRegionName)
		Expect(err).NotTo(HaveOccurred(), "failed to delete AWS secret test/e2e")
	})

	BeforeEach(func() {
		By("Verifying ESO pods are running and ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
			"external-secrets-",
			"external-secrets-cert-controller-",
			"external-secrets-webhook-",
		})).To(Succeed())
	})

	It("should create secrets mentioned in ExternalSecret using the referenced SecretStore", func() {
		expectedSecretValue, err := utils.ReadExpectedSecretValue(expectedSecretValueFile)
		Expect(err).To(Succeed())

		By("Creating kubernetes secret to be used in PushSecret")
		secretsAssetFunc := utils.ReplacePatternInAsset("${SECRET_VALUE}", base64.StdEncoding.EncodeToString(expectedSecretValue))
		loader.CreateFromFile(secretsAssetFunc, awsSecretToPushFile, operandNamespace)
		defer loader.DeleteFromFile(testassets.ReadFile, awsSecretToPushFile, operandNamespace)

		By("Creating SecretStore")
		loader.CreateFromFile(testassets.ReadFile, secretStoreFile, operandNamespace)
		defer loader.DeleteFromFile(testassets.ReadFile, secretStoreFile, operandNamespace)

		By("Waiting for SecretStore to become Ready")
		Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
			schema.GroupVersionResource{
				Group:    "external-secrets.io",
				Version:  "v1",
				Resource: "clustersecretstores",
			},
			"", "aws-secret-store", time.Minute,
		)).To(Succeed())

		By("Creating PushSecret")
		assetFunc := utils.ReplacePatternInAsset("${AWS_SECRET_KEY_NAME}", awsSecretName)
		loader.CreateFromFile(assetFunc, pushSecretFile, operandNamespace)
		defer loader.DeleteFromFile(testassets.ReadFile, pushSecretFile, operandNamespace)

		By("Waiting for PushSecret to become Ready")
		Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
			schema.GroupVersionResource{
				Group:    "external-secrets.io",
				Version:  "v1alpha1",
				Resource: "pushsecrets",
			},
			operandNamespace, "aws-push-secret", time.Minute,
		)).To(Succeed())

		By("Creating ExternalSecret")
		loader.CreateFromFile(assetFunc, externalSecretFile, operandNamespace)
		defer loader.DeleteFromFile(testassets.ReadFile, externalSecretFile, operandNamespace)

		By("Waiting for ExternalSecret to become Ready")
		Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
			schema.GroupVersionResource{
				Group:    "external-secrets.io",
				Version:  "v1",
				Resource: "externalsecrets",
			},
			operandNamespace, "aws-external-secret", time.Minute,
		)).To(Succeed())

		By("Waiting for target secret to be created with expected data")
		Eventually(func(g Gomega) {
			secret, err := loader.KubeClient.CoreV1().Secrets(operandNamespace).Get(ctx, "aws-secret", metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred(), "should get aws-secret from namespace %s", operandNamespace)

			val, ok := secret.Data["aws_secret_access_key"]
			g.Expect(ok).To(BeTrue(), "aws_secret_access_key should be present in secret %s", secret.Name)

			g.Expect(val).To(Equal(expectedSecretValue), "aws_secret_access_key does not match expected value")
		}, time.Minute, 10*time.Second).Should(Succeed())
	})
})
