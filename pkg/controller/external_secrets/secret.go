package external_secrets

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

func (r *Reconciler) createOrApplySecret(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string, recon bool) error {
	// secrets are only created if isCertManagerConfig is not enabled
	if isCertManagerConfigEnabled(esc) {
		r.log.V(4).Info("cert-manager config is enabled, skipping webhook component secret resource creation")
		return nil
	}

	desired, err := r.getSecretObject(esc, resourceLabels)
	if err != nil {
		return fmt.Errorf("failed to generate secret resource for creation: %w", err)
	}

	secretName := fmt.Sprintf("%s/%s", desired.GetNamespace(), desired.GetName())
	r.log.V(4).Info("reconciling secret resource", "name", secretName)
	fetched := &corev1.Secret{}

	exist, err := r.Exists(r.ctx, client.ObjectKeyFromObject(desired), fetched)
	if err != nil {
		return common.FromClientError(err, "failed to check %s secret resource already exists", secretName)
	}

	if exist && recon {
		r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s secret resource already exists, maybe from previous installation", secretName)
	}

	if exist && common.ObjectMetadataModified(desired, fetched) {
		r.log.V(1).Info("secret has been modified, updating to desired state", "name", secretName)
		if err := r.UpdateWithRetry(r.ctx, desired); err != nil {
			return common.FromClientError(err, "failed to update %s secret resource", secretName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "secret resource %s reconciled back to desired state", secretName)
	} else {
		r.log.V(4).Info("secret resource already exists and is in expected state", "name", secretName)
	}

	if !exist {
		if err := r.Create(r.ctx, desired); err != nil {
			return common.FromClientError(err, "failed to create %s secret resource", secretName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "secret resource %s created", secretName)
	}
	return nil
}

func (r *Reconciler) getSecretObject(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string) (*corev1.Secret, error) {
	secret := common.DecodeSecretObjBytes(assets.MustAsset(webhookTLSSecretAssetName))

	updateNamespace(secret, esc)
	common.UpdateResourceLabels(secret, resourceLabels)
	return secret, nil
}
