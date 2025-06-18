package commontest

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

const (
	// TestExternalSecretsResourceName is the name for ExternalSecrets test CR.
	TestExternalSecretsResourceName = "cluster"

	// TestExternalSecretsImageName is the sample image name for external-secrets operand.
	TestExternalSecretsImageName = "registry.redhat.io/external-secrets-operator/external-secrets-operator-rhel9"

	// TestBitwardenImageName is the sample image name for bitwarden-sdk-server.
	TestBitwardenImageName = "registry.stage.redhat.io/external-secrets-operator/bitwarden-sdk-server-rhel9"

	// TestExternalSecretsNamespace is the sample namespace name for external-secrets deployment.
	TestExternalSecretsNamespace = "test-external-secrets"

	// TestCRDName can be used for sample CRD resources.
	TestCRDName = "test-crd"
)

var (
	// TestClientError is the error to return for client failure scenarios.
	TestClientError = fmt.Errorf("test client error")
)

// TestExternalSecrets returns a sample ExternalSecrets object.
func TestExternalSecrets() *operatorv1alpha1.ExternalSecrets {
	return &operatorv1alpha1.ExternalSecrets{
		ObjectMeta: metav1.ObjectMeta{
			Name: TestExternalSecretsResourceName,
		},
	}
}

// TestExternalSecretsManager returns a sample ExternalSecretsManager object.
func TestExternalSecretsManager() *operatorv1alpha1.ExternalSecretsManager {
	return &operatorv1alpha1.ExternalSecretsManager{
		ObjectMeta: metav1.ObjectMeta{
			Name: TestExternalSecretsResourceName,
		},
	}
}
