package external_secrets

import (
	"fmt"

	webhook "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

func (r *Reconciler) createOrApplyValidatingWebhookConfiguration(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string, recon bool) error {
	desiredWebhooks, err := r.getValidatingWebhookObjects(esc, resourceLabels)
	if err != nil {
		return fmt.Errorf("failed to generate validatingWebhook resource for creation: %w", err)
	}

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
		if exist && common.HasObjectChanged(desired, fetched) {
			r.log.V(1).Info("validatingWebhook has been modified", "updating to desired state", "name", validatingWebhookName)
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

func (r *Reconciler) getValidatingWebhookObjects(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string) ([]*webhook.ValidatingWebhookConfiguration, error) {
	var webhooks []*webhook.ValidatingWebhookConfiguration

	for _, assetName := range []string{validatingWebhookExternalSecretCRDAssetName, validatingWebhookSecretStoreCRDAssetName} {
		validatingWebhook := common.DecodeValidatingWebhookConfigurationObjBytes(assets.MustAsset(assetName))

		common.UpdateResourceLabels(validatingWebhook, resourceLabels)
		if err := updateValidatingWebhookAnnotation(esc, validatingWebhook); err != nil {
			return nil, fmt.Errorf("failed to update validatingWebhook resource for %s external secrets: %s", esc.GetName(), err.Error())
		}

		webhooks = append(webhooks, validatingWebhook)
	}

	return webhooks, nil
}

func updateValidatingWebhookAnnotation(esc *operatorv1alpha1.ExternalSecretsConfig, webhook *webhook.ValidatingWebhookConfiguration) error {
	if common.IsInjectCertManagerAnnotationEnabled(esc) {
		if webhook.Annotations == nil {
			webhook.Annotations = map[string]string{}
		}
		webhook.Annotations[common.CertManagerInjectCAFromAnnotation] = common.CertManagerInjectCAFromAnnotationValue
		return nil
	}
	if webhook.Annotations != nil {
		delete(webhook.Annotations, common.CertManagerInjectCAFromAnnotation)
		if len(webhook.Annotations) == 0 {
			webhook.Annotations = nil
		}
	}
	return nil
}
