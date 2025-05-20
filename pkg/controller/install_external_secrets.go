package controller

import operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"

func (r *ExternalSecretsReconciler) reconcileExternalSecretsDeployment(externalsecrets *operatorv1alpha1.ExternalSecrets, recon bool) error {
	if err := validateExternalSecrets(externalsecrets); err != nil {
		return NewIrrecoverableError(err, "%s/%s configuration validation failed", externalsecrets.GetNamespace(), externalsecrets.GetName())
	}

	// if user has set custom labels to be added to all resources created by the controller
	// merge it with the controller's own default labels.
	resourceLabels := make(map[string]string)
	if externalsecrets.Spec.ControllerConfig != nil && len(externalsecrets.Spec.ControllerConfig.Labels) != 0 {
		for k, v := range externalsecrets.Spec.ControllerConfig.Labels {
			resourceLabels[k] = v
		}
	}
	for k, v := range controllerDefaultResourceLabels {
		resourceLabels[k] = v
	}

	if err := r.createOrApplyServiceAccounts(externalsecrets, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile serviceaccount resource")
		return err
	}

	r.log.V(4).Info("finished reconciliation of external-secrets", "namespace", externalsecrets.GetNamespace(), "name", externalsecrets.GetName())
	return nil
}
