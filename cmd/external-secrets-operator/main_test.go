package main

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
)

// TestResolveTLSProfileType guards against a nil pointer dereference on
// apiServer.Spec.TLSSecurityProfile, which is optional and commonly unset on
// clusters (e.g. no explicit spec.tlsSecurityProfile on
// apiserver.config.openshift.io/cluster). It must fall back to the same
// default (Intermediate) profile type that GetTLSProfileSpec falls back to
// for tlsProfileSpec, so tlsProfileType and tlsProfileSpec stay consistent.
func TestResolveTLSProfileType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		profile *configv1.TLSSecurityProfile
		want    configv1.TLSProfileType
	}{
		{
			name:    "nil profile falls back to default type",
			profile: nil,
			want:    libgocrypto.DefaultTLSProfileType,
		},
		{
			name:    "non-nil profile with empty type falls back to default type",
			profile: &configv1.TLSSecurityProfile{},
			want:    libgocrypto.DefaultTLSProfileType,
		},
		{
			name:    "explicit type is honored",
			profile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType},
			want:    configv1.TLSProfileModernType,
		},
		{
			name:    "old type is honored",
			profile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileOldType},
			want:    configv1.TLSProfileOldType,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := resolveTLSProfileType(tc.profile)
			if got != tc.want {
				t.Errorf("resolveTLSProfileType(%+v) = %q, want %q", tc.profile, got, tc.want)
			}
		})
	}
}
