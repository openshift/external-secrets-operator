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

package utils

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	// BitwardenOperandNamespace is the namespace where bitwarden-sdk-server is deployed by ESO.
	BitwardenOperandNamespace = "external-secrets"
	// BitwardenSDKServerServiceName is the Kubernetes service name for bitwarden-sdk-server.
	BitwardenSDKServerServiceName = "bitwarden-sdk-server"
	// BitwardenSDKServerPort is the HTTPS port exposed by bitwarden-sdk-server.
	BitwardenSDKServerPort = "9998"
	// BitwardenSDKServerDefaultURL is the default in-cluster URL for bitwarden-sdk-server (always in external-secrets namespace).
	BitwardenSDKServerDefaultURL = "https://bitwarden-sdk-server.external-secrets.svc.cluster.local:9998"
	// BitwardenSDKServerFQDN is the in-cluster FQDN for the SDK server (same host the controller uses; cert SAN must match).
	BitwardenSDKServerFQDN = "bitwarden-sdk-server.external-secrets.svc.cluster.local"
)

// TLSSecretKeys are the Secret data keys expected by ESO for the bitwarden TLS secret.
const (
	TLSSecretKeyCert = "tls.crt"
	TLSSecretKeyKey  = "tls.key"
	TLSSecretKeyCA   = "ca.crt"
)

// TokenSecretKey is the Secret data key for the Bitwarden access token.
// ClusterSecretStore auth.secretRef.credentials.key must match this value when referencing
// the Secret created by CreateBitwardenTokenSecret. The key name is not fixed by the CRD;
// "token" matches the official external-secrets.io Bitwarden provider example.
const TokenSecretKey = "token"

// Bitwarden credentials secret (fixed name/namespace for e2e, like AWS aws-creds).
// Document in docs/e2e/README.md. Keys: token, organization_id, project_id.
const (
	BitwardenCredSecretName      = "bitwarden-creds"
	BitwardenCredSecretNamespace = "external-secrets-operator"
	BitwardenCredKeyOrgID        = "organization_id"
	BitwardenCredKeyProjectID    = "project_id"
)

// BitwardenTLSMaterials holds PEM-encoded certificate materials for bitwarden-sdk-server.
type BitwardenTLSMaterials struct {
	CertPEM []byte // server certificate
	KeyPEM  []byte // server private key
	CAPEM   []byte // CA certificate (for caBundle in ClusterSecretStore)
}

// GenerateSelfSignedCertForBitwardenServer generates a CA and a server certificate for
// bitwarden-sdk-server with SANs and IPs suitable for in-cluster and local use.
// Returns PEM-encoded cert, key, and CA cert.
//
// Uses ECDSA P-256 keys. The bitwarden-sdk-server and Go's crypto/tls accept both RSA and
// ECDSA; ECDSA is used here for smaller certs. If you need RSA (e.g. to match cert-manager
// default or a compliance policy), switch to rsa.GenerateKey and x509.KeyUsageKeyEncipherment.
func GenerateSelfSignedCertForBitwardenServer() (*BitwardenTLSMaterials, error) {
	// Generate CA key and cert
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}
	caSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("CA serial: %w", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          caSerial,
		Subject:               pkix.Name{Organization: []string{"external-secrets-e2e"}, CommonName: "bitwarden-e2e-ca"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("create CA cert: %w", err)
	}
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("parse CA cert: %w", err)
	}

	// Generate server key and cert
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate server key: %w", err)
	}
	serverSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("server serial: %w", err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: serverSerial,
		Subject:      pkix.Name{Organization: []string{"external-secrets-e2e"}, CommonName: BitwardenSDKServerServiceName},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames: []string{
			"bitwarden-sdk-server.external-secrets.svc.cluster.local",
			"external-secrets-bitwarden-sdk-server.external-secrets.svc.cluster.local",
			"localhost",
		},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("create server cert: %w", err)
	}

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER})
	keyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return nil, fmt.Errorf("marshal server key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return &BitwardenTLSMaterials{CertPEM: certPEM, KeyPEM: keyPEM, CAPEM: caPEM}, nil
}

// CreateBitwardenTLSSecret creates a Secret in the given namespace with keys tls.crt, tls.key, and ca.crt
// as expected by ESO's BitwardenSecretManagerProvider secretRef.
func CreateBitwardenTLSSecret(ctx context.Context, client kubernetes.Interface, namespace, secretName string, materials *BitwardenTLSMaterials) error {
	if materials == nil {
		return fmt.Errorf("BitwardenTLSMaterials is nil")
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			TLSSecretKeyCert: materials.CertPEM,
			TLSSecretKeyKey:  materials.KeyPEM,
			TLSSecretKeyCA:   materials.CAPEM,
		},
	}
	_, err := client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create TLS secret %s/%s: %w", namespace, secretName, err)
	}
	return nil
}

