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
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/test/utils"
)

const (
	bitwardenTLSSecretName            = "bitwarden-tls-certs"
	bitwardenSDKServerPodPrefix       = "bitwarden-sdk-server"
	bitwardenOperandWebhookPodPrefix  = "external-secrets-webhook-"
	bitwardenPushSecretResourceName   = "bitwarden-push-secret"
	bitwardenExternalSecretByNameName = "bitwarden-external-secret-by-name"
	bitwardenExternalSecretByUUIDName = "bitwarden-external-secret"
	// bitwardenResourceWaitTimeout allows for slow first requests and provider retries to the SDK server.
	bitwardenResourceWaitTimeout = 4 * time.Minute
)

// ensureBitwardenOperandReady ensures the cluster has ExternalSecretsConfig with Bitwarden enabled and
// bitwarden-sdk-server is running and reachable. It is used by the API:Bitwarden suite when run standalone
// (without Provider:Bitwarden) so that GET /ready can succeed. Uses package-level cfg and suite clients.
func ensureBitwardenOperandReady(ctx context.Context) error {
	clientset := suiteClientset
	dynamicClient := suiteDynamicClient
	runtimeClient := suiteRuntimeClient
	if clientset == nil || dynamicClient == nil || runtimeClient == nil {
		return fmt.Errorf("suite clients not initialized (run full suite or ensure BeforeSuite ran)")
	}

	tlsMaterials, err := utils.GenerateSelfSignedCertForBitwardenServer()
	if err != nil {
		return err
	}

	esc := &operatorv1alpha1.ExternalSecretsConfig{}
	if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
		if k8serrors.IsNotFound(err) {
			createESC, err := loadExternalSecretsConfigFromFileWithBitwardenNetworkPolicy(testassets.ReadFile, externalSecretsFile)
			if err != nil {
				return err
			}
			if err := runtimeClient.Create(ctx, createESC); err != nil {
				return err
			}
			if err := utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, "cluster", 2*time.Minute); err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		if err := utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, "cluster", 2*time.Minute); err != nil {
			return err
		}
	}

	_, err = clientset.CoreV1().Namespaces().Get(ctx, utils.BitwardenOperandNamespace, metav1.GetOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		operandNS := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   utils.BitwardenOperandNamespace,
				Labels: map[string]string{"app": "external-secrets"},
			},
		}
		if _, err := clientset.CoreV1().Namespaces().Create(ctx, operandNS, metav1.CreateOptions{}); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	_ = clientset.CoreV1().Secrets(utils.BitwardenOperandNamespace).Delete(ctx, bitwardenTLSSecretName, metav1.DeleteOptions{})
	if err := utils.CreateBitwardenTLSSecret(ctx, clientset, utils.BitwardenOperandNamespace, bitwardenTLSSecretName, tlsMaterials); err != nil {
		return err
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		esc := &operatorv1alpha1.ExternalSecretsConfig{}
		if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
			return err
		}
		if esc.Spec.Plugins.BitwardenSecretManagerProvider == nil {
			esc.Spec.Plugins.BitwardenSecretManagerProvider = &operatorv1alpha1.BitwardenSecretManagerProvider{}
		}
		esc.Spec.Plugins.BitwardenSecretManagerProvider.Mode = operatorv1alpha1.Enabled
		esc.Spec.Plugins.BitwardenSecretManagerProvider.SecretRef = &operatorv1alpha1.SecretReference{Name: bitwardenTLSSecretName}
		return runtimeClient.Update(ctx, esc)
	})
	if err != nil {
		return err
	}

	if err := utils.VerifyPodsReadyByPrefix(ctx, clientset, utils.BitwardenOperandNamespace, []string{bitwardenSDKServerPodPrefix}); err != nil {
		return err
	}
	if err := utils.VerifyPodsReadyByPrefix(ctx, clientset, utils.BitwardenOperandNamespace, []string{bitwardenOperandWebhookPodPrefix}); err != nil {
		return err
	}
	if err := utils.WaitForBitwardenSDKServerReachableFromCluster(ctx, clientset, 90*time.Second); err != nil {
		return err
	}
	return nil
}

