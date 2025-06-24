package e2e

import (
	"context"
	"embed"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	utils "github.com/openshift/external-secrets-operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"testing"
	"time"
)

//go:embed testdata/*
var testassets embed.FS

const (
	operatorNamespace  = "external-secrets-operator"
	operandNamespace   = "external-secrets"
	secretStoreFile    = "testdata/aws_secret_store.yaml"
	externalSecretFile = "testdata/aws_external_secret.yaml"
	pushSecretFile     = "testdata/push_secret.yaml"
	externalSecrets    = "testdata/external_secret.yaml"
)

var _ = Describe("External Secrets Operator End-to-End", Ordered, func() {
	ctx := context.TODO()
	var (
		clientset     *kubernetes.Clientset
		dynamicClient *dynamic.DynamicClient
		loader        utils.DynamicResourceLoader
	)

	BeforeAll(func() {
		var err error
		loader = utils.NewDynamicResourceLoader(ctx, &testing.T{})

		clientset, err = kubernetes.NewForConfig(cfg)
		Expect(err).Should(BeNil())

		dynamicClient, err = dynamic.NewForConfig(cfg)
		Expect(err).Should(BeNil())

		By("Waiting for external-secrets-operator controller-manager pod to be ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operatorNamespace, []string{
			"external-secrets-operator-controller-manager-",
		})).To(Succeed())

		By("Creating the ExternalSecrets Operator CR")
		loader.CreateFromFile(testassets.ReadFile, externalSecrets, operatorNamespace)
	})

	AfterAll(func() {
		By("Deleting the ExternalSecrets Operator CR")
		loader.DeleteFromFile(testassets.ReadFile, externalSecrets, operatorNamespace)

		err := utils.DeleteAWSSecret("test/e2e", "eu-north-1")
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

	It("should create secrets from SecretStore and ExternalSecret", func() {
		By("Creating SecretStore")
		loader.CreateFromFile(testassets.ReadFile, secretStoreFile, operandNamespace)
		defer loader.DeleteFromFile(testassets.ReadFile, secretStoreFile, operandNamespace)

		By("Waiting for SecretStore to become Ready")
		Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
			schema.GroupVersionResource{
				Group:    "external-secrets.io",
				Version:  "v1beta1",
				Resource: "clustersecretstores",
			},
			"", "aws-secret-store", time.Minute,
		)).To(Succeed())

		By("Creating PushSecret")
		loader.CreateFromFile(testassets.ReadFile, pushSecretFile, operandNamespace)
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
		loader.CreateFromFile(testassets.ReadFile, externalSecretFile, operandNamespace)
		defer loader.DeleteFromFile(testassets.ReadFile, externalSecretFile, operandNamespace)

		By("Waiting for ExternalSecret to become Ready")
		Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
			schema.GroupVersionResource{
				Group:    "external-secrets.io",
				Version:  "v1beta1",
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

			expectedValue := []byte("hqTTSYkFYgkw3OfQ9lFvQgtsReb1g1a+Po5Y/HNU")
			g.Expect(val).To(Equal(expectedValue), "aws_secret_access_key does not match expected value")
		}, time.Minute, 5*time.Second).Should(Succeed())

	})
})
