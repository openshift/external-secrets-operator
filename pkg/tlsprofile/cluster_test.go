package tlsprofile

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	configv1 "github.com/openshift/api/config/v1"
)

func TestResolveHonoredTLSProfile_fetchErrors(t *testing.T) {
	notFound := apierrors.NewNotFound(schema.GroupResource{Group: configv1.GroupName, Resource: "apiservers"}, APIServerClusterName)
	forbidden := apierrors.NewForbidden(schema.GroupResource{Group: configv1.GroupName, Resource: "apiservers"}, APIServerClusterName, errors.New("denied"))

	t.Run("NotFound is soft skip", func(t *testing.T) {
		spec, err := ResolveHonoredTLSProfile(context.Background(), func(context.Context) (*configv1.APIServer, error) {
			return nil, notFound
		}, "test", FetchErrorPropagateExceptNotFound)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spec != nil {
			t.Fatalf("expected nil spec on NotFound, got %#v", spec)
		}
	})

	t.Run("Forbidden propagates", func(t *testing.T) {
		spec, err := ResolveHonoredTLSProfile(context.Background(), func(context.Context) (*configv1.APIServer, error) {
			return nil, forbidden
		}, "test", FetchErrorPropagateExceptNotFound)
		if err == nil {
			t.Fatal("expected error")
		}
		if spec != nil {
			t.Fatalf("expected nil spec on Forbidden, got %#v", spec)
		}
	})

	t.Run("nil fetch function errors", func(t *testing.T) {
		_, err := ResolveHonoredTLSProfile(context.Background(), nil, "test", FetchErrorPropagateExceptNotFound)
		if err == nil {
			t.Fatal("expected error for nil fetch")
		}
	})
}

func TestResolveHonoredTLSProfile_adherence(t *testing.T) {
	wantModern, err := EffectiveSpec(&configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType})
	if err != nil {
		t.Fatal(err)
	}
	wantIntermediate, err := EffectiveSpec(nil)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		apiServer *configv1.APIServer
		wantSpec  *configv1.TLSProfileSpec
		wantErr   string
	}{
		{
			name: "empty adherence skips",
			apiServer: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType},
				},
			},
		},
		{
			name: "legacy adherence skips",
			apiServer: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSAdherence:       configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
					TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType},
				},
			},
		},
		{
			name: "strict modern returns effective spec",
			apiServer: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSAdherence:       configv1.TLSAdherencePolicyStrictAllComponents,
					TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType},
				},
			},
			wantSpec: wantModern,
		},
		{
			name: "unknown adherence treated as strict with nil profile falls back to Intermediate",
			apiServer: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSAdherence: configv1.TLSAdherencePolicy("FutureStrictMode"),
				},
			},
			wantSpec: wantIntermediate,
		},
		{
			name: "strict with invalid custom profile propagates error",
			apiServer: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSAdherence: configv1.TLSAdherencePolicyStrictAllComponents,
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileCustomType,
					},
				},
			},
			wantErr: "custom TLS profile is missing custom settings",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveHonoredTLSProfile(context.Background(), func(context.Context) (*configv1.APIServer, error) {
				return tc.apiServer, nil
			}, "test", FetchErrorPropagateExceptNotFound)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
				}
				if got != nil {
					t.Fatalf("expected nil spec on error, got %#v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantSpec == nil {
				if got != nil {
					t.Fatalf("expected nil spec, got %#v", got)
				}
				return
			}
			if got.MinTLSVersion != tc.wantSpec.MinTLSVersion {
				t.Fatalf("MinTLSVersion = %q, want %q", got.MinTLSVersion, tc.wantSpec.MinTLSVersion)
			}
			if !reflect.DeepEqual(got.Ciphers, tc.wantSpec.Ciphers) {
				t.Fatalf("Ciphers = %#v, want %#v", got.Ciphers, tc.wantSpec.Ciphers)
			}
		})
	}
}
