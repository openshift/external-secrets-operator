package tlsprofile

import (
	"crypto/tls"
	"crypto/x509"
	"sync"

	configv1 "github.com/openshift/api/config/v1"
)

// config caches the cluster TLS profile's resolved minimum version, cipher
// suites, and curve preferences, plus the operand --tls-* args derived from
// them, so neither the operator's own metrics server TLS config nor the
// operand args are re-resolved on every use. SetTLSConfig is the only writer;
// configured is false until it's called with a non-nil spec.
type config struct {
	sync.RWMutex

	configured       bool
	minVersion       uint16
	cipherSuites     []uint16
	curvePreferences []tls.CurveID
	operandTLSArgs   []string
}

var cfg = new(config)

// SetTLSConfig resolves spec into the cached TLS settings and the operand
// --tls-min-version/--tls-ciphers args derived from them. A nil spec clears
// the cache: ClientTLSConfig/ServerTLSConfig/GetOperandTLSArgs then report
// nothing configured.
//
// Callers decide whether the profile should be honored at all using
// github.com/openshift/library-go/pkg/crypto.ShouldHonorClusterTLSProfile
// before calling this with a non-nil spec.
func SetTLSConfig(spec *configv1.TLSProfileSpec) error {
	cfg.Lock()
	defer cfg.Unlock()

	if spec == nil {
		cfg.configured = false
		cfg.minVersion = 0
		cfg.cipherSuites = nil
		cfg.curvePreferences = nil
		cfg.operandTLSArgs = nil
		return nil
	}

	tlsConfig, err := getTLSConfig(spec)
	if err != nil {
		return err
	}
	cfg.configured = true
	cfg.minVersion = tlsConfig.MinVersion
	cfg.cipherSuites = tlsConfig.CipherSuites
	cfg.curvePreferences = tlsConfig.CurvePreferences
	cfg.operandTLSArgs = operandTLSArgsFromConfig(tlsConfig)
	return nil
}

// GetOperandTLSArgs returns the --tls-min-version/--tls-ciphers args for the
// honored cluster TLS profile, or nil if none is configured.
func GetOperandTLSArgs() []string {
	cfg.RLock()
	defer cfg.RUnlock()
	return cfg.operandTLSArgs
}

// baseTLSConfig returns a tls.Config with the minimum protocol version,
// cipher suites, and curve preferences resolved by SetTLSConfig, or nil if no
// cluster TLS profile is configured. Callers set whichever of
// RootCAs/ClientCAs their role (client vs server) actually consults.
func baseTLSConfig() *tls.Config {
	cfg.RLock()
	defer cfg.RUnlock()
	if !cfg.configured {
		return nil
	}
	return &tls.Config{
		MinVersion:       cfg.minVersion,
		CipherSuites:     cfg.cipherSuites,
		CurvePreferences: cfg.curvePreferences,
	}
}

// ClientTLSConfig returns a tls.Config for outbound HTTPS/TLS clients, or nil
// if no cluster TLS profile is configured. rootCAs, if non-nil, is used to
// verify the remote server's certificate chain.
func ClientTLSConfig(rootCAs *x509.CertPool) *tls.Config {
	tlsConfig := baseTLSConfig()
	if tlsConfig == nil {
		return nil
	}
	tlsConfig.RootCAs = rootCAs
	return tlsConfig
}

// ServerTLSConfig returns a tls.Config for serving HTTPS/TLS, or nil if no
// cluster TLS profile is configured. clientCAs, if non-nil, is used to verify
// client certificates presented during the handshake (mTLS).
func ServerTLSConfig(clientCAs *x509.CertPool) *tls.Config {
	tlsConfig := baseTLSConfig()
	if tlsConfig == nil {
		return nil
	}
	tlsConfig.ClientCAs = clientCAs
	return tlsConfig
}
