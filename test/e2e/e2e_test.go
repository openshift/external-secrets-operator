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
	"maps"
	"os/exec"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/test/utils"
)

//go:embed testdata/*
var testassets embed.FS

const (
	// test bindata
	externalSecretsFile                  = "testdata/external_secret.yaml"
	externalSecretsFileWithRevisionLimit = "testdata/external_secret_with_revision_limits.yaml"
	expectedSecretValueFile              = "testdata/expected_value.yaml"
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
	vaultNamespace                 = "vault-test"
	vaultManifestFile              = "testdata/vault/vault.yaml"
	vaultServiceName               = "vault"
	vaultNetworkPolicyFile         = "testdata/vault/vault-networkpolicy.yaml"
	vaultAddr                      = "http://vault.vault-test.svc.cluster.local:8200"
	targetSecretName               = "k8s-secret-to-create" //must match with external_secret.yaml target.name
)

const (
	externalSecretsGroupName = "external-secrets.io"
	v1APIVersion             = "v1"
	v1alpha1APIVersion       = "v1alpha1"
	clusterSecretStoresKind  = "clustersecretstores"
	secretStoresKind         = "secretstores"
	PushSecretsKind          = "pushsecrets"
	externalSecretsKind      = "externalsecrets"
)

var _ = Describe("External Secrets Operator End-to-End test scenarios", Ordered, func() {
	ctx := context.Background()
	var (
		clientset     *kubernetes.Clientset
		dynamicClient *dynamic.DynamicClient
		runtimeClient client.Client
		loader        utils.DynamicResourceLoader
		awsSecretName string
		testNamespace string
	)

	BeforeAll(func() {
		var err error
		loader = utils.NewDynamicResourceLoader(ctx, &testing.T{})

		clientset = suiteClientset
		dynamicClient = suiteDynamicClient
		runtimeClient = suiteRuntimeClient

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

		esc := &operatorv1alpha1.ExternalSecretsConfig{}
		if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
			if k8serrors.IsNotFound(err) {
				By("Creating the externalsecrets.openshift.operator.io/cluster CR")
				loader.CreateFromFile(testassets.ReadFile, externalSecretsFile, "")
			} else {
				Expect(err).NotTo(HaveOccurred(), "failed to get cluster ExternalSecretsConfig")
			}
		}

		By("Waiting for ExternalSecretsConfig to be Ready (with Degraded=False)")
		Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, "cluster", 2*time.Minute)).To(Succeed(),
			"ExternalSecretsConfig should have Ready=True and Degraded=False conditions")
	})

	BeforeEach(func() {
		By("Verifying external-secrets operand pods are ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
			operandCoreControllerPodPrefix,
			operandCertControllerPodPrefix,
			operandWebhookPodPrefix,
		})).To(Succeed())
	})

	AfterEach(func() {
		if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
			return
		}
		artifactDir := getTestDir()
		By(fmt.Sprintf("Test failed: dumping logs and resources to %s/e2e-artifacts/", artifactDir))
		if err := utils.DumpE2EArtifacts(ctx, clientset, dynamicClient, operatorNamespace, operandNamespace, testNamespace, artifactDir); err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "warning: failed to dump e2e artifacts: %v\n", err)
		}
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

	Context("Cross-platform: GCP cluster and AWS Secrets Manager", Label("CrossPlatform:GCP-AWS"), func() {
		const (
			externalSecretFile               = "testdata/aws_external_secret.yaml"
			pushSecretFile                   = "testdata/push_secret.yaml"
			awsSecretToPushFile              = "testdata/aws_k8s_push_secret.yaml"
			awsSecretNamePattern             = "${AWS_SECRET_KEY_NAME}"
			awsSecretValuePattern            = "${SECRET_VALUE}"
			awsClusterSecretStoreNamePattern = "${CLUSTERSECRETSTORE_NAME}"
			awsSecretRegionName              = "ap-south-1"
		)
		var crossPlatformAWSSecretName string

		AfterAll(func() {
			if crossPlatformAWSSecretName != "" {
				By("Deleting the AWS secret")
				Expect(utils.DeleteAWSSecretFromCredsSecret(ctx, clientset, utils.AWSCredSecretName, utils.AWSCredNamespace, crossPlatformAWSSecretName, awsSecretRegionName)).
					NotTo(HaveOccurred(), "failed to delete AWS secret (cross-platform e2e)")
			}
		})

		It("should create secrets using ClusterSecretStore with AWS credentials secret in fixed namespace", func() {
			var (
				clusterSecretStoreResourceName = fmt.Sprintf("aws-secret-store-cross-%s", utils.GetRandomString(5))
				pushSecretResourceName         = "aws-push-secret"
				externalSecretResourceName     = "aws-external-secret"
				secretResourceName             = "aws-secret"
				keyNameInSecret                = "aws_secret_access_key"
			)

			crossPlatformAWSSecretName = fmt.Sprintf("e2e-cross-platform-%s", utils.GetRandomString(8))
			defer func() {
				if crossPlatformAWSSecretName != "" {
					_ = utils.DeleteAWSSecretFromCredsSecret(ctx, clientset, utils.AWSCredSecretName, utils.AWSCredNamespace, crossPlatformAWSSecretName, awsSecretRegionName)
				}
			}()

			expectedSecretValue, err := utils.ReadExpectedSecretValue(expectedSecretValueFile)
			Expect(err).To(Succeed())

			By("Creating kubernetes secret to be used in PushSecret")
			secretsAssetFunc := utils.ReplacePatternInAsset(awsSecretValuePattern, base64.StdEncoding.EncodeToString(expectedSecretValue))
			loader.CreateFromFile(secretsAssetFunc, awsSecretToPushFile, testNamespace)
			defer loader.DeleteFromFile(testassets.ReadFile, awsSecretToPushFile, testNamespace)

			By("Creating ClusterSecretStore (AWS) from API")
			cssObj := utils.AWSClusterSecretStore(clusterSecretStoreResourceName, awsSecretRegionName)
			loader.CreateFromUnstructured(cssObj, "")
			defer loader.DeleteFromUnstructured(cssObj, "")

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
			assetFunc := utils.ReplacePatternInAsset(awsSecretNamePattern, crossPlatformAWSSecretName,
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

	Context("Environment Variables", func() {
		// Map component names to deployment names and target container names
		componentToDeployment := map[string]string{
			"ExternalSecretsCoreController": "external-secrets",
			"Webhook":                       "external-secrets-webhook",
			"CertController":                "external-secrets-cert-controller",
		}
		componentToContainer := map[string]string{
			"ExternalSecretsCoreController": "external-secrets",
			"Webhook":                       "webhook",
			"CertController":                "cert-controller",
		}

		// Define test env vars
		envConfigs := []operatorv1alpha1.ComponentConfig{
			{
				ComponentName: "ExternalSecretsCoreController",
				OverrideEnv: []corev1.EnvVar{
					{Name: "LOG_LEVEL", Value: "debug"},
					{Name: "TEST_CONTROLLER_VAR", Value: "controller-value"},
				},
			},
			{
				ComponentName: "Webhook",
				OverrideEnv: []corev1.EnvVar{
					{Name: "TLS_MIN_VERSION", Value: "1.2"},
					{Name: "TEST_WEBHOOK_VAR", Value: "webhook-value"},
				},
			},
			{
				ComponentName: "CertController",
				OverrideEnv: []corev1.EnvVar{
					{Name: "TEST_CERT_VAR", Value: "cert-value"},
					{Name: "FOO", Value: "bar"},
				},
			},
		}

		It("should set custom environment variables for all component deployments", func() {
			By("Updating ExternalSecretsConfig with custom env vars")
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				existingCR := &operatorv1alpha1.ExternalSecretsConfig{}
				if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, existingCR); err != nil {
					return err
				}

				updatedCR := existingCR.DeepCopy()
				updatedCR.Spec.ControllerConfig = operatorv1alpha1.ControllerConfig{
					ComponentConfigs: envConfigs,
				}

				return runtimeClient.Update(ctx, updatedCR)
			})
			Expect(err).NotTo(HaveOccurred(), "should update ExternalSecretsConfig with custom env vars")

			By("Waiting for pods to be ready after config update")
			Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
				operandCoreControllerPodPrefix,
				operandCertControllerPodPrefix,
				operandWebhookPodPrefix,
			})).To(Succeed())

			for _, config := range envConfigs {
				By(fmt.Sprintf("Verifying custom environment variables in %s deployment", config.ComponentName))

				deploymentName := componentToDeployment[string(config.ComponentName)]
				targetContainerName := componentToContainer[string(config.ComponentName)]
				Eventually(func(g Gomega) {
					deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, deploymentName, metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred(), "should get %s deployment", deploymentName)

					// Verify env vars on the target container specifically
					for _, container := range deployment.Spec.Template.Spec.Containers {
						if container.Name != targetContainerName {
							continue
						}
						envMap := make(map[string]string)
						for _, env := range container.Env {
							envMap[env.Name] = env.Value
						}
						for _, expectedEnv := range config.OverrideEnv {
							g.Expect(envMap).To(HaveKeyWithValue(expectedEnv.Name, expectedEnv.Value),
								"container %s in %s should have env var %s=%s", targetContainerName, deploymentName, expectedEnv.Name, expectedEnv.Value)
						}
					}
				}, time.Minute, 5*time.Second).Should(Succeed(), "env vars should be set for %s", config.ComponentName)
			}
		})

		It("should remove custom environment variables when config is cleared", func() {
			By("Removing custom env vars from ExternalSecretsConfig")
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				existingCR := &operatorv1alpha1.ExternalSecretsConfig{}
				if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, existingCR); err != nil {
					return err
				}

				updatedCR := existingCR.DeepCopy()
				updatedCR.Spec.ControllerConfig = operatorv1alpha1.ControllerConfig{
					ComponentConfigs: nil,
				}

				return runtimeClient.Update(ctx, updatedCR)
			})
			Expect(err).NotTo(HaveOccurred(), "should update ExternalSecretsConfig to remove custom env vars")

			By("Waiting for pods to be ready after config update")
			Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
				operandCoreControllerPodPrefix,
				operandCertControllerPodPrefix,
				operandWebhookPodPrefix,
			})).To(Succeed())

			for _, config := range envConfigs {
				By(fmt.Sprintf("Verifying custom environment variables removed from %s deployment", config.ComponentName))

				deploymentName := componentToDeployment[string(config.ComponentName)]
				targetContainerName := componentToContainer[string(config.ComponentName)]
				Eventually(func(g Gomega) {
					deployment, err := clientset.AppsV1().Deployments(operandNamespace).Get(ctx, deploymentName, metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred(), "should get %s deployment", deploymentName)

					// Verify env vars are removed from the target container
					for _, container := range deployment.Spec.Template.Spec.Containers {
						if container.Name != targetContainerName {
							continue
						}
						envNames := make(map[string]bool)
						for _, env := range container.Env {
							envNames[env.Name] = true
						}
						for _, expectedEnv := range config.OverrideEnv {
							g.Expect(envNames).NotTo(HaveKey(expectedEnv.Name),
								"container %s in %s should not have env var %s after removal", targetContainerName, deploymentName, expectedEnv.Name)
						}
					}
				}, time.Minute, 5*time.Second).Should(Succeed(), "env vars should be removed from %s", config.ComponentName)
			}
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

	Context("Annotations", func() {
		It("should apply and remove custom annotations to created resources", func() {
			// Define test annotations
			testAnnotations := map[string]string{
				"example.com/custom-annotation": "test-value",
				"mycompany.io/owner":            "platform-team",
			}

			// Capture original annotations so we can restore them and avoid test pollution
			existingCR := &operatorv1alpha1.ExternalSecretsConfig{}
			Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, existingCR)).To(Succeed(), "should get ExternalSecretsConfig to capture initial state")
			originalAnnotations := maps.Clone(existingCR.Spec.ControllerConfig.Annotations)

			defer func() {
				By("Restoring original annotations on ExternalSecretsConfig CR")
				err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					currentCR := &operatorv1alpha1.ExternalSecretsConfig{}
					if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, currentCR); err != nil {
						return err
					}
					currentCR.Spec.ControllerConfig.Annotations = originalAnnotations
					return runtimeClient.Update(ctx, currentCR)
				})
				Expect(err).NotTo(HaveOccurred(), "should restore annotations on ExternalSecretsConfig")
			}()

			By("Updating ExternalSecretsConfig with custom annotations")
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				existingCR := &operatorv1alpha1.ExternalSecretsConfig{}
				if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, existingCR); err != nil {
					return err
				}

				updatedCR := existingCR.DeepCopy()
				merged := make(map[string]string)
				if originalAnnotations != nil {
					maps.Copy(merged, originalAnnotations)
				}
				maps.Copy(merged, testAnnotations)
				updatedCR.Spec.ControllerConfig.Annotations = merged

				return runtimeClient.Update(ctx, updatedCR)
			})
			Expect(err).NotTo(HaveOccurred(), "should update ExternalSecretsConfig with annotations")

			By("Waiting for external-secrets operand pods to be ready")
			Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
				operandCoreControllerPodPrefix,
				operandCertControllerPodPrefix,
				operandWebhookPodPrefix,
			})).To(Succeed())

			// Verify annotations are applied to each resource type
			for _, resourceType := range getResourceTypesToVerify() {
				By(fmt.Sprintf("Verifying annotations are applied to %s resources", resourceType.name))
				Eventually(func(g Gomega) {
					objects, err := resourceType.listFunc(ctx, clientset, operandNamespace, g)
					g.Expect(err).NotTo(HaveOccurred(), "should list %s in %s namespace", resourceType.name, operandNamespace)

					for _, obj := range objects {
						if !strings.HasPrefix(obj.GetName(), "external-secrets") {
							continue
						}

						annotations := obj.GetAnnotations()
						for key, value := range testAnnotations {
							g.Expect(annotations).To(HaveKeyWithValue(key, value),
								"%s %s should have annotation %s=%s", resourceType.name, obj.GetName(), key, value)
						}

						if resourceType.checkPodSpec {
							deployment := asDeployment(obj)
							templateAnnotations := deployment.Spec.Template.Annotations
							for key, value := range testAnnotations {
								g.Expect(templateAnnotations).To(HaveKeyWithValue(key, value),
									"deployment %s pod template should have annotation %s=%s", deployment.Name, key, value)
							}
						}
					}
				}, 2*time.Minute, 5*time.Second).Should(Succeed())
			}

			By("Removing test annotations from ExternalSecretsConfig CR")
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				currentCR := &operatorv1alpha1.ExternalSecretsConfig{}
				if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, currentCR); err != nil {
					return err
				}
				for key := range testAnnotations {
					delete(currentCR.Spec.ControllerConfig.Annotations, key)
				}
				return runtimeClient.Update(ctx, currentCR)
			})
			Expect(err).NotTo(HaveOccurred(), "should remove test annotations from ExternalSecretsConfig")

			// Verify annotations are removed from each resource type
			for _, resourceType := range getResourceTypesToVerify() {
				By(fmt.Sprintf("Verifying annotations are removed from %s resources", resourceType.name))
				Eventually(func(g Gomega) {
					objects, err := resourceType.listFunc(ctx, clientset, operandNamespace, g)
					g.Expect(err).NotTo(HaveOccurred(), "should list %s in %s namespace", resourceType.name, operandNamespace)

					for _, obj := range objects {
						if !strings.HasPrefix(obj.GetName(), "external-secrets") {
							continue
						}

						annotations := obj.GetAnnotations()
						for key := range testAnnotations {
							g.Expect(annotations).NotTo(HaveKey(key),
								"%s %s should NOT have annotation %s after removal", resourceType.name, obj.GetName(), key)
						}

						if resourceType.checkPodSpec {
							deployment := asDeployment(obj)
							templateAnnotations := deployment.Spec.Template.Annotations
							for key := range testAnnotations {
								g.Expect(templateAnnotations).NotTo(HaveKey(key),
									"deployment %s pod template should NOT have annotation %s after removal", deployment.Name, key)
							}
						}
					}
				}, 2*time.Minute, 5*time.Second).Should(Succeed())
			}
		})

		It("should not allow updating annotations with reserved domain prefix", func() {
			By("Getting the existing ExternalSecretsConfig CR")
			existingCR := &operatorv1alpha1.ExternalSecretsConfig{}
			err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, existingCR)
			Expect(err).NotTo(HaveOccurred(), "should get ExternalSecretsConfig CR")

			By("Attempting to update with a reserved domain annotation")
			updatedCR := existingCR.DeepCopy()
			if updatedCR.Spec.ControllerConfig.Annotations == nil {
				updatedCR.Spec.ControllerConfig.Annotations = make(map[string]string)
			}

			// Add two reserved annotations that should be rejected
			updatedCR.Spec.ControllerConfig.Annotations["deployment.kubernetes.io/revision"] = "9"
			updatedCR.Spec.ControllerConfig.Annotations["k8s.io/not-allowed"] = "denied"

			err = runtimeClient.Update(ctx, updatedCR)
			Expect(err).To(HaveOccurred(), "update with reserved domain annotations should fail")
		})
	})

	Context("Vault Secret Manager", Label("Platform:Vault"), func() {
		const (
			vaultSecretName  = "foo"
			vaultSecretKey   = "my-value"
			vaultSecretValue = "bar"
		)

		BeforeAll(func() {
			By("Deploying Vault")
			Expect(applyVault(ctx)).To(Succeed())

			By("Waiting for Vault pod")
			Expect(waitForVaultPod(ctx, clientset)).To(Succeed())

			By("Initializing and unsealing Vault")
			token, err := setupVault(ctx, clientset)
			Expect(err).ToNot(HaveOccurred())

			By("Enable KV Engine")
			Expect(enableKVEngine(ctx, clientset, token)).To(Succeed())

			By("Create test secret in vault")
			Expect(createVaultTestSecret(
				ctx,
				clientset,
				token,
				vaultSecretName,
				vaultSecretKey,
				vaultSecretValue,
			)).To(Succeed())

			By("Creating vault-token Secret")
			Expect(createVaultTokenSecret(ctx, clientset, token)).To(Succeed())

		})

		AfterAll(func() {
			By("Cleaning up Vault namespace")
			safeDelete(ctx,
				"delete",
				"namespace", vaultNamespace,
				"--ignore-not-found",
				"--wait=true",
			)

			By("Cleaning up NetworkPolicy")
			safeDelete(ctx,
				"delete",
				"networkpolicy", "allow-to-vault-test",
				"-n", "external-secrets",
				"--ignore-not-found",
			)
		})

		It("should create secret mentioned in ExternalSecret using the referenced SecretStore", func() {
			var (
				// test bindata for Vault
				externalsecretsConfigFile  = "testdata/vault/externalsecretsconfig.yaml"
				vaultSecretStoreFile       = "testdata/vault/cluster_secret_store.yaml"
				vaultExternalSecretFile    = "testdata/vault/external_secret.yaml"
				secretStoreResourceName    = "vault-backend"
				externalSecretResourceName = "vault-example"
				targetSecretKey            = "password" //must match with external_secret.yaml data.secretKey
			)

			By("Applying ExternalSecretsConfig")
			loader.CreateFromFile(
				testassets.ReadFile,
				externalsecretsConfigFile,
				"",
			)

			By("Applying Vault NetworkPolicy")
			loader.CreateFromFile(
				testassets.ReadFile,
				vaultNetworkPolicyFile,
				"",
			)

			By("Creating SecretStore")
			loader.CreateFromFile(
				testassets.ReadFile,
				vaultSecretStoreFile,
				"",
			)

			By("Waiting for SecretStore to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: secretStoresKind,
				},
				vaultNamespace, secretStoreResourceName, time.Minute,
			)).To(Succeed())

			By("Creating ExternalSecret")
			loader.CreateFromFile(
				testassets.ReadFile,
				vaultExternalSecretFile,
				"",
			)

			By("Waiting for ExternalSecret to become Ready")
			Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
				schema.GroupVersionResource{
					Group:    externalSecretsGroupName,
					Version:  v1APIVersion,
					Resource: externalSecretsKind,
				},
				vaultNamespace, externalSecretResourceName, time.Minute,
			)).To(Succeed())

			By("Verifying the generated Kubernetes Secret contains expected value")
			Eventually(func(g Gomega) {
				secret, err := clientset.CoreV1().
					Secrets(vaultNamespace).
					Get(ctx, targetSecretName, metav1.GetOptions{})

				g.Expect(err).NotTo(HaveOccurred())

				value, exists := secret.Data[targetSecretKey]
				g.Expect(exists).To(BeTrue())

				actual := string(value)

				By(fmt.Sprintf("Expected: %s | Actual: %s",
					vaultSecretValue, actual))

				g.Expect(actual).To(Equal(vaultSecretValue),
					"Secret value mismatch. Expected=%s Actual=%s",
					vaultSecretValue, actual)

			}, time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	AfterAll(func() {
		By("Deleting the externalsecrets.openshift.operator.io/cluster CR")
		loader.DeleteFromFile(testassets.ReadFile, externalSecretsFile, "")

		By("Deleting the test namespace")
		Expect(clientset.CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})).
			NotTo(HaveOccurred(), "failed to delete test namespace")
	})
})