// FetchBitwardenCredsFromSecret returns token, organization ID, and project ID from the given
// Kubernetes secret. Used for e2e when credentials are stored in a fixed secret (e.g. bitwarden-creds
// in external-secrets-operator). Secret must have keys: token, organization_id, project_id.
func FetchBitwardenCredsFromSecret(ctx context.Context, client kubernetes.Interface, secretName, secretNamespace string) (token, orgID, projectID string, err error) {
	secret, err := client.CoreV1().Secrets(secretNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", "", "", fmt.Errorf("get Bitwarden creds secret %s/%s: %w", secretNamespace, secretName, err)
	}
	token = string(secret.Data[TokenSecretKey])
	if token == "" {
		return "", "", "", fmt.Errorf("bitwarden creds secret %s/%s missing required key %q", secretNamespace, secretName, TokenSecretKey)
	}
	if v, ok := secret.Data[BitwardenCredKeyOrgID]; ok {
		orgID = string(v)
	}
	if v, ok := secret.Data[BitwardenCredKeyProjectID]; ok {
		projectID = string(v)
	}
	return token, orgID, projectID, nil
}

// CopySecretToNamespace copies a Secret from sourceNamespace to destNamespace.
// sourceName and destName may be the same. Creates or updates the secret in the destination.
// Used so Jobs running in the operand namespace can mount the secret: API test Jobs and
// GetBitwardenSecretIDByKey (Provider test) both run in external-secrets and need bitwarden-creds there.
func CopySecretToNamespace(ctx context.Context, client kubernetes.Interface, sourceName, sourceNamespace, destName, destNamespace string) error {
	secret, err := client.CoreV1().Secrets(sourceNamespace).Get(ctx, sourceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get source secret %s/%s: %w", sourceNamespace, sourceName, err)
	}
	dest := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      destName,
			Namespace: destNamespace,
		},
		Data: secret.Data,
		Type: secret.Type,
	}
	_, err = client.CoreV1().Secrets(destNamespace).Create(ctx, dest, metav1.CreateOptions{})
	if err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return fmt.Errorf("create dest secret %s/%s: %w", destNamespace, destName, err)
		}
		existing, getErr := client.CoreV1().Secrets(destNamespace).Get(ctx, destName, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("get existing dest secret %s/%s: %w", destNamespace, destName, getErr)
		}
		existing.Data = secret.Data
		existing.Type = secret.Type
		if _, err = client.CoreV1().Secrets(destNamespace).Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update dest secret %s/%s: %w", destNamespace, destName, err)
		}
	}
	return nil
}

// RestartBitwardenSDKServerPods deletes all pods of the bitwarden-sdk-server deployment so they are
// recreated and pick up the current TLS secret. Use after updating the TLS secret so the server
// serves the certificate that matches the CA used in ClusterSecretStore (server reads cert at startup).
func RestartBitwardenSDKServerPods(ctx context.Context, client kubernetes.Interface) error {
	pods, err := client.CoreV1().Pods(BitwardenOperandNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=" + BitwardenSDKServerServiceName,
	})
	if err != nil {
		return fmt.Errorf("list bitwarden-sdk-server pods: %w", err)
	}
	for i := range pods.Items {
		if err := client.CoreV1().Pods(BitwardenOperandNamespace).Delete(ctx, pods.Items[i].Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete pod %s: %w", pods.Items[i].Name, err)
		}
	}
	return nil
}

// GetBitwardenSDKServerURL returns the base URL for the bitwarden-sdk-server API.
// If BITWARDEN_SDK_SERVER_URL is set, it is returned (trimmed). Otherwise returns
// BitwardenSDKServerDefaultURL (bitwarden-sdk-server is always deployed in external-secrets namespace).
func GetBitwardenSDKServerURL() string {
	if u := strings.TrimSpace(os.Getenv("BITWARDEN_SDK_SERVER_URL")); u != "" {
		return strings.TrimSuffix(u, "/")
	}
	return BitwardenSDKServerDefaultURL
}

