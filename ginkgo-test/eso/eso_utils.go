package oap

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	olmv1util "github.com/openshift/openshift-tests-private/test/extended/operators/olmv1util"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Immutable constant variables
const (
	ESOPackageName          = "openshift-external-secrets-operator"
	ESONamespace            = "external-secrets-operator"
	ESOperatorLabel         = "name=external-secrets-operator"
	ESODeploymentName       = "external-secrets-operator-controller-manager"
	ESOManagerLabel         = "app.kubernetes.io/name=external-secrets"
	ESOWebhookLabel         = "app.kubernetes.io/name=external-secrets-webhook"
	ESOCertControllerLabel  = "app.kubernetes.io/name=external-secrets-cert-controller"
	ESOperandsNamespace     = "external-secrets"
	ESOperandsLabel         = "app.kubernetes.io/instance=external-secrets"
	ESOCRDLabel             = "operators.coreos.com/openshift-external-secrets-operator.external-secrets-operator"
	ESOperandsDefaultPodNum = 3
)

// Other constant variables universally used
const (
	ESOSubscriptionName = "openshift-external-secrets-operator"
	ESOChannelName      = "tech-preview-v0.1"
	ESOExtensionName    = "clusterextension-eso"
	ESOFBCName          = "konflux-fbc-eso"
	ESOFBCImage         = "quay.io/redhat-user-workloads/external-secrets-oap-tenant/external-secrets-operator-fbc/external-secrets-operator-fbc:latest"
	ESOPreRelease       = true
)

type olmInstallConfig struct {
	mode                   string
	operatorNamespace      string
	buildPruningBaseDir    string
	subscriptionName       string // OLMv0
	catalogSourceName      string // OLMv0
	catalogSourceNamespace string // OLMv0
	channel                string
	packageName            string // OLMv1
	extensionName          string // OLMv1
	serviceAccountName     string // OLMv1
}

// Create External Secrets Operator
func installExternalSecretsOperator(oc *exutil.CLI, cfg olmInstallConfig) {

	exutil.SkipNoOLMCore(oc)
	createOperatorNamespace(oc, cfg.buildPruningBaseDir)

	switch cfg.mode {
	case "OLMv0":
		catalogSourceName, catalogSourceNamespace := determineCatalogSource(oc)

		installViaOLMv0(oc, cfg.operatorNamespace, cfg.buildPruningBaseDir, cfg.subscriptionName, catalogSourceName, catalogSourceNamespace, cfg.channel)
	case "OLMv1":
		installViaOLMv1(oc, cfg.operatorNamespace, cfg.packageName, cfg.extensionName, cfg.channel, cfg.serviceAccountName)
	default:
		e2e.Failf("Please set correct olmMode:%v expected: 'OLMv0' or 'OLMv1'", cfg.mode)
	}

	e2e.Logf("Create customresource for operand")
	operandConfig := filepath.Join(cfg.buildPruningBaseDir, "operandConfig.yaml")
	err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", operandConfig, "-n", ESONamespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	verifyOperandsForESO(oc, ESOperandsNamespace)
}

func determineCatalogSource(oc *exutil.CLI) (string, string) {
	e2e.Logf("=========Determining which catalogsource to use=========")
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", DefaultCatalogSourceNamespace, "catalogsource", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	var catalogSourceName, catalogSourceNamespace string
	if ESOPreRelease && strings.Contains(output, QECatalogSourceName) && strings.Contains(output, RHCatalogSourceName) && !strings.Contains(output, AutoReleaseCatalogSourceName) {
		e2e.Logf("=========Creating the catalogsource for ESO=========")
		catalogSourceName = ESOFBCName
		catalogSourceNamespace = ESONamespace
		createFBC(oc, catalogSourceName, catalogSourceNamespace, ESOFBCImage)
	} else {
		catalogSourceName = RHCatalogSourceName
		catalogSourceNamespace = DefaultCatalogSourceNamespace
	}

	// check if packagemanifest exists under the selected catalogsource
	output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "-n", catalogSourceNamespace, "-l", "catalog="+catalogSourceName, "--field-selector", "metadata.name="+ESOPackageName).Output()
	if !strings.Contains(output, ESOPackageName) || err != nil {
		g.Skip("skip since no available packagemanifest was found")
	}
	e2e.Logf("=> using catalogsource '%s' from namespace '%s'", catalogSourceName, catalogSourceNamespace)
	return catalogSourceName, catalogSourceNamespace
}

