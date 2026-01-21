package external_secrets

import (
	"fmt"
	"maps"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

// createOrApplyNamespace ensures the namespace for external-secrets resources exists
// with the correct labels. It creates the namespace if it doesn't exist, or updates
// the labels if they have changed. Unlike other resources, namespaces may be pre-created
// by users with their own labels, so we only add/update our desired labels without
// removing existing ones.
func (r *Reconciler) createOrApplyNamespace(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string) error {
	namespaceName := getNamespace(esc)

	desired := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespaceName,
			Labels: resourceLabels,
		},
	}

	fetched := &corev1.Namespace{}
	exists, err := r.Exists(r.ctx, client.ObjectKeyFromObject(desired), fetched)
	if err != nil {
		return fmt.Errorf("failed to check if namespace %s exists: %w", namespaceName, err)
	}

	switch {
	case !exists:
		r.log.V(4).Info("Creating namespace", "name", namespaceName)
		if err := r.Create(r.ctx, desired); err != nil {
			return fmt.Errorf("failed to create namespace %s: %w", namespaceName, err)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "Namespace %s created", namespaceName)
	case exists && namespaceLabelsNeedUpdate(fetched, resourceLabels):
		r.log.V(1).Info("Namespace labels changed, updating", "name", namespaceName)
		// Merge existing labels with desired labels (desired labels take precedence)
		if fetched.Labels == nil {
			fetched.Labels = make(map[string]string)
		}
		maps.Copy(fetched.Labels, resourceLabels)
		if err := r.UpdateWithRetry(r.ctx, fetched); err != nil {
			return fmt.Errorf("failed to update namespace %s: %w", namespaceName, err)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "Namespace %s updated", namespaceName)
	default:
		r.log.V(4).Info("Namespace already exists with correct labels", "name", namespaceName)
	}

	return nil
}

// namespaceLabelsNeedUpdate checks if any of the desired labels are missing or different
// in the existing namespace. This is different from ObjectMetadataModified because namespaces
// may be pre-created by users with their own labels that should be preserved.
func namespaceLabelsNeedUpdate(existing *corev1.Namespace, desiredLabels map[string]string) bool {
	if existing.Labels == nil && len(desiredLabels) > 0 {
		return true
	}
	for k, v := range desiredLabels {
		if existing.Labels[k] != v {
			return true
		}
	}
	return false
}
