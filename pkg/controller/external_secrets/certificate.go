package external_secrets

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	v1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

var (
	serviceExternalSecretWebhookName string = "external-secrets-webhook"
)

func (r *Reconciler) createOrApplyCertificates(es *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string, recon bool) error {
	// Handle webhook certificate
	if err := r.handleWebhookCertificate(es, resourceLabels, recon); err != nil {
		return err
	}

	// Handle bitwarden certificate
	if err := r.handleBitwardenCertificate(es, resourceLabels, recon); err != nil {
		return err
	}

	return nil
}

// handleWebhookCertificate manages the webhook certificate lifecycle
func (r *Reconciler) handleWebhookCertificate(es *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string, recon bool) error {
	// Only create webhook certificate if cert-manager is enabled
	if !isCertManagerConfigEnabled(es) {
		return nil
	}

	return r.createOrApplyCertificate(es, resourceLabels, webhookCertificateAssetName, recon)
}

// handleBitwardenCertificate manages the bitwarden certificate lifecycle
func (r *Reconciler) handleBitwardenCertificate(es *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string, recon bool) error {
	// Bitwarden certificate handling is independent of cert-manager configuration
	// Only handle bitwarden certificates when bitwarden is enabled
	if isBitwardenConfigEnabled(es) {
		bitwardenConfig := es.Spec.ExternalSecretsConfig.BitwardenSecretManagerProvider
		
		// If a secret reference is provided, validate it exists instead of creating a certificate
		if bitwardenConfig.SecretRef.Name != "" {
			return r.assertSecretRefExists(es, bitwardenConfig)
		}
		
		// Create or update bitwarden certificate
		return r.createOrApplyCertificate(es, resourceLabels, bitwardenCertificateAssetName, recon)
	}

	// If bitwarden is not enabled, clean up any existing bitwarden certificate
	return r.cleanupBitwardenCertificate()
}

// cleanupBitwardenCertificate removes the bitwarden certificate when bitwarden is disabled
func (r *Reconciler) cleanupBitwardenCertificate() error {
	if err := common.DeleteObject(r.ctx, r.CtrlClient, &certmanagerv1.Certificate{}, bitwardenCertificateAssetName); err != nil {
		return fmt.Errorf("failed to delete bitwarden certificate: %w", err)
	}
	return nil
}

