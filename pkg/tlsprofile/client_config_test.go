package tlsprofile

import (
	"crypto/tls"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestClientTLSConfig_nilSpecErrors(t *testing.T) {
	_, err := ClientTLSConfig(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil spec")
	}
}

func TestClientTLSConfig_intermediate(t *testing.T) {
	spec, err := EffectiveSpec(nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ClientTLSConfig(spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %d, want %d", cfg.MinVersion, tls.VersionTLS12)
	}
	if len(cfg.CipherSuites) == 0 {
		t.Fatal("expected non-empty cipher suites for TLS 1.2")
	}
	if len(cfg.CurvePreferences) == 0 {
		t.Fatal("expected non-empty curve preferences")
	}
}

func TestClientTLSConfig_modern(t *testing.T) {
	spec, err := EffectiveSpec(&configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ClientTLSConfig(spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion = %d, want %d", cfg.MinVersion, tls.VersionTLS13)
	}
	if len(cfg.CipherSuites) != 0 {
		t.Fatalf("expected empty cipher suites for TLS 1.3, got %d", len(cfg.CipherSuites))
	}
}

func TestClientTLSConfig_emptyCiphers(t *testing.T) {
	spec := &configv1.TLSProfileSpec{
		MinTLSVersion: configv1.VersionTLS12,
	}
	cfg, err := ClientTLSConfig(spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CipherSuites) != 0 {
		t.Fatalf("expected empty cipher suites for empty ciphers list, got %d", len(cfg.CipherSuites))
	}
}

func TestClientTLSConfig_curvePreferencesSet(t *testing.T) {
	spec, err := EffectiveSpec(nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ClientTLSConfig(spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CurvePreferences) != len(DefaultCurvePreferences) {
		t.Fatalf("CurvePreferences length = %d, want %d", len(cfg.CurvePreferences), len(DefaultCurvePreferences))
	}
	for i, want := range DefaultCurvePreferences {
		if cfg.CurvePreferences[i] != want {
			t.Fatalf("CurvePreferences[%d] = %d, want %d", i, cfg.CurvePreferences[i], want)
		}
	}
}
