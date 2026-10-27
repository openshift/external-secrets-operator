package tlsprofile

import (
	"strings"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	openshifttls "github.com/openshift/controller-runtime-common/pkg/tls"
)

func TestOperandTLSArgs_nilSpecReturnsNil(t *testing.T) {
	args := OperandTLSArgs(nil)
	if args != nil {
		t.Fatalf("expected nil args, got %#v", args)
	}
}

func parseArgs(t *testing.T, args []string) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, a := range args {
		parts := strings.SplitN(a, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("bad arg %q", a)
		}
		m[parts[0]] = parts[1]
	}
	return m
}

func TestOperandTLSArgs_intermediate(t *testing.T) {
	spec, err := openshifttls.GetTLSProfileSpec(nil)
	if err != nil {
		t.Fatal(err)
	}
	args := OperandTLSArgs(&spec)
	m := parseArgs(t, args)

	if m["--tls-min-version"] != "1.2" {
		t.Fatalf("expected --tls-min-version=1.2, got %q", m["--tls-min-version"])
	}
	ciphers, ok := m["--tls-ciphers"]
	if !ok || ciphers == "" {
		t.Fatal("expected --tls-ciphers to be set for TLS 1.2")
	}
	if !strings.Contains(ciphers, "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256") {
		t.Fatalf("expected IANA cipher name in --tls-ciphers, got %q", ciphers)
	}
	if _, ok := m["--tls-curve-preferences"]; ok {
		t.Fatal("--tls-curve-preferences must not be set; Go defaults apply")
	}
}

func TestOperandTLSArgs_tls13OmitsCipherFlag(t *testing.T) {
	spec := &configv1.TLSProfileSpec{
		Ciphers: []string{
			"TLS_AES_128_GCM_SHA256",
			"TLS_AES_256_GCM_SHA384",
			"TLS_CHACHA20_POLY1305_SHA256",
		},
		MinTLSVersion: configv1.VersionTLS13,
	}
	args := OperandTLSArgs(spec)
	m := parseArgs(t, args)

	if m["--tls-min-version"] != "1.3" {
		t.Fatalf("expected --tls-min-version=1.3, got %q", m["--tls-min-version"])
	}
	if _, ok := m["--tls-ciphers"]; ok {
		t.Fatal("TLS 1.3 must not set --tls-ciphers")
	}
	if _, ok := m["--tls-curve-preferences"]; ok {
		t.Fatal("--tls-curve-preferences must not be set; Go defaults apply")
	}
}

// bitwarden-sdk-server runs with HTTP/2 enabled and rejects a sub-1.3 cipher list
// without an AES_128_GCM_SHA256 suite, so every builtin profile must carry one.
func TestOperandTLSArgs_builtinProfilesKeepHTTP2Cipher(t *testing.T) {
	for _, profileType := range []configv1.TLSProfileType{
		configv1.TLSProfileOldType,
		configv1.TLSProfileIntermediateType,
	} {
		spec, err := openshifttls.GetTLSProfileSpec(&configv1.TLSSecurityProfile{Type: profileType})
		if err != nil {
			t.Fatalf("%s: %v", profileType, err)
		}
		ciphers := parseArgs(t, OperandTLSArgs(&spec))["--tls-ciphers"]
		if !strings.Contains(ciphers, "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256") &&
			!strings.Contains(ciphers, "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256") {
			t.Errorf("%s: expected an HTTP/2-required AES_128_GCM_SHA256 cipher, got %q", profileType, ciphers)
		}
	}
}

func TestTlsVersionString(t *testing.T) {
	cases := []struct {
		input configv1.TLSProtocolVersion
		want  string
	}{
		{configv1.VersionTLS10, "1.0"},
		{configv1.VersionTLS11, "1.1"},
		{configv1.VersionTLS12, "1.2"},
		{configv1.VersionTLS13, "1.3"},
		{"UnknownVersion", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := tlsVersionString(tc.input)
		if got != tc.want {
			t.Errorf("tlsVersionString(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
