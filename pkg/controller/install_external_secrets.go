package controller

import (
	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

func (r *ExternalSecretsReconciler) reconcileExternalSecretsDeployment(es *operatorv1alpha1.ExternalSecrets, recon bool) error {

	if err := r.validateExternalSecretsConfig(es); err != nil {
		return NewIrrecoverableError(err, "%s/%s configuration validation failed", es.GetObjectKind().GroupVersionKind().String(), es.GetName())
	}

	// if user has set custom labels to be added to all resources created by the controller
	// merge it with the controller's own default labels.
	resourceLabels := make(map[string]string)
	if es.Spec.ControllerConfig != nil && len(es.Spec.ControllerConfig.Labels) != 0 {
		for k, v := range es.Spec.ControllerConfig.Labels {
			resourceLabels[k] = v
		}
	}
	if !isESMSpecEmpty(r.esm) && r.esm.Spec.GlobalConfig != nil {
		for k, v := range r.esm.Spec.GlobalConfig.Labels {
			resourceLabels[k] = v
		}
	}
	for k, v := range controllerDefaultResourceLabels {
		resourceLabels[k] = v
	}

	if err := r.createOrApplyServices(es, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile service resource")
		return err
	}

	if err := r.createOrApplyServiceAccounts(es, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile serviceaccount resource")
		return err
	}

	if err := r.createOrApplyDeployments(es, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile deployment resource")
		return err
	}

	if err := r.createOrApplyValidatingWebhookConfiguration(es, recon); err != nil {
		r.log.Error(err, "failed to reconcile validating webhook resource")
		return err
	}

	r.log.V(4).Info("finished reconciliation of external-secrets", "namespace", es.GetNamespace(), "name", es.GetName())
	return nil
}
