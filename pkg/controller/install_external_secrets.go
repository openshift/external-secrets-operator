package controller

import (
	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

func (r *ExternalSecretsReconciler) reconcileExternalSecretsDeployment(es *operatorv1alpha1.ExternalSecrets, recon bool) error {
	resourceLabels := make(map[string]string)
	if es.Spec.ControllerConfig != nil && len(es.Spec.ControllerConfig.Labels) != 0 {
		for k, v := range es.Spec.ControllerConfig.Labels {
			resourceLabels[k] = v
		}
	}
	for k, v := range controllerDefaultResourceLabels {
		resourceLabels[k] = v
	}
	if err := r.createOrApplyValidatingWebhookConfiguration(es, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile validating webhook resource")
		return err
	}

	if err := r.createOrApplyCertificates(es, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile certificates resource")
		return err
	}

	if err := r.createOrApplySecret(es, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile secret resource")
		return err
	}

	return nil
}
