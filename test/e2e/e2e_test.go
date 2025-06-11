package e2e

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	utils "github.com/openshift/external-secrets-operator/test/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"testing"
	"time"
)

const (
	namespace          = "external-secrets-operator"
	secretStoreFile    = "testdata/aws_secret_store.yaml"
	externalSecretFile = "testdata/aws_external_secret.yaml"
)

var _ = Describe("External Secrets Operator", Ordered, func() {

	var (
		ctx    = context.TODO()
		loader utils.DynamicResourceLoader
	)

	BeforeAll(func() {
		loader = utils.NewDynamicResourceLoader(ctx, &testing.T{})

		// Create ExternalSecret resource
		loader.CreateFromFile(loadFromFile, "testdata/external_secret.yaml", namespace)
	})

	AfterAll(func() {
		loader.DeleteFromFile(loadFromFile, "testdata/external_secret.yaml", namespace)
	})

	Context("Operator", func() {
		It("should have controller pod running", func() {
			verifyControllerPod := func() error {
				pods, err := loader.KubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
					LabelSelector: "control-plane=controller-manager",
				})
				ExpectWithOffset(2, err).NotTo(HaveOccurred())

				var runningPods []corev1.Pod
				for _, pod := range pods.Items {
					if pod.DeletionTimestamp == nil && pod.Status.Phase == corev1.PodRunning {
						runningPods = append(runningPods, pod)
					}
				}

				if len(runningPods) != 1 {
					return fmt.Errorf("expected 1 running controller pod, got %d", len(runningPods))
				}

				ExpectWithOffset(2, runningPods[0].Name).To(ContainSubstring("controller-manager"))
				fmt.Println(runningPods[0].Name)
				return nil
			}

			EventuallyWithOffset(1, verifyControllerPod, time.Minute, time.Second).Should(Succeed())
		})
	})

	Context("AWS SecretStore", func() {
		BeforeEach(func() {
			loader.CreateFromFile(loadFromFile, secretStoreFile, namespace)
			loader.CreateFromFile(loadFromFile, externalSecretFile, namespace)

		})

		AfterEach(func() {
			// Clean up SecretStore
			loader.DeleteFromFile(loadFromFile, secretStoreFile, namespace)
			// Clean up ExternalStore
			loader.DeleteFromFile(loadFromFile, externalSecretFile, namespace)

		})

		It("should synchronize secrets from AWS Secrets Manager", func() {
			By("verifying the synchronization of the secret")
			Eventually(func() error {
				k8sSecret, err := loader.KubeClient.CoreV1().Secrets(namespace).Get(ctx, "aws-secret", metav1.GetOptions{})

				secretsList, err := loader.KubeClient.CoreV1().Secrets("kube-system").List(ctx, metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())

				fmt.Println("Secrets in kube-system:")
				for _, s := range secretsList.Items {
					fmt.Println("-", s.Name)
				}
				if err != nil {
					return fmt.Errorf("failed to get secret: %v", err)
				}

				if string(k8sSecret.Data["aws_secret_access_key"]) == "" {
					return fmt.Errorf("secret data is empty")
				}

				decodedValue, err := os.ReadFile("testdata/expected_value.yaml")
				if err != nil {
					return fmt.Errorf("failed to read expected secret value: %v", err)
				}

				if string(k8sSecret.Data["aws_secret_access_key"]) != string(decodedValue) {
					return fmt.Errorf("secret value does not match expected")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})

func loadFromFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}
