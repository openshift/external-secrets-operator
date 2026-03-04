package external_secrets

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

// reconcileExternalSecretsDeployment runs the full install/reconcile of the external-secrets
// operand: it validates the config, then creates or updates resources in dependency order
// (namespace first, then RBAC, services, deployments, webhook). Only after all resources are
// reconciled does it patch the CR's managed-annotations tracking and processed annotation.
// That order ensures we never advance tracking on the CR before obsolete annotations have been
// removed from resources (e.g. spec a,b→c,d: we remove a,b from resources first, then patch CR).
func (r *Reconciler) reconcileExternalSecretsDeployment(esc *operatorv1alpha1.ExternalSecretsConfig, recon bool) error {
	if err := r.validateExternalSecretsConfig(esc); err != nil {
		return common.NewIrrecoverableError(err, "%s/%s configuration validation failed", esc.GetObjectKind().GroupVersionKind().String(), esc.GetName())
	}

	resourceMetadata, err := r.getResourceMetadata(esc)
	if err != nil {
		r.log.Error(err, "failed to get resource metadata")
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

	if err := r.updateCRAnnotationsIfNeeded(esc, resourceMetadata); err != nil {
		return err
	}

	r.log.V(4).Info("finished reconciliation of external-secrets", "namespace", esc.GetNamespace(), "name", esc.GetName())
	return nil
}

// updateCRAnnotationsIfNeeded is called only after all resources have been reconciled. It
// computes managed-annotations tracking and processed annotation on the in-memory CR and,
// when either changed, patches only metadata.annotations on the server. Using Patch avoids
// overwriting user spec or other metadata and reduces conflicts; doing it after reconciliation
// ensures tracking is never advanced before obsolete annotations are removed from resources.
func (r *Reconciler) updateCRAnnotationsIfNeeded(esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata common.ResourceMetadata) error {
	trackingChanged, err := common.AddManagedMetadataAnnotation(esc, common.ManagedAnnotationsKey, resourceMetadata)
	if err != nil {
		r.log.Error(err, "failed to add resource metadata annotation to CR")
		return err
	}
	processedChanged := addProcessedAnnotation(esc)
	if !trackingChanged && !processedChanged {
		return nil
	}
	r.log.V(4).Info("patching operator-specific annotations on CR", "name", esc.GetName())
	annotations := esc.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	patchAnnotations := map[string]string{
		common.ManagedAnnotationsKey:  annotations[common.ManagedAnnotationsKey],
		controllerProcessedAnnotation: annotations[controllerProcessedAnnotation],
	}
	patchBody := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": patchAnnotations,
		},
	}
	patchBytes, err := json.Marshal(patchBody)
	if err != nil {
		return fmt.Errorf("failed to marshal annotation patch for %s: %w", esc.GetName(), err)
	}
	patch := client.RawPatch(types.MergePatchType, patchBytes)
	if err := r.Patch(r.ctx, esc, patch, client.FieldOwner(common.ExternalSecretsOperatorCommonName)); err != nil {
		return fmt.Errorf("failed to patch annotations on %s: %w", esc.GetName(), err)
	}
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

// GetResourceMetadata builds the labels and annotations to apply to all operand resources,
// and computes which annotation keys were previously managed but are no longer in spec
// (DeletedAnnotationKeys) so downstream logic can remove them from resources.
func (r *Reconciler) getResourceMetadata(esc *operatorv1alpha1.ExternalSecretsConfig) (common.ResourceMetadata, error) {
	resourceMetadata := common.ResourceMetadata{}
	r.getResourceLabels(esc, &resourceMetadata)
	if err := r.getResourceAnnotations(esc, &resourceMetadata); err != nil {
		return resourceMetadata, err
	}
	r.log.V(4).Info("built resource metadata for reconcile",
		"name", esc.GetName(),
		"labelCount", len(resourceMetadata.Labels),
		"annotationCount", len(resourceMetadata.Annotations),
		"deletedAnnotationKeyCount", len(resourceMetadata.DeletedAnnotationKeys))
	return resourceMetadata, nil
}

// GetResourceLabels builds the labels map for all operand resources. Labels defined in
// ExternalSecretsManager have the lowest priority, followed by ExternalSecretsConfig labels,
// and defaultLabels have the highest priority. Labels matching DisallowedLabelMatcher are skipped.
func (r *Reconciler) getResourceLabels(esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata *common.ResourceMetadata) {
	resourceMetadata.Labels = make(map[string]string)
	if r.esm != nil && common.IsESMSpecEmpty(r.esm) && r.esm.Spec.GlobalConfig != nil {
		for k, v := range r.esm.Spec.GlobalConfig.Labels {
			if disallowedLabelMatcher.MatchString(k) {
				r.log.V(1).Info("skip adding unallowed label configured in externalsecretsmanagers.operator.openshift.io", "label", k, "value", v)
				continue
			}
			resourceMetadata.Labels[k] = v
		}
	}
	if len(esc.Spec.ControllerConfig.Labels) != 0 {
		for k, v := range esc.Spec.ControllerConfig.Labels {
			if disallowedLabelMatcher.MatchString(k) {
				r.log.V(1).Info("skip adding unallowed label configured in externalsecretsconfig.operator.openshift.io", "label", k, "value", v)
				continue
			}
			resourceMetadata.Labels[k] = v
		}
	}
	maps.Copy(resourceMetadata.Labels, controllerDefaultResourceLabels)
}

// getResourceAnnotations copies the current ControllerConfig.Annotations into resourceMetadata
// and populates DeletedAnnotationKeys with any keys that were previously managed (stored in
// the CR's ManagedAnnotationsKey annotation) but are no longer in spec, so callers can remove
// them from child resources.
func (r *Reconciler) getResourceAnnotations(esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata *common.ResourceMetadata) error {
	resourceMetadata.Annotations = make(map[string]string)
	if esc.Spec.ControllerConfig.Annotations != nil {
		maps.Copy(resourceMetadata.Annotations, esc.Spec.ControllerConfig.Annotations)
	}
	previousAnnotations, err := common.GetPreviouslyAppliedAnnotationKeys(esc.GetAnnotations(), common.ManagedAnnotationsKey)
	if err != nil {
		return fmt.Errorf("failed to get previously applied annotation keys for %s: %w", esc.GetName(), err)
	}
	for _, k := range previousAnnotations {
		if _, ok := resourceMetadata.Annotations[k]; !ok {
			resourceMetadata.DeletedAnnotationKeys = append(resourceMetadata.DeletedAnnotationKeys, k)
		}
	}
	if len(resourceMetadata.DeletedAnnotationKeys) > 0 {
		r.log.V(1).Info("annotation keys no longer in spec will be removed from resources",
			"name", esc.GetName(), "deletedKeys", resourceMetadata.DeletedAnnotationKeys)
	}
	return nil
}