// create operator Namespace
func createOperatorNamespace(oc *exutil.CLI, buildPruningBaseDir string) {
	e2e.Logf("=========Create the operator namespace=========")
	namespaceFile := filepath.Join(buildPruningBaseDir, "namespace.yaml")
	output, err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", namespaceFile).Output()
	if strings.Contains(output, "being deleted") {
		g.Skip("skip the install process as the namespace is being terminated due to other env issue e.g. we ever hit such failures caused by OCPBUGS-31443")
	}
	if err != nil && !strings.Contains(output, "AlreadyExists") {
		e2e.Failf("Failed to apply namespace: %v", err)
	}
}

// install Via OLMv0
func installViaOLMv0(oc *exutil.CLI, operatorNamespace, buildPruningBaseDir, subscriptionName, catalogSourceName, catalogSourceNamespace, channel string) {
	e2e.Logf("=========Installing via OLMv0 (Subscription)=========")
	// Create operator group
	operatorGroupFile := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
	err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", operatorGroupFile).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("=========Create the subscription=========")
	subscriptionTemplate := filepath.Join(buildPruningBaseDir, "subscription.yaml")
	params := []string{"-f", subscriptionTemplate, "-p", "NAME=" + subscriptionName, "SOURCE=" + catalogSourceName, "SOURCE_NAMESPACE=" + catalogSourceNamespace, "CHANNEL=" + channel}
	exutil.ApplyNsResourceFromTemplate(oc, operatorNamespace, params...)
	// Wait for subscription state to become AtLatestKnown
	err = wait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 180*time.Second, true, func(context.Context) (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", subscriptionName, "-n", operatorNamespace, "-o=jsonpath={.status.state}").Output()
		if strings.Contains(output, "AtLatestKnown") {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		dumpResource(oc, operatorNamespace, "sub", subscriptionName, "-o=jsonpath={.status}")
	}
	exutil.AssertWaitPollNoErr(err, "timeout waiting for subscription state to become AtLatestKnown")

	e2e.Logf("=========retrieve the installed CSV name=========")
	csvName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", subscriptionName, "-n", operatorNamespace, "-o=jsonpath={.status.installedCSV}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(csvName).NotTo(o.BeEmpty())
	// Wait for csv phase to become Succeeded
	err = wait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 180*time.Second, true, func(context.Context) (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", csvName, "-n", operatorNamespace, "-o=jsonpath={.status.phase}").Output()
		if strings.Contains(output, "Succeeded") {
			e2e.Logf("csv '%s' installed successfully", csvName)
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		dumpResource(oc, operatorNamespace, "csv", csvName, "-o=jsonpath={.status}")
	}
	exutil.AssertWaitPollNoErr(err, "timeout waiting for csv phase to become Succeeded")
}

// install Via OLMv1
func installViaOLMv1(oc *exutil.CLI, operatorNamespace, packageName, clusterextensionName, channel, saCrbName string) {
	e2e.Logf("=========Installing via OLMv1 (ClusterExtension)=========")
	e2e.Logf("Create SA for clusterextension")
	var (
		baseDir                      = exutil.FixturePath("testdata", "olm", "v1")
		clusterextensionTemplate     = filepath.Join(baseDir, "clusterextensionWithoutVersion.yaml")
		saClusterRoleBindingTemplate = filepath.Join(baseDir, "sa-admin.yaml")
		saCrb                        = olmv1util.SaCLusterRolebindingDescription{
			Name:      saCrbName,
			Namespace: operatorNamespace,
			Template:  saClusterRoleBindingTemplate,
		}
		clusterextension = olmv1util.ClusterExtensionDescription{
			Name:             clusterextensionName,
			InstallNamespace: operatorNamespace,
			PackageName:      packageName,
			Channel:          channel,
			SaName:           saCrb.Name,
			Template:         clusterextensionTemplate,
		}
	)
	saCrb.Create(oc)

	e2e.Logf("Create ClusterExtension")
	clusterextension.Create(oc)
}

func verifyOperandsForESO(oc *exutil.CLI, ns string) {
	e2e.Logf("=========Checking the operand pods readiness in %s=========", ns)
	// Wait for pods phase to become Running
	err := wait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 120*time.Second, true, func(context.Context) (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", ns, "-l", ESOperandsLabel, "--field-selector=status.phase=Running", "-o=jsonpath={.items[*].metadata.name}").Output()
		if len(strings.Fields(output)) == ESOperandsDefaultPodNum {
			e2e.Logf("all operand pods are up and running!")
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", ns, "-l", ESOperandsLabel).Execute()
	}
	exutil.AssertWaitPollNoErr(err, "timeout waiting for all operand pods phase to become Running")

	waitForPodReadiness(oc, ns, ESOManagerLabel, 10*time.Second, 120*time.Second)
	waitForPodReadiness(oc, ns, ESOWebhookLabel, 10*time.Second, 120*time.Second)
	waitForPodReadiness(oc, ns, ESOCertControllerLabel, 10*time.Second, 120*time.Second)
}

func waitForPodReadiness(oc *exutil.CLI, namespace, label string, interval, timeout time.Duration) {
	err := wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, func(context.Context) (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "-l", label, `-o=jsonpath={..status.conditions[?(@.type=="Ready")].status}`).Output()
		if output == "True" {
			e2e.Logf("Pod with label %s is Ready!", label)
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "-l", label).Execute()
	}
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("timeout waiting for pod with label %s to become Ready", label))
}

func waitForPushSecretStatus(oc *exutil.CLI, namespace, name string, interval, timeout time.Duration) error {
	statusErr := wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, func(context.Context) (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pushsecret", "-n", namespace, name, `-o=jsonpath={..status.conditions[?(@.type=="Ready")].status}`).Output()
		if output == "True" {
			e2e.Logf("pushsecret is Ready!")
			return true, nil
		}
		e2e.Logf("pushsecret Ready status is %v", output)
		return false, nil
	})
	return statusErr
}