func (r *Reconciler) createOrApplyCertificate(es *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string, fileName string, recon bool) error {
	desired, err := r.getCertificateObject(es, resourceLabels, fileName)
	if err != nil {
		return err
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
		return common.FromClientError(err, "failed to check %s certificate resource already exists", certificateName)
	}

	if exist && recon {
		r.eventRecorder.Eventf(es, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s certificate resource already exists, maybe from previous installation", certificateName)
	}
	if exist && common.HasObjectChanged(desired, fetched) {
		r.log.V(1).Info("certificate has been modified, updating to desired state", "name", certificateName)
		if err := r.UpdateWithRetry(r.ctx, desired); err != nil {
			return common.FromClientError(err, "failed to update %s certificate resource", certificateName)
		}
		r.eventRecorder.Eventf(es, corev1.EventTypeNormal, "Reconciled", "certificate resource %s reconciled back to desired state", certificateName)
	} else {
		r.log.V(4).Info("certificate resource already exists and is in expected state", "name", certificateName)
	}
	if !exist {
		if err := r.Create(r.ctx, desired); err != nil {
			return common.FromClientError(err, "failed to create %s certificate resource", certificateName)
		}
		r.eventRecorder.Eventf(es, corev1.EventTypeNormal, "Reconciled", "certificate resource %s created", certificateName)
	}

	return nil
}

func (r *Reconciler) getCertificateObject(es *operatorv1alpha1.ExternalSecrets, resourceLabels map[string]string, fileName string) (*certmanagerv1.Certificate, error) {
	certificate := common.DecodeCertificateObjBytes(assets.MustAsset(fileName))

	updateNamespace(certificate, es)
	common.UpdateResourceLabels(certificate, resourceLabels)

	if err := r.updateCertificateParams(es, certificate); err != nil {
		return nil, common.NewIrrecoverableError(err, "failed to update certificate resource for %s/%s deployment", getNamespace(es), es.GetName())
	}

	return certificate, nil
}

func (r *Reconciler) updateCertificateParams(es *operatorv1alpha1.ExternalSecrets, certificate *certmanagerv1.Certificate) error {
	certManageConfig := &operatorv1alpha1.CertManagerConfig{}
	if es.Spec.ExternalSecretsConfig != nil && es.Spec.ExternalSecretsConfig.CertManagerConfig != nil {
		certManageConfig = es.Spec.ExternalSecretsConfig.CertManagerConfig
	}
	externalSecretsNamespace := getNamespace(es)

	if certManageConfig.IssuerRef.Name == "" {
		return fmt.Errorf("issuerRef.Name not present")
	}

	certificate.Spec.IssuerRef = v1.ObjectReference{
		Name:  certManageConfig.IssuerRef.Name,
		Kind:  certManageConfig.IssuerRef.Kind,
		Group: certManageConfig.IssuerRef.Group,
	}

	// Since Kind and Group configs are optional. certmanagerv1.IssuerKind will
	// be used as default for Kind and certmanagerapi.GroupName as default for
	// Group.
	if certificate.Spec.IssuerRef.Kind == "" {
		certificate.Spec.IssuerRef.Kind = issuerKind
	}
	if certificate.Spec.IssuerRef.Group == "" {
		certificate.Spec.IssuerRef.Group = issuerGroup
	}

	if err := r.assertIssuerRefExists(certificate.Spec.IssuerRef, externalSecretsNamespace); err != nil {
		return err
	}

	certificate.Spec.DNSNames = updateNamespaceForFQDN(certificate.Spec.DNSNames, externalSecretsNamespace)

	if certManageConfig.CertificateRenewBefore != nil {
		certificate.Spec.RenewBefore = certManageConfig.CertificateRenewBefore
	}

	if certManageConfig.CertificateDuration != nil {
		certificate.Spec.Duration = certManageConfig.CertificateDuration
	}

	return nil
}

func (r *Reconciler) assertIssuerRefExists(issueRef v1.ObjectReference, namespace string) error {
	ifExists, err := r.getIssuer(issueRef, namespace)
	if err != nil || !ifExists {
		return common.FromClientError(err, "failed to fetch issuer")
	}
	return nil
}

func (r *Reconciler) assertSecretRefExists(es *operatorv1alpha1.ExternalSecrets, bitwardenConfig *operatorv1alpha1.BitwardenSecretManagerProvider) error {
	namespacedName := types.NamespacedName{
		Name:      bitwardenConfig.SecretRef.Name,
		Namespace: getNamespace(es),
	}
	object := &corev1.Secret{}

	if err := r.UncachedClient.Get(r.ctx, namespacedName, object); err != nil {
		return fmt.Errorf("failed to fetch %q secret: %w", namespacedName, err)
	}

	return nil
}

func (r *Reconciler) getIssuer(issuerRef v1.ObjectReference, namespace string) (bool, error) {
	namespacedName := types.NamespacedName{
		Name:      issuerRef.Name,
		Namespace: namespace,
	}

	var object client.Object
	switch issuerRef.Kind {
	case clusterIssuerKind:
		object = &certmanagerv1.ClusterIssuer{}
	case issuerKind:
		object = &certmanagerv1.Issuer{}
	}

	if ifExists, err := r.UncachedClient.Exists(r.ctx, namespacedName, object); err != nil {
		return ifExists, fmt.Errorf("failed to fetch %q issuer: %w", namespacedName, err)
	} else {
		return ifExists, nil
	}
}

func updateNamespaceForFQDN(fqdns []string, namespace string) []string {
	updated := make([]string, 0, len(fqdns))
	for _, fqdn := range fqdns {
		parts := strings.Split(fqdn, ".")
		// DNSNames for kubernetes service will be of the form
		// <service-name>.<service-namespace>.svc.<cluster-domain>
		if len(parts) >= 2 {
			parts[1] = namespace
			updated = append(updated, strings.Join(parts, "."))
		} else {
			updated = append(updated, fqdn)
		}
	}
	return updated
}
