package external_secrets

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

// createOrApplyServices handles conditional and default creation of Services.
func (r *Reconciler) createOrApplyServices(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string, externalSecretsConfigCreateRecon bool) error {
	servicesToCreate := []struct {
		assetName string
		condition bool
	}{
		{
			assetName: webhookServiceAssetName,
			condition: true,
		},
		{
			assetName: bitwardenServiceAssetName,
			condition: isBitwardenConfigEnabled(esc),
		},
	}

	for _, service := range servicesToCreate {
		if !service.condition {
			continue
		}
		if err := r.createOrApplyServiceFromAsset(esc, service.assetName, resourceLabels, externalSecretsConfigCreateRecon); err != nil {
			return err
		}
	}

	return nil
}

// createOrApplyServiceFromAsset decodes a Service YAML asset and ensures it exists in the cluster.
func (r *Reconciler) createOrApplyServiceFromAsset(esc *operatorv1alpha1.ExternalSecretsConfig, assetName string, resourceLabels map[string]string, externalsecretsCreateRecon bool) error {
	service := common.DecodeServiceObjBytes(assets.MustAsset(assetName))
	updateNamespace(service, esc)
	common.UpdateResourceLabels(service, resourceLabels)

	serviceName := fmt.Sprintf("%s/%s", service.GetNamespace(), service.GetName())
	r.log.V(4).Info("Reconciling service", "name", serviceName)

	fetched := &corev1.Service{}
	key := types.NamespacedName{
		Name:      service.GetName(),
		Namespace: service.GetNamespace(),
	}
	exists, err := r.Exists(r.ctx, key, fetched)
	if err != nil {
		return common.FromClientError(err, "failed to check existence of service %s", serviceName)
	}

	if exists {
		if externalsecretsCreateRecon {
			r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s already exists", serviceName)
		}
		if common.HasObjectChanged(service, fetched) {
			r.log.V(1).Info("Service modified, updating", "name", serviceName)
			if err := r.UpdateWithRetry(r.ctx, service); err != nil {
				return common.FromClientError(err, "failed to update service %s", serviceName)
			}
			r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "Service %s updated", serviceName)
		} else {
			r.log.V(4).Info("Service already up-to-date", "name", serviceName)
		}
	} else {
		if err := r.Create(r.ctx, service); err != nil {
			return common.FromClientError(err, "failed to create service %s", serviceName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "Service %s created", serviceName)
	}

	return nil
}
