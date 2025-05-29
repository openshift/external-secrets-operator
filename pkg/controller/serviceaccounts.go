package controller

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

// createOrApplyServiceAccounts ensures required service Account resources exist and are correctly configured.
func (r *ExternalSecretsReconciler) createOrApplyServiceAccounts(externalsecrets *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string, externalsecretsCreateRecon bool) error {
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
			condition: !isCertManagerConfigEnabled(externalsecrets),
		},
		{
			assetName: bitwardenServiceAccountAssetName,
			condition: isBitwardenConfigEnabled(externalsecrets),
		},
	}

	for _, serviceAccount := range serviceAccountsToCreate {
		if !serviceAccount.condition {
			continue
		}

		desired := decodeServiceAccountObjBytes(assets.MustAsset(serviceAccount.assetName))
		updateNamespace(desired, externalsecrets.GetNamespace())
		updateResourceLabels(desired, resourceLabels)

		serviceAccountName := fmt.Sprintf("%s/%s", desired.GetNamespace(), desired.GetName())
		r.log.V(4).Info("reconciling serviceaccount resource", "name", serviceAccountName)

		key := types.NamespacedName{
			Name:      desired.GetName(),
			Namespace: desired.GetNamespace(),
		}

		fetched := &corev1.ServiceAccount{}
		exist, err := r.Exists(r.ctx, key, fetched)
		if err != nil {
			return FromClientError(err, "failed to check if serviceaccount %s exists", serviceAccountName)
		}

		if exist {
			if externalsecretsCreateRecon {
				r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s serviceaccount already exists, possibly from a previous install", serviceAccountName)
			}
			r.log.V(4).Info("serviceaccount exists", "name", serviceAccountName)
		} else {
			if err := r.Create(r.ctx, desired); err != nil {
				return FromClientError(err, "failed to create serviceaccount %s", serviceAccountName)
			}
			r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeNormal, "Reconciled", "Created serviceaccount %s", serviceAccountName)
		}
	}

	return nil
}
