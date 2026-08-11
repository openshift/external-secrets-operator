package tlsprofile

import (
	"crypto/tls"
	"fmt"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	crtls "github.com/openshift/controller-runtime-common/pkg/tls"
)

func cloneBuiltinProfileSpec(profileType configv1.TLSProfileType) *configv1.TLSProfileSpec {
	spec := *configv1.TLSProfiles[profileType]
	spec.Ciphers = append([]string(nil), spec.Ciphers...)
	return &spec
}

// EffectiveSpec resolves apiserver.config.openshift.io/cluster
// spec.tlsSecurityProfile into concrete cipher and minimum TLS version settings.
// A nil or empty profile follows API default semantics (Intermediate).
func EffectiveSpec(profile *configv1.TLSSecurityProfile) (*configv1.TLSProfileSpec, error) {
	if profile == nil || profile.Type == "" {
		return cloneBuiltinProfileSpec(configv1.TLSProfileIntermediateType), nil
	}

	switch profile.Type {
	case configv1.TLSProfileOldType:
		return cloneBuiltinProfileSpec(configv1.TLSProfileOldType), nil
	case configv1.TLSProfileIntermediateType:
		return cloneBuiltinProfileSpec(configv1.TLSProfileIntermediateType), nil
	case configv1.TLSProfileModernType:
		return cloneBuiltinProfileSpec(configv1.TLSProfileModernType), nil
	case configv1.TLSProfileCustomType:
		if profile.Custom == nil {
			return nil, fmt.Errorf("custom TLS profile is missing custom settings")
		}
		custom := profile.Custom.DeepCopy()
		return &custom.TLSProfileSpec, nil
	default:
		return nil, fmt.Errorf("unrecognized TLSSecurityProfile.Type %q", profile.Type)
	}
}

// ExternalSecretsCipherSuiteArgKeys lists operand flags that must not be set
// when the effective minimum TLS version is 1.3 (Go does not honor cipher
// configuration for TLS 1.3).
var ExternalSecretsCipherSuiteArgKeys = []string{
	"--tls-ciphers",
}

// tlsVersionString converts an OpenShift TLS version constant (e.g.
// VersionTLS12) to the short form expected by the upstream external-secrets
// binary (e.g. "1.2"). An unset or unrecognized version returns an empty
// string, so that no --tls-min-version flag is passed and the operand's own
// default applies.
func tlsVersionString(v configv1.TLSProtocolVersion) string {
	switch v {
	case configv1.VersionTLS10:
		return "1.0"
	case configv1.VersionTLS11:
		return "1.1"
	case configv1.VersionTLS12:
		return "1.2"
	case configv1.VersionTLS13:
		return "1.3"
	default:
		return ""
	}
}

// joinIANACiphers converts the profile's cipher names (OpenSSL or IANA form) to
// the IANA names expected by the upstream external-secrets binary, dropping any
// that are unsupported. It leans on controller-runtime-common to resolve cipher
// codes for the profile's own minimum TLS version and the stdlib to map those
// codes back to canonical IANA names.
func joinIANACiphers(spec configv1.TLSProfileSpec) string {
	configure, _ := crtls.NewTLSConfigFromProfile(spec)
	tlsConf := &tls.Config{}
	configure(tlsConf)

	iana := make([]string, 0, len(tlsConf.CipherSuites))
	for _, id := range tlsConf.CipherSuites {
		iana = append(iana, tls.CipherSuiteName(id))
	}
	return strings.Join(iana, ",")
}

// OperandTLSArgs builds the --tls-min-version and --tls-ciphers flags for the
// operand deployments. A nil spec returns no args, meaning no cluster profile is
// enforced and the operand's own defaults apply.
func OperandTLSArgs(spec *configv1.TLSProfileSpec) []string {
	if spec == nil {
		return nil
	}
	var args []string
	if minVersion := tlsVersionString(spec.MinTLSVersion); minVersion != "" {
		args = append(args, "--tls-min-version="+minVersion)
	}
	if spec.MinTLSVersion != configv1.VersionTLS13 {
		ciphers := joinIANACiphers(*spec)
		if ciphers != "" {
			args = append(args, "--tls-ciphers="+ciphers)
		}
	}
	return args
}
