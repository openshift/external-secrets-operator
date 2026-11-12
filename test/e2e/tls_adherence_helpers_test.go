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
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	externalsecrets "github.com/openshift/external-secrets-operator/pkg/controller/external_secrets"
	"github.com/openshift/external-secrets-operator/test/utils"
)

const (
	apiServerClusterName = "cluster"

	operatorMetricsServiceName = "external-secrets-operator-controller-manager-metrics-service"
	operatorMetricsPort        = 8443
	webhookServiceName         = "external-secrets-webhook"
	webhookServicePort         = 443
	bitwardenServicePort       = 9998

	tlsAdherenceHealthyTimeout = 5 * time.Minute
	tlsAdherencePollInterval   = 5 * time.Second
	opensslProbeTimeout        = 20 * time.Second
)

// tlsEndpoint describes a TLS server probed via oc port-forward + openssl s_client.
type tlsEndpoint struct {
	name          string
	namespace     string
	resource      string // e.g. svc/external-secrets-webhook
	containerPort int
}

// opensslProbeExpectation describes accept/reject openssl handshake checks for a profile.
type opensslProbeExpectation struct {
	// rejectTLS11 expects a TLS 1.1 attempt to be rejected by the server when the
	// local openssl client can offer TLS 1.1; otherwise the probe is skipped.
	rejectTLS11 bool
	// acceptTLS12 expects a successful TLSv1.2 handshake.
	acceptTLS12 bool
	// acceptTLS13 expects a successful TLSv1.3 handshake.
	acceptTLS13 bool
}

var (
	intermediateOpenSSLExpectation = opensslProbeExpectation{
		rejectTLS11: true,
		acceptTLS12: true,
		acceptTLS13: true,
	}
	// Modern and Custom(TLS1.3) both require TLS 1.3 minimum.
	tls13OnlyOpenSSLExpectation = opensslProbeExpectation{
		rejectTLS11: true,
		acceptTLS12: false,
		acceptTLS13: true,
	}
)

func getClusterAPIServer(ctx context.Context) (*configv1.APIServer, error) {
	apiServer := &configv1.APIServer{}
	if err := suiteRuntimeClient.Get(ctx, types.NamespacedName{Name: apiServerClusterName}, apiServer); err != nil {
		return nil, err
	}
	return apiServer, nil
}

// updateAPIServerTLS patches tlsAdherence and tlsSecurityProfile on APIServer/cluster.
func updateAPIServerTLS(ctx context.Context, adherence configv1.TLSAdherencePolicy, profile *configv1.TLSSecurityProfile) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		apiServer := &configv1.APIServer{}
		if err := suiteRuntimeClient.Get(ctx, types.NamespacedName{Name: apiServerClusterName}, apiServer); err != nil {
			return err
		}
		apiServer.Spec.TLSAdherence = adherence
		if profile == nil {
			apiServer.Spec.TLSSecurityProfile = nil
		} else {
			apiServer.Spec.TLSSecurityProfile = profile.DeepCopy()
		}
		return suiteRuntimeClient.Update(ctx, apiServer)
	})
}

// restoreAPIServerTLS restores a previously captured TLS config.
// tlsAdherence cannot be cleared once set (CEL), so an originally-empty adherence
// is restored as LegacyAdheringComponentsOnly.
func restoreAPIServerTLS(ctx context.Context, original *configv1.APIServer) error {
	adherence := original.Spec.TLSAdherence
	if adherence == configv1.TLSAdherencePolicyNoOpinion {
		adherence = configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly
	}
	return updateAPIServerTLS(ctx, adherence, original.Spec.TLSSecurityProfile)
}