// uninstall External Secrets Operator and cleanup its operand resources
func cleanupExternalSecretsOperator(oc *exutil.CLI, cfg olmInstallConfig) {

	switch cfg.mode {
	case "OLMv0":
		uninstallViaOLMv0(oc, cfg.operatorNamespace, cfg.subscriptionName)
	case "OLMv1":
		uninstallViaOLMv1(oc, cfg.operatorNamespace, cfg.extensionName)
	default:
		e2e.Failf("Please set correct olmMode:%v expected: 'OLMv0' or 'OLMv1'", cfg.mode)
	}

	e2e.Logf("=========Delete the operatorconfig cluster object=========")
	// remove the finalizers from that object first, otherwise the deletion would be stuck
	err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("operatorconfigs", "cluster", "--type=merge", `-p={"metadata":{"finalizers":null}}`).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("operatorconfigs", "cluster").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	deleteNamespace(oc, ESONamespace)
	//deleteNamespace(oc, ESOperandsNamespace)

	e2e.Logf("=========Delete the operator CRD=========")
	err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("crd", "-l", ESOCRDLabel).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("=========Checking any of the resource types should be gone=========")
	statusErr := wait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
		err = oc.AsAdmin().Run("get").Args("secretstore").Execute()
		if err != nil {
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(statusErr, "timeout waiting for the CRDs deletion to take effect")

	e2e.Logf("=========Delete the admission webhook configurations of the the cert-manager=========")
	err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("validatingwebhookconfigurations", "-l", ESOperandsLabel).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("mutatingwebhookconfigurations", "-l", ESOperandsLabel).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("=========Delete the clusterrolebindings and clusterroles=========")
	err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterrolebindings", "-l", ESOCRDLabel).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterrole", "-l", ESOCRDLabel).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterrolebindings", "-l", ESOperandsLabel).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterrole", "-l", ESOperandsLabel).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// uninstall Via OLMv0