// Apply vault manifest
func applyVault(ctx context.Context) error {

	cmd := exec.CommandContext(ctx,
		"oc",
		"apply",
		"-f",
		vaultManifestFile,
	)

	out, err := cmd.CombinedOutput()
	By(fmt.Sprintf("oc apply output:\n%s", string(out)))

	if err != nil {
		return fmt.Errorf("Failed to apply vault manifest: %w", err)
	}

	return nil
}

// wait for vault pod
func waitForVaultPod(ctx context.Context, client *kubernetes.Clientset) error {
	return utils.VerifyPodsReadyByPrefix(
		ctx,
		client,
		vaultNamespace,
		[]string{"vault"},
	)
}

// setupVault function initializes and unselas the Vault instance running in the test namespace, then returns the generated root token.
// it executes the vault CLI commands inside the vault pod and extracts the unseal key and root token from the output,
// and prepares vault for further configuration in E2E tests.
func setupVault(ctx context.Context, client *kubernetes.Clientset) (string, error) {
	podName, err := getVaultPodName(ctx, client)
	if err != nil {
		return "", err
	}

	By(fmt.Sprintf("Initializing Vault, pod=%s", podName))

	// Step 1: Initialize Vault
	initCmd := exec.Command(
		"oc", "exec", "-n", vaultNamespace, podName, "--",
		"vault", "operator", "init", "-key-shares=1", "-key-threshold=1",
	)

	initOut, err := utils.Run(initCmd)
	if err != nil {
		return "", fmt.Errorf("vault init failed: %w", err)
	}

	By("Vault initialized successfully")

	// Step 2: Extract keys
	lines := strings.Split(string(initOut), "\n")

	var unsealKey, rootToken string

	for _, l := range lines {
		if strings.Contains(l, "Unseal Key 1:") {
			unsealKey = strings.TrimSpace(strings.Split(l, ":")[1])
		}
		if strings.Contains(l, "Initial Root Token:") {
			rootToken = strings.TrimSpace(strings.Split(l, ":")[1])
		}
	}

	if unsealKey == "" || rootToken == "" {
		return "", fmt.Errorf("failed to parse vault init output")
	}

	// Step 3: Unseal Vault
	By("Unsealing Vault")

	unsealCmd := exec.Command(
		"oc", "exec", "-n", vaultNamespace, podName, "--",
		"vault", "operator", "unseal", unsealKey,
	)

	unsealOut, err := utils.Run(unsealCmd)
	if err != nil {
		return "", fmt.Errorf("vault unseal failed: %w\n%s", err, string(unsealOut))
	}

	By(fmt.Sprintf("Vault unseal output:\n%s", string(unsealOut)))

	// Step 4: Login
	By("Logging into Vault")

	loginCmd := exec.Command(
		"oc", "exec", "-n", vaultNamespace, podName, "--",
		"vault", "login", rootToken,
	)

	loginOut, err := utils.Run(loginCmd)
	if err != nil {
		return "", fmt.Errorf("vault login failed: %w\n%s", err, string(loginOut))
	}

	By(fmt.Sprintf("Vault login output:\n%s", string(loginOut)))

	By("Vault initialized and unsealed successfully")

	return rootToken, nil
}