// waitOperatorAndOperandsHealthy fails the spec via Eventually if health checks do not pass.
func waitOperatorAndOperandsHealthy(ctx context.Context) {
	By("Waiting for operator Deployment rolled out")
	waitDeploymentRolledOut(ctx, operatorNamespace, operatorDeploymentName)

	By("Waiting for operator pods Ready")
	Eventually(func() error {
		return utils.VerifyPodsReadyByPrefix(ctx, suiteClientset, operatorNamespace, []string{operatorPodPrefix})
	}, tlsAdherenceHealthyTimeout, tlsAdherencePollInterval).Should(Succeed())

	By("Waiting for ExternalSecretsConfig Ready")
	Eventually(func() error {
		return utils.WaitForExternalSecretsConfigReady(ctx, suiteDynamicClient, common.ExternalSecretsConfigObjectName, time.Minute)
	}, tlsAdherenceHealthyTimeout, tlsAdherencePollInterval).Should(Succeed())

	esc := &operatorv1alpha1.ExternalSecretsConfig{}
	Expect(suiteRuntimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc)).To(Succeed())

	By("Waiting for operand Deployments rolled out")
	waitOperandDeploymentsRolledOut(ctx, esc)

	By("Waiting for operand pods Ready")
	Eventually(func() error {
		return utils.VerifyOperandPodsReady(ctx, suiteClientset, operandNamespace, esc)
	}, tlsAdherenceHealthyTimeout, tlsAdherencePollInterval).Should(Succeed())
}

func deploymentRolledOut(dep *appsv1.Deployment) bool {
	if dep == nil {
		return false
	}
	want := int32(1)
	if dep.Spec.Replicas != nil {
		want = *dep.Spec.Replicas
	}
	s := dep.Status
	return s.ObservedGeneration >= dep.Generation &&
		s.UpdatedReplicas == want &&
		s.Replicas == want &&
		s.AvailableReplicas == want
}

func waitDeploymentRolledOut(ctx context.Context, namespace, name string) {
	Eventually(func(g Gomega) {
		dep, err := suiteClientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred(), "get deployment %s/%s", namespace, name)
		g.Expect(deploymentRolledOut(dep)).To(BeTrue(),
			"deployment %s/%s should be fully rolled out (gen=%d observed=%d updated=%d replicas=%d available=%d)",
			namespace, name, dep.Generation, dep.Status.ObservedGeneration,
			dep.Status.UpdatedReplicas, dep.Status.Replicas, dep.Status.AvailableReplicas)
	}, tlsAdherenceHealthyTimeout, tlsAdherencePollInterval).Should(Succeed())
}

func waitOperandDeploymentsRolledOut(ctx context.Context, esc *operatorv1alpha1.ExternalSecretsConfig) {
	for _, t := range operandTLSTargets(esc) {
		waitDeploymentRolledOut(ctx, operandNamespace, t.deployment)
	}
}

type operandTLSTarget struct {
	deployment string
	container  string
}

func operandTLSTargets(esc *operatorv1alpha1.ExternalSecretsConfig) []operandTLSTarget {
	targets := []operandTLSTarget{
		{deployment: externalsecrets.OperandCoreControllerDeployment, container: externalsecrets.OperandCoreControllerContainer},
		{deployment: externalsecrets.OperandWebhookDeployment, container: externalsecrets.OperandWebhookContainer},
		{deployment: externalsecrets.OperandBitwardenSDKServerDeployment, container: externalsecrets.OperandBitwardenContainer},
	}
	if utils.IsCertControllerExpected(esc) {
		targets = append(targets, operandTLSTarget{
			deployment: externalsecrets.OperandCertControllerDeployment,
			container:  externalsecrets.OperandCertControllerContainer,
		})
	}
	return targets
}

func assertOperandTLSArgsAbsent(ctx context.Context) {
	esc := &operatorv1alpha1.ExternalSecretsConfig{}
	Expect(suiteRuntimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc)).To(Succeed())

	Eventually(func(g Gomega) {
		for _, t := range operandTLSTargets(esc) {
			dep, err := suiteClientset.AppsV1().Deployments(operandNamespace).Get(ctx, t.deployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred(), "get deployment %s", t.deployment)
			args, found := getDeploymentContainerArgs(dep, t.container)
			g.Expect(found).To(BeTrue(), "container %s in %s", t.container, t.deployment)
			for _, arg := range args {
				g.Expect(arg).NotTo(HavePrefix("--tls-min-version="),
					"%s/%s should not have --tls-min-version when adherence is off; args=%v", t.deployment, t.container, args)
				g.Expect(arg).NotTo(HavePrefix("--tls-ciphers="),
					"%s/%s should not have --tls-ciphers when adherence is off; args=%v", t.deployment, t.container, args)
			}
		}
	}, tlsAdherenceHealthyTimeout, tlsAdherencePollInterval).Should(Succeed())

	// Spec may update before pods roll; wait so later probes hit the new ReplicaSet.
	By("Waiting for operator and operand Deployments to finish rolling after TLS arg change")
	waitDeploymentRolledOut(ctx, operatorNamespace, operatorDeploymentName)
	waitOperandDeploymentsRolledOut(ctx, esc)
}

