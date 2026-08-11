package tlsprofile

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	crtls "github.com/openshift/controller-runtime-common/pkg/tls"
)

// DefaultCurvePreferences is the explicit key-exchange curve order for TLS
// clients (and can be reused for servers) when the cluster TLS profile API does
// not yet expose curve preferences.
var DefaultCurvePreferences = []tls.CurveID{
	tls.X25519,
	tls.CurveP256,
	tls.CurveP384,
	tls.CurveP521,
}

// ClientTLSConfig returns a tls.Config for outbound HTTPS/TLS clients using
// the same minimum protocol version, cipher suites, and curve preferences as
// the given resolved TLS profile spec.
func ClientTLSConfig(spec *configv1.TLSProfileSpec, rootCAs *x509.CertPool) (*tls.Config, error) {
	if spec == nil {
		return nil, fmt.Errorf("TLS profile spec is nil")
	}

	// NewTLSConfigFromProfile applies the profile's minimum TLS version and
	// (below TLS 1.3) cipher suites to the returned tls.Config.
	configure, unsupported := crtls.NewTLSConfigFromProfile(*spec)
	if len(unsupported) > 0 {
		return nil, fmt.Errorf("unsupported TLS cipher/group names in profile: %v", unsupported)
	}

	tlsConfig := &tls.Config{
		RootCAs: rootCAs,
		// The cluster TLS profile API does not yet expose curve preferences, so
		// default them here; NewTLSConfigFromProfile only overrides this when the
		// profile sets explicit groups.
		CurvePreferences: append([]tls.CurveID(nil), DefaultCurvePreferences...),
	}
	configure(tlsConfig)
	return tlsConfig, nil
}
