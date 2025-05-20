package controller

import (
	"fmt"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

func (r *ExternalSecretsReconciler) createOrApplyServiceAccounts(externalsecrets *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string, externalsecretsCreateRecon bool) error {
	desired := r.getServiceAccountObject(externalsecrets, resourceLabels)

	serviceAccountName := fmt.Sprintf("%s/%s", desired.GetNamespace(), desired.GetName())
	r.log.V(4).Info("reconciling serviceaccount resource", "name", serviceAccountName)
	fetched := &corev1.ServiceAccount{}
	key := types.NamespacedName{
		Name:      desired.GetName(),
		Namespace: desired.GetNamespace(),
	}
	exist, err := r.Exists(r.ctx, key, fetched)
	if err != nil {
		return FromClientError(err, "failed to check %s serviceaccount resource already exists", serviceAccountName)
	}

	if exist {
		if externalsecretsCreateRecon {
			r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s serviceaccount resource already exists, maybe from previous installation", serviceAccountName)
		}
		r.log.V(4).Info("serviceaccount resource already exists and is in expected state", "name", serviceAccountName)
	}
	if !exist {
		if err := r.Create(r.ctx, desired); err != nil {
			return FromClientError(err, "failed to create %s serviceaccount resource", serviceAccountName)
		}
		r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeNormal, "Reconciled", "serviceaccount resource %s created", serviceAccountName)
	}

	if err := r.updateServiceAccountNameInStatus(externalsecrets, desired); err != nil {
		return FromClientError(err, "failed to update %s/%s external-secrets status with %s serviceaccount resource name", externalsecrets.GetNamespace(), externalsecrets.GetName(), serviceAccountName)
	}

	if externalsecrets.Spec.ExternalSecretsConfig.BitwardenSecretManagerProvider != nil && externalsecrets.Spec.ExternalSecretsConfig.BitwardenSecretManagerProvider.Enabled == "true" {
		if err := r.createBitwardenServiceAccount(externalsecrets, resourceLabels); err != nil {
			return err
		}
	}

	return nil
}
func (r *ExternalSecretsReconciler) createBitwardenServiceAccount(externalsecrets *operatorv1alpha1.ExternalSecrets, labels map[string]string) error {
	serviceAccount := decodeServiceAccountObjBytes(assets.MustAsset(bitwardenserviceAccountAssetName))
	// Set namespace and labels
	updateNamespace(serviceAccount, externalsecrets.GetNamespace())
	updateResourceLabels(serviceAccount, labels)
	// Define the key for lookup
	key := types.NamespacedName{
		Name:      serviceAccount.GetName(),
		Namespace: serviceAccount.GetNamespace(),
	}
	// Check if the ServiceAccount already exists
	exists, err := r.Exists(r.ctx, key, &corev1.ServiceAccount{})
	if err != nil {
		return FromClientError(err, "failed to check existence of Bitwarden ServiceAccount %s/%s", serviceAccount.GetNamespace(), serviceAccount.GetName())
	}

	if exists {
		r.log.V(4).Info("Bitwarden ServiceAccount already exists", "name", serviceAccount.GetName())
		return nil
	}

	// Create the Bitwarden-specific ServiceAccount
	if err := r.Create(r.ctx, serviceAccount); err != nil {
		return FromClientError(err, "failed to create Bitwarden ServiceAccount %s/%s", serviceAccount.GetNamespace(), serviceAccount.GetName())
	}
	r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeNormal, "Reconciled", "Bitwarden ServiceAccount %s/%s created",
		serviceAccount.GetNamespace(),
		serviceAccount.GetName(),
	)

	if err := r.updateServiceAccountNameInStatus(externalsecrets, serviceAccount); err != nil {
		return FromClientError(err, "failed to update ExternalSecrets status with Bitwarden ServiceAccount name")
	}

	return nil
}

func (r *ExternalSecretsReconciler) getServiceAccountObject(externalsecrets *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string) *corev1.ServiceAccount {
	serviceAccount := decodeServiceAccountObjBytes(assets.MustAsset(serviceAccountAssetName))
	updateNamespace(serviceAccount, externalsecrets.GetNamespace())
	updateResourceLabels(serviceAccount, resourceLabels)
	return serviceAccount
}

func (r *ExternalSecretsReconciler) updateServiceAccountNameInStatus(externalsecrets *operatorv1alpha1.ExternalSecrets, serviceAccount *corev1.ServiceAccount) error {
	if externalsecrets.Status.ServiceAccount == serviceAccount.GetName() {
		return nil
	}
	externalsecrets.Status.ServiceAccount = serviceAccount.GetName()
	return r.updateStatus(r.ctx, externalsecrets)
}