func assertOperandTLSArgsMatch(ctx context.Context, wantMinVersion string, wantCipherSubstring string, expectCiphers bool) {
	esc := &operatorv1alpha1.ExternalSecretsConfig{}
	Expect(suiteRuntimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc)).To(Succeed())

	Eventually(func(g Gomega) {
		for _, t := range operandTLSTargets(esc) {
			dep, err := suiteClientset.AppsV1().Deployments(operandNamespace).Get(ctx, t.deployment, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred(), "get deployment %s", t.deployment)
			args, found := getDeploymentContainerArgs(dep, t.container)
			g.Expect(found).To(BeTrue(), "container %s in %s", t.container, t.deployment)

			minFound := false
			ciphersFound := false
			for _, arg := range args {
				if arg == "--tls-min-version="+wantMinVersion {
					minFound = true
				}
				if strings.HasPrefix(arg, "--tls-ciphers=") {
					ciphersFound = true
					if wantCipherSubstring != "" {
						g.Expect(arg).To(ContainSubstring(wantCipherSubstring),
							"%s/%s --tls-ciphers should contain %q; got %q", t.deployment, t.container, wantCipherSubstring, arg)
					}
				}
			}
			g.Expect(minFound).To(BeTrue(),
				"%s/%s should include --tls-min-version=%s; args=%v", t.deployment, t.container, wantMinVersion, args)
			if expectCiphers {
				g.Expect(ciphersFound).To(BeTrue(),
					"%s/%s should include --tls-ciphers; args=%v", t.deployment, t.container, args)
			} else {
				g.Expect(ciphersFound).To(BeFalse(),
					"%s/%s should not include --tls-ciphers; args=%v", t.deployment, t.container, args)
			}
		}
	}, tlsAdherenceHealthyTimeout, tlsAdherencePollInterval).Should(Succeed())

	// Spec may update before pods roll; wait so openssl probes hit the new ReplicaSet.
	By("Waiting for operator and operand Deployments to finish rolling after TLS arg change")
	waitDeploymentRolledOut(ctx, operatorNamespace, operatorDeploymentName)
	waitOperandDeploymentsRolledOut(ctx, esc)
}

func tlsProbeEndpoints() []tlsEndpoint {
	return []tlsEndpoint{
		{
			name:          "operator-metrics",
			namespace:     operatorNamespace,
			resource:      "svc/" + operatorMetricsServiceName,
			containerPort: operatorMetricsPort,
		},
		{
			name:          "webhook",
			namespace:     operandNamespace,
			resource:      "svc/" + webhookServiceName,
			containerPort: webhookServicePort,
		},
		{
			name:          "bitwarden-sdk-server",
			namespace:     operandNamespace,
			resource:      "svc/" + utils.BitwardenSDKServerServiceName,
			containerPort: bitwardenServicePort,
		},
	}
}