func getVaultPodName(ctx context.Context, clientset *kubernetes.Clientset) (string, error) {
	pods, err := clientset.CoreV1().
		Pods(vaultNamespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: "app=vault",
		})
	if err != nil {
		return "", err
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			return pod.Name, nil
		}
	}
	return "", fmt.Errorf("no running vault pod found")
}

// Enable KV engine
func enableKVEngine(ctx context.Context, client *kubernetes.Clientset, token string) error {
	podName, err := getVaultPodName(ctx, client)
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"oc", "exec", "-n", vaultNamespace, podName, "--", "sh", "-c",
		fmt.Sprintf(
			"vault status && vault login %s && (vault secrets enable -path=secret kv-v2 2>&1 || echo 'KV engine may already be enabled')",
			token,
		),
	)

	out, err := utils.Run(cmd)
	By(fmt.Sprintf("oc apply output:\n%s", string(out)))
	return err
}

// Create a vault test secret
func createVaultTestSecret(ctx context.Context, client *kubernetes.Clientset, token string, secretname string, key, value string) error {
	podName, err := getVaultPodName(ctx, client)
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"oc", "exec", "-n", vaultNamespace, podName, "--",
		"vault", "kv", "put", fmt.Sprintf("secret/%s", secretname),
		fmt.Sprintf("%s=%s", key, value),
	)

	out, err := utils.Run(cmd)
	By(fmt.Sprintf("oc apply output:\n%s", string(out)))
	return err
}

func createVaultTokenSecret(ctx context.Context, client *kubernetes.Clientset, token string) error {
	secretsClient := client.CoreV1().Secrets(vaultNamespace)
	existing, err := secretsClient.Get(ctx, "vault-token", metav1.GetOptions{})
	if err == nil {
		// Secret exists → update it
		existing.StringData = map[string]string{
			"token": token,
		}
		_, err = secretsClient.Update(ctx, existing, metav1.UpdateOptions{})
		return err
	}

	if !k8serrors.IsNotFound(err) {
		return err
	}

	// Secret does not exist → create
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vault-token",
			Namespace: vaultNamespace,
		},
		StringData: map[string]string{
			"token": token,
		},
		Type: corev1.SecretTypeOpaque,
	}

	_, err = secretsClient.Create(ctx, secret, metav1.CreateOptions{})
	return err
}

func safeDelete(ctx context.Context, args ...string) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, "oc", args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		By(fmt.Sprintf("Cleanup error: %s", string(out)))
	}
}