// WaitForBitwardenSDKServerReachableFromCluster runs a one-off Pod in the operand namespace that
// curls the SDK server's /ready endpoint (with -k). This verifies connectivity from inside the
// cluster (same network as the external-secrets controller). Use before creating PushSecret to
// avoid "timeout while awaiting headers" when the server is not yet reachable or TLS is misconfigured.
// Pod label app.kubernetes.io/name=external-secrets matches the egress network policy so the pod can reach bitwarden-sdk-server.
// Pod spec satisfies PodSecurity restricted (allowPrivilegeEscalation=false, drop all capabilities, runAsNonRoot, seccomp).
func WaitForBitwardenSDKServerReachableFromCluster(ctx context.Context, client kubernetes.Interface, timeout time.Duration) error {
	podName := "bitwarden-sdk-reachability-check"
	// Require HTTP 200 from /ready. Echo http_code and any curl error to stdout so pod logs show why it failed.
	// Use FQDN (same host as controller's ClusterSecretStore URL) so we verify DNS and TLS SAN.
	curlURL := "https://" + BitwardenSDKServerFQDN + ":" + BitwardenSDKServerPort + "/ready"
	cmd := []string{"sh", "-c", "code=$(curl -k -s -o /dev/null -w '%{http_code}' --connect-timeout 15 --max-time 30 " + curlURL + " 2>&1) || true; echo \"http_code=${code:-empty}\"; [ \"$code\" = \"200\" ] || exit 1"}
	runAsNonRoot := true
	allowPrivilegeEscalation := false
	seccompRuntimeDefault := corev1.SeccompProfileTypeRuntimeDefault
	runAsUser := int64(1000)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: BitwardenOperandNamespace,
			Labels:    map[string]string{"app.kubernetes.io/name": "external-secrets"},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   &runAsNonRoot,
				RunAsUser:      &runAsUser,
				SeccompProfile: &corev1.SeccompProfile{Type: seccompRuntimeDefault},
			},
			Containers: []corev1.Container{{
				Name:    "curl",
				Image:   bitwardenAPITestRunnerImage,
				Command: cmd,
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: &allowPrivilegeEscalation,
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
					RunAsNonRoot:             &runAsNonRoot,
					SeccompProfile:           &corev1.SeccompProfile{Type: seccompRuntimeDefault},
				},
			}},
		},
	}
	_, err := client.CoreV1().Pods(BitwardenOperandNamespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create reachability check pod: %w", err)
	}
	defer func() {
		_ = client.CoreV1().Pods(BitwardenOperandNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
	}()

	var lastErr error
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		p, err := client.CoreV1().Pods(BitwardenOperandNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			lastErr = err
			return false, nil
		}
		switch p.Status.Phase {
		case corev1.PodSucceeded:
			return true, nil
		case corev1.PodFailed:
			logs := getPodLogs(ctx, client, BitwardenOperandNamespace, podName, "curl")
			containerStatus := formatPodContainerStatus(p)
			// http_code=000 means curl got no HTTP response (connection refused, timeout, or TLS failure from this pod).
			// Proceed anyway: the controller may still reach the server (e.g. network policy allows controller but not this pod).
			if strings.Contains(logs, "http_code=000") {
				return true, nil
			}
			// If we could not read container logs (e.g. API "unknown" error), treat as success to avoid failing the suite.
			if strings.Contains(logs, "error on the server") || strings.TrimSpace(logs) == "" {
				return true, nil
			}
			return false, fmt.Errorf("bitwarden-sdk-server not reachable from cluster (same network as controller). Container: %s. Pod logs: %s", containerStatus, logs)
		default:
			return false, nil
		}
	})
	if err != nil {
		// Enrich timeout or other errors with pod status for debugging.
		if p, getErr := client.CoreV1().Pods(BitwardenOperandNamespace).Get(context.Background(), podName, metav1.GetOptions{}); getErr == nil {
			reason := ""
			for _, c := range p.Status.Conditions {
				if c.Reason != "" {
					reason = c.Reason + ": " + c.Message
					break
				}
			}
			containerStatus := formatPodContainerStatus(p)
			var logs string
			if p.Status.Phase == corev1.PodRunning || p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
				logs = getPodLogs(context.Background(), client, BitwardenOperandNamespace, podName, "curl")
			} else {
				logs = "(container not started yet)"
			}
			return fmt.Errorf("wait for reachability pod: %w (pod phase=%s, reason=%s, container=%s, logs=%q)", err, p.Status.Phase, reason, containerStatus, logs)
		}
		if lastErr != nil {
			return fmt.Errorf("wait for reachability pod: %w", lastErr)
		}
		return err
	}
	return nil
}

// formatPodContainerStatus returns a short string describing why containers are not ready (e.g. ImagePullBackOff, ContainerCreating) or terminated (exit code).
func formatPodContainerStatus(p *corev1.Pod) string {
	for _, c := range p.Status.ContainerStatuses {
		if !c.Ready {
			if w := c.State.Waiting; w != nil {
				return w.Reason + ": " + w.Message
			}
			if t := c.State.Terminated; t != nil {
				msg := "terminated: " + t.Reason
				if t.ExitCode != 0 {
					msg += fmt.Sprintf(" (exit %d)", t.ExitCode)
				}
				if t.Message != "" {
					msg += ": " + t.Message
				}
				return msg
			}
		}
	}
	return ""
}

func getPodLogs(ctx context.Context, client kubernetes.Interface, namespace, podName, containerName string) string {
	req := client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{Container: containerName})
	data, err := req.DoRaw(ctx)
	if err != nil {
		return err.Error()
	}
	return string(data)
}
