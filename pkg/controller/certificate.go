package controller

import (
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	v1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

var (
	serviceExternalSecretWebhookName string = "external-secrets-webhook"
)

func (r *ExternalSecretsReconciler) createOrApplyCertificates(es *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string, recon bool) error {
	//if _, ok := r.optionalResourcesList[&certmanagerv1.Certificate{}]; !ok {
	//	return nil
	//}

	if !isCertManagerConfigEnabled(es) {
		return nil
	}

	desired, err := r.getCertificateObject(es, resourceLabels)
	if err != nil {
		return fmt.Errorf("failed to generate certificate resource for creation in %s: %w", es.GetNamespace(), err)
	}

	certificateName := fmt.Sprintf("%s/%s", desired.GetNamespace(), desired.GetName())
	r.log.V(4).Info("reconciling certificate resource", "name", certificateName)
	fetched := &certmanagerv1.Certificate{}
	key := types.NamespacedName{
		Name:      desired.GetName(),
		Namespace: desired.GetNamespace(),
	}
	exist, err := r.Exists(r.ctx, key, fetched)
	if err != nil {
		return FromClientError(err, "failed to check %s certificate resource already exists", certificateName)
	}

	if exist && recon {
		r.eventRecorder.Eventf(es, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s certificate resource already exists, maybe from previous installation", certificateName)
	}
	if exist && hasObjectChanged(desired, fetched) {
		r.log.V(1).Info("certificate has been modified, updating to desired state", "name", certificateName)
		if err := r.UpdateWithRetry(r.ctx, desired); err != nil {
			return FromClientError(err, "failed to update %s certificate resource", certificateName)
		}
		r.eventRecorder.Eventf(es, corev1.EventTypeNormal, "Reconciled", "certificate resource %s reconciled back to desired state", certificateName)
	} else {
		r.log.V(4).Info("certificate resource already exists and is in expected state", "name", certificateName)
	}
	if !exist {
		if err := r.Create(r.ctx, desired); err != nil {
			return FromClientError(err, "failed to create %s certificate resource", certificateName)
		}
		r.eventRecorder.Eventf(es, corev1.EventTypeNormal, "Reconciled", "certificate resource %s created", certificateName)
	}

	return nil
}

func (r *ExternalSecretsReconciler) getCertificateObject(es *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string) (*certmanagerv1.Certificate, error) {
	certificate := decodeCertificateObjBytes(assets.MustAsset(webhookCertificateAssetName))

	externalSecretsNamespace := certificate.GetNamespace()
	if es.Spec.ControllerConfig != nil && es.Spec.ControllerConfig.Namespace != "" {
		externalSecretsNamespace = es.Spec.ControllerConfig.Namespace
	}

	updateNamespace(certificate, externalSecretsNamespace)
	updateResourceLabels(certificate, resourceLabels)

	if err := r.updateCertificateParams(es, certificate); err != nil {
		return nil, NewIrrecoverableError(err, "failed to update certificate resource for %s/%s istiocsr deployment", es.GetNamespace(), es.GetName())
	}

	return certificate, nil
}

func (r *ExternalSecretsReconciler) updateCertificateParams(es *operatorv1alpha1.ExternalSecrets, certificate *certmanagerv1.Certificate) error {
	certManageConfig := es.Spec.ExternalSecretsConfig.WebhookConfig.CertManagerConfig

	externalSecretsNamespace := certificate.GetNamespace()

	dns := []string{
		serviceExternalSecretWebhookName,
		fmt.Sprintf("%s.%s", serviceExternalSecretWebhookName, externalSecretsNamespace),
		fmt.Sprintf("%s.%s.svc", serviceExternalSecretWebhookName, externalSecretsNamespace),
	}
	certificate.Spec.DNSNames = dns

	if certManageConfig.CertificateRenewBefore != nil {
		certificate.Spec.RenewBefore = certManageConfig.CertificateRenewBefore
	}

	if certManageConfig.CertificateDuration != nil {
		certificate.Spec.Duration = certManageConfig.CertificateDuration
	}

	if certManageConfig.IssuerRef != (operatorv1alpha1.ObjectReference{}) {
		if err := r.assertIssuerRefExists(certManageConfig.IssuerRef, externalSecretsNamespace); err != nil {
			return err
		}
		certificate.Spec.IssuerRef = v1.ObjectReference{
			Kind:  certManageConfig.IssuerRef.Kind,
			Group: certManageConfig.IssuerRef.Group,
			Name:  certManageConfig.IssuerRef.Name,
		}
	}

	return nil
}

func (r *ExternalSecretsReconciler) assertIssuerRefExists(certManageConfig operatorv1alpha1.ObjectReference, namespace string) error {
	_, err := r.getIssuer(certManageConfig, namespace)
	if err != nil {
		return FromClientError(err, "failed to fetch issuer")
	}
	return nil
}

func (r *ExternalSecretsReconciler) getIssuer(certManageConfig operatorv1alpha1.ObjectReference, namespace string) (client.Object, error) {
	issuerRefKind := strings.ToLower(certManageConfig.Kind)
	namespacedName := types.NamespacedName{
		Name:      certManageConfig.Name,
		Namespace: namespace,
	}

	var object client.Object
	switch issuerRefKind {
	case clusterIssuerKind:
		object = &certmanagerv1.ClusterIssuer{}
	case issuerKind:
		object = &certmanagerv1.Issuer{}
	}

	if err := r.Get(r.ctx, namespacedName, object); err != nil {
		return nil, fmt.Errorf("failed to fetch %q issuer: %w", namespacedName, err)
	}
	return object, nil
}