var _ = Describe("Bitwarden Provider", Ordered, Label("Provider:Bitwarden", "Suite:Bitwarden"), func() {
	ctx := context.Background()
	var (
		clientset                  *kubernetes.Clientset
		dynamicClient              *dynamic.DynamicClient
		runtimeClient              client.Client
		loader                     utils.DynamicResourceLoader
		testNamespace              string
		clusterStoreName           string
		tlsMaterials               *utils.BitwardenTLSMaterials
		originalESC                *operatorv1alpha1.ExternalSecretsConfig
		cssObj                     *unstructured.Unstructured
		bitwardenOrgID             string
		bitwardenProjectID         string
		pushedRemoteKeyForUUIDTest string // set by first It, used by second It to look up secret UUID
	)

	BeforeAll(func() {
		var err error
		loader = utils.NewDynamicResourceLoader(ctx, &testing.T{})

		clientset = suiteClientset
		dynamicClient = suiteDynamicClient
		runtimeClient = suiteRuntimeClient

		token, orgID, projectID, err := utils.FetchBitwardenCredsFromSecret(ctx, clientset, utils.BitwardenCredSecretName, utils.BitwardenCredSecretNamespace)
		if err != nil || token == "" || orgID == "" || projectID == "" {
			Skip(fmt.Sprintf("Bitwarden credentials secret %s/%s required (keys: token, organization_id, project_id). See docs/e2e/README.md. Error: %v", utils.BitwardenCredSecretNamespace, utils.BitwardenCredSecretName, err))
		}
		bitwardenOrgID = orgID
		bitwardenProjectID = projectID

		By("Generating self-signed TLS materials for bitwarden-sdk-server")
		tlsMaterials, err = utils.GenerateSelfSignedCertForBitwardenServer()
		Expect(err).NotTo(HaveOccurred())

		// Ensure cluster ExternalSecretsConfig exists first so the operator creates the operand namespace (external-secrets).
		// When this suite runs before the main e2e Describe, create from testdata (with Bitwarden egress network policy) and wait for Ready.
		esc := &operatorv1alpha1.ExternalSecretsConfig{}
		if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
			if k8serrors.IsNotFound(err) {
				By("Creating cluster ExternalSecretsConfig from testdata with Bitwarden egress network policy")
				createESC, err := loadExternalSecretsConfigFromFileWithBitwardenNetworkPolicy(testassets.ReadFile, externalSecretsFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(runtimeClient.Create(ctx, createESC)).To(Succeed())
				By("Waiting for ExternalSecretsConfig to be Ready")
				Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, "cluster", 2*time.Minute)).To(Succeed())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		} else {
			By("Waiting for ExternalSecretsConfig to be Ready (cluster CR already exists)")
			Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, "cluster", 2*time.Minute)).To(Succeed())
		}

		// Ensure operand namespace exists: create it if missing (e.g. deleted by a previous AfterSuite).
		By("Ensuring operand namespace " + utils.BitwardenOperandNamespace + " exists")
		_, err = clientset.CoreV1().Namespaces().Get(ctx, utils.BitwardenOperandNamespace, metav1.GetOptions{})
		if err != nil && k8serrors.IsNotFound(err) {
			operandNS := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   utils.BitwardenOperandNamespace,
					Labels: map[string]string{"app": "external-secrets"},
				},
			}
			_, err = clientset.CoreV1().Namespaces().Create(ctx, operandNS, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		} else if err != nil {
			Expect(err).NotTo(HaveOccurred())
		}

		// Per CRD: when bitwardenSecretManagerProvider.mode is Enabled, secretRef or certManager must be set.
		// secretRef names the TLS secret (tls.crt, tls.key, ca.crt) for bitwarden-sdk-server in the operand namespace.
		By("Creating TLS secret in " + utils.BitwardenOperandNamespace + " namespace")
		_ = clientset.CoreV1().Secrets(utils.BitwardenOperandNamespace).Delete(ctx, bitwardenTLSSecretName, metav1.DeleteOptions{})
		err = utils.CreateBitwardenTLSSecret(ctx, clientset, utils.BitwardenOperandNamespace, bitwardenTLSSecretName, tlsMaterials)
		Expect(err).NotTo(HaveOccurred())

		By("Enabling Bitwarden in ExternalSecretsConfig with secretRef")
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			esc := &operatorv1alpha1.ExternalSecretsConfig{}
			if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
				return err
			}
			originalESC = esc.DeepCopy()
			if esc.Spec.Plugins.BitwardenSecretManagerProvider == nil {
				esc.Spec.Plugins.BitwardenSecretManagerProvider = &operatorv1alpha1.BitwardenSecretManagerProvider{}
			}
			esc.Spec.Plugins.BitwardenSecretManagerProvider.Mode = operatorv1alpha1.Enabled
			esc.Spec.Plugins.BitwardenSecretManagerProvider.SecretRef = &operatorv1alpha1.SecretReference{Name: bitwardenTLSSecretName}
			return runtimeClient.Update(ctx, esc)
		})
		Expect(err).NotTo(HaveOccurred())

		By("Restarting bitwarden-sdk-server pods so they load the new TLS secret")
		Expect(utils.RestartBitwardenSDKServerPods(ctx, clientset)).To(Succeed())

		By("Creating test namespace")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Labels:       map[string]string{"e2e-test": "true", "operator": "openshift-external-secrets-operator"},
				GenerateName: testNamespacePrefix,
			},
		}
		created, err := clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		testNamespace = created.Name

		By("Waiting for bitwarden-sdk-server pod to be ready")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, utils.BitwardenOperandNamespace, []string{
			bitwardenSDKServerPodPrefix,
		})).To(Succeed())

		By("Waiting for external-secrets webhook pod to be ready (required for ClusterSecretStore validation)")
		Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, utils.BitwardenOperandNamespace, []string{
			bitwardenOperandWebhookPodPrefix,
		})).To(Succeed())

		By("Verifying bitwarden-sdk-server is reachable from cluster (same network as controller)")
		Expect(utils.WaitForBitwardenSDKServerReachableFromCluster(ctx, clientset, 90*time.Second)).To(Succeed(),
			"bitwarden-sdk-server must be reachable at https://%s:%s/ready from within the cluster; check pod logs and TLS", utils.BitwardenSDKServerFQDN, utils.BitwardenSDKServerPort)

		clusterStoreName = fmt.Sprintf("bitwarden-store-%s", utils.GetRandomString(5))

		By("Creating ClusterSecretStore")
		caBundle := base64.StdEncoding.EncodeToString(tlsMaterials.CAPEM)
		sdkURL := utils.GetBitwardenSDKServerURL()
		cssObj = utils.BitwardenClusterSecretStore(clusterStoreName, utils.BitwardenCredSecretName, utils.BitwardenCredSecretNamespace, sdkURL, caBundle, bitwardenOrgID, bitwardenProjectID)
		err = loader.CreateFromUnstructuredReturnErr(cssObj, testNamespace)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred(), "create ClusterSecretStore %s", clusterStoreName)
		}

		By("Waiting for ClusterSecretStore to become Ready")
		Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
			schema.GroupVersionResource{
				Group:    externalSecretsGroupName,
				Version:  v1APIVersion,
				Resource: clusterSecretStoresKind,
			},
			"", clusterStoreName, bitwardenResourceWaitTimeout,
		)).To(Succeed())

		By("Copying Bitwarden cred secret to " + utils.BitwardenOperandNamespace + " for GetBitwardenSecretIDByKey Job")
		Expect(utils.CopySecretToNamespace(ctx, clientset, utils.BitwardenCredSecretName, utils.BitwardenCredSecretNamespace, utils.BitwardenCredSecretName, utils.BitwardenOperandNamespace)).To(Succeed())
	})

	AfterAll(func() {
		if pushedRemoteKeyForUUIDTest != "" {
			By("Deleting Bitwarden secret created by PushSecret")
			utils.DeleteBitwardenSecretByKey(ctx, clientset, utils.BitwardenOperandNamespace, pushedRemoteKeyForUUIDTest)
		}
		if cssObj != nil && clusterStoreName != "" {
			By("Deleting ClusterSecretStore")
			loader.DeleteFromUnstructured(cssObj, testNamespace)
		}
		if testNamespace != "" {
			By("Deleting test namespace")
			_ = clientset.CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})
		}
		if originalESC != nil {
			By("Reverting ExternalSecretsConfig Bitwarden plugin")
			_ = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				esc := &operatorv1alpha1.ExternalSecretsConfig{}
				if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, esc); err != nil {
					return err
				}
				esc.Spec.Plugins.BitwardenSecretManagerProvider = originalESC.Spec.Plugins.BitwardenSecretManagerProvider
				return runtimeClient.Update(ctx, esc)
			})
		}
	})

	It("should sync a secret via PushSecret then ExternalSecret by name", func() {
		Expect(bitwardenOrgID).NotTo(BeEmpty())
		Expect(bitwardenProjectID).NotTo(BeEmpty())

		pushRemoteKey := "e2e-bitwarden-" + utils.GetRandomString(5)
		pushSourceSecretName := "bitwarden-k8s-push-secret"
		pushValue := "secret-value-from-e2e"
		targetSecretByName := "bitwarden-synced-by-name"

		By("Creating source K8s Secret for PushSecret")
		pushSourceSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: pushSourceSecretName, Namespace: testNamespace},
			Type:       corev1.SecretTypeOpaque,
			Data:       map[string][]byte{"value": []byte(pushValue)},
		}
		_, err := clientset.CoreV1().Secrets(testNamespace).Create(ctx, pushSourceSecret, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			_ = clientset.CoreV1().Secrets(testNamespace).Delete(ctx, pushSourceSecretName, metav1.DeleteOptions{})
		}()

		By("Creating PushSecret")
		pushObj := utils.BitwardenPushSecret(bitwardenPushSecretResourceName, testNamespace, clusterStoreName, pushSourceSecretName, pushRemoteKey, "e2e push test")
		err = loader.CreateFromUnstructuredReturnErr(pushObj, testNamespace)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred(), "create PushSecret %s", bitwardenPushSecretResourceName)
		}
		defer loader.DeleteFromUnstructured(pushObj, testNamespace)

		By("Waiting for PushSecret to become Ready")
		Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
			schema.GroupVersionResource{
				Group:    externalSecretsGroupName,
				Version:  v1alpha1APIVersion,
				Resource: PushSecretsKind,
			},
			testNamespace, bitwardenPushSecretResourceName, bitwardenResourceWaitTimeout,
		)).To(Succeed())

		pushedRemoteKeyForUUIDTest = pushRemoteKey

		By("Creating ExternalSecret to pull by name")
		esByNameObj := utils.BitwardenExternalSecretByName(bitwardenExternalSecretByNameName, testNamespace, targetSecretByName, clusterStoreName, pushRemoteKey)
		err = loader.CreateFromUnstructuredReturnErr(esByNameObj, testNamespace)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred(), "create ExternalSecret %s", bitwardenExternalSecretByNameName)
		}
		defer loader.DeleteFromUnstructured(esByNameObj, testNamespace)

		By("Waiting for ExternalSecret (by name) to become Ready")
		Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
			schema.GroupVersionResource{
				Group:    externalSecretsGroupName,
				Version:  v1APIVersion,
				Resource: externalSecretsKind,
			},
			testNamespace, bitwardenExternalSecretByNameName, bitwardenResourceWaitTimeout,
		)).To(Succeed())

		By("Verifying target secret contains expected data")
		Eventually(func(g Gomega) {
			secret, err := clientset.CoreV1().Secrets(testNamespace).Get(ctx, targetSecretByName, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(secret.Data).To(HaveKey("value"))
			g.Expect(secret.Data["value"]).To(Equal([]byte(pushValue)))
		}, time.Minute, 10*time.Second).Should(Succeed())
	})

	It("should pull a secret by UUID (using secret created by PushSecret)", func() {
		Expect(pushedRemoteKeyForUUIDTest).NotTo(BeEmpty(), "first test must run first and set pushedRemoteKeyForUUIDTest")

		By("Looking up secret UUID by key via Bitwarden API")
		secretUUID, err := utils.GetBitwardenSecretIDByKey(ctx, clientset, utils.BitwardenOperandNamespace, pushedRemoteKeyForUUIDTest)
		Expect(err).NotTo(HaveOccurred(), "get secret ID by key %q", pushedRemoteKeyForUUIDTest)

		targetSecretName := "bitwarden-synced-by-uuid"

		By("Creating ExternalSecret (by UUID)")
		esByUUIDObj := utils.BitwardenExternalSecretByUUID(bitwardenExternalSecretByUUIDName, testNamespace, targetSecretName, clusterStoreName, secretUUID)
		err = loader.CreateFromUnstructuredReturnErr(esByUUIDObj, testNamespace)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred(), "create ExternalSecret %s", bitwardenExternalSecretByUUIDName)
		}
		defer loader.DeleteFromUnstructured(esByUUIDObj, testNamespace)

		By("Waiting for ExternalSecret (by UUID) to become Ready")
		Expect(utils.WaitForESOResourceReady(ctx, dynamicClient,
			schema.GroupVersionResource{
				Group:    externalSecretsGroupName,
				Version:  v1APIVersion,
				Resource: externalSecretsKind,
			},
			testNamespace, bitwardenExternalSecretByUUIDName, bitwardenResourceWaitTimeout,
		)).To(Succeed())

		By("Verifying target secret exists")
		Eventually(func(g Gomega) {
			secret, err := clientset.CoreV1().Secrets(testNamespace).Get(ctx, targetSecretName, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(secret.Data).To(HaveKey("value"))
		}, time.Minute, 10*time.Second).Should(Succeed())
	})
})

