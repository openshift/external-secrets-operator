package common

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFilterAnnotations(t *testing.T) {
	source := map[string]string{
		"kubernetes.io/foo":                 "bar",
		"deployment.kubernetes.io/revision": "4",

		"k8s.io/foo":         "bar",
		"friends.k8s.io/xyz": "infinity",

		"openshift.io/foo":                "p",
		"services.openshift.io/apiserver": "self",

		"cert-manager.io/secret-name":          "x-cert",
		"acme.cert-manager.io/dns01-challenge": "dns",
	}

	// the legit annotations should be retained.
	retained := map[string]string{
		"platform.openshift.site/eso":        "included",
		"platform-team.company.io/component": "eso",

		"kubernetes.io": "foobar",
		"k8s.io":        "foobar",

		"app.kubernetes.io": "none",
		"none.k8s.io":       "foo",

		"openshift.io":      "enterprise",
		"beta.openshift.io": "foobar",

		"cert-manager.io":      "ready",
		"acme.cert-manager.io": "cert",
	}

	// include all non-rejected annotations in source.
	for k, v := range retained {
		source[k] = v
	}

	filtered := FilterReservedAnnotations(retained)
	if !reflect.DeepEqual(filtered, retained) {
		t.Fatalf("expected filtered annotations mismatch: %s", cmp.Diff(filtered, retained))
	}
}
func TestDeploymentObjectChanged(t *testing.T) {
	x := appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{
				"deployment.kubernetes.io/revision": "4",
			},
		},
	}

	y := appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{},
		},
	}

	if HasObjectChanged(&y, &x) {
		t.Fatal("expected mismatch")
	}
}