func uninstallViaOLMv0(oc *exutil.CLI, operatorNamespace, subscriptionName string) {
	e2e.Logf("=========Uninstalling via OLMv0 (Subscription)=========")
	e2e.Logf("=========Delete the subscription and installed CSV=========")
	csvName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", subscriptionName, "-n", operatorNamespace, "-o=jsonpath={.status.installedCSV}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("sub", subscriptionName, "-n", operatorNamespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("csv", csvName, "-n", operatorNamespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	waitForPodToBeDeleted(oc, operatorNamespace, ESOperatorLabel, 10*time.Second, 60*time.Second)
}

// uninstall Via OLMv1
func uninstallViaOLMv1(oc *exutil.CLI, operatorNamespace, clusterextensionName string) {
	e2e.Logf("=========Uninstalling via OLMv1 (ClusterExtension)=========")
	e2e.Logf("=========Delete the clusterextension=========")
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterextension", clusterextensionName).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	waitForPodToBeDeleted(oc, operatorNamespace, ESOperatorLabel, 10*time.Second, 60*time.Second)
}

// delete Namespace
func deleteNamespace(oc *exutil.CLI, namespace string) {
	e2e.Logf("=========delete namespace %v=========", namespace)
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("all", "--all", "-n", namespace, "--force", "--grace-period=0", "--wait=false").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
	err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", namespace, "--force", "--grace-period=0", "--wait=false", "-v=6").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForPodToBeDeleted(oc *exutil.CLI, namespace, label string, interval, timeout time.Duration) {
	err := wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, func(context.Context) (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "-l", label).Output()
		if strings.Contains(output, "No resources found") {
			e2e.Logf("pod with label '%s' deleted", label)
			return true, nil
		}
		return false, nil
	})
	exutil.AssertWaitPollNoErr(err, "timeout waiting for pod to be deleted")
}

// GetAWSSecret retrieves a secret from AWS Secrets Manager
func GetSecretAWS(accessKeyID, secureKey, region, secretName string) (string, error) {

	awsConfig, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secureKey, "")),
		config.WithRegion(region),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %v", err)
	}

	svc := secretsmanager.NewFromConfig(awsConfig)
	result, err := svc.GetSecretValue(context.TODO(), &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %v", err)
	}

	// SecretString
	if result.SecretString != nil {
		return *result.SecretString, nil
	}

	// SecretBinary
	if result.SecretBinary != nil {
		return string(result.SecretBinary), nil
	}

	return "", fmt.Errorf("secret value is nil")
}

// UpdateSecret updates the value of a secret in AWS Secrets Manager
func UpdateSecretAWS(accessKeyID, secureKey, region, secretName, newSecretValue string) error {

	awsConfig, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secureKey, "")),
		config.WithRegion(region),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %v", err)
	}

	svc := secretsmanager.NewFromConfig(awsConfig)
	_, err = svc.UpdateSecret(context.TODO(), &secretsmanager.UpdateSecretInput{
		SecretId:     aws.String(secretName),
		SecretString: aws.String(newSecretValue),
	})
	if err != nil {
		return fmt.Errorf("failed to update secret: %v", err)
	}
	e2e.Logf("Secret updated successfully!")
	return nil
}

// GetSecretValueByKeyAWS retrieve a specific secret value from AWS Secrets Manager
func GetSecretValueByKeyAWS(accessKeyID, secureKey, region, secretName, key string) (string, error) {

	secretValue, err := GetSecretAWS(accessKeyID, secureKey, region, secretName)
	if err != nil {
		return "", err
	}
	e2e.Logf("Secret Value: %v", secretValue)

	var secretData map[string]string
	if err := json.Unmarshal([]byte(secretValue), &secretData); err != nil {
		return "", fmt.Errorf("failed to parse secret JSON: %v", err)
	}

	// Extract the value of the specified Key
	value, exists := secretData[key]
	if !exists {
		return "", fmt.Errorf("key %v not found in secret", key)
	}

	return value, nil
}

