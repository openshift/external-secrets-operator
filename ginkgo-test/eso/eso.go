package oap

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	gcpcrm "google.golang.org/api/cloudresourcemanager/v1"
	gcpiam "google.golang.org/api/iam/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-oap] OAP eso", func() {
	defer g.GinkgoRecover()

	var (
		oc                  = exutil.NewCLI("eso", exutil.KubeConfigPath())
		buildPruningBaseDir = exutil.FixturePath("testdata", "oap/eso")
		cfg                 = olmInstallConfig{
			mode:                "OLMv0",
			operatorNamespace:   ESONamespace,
			buildPruningBaseDir: buildPruningBaseDir,
			subscriptionName:    ESOSubscriptionName,
			channel:             ESOChannelName,
			packageName:         "external-secrets-operator",
			extensionName:       ESOExtensionName,
			serviceAccountName:  "sa-eso",
		}
	)
	g.BeforeEach(func() {

		if !isDeploymentReady(oc, ESONamespace, ESODeploymentName) {
			e2e.Logf("Creating External Secrets Operator...")
			installExternalSecretsOperator(oc, cfg)
		}

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-ConnectedOnly-High-80066-Get the secret value from AWS Secrets Manager", func() {

		exutil.SkipIfPlatformTypeNot(oc, "AWS")
		if exutil.IsSTSCluster(oc) {
			g.Skip("Skip for STS cluster")
		}
		exutil.SkipOnProxyCluster(oc)

		const (
			awsSecretName      = "aws-creds"
			secretstoreName    = "secretstore-80066"
			externalsecretName = "externalsecret-80066"
			secretRegion       = "us-east-2"
		)
		ns := oc.Namespace()

		exutil.By("Create secret that contains AWS accessKey")
		defer func() {
			e2e.Logf("Cleanup the created secret")
			err := oc.AsAdmin().Run("delete").Args("-n", ns, "secret", awsSecretName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		accessKeyID, secureKey := getCredentialFromCluster(oc, "aws")
		oc.NotShowInfo()
		err := oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", awsSecretName, "--from-literal=access-key="+accessKeyID, "--from-literal=secret-access-key="+secureKey).Execute()
		oc.SetShowInfo()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		defer func() {
			e2e.Logf("Cleanup the secret store")
			err := oc.AsAdmin().Run("delete").Args("-n", ns, "secretstore", secretstoreName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-awssm.yaml")
		params := []string{"-f", secretStoreTemplate, "-p", "NAME=" + secretstoreName, "REGION=" + secretRegion, "SECRETNAME=" + awsSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		exutil.By("Create external secret")
		defer func() {
			e2e.Logf("Cleanup the external secret")
			err := oc.AsAdmin().Run("delete").Args("-n", ns, "externalsecret", externalsecretName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		externalSecretTemplate := filepath.Join(buildPruningBaseDir, "externalsecret-awssm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p", "NAME=" + externalsecretName, "REFREASHINTERVAL=" + "1m", "SECRETSTORENAME=" + secretstoreName, "SECRETNAME=" + "secret-from-awssm"}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret content")
		data, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "secret", "secret-from-awssm", "-o=jsonpath={.data.secret-value-from-awssm}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		value, err := base64.StdEncoding.DecodeString(data)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(value).To(o.ContainSubstring(`"username":"jitli"`))

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-ConnectedOnly-High-80069-Check the secret value is updated from AWS Secrets Manager", func() {

		exutil.SkipIfPlatformTypeNot(oc, "AWS")
		if exutil.IsSTSCluster(oc) {
			g.Skip("Skip for STS cluster")
		}
		exutil.SkipOnProxyCluster(oc)

		const (
			awsSecretName       = "aws-creds"
			secretstoreName     = "secretstore-80069"
			externalsecretName  = "externalsecret-80069"
			secretRegion        = "us-east-2"
			secretName          = "jitliSecret"
			secretKey           = "password-80069"
			generatedSecretName = "secret-from-awssm"
		)
		var (
			newPasswd = getRandomString(8)
			ns        = oc.Namespace()
		)

		exutil.By("Create secret that contains AWS accessKey")
		defer func() {
			e2e.Logf("Cleanup the created secret")
			err := oc.AsAdmin().Run("delete").Args("-n", ns, "secret", awsSecretName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		accessKeyID, secureKey := getCredentialFromCluster(oc, "aws")
		oc.NotShowInfo()
		err := oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", awsSecretName, "--from-literal=access-key="+accessKeyID, "--from-literal=secret-access-key="+secureKey).Execute()
		oc.SetShowInfo()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-awssm.yaml")
		params := []string{"-f", secretStoreTemplate, "-p",
			"NAME=" + secretstoreName,
			"REGION=" + secretRegion,
			"SECRETNAME=" + awsSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		exutil.By("Create external secret")
		externalSecretTemplate := filepath.Join(buildPruningBaseDir, "externalsecret-awssm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"SECRETKEY=" + secretKey,
			"PROPERTY=" + secretKey}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret content")
		data, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "secret", generatedSecretName, "-o=jsonpath={.data."+secretKey+"}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(data).NotTo(o.BeEmpty())

		exutil.By("Update secret value")
		err = UpdateSecretValueByKeyAWS(accessKeyID, secureKey, secretRegion, secretName, secretKey, newPasswd)
		if err != nil {
			e2e.Failf("Failed to update secret: %v", err)
		}
		e2e.Logf("Secret key %v updated successfully!", secretKey)

		exutil.By("Check the secret value be synced")
		waitForSecretUpdate(oc, ns, generatedSecretName, secretKey, newPasswd)

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-ConnectedOnly-High-80759-Sync secret from AWS Parameter Store and verify updates", func() {

		exutil.SkipIfPlatformTypeNot(oc, "AWS")
		if exutil.IsSTSCluster(oc) {
			g.Skip("Skip for STS cluster")
		}
		exutil.SkipOnProxyCluster(oc)

		const (
			awsSecretName       = "aws-creds"
			secretstoreName     = "secretstore-80759"
			externalsecretName  = "externalsecret-80759"
			secretRegion        = "us-east-2"
			parameterName       = "esoParameter"
			secretKey           = "value-80759"
			generatedSecretName = "secret-from-parameter-store"
		)
		var (
			newValue = "/eso/test80759 = " + getRandomString(8)
			ns       = oc.Namespace()
		)

		exutil.By("Create secret that contains AWS accessKey")
		defer func() {
			e2e.Logf("Cleanup the created secret")
			err := oc.AsAdmin().Run("delete").Args("-n", ns, "secret", awsSecretName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		accessKeyID, secureKey := getCredentialFromCluster(oc, "aws")
		oc.NotShowInfo()
		err := oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", awsSecretName, "--from-literal=access-key="+accessKeyID, "--from-literal=secret-access-key="+secureKey).Execute()
		oc.SetShowInfo()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-awssm.yaml")
		params := []string{"-f", secretStoreTemplate, "-p",
			"NAME=" + secretstoreName,
			"SERVICE=" + "ParameterStore",
			"REGION=" + secretRegion,
			"SECRETNAME=" + awsSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		exutil.By("Create external secret")
		externalSecretTemplate := filepath.Join(buildPruningBaseDir, "externalsecret-awsps.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "10s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"SECRETKEY=" + secretKey,
			"KEY=" + parameterName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret content")
		data, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "secret", generatedSecretName, "-o=jsonpath={.data."+secretKey+"}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(data).NotTo(o.BeEmpty())

		exutil.By("Update parameter value")
		err = UpdateParameterAWS(accessKeyID, secureKey, secretRegion, parameterName, newValue)
		if err != nil {
			e2e.Failf("Failed to update parameter: %v", err)
		}
		e2e.Logf("Parameter %v updated successfully!", parameterName)

		exutil.By("Check the parameter value be synced")
		waitForSecretUpdate(oc, ns, generatedSecretName, secretKey, newValue)

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-NonPreRelease-PreChkUpgrade-ROSA-Medium-80703-needs prepare test data before OCP upgrade", func() {
		exutil.SkipIfPlatformTypeNot(oc, "AWS")
		if exutil.IsSTSCluster(oc) {
			g.Skip("Skip for STS cluster")
		}
		exutil.SkipOnProxyCluster(oc)

		const (
			awsSecretName       = "aws-creds"
			secretstoreName     = "secretstore-80703"
			externalsecretName  = "externalsecret-80703"
			secretRegion        = "us-east-2"
			sharedNamespace     = "ocp-65134-shared-ns"
			secretName          = "jitliSecret"
			secretKey           = "password-80703"
			generatedSecretName = "secret-from-awssm"
		)

		exutil.By("create a shared testing namespace")
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("namespace", sharedNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret that contains AWS accessKey")
		accessKeyID, secureKey := getCredentialFromCluster(oc, "aws")
		oc.NotShowInfo()
		err = oc.AsAdmin().Run("create").Args("-n", sharedNamespace, "secret", "generic", awsSecretName, "--from-literal=access-key="+accessKeyID, "--from-literal=secret-access-key="+secureKey).Execute()
		oc.SetShowInfo()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-awssm.yaml")
		params := []string{"-f", secretStoreTemplate, "-p", "NAME=" + secretstoreName, "REGION=" + secretRegion, "SECRETNAME=" + awsSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, sharedNamespace, params...)
		err = waitForResourceReadiness(oc, sharedNamespace, "secretstore", secretstoreName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, sharedNamespace, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		exutil.By("Create external secret")
		externalSecretTemplate := filepath.Join(buildPruningBaseDir, "externalsecret-awssm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"SECRETKEY=" + secretKey,
			"PROPERTY=" + secretKey}
		exutil.ApplyNsResourceFromTemplate(oc, sharedNamespace, params...)
		err = waitForResourceReadiness(oc, sharedNamespace, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, sharedNamespace, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret content")
		data, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", sharedNamespace, "secret", generatedSecretName, "-o=jsonpath={.data."+secretKey+"}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		value, err := base64.StdEncoding.DecodeString(data)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(value).NotTo(o.BeEmpty())

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-NonPreRelease-PstChkUpgrade-ROSA-Medium-80703-functions should work normally after OCP upgrade", func() {
		const (
			awsSecretName       = "aws-creds"
			secretstoreName     = "secretstore-80703"
			externalsecretName  = "externalsecret-80703"
			secretRegion        = "us-east-2"
			sharedNamespace     = "ocp-65134-shared-ns"
			secretName          = "jitliSecret"
			secretKey           = "password-80703"
			generatedSecretName = "secret-from-awssm"
		)

		// check if the shared testing namespace exists first
		err := oc.AsAdmin().WithoutNamespace().Run("get").Args("namespace", sharedNamespace).Execute()
		if err != nil {
			g.Skip("Skip the PstChkUpgrade test as namespace '" + sharedNamespace + "' does not exist, PreChkUpgrade test did not finish successfully")
		}
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("namespace", sharedNamespace, "--ignore-not-found").Execute()

		exutil.By("log the CSV post OCP upgrade")
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", ESONamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("check the operator and operands pods status, all of them should be Ready")
		exutil.AssertAllPodsToBeReadyWithPollerParams(oc, ESONamespace, 10*time.Second, 120*time.Second)
		//exutil.AssertAllPodsToBeReadyWithPollerParams(oc, operandNamespace, 10*time.Second, 120*time.Second)

		exutil.By("check the existing secretstore and externalsecret status, all of them should be Ready")
		err = waitForResourceReadiness(oc, sharedNamespace, "secretstore", secretstoreName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, sharedNamespace, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		err = waitForResourceReadiness(oc, sharedNamespace, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, sharedNamespace, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		data, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", sharedNamespace, "secret", generatedSecretName, "-o=jsonpath={.data."+secretKey+"}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		value, err := base64.StdEncoding.DecodeString(data)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(value).NotTo(o.BeEmpty())

		exutil.By("Update secret value")
		accessKeyID, secureKey := getCredentialFromCluster(oc, "aws")
		newPasswd := getRandomString(8)
		err = UpdateSecretValueByKeyAWS(accessKeyID, secureKey, secretRegion, secretName, secretKey, newPasswd)
		if err != nil {
			e2e.Failf("Failed to update secret: %v", err)
		}
		e2e.Logf("Secret key %v updated successfully!", secretKey)

		exutil.By("Check the secret value be synced")
		waitForSecretUpdate(oc, sharedNamespace, generatedSecretName, secretKey, newPasswd)
	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-ConnectedOnly-High-80711-Install HashiCorp Vault and sync secrets via ESO", func() {

		const (
			vaultSecretName     = "vault-token"
			secretstoreName     = "secretstore-80711"
			externalsecretName  = "externalsecret-80711"
			secretPath          = "secret"
			secretName          = "Secret80711"
			secretKey           = "password-80711"
			generatedSecretName = "secret-from-vault"
		)
		var (
			passwd           = getRandomString(8)
			newPasswd        = getRandomString(8)
			ns               = oc.Namespace()
			vaultReleaseName = "vault-" + getRandomString(4)
			route            = ""
		)

		exutil.By("Create Vault server")
		helmConfigFile := filepath.Join(buildPruningBaseDir, "helm-vault-config.yaml")
		vaultPodName, vaultRootToken := setupVaultServer(oc, ns, vaultReleaseName, helmConfigFile, false)

		exutil.By("Create a secret in Vault server")
		initVaultSecret(oc, ns, vaultPodName, vaultRootToken, secretPath)
		putVaultSecret(oc, ns, vaultPodName, secretPath, secretName, secretKey, passwd)

		err := oc.AsAdmin().Run("expose").Args("-n", ns, "service", vaultReleaseName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		errWait := wait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 10*time.Second, false, func(ctx context.Context) (bool, error) {
			route, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("route", vaultReleaseName, "-n", ns, "-o=jsonpath={.spec.host}").Output()
			if err != nil {
				e2e.Logf("output is %v, error is %v, and try next", route, err)
				return false, nil
			}
			if route == "" {
				e2e.Logf("route is empty")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errWait, "get vault route failed")

		exutil.By("Create secret that contains vault server token")
		err = oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", vaultSecretName, "--from-literal=token="+vaultRootToken).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-vault.yaml")
		serverURl := "http://" + route
		params := []string{"-f", secretStoreTemplate, "-p",
			"NAME=" + secretstoreName,
			"SERVER=" + serverURl,
			"PATH=" + secretPath}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		exutil.By("Create external secret")
		externalSecretTemplate := filepath.Join(buildPruningBaseDir, "externalsecret-vault.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"SECRETKEY=" + secretKey,
			"KEY=" + secretName,
			"PROPERTY=" + secretKey}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret content")
		data, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "secret", generatedSecretName, "-o=jsonpath={.data."+secretKey+"}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		value, err := base64.StdEncoding.DecodeString(data)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(value).To(o.ContainSubstring(passwd))

		exutil.By("Update secret value")
		putVaultSecret(oc, ns, vaultPodName, secretPath, secretName, secretKey, newPasswd)

		exutil.By("Check the secret value be synced")
		waitForSecretUpdate(oc, ns, generatedSecretName, secretKey, newPasswd)

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-ConnectedOnly-Medium-81818-Back up local Vault secret to AWS Parameter Store", func() {

		exutil.SkipIfPlatformTypeNot(oc, "AWS")
		if exutil.IsSTSCluster(oc) {
			g.Skip("Skip for STS cluster")
		}
		exutil.SkipOnProxyCluster(oc)

		const (
			vaultSecretName      = "vault-token"
			secretstoreVaultName = "secretstore-vault-81818"
			externalsecretName   = "externalsecret-81818"
			secretPath           = "secret"
			secretName           = "Secret81818"
			secretKey            = "password-81818"
			generatedSecretName  = "secret-from-vault"
			awsSecretName        = "aws-creds"
			secretstoreAWSName   = "secretstore-aws-81818"
			pushSecretName       = "pushsecret-81818"
			secretRegion         = "us-east-2"
			parameterName        = "Parameter-81818"
		)
		var (
			passwd           = getRandomString(8)
			newPasswd        = getRandomString(8)
			ns               = oc.Namespace()
			vaultReleaseName = "vault-" + getRandomString(4)
			route            = ""
		)

		exutil.By("Create Vault server")
		helmConfigFile := filepath.Join(buildPruningBaseDir, "helm-vault-config.yaml")
		vaultPodName, vaultRootToken := setupVaultServer(oc, ns, vaultReleaseName, helmConfigFile, false)

		exutil.By("Create a secret in Vault server")
		initVaultSecret(oc, ns, vaultPodName, vaultRootToken, secretPath)
		putVaultSecret(oc, ns, vaultPodName, secretPath, secretName, secretKey, passwd)

		err := oc.AsAdmin().Run("expose").Args("-n", ns, "service", vaultReleaseName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		errWait := wait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 10*time.Second, false, func(ctx context.Context) (bool, error) {
			route, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("route", vaultReleaseName, "-n", ns, "-o=jsonpath={.spec.host}").Output()
			if err != nil {
				e2e.Logf("output is %v, error is %v, and try next", route, err)
				return false, nil
			}
			if route == "" {
				e2e.Logf("route is empty")
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errWait, "get vault route failed")

		exutil.By("Create secret that contains vault server token")
		err = oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", vaultSecretName, "--from-literal=token="+vaultRootToken).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-vault.yaml")
		serverURl := "http://" + route
		params := []string{"-f", secretStoreTemplate, "-p",
			"NAME=" + secretstoreVaultName,
			"SERVER=" + serverURl,
			"PATH=" + secretPath}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreVaultName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreVaultName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		exutil.By("Create external secret")
		externalSecretTemplate := filepath.Join(buildPruningBaseDir, "externalsecret-vault.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreVaultName,
			"SECRETNAME=" + generatedSecretName,
			"SECRETKEY=" + secretKey,
			"KEY=" + secretName,
			"PROPERTY=" + secretKey}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret content")
		data, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "secret", generatedSecretName, "-o=jsonpath={.data."+secretKey+"}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		value, err := base64.StdEncoding.DecodeString(data)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(value).To(o.ContainSubstring(passwd))

		exutil.By("Create secret that contains AWS accessKey")
		defer func() {
			e2e.Logf("Cleanup the created secret")
			err := oc.AsAdmin().Run("delete").Args("-n", ns, "secret", awsSecretName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		accessKeyID, secureKey := getCredentialFromCluster(oc, "aws")
		oc.NotShowInfo()
		err = oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", awsSecretName, "--from-literal=access-key="+accessKeyID, "--from-literal=secret-access-key="+secureKey).Execute()
		oc.SetShowInfo()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		secretStoreTemplate = filepath.Join(buildPruningBaseDir, "secretstore-awssm.yaml")
		params = []string{"-f", secretStoreTemplate, "-p",
			"NAME=" + secretstoreAWSName,
			"SERVICE=" + "ParameterStore",
			"REGION=" + secretRegion,
			"SECRETNAME=" + awsSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreAWSName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreAWSName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		exutil.By("Create a PushSecret use this Generator as source")
		pushSecretTemplate := filepath.Join(buildPruningBaseDir, "pushsecret-aws-secretkey.yaml")
		params = []string{"-f", pushSecretTemplate, "-p",
			"NAME=" + pushSecretName,
			"REFREASHINTERVAL=" + "10s",
			"SECRETSTORENAME=" + secretstoreAWSName,
			"SECRETNAME=" + generatedSecretName,
			"SECRETKEY=" + secretKey,
			"KEY=" + parameterName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForPushSecretStatus(oc, ns, pushSecretName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "pushsecret", pushSecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for pushsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret content")
		secret, err := GetSecretAWSPS(accessKeyID, secureKey, secretRegion, parameterName)
		if err != nil && !strings.Contains(secret, secretKey) {
			e2e.Failf("Failed to get secret %v. or value not correct %v,%v", err, value, secretKey)
		}

		exutil.By("Update secret value")
		putVaultSecret(oc, ns, vaultPodName, secretPath, secretName, secretKey, newPasswd)

		exutil.By("Check the secret value be synced")
		waitForSecretUpdate(oc, ns, generatedSecretName, secretKey, newPasswd)

		exutil.By("Check the parameter value be updated")
		errWait = wait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
			secret, err = GetSecretAWSPS(accessKeyID, secureKey, secretRegion, parameterName)
			if err != nil {
				e2e.Logf("Error fetching secret: %v", err)
				return false, nil
			}
			if !strings.Contains(secret, newPasswd) {
				e2e.Logf("Value not correct %v,%v", secret, newPasswd)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errWait, "Error parameter store not updated")

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-ConnectedOnly-High-80443-Validate creationPolicy lifecycle across all modes", func() {

		exutil.SkipIfPlatformTypeNot(oc, "AWS")
		if exutil.IsSTSCluster(oc) {
			g.Skip("Skip for STS cluster")
		}
		exutil.SkipOnProxyCluster(oc)

		const (
			awsSecretName       = "aws-creds"
			secretstoreName     = "secretstore-80443"
			externalsecretName  = "externalsecret-80443"
			secretRegion        = "us-east-2"
			secretName          = "Secret-80443"
			secretKey           = "password-80443"
			generatedSecretName = "secret-from-awssm"
		)
		ns := oc.Namespace()

		exutil.By("Create secret that contains AWS accessKey")
		defer func() {
			e2e.Logf("Cleanup the created secret")
			err := oc.AsAdmin().Run("delete").Args("-n", ns, "secret", awsSecretName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		accessKeyID, secureKey := getCredentialFromCluster(oc, "aws")
		oc.NotShowInfo()
		err := oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", awsSecretName, "--from-literal=access-key="+accessKeyID, "--from-literal=secret-access-key="+secureKey).Execute()
		oc.SetShowInfo()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-awssm.yaml")
		params := []string{"-f", secretStoreTemplate, "-p",
			"NAME=" + secretstoreName,
			"REGION=" + secretRegion,
			"SECRETNAME=" + awsSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		exutil.By("Create secret with an irrelevant key")
		err = oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", generatedSecretName, "--from-literal=secret-store="+secretstoreName, "--from-literal=username="+secretRegion).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create external secret with creationPolicy is Owner")
		externalSecretTemplate := filepath.Join(buildPruningBaseDir, "externalsecret-awssm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"CREATIONPOLICY=" + "Owner",
			"KEY=" + secretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret username is updated")
		waitForSecretUpdate(oc, ns, generatedSecretName, "username", "jitli")

		exutil.By("Delete ExternalSecret, check secret has been deleted")
		err = oc.AsAdmin().Run("delete").Args("-n", ns, "externalsecret", externalsecretName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("-n", ns, "secret", generatedSecretName).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("not found"))
		e2e.Logf("Secret deleted successfully! %v", output)

		exutil.By("Create external secret with creationPolicy is Merge")
		externalSecretTemplate = filepath.Join(buildPruningBaseDir, "externalsecret-awssm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"CREATIONPOLICY=" + "Merge",
			"KEY=" + secretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Check the secret exists is SecretMissing, secret will not be created")
		errWait := wait.PollUntilContextTimeout(context.TODO(), 3*time.Second, 15*time.Second, false, func(ctx context.Context) (bool, error) {
			status, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("-n", ns, "es", externalsecretName, `-o=jsonpath={.status.conditions[?(@.type=="Ready")].reason}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if status != "SecretMissing" {
				e2e.Logf("status: %v, expecte: SecretMissing", status)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errWait, "status expecte: SecretMissing")
		output, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("-n", ns, "secret", generatedSecretName).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("not found"))

		exutil.By("Create secret with an irrelevant key and a key contained in the external key")
		err = oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", generatedSecretName, "--from-literal=secretRegion="+secretRegion, "--from-literal=username="+secretRegion).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		errWait = wait.PollUntilContextTimeout(context.TODO(), 3*time.Second, 15*time.Second, false, func(ctx context.Context) (bool, error) {
			status, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("-n", ns, "es", externalsecretName, `-o=jsonpath={.status.conditions[?(@.type=="Ready")].reason}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if status != "SecretSynced" {
				e2e.Logf("status: %v, expecte: SecretSynced", status)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errWait, "status expecte: SecretSynced")

		exutil.By("Check the secret value")
		waitForSecretUpdate(oc, ns, generatedSecretName, "username", "jitli")
		waitForSecretUpdate(oc, ns, generatedSecretName, "secretRegion", secretRegion)
		waitForSecretUpdate(oc, ns, generatedSecretName, "email", "jitli@redhat.com")

		exutil.By("Delete ExternalSecret, check secret not be deleted")
		err = oc.AsAdmin().Run("delete").Args("-n", ns, "externalsecret", externalsecretName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForSecretUpdate(oc, ns, generatedSecretName, "username", "jitli")
		waitForSecretUpdate(oc, ns, generatedSecretName, "secretRegion", secretRegion)
		waitForSecretUpdate(oc, ns, generatedSecretName, secretKey, "80443")
		err = oc.AsAdmin().Run("delete").Args("-n", ns, "secret", generatedSecretName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create external secret with creationPolicy is Orphan")
		externalSecretTemplate = filepath.Join(buildPruningBaseDir, "externalsecret-awssm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"CREATIONPOLICY=" + "Orphan",
			"KEY=" + secretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")
		waitForSecretUpdate(oc, ns, generatedSecretName, "email", "jitli@redhat.com")

		exutil.By("Delete ExternalSecret, check secret not be deleted")
		err = oc.AsAdmin().Run("delete").Args("-n", ns, "externalsecret", externalsecretName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForSecretUpdate(oc, ns, generatedSecretName, secretKey, "80443")
		err = oc.AsAdmin().Run("delete").Args("-n", ns, "secret", generatedSecretName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create external secret with creationPolicy is None")
		externalSecretTemplate = filepath.Join(buildPruningBaseDir, "externalsecret-awssm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"CREATIONPOLICY=" + "None",
			"KEY=" + secretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")
		output, err = oc.WithoutNamespace().AsAdmin().Run("get").Args("-n", ns, "secret", generatedSecretName).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("not found"))

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-ConnectedOnly-Medium-80549-Validate deletionPolicy lifecycle across all modes", func() {

		exutil.SkipIfPlatformTypeNot(oc, "AWS")
		if exutil.IsSTSCluster(oc) {
			g.Skip("Skip for STS cluster")
		}
		exutil.SkipOnProxyCluster(oc)

		const (
			awsSecretName       = "aws-creds"
			secretstoreName     = "secretstore-80549"
			externalsecretName  = "externalsecret-80549"
			secretRegion        = "us-east-2"
			secretName          = "Secret-80549"
			secretKey           = "password-80549"
			generatedSecretName = "secret-from-awssm"
		)
		ns := oc.Namespace()

		exutil.By("Create secret that contains AWS accessKey")
		defer func() {
			e2e.Logf("Cleanup the created secret")
			err := oc.AsAdmin().Run("delete").Args("-n", ns, "secret", awsSecretName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		accessKeyID, secureKey := getCredentialFromCluster(oc, "aws")
		oc.NotShowInfo()
		err := oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", awsSecretName, "--from-literal=access-key="+accessKeyID, "--from-literal=secret-access-key="+secureKey).Execute()
		oc.SetShowInfo()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-awssm.yaml")
		params := []string{"-f", secretStoreTemplate, "-p",
			"NAME=" + secretstoreName,
			"REGION=" + secretRegion,
			"SECRETNAME=" + awsSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		exutil.By("Create remote secret in AWSSM")
		defer func() {
			e2e.Logf("Cleanup the created secret in AWSSM")
			err = DeleteSecretAWS(accessKeyID, secureKey, secretRegion, secretName, true)
			if err != nil {
				e2e.Failf("Delete secret failed: %v", err)
			}
		}()
		newValue := getRandomString(8)
		err = CreateSecretAWS(accessKeyID, secureKey, secretRegion, secretName, `{"password":"`+newValue+`"}`)
		if err != nil {
			e2e.Failf("Create secret failed: %v", err)
		}

		exutil.By("Create external secret with default deletionPolicy is Retain")
		externalSecretTemplate := filepath.Join(buildPruningBaseDir, "externalsecret-awssm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"CREATIONPOLICY=" + "Owner",
			"DELPOLICY=" + "Retain",
			"KEY=" + secretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret username is updated")
		waitForSecretUpdate(oc, ns, generatedSecretName, "password", newValue)

		exutil.By("Retain the secret if all provider secrets have been deleted")
		err = DeleteSecretAWS(accessKeyID, secureKey, secretRegion, secretName, true)
		if err != nil {
			e2e.Failf("Delete secret failed: %v", err)
		}
		waitForExternalSecretStatus(oc, ns, externalsecretName, "SecretSyncedError", 15*time.Second)

		exutil.By("Check the secret value")
		waitForSecretUpdate(oc, ns, generatedSecretName, "password", newValue)

		err = oc.AsAdmin().Run("delete").Args("-n", ns, "externalsecret", externalsecretName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = CreateSecretAWS(accessKeyID, secureKey, secretRegion, secretName, `{"password":"`+newValue+`"}`)
		if err != nil {
			e2e.Failf("Create secret failed: %v", err)
		}

		exutil.By("Create external secret with deletionPolicy is Delete")
		externalSecretTemplate = filepath.Join(buildPruningBaseDir, "externalsecret-awssm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"DELPOLICY=" + "Delete",
			"KEY=" + secretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")
		waitForSecretUpdate(oc, ns, generatedSecretName, "password", newValue)

		exutil.By("Delete the secret if all provider secrets have been deleted")
		err = DeleteSecretAWS(accessKeyID, secureKey, secretRegion, secretName, true)
		if err != nil {
			e2e.Failf("Delete secret failed: %v", err)
		}
		waitForExternalSecretStatus(oc, ns, externalsecretName, "SecretDeleted", 15*time.Second)

		output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("-n", ns, "secret", generatedSecretName).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("not found"))

		err = oc.AsAdmin().Run("delete").Args("-n", ns, "externalsecret", externalsecretName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret with an irrelevant key")
		err = oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", generatedSecretName, "--from-literal=secret-store="+secretstoreName, "--from-literal=username="+secretRegion).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = CreateSecretAWS(accessKeyID, secureKey, secretRegion, secretName, `{"password":"`+newValue+`"}`)
		if err != nil {
			e2e.Failf("Create secret failed: %v", err)
		}

		exutil.By("Create external secret with deletionPolicy is Merge")
		externalSecretTemplate = filepath.Join(buildPruningBaseDir, "externalsecret-awssm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"CREATIONPOLICY=" + "Merge",
			"DELPOLICY=" + "Merge",
			"KEY=" + secretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")
		waitForSecretUpdate(oc, ns, generatedSecretName, "password", newValue)
		waitForSecretUpdate(oc, ns, generatedSecretName, "username", secretRegion)

		exutil.By("Recover the secret if all provider secrets have been deleted")
		err = DeleteSecretAWS(accessKeyID, secureKey, secretRegion, secretName, true)
		if err != nil {
			e2e.Failf("Delete secret failed: %v", err)
		}

		waitForExternalSecretStatus(oc, ns, externalsecretName, "SecretSynced", 15*time.Second)
		waitForSecretUpdate(oc, ns, generatedSecretName, "username", secretRegion)
		errWait := wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 30*time.Second, false, func(ctx context.Context) (bool, error) {
			data, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("-n", ns, "secret", secretName, "-o=jsonpath={.data.password").Output()
			if data != "" && err == nil {
				return false, fmt.Errorf("error secret password not deleted: %v", data)
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errWait, fmt.Sprintf("Error: secret %s not updated", secretName))
	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-ConnectedOnly-Medium-80569-ESO will take ownership of orphan secrets", func() {

		exutil.SkipIfPlatformTypeNot(oc, "AWS")
		if exutil.IsSTSCluster(oc) {
			g.Skip("Skip for STS cluster")
		}
		exutil.SkipOnProxyCluster(oc)

		const (
			awsSecretName       = "aws-creds"
			secretstoreName     = "secretstore-80569"
			externalsecretName  = "externalsecret-80569"
			secretRegion        = "us-east-2"
			secretName          = "jitliSecret"
			secretKey           = "password-80569"
			generatedSecretName = "secret-from-awssm-80569"
		)
		var (
			newValue = getRandomString(8)
			ns       = oc.Namespace()
		)

		exutil.By("Create secret that contains AWS accessKey")
		defer func() {
			e2e.Logf("Cleanup the created secret")
			err := oc.AsAdmin().Run("delete").Args("-n", ns, "secret", awsSecretName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		accessKeyID, secureKey := getCredentialFromCluster(oc, "aws")
		oc.NotShowInfo()
		err := oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", awsSecretName, "--from-literal=access-key="+accessKeyID, "--from-literal=secret-access-key="+secureKey).Execute()
		oc.SetShowInfo()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-awssm.yaml")
		params := []string{"-f", secretStoreTemplate, "-p",
			"NAME=" + secretstoreName,
			"REGION=" + secretRegion,
			"SECRETNAME=" + awsSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		exutil.By("Create a orphan secret")
		err = oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", generatedSecretName, "--from-literal=password-80569="+newValue, "--from-literal=unrelatedkey="+newValue).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Check the orphan secret ownerReferences")
		ownerReferences, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "secret", generatedSecretName, "-o=jsonpath={.ownerReferences}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ownerReferences).To(o.BeEmpty())

		exutil.By("Create external secret use the orphan secret same name")
		externalSecretTemplate := filepath.Join(buildPruningBaseDir, "externalsecret-awssm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"SECRETKEY=" + secretKey,
			"CREATIONPOLICY=" + "Owner",
			"PROPERTY=" + secretKey}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Check ESO will take ownership of orphan secret")
		ownerReferences, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "secret", generatedSecretName, `-o=jsonpath={.metadata.ownerReferences[?(@.kind=="ExternalSecret")].controller}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ownerReferences).To(o.ContainSubstring("true"))

		ownerReferences, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "secret", generatedSecretName, `-o=jsonpath={.metadata.ownerReferences[?(@.kind=="ExternalSecret")].blockOwnerDeletion}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ownerReferences).To(o.ContainSubstring("true"))

		exutil.By("Check unrelated key is gone")
		data, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "secret", generatedSecretName, "-o=jsonpath={.data.unrelatedkey}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(data).To(o.BeEmpty())

		exutil.By("Check managed key exists and has correct")
		data, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "secret", generatedSecretName, "-o=jsonpath={.data."+secretKey+"}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(data).NotTo(o.BeEmpty())
		value, err := base64.StdEncoding.DecodeString(data)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(value)).NotTo(o.Equal(newValue))

		exutil.By("Create a secret that already has ownership, create es, and fail to fight for ownership")
		externalSecretTemplate = filepath.Join(buildPruningBaseDir, "externalsecret-awssm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName + "-vie",
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"SECRETKEY=" + secretKey,
			"CREATIONPOLICY=" + "Owner",
			"PROPERTY=" + secretKey}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		errWait := wait.PollUntilContextTimeout(context.TODO(), 3*time.Second, 15*time.Second, false, func(ctx context.Context) (bool, error) {
			status, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("-n", ns, "es", externalsecretName+"-vie", `-o=jsonpath={.status.conditions[?(@.type=="Ready")].reason}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			message, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("-n", ns, "es", externalsecretName+"-vie", `-o=jsonpath={.status.conditions[?(@.type=="Ready")].message}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if status == "SecretSyncedError" && strings.Contains(message, "target is owned by another ExternalSecret") {
				return true, nil
			}
			e2e.Logf("status: %v, expecte: SecretSyncedError , message: %v", status, message)
			return false, nil
		})
		exutil.AssertWaitPollNoErr(errWait, "status expecte: SecretSyncedError")

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-ConnectedOnly-Low-81666-Check ESO decoding strategies", func() {

		exutil.SkipIfPlatformTypeNot(oc, "AWS")
		if exutil.IsSTSCluster(oc) {
			g.Skip("Skip for STS cluster")
		}
		exutil.SkipOnProxyCluster(oc)

		const (
			awsSecretName       = "aws-creds"
			secretstoreName     = "secretstore-81666"
			externalsecretName  = "externalsecret-81666"
			secretRegion        = "us-east-2"
			secretName          = "jitliSecret"
			secretKey           = "password-81666"
			generatedSecretName = "secret-from-awssm"
		)
		var (
			ns = oc.Namespace()
		)

		exutil.By("Create secret that contains AWS accessKey")
		defer func() {
			e2e.Logf("Cleanup the created secret")
			err := oc.AsAdmin().Run("delete").Args("-n", ns, "secret", awsSecretName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		accessKeyID, secureKey := getCredentialFromCluster(oc, "aws")
		oc.NotShowInfo()
		err := oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", awsSecretName, "--from-literal=access-key="+accessKeyID, "--from-literal=secret-access-key="+secureKey).Execute()
		oc.SetShowInfo()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-awssm.yaml")
		params := []string{"-f", secretStoreTemplate, "-p",
			"NAME=" + secretstoreName,
			"REGION=" + secretRegion,
			"SECRETNAME=" + awsSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		exutil.By("Create external secret")
		externalSecretTemplate := filepath.Join(buildPruningBaseDir, "externalsecret-awssm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "5s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 120*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Get secret value")
		secretValue, err := GetSecretValueByKeyAWS(accessKeyID, secureKey, secretRegion, secretName, secretKey)
		if err != nil {
			e2e.Failf("Failed to get secret: %v", err)
		}

		exutil.By("Check the secret exists and verify the secret content")
		waitForSecretUpdate(oc, ns, generatedSecretName, secretKey, secretValue)

		exutil.By("Enable the decoding strategies")
		err = oc.AsAdmin().Run("patch").Args("externalsecret", externalsecretName, "-p", `{"spec":{"dataFrom":[{"extract":{"key":"jitliSecret","decodingStrategy":"Auto"}}]}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Check decoding value")
		errWait := wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 30*time.Second, false, func(ctx context.Context) (bool, error) {
			data, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("-n", ns, "secret", generatedSecretName, "-o=jsonpath={.data."+secretKey+"}").Output()
			if err != nil {
				e2e.Logf("Error fetching secret: %v", err)
				return false, nil
			}
			if secretValue != data {
				e2e.Logf("Secret %s: expected value %v, got value %v", generatedSecretName, secretValue, data)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errWait, "Error secret not decoding")
	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-Medium-81695-Generate random passwords using ESO", func() {

		const (
			clustergeneratorName       = "clustergenerator-81695"
			generatorName              = "generator-81695"
			externalsecretName         = "externalsecret-81695"
			secretKey                  = "password"
			generatedSecretName        = "secret-from-generator"
			generatedClusterSecretName = "secret-from-clustergenerator"
		)
		ns := oc.Namespace()

		defer func() {
			e2e.Logf("Cleanup the cluster generator")
			err := oc.AsAdmin().Run("delete").Args("clustergenerator", clustergeneratorName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		exutil.By("Create a kind password cluster generator")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "clustergenerator-password.yaml")
		params := []string{"-f", secretStoreTemplate, "-p", "NAME=" + clustergeneratorName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)

		exutil.By("Create external secret")
		externalSecretTemplate := filepath.Join(buildPruningBaseDir, "externalsecret-generator.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "10s",
			"SECRETNAME=" + generatedClusterSecretName,
			"GENERATORKIND=" + "ClusterGenerator",
			"GENERATOR=" + clustergeneratorName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err := waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Get secret value")
		decodedValue, err := getSecretValueDecoded(oc, ns, generatedClusterSecretName, secretKey)
		o.Expect(err).NotTo(o.HaveOccurred())
		decodedStr := string(decodedValue)
		e2e.Logf("Decoded secret value: %s", decodedStr)

		o.Expect(len(decodedStr)).To(o.Equal(16), "expected generated password to have length 16")
		o.Expect(regexp.MustCompile(`[0-9]`).FindAllString(decodedStr, -1)).To(o.HaveLen(5), "expected at least 5 digits")
		o.Expect(regexp.MustCompile(`[-_\$@]`).FindAllString(decodedStr, -1)).To(o.HaveLen(5), "expected at least 5 symbols from -_$@")
		o.Expect(regexp.MustCompile(`[A-Z]`).MatchString(decodedStr)).To(o.BeTrue(), "expected at least one uppercase letter")
		o.Expect(regexp.MustCompile(`[a-z]`).MatchString(decodedStr)).To(o.BeTrue(), "expected at least one lowercase letter")

		exutil.By("Check the secret update and verify the secret content")
		errWait := wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 30*time.Second, false, func(ctx context.Context) (bool, error) {
			newValue, err := getSecretValueDecoded(oc, ns, generatedClusterSecretName, secretKey)
			if err != nil {
				e2e.Logf("Error get decoded secret: %v", err)
				return false, nil
			}
			if string(decodedValue) == string(newValue) {
				e2e.Logf("Secret %s: expected value updated %v, got value %v", generatedClusterSecretName, string(decodedValue), string(newValue))
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errWait, "Error secret not updated")

		err = oc.AsAdmin().Run("delete").Args("-n", ns, "externalsecret", externalsecretName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create a Namespaced generator")
		secretStoreTemplate = filepath.Join(buildPruningBaseDir, "generator-password.yaml")
		params = []string{"-f", secretStoreTemplate, "-p", "NAME=" + generatorName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)

		exutil.By("Create external secret")
		externalSecretTemplate = filepath.Join(buildPruningBaseDir, "externalsecret-generator.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "10s",
			"SECRETNAME=" + generatedSecretName,
			"GENERATORKIND=" + "Password",
			"GENERATOR=" + generatorName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Get secret value")
		decodedValue, err = getSecretValueDecoded(oc, ns, generatedSecretName, secretKey)
		o.Expect(err).NotTo(o.HaveOccurred())
		decodedStr = string(decodedValue)
		e2e.Logf("Decoded secret value: %s", decodedStr)

		o.Expect(len(decodedStr)).To(o.Equal(16), "expected generated password to have length 16")
		o.Expect(regexp.MustCompile(`[0-9]`).FindAllString(decodedStr, -1)).To(o.HaveLen(5), "expected at least 5 digits")
		o.Expect(regexp.MustCompile(`[-_\$@]`).FindAllString(decodedStr, -1)).To(o.HaveLen(5), "expected at least 5 symbols from -_$@")
		o.Expect(regexp.MustCompile(`[A-Z]`).MatchString(decodedStr)).To(o.BeTrue(), "expected at least one uppercase letter")
		o.Expect(regexp.MustCompile(`[a-z]`).MatchString(decodedStr)).To(o.BeTrue(), "expected at least one lowercase letter")

		exutil.By("Check the secret update and verify the secret content")
		errWait = wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 30*time.Second, false, func(ctx context.Context) (bool, error) {
			newValue, err := getSecretValueDecoded(oc, ns, generatedSecretName, secretKey)
			if err != nil {
				e2e.Logf("Error get decoded secret: %v", err)
				return false, nil
			}
			if string(decodedValue) == string(newValue) {
				e2e.Logf("Secret %s: expected value updated %v, got value %v", generatedSecretName, string(decodedValue), string(newValue))
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errWait, "Error secret not updated")

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-ARO-OSD_CCS-Medium-80722-ESO can be uninstalled from CLI and then reinstalled [Disruptive]", func() {
		exutil.By("uninstall the external-secrets-operator and cleanup its operand resources")
		cleanupExternalSecretsOperator(oc, cfg)

		exutil.By("install the external-secrets-operator again")
		installExternalSecretsOperator(oc, cfg)

		exutil.By("checking the resource types should be ready")
		statusErr := wait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
			err := oc.AsAdmin().Run("get").Args("secretstore").Execute()
			if err == nil {
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(statusErr, "timeout waiting for the CRDs ready")

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-ConnectedOnly-High-81709-Push the entire Secret AWS Parameter Store", func() {

		exutil.SkipIfPlatformTypeNot(oc, "AWS")
		if exutil.IsSTSCluster(oc) {
			g.Skip("Skip for STS cluster")
		}
		exutil.SkipOnProxyCluster(oc)

		const (
			awsSecretName   = "aws-creds"
			secretstoreName = "secretstore-81709"
			pushSecretName  = "pushsecret-81709"
			secretRegion    = "us-east-2"
			parameterName   = "Parameter-81709"
			secretKey       = "value-81709"
			secretName      = "secret-push-parameter-store"
		)
		var (
			newValue = getRandomString(8)
			ns       = oc.Namespace()
		)

		exutil.By("Create secret that contains AWS accessKey")
		defer func() {
			e2e.Logf("Cleanup the created secret")
			err := oc.AsAdmin().Run("delete").Args("-n", ns, "secret", awsSecretName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		accessKeyID, secureKey := getCredentialFromCluster(oc, "aws")
		oc.NotShowInfo()
		err := oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", awsSecretName, "--from-literal=access-key="+accessKeyID, "--from-literal=secret-access-key="+secureKey).Execute()
		oc.SetShowInfo()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-awssm.yaml")
		params := []string{"-f", secretStoreTemplate, "-p",
			"NAME=" + secretstoreName,
			"SERVICE=" + "ParameterStore",
			"REGION=" + secretRegion,
			"SECRETNAME=" + awsSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		err = oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", secretName, "--from-literal=secret-access-key="+secretKey).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create a pushsecret to push the secret")
		pushSecretTemplate := filepath.Join(buildPruningBaseDir, "pushsecret-aws.yaml")
		params = []string{"-f", pushSecretTemplate, "-p",
			"NAME=" + pushSecretName,
			"REFREASHINTERVAL=" + "10s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + secretName,
			"KEY=" + parameterName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForPushSecretStatus(oc, ns, pushSecretName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "pushsecret", pushSecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for pushsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret content")
		value, err := GetSecretAWSPS(accessKeyID, secureKey, secretRegion, parameterName)
		if err != nil && !strings.Contains(value, secretKey) {
			e2e.Failf("Failed to get secret %v. or value not correct %v,%v", err, value, secretKey)
		}

		exutil.By("Update the secret value")
		err = oc.AsAdmin().Run("patch").Args("secret", secretName, "-p", `{"data":{"access-key":"`+newValue+`"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Check the parameter value be synced")
		errWait := wait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
			value, err = GetSecretAWSPS(accessKeyID, secureKey, secretRegion, parameterName)
			if err != nil {
				e2e.Logf("Error fetching secret: %v", err)
				return false, nil
			}
			if !strings.Contains(value, newValue) {
				e2e.Logf("Value not correct %v,%v", value, newValue)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errWait, "Error parameter store not updated")
	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-ConnectedOnly-High-81708-Push key value to AWS Secrets Manager", func() {

		exutil.SkipIfPlatformTypeNot(oc, "AWS")
		if exutil.IsSTSCluster(oc) {
			g.Skip("Skip for STS cluster")
		}
		exutil.SkipOnProxyCluster(oc)

		const (
			awsSecretName   = "aws-creds"
			secretstoreName = "secretstore-81708"
			pushSecretName  = "pushsecret-81708"
			secretRegion    = "us-east-2"
			smSecretName    = "Secret-81708"
			secretKey       = "value-81708"
			pushSecret      = "secret-push-secret-manager"
		)
		var (
			newValue = getRandomString(8)
			ns       = oc.Namespace()
		)

		exutil.By("Create secret that contains AWS accessKey")
		defer func() {
			e2e.Logf("Cleanup the created secret")
			err := oc.AsAdmin().Run("delete").Args("-n", ns, "secret", awsSecretName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		accessKeyID, secureKey := getCredentialFromCluster(oc, "aws")
		oc.NotShowInfo()
		err := oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", awsSecretName, "--from-literal=access-key="+accessKeyID, "--from-literal=secret-access-key="+secureKey).Execute()
		oc.SetShowInfo()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-awssm.yaml")
		params := []string{"-f", secretStoreTemplate, "-p",
			"NAME=" + secretstoreName,
			"SERVICE=" + "SecretsManager",
			"REGION=" + secretRegion,
			"SECRETNAME=" + awsSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		err = oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", pushSecret, "--from-literal=secret-access-key="+secretKey).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create a pushsecret to push the secret")
		pushSecretTemplate := filepath.Join(buildPruningBaseDir, "pushsecret-aws-secretkey.yaml")
		params = []string{"-f", pushSecretTemplate, "-p",
			"NAME=" + pushSecretName,
			"REFREASHINTERVAL=" + "10s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + pushSecret,
			"SECRETKEY=" + "secret-access-key",
			"KEY=" + smSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForPushSecretStatus(oc, ns, pushSecretName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "pushsecret", pushSecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for pushsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret content")
		errWait := wait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
			secretValue, err := GetSecretAWS(accessKeyID, secureKey, secretRegion, smSecretName)
			if err != nil && !strings.Contains(secretValue, secretKey) {
				e2e.Logf("Failed to get secret %v. or value not correct %v,%v", err, secretValue, secretKey)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errWait, "Error AWSSM not updated")

		exutil.By("Update the secret value")
		secretVault := `{"password":"` + newValue + `"}`
		encodedValue := base64.StdEncoding.EncodeToString([]byte(secretVault))
		err = oc.AsAdmin().Run("patch").Args("secret", pushSecret, "-p", `{"data":{"secret-access-key":"`+encodedValue+`"}}`, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Check the AWSSM value be synced")
		errWait = wait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
			value, err := GetSecretAWS(accessKeyID, secureKey, secretRegion, smSecretName)
			if err != nil {
				e2e.Logf("Error fetching secret: %v", err)
				return false, nil
			}
			if !strings.Contains(value, newValue) {
				e2e.Logf("Value not correct %v,%v", value, newValue)
				return false, nil
			}
			return true, nil
		})
		exutil.AssertWaitPollNoErr(errWait, "Error AWSSM not updated")

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ConnectedOnly-High-80719-Get the secret value from Google Secret Manager", func() {

		exutil.SkipIfPlatformTypeNot(oc, "GCP")
		if exutil.IsSTSCluster(oc) {
			g.Skip("Skip for STS cluster")
		}
		exutil.SkipOnProxyCluster(oc)

		exutil.SkipIfPlatformTypeNot(oc, "GCP")
		const projectID = "openshift-qe"
		if id, _ := exutil.GetGcpProjectID(oc); id != projectID {
			e2e.Logf("current GCP project ID: %s", id)
			g.Skip("Skip as the testing environment is only pre-setup under 'openshift-qe' project")
		}

		const (
			secretstoreName     = "secretstore-80719"
			externalsecretName  = "externalsecret-80719"
			secretName          = "Secret-80719"
			secretKey           = "password-80719"
			generatedSecretName = "secret-from-gcpsm"
			pushSecretName      = "secret-push-gcpsm"
			saSecretName        = "google-eso-sa-key"
			saPrefix            = "test-private-80719-eso-"
		)
		ns := oc.Namespace()

		exutil.By("create the GCP IAM and CloudResourceManager client")
		// Note that in Prow CI, the credentials source is automatically pre-configured to by the step 'openshift-extended-test'
		// See https://github.com/openshift/release/blob/69b2b9c4f28adcfcc5b9ff4820ecbd8d2582a3d7/ci-operator/step-registry/openshift-extended/test/openshift-extended-test-commands.sh#L43
		iamService, err := gcpiam.NewService(context.Background())
		o.Expect(err).NotTo(o.HaveOccurred())
		crmService, err := gcpcrm.NewService(context.Background())
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("create a GCP service account")
		serviceAccountName := saPrefix + getRandomString(4)
		request := &gcpiam.CreateServiceAccountRequest{
			AccountId: serviceAccountName,
			ServiceAccount: &gcpiam.ServiceAccount{
				DisplayName: "google-secret-manager service account for ESO",
			},
		}
		result, err := iamService.Projects.ServiceAccounts.Create("projects/"+projectID, request).Do()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			e2e.Logf("cleanup the created GCP service account")
			_, err = iamService.Projects.ServiceAccounts.Delete(result.Name).Do()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		exutil.By("add IAM policy binding with role 'secretmanager.admin' to GCP project")
		projectRole := "roles/secretmanager.admin"
		projectMember := fmt.Sprintf("serviceAccount:%s", result.Email)
		defer func() {
			e2e.Logf("cleanup the added IAM policy binding from GCP project")
			updateIamPolicyBinding(crmService, projectID, projectRole, projectMember, false)
		}()
		updateIamPolicyBinding(crmService, projectID, projectRole, projectMember, true)

		exutil.By("create key for the GCP service account and store as a secret")
		resource := fmt.Sprintf("projects/-/serviceAccounts/%s", result.Email)
		key, err := iamService.Projects.ServiceAccounts.Keys.Create(resource, &gcpiam.CreateServiceAccountKeyRequest{}).Do()
		o.Expect(err).NotTo(o.HaveOccurred())
		value, err := base64.StdEncoding.DecodeString(key.PrivateKeyData)
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.NotShowInfo()
		err = oc.AsAdmin().Run("create").Args("-n", oc.Namespace(), "secret", "generic", saSecretName, "--from-literal=key.json="+string(value)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.SetShowInfo()

		exutil.By("Create secret store")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-gcpsm.yaml")
		params := []string{"-f", secretStoreTemplate, "-p",
			"NAME=" + secretstoreName,
			"SECRETNAME=" + saSecretName,
			"SECRETACCESSKEY=" + "key.json"}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		exutil.By("Create external secret")
		externalSecretTemplate := filepath.Join(buildPruningBaseDir, "externalsecret-gcpsm.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "10s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedSecretName,
			"SECRETKEY=" + secretKey,
			"KEY=" + secretName,
			"FROMKEY=" + secretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret content")
		data, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, "secret", generatedSecretName, "-o=jsonpath={.data."+secretKey+"}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(data).NotTo(o.BeEmpty())

		newValue := getRandomString(8)
		exutil.By("Create a pushsecret to push the secret")
		err = oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", pushSecretName, "--from-literal=secret-access-key="+newValue).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		pushSecretTemplate := filepath.Join(buildPruningBaseDir, "pushsecret-aws.yaml")
		params = []string{"-f", pushSecretTemplate, "-p",
			"NAME=" + pushSecretName,
			"REFREASHINTERVAL=" + "10s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + pushSecretName,
			"KEY=" + secretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForPushSecretStatus(oc, ns, pushSecretName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "pushsecret", pushSecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for pushsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret content")
		waitForSecretUpdate(oc, ns, generatedSecretName, secretKey, newValue)

	})

	// author: jitli@redhat.com
	g.It("Author:jitli-ROSA-Medium-81813-Use Generator to dynamically generate and push password", func() {

		exutil.SkipIfPlatformTypeNot(oc, "AWS")
		if exutil.IsSTSCluster(oc) {
			g.Skip("Skip for STS cluster")
		}
		exutil.SkipOnProxyCluster(oc)

		const (
			awsSecretName              = "aws-creds"
			secretstoreName            = "secretstore-81813"
			pushSecretName             = "pushsecret-81813"
			secretRegion               = "us-east-2"
			parameterName              = "Parameter-81813"
			secretKey                  = "password"
			clustergeneratorName       = "clustergenerator-81813"
			externalsecretName         = "externalsecret-81813"
			generatedClusterSecretName = "secret-from-clustergenerator"
		)
		ns := oc.Namespace()

		defer func() {
			e2e.Logf("Cleanup the cluster generator")
			err := oc.AsAdmin().Run("delete").Args("clustergenerator", clustergeneratorName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		exutil.By("Create a kind password cluster generator")
		generatorTemplate := filepath.Join(buildPruningBaseDir, "clustergenerator-password.yaml")
		params := []string{"-f", generatorTemplate, "-p", "NAME=" + clustergeneratorName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)

		exutil.By("Create external secret")
		externalSecretTemplate := filepath.Join(buildPruningBaseDir, "externalsecret-generator.yaml")
		params = []string{"-f", externalSecretTemplate, "-p",
			"NAME=" + externalsecretName,
			"REFREASHINTERVAL=" + "10s",
			"SECRETNAME=" + generatedClusterSecretName,
			"GENERATORKIND=" + "ClusterGenerator",
			"GENERATOR=" + clustergeneratorName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err := waitForResourceReadiness(oc, ns, "externalsecret", externalsecretName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "externalsecret", externalsecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for externalsecret to become Ready")

		exutil.By("Get secret value")
		decodedValue, err := getSecretValueDecoded(oc, ns, generatedClusterSecretName, secretKey)
		o.Expect(err).NotTo(o.HaveOccurred())
		decodedStr := string(decodedValue)
		e2e.Logf("Decoded secret value: %s", decodedStr)
		o.Expect(len(decodedStr)).To(o.Equal(16), "expected generated password to have length 16")

		exutil.By("Create secret that contains AWS accessKey")
		defer func() {
			e2e.Logf("Cleanup the created secret")
			err := oc.AsAdmin().Run("delete").Args("-n", ns, "secret", awsSecretName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		accessKeyID, secureKey := getCredentialFromCluster(oc, "aws")
		oc.NotShowInfo()
		err = oc.AsAdmin().Run("create").Args("-n", ns, "secret", "generic", awsSecretName, "--from-literal=access-key="+accessKeyID, "--from-literal=secret-access-key="+secureKey).Execute()
		oc.SetShowInfo()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.By("Create secret store")
		secretStoreTemplate := filepath.Join(buildPruningBaseDir, "secretstore-awssm.yaml")
		params = []string{"-f", secretStoreTemplate, "-p",
			"NAME=" + secretstoreName,
			"SERVICE=" + "ParameterStore",
			"REGION=" + secretRegion,
			"SECRETNAME=" + awsSecretName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForResourceReadiness(oc, ns, "secretstore", secretstoreName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "secretstore", secretstoreName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for secretstore to become Ready")

		exutil.By("Create a PushSecret use this Generator as source")
		pushSecretTemplate := filepath.Join(buildPruningBaseDir, "pushsecret-aws-secretkey.yaml")
		params = []string{"-f", pushSecretTemplate, "-p",
			"NAME=" + pushSecretName,
			"REFREASHINTERVAL=" + "10s",
			"SECRETSTORENAME=" + secretstoreName,
			"SECRETNAME=" + generatedClusterSecretName,
			"SECRETKEY=" + secretKey,
			"KEY=" + parameterName}
		exutil.ApplyNsResourceFromTemplate(oc, ns, params...)
		err = waitForPushSecretStatus(oc, ns, pushSecretName, 10*time.Second, 60*time.Second)
		if err != nil {
			dumpResource(oc, ns, "pushsecret", pushSecretName, "-o=yaml")
		}
		exutil.AssertWaitPollNoErr(err, "timeout waiting for pushsecret to become Ready")

		exutil.By("Check the secret exists and verify the secret content")
		value, err := GetSecretAWSPS(accessKeyID, secureKey, secretRegion, parameterName)
		if err != nil && !strings.Contains(value, secretKey) {
			e2e.Failf("Failed to get secret %v. or value not correct %v,%v", err, value, secretKey)
		}

		exutil.By("Check the parameter value be updated")
		errWait := wait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
			value, err = GetSecretAWSPS(accessKeyID, secureKey, secretRegion, parameterName)
			if err != nil {
				e2e.Logf("Error fetching secret: %v", err)
				return false, nil
			}
			if !strings.Contains(value, decodedStr) {
				e2e.Logf("Value not correct %v,%v", value, decodedStr)
				return true, nil
			}
			return false, nil
		})
		exutil.AssertWaitPollNoErr(errWait, "Error parameter store not updated")

	})

})
