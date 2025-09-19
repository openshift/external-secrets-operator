package external_secrets

import (
	"fmt"
	"reflect"
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
	serviceExternalSecretWebhookName = "external-secrets-webhook"
)

func (r *Reconciler) createOrApplyCertificates(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string, recon bool) error {
	if isCertManagerConfigEnabled(esc) {
		if err := r.createOrApplyCertificate(esc, resourceLabels, webhookCertificateAssetName, recon); err != nil {
			return err
		}
	}

	if isBitwardenConfigEnabled(esc) {
		bitwardenConfig := esc.Spec.ApplicationConfig.BitwardenSecretManagerProvider
		if bitwardenConfig.SecretRef != nil && bitwardenConfig.SecretRef.Name != "" {
			return r.assertSecretRefExists(esc, esc.Spec.ApplicationConfig.BitwardenSecretManagerProvider)
		}
		if err := r.createOrApplyCertificate(esc, resourceLabels, bitwardenCertificateAssetName, recon); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) createOrApplyCertificate(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string, fileName string, recon bool) error {
	desired, err := r.getCertificateObject(esc, resourceLabels, fileName)
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
		r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s certificate resource already exists, maybe from previous installation", certificateName)
	}
	if exist && common.HasObjectChanged(desired, fetched) {
		r.log.V(1).Info("certificate has been modified, updating to desired state", "name", certificateName)
		if err := r.UpdateWithRetry(r.ctx, desired); err != nil {
			return common.FromClientError(err, "failed to update %s certificate resource", certificateName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "certificate resource %s reconciled back to desired state", certificateName)
	} else {
		r.log.V(4).Info("certificate resource already exists and is in expected state", "name", certificateName)
	}
	if !exist {
		if err := r.Create(r.ctx, desired); err != nil {
			return common.FromClientError(err, "failed to create %s certificate resource", certificateName)
		}
		r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "certificate resource %s created", certificateName)
	}

	return nil
}

func (r *Reconciler) getCertificateObject(esc *operatorv1alpha1.ExternalSecretsConfig, resourceLabels map[string]string, fileName string) (*certmanagerv1.Certificate, error) {
	certificate := common.DecodeCertificateObjBytes(assets.MustAsset(fileName))

	updateNamespace(certificate, esc)
	common.UpdateResourceLabels(certificate, resourceLabels)

	if err := r.updateCertificateParams(esc, certificate); err != nil {
		return nil, common.NewIrrecoverableError(err, "failed to update certificate resource for %s/%s deployment", getNamespace(esc), esc.GetName())
	}

	return certificate, nil
}

func (r *Reconciler) updateCertificateParams(esc *operatorv1alpha1.ExternalSecretsConfig, certificate *certmanagerv1.Certificate) error {
	certManageConfig := &operatorv1alpha1.CertManagerConfig{}
	if esc.Spec.ApplicationConfig.CertManagerConfig != nil {
		certManageConfig = esc.Spec.ApplicationConfig.CertManagerConfig
	}
	if reflect.ValueOf(certManageConfig.IssuerRef).IsZero() {
		return fmt.Errorf("certManageConfig is enabled without IssuerRef")
	}
	if certManageConfig.IssuerRef.Name == "" {
		return fmt.Errorf("issuerRef.Name not present")
	}
	externalSecretsNamespace := getNamespace(esc)

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

func (r *Reconciler) assertSecretRefExists(esc *operatorv1alpha1.ExternalSecretsConfig, bitwardenConfig *operatorv1alpha1.BitwardenSecretManagerProvider) error {
	namespacedName := types.NamespacedName{
		Name:      bitwardenConfig.SecretRef.Name,
		Namespace: getNamespace(esc),
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

	ifExists, err := r.UncachedClient.Exists(r.ctx, namespacedName, object)
	if err != nil {
		return ifExists, fmt.Errorf("failed to fetch %q issuer: %w", namespacedName, err)
	}
	return ifExists, nil
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