// UpdateSecretValueByKeyAWS update specific fields in AWS Secrets Manager
func UpdateSecretValueByKeyAWS(accessKeyID, secureKey, region, secretName, key, newValue string) error {

	secretValue, err := GetSecretAWS(accessKeyID, secureKey, region, secretName)
	if err != nil {
		return fmt.Errorf("failed to get secret: %v", err)
	}

	var secretData map[string]string
	if err := json.Unmarshal([]byte(secretValue), &secretData); err != nil {
		return fmt.Errorf("failed to parse secret JSON: %v", err)
	}

	secretData[key] = newValue
	updatedSecretValue, err := json.Marshal(secretData)
	if err != nil {
		return fmt.Errorf("failed to encode updated secret JSON: %v", err)
	}

	return UpdateSecretAWS(accessKeyID, secureKey, region, secretName, string(updatedSecretValue))
}

// CreateSecretAWS creates a new secret in AWS Secrets Manager
func CreateSecretAWS(accessKeyID, secretKey, region, secretName, secretValue string) error {
	awsConfig, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, "")),
		config.WithRegion(region),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %v", err)
	}

	svc := secretsmanager.NewFromConfig(awsConfig)

	_, err = svc.CreateSecret(context.TODO(), &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(secretValue),
	})
	if err != nil {
		return fmt.Errorf("failed to create secret: %v", err)
	}

	e2e.Logf("Secret %s created successfully!", secretName)
	return nil
}

// DeleteSecretAWS deletes a secret from AWS Secrets Manager
func DeleteSecretAWS(accessKeyID, secretKey, region, secretName string, forceDelete bool) error {
	awsConfig, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, "")),
		config.WithRegion(region),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %v", err)
	}

	svc := secretsmanager.NewFromConfig(awsConfig)

	input := &secretsmanager.DeleteSecretInput{
		SecretId: aws.String(secretName),
	}

	// If forceDelete is true, skip recovery window and delete immediately
	if forceDelete {
		input.ForceDeleteWithoutRecovery = aws.Bool(true)
	} else {
		// Optional: specify recovery window in days (7 to 30)
		input.RecoveryWindowInDays = aws.Int64(7)
	}

	_, err = svc.DeleteSecret(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %v", err)
	}

	if forceDelete {
		e2e.Logf("Secret %s deleted permanently (no recovery).", secretName)
	} else {
		e2e.Logf("Secret %s scheduled for deletion (recoverable for 7 days).", secretName)
	}
	return nil
}

