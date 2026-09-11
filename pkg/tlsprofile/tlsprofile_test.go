package tlsprofile

import (
	"strings"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestEffectiveSpec_builtinDeepCopiesCiphers(t *testing.T) {
	ref := configv1.TLSProfiles[configv1.TLSProfileIntermediateType]
	if len(ref.Ciphers) == 0 {
		t.Fatal("expected non-empty intermediate cipher list")
	}
	origFirst := ref.Ciphers[0]

	spec, err := EffectiveSpec(&configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType})
	if err != nil {
		t.Fatal(err)
	}
	spec.Ciphers[0] = "MUTATED-CIPHER-SHOULD-NOT-LEAK"

	spec2, err := EffectiveSpec(&configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType})
	if err != nil {
		t.Fatal(err)
	}
	if spec2.Ciphers[0] != origFirst {
		t.Fatalf("second EffectiveSpec first cipher %q, want %q", spec2.Ciphers[0], origFirst)
	}
	if ref.Ciphers[0] != origFirst {
		t.Fatalf("global TLSProfiles mutated: got %q want %q", ref.Ciphers[0], origFirst)
	}
}

func TestEffectiveSpec_nilUsesIntermediate(t *testing.T) {
	spec, err := EffectiveSpec(nil)
	if err != nil {
		t.Fatal(err)
	}
	if spec.MinTLSVersion != configv1.VersionTLS12 {
		t.Fatalf("expected intermediate min TLS 1.2, got %q", spec.MinTLSVersion)
	}
	if len(spec.Ciphers) == 0 {
		t.Fatal("expected non-empty cipher list")
	}
}

func TestEffectiveSpec_unknownTypeReturnsError(t *testing.T) {
	profile := &configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileType("bogus-profile-type"),
	}
	_, err := EffectiveSpec(profile)
	if err == nil {
		t.Fatal("expected error for unrecognized TLS profile type")
	}
	if !strings.Contains(err.Error(), "bogus-profile-type") {
		t.Fatalf("expected error to mention unknown type, got: %v", err)
	}
}

func TestEffectiveSpec_custom(t *testing.T) {
	profile := &configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileCustomType,
		Custom: &configv1.CustomTLSProfile{
			TLSProfileSpec: configv1.TLSProfileSpec{
				Ciphers:       []string{"TLS_AES_128_GCM_SHA256", "ECDHE-RSA-AES128-GCM-SHA256"},
				MinTLSVersion: configv1.VersionTLS12,
			},
		},
	}
	spec, err := EffectiveSpec(profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Ciphers) != 2 {
		t.Fatalf("cipher count: %d", len(spec.Ciphers))
	}
}

func TestEffectiveSpec_customNilReturnsError(t *testing.T) {
	profile := &configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileCustomType,
	}
	_, err := EffectiveSpec(profile)
	if err == nil {
		t.Fatal("expected error for nil custom profile")
	}
}

func TestExternalSecretsWebhookTLSArgs_nilSpecReturnsNil(t *testing.T) {
	args := ExternalSecretsWebhookTLSArgs(nil)
	if args != nil {
		t.Fatalf("expected nil args, got %#v", args)
	}
}

func TestExternalSecretsControllerTLSArgs_nilSpecReturnsNil(t *testing.T) {
	args := ExternalSecretsControllerTLSArgs(nil)
	if args != nil {
		t.Fatalf("expected nil args, got %#v", args)
	}
}

func TestExternalSecretsCertControllerTLSArgs_nilSpecReturnsNil(t *testing.T) {
	args := ExternalSecretsCertControllerTLSArgs(nil)
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

func TestExternalSecretsWebhookTLSArgs_intermediate(t *testing.T) {
	spec, err := EffectiveSpec(nil)
	if err != nil {
		t.Fatal(err)
	}
	args := ExternalSecretsWebhookTLSArgs(spec)
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
	curves, ok := m["--tls-curve-preferences"]
	if !ok {
		t.Fatal("expected --tls-curve-preferences")
	}
	if curves != DefaultCurvePreferencesArg {
		t.Fatalf("curve preferences: got %q, want %q", curves, DefaultCurvePreferencesArg)
	}
}

func TestExternalSecretsWebhookTLSArgs_tls13OmitsCipherFlag(t *testing.T) {
	spec := &configv1.TLSProfileSpec{
		Ciphers: []string{
			"TLS_AES_128_GCM_SHA256",
			"TLS_AES_256_GCM_SHA384",
			"TLS_CHACHA20_POLY1305_SHA256",
		},
		MinTLSVersion: configv1.VersionTLS13,
	}
	args := ExternalSecretsWebhookTLSArgs(spec)
	m := parseArgs(t, args)

	if m["--tls-min-version"] != "1.3" {
		t.Fatalf("expected --tls-min-version=1.3, got %q", m["--tls-min-version"])
	}
	if _, ok := m["--tls-ciphers"]; ok {
		t.Fatal("TLS 1.3 must not set --tls-ciphers")
	}
	if _, ok := m["--tls-curve-preferences"]; !ok {
		t.Fatal("expected --tls-curve-preferences even for TLS 1.3")
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
		{"UnknownVersion", "1.2"},
	}
	for _, tc := range cases {
		got := tlsVersionString(tc.input)
		if got != tc.want {
			t.Errorf("tlsVersionString(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
