package external_secrets

import (
	"fmt"

	webhook "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

func (r *Reconciler) createOrApplyValidatingWebhookConfiguration(externalsecrets *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string, recon bool) error {
	desiredWebhooks, err := r.getValidatingWebhookObjects(externalsecrets, resourceLabels)
	if err != nil {
		return fmt.Errorf("failed to generate validatingWebhook resource for creation: %w", err)
	}

	for _, desired := range desiredWebhooks {
		validatingWebhookName := desired.GetName()
		r.log.V(4).Info("reconciling validatingWebhook resource", "name", validatingWebhookName)
		fetched := &webhook.ValidatingWebhookConfiguration{}
		key := types.NamespacedName{
			Name: desired.GetName(),
		}
		exist, err := r.Exists(r.ctx, key, fetched)
		if err != nil {
			return common.FromClientError(err, "failed to check %s validatingWebhook resource already exists", validatingWebhookName)
		}

		if exist && recon {
			r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s validatingWebhook resource already exists, maybe from previous installation", validatingWebhookName)
		}
		if exist && common.HasObjectChanged(desired, fetched) {
			r.log.V(1).Info("validatingWebhook has been modified", "updating to desired state", "name", validatingWebhookName)
			if err := r.UpdateWithRetry(r.ctx, desired); err != nil {
				return common.FromClientError(err, "failed to update %s validatingWebhook resource with desired state", validatingWebhookName)
			}
			r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeNormal, "Reconciled", "validatingWebhook resource %s reconciled back to desired state", validatingWebhookName)
		} else {
			r.log.V(4).Info("validatingWebhook resource already exists and is in expected state", "name", validatingWebhookName)
		}

		if !exist {
			if err := r.Create(r.ctx, desired); err != nil {
				return common.FromClientError(err, "failed to create validatingWebhook resource %s", validatingWebhookName)
			}
			r.eventRecorder.Eventf(externalsecrets, corev1.EventTypeNormal, "Reconciled", "validatingWebhook resource %s created", validatingWebhookName)
		}
	}
	return nil

}

func (r *Reconciler) getValidatingWebhookObjects(externalsecrets *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string) ([]*webhook.ValidatingWebhookConfiguration, error) {
	var webhooks []*webhook.ValidatingWebhookConfiguration

	for _, assetName := range []string{validatingWebhookExternalSecretCRDAssetName, validatingWebhookSecretStoreCRDAssetName} {

		validatingWebhook := common.DecodeValidatingWebhookConfigurationObjBytes(assets.MustAsset(assetName))

		common.UpdateResourceLabels(validatingWebhook, resourceLabels)
		if err := updateValidatingWebhookAnnotation(externalsecrets, validatingWebhook); err != nil {
			return nil, fmt.Errorf("failed to update validatingWebhook resource for %s external secrets: %s", externalsecrets.GetName(), err.Error())
		}

		webhooks = append(webhooks, validatingWebhook)
	}

	return webhooks, nil
}

func updateValidatingWebhookAnnotation(externalsecrets *operatorv1alpha1.ExternalSecrets, webhook *webhook.ValidatingWebhookConfiguration) error {
	if externalsecrets != nil &&
		externalsecrets.Spec.ExternalSecretsConfig != nil &&
		externalsecrets.Spec.ExternalSecretsConfig.WebhookConfig != nil &&
		externalsecrets.Spec.ExternalSecretsConfig.WebhookConfig.CertManagerConfig != nil {
		if common.ParseBool(externalsecrets.Spec.ExternalSecretsConfig.WebhookConfig.CertManagerConfig.AddInjectorAnnotations) {
			if webhook.Annotations == nil {
				webhook.Annotations = map[string]string{}
			}
			webhook.Annotations[common.CertManagerInjectCAFromAnnotation] = common.CertManagerInjectCAFromAnnotationValue
		}
	}
	return nil
}
