package external_secrets

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

// createOrApplyServiceAccounts ensures required service Account resources exist and are correctly configured.
func (r *Reconciler) createOrApplyServiceAccounts(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string, externalSecretsConfigCreateRecon bool) error {
	serviceAccountsToCreate := []struct {
		assetName string
		condition bool
	}{
		{
			assetName: controllerServiceAccountAssetName,
			condition: true,
		},
		{
			assetName: webhookServiceAccountAssetName,
			condition: true,
		},
		{
			assetName: certControllerServiceAccountAssetName,
			condition: !isCertManagerConfigEnabled(esc),
		},
		{
			assetName: bitwardenServiceAccountAssetName,
			condition: isBitwardenConfigEnabled(esc),
		},
	}

	for _, serviceAccount := range serviceAccountsToCreate {
		if !serviceAccount.condition {
			if err := common.DeleteObject(r.ctx, r.CtrlClient, &corev1.ServiceAccount{}, serviceAccount.assetName); err != nil {
				return fmt.Errorf("failed to delete cert-controller serviceaccount: %w", err)
			}
			continue
		}

		desired := common.DecodeServiceAccountObjBytes(assets.MustAsset(serviceAccount.assetName))
		updateNamespace(desired, esc)
		common.UpdateResourceLabels(desired, resourceLabels)

		serviceAccountName := fmt.Sprintf("%s/%s", desired.GetNamespace(), desired.GetName())
		r.log.V(4).Info("reconciling serviceaccount resource", "name", serviceAccountName)

		key := types.NamespacedName{
			Name:      desired.GetName(),
			Namespace: desired.GetNamespace(),
		}

		fetched := &corev1.ServiceAccount{}
		exist, err := r.Exists(r.ctx, key, fetched)
		if err != nil {
			return common.FromClientError(err, "failed to check if serviceaccount %s exists", serviceAccountName)
		}

		if exist {
			if externalSecretsConfigCreateRecon {
				r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s serviceaccount already exists, possibly from a previous install", serviceAccountName)
			}
			r.log.V(4).Info("serviceaccount exists", "name", serviceAccountName)
		} else {
			if err := r.Create(r.ctx, desired); err != nil {
				return common.FromClientError(err, "failed to create serviceaccount %s", serviceAccountName)
			}
			r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "Created serviceaccount %s", serviceAccountName)
		}
	}

	return nil
}
