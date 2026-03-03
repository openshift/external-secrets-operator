package external_secrets

import (
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

// reconcileExternalSecretsDeployment runs the full install/reconcile of the external-secrets
// operand: it validates the config, updates managed metadata tracking on the CR, then creates
// or updates resources in dependency order (namespace first, then RBAC, services, deployments,
// webhook). At the end it updates the CR if the managed-annotations tracking or processed
// annotation changed.
func (r *Reconciler) reconcileExternalSecretsDeployment(esc *operatorv1alpha1.ExternalSecretsConfig, recon bool) error {
	if err := r.validateExternalSecretsConfig(esc); err != nil {
		return common.NewIrrecoverableError(err, "%s/%s configuration validation failed", esc.GetObjectKind().GroupVersionKind().String(), esc.GetName())
	}

	resourceMetadata, err := common.GetResourceMetadata(r.log, r.esm, esc, controllerDefaultResourceLabels)
	if err != nil {
		r.log.Error(err, "failed to get resource metadata")
		return err
	}
	resourceMetadataChanged, err := common.AddManagedMetadataAnnotation(esc, resourceMetadata)
	if err != nil {
		r.log.Error(err, "failed to add resource metadata annotation")
		return err
	}

	if err := r.createOrApplyNamespace(esc, resourceMetadata); err != nil {
		r.log.Error(err, "failed to create namespace")
		return err
	}

	if err := r.createOrApplyNetworkPolicies(esc, resourceMetadata, recon); err != nil {
		r.log.Error(err, "failed to reconcile network policy resource")
		return err
	}

	if err := r.createOrApplyServiceAccounts(esc, resourceMetadata, recon); err != nil {
		r.log.Error(err, "failed to reconcile serviceaccount resource")
		return err
	}

	if err := r.createOrApplyCertificates(esc, resourceMetadata, recon); err != nil {
		r.log.Error(err, "failed to reconcile certificates resource")
		return err
	}

	if err := r.createOrApplySecret(esc, resourceMetadata, recon); err != nil {
		r.log.Error(err, "failed to reconcile secret resource")
		return err
	}

	if err := r.ensureTrustedCABundleConfigMap(esc, resourceMetadata); err != nil {
		r.log.Error(err, "failed to ensure trusted CA bundle ConfigMap")
		return err
	}

	if err := r.createOrApplyRBACResource(esc, resourceMetadata, recon); err != nil {
		r.log.Error(err, "failed to reconcile rbac resources")
		return err
	}

	if err := r.createOrApplyServices(esc, resourceMetadata, recon); err != nil {
		r.log.Error(err, "failed to reconcile service resource")
		return err
	}

	if err := r.createOrApplyDeployments(esc, resourceMetadata, recon); err != nil {
		r.log.Error(err, "failed to reconcile deployment resource")
		return err
	}

	if err := r.createOrApplyValidatingWebhookConfiguration(esc, resourceMetadata, recon); err != nil {
		r.log.Error(err, "failed to reconcile validating webhook resource")
		return err
	}

	if addProcessedAnnotation(esc) || resourceMetadataChanged {
		if err := r.UpdateWithRetry(r.ctx, esc); err != nil {
			return fmt.Errorf("failed to update annotations on %s: %w", esc.GetName(), err)
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
func (r *Reconciler) createOrApplyNamespace(esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata common.ResourceMetadata) error {
	namespaceName := getNamespace(esc)

	desired := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}

	common.ApplyResourceMetadata(desired, resourceMetadata)

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
	case r.resourceMetadataNeedsUpdate(fetched, resourceMetadata):
		if err := r.UpdateWithRetry(r.ctx, fetched); err != nil {
			return fmt.Errorf("failed to update namespace %s: %w", namespaceName, err)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "Namespace %s updated", namespaceName)
	default:
		r.log.V(4).Info("Namespace already exists in desired state", "name", namespaceName)
	}

	return nil
}

func (r *Reconciler) resourceMetadataNeedsUpdate(existing *corev1.Namespace, resourceMetadata common.ResourceMetadata) bool {
	needsUpdate := false
	if namespaceLabelsNeedUpdate(existing, resourceMetadata.Labels) {
		needsUpdate = true
		r.log.V(1).Info("Namespace labels changed, updating", "name", existing.GetName())
		// Merge existing labels with desired labels (desired labels take precedence)
		if existing.Labels == nil {
			existing.Labels = make(map[string]string)
		}
		maps.Copy(existing.Labels, resourceMetadata.Labels)
	}

	if namespaceAnnotationsNeedUpdate(existing, resourceMetadata.Annotations) || len(resourceMetadata.DeletedAnnotationKeys) != 0 {
		needsUpdate = true
		r.log.V(1).Info("Namespace annotations changed, updating", "name", existing.GetName())
		if existing.Annotations == nil {
			existing.Annotations = make(map[string]string)
		}
		common.UpdateResourceAnnotations(existing, resourceMetadata.Annotations)
		common.RemoveObsoleteAnnotations(existing, resourceMetadata)
	}

	return needsUpdate
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

// namespaceAnnotationsNeedUpdate checks if any of the desired annotations are missing or different
// in the existing namespace. This is different from ObjectMetadataModified because namespaces
// may be pre-created by users with their own annotations that should be preserved.
func namespaceAnnotationsNeedUpdate(existing *corev1.Namespace, desiredAnnots map[string]string) bool {
	if existing.Annotations == nil && len(desiredAnnots) > 0 {
		return true
	}

	for k, v := range desiredAnnots {
		if existing.Annotations[k] != v {
			return true
		}
	}

	return false
}
