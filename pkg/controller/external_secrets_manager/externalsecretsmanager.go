package external_secrets_manager

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

// CreateDefaultESMResource is for creating the default externalsecretsmanagers.operator.openshift.io resource,
// which will be updated by the user with required configurations. Controller creates and manages the resource.
func CreateDefaultESMResource(ctx context.Context, client client.Client) error {
	esm := &operatorv1alpha1.ExternalSecretsManager{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.ExternalSecretsManagerObjectName,
			Labels: map[string]string{
				"app.kubernetes.io/name":       common.ExternalSecretsManagerObjectName,
				"app.kubernetes.io/instance":   common.ExternalSecretsManagerObjectName,
				"app.kubernetes.io/version":    common.ExternalSecretsOperatorVersion,
				"app.kubernetes.io/managed-by": "external-secrets-operator",
				"app.kubernetes.io/part-of":    "external-secrets-operator",
			},
		},
		Spec: operatorv1alpha1.ExternalSecretsManagerSpec{},
	}

	shouldRetryOnError := func(err error) bool {
		if err == nil {
			return false
		}
		// Don't retry on these permanent errors
		if errors.IsAlreadyExists(err) || errors.IsConflict(err) ||
			errors.IsInvalid(err) || errors.IsBadRequest(err) ||
			errors.IsUnauthorized(err) || errors.IsForbidden(err) ||
			errors.IsTooManyRequests(err) {
			return false
		}
		// Don't retry if CRD is terminating - this is a transient state
		// that requires manual intervention
		if strings.Contains(err.Error(), "terminating") {
			return false
		}
		// Retry on other errors (network issues, etc.)
		return true
	}

	if err := retry.OnError(retry.DefaultRetry, shouldRetryOnError, func() error {
		err := client.Create(ctx, esm)
		// If resource already exists, that's fine - consider it success
		if errors.IsAlreadyExists(err) {
			return nil
		}
		// If CRD is terminating, don't retry
		if err != nil && strings.Contains(err.Error(), "terminating") {
			return nil // Return nil to not fail startup, controller will retry later
		}
		if shouldRetryOnError(err) {
			return err
		}
		return nil
	}); err != nil {
		// Log but don't fail startup if resource creation fails
		// The controller will reconcile and create it later
		return err
	}
	return nil
}