// loadExternalSecretsConfigFromFileWithBitwardenNetworkPolicy loads the cluster ExternalSecretsConfig from the
// given file and appends the network policy that allows the main controller to reach bitwarden-sdk-server on port 9998.
// This is used when the Bitwarden e2e creates the CR (CR does not exist yet). networkPolicies are immutable, so the
// policy must be set at create time.
func loadExternalSecretsConfigFromFileWithBitwardenNetworkPolicy(assetFunc func(string) ([]byte, error), filename string) (*operatorv1alpha1.ExternalSecretsConfig, error) {
	data, err := assetFunc(filename)
	if err != nil {
		return nil, err
	}
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(data), 1024)
	var rawObj runtime.RawExtension
	if err := decoder.Decode(&rawObj); err != nil {
		return nil, err
	}
	obj, _, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
	if err != nil {
		return nil, err
	}
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	esc := &operatorv1alpha1.ExternalSecretsConfig{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredMap, esc); err != nil {
		return nil, err
	}
	// Append egress so ExternalSecretsCoreController can reach bitwarden-sdk-server:9998 when Bitwarden plugin is enabled.
	port9998 := intstr.FromInt32(9998)
	tcp := corev1.ProtocolTCP
	esc.Spec.ControllerConfig.NetworkPolicies = append(esc.Spec.ControllerConfig.NetworkPolicies, operatorv1alpha1.NetworkPolicy{
		Name:          "allow-egress-to-bitwarden-sdk-server",
		ComponentName: operatorv1alpha1.CoreController,
		Egress: []networkingv1.NetworkPolicyEgressRule{
			{
				To: []networkingv1.NetworkPolicyPeer{
					{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": "bitwarden-sdk-server"}}},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{Protocol: &tcp, Port: &port9998},
				},
			},
		},
	})
	return esc, nil
}
