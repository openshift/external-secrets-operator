package tlsprofile

import (
	"crypto/tls"
	"crypto/x509"
	"slices"
	"strings"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	openshifttls "github.com/openshift/controller-runtime-common/pkg/tls"
)

// specOf resolves profile (nil means the API default, i.e. Intermediate) via
// openshifttls.GetTLSProfileSpec and returns a pointer, failing the test on
// error. It exists purely to keep the test tables below readable.
func specOf(t *testing.T, profile *configv1.TLSSecurityProfile) *configv1.TLSProfileSpec {
	t.Helper()
	spec, err := openshifttls.GetTLSProfileSpec(profile)
	if err != nil {
		t.Fatal(err)
	}
	return &spec
}

// setTLSConfig calls SetTLSConfig and restores the cache to "nothing
// configured" afterward. Not run with t.Parallel(): SetTLSConfig mutates the
// package-level cache shared by every caller, so tests using it must run
// sequentially relative to each other.
func setTLSConfig(t *testing.T, spec *configv1.TLSProfileSpec) {
	t.Helper()
	if err := SetTLSConfig(spec); err != nil {
		t.Fatalf("SetTLSConfig: %v", err)
	}
	t.Cleanup(func() { _ = SetTLSConfig(nil) })
}

func TestClientTLSConfig(t *testing.T) {
	pool := x509.NewCertPool()

	tests := []struct {
		name    string
		spec    func(t *testing.T) *configv1.TLSProfileSpec
		rootCAs *x509.CertPool
		wantNil bool
		check   func(t *testing.T, cfg *tls.Config)
	}{
		{
			name:    "no cluster profile returns nil",
			spec:    func(t *testing.T) *configv1.TLSProfileSpec { return nil },
			wantNil: true,
		},
		{
			name: "intermediate (default): TLS1.2, non-empty ciphers and curve preferences",
			spec: func(t *testing.T) *configv1.TLSProfileSpec { return specOf(t, nil) },
			check: func(t *testing.T, cfg *tls.Config) {
				if cfg.MinVersion != tls.VersionTLS12 {
					t.Errorf("MinVersion = %d, want %d", cfg.MinVersion, tls.VersionTLS12)
				}
				if len(cfg.CipherSuites) == 0 {
					t.Error("expected non-empty cipher suites for TLS 1.2")
				}
				if len(cfg.CurvePreferences) == 0 {
					t.Error("expected non-empty curve preferences")
				}
			},
		},
		{
			name: "modern: TLS1.3, empty cipher suites (Go doesn't allow configuring TLS1.3 ciphers)",
			spec: func(t *testing.T) *configv1.TLSProfileSpec {
				return specOf(t, &configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType})
			},
			check: func(t *testing.T, cfg *tls.Config) {
				if cfg.MinVersion != tls.VersionTLS13 {
					t.Errorf("MinVersion = %d, want %d", cfg.MinVersion, tls.VersionTLS13)
				}
				if len(cfg.CipherSuites) != 0 {
					t.Errorf("expected empty cipher suites for TLS 1.3, got %d", len(cfg.CipherSuites))
				}
			},
		},
		{
			name: "old: must not error despite 3 ciphers Go doesn't implement",
			spec: func(t *testing.T) *configv1.TLSProfileSpec {
				return specOf(t, &configv1.TLSSecurityProfile{Type: configv1.TLSProfileOldType})
			},
			check: func(t *testing.T, cfg *tls.Config) {
				if cfg.MinVersion != tls.VersionTLS10 {
					t.Errorf("MinVersion = %d, want %d", cfg.MinVersion, tls.VersionTLS10)
				}
				if len(cfg.CipherSuites) == 0 {
					t.Error("expected non-empty cipher suites for Old (most of its ciphers are supported)")
				}
			},
		},
		{
			name: "empty ciphers list yields empty CipherSuites",
			spec: func(t *testing.T) *configv1.TLSProfileSpec {
				return &configv1.TLSProfileSpec{MinTLSVersion: configv1.VersionTLS12}
			},
			check: func(t *testing.T, cfg *tls.Config) {
				if len(cfg.CipherSuites) != 0 {
					t.Errorf("expected empty cipher suites for empty ciphers list, got %d", len(cfg.CipherSuites))
				}
			},
		},
		{
			name: "no groups falls back to DefaultCurvePreferences exactly",
			spec: func(t *testing.T) *configv1.TLSProfileSpec {
				return &configv1.TLSProfileSpec{MinTLSVersion: configv1.VersionTLS12}
			},
			check: func(t *testing.T, cfg *tls.Config) {
				if !slices.Equal(cfg.CurvePreferences, DefaultCurvePreferences) {
					t.Errorf("CurvePreferences = %v, want %v", cfg.CurvePreferences, DefaultCurvePreferences)
				}
			},
		},
		{
			name: "CA pool sets RootCAs, not ClientCAs",
			spec: func(t *testing.T) *configv1.TLSProfileSpec {
				return &configv1.TLSProfileSpec{MinTLSVersion: configv1.VersionTLS12}
			},
			rootCAs: pool,
			check: func(t *testing.T, cfg *tls.Config) {
				if cfg.RootCAs != pool {
					t.Errorf("RootCAs = %p, want %p", cfg.RootCAs, pool)
				}
				if cfg.ClientCAs != nil {
					t.Errorf("ClientCAs = %p, want nil", cfg.ClientCAs)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setTLSConfig(t, tt.spec(t))

			got := ClientTLSConfig(tt.rootCAs)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil tls.Config, got %#v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil tls.Config")
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestServerTLSConfig(t *testing.T) {
	pool := x509.NewCertPool()

	tests := []struct {
		name      string
		spec      func(t *testing.T) *configv1.TLSProfileSpec
		clientCAs *x509.CertPool
		wantNil   bool
		check     func(t *testing.T, cfg *tls.Config)
	}{
		{
			name:    "no cluster profile returns nil",
			spec:    func(t *testing.T) *configv1.TLSProfileSpec { return nil },
			wantNil: true,
		},
		{
			name: "old: must not error despite 3 ciphers Go doesn't implement",
			spec: func(t *testing.T) *configv1.TLSProfileSpec {
				return specOf(t, &configv1.TLSSecurityProfile{Type: configv1.TLSProfileOldType})
			},
			check: func(t *testing.T, cfg *tls.Config) {
				if cfg.MinVersion != tls.VersionTLS10 {
					t.Errorf("MinVersion = %d, want %d", cfg.MinVersion, tls.VersionTLS10)
				}
				if len(cfg.CipherSuites) == 0 {
					t.Error("expected non-empty cipher suites for Old (most of its ciphers are supported)")
				}
			},
		},
		{
			name: "CA pool sets ClientCAs, not RootCAs",
			spec: func(t *testing.T) *configv1.TLSProfileSpec {
				return &configv1.TLSProfileSpec{MinTLSVersion: configv1.VersionTLS12}
			},
			clientCAs: pool,
			check: func(t *testing.T, cfg *tls.Config) {
				if cfg.ClientCAs != pool {
					t.Errorf("ClientCAs = %p, want %p", cfg.ClientCAs, pool)
				}
				if cfg.RootCAs != nil {
					t.Errorf("RootCAs = %p, want nil", cfg.RootCAs)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setTLSConfig(t, tt.spec(t))

			got := ServerTLSConfig(tt.clientCAs)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil tls.Config, got %#v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil tls.Config")
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// TestSetTLSConfig_typoProtection verifies that a genuinely unrecognized
// cipher/group name (as opposed to one Go simply doesn't implement, see
// knownGoUnsupportedCiphers) is a hard error, not a silently-skipped entry.
func TestSetTLSConfig_typoProtection(t *testing.T) {
	tests := []struct {
		name string
		spec *configv1.TLSProfileSpec
	}{
		{
			name: "unrecognized cipher name",
			spec: &configv1.TLSProfileSpec{
				MinTLSVersion: configv1.VersionTLS12,
				Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256", "TOTALLY-NOT-A-CIPHER"},
			},
		},
		{
			name: "unrecognized group name",
			spec: &configv1.TLSProfileSpec{
				MinTLSVersion: configv1.VersionTLS12,
				Groups:        []configv1.TLSGroup{"not-a-real-group"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { _ = SetTLSConfig(nil) })
			if err := SetTLSConfig(tt.spec); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
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

func TestGetOperandTLSArgs(t *testing.T) {
	tests := []struct {
		name  string
		spec  func(t *testing.T) *configv1.TLSProfileSpec
		check func(t *testing.T, m map[string]string)
	}{
		{
			name: "no cluster profile leaves args empty",
			spec: func(t *testing.T) *configv1.TLSProfileSpec { return nil },
			check: func(t *testing.T, m map[string]string) {
				if len(m) != 0 {
					t.Errorf("expected no args, got %#v", m)
				}
			},
		},
		{
			name: "intermediate (default)",
			spec: func(t *testing.T) *configv1.TLSProfileSpec { return specOf(t, nil) },
			check: func(t *testing.T, m map[string]string) {
				if m["--tls-min-version"] != "1.2" {
					t.Errorf("expected --tls-min-version=1.2, got %q", m["--tls-min-version"])
				}
				if m["--tls-ciphers"] == "" {
					t.Error("expected --tls-ciphers to be set for TLS 1.2")
				}
				if !strings.Contains(m["--tls-ciphers"], "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256") {
					t.Errorf("expected IANA cipher name in --tls-ciphers, got %q", m["--tls-ciphers"])
				}
			},
		},
		{
			name: "TLS 1.3 omits --tls-ciphers",
			spec: func(t *testing.T) *configv1.TLSProfileSpec {
				return &configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS13,
					Ciphers: []string{
						"TLS_AES_128_GCM_SHA256",
						"TLS_AES_256_GCM_SHA384",
						"TLS_CHACHA20_POLY1305_SHA256",
					},
				}
			},
			check: func(t *testing.T, m map[string]string) {
				if m["--tls-min-version"] != "1.3" {
					t.Errorf("expected --tls-min-version=1.3, got %q", m["--tls-min-version"])
				}
				if _, ok := m["--tls-ciphers"]; ok {
					t.Error("TLS 1.3 must not set --tls-ciphers")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setTLSConfig(t, tt.spec(t))
			m := parseArgs(t, GetOperandTLSArgs())
			tt.check(t, m)
		})
	}
}

// bitwarden-sdk-server runs with HTTP/2 enabled and rejects a sub-1.3 cipher
// list without an AES_128_GCM_SHA256 suite, so every builtin profile must
// carry one through to the operand args.
func TestGetOperandTLSArgs_builtinProfilesKeepHTTP2Cipher(t *testing.T) {
	for _, profileType := range []configv1.TLSProfileType{
		configv1.TLSProfileOldType,
		configv1.TLSProfileIntermediateType,
	} {
		t.Run(string(profileType), func(t *testing.T) {
			setTLSConfig(t, specOf(t, &configv1.TLSSecurityProfile{Type: profileType}))
			ciphers := parseArgs(t, GetOperandTLSArgs())["--tls-ciphers"]
			if !strings.Contains(ciphers, "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256") &&
				!strings.Contains(ciphers, "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256") {
				t.Errorf("expected an HTTP/2-required AES_128_GCM_SHA256 cipher, got %q", ciphers)
			}
		})
	}
}
