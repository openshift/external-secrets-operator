package external_secrets

import (
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	certmanagerapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	v1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

var (
	serviceExternalSecretWebhookName = "external-secrets-webhook"
	errUnsupportedIssuerKind         = errors.New("unsupported issuer kind")
)

func (r *Reconciler) createOrApplyCertificates(esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata common.ResourceMetadata, recon bool) error {
	if isCertManagerConfigEnabled(esc) {
		if err := r.createOrApplyCertificate(esc, resourceMetadata, webhookCertificateAssetName, recon); err != nil {
			return err
		}
	}

	if isBitwardenConfigEnabled(esc) {
		bitwardenConfig := esc.Spec.Plugins.BitwardenSecretManagerProvider
		if bitwardenConfig.SecretRef != nil && bitwardenConfig.SecretRef.Name != "" {
			return r.assertSecretRefExists(esc, esc.Spec.Plugins.BitwardenSecretManagerProvider)
		}
		if !isCertManagerConfigEnabled(esc) {
			return common.NewUserConfigurationError(
				fmt.Errorf("invalid bitwardenSecretManagerProvider config"),
				"either secretRef or certManagerConfig must be configured when bitwardenSecretManagerProvider is enabled",
			)
		}
		if err := r.createOrApplyCertificate(esc, resourceMetadata, bitwardenCertificateAssetName, recon); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) createOrApplyCertificate(esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata common.ResourceMetadata, fileName string, recon bool) error {
	desired, err := r.getCertificateObject(esc, resourceMetadata, fileName)
	if err != nil {
		return err
	}

	certificateName := fmt.Sprintf("%s/%s", desired.GetNamespace(), desired.GetName())
	r.log.V(4).Info("reconciling certificate resource", "name", certificateName)
	fetched := &certmanagerv1.Certificate{}
	exist, err := r.Exists(r.ctx, client.ObjectKeyFromObject(desired), fetched)
	if err != nil {
		return common.FromClientError(err, "failed to check %s certificate resource already exists", certificateName)
	}

	if !exist {
		return r.createWithFallback(desired, resourceMetadata, certificateName, esc)
	}

	if recon {
		r.eventRecorder.Eventf(esc, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s certificate resource already exists, maybe from previous installation", certificateName)
	}

	if !common.HasObjectChanged(desired, fetched, &resourceMetadata) {
		r.log.V(4).Info("certificate resource already exists and is in expected state", "name", certificateName)
		return nil
	}

	r.log.V(1).Info("certificate has been modified, updating to desired state", "name", certificateName)
	common.RemoveObsoleteAnnotations(desired, resourceMetadata)
	if err := r.UpdateWithRetry(r.ctx, desired); err != nil {
		return common.FromClientError(err, "failed to update %s certificate resource", certificateName)
	}
	r.eventRecorder.Eventf(esc, corev1.EventTypeNormal, "Reconciled", "certificate resource %s reconciled back to desired state", certificateName)

	return nil
}

func (r *Reconciler) getCertificateObject(esc *operatorv1alpha1.ExternalSecretsConfig, resourceMetadata common.ResourceMetadata, fileName string) (*certmanagerv1.Certificate, error) {
	certificate := common.DecodeCertificateObjBytes(assets.MustAsset(fileName))

	// update the secret name in the Certificate resource of the webhook component.
	if fileName == webhookCertificateAssetName {
		certificate.Spec.SecretName = certmanagerTLSSecretWebhook
	}

	updateNamespace(certificate, esc)
	common.ApplyResourceMetadata(certificate, resourceMetadata)

	if err := r.updateCertificateParams(esc, certificate); err != nil {
		return nil, err
	}

	return certificate, nil
}

func (r *Reconciler) updateCertificateParams(esc *operatorv1alpha1.ExternalSecretsConfig, certificate *certmanagerv1.Certificate) error {
	certManageConfig := &operatorv1alpha1.CertManagerConfig{}
	if esc.Spec.ControllerConfig.CertProvider != nil && esc.Spec.ControllerConfig.CertProvider.CertManager != nil {
		certManageConfig = esc.Spec.ControllerConfig.CertProvider.CertManager
	}
	if certManageConfig.IssuerRef == nil {
		return common.NewUserConfigurationError(
			fmt.Errorf("cert-manager is enabled but issuerRef is not configured"),
			"cert-manager issuerRef must be configured in %s/%s",
			getNamespace(esc), esc.GetName(),
		)
	}
	if certManageConfig.IssuerRef.Name == "" {
		return common.NewUserConfigurationError(
			fmt.Errorf("cert-manager.issuerRef.name is not configured"),
			"cert-manager issuerRef.name must be set in %s/%s",
			getNamespace(esc), esc.GetName(),
		)
	}
	externalSecretsNamespace := getNamespace(esc)

	certificate.Spec.IssuerRef = v1.IssuerReference{
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

func (r *Reconciler) assertIssuerRefExists(issueRef v1.IssuerReference, namespace string) error {
	issuerExists, err := r.getIssuer(issueRef, namespace)
	if err != nil {
		if errors.Is(err, errUnsupportedIssuerKind) {
			return common.NewUserConfigurationError(
				err,
				"cert-manager issuerRef.kind %q is not supported in %s",
				issueRef.Kind, namespace,
			)
		}
		if clientErr := common.FromClientError(err, "failed to fetch issuer %q", issueRef.Name); clientErr != nil {
			return clientErr
		}
		return common.NewRetryRequiredError(err, "failed to fetch issuer %q", issueRef.Name)
	}
	if !issuerExists {
		return common.NewUserConfigurationError(
			issuerNotFoundError(issueRef),
			"issuer %q of kind %q not found in %s",
			issueRef.Name, issueRef.Kind, namespace,
		)
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
		if apierrors.IsNotFound(err) {
			return common.NewUserConfigurationError(
				err,
				"bitwarden TLS secret %q not found",
				namespacedName,
			)
		}
		if clientErr := common.FromClientError(err, "failed to fetch %q secret", namespacedName); clientErr != nil {
			return clientErr
		}
		return common.NewRetryRequiredError(err, "failed to fetch %q secret", namespacedName)
	}

	return nil
}

func (r *Reconciler) getIssuer(issuerRef v1.IssuerReference, namespace string) (issuerExists bool, err error) {
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
	default:
		return false, fmt.Errorf("%w: %q", errUnsupportedIssuerKind, issuerRef.Kind)
	}

	issuerExists, err = r.UncachedClient.Exists(r.ctx, namespacedName, object)
	if err != nil {
		return issuerExists, fmt.Errorf("failed to fetch %q issuer: %w", namespacedName, err)
	}
	return issuerExists, nil
}

func issuerNotFoundError(issueRef v1.IssuerReference) error {
	return apierrors.NewNotFound(issuerGroupResource(issueRef.Kind), issueRef.Name)
}

func issuerGroupResource(kind string) schema.GroupResource {
	switch kind {
	case clusterIssuerKind:
		return schema.GroupResource{Group: certmanagerapi.GroupName, Resource: "clusterissuers"}
	case issuerKind:
		return schema.GroupResource{Group: certmanagerapi.GroupName, Resource: "issuers"}
	default:
		return schema.GroupResource{Resource: strings.ToLower(kind) + "s"}
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
