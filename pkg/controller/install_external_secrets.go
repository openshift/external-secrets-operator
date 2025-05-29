package controller

import (
	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

func (r *ExternalSecretsReconciler) reconcileExternalSecretsDeployment(es *operatorv1alpha1.ExternalSecrets, recon bool) error {
	if err := r.validateExternalSecretsConfig(es); err != nil {
		return NewIrrecoverableError(err, "%s/%s configuration validation failed", es.GetObjectKind().GroupVersionKind().String(), es.GetName())
	}

	// if user has set custom labels to be added to all resources created by the controller
	// merge it with the controller's own default labels. Labels defined in `ExternalSecretsManager`
	// Spec will have the lowest priority, followed by the labels in `ExternalSecrets` Spec and
	// controllerDefaultResourceLabels will have the highest priority.
	resourceLabels := make(map[string]string)
	if !isESMSpecEmpty(r.esm) && r.esm.Spec.GlobalConfig != nil {
		for k, v := range r.esm.Spec.GlobalConfig.Labels {
			resourceLabels[k] = v
		}
	}
	if es.Spec.ControllerConfig != nil && len(es.Spec.ControllerConfig.Labels) != 0 {
		for k, v := range es.Spec.ControllerConfig.Labels {
			resourceLabels[k] = v
		}
	}
	for k, v := range controllerDefaultResourceLabels {
		resourceLabels[k] = v
	}

	if err := r.createOrApplyRBACResource(es, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile rbac resources")
		return err
	}

	if err := r.createOrApplyValidatingWebhookConfiguration(es, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile validating webhook resource")
		return err
	}

	return nil
}
