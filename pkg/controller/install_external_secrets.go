package controller

import (
	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

func (r *ExternalSecretsReconciler) reconcileExternalSecretsDeployment(externalsecrets *operatorv1alpha1.ExternalSecrets, recon bool) error {
	if err := r.createOrApplyValidatingWebhookConfiguration(externalsecrets, recon); err != nil {
		r.log.Error(err, "failed to reconcile validating webhook resource")
		return err
	}
	
	return nil
}
