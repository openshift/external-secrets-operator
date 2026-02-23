package external_secrets

import (
	"maps"

	webhook "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

func (r *Reconciler) createOrApplyValidatingWebhookConfiguration(esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata common.ResourceMetadata, recon bool) error {
	desiredWebhooks := r.getValidatingWebhookObjects(esc, resourceMetadata)

	for _, desired := range desiredWebhooks {
		validatingWebhookName := desired.GetName()
		r.log.V(4).Info("reconciling validatingWebhook resource", "name", validatingWebhookName)
		fetched := &webhook.ValidatingWebhookConfiguration{}
		exist, err := r.Exists(r.ctx, client.ObjectKeyFromObject(desired), fetched)
		if err != nil {
			return common.FromClientError(err, "failed to check %s validatingWebhook resource already exists", validatingWebhookName)
		}

		if exist && recon {
			r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s validatingWebhook resource already exists, maybe from previous installation", validatingWebhookName)
		}
		if exist && common.HasObjectChanged(desired, fetched, &resourceMetadata) {
			r.log.V(1).Info("validatingWebhook has been modified", "updating to desired state", "name", validatingWebhookName)
			common.MergeFetchedAnnotations(desired, fetched, &resourceMetadata)
			if err := r.UpdateWithRetry(r.ctx, desired); err != nil {
				return common.FromClientError(err, "failed to update %s validatingWebhook resource with desired state", validatingWebhookName)
			}
			r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "validatingWebhook resource %s reconciled back to desired state", validatingWebhookName)
		} else {
			r.log.V(4).Info("validatingWebhook resource already exists and is in expected state", "name", validatingWebhookName)
		}

		if !exist {
			if err := r.Create(r.ctx, desired); err != nil {
				return common.FromClientError(err, "failed to create validatingWebhook resource %s", validatingWebhookName)
			}
			r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "validatingWebhook resource %s created", validatingWebhookName)
		}
	}
	return nil
}

func (r *Reconciler) getValidatingWebhookObjects(esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata common.ResourceMetadata) []*webhook.ValidatingWebhookConfiguration {
	assetNames := []string{validatingWebhookExternalSecretCRDAssetName, validatingWebhookSecretStoreCRDAssetName}
	webhooks := make([]*webhook.ValidatingWebhookConfiguration, 0, len(assetNames))

	// Include cert-manager inject annotation in managed metadata so it's
	// tracked by the managed-annotations key. This ensures toggling
	// cert-manager on/off is detected by ObjectMetadataModified and
	// MergeFetchedAnnotations won't reintroduce a stale value.
	for _, assetName := range assetNames {
		validatingWebhook := common.DecodeValidatingWebhookConfigurationObjBytes(assets.MustAsset(assetName))

		common.ApplyResourceMetadata(validatingWebhook, withCertManagerAnnotation(esc, resourceMetadata))

		webhooks = append(webhooks, validatingWebhook)
	}

	return webhooks
}

// withCertManagerAnnotation returns a copy of resourceMetadata with the
// cert-manager inject annotation included when cert-manager is enabled.
// The annotation is added to the managed set so it's properly tracked
// for add/remove lifecycle.
func withCertManagerAnnotation(esc *operatorv1alpha1.ExternalSecretsConfig, metadata common.ResourceMetadata) common.ResourceMetadata {
	annotations := make(map[string]string, len(metadata.Annotations)+1)
	maps.Copy(annotations, metadata.Annotations)

	if common.IsInjectCertManagerAnnotationEnabled(esc) {
		annotations[common.CertManagerInjectCAFromAnnotation] = common.CertManagerInjectCAFromAnnotationValue
	}

	metadata.Annotations = annotations
	return metadata
}
