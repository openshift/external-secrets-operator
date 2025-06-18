package external_secrets

import (
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

var (
	// disallowedLabelMatcher is for restricting the labels defined to apply on all resources
	// created for `external-secrets` operand deployment. Operator will just update required labels
	// on the resources, and other labels will be carried as is from the static manifest, hence
	// adding this rule to restrict users from updating one of aforementioned labels.
	disallowedLabelMatcher = regexp.MustCompile(`^app.kubernetes.io\/|^external-secrets.io\/|^rbac.authorization.k8s.io\/|^servicebinding.io\/controller$|^app$`)
)

func (r *Reconciler) reconcileExternalSecretsDeployment(es *operatorv1alpha1.ExternalSecrets, recon bool) error {
	if err := r.validateExternalSecretsConfig(es); err != nil {
		return common.NewIrrecoverableError(err, "%s/%s configuration validation failed", es.GetObjectKind().GroupVersionKind().String(), es.GetName())
	}

	// if user has set custom labels to be added to all resources created by the controller
	// merge it with the controller's own default labels. Labels defined in `ExternalSecretsManager`
	// Spec will have the lowest priority, followed by the labels in `ExternalSecrets` Spec and
	// controllerDefaultResourceLabels will have the highest priority.
	resourceLabels := make(map[string]string)
	if !common.IsESMSpecEmpty(r.esm) && r.esm.Spec.GlobalConfig != nil {
		for k, v := range r.esm.Spec.GlobalConfig.Labels {
			if disallowedLabelMatcher.MatchString(k) {
				r.log.V(1).Info("skip adding unallowed label configured in externalsecretsmanagers.operator.openshift.io", "label", k, "value", v)
				continue
			}
			resourceLabels[k] = v
		}
	}
	if es.Spec.ControllerConfig != nil && len(es.Spec.ControllerConfig.Labels) != 0 {
		for k, v := range es.Spec.ControllerConfig.Labels {
			if disallowedLabelMatcher.MatchString(k) {
				r.log.V(1).Info("skip adding unallowed label configured in externalsecrets.operator.openshift.io", "label", k, "value", v)
				continue
			}
			resourceLabels[k] = v
		}
	}
	for k, v := range controllerDefaultResourceLabels {
		resourceLabels[k] = v
	}

	if err := r.createOrApplyNamespace(es, resourceLabels); err != nil {
		r.log.Error(err, "failed to create namespace")
	}

	if err := r.createOrApplyServiceAccounts(es, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile serviceaccount resource")
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

	if err := r.createOrApplyRBACResource(es, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile rbac resources")
		return err
	}

	if err := r.createOrApplyServices(es, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile service resource")
		return err
	}

	if err := r.createOrApplyDeployments(es, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile deployment resource")
		return err
	}

	if err := r.createOrApplyValidatingWebhookConfiguration(es, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile validating webhook resource")
		return err
	}

	if addProcessedAnnotation(es) {
		if err := r.UpdateWithRetry(r.ctx, es); err != nil {
			return fmt.Errorf("failed to update processed annotation to %s: %w", es.GetName(), err)
		}
	}

	r.log.V(4).Info("finished reconciliation of external-secrets", "namespace", es.GetNamespace(), "name", es.GetName())
	return nil
}

// createOrApplyNamespace is for the creating the namespace in which the `external-secrets`
// resources will be created.
func (r *Reconciler) createOrApplyNamespace(es *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string) error {
	namespace := getNamespace(es)
	obj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespace,
			Labels: resourceLabels,
		},
	}
	if err := r.Create(r.ctx, obj); err != nil {
		if errors.IsAlreadyExists(err) {
			r.log.V(4).Info("namespace already exists", "namespace", namespace)
			return nil
		}
		return err
	}
	return nil
}