func assertOpenSSLProbes(ctx context.Context, expect opensslProbeExpectation) {
	// Ensure rollouts finished before port-forwarding to Services (old pods can still be endpoints).
	esc := &operatorv1alpha1.ExternalSecretsConfig{}
	Expect(suiteRuntimeClient.Get(ctx, client.ObjectKey{Name: common.ExternalSecretsConfigObjectName}, esc)).To(Succeed())
	waitDeploymentRolledOut(ctx, operatorNamespace, operatorDeploymentName)
	waitOperandDeploymentsRolledOut(ctx, esc)

	for _, ep := range tlsProbeEndpoints() {
		By(fmt.Sprintf("openssl probes against %s (%s)", ep.name, ep.resource))
		func() {
			localPort, stop := startPortForward(ctx, ep)
			defer stop()

			if expect.rejectTLS11 {
				// Offer ECDSA and RSA TLS1.1 suites: bitwarden e2e cert is ECDSA; operator/webhook are typically RSA.
				// AES128-SHA alone would fail cipher selection on ECDSA endpoints and look like a version reject.
				out := runOpenSSLClient(ctx, localPort, "-tls1_1", "-cipher", "ECDHE-ECDSA-AES128-SHA:AES128-SHA:@SECLEVEL=0")
				if opensslClientCannotNegotiate(out) {
					By(fmt.Sprintf("Skipping TLS1.1 rejection probe for %s: local openssl cannot offer TLS1.1", ep.name))
				} else {
					Expect(opensslHandshakeSucceeded(out)).To(BeFalse(),
						"%s must not complete a TLS1.1 handshake; openssl output:\n%s", ep.name, out)
					Expect(opensslServerRejected(out)).To(BeTrue(),
						"%s should reject TLS1.1 with a server-side failure (not a client/transport error); openssl output:\n%s", ep.name, out)
				}
			}
			if expect.acceptTLS12 {
				out := runOpenSSLClient(ctx, localPort, "-tls1_2")
				Expect(opensslHandshakeSucceeded(out)).To(BeTrue(),
					"%s should accept TLS1.2; openssl output:\n%s", ep.name, out)
				Expect(opensslNegotiatedVersion(out, "1.2")).To(BeTrue(),
					"%s should negotiate TLSv1.2 (Protocol line or New, TLSv1.2 summary); openssl output:\n%s", ep.name, out)
			} else {
				out := runOpenSSLClient(ctx, localPort, "-tls1_2")
				Expect(opensslClientCannotNegotiate(out)).To(BeFalse(),
					"local openssl must be able to offer TLS1.2 to validate rejection; openssl output:\n%s", out)
				Expect(opensslHandshakeSucceeded(out)).To(BeFalse(),
					"%s must not complete a TLS1.2 handshake; openssl output:\n%s", ep.name, out)
				Expect(opensslServerRejected(out)).To(BeTrue(),
					"%s should reject TLS1.2 with a server-side failure (not a client/transport error); openssl output:\n%s", ep.name, out)
			}
			if expect.acceptTLS13 {
				out := runOpenSSLClient(ctx, localPort, "-tls1_3")
				Expect(opensslHandshakeSucceeded(out)).To(BeTrue(),
					"%s should accept TLS1.3; openssl output:\n%s", ep.name, out)
				Expect(opensslNegotiatedVersion(out, "1.3")).To(BeTrue(),
					"%s should negotiate TLSv1.3 (Protocol line or New, TLSv1.3 summary); openssl output:\n%s", ep.name, out)
			}
		}()
	}
}

// syncBuffer is a mutex-protected bytes.Buffer for concurrent cmd stderr reads.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func startPortForward(ctx context.Context, ep tlsEndpoint) (localPort int, stop func()) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	Expect(err).NotTo(HaveOccurred(), "allocate local port for %s", ep.name)
	localPort = listener.Addr().(*net.TCPAddr).Port
	Expect(listener.Close()).To(Succeed())

	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, "oc", "port-forward", "-n", ep.namespace,
		ep.resource, fmt.Sprintf("%d:%d", localPort, ep.containerPort))
	stderr := &syncBuffer{}
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = stderr
	Expect(cmd.Start()).To(Succeed(), "start port-forward for %s", ep.name)

	// Idempotent: DeferCleanup and caller defer stop() may both run.
	var stopOnce sync.Once
	stop = func() {
		stopOnce.Do(func() {
			cancel()
			if waitErr := cmd.Wait(); waitErr != nil && !errors.Is(waitErr, context.Canceled) {
				fmt.Fprintf(GinkgoWriter, "port-forward for %s wait: %v\n", ep.name, waitErr)
			}
		})
	}
	DeferCleanup(stop)

	Eventually(func() error {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), time.Second)
		if err != nil {
			return fmt.Errorf("%w; port-forward stderr=%s", err, stderr.String())
		}
		if closeErr := conn.Close(); closeErr != nil {
			return fmt.Errorf("close readiness dial: %w; port-forward stderr=%s", closeErr, stderr.String())
		}
		return nil
	}, 30*time.Second, 500*time.Millisecond).Should(Succeed(),
		"port-forward for %s should listen on 127.0.0.1:%d", ep.name, localPort)

	return localPort, stop
}