// initVaultSecret init Vault，enable KV v2
func initVaultSecret(oc *exutil.CLI, ns, vaultPodName, vaultToken, secretPath string) {
	// login to Vault with the VAULT_ROOT_TOKEN
	cmd := `vault login ` + vaultToken
	oc.NotShowInfo()
	_, err := exutil.RemoteShPod(oc, ns, vaultPodName, "sh", "-c", cmd)
	oc.SetShowInfo()
	o.Expect(err).NotTo(o.HaveOccurred())

	// enable KV v2
	cmd = `vault secrets enable -path=` + secretPath + ` -version=2 kv`
	_, err = exutil.RemoteShPod(oc, ns, vaultPodName, "sh", "-c", cmd)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// putVaultSecret put Secret and check
func putVaultSecret(oc *exutil.CLI, ns, vaultPodName, secretPath, SecretName, secretKey, secretValue string) {
	// put Secret
	cmd := fmt.Sprintf(`vault kv put %s/%s %s='%s'`, secretPath, SecretName, secretKey, secretValue)
	_, err := exutil.RemoteShPod(oc, ns, vaultPodName, "sh", "-c", cmd)
	o.Expect(err).NotTo(o.HaveOccurred())

	// check Secret
	cmd = fmt.Sprintf(`vault kv get -format=json %s/%s`, secretPath, SecretName)
	_, err = exutil.RemoteShPod(oc, ns, vaultPodName, "sh", "-c", cmd)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// getSecretValue fetches the base64-encoded value of a specific key in a Secret
func getSecretValue(oc *exutil.CLI, ns, secretName, secretKey string) (string, error) {
	data, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("-n", ns, "secret", secretName, "-o=jsonpath={.data."+secretKey+"}").Output()
	if err != nil {
		return "", fmt.Errorf("error fetching secret %s: %w", secretName, err)
	}
	return data, nil
}

// getSecretValueDecoded fetches and decodes the base64-encoded value of a Secret key
func getSecretValueDecoded(oc *exutil.CLI, ns, secretName, secretKey string) ([]byte, error) {
	encoded, err := getSecretValue(oc, ns, secretName, secretKey)
	if err != nil {
		return nil, err
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("error decoding value of secret %s: %w", secretName, err)
	}
	return decoded, nil
}

// waitForSecretUpdate wait for the specified key of the specified Secret to be updated to the expected value
func waitForSecretUpdate(oc *exutil.CLI, ns, secretName, secretKey, expectedValue string) {
	errWait := wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 30*time.Second, false, func(ctx context.Context) (bool, error) {
		currentValEncoded, err := getSecretValue(oc, ns, secretName, secretKey)
		if err != nil {
			e2e.Logf("Error fetching secret: %v", err)
			return false, nil
		}
		currentVal, err := base64.StdEncoding.DecodeString(currentValEncoded)
		if err != nil {
			e2e.Logf("Error decoding secret: %v", err)
			return false, nil
		}
		if !strings.Contains(string(currentVal), expectedValue) {
			e2e.Logf("Secret %s not updated yet. Old value: %s, New value: %s", secretName, expectedValue, string(currentVal))
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(errWait, fmt.Sprintf("Error: secret %s not updated", secretName))
}

func waitForExternalSecretStatus(oc *exutil.CLI, ns, name, expectedReason string, timeout time.Duration) {
	err := wait.PollUntilContextTimeout(context.TODO(), 3*time.Second, timeout, false, func(ctx context.Context) (bool, error) {
		status, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("-n", ns, "es", name, `-o=jsonpath={.status.conditions[?(@.type=="Ready")].reason}`).Output()
		if status != expectedReason || err != nil {
			e2e.Logf("status: %v, expected: %s", status, expectedReason)
			return false, nil
		}
		return true, nil
	})
	exutil.AssertWaitPollNoErr(err, fmt.Sprintf("status expected: %s", expectedReason))
}

// UpdateParameterAWS updates the value of a parameter in AWS SSM Parameter Store
func UpdateParameterAWS(accessKeyID, secureKey, region, parameterName, newValue string) error {
	awsConfig, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secureKey, "")),
		config.WithRegion(region),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %v", err)
	}

	ssmClient := ssm.NewFromConfig(awsConfig)
	_, err = ssmClient.PutParameter(context.TODO(), &ssm.PutParameterInput{
		Name:      aws.String(parameterName),
		Value:     aws.String(newValue),
		Overwrite: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("failed to update parameter: %v", err)
	}

	return nil
}

// GetSecretAWSPS retrieves the value of a parameter from AWS SSM Parameter Store
func GetSecretAWSPS(accessKeyID, secureKey, region, parameterName string) (string, error) {
	awsConfig, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secureKey, "")),
		config.WithRegion(region),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %v", err)
	}

	ssmClient := ssm.NewFromConfig(awsConfig)
	output, err := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name: aws.String(parameterName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get parameter: %v", err)
	}
	if output.Parameter == nil || output.Parameter.Value == nil {
		return "", fmt.Errorf("parameter %s not found or value is nil", parameterName)
	}

	return *output.Parameter.Value, nil
}
