package tlsprofile

import (
	"crypto/tls"
	"fmt"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	crtls "github.com/openshift/controller-runtime-common/pkg/tls"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("tlsprofile")

// DefaultCurvePreferences is the explicit key-exchange curve order for TLS
// clients (and can be reused for servers) when the cluster TLS profile API does
// not yet expose curve preferences.
var DefaultCurvePreferences = []tls.CurveID{
	tls.X25519,
	tls.CurveP256,
	tls.CurveP384,
	tls.CurveP521,
}

// knownGoUnsupportedCiphers lists the exact OpenSSL cipher names that Go's
// crypto/tls standard library has never implemented, even though they are
// legitimate, well-known ciphers that appear in OpenShift's builtin "Old" TLS
// profile (following Mozilla's compatibility guidelines).
var knownGoUnsupportedCiphers = map[string]bool{
	"ECDHE-ECDSA-AES256-SHA384": true,
	"ECDHE-RSA-AES256-SHA384":   true,
	"AES256-SHA256":             true,
}

// getTLSConfig resolves spec into a tls.Config: minimum TLS version, cipher
// suites, and curve preferences. RootCAs/ClientCAs are left unset - those are
// a per-role (client vs server) concern, not part of what the cluster TLS
// profile resolves to.
func getTLSConfig(spec *configv1.TLSProfileSpec) (*tls.Config, error) {
	if spec == nil {
		return nil, fmt.Errorf("TLS profile spec is nil")
	}

	// NewTLSConfigFromProfile applies the profile's minimum TLS version and
	// (below TLS 1.3) cipher suites to the returned tls.Config.
	configure, reported := crtls.NewTLSConfigFromProfile(*spec)

	var badCiphers, unsupportedCiphers []string
	for _, name := range reported {
		if knownGoUnsupportedCiphers[name] {
			unsupportedCiphers = append(unsupportedCiphers, name)
			continue
		}
		badCiphers = append(badCiphers, name)
	}
	if len(badCiphers) > 0 {
		return nil, fmt.Errorf("unsupported TLS cipher/group names in profile: %v", badCiphers)
	}
	if len(unsupportedCiphers) > 0 {
		log.Info("cluster TLS profile includes ciphers unsupported by Go, they will be skipped",
			"unsupportedCiphers", unsupportedCiphers)
	}

	tlsConfig := &tls.Config{
		// The cluster TLS profile API does not yet expose curve preferences, so
		// default them here; NewTLSConfigFromProfile only overrides this when the
		// profile sets explicit groups.
		CurvePreferences: append([]tls.CurveID(nil), DefaultCurvePreferences...),
	}
	configure(tlsConfig)
	return tlsConfig, nil
}

func tlsVersionString(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "1.0"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS13:
		return "1.3"
	default:
		return ""
	}
}

// operandTLSArgsFromConfig derives the --tls-min-version/--tls-ciphers args
// the external-secrets operand understands from a resolved tls.Config.
func operandTLSArgsFromConfig(tlsConfig *tls.Config) []string {
	if tlsConfig == nil {
		return nil
	}
	var args []string
	if minVersion := tlsVersionString(tlsConfig.MinVersion); minVersion != "" {
		args = append(args, "--tls-min-version="+minVersion)
	}
	if tlsConfig.MinVersion != tls.VersionTLS13 && len(tlsConfig.CipherSuites) > 0 {
		names := make([]string, 0, len(tlsConfig.CipherSuites))
		for _, id := range tlsConfig.CipherSuites {
			names = append(names, tls.CipherSuiteName(id))
		}
		args = append(args, "--tls-ciphers="+strings.Join(names, ","))
	}
	return args
}