func runOpenSSLClient(ctx context.Context, localPort int, extraArgs ...string) string {
	ctx, cancel := context.WithTimeout(ctx, opensslProbeTimeout)
	defer cancel()

	args := []string{"s_client", "-connect", fmt.Sprintf("127.0.0.1:%d", localPort), "-servername", "localhost"}
	args = append(args, extraArgs...)
	cmd := exec.CommandContext(ctx, "openssl", args...)
	cmd.Stdin = bytes.NewReader(nil)
	cmd.WaitDelay = 5 * time.Second
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	// Non-zero exit is expected for rejected handshakes; callers parse the output.
	// Surface timeout/cancel distinctly so negative probes do not treat a hung client as rejection.
	err := cmd.Run()
	out := combined.String()
	if err != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded)) {
		return out + "\n[openssl probe timed out]\n"
	}
	if err != nil && (errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled)) {
		return out + "\n[openssl probe canceled]\n"
	}
	return out
}

// opensslProtocolLine matches SSL-Session "Protocol  : TLSv1.x" and terse "Protocol: TLSv1.x".
var opensslProtocolLine = regexp.MustCompile(`(?i)protocol\s*:\s*tlsv`)

// opensslNewSummary matches OpenSSL 3.x "New, TLSv1.x, Cipher is ..." (TLS 1.3 often omits SSL-Session Protocol).
var opensslNewSummary = regexp.MustCompile(`(?i)new,\s*tlsv\d`)

func opensslHandshakeSucceeded(out string) bool {
	lower := strings.ToLower(out)
	if strings.Contains(lower, "cipher is (none)") || strings.Contains(lower, "cipher    : 0000") {
		return false
	}
	if opensslServerRejected(out) || opensslClientCannotNegotiate(out) {
		return false
	}
	return opensslProtocolLine.MatchString(out) || opensslNewSummary.MatchString(out)
}

// opensslNegotiatedVersion reports whether openssl shows the given minor version (e.g. "1.2", "1.3")
// via a Protocol line or New, TLSv summary.
func opensslNegotiatedVersion(out, minor string) bool {
	re := regexp.MustCompile(`(?i)(?:protocol\s*:\s*tlsv|new,\s*tlsv)` + regexp.QuoteMeta(minor) + `\b`)
	return re.MatchString(out)
}

// opensslClientCannotNegotiate reports client-side crypto-policy / protocol availability failures
// (the probe never meaningfully reached a server TLS decision).
func opensslClientCannotNegotiate(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "no protocols available") ||
		strings.Contains(lower, "unsupported protocol") ||
		strings.Contains(lower, "no ciphers available") ||
		strings.Contains(lower, "no cipher match")
}

// opensslServerRejected reports evidence the peer rejected the handshake.
// Broader than alert-only matching: Go crypto/tls and openssl wording varies.
func opensslServerRejected(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "alert protocol version") ||
		strings.Contains(lower, "alert handshake failure") ||
		strings.Contains(lower, "handshake failure") ||
		strings.Contains(lower, "tlsv1 alert") ||
		strings.Contains(lower, "sslv3 alert") ||
		(strings.Contains(lower, "ssl routines") && strings.Contains(lower, "wrong version number"))
}

func intermediateTLSProfile() *configv1.TLSSecurityProfile {
	return &configv1.TLSSecurityProfile{
		Type:         configv1.TLSProfileIntermediateType,
		Intermediate: &configv1.IntermediateTLSProfile{},
	}
}

func modernTLSProfile() *configv1.TLSSecurityProfile {
	return &configv1.TLSSecurityProfile{
		Type:   configv1.TLSProfileModernType,
		Modern: &configv1.ModernTLSProfile{},
	}
}

// customTLS13Profile is distinguishable from Intermediate: TLS 1.3 only, no --tls-ciphers.
// OpenShift rejects TLS 1.3 suites in custom.ciphers ("TLS 1.3 cipher suites are not
// configurable"); leave Ciphers empty so the API accepts VersionTLS13.
func customTLS13Profile() *configv1.TLSSecurityProfile {
	return &configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileCustomType,
		Custom: &configv1.CustomTLSProfile{
			TLSProfileSpec: configv1.TLSProfileSpec{
				MinTLSVersion: configv1.VersionTLS13,
			},
		},
	}
}

func requireOpenSSLAndOC() {
	_, err := exec.LookPath("openssl")
	Expect(err).NotTo(HaveOccurred(), "openssl must be in PATH for Feature:TLSAdherence e2e")
	_, err = exec.LookPath("oc")
	Expect(err).NotTo(HaveOccurred(), "oc must be in PATH for Feature:TLSAdherence e2e")
}
