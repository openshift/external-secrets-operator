package external_secrets

import (
	"fmt"
	"maps"
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

func (r *Reconciler) reconcileExternalSecretsDeployment(esc *operatorv1alpha1.ExternalSecretsConfig, recon bool) error {
	if err := r.validateExternalSecretsConfig(esc); err != nil {
		return common.NewIrrecoverableError(err, "%s/%s configuration validation failed", esc.GetObjectKind().GroupVersionKind().String(), esc.GetName())
	}

	// if user has set custom labels to be added to all resources created by the controller
	// merge it with the controller's own default labels. Labels defined in `ExternalSecretsManager`
	// Spec will have the lowest priority, followed by the labels in `ExternalSecretsConfig` Spec and
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
	if len(esc.Spec.ControllerConfig.Labels) != 0 {
		for k, v := range esc.Spec.ControllerConfig.Labels {
			if disallowedLabelMatcher.MatchString(k) {
				r.log.V(1).Info("skip adding unallowed label configured in externalsecretsconfig.operator.openshift.io", "label", k, "value", v)
				continue
			}
			resourceLabels[k] = v
		}
	}
	maps.Copy(resourceLabels, controllerDefaultResourceLabels)

	if err := r.createOrApplyNamespace(esc, resourceLabels); err != nil {
		r.log.Error(err, "failed to create namespace")
		return err
	}

	if err := r.createOrApplyNetworkPolicies(esc, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile network policy resource")
		return err
	}

	if err := r.createOrApplyServiceAccounts(esc, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile serviceaccount resource")
		return err
	}

	if err := r.createOrApplyCertificates(esc, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile certificates resource")
		return err
	}

	if err := r.createOrApplySecret(esc, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile secret resource")
		return err
	}

	if err := r.ensureTrustedCABundleConfigMap(esc, resourceLabels); err != nil {
		r.log.Error(err, "failed to ensure trusted CA bundle ConfigMap")
		return err
	}

	if err := r.createOrApplyRBACResource(esc, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile rbac resources")
		return err
	}

	if err := r.createOrApplyServices(esc, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile service resource")
		return err
	}

	if err := r.createOrApplyDeployments(esc, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile deployment resource")
		return err
	}

	if err := r.createOrApplyValidatingWebhookConfiguration(esc, resourceLabels, recon); err != nil {
		r.log.Error(err, "failed to reconcile validating webhook resource")
		return err
	}

	if addProcessedAnnotation(esc) {
		if err := r.UpdateWithRetry(r.ctx, esc); err != nil {
			return fmt.Errorf("failed to update processed annotation to %s: %w", esc.GetName(), err)
		}
	}

	r.log.V(4).Info("finished reconciliation of external-secrets", "namespace", esc.GetNamespace(), "name", esc.GetName())
	return nil
}

// createOrApplyNamespace is for the creating the namespace in which the `external-secrets`
// resources will be created.
func (r *Reconciler) createOrApplyNamespace(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string) error {
	namespace := getNamespace(esc)
	obj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	if err := r.Create(r.ctx, obj); err != nil {
		if errors.IsAlreadyExists(err) {
			r.log.V(4).Info("namespace already exists", "namespace", namespace)
			return nil
		}
		return fmt.Errorf("failed to create %s namespace: %w", namespace, err)
	}
	return nil
}
