// Code generated for package assets by go-bindata DO NOT EDIT. (@generated)
// sources:
// bindata/external-secrets/networkpolicies/allow-api-server-egress-for-bitwarden-sever.yml
// bindata/external-secrets/networkpolicies/allow-api-server-egress-for-cert-controller-traffic.yml
// bindata/external-secrets/networkpolicies/allow-api-server-egress-for-main-controller-traffic.yml
// bindata/external-secrets/networkpolicies/allow-api-server-egress-for-webhook-traffic.yml
// bindata/external-secrets/networkpolicies/allow-dns.yml
// bindata/external-secrets/networkpolicies/deny-all.yml
// bindata/external-secrets/operand/certificate_bitwarden-tls-certs.yml
// bindata/external-secrets/operand/certificate_external-secrets-webhook.yml
// bindata/external-secrets/operand/clusterrole_external-secrets-cert-controller.yml
// bindata/external-secrets/operand/clusterrole_external-secrets-controller.yml
// bindata/external-secrets/operand/clusterrole_external-secrets-edit.yml
// bindata/external-secrets/operand/clusterrole_external-secrets-servicebindings.yml
// bindata/external-secrets/operand/clusterrole_external-secrets-view.yml
// bindata/external-secrets/operand/clusterrolebinding_external-secrets-cert-controller.yml
// bindata/external-secrets/operand/clusterrolebinding_external-secrets-controller.yml
// bindata/external-secrets/operand/deployment_bitwarden-sdk-server.yml
// bindata/external-secrets/operand/deployment_external-secrets-cert-controller.yml
// bindata/external-secrets/operand/deployment_external-secrets-webhook.yml
// bindata/external-secrets/operand/deployment_external-secrets.yml
// bindata/external-secrets/operand/namespace_external-secrets.yml
// bindata/external-secrets/operand/role_external-secrets-leaderelection.yml
// bindata/external-secrets/operand/rolebinding_external-secrets-leaderelection.yml
// bindata/external-secrets/operand/secret_external-secrets-webhook.yml
// bindata/external-secrets/operand/service_bitwarden-sdk-server.yml
// bindata/external-secrets/operand/service_external-secrets-cert-controller-metrics.yml
// bindata/external-secrets/operand/service_external-secrets-metrics.yml
// bindata/external-secrets/operand/service_external-secrets-webhook.yml
// bindata/external-secrets/operand/serviceaccount_bitwarden-sdk-server.yml
// bindata/external-secrets/operand/serviceaccount_external-secrets-cert-controller.yml
// bindata/external-secrets/operand/serviceaccount_external-secrets-webhook.yml
// bindata/external-secrets/operand/serviceaccount_external-secrets.yml
// bindata/external-secrets/operand/validatingwebhookconfiguration_externalsecret-validate.yml
// bindata/external-secrets/operand/validatingwebhookconfiguration_secretstore-validate.yml
package assets

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _externalSecretsNetworkpoliciesAllowApiServerEgressForBitwardenSeverYml = []byte(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: eso-sys-allow-api-server-egress-for-bitwarden-server
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: bitwarden-sdk-server
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v1.2.0"
    app.kubernetes.io/managed-by: external-secrets-operator
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: bitwarden-sdk-server
  policyTypes:
    - Ingress
    - Egress
  ingress:
    # Allow External Secrets Controller to communicate with Bitwarden SDK Server
    - ports:
        - protocol: TCP
          port: 9998
  # Allow access to Kubernetes API server and bitwarden sdk external server
  egress:
    - ports:
        - protocol: TCP
          port: 6443
        - protocol: TCP
          port: 443`)

func externalSecretsNetworkpoliciesAllowApiServerEgressForBitwardenSeverYmlBytes() ([]byte, error) {
	return _externalSecretsNetworkpoliciesAllowApiServerEgressForBitwardenSeverYml, nil
}

func externalSecretsNetworkpoliciesAllowApiServerEgressForBitwardenSeverYml() (*asset, error) {
	bytes, err := externalSecretsNetworkpoliciesAllowApiServerEgressForBitwardenSeverYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/networkpolicies/allow-api-server-egress-for-bitwarden-sever.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYml = []byte(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: eso-sys-allow-api-server-egress-for-cert-controller
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-cert-controller
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v1.2.0"
    app.kubernetes.io/managed-by: external-secrets-operator
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: external-secrets-cert-controller
  policyTypes:
    - Egress
    - Ingress
  egress:
    - ports:
        - protocol: TCP
          port: 6443
  ingress:
    # Allow Prometheus/monitoring to scrape metrics
    - from:
      - namespaceSelector:
          matchLabels:
            name: openshift-user-workload-monitoring
      ports:
        - protocol: TCP
          port: 8080`)

func externalSecretsNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYmlBytes() ([]byte, error) {
	return _externalSecretsNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYml, nil
}

func externalSecretsNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYml() (*asset, error) {
	bytes, err := externalSecretsNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/networkpolicies/allow-api-server-egress-for-cert-controller-traffic.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYml = []byte(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: eso-sys-allow-api-server-egress-for-main-controller
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v1.2.0"
    app.kubernetes.io/managed-by: external-secrets-operator
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: external-secrets
  policyTypes:
    - Egress
    - Ingress
  egress:
    - ports:
        - protocol: TCP
          port: 6443
  ingress:
    # Allow Prometheus/monitoring to scrape metrics
    - from:
      - namespaceSelector:
          matchLabels:
            name: openshift-user-workload-monitoring
      ports:
        - protocol: TCP
          port: 8080`)

func externalSecretsNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYmlBytes() ([]byte, error) {
	return _externalSecretsNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYml, nil
}

func externalSecretsNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYml() (*asset, error) {
	bytes, err := externalSecretsNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/networkpolicies/allow-api-server-egress-for-main-controller-traffic.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsNetworkpoliciesAllowApiServerEgressForWebhookTrafficYml = []byte(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: eso-sys-allow-api-server-egress-for-webhook
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v1.2.0"
    app.kubernetes.io/managed-by: external-secrets-operator
    external-secrets.io/component: webhook
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: external-secrets-webhook
  policyTypes:
    - Egress
    - Ingress
  egress:
    - ports:
        - protocol: TCP
          port: 6443
  ingress:
    - ports:
        - protocol: TCP
          port: 10250
    # Allow Prometheus/monitoring to scrape metrics
    - from:
      - namespaceSelector:
          matchLabels:
            name: openshift-user-workload-monitoring
      ports:
        - protocol: TCP
          port: 8080`)

func externalSecretsNetworkpoliciesAllowApiServerEgressForWebhookTrafficYmlBytes() ([]byte, error) {
	return _externalSecretsNetworkpoliciesAllowApiServerEgressForWebhookTrafficYml, nil
}

func externalSecretsNetworkpoliciesAllowApiServerEgressForWebhookTrafficYml() (*asset, error) {
	bytes, err := externalSecretsNetworkpoliciesAllowApiServerEgressForWebhookTrafficYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/networkpolicies/allow-api-server-egress-for-webhook-traffic.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsNetworkpoliciesAllowDnsYml = []byte(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v1.2.0"
    app.kubernetes.io/managed-by: external-secrets-operator
  name: eso-sys-allow-to-dns
spec:
  podSelector:
    matchExpressions:
      - key: app.kubernetes.io/name
        operator: In
        values:
          - external-secrets
          - bitwarden-sdk-server
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: openshift-dns
          podSelector:
            matchLabels:
              dns.operator.openshift.io/daemonset-dns: default
      ports:
        - protocol: TCP
          port: 5353
        - protocol: UDP
          port: 5353
        - protocol: TCP
          port: 53
        - protocol: UDP
          port: 53
  policyTypes:
      - Egress`)

func externalSecretsNetworkpoliciesAllowDnsYmlBytes() ([]byte, error) {
	return _externalSecretsNetworkpoliciesAllowDnsYml, nil
}

func externalSecretsNetworkpoliciesAllowDnsYml() (*asset, error) {
	bytes, err := externalSecretsNetworkpoliciesAllowDnsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/networkpolicies/allow-dns.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsNetworkpoliciesDenyAllYml = []byte(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: eso-sys-deny-all-traffic
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v1.2.0"
    app.kubernetes.io/managed-by: external-secrets-operator
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress`)

func externalSecretsNetworkpoliciesDenyAllYmlBytes() ([]byte, error) {
	return _externalSecretsNetworkpoliciesDenyAllYml, nil
}

func externalSecretsNetworkpoliciesDenyAllYml() (*asset, error) {
	bytes, err := externalSecretsNetworkpoliciesDenyAllYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/networkpolicies/deny-all.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandCertificate_bitwardenTlsCertsYml = []byte(`apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: bitwarden-tls-certs
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: bitwarden-tls-certs
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
    app.kubernetes.io/managed-by: external-secrets-operator
spec:
  secretName: bitwarden-tls-certs
  dnsNames:
    - bitwarden-sdk-server.external-secrets.svc.cluster.local
    - external-secrets-bitwarden-sdk-server.external-secrets.svc.cluster.local
    - localhost
  ipAddresses:
    - 127.0.0.1
    - ::1
  privateKey:
    algorithm: RSA
    encoding: PKCS8
    size: 2048
  issuerRef:
    group: cert-manager.io
    kind: Issuer
    name: my-issuer
  duration: "8760h"`)

func externalSecretsOperandCertificate_bitwardenTlsCertsYmlBytes() ([]byte, error) {
	return _externalSecretsOperandCertificate_bitwardenTlsCertsYml, nil
}

func externalSecretsOperandCertificate_bitwardenTlsCertsYml() (*asset, error) {
	bytes, err := externalSecretsOperandCertificate_bitwardenTlsCertsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/certificate_bitwarden-tls-certs.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandCertificate_externalSecretsWebhookYml = []byte(`---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: external-secrets-webhook
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
    external-secrets.io/component: webhook
spec:
  commonName: external-secrets-webhook
  dnsNames:
    - external-secrets-webhook
    - external-secrets-webhook.external-secrets
    - external-secrets-webhook.external-secrets.svc
  issuerRef:
    group: cert-manager.io
    kind: Issuer
    name: my-issuer
  duration: "8760h0m0s"
  secretName: external-secrets-webhook
`)

func externalSecretsOperandCertificate_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _externalSecretsOperandCertificate_externalSecretsWebhookYml, nil
}

func externalSecretsOperandCertificate_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := externalSecretsOperandCertificate_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/certificate_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandClusterrole_externalSecretsCertControllerYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-secrets-cert-controller
  labels:
    app.kubernetes.io/name: external-secrets-cert-controller
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
rules:
  - apiGroups:
      - "apiextensions.k8s.io"
    resources:
      - "customresourcedefinitions"
    verbs:
      - "get"
      - "list"
      - "watch"
      - "update"
      - "patch"
  - apiGroups:
      - "admissionregistration.k8s.io"
    resources:
      - "validatingwebhookconfigurations"
    verbs:
      - "list"
      - "watch"
      - "get"
  - apiGroups:
      - "admissionregistration.k8s.io"
    resources:
      - "validatingwebhookconfigurations"
    resourceNames:
      - "secretstore-validate"
      - "externalsecret-validate"
    verbs:
      - "update"
      - "patch"
  - apiGroups:
      - ""
    resources:
      - "endpoints"
    verbs:
      - "list"
      - "get"
      - "watch"
  - apiGroups:
      - "discovery.k8s.io"
    resources:
      - "endpointslices"
    verbs:
      - "list"
      - "get"
      - "watch"
  - apiGroups:
      - ""
    resources:
      - "events"
    verbs:
      - "create"
      - "patch"
  - apiGroups:
      - ""
    resources:
      - "secrets"
    verbs:
      - "get"
      - "list"
      - "watch"
      - "update"
      - "patch"
  - apiGroups:
      - "coordination.k8s.io"
    resources:
      - "leases"
    verbs:
      - "get"
      - "create"
      - "update"
      - "patch"
`)

func externalSecretsOperandClusterrole_externalSecretsCertControllerYmlBytes() ([]byte, error) {
	return _externalSecretsOperandClusterrole_externalSecretsCertControllerYml, nil
}

func externalSecretsOperandClusterrole_externalSecretsCertControllerYml() (*asset, error) {
	bytes, err := externalSecretsOperandClusterrole_externalSecretsCertControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/clusterrole_external-secrets-cert-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandClusterrole_externalSecretsControllerYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-secrets-controller
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
rules:
  - apiGroups:
      - "external-secrets.io"
    resources:
      - "secretstores"
      - "clustersecretstores"
      - "externalsecrets"
      - "clusterexternalsecrets"
      - "pushsecrets"
      - "clusterpushsecrets"
    verbs:
      - "get"
      - "list"
      - "watch"
  - apiGroups:
      - "external-secrets.io"
    resources:
      - "externalsecrets"
      - "externalsecrets/status"
      - "externalsecrets/finalizers"
      - "secretstores"
      - "secretstores/status"
      - "secretstores/finalizers"
      - "clustersecretstores"
      - "clustersecretstores/status"
      - "clustersecretstores/finalizers"
      - "clusterexternalsecrets"
      - "clusterexternalsecrets/status"
      - "clusterexternalsecrets/finalizers"
      - "pushsecrets"
      - "pushsecrets/status"
      - "pushsecrets/finalizers"
      - "clusterpushsecrets"
      - "clusterpushsecrets/status"
      - "clusterpushsecrets/finalizers"
    verbs:
      - "get"
      - "update"
      - "patch"
  - apiGroups:
      - "generators.external-secrets.io"
    resources:
      - "generatorstates"
    verbs:
      - "get"
      - "list"
      - "watch"
      - "create"
      - "update"
      - "patch"
      - "delete"
      - "deletecollection"
  - apiGroups:
      - "generators.external-secrets.io"
    resources:
      - "acraccesstokens"
      - "cloudsmithaccesstokens"
      - "clustergenerators"
      - "ecrauthorizationtokens"
      - "fakes"
      - "gcraccesstokens"
      - "githubaccesstokens"
      - "quayaccesstokens"
      - "passwords"
      - "sshkeys"
      - "stssessiontokens"
      - "uuids"
      - "vaultdynamicsecrets"
      - "webhooks"
      - "grafanas"
      - "mfas"
    verbs:
      - "get"
      - "list"
      - "watch"
  - apiGroups:
      - ""
    resources:
      - "serviceaccounts"
      - "namespaces"
    verbs:
      - "get"
      - "list"
      - "watch"
  - apiGroups:
      - ""
    resources:
      - "namespaces"
    verbs:
      - "update"
      - "patch"
  - apiGroups:
      - ""
    resources:
      - "configmaps"
    verbs:
      - "get"
      - "list"
      - "watch"
  - apiGroups:
      - ""
    resources:
      - "secrets"
    verbs:
      - "get"
      - "list"
      - "watch"
      - "create"
      - "update"
      - "delete"
      - "patch"
  - apiGroups:
      - ""
    resources:
      - "serviceaccounts/token"
    verbs:
      - "create"
  - apiGroups:
      - ""
    resources:
      - "events"
    verbs:
      - "create"
      - "patch"
  - apiGroups:
      - "external-secrets.io"
    resources:
      - "externalsecrets"
    verbs:
      - "create"
      - "update"
      - "delete"
  - apiGroups:
      - "external-secrets.io"
    resources:
      - "pushsecrets"
    verbs:
      - "create"
      - "update"
      - "delete"
`)

func externalSecretsOperandClusterrole_externalSecretsControllerYmlBytes() ([]byte, error) {
	return _externalSecretsOperandClusterrole_externalSecretsControllerYml, nil
}

func externalSecretsOperandClusterrole_externalSecretsControllerYml() (*asset, error) {
	bytes, err := externalSecretsOperandClusterrole_externalSecretsControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/clusterrole_external-secrets-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandClusterrole_externalSecretsEditYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-secrets-edit
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
    rbac.authorization.k8s.io/aggregate-to-edit: "true"
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
rules:
  - apiGroups:
      - "external-secrets.io"
    resources:
      - "externalsecrets"
      - "secretstores"
      - "clustersecretstores"
      - "pushsecrets"
      - "clusterpushsecrets"
    verbs:
      - "create"
      - "delete"
      - "deletecollection"
      - "patch"
      - "update"
  - apiGroups:
      - "generators.external-secrets.io"
    resources:
      - "acraccesstokens"
      - "cloudsmithaccesstokens"
      - "clustergenerators"
      - "ecrauthorizationtokens"
      - "fakes"
      - "gcraccesstokens"
      - "githubaccesstokens"
      - "quayaccesstokens"
      - "passwords"
      - "sshkeys"
      - "vaultdynamicsecrets"
      - "webhooks"
      - "grafanas"
      - "generatorstates"
      - "mfas"
      - "uuids"
    verbs:
      - "create"
      - "delete"
      - "deletecollection"
      - "patch"
      - "update"
`)

func externalSecretsOperandClusterrole_externalSecretsEditYmlBytes() ([]byte, error) {
	return _externalSecretsOperandClusterrole_externalSecretsEditYml, nil
}

func externalSecretsOperandClusterrole_externalSecretsEditYml() (*asset, error) {
	bytes, err := externalSecretsOperandClusterrole_externalSecretsEditYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/clusterrole_external-secrets-edit.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandClusterrole_externalSecretsServicebindingsYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-secrets-servicebindings
  labels:
    servicebinding.io/controller: "true"
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
rules:
  - apiGroups:
      - "external-secrets.io"
    resources:
      - "externalsecrets"
      - "pushsecrets"
    verbs:
      - "get"
      - "list"
      - "watch"
`)

func externalSecretsOperandClusterrole_externalSecretsServicebindingsYmlBytes() ([]byte, error) {
	return _externalSecretsOperandClusterrole_externalSecretsServicebindingsYml, nil
}

func externalSecretsOperandClusterrole_externalSecretsServicebindingsYml() (*asset, error) {
	bytes, err := externalSecretsOperandClusterrole_externalSecretsServicebindingsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/clusterrole_external-secrets-servicebindings.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandClusterrole_externalSecretsViewYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-secrets-view
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
    rbac.authorization.k8s.io/aggregate-to-view: "true"
    rbac.authorization.k8s.io/aggregate-to-edit: "true"
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
rules:
  - apiGroups:
      - "external-secrets.io"
    resources:
      - "externalsecrets"
      - "secretstores"
      - "clustersecretstores"
      - "pushsecrets"
      - "clusterpushsecrets"
    verbs:
      - "get"
      - "watch"
      - "list"
  - apiGroups:
      - "generators.external-secrets.io"
    resources:
      - "acraccesstokens"
      - "cloudsmithaccesstokens"
      - "clustergenerators"
      - "ecrauthorizationtokens"
      - "fakes"
      - "gcraccesstokens"
      - "githubaccesstokens"
      - "quayaccesstokens"
      - "passwords"
      - "sshkeys"
      - "vaultdynamicsecrets"
      - "webhooks"
      - "grafanas"
      - "generatorstates"
      - "mfas"
      - "uuids"
    verbs:
      - "get"
      - "watch"
      - "list"
`)

func externalSecretsOperandClusterrole_externalSecretsViewYmlBytes() ([]byte, error) {
	return _externalSecretsOperandClusterrole_externalSecretsViewYml, nil
}

func externalSecretsOperandClusterrole_externalSecretsViewYml() (*asset, error) {
	bytes, err := externalSecretsOperandClusterrole_externalSecretsViewYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/clusterrole_external-secrets-view.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandClusterrolebinding_externalSecretsCertControllerYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: external-secrets-cert-controller
  labels:
    app.kubernetes.io/name: external-secrets-cert-controller
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: external-secrets-cert-controller
subjects:
  - name: external-secrets-cert-controller
    namespace: external-secrets
    kind: ServiceAccount
`)

func externalSecretsOperandClusterrolebinding_externalSecretsCertControllerYmlBytes() ([]byte, error) {
	return _externalSecretsOperandClusterrolebinding_externalSecretsCertControllerYml, nil
}

func externalSecretsOperandClusterrolebinding_externalSecretsCertControllerYml() (*asset, error) {
	bytes, err := externalSecretsOperandClusterrolebinding_externalSecretsCertControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/clusterrolebinding_external-secrets-cert-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandClusterrolebinding_externalSecretsControllerYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: external-secrets-controller
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: external-secrets-controller
subjects:
  - name: external-secrets
    namespace: external-secrets
    kind: ServiceAccount
`)

func externalSecretsOperandClusterrolebinding_externalSecretsControllerYmlBytes() ([]byte, error) {
	return _externalSecretsOperandClusterrolebinding_externalSecretsControllerYml, nil
}

func externalSecretsOperandClusterrolebinding_externalSecretsControllerYml() (*asset, error) {
	bytes, err := externalSecretsOperandClusterrolebinding_externalSecretsControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/clusterrolebinding_external-secrets-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandDeployment_bitwardenSdkServerYml = []byte(`---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bitwarden-sdk-server
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: bitwarden-sdk-server
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.6.0"
    app.kubernetes.io/managed-by: external-secrets-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: bitwarden-sdk-server
      app.kubernetes.io/instance: external-secrets
  template:
    metadata:
      labels:
        app.kubernetes.io/name: bitwarden-sdk-server
        app.kubernetes.io/instance: external-secrets
    spec:
      serviceAccountName: bitwarden-sdk-server
      securityContext: {}
      containers:
        - name: bitwarden-sdk-server
          securityContext: {}
          image: "ghcr.io/external-secrets/bitwarden-sdk-server:v0.6.0"
          imagePullPolicy: IfNotPresent
          volumeMounts:
            - mountPath: /certs
              name: bitwarden-tls-certs
          ports:
            - name: http
              containerPort: 9998
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /live
              port: http
              scheme: HTTPS
          readinessProbe:
            httpGet:
              path: /ready
              port: http
              scheme: HTTPS
          resources: {}
      volumes:
        - name: bitwarden-tls-certs
          secret:
            secretName: bitwarden-tls-certs
            items:
              - key: tls.crt
                path: cert.pem
              - key: tls.key
                path: key.pem
              - key: ca.crt
                path: ca.pem
`)

func externalSecretsOperandDeployment_bitwardenSdkServerYmlBytes() ([]byte, error) {
	return _externalSecretsOperandDeployment_bitwardenSdkServerYml, nil
}

func externalSecretsOperandDeployment_bitwardenSdkServerYml() (*asset, error) {
	bytes, err := externalSecretsOperandDeployment_bitwardenSdkServerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/deployment_bitwarden-sdk-server.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandDeployment_externalSecretsCertControllerYml = []byte(`---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: external-secrets-cert-controller
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-cert-controller
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
spec:
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app.kubernetes.io/name: external-secrets-cert-controller
      app.kubernetes.io/instance: external-secrets
  template:
    metadata:
      labels:
        app.kubernetes.io/name: external-secrets-cert-controller
        app.kubernetes.io/instance: external-secrets
        app.kubernetes.io/version: "v2.5.0"
        app.kubernetes.io/managed-by: external-secrets-operator
    spec:
      serviceAccountName: external-secrets-cert-controller
      automountServiceAccountToken: true
      hostNetwork: false
      containers:
        - name: cert-controller
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
            runAsNonRoot: true
            runAsUser: 1000
            seccompProfile:
              type: RuntimeDefault
          image: ghcr.io/external-secrets/external-secrets:v2.5.0
          imagePullPolicy: IfNotPresent
          args:
            - certcontroller
            - --crd-requeue-interval=5m
            - --service-name=external-secrets-webhook
            - --service-namespace=external-secrets
            - --secret-name=external-secrets-webhook
            - --secret-namespace=external-secrets
            - --metrics-addr=:8080
            - --healthz-addr=:8081
            - --loglevel=info
            - --zap-time-encoding=epoch
            - --enable-partial-cache=true
          ports:
            - containerPort: 8080
              protocol: TCP
              name: metrics
            - containerPort: 8081
              protocol: TCP
              name: ready
          readinessProbe:
            httpGet:
              port: ready
              path: /readyz
            initialDelaySeconds: 20
            periodSeconds: 5
            timeoutSeconds: 5
            failureThreshold: 3
            successThreshold: 1
`)

func externalSecretsOperandDeployment_externalSecretsCertControllerYmlBytes() ([]byte, error) {
	return _externalSecretsOperandDeployment_externalSecretsCertControllerYml, nil
}

func externalSecretsOperandDeployment_externalSecretsCertControllerYml() (*asset, error) {
	bytes, err := externalSecretsOperandDeployment_externalSecretsCertControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/deployment_external-secrets-cert-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandDeployment_externalSecretsWebhookYml = []byte(`---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: external-secrets-webhook
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
spec:
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app.kubernetes.io/name: external-secrets-webhook
      app.kubernetes.io/instance: external-secrets
  template:
    metadata:
      labels:
        app.kubernetes.io/name: external-secrets-webhook
        app.kubernetes.io/instance: external-secrets
        app.kubernetes.io/version: "v2.5.0"
        app.kubernetes.io/managed-by: external-secrets-operator
    spec:
      hostNetwork: false
      serviceAccountName: external-secrets-webhook
      automountServiceAccountToken: true
      containers:
        - name: webhook
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
            runAsNonRoot: true
            runAsUser: 1000
            seccompProfile:
              type: RuntimeDefault
          image: ghcr.io/external-secrets/external-secrets:v2.5.0
          imagePullPolicy: IfNotPresent
          args:
            - webhook
            - --port=10250
            - --dns-name=external-secrets-webhook.external-secrets.svc
            - --cert-dir=/tmp/certs
            - --check-interval=5m
            - --metrics-addr=:8080
            - --healthz-addr=:8081
            - --loglevel=info
            - --zap-time-encoding=epoch
          ports:
            - containerPort: 8080
              protocol: TCP
              name: metrics
            - containerPort: 10250
              protocol: TCP
              name: webhook
            - containerPort: 8081
              protocol: TCP
              name: ready
          readinessProbe:
            httpGet:
              port: ready
              path: /readyz
            initialDelaySeconds: 20
            periodSeconds: 5
            timeoutSeconds: 5
            failureThreshold: 3
            successThreshold: 1
          volumeMounts:
            - name: certs
              mountPath: /tmp/certs
              readOnly: true
      volumes:
        - name: certs
          secret:
            secretName: external-secrets-webhook
`)

func externalSecretsOperandDeployment_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _externalSecretsOperandDeployment_externalSecretsWebhookYml, nil
}

func externalSecretsOperandDeployment_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := externalSecretsOperandDeployment_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/deployment_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandDeployment_externalSecretsYml = []byte(`---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: external-secrets
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
spec:
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app.kubernetes.io/name: external-secrets
      app.kubernetes.io/instance: external-secrets
  template:
    metadata:
      labels:
        app.kubernetes.io/name: external-secrets
        app.kubernetes.io/instance: external-secrets
        app.kubernetes.io/version: "v2.5.0"
        app.kubernetes.io/managed-by: external-secrets-operator
    spec:
      serviceAccountName: external-secrets
      automountServiceAccountToken: true
      hostNetwork: false
      containers:
        - name: external-secrets
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
            runAsNonRoot: true
            runAsUser: 1000
            seccompProfile:
              type: RuntimeDefault
          image: ghcr.io/external-secrets/external-secrets:v2.5.0
          imagePullPolicy: IfNotPresent
          args:
            - --concurrent=1
            - --metrics-addr=:8080
            - --loglevel=info
            - --zap-time-encoding=epoch
            - --enable-leader-election=false
            - --enable-cluster-store-reconciler=false
            - --enable-cluster-external-secret-reconciler=false
            - --enable-push-secret-reconciler=false
          ports:
            - containerPort: 8080
              protocol: TCP
              name: metrics
      dnsPolicy: ClusterFirst
`)

func externalSecretsOperandDeployment_externalSecretsYmlBytes() ([]byte, error) {
	return _externalSecretsOperandDeployment_externalSecretsYml, nil
}

func externalSecretsOperandDeployment_externalSecretsYml() (*asset, error) {
	bytes, err := externalSecretsOperandDeployment_externalSecretsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/deployment_external-secrets.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandNamespace_externalSecretsYml = []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: external-secrets
`)

func externalSecretsOperandNamespace_externalSecretsYmlBytes() ([]byte, error) {
	return _externalSecretsOperandNamespace_externalSecretsYml, nil
}

func externalSecretsOperandNamespace_externalSecretsYml() (*asset, error) {
	bytes, err := externalSecretsOperandNamespace_externalSecretsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/namespace_external-secrets.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandRole_externalSecretsLeaderelectionYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: external-secrets-leaderelection
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
rules:
  - apiGroups:
      - ""
    resources:
      - "configmaps"
    resourceNames:
      - "external-secrets-controller"
    verbs:
      - "get"
      - "update"
      - "patch"
  - apiGroups:
      - ""
    resources:
      - "configmaps"
    verbs:
      - "create"
  - apiGroups:
      - "coordination.k8s.io"
    resources:
      - "leases"
    verbs:
      - "get"
      - "create"
      - "update"
      - "patch"
`)

func externalSecretsOperandRole_externalSecretsLeaderelectionYmlBytes() ([]byte, error) {
	return _externalSecretsOperandRole_externalSecretsLeaderelectionYml, nil
}

func externalSecretsOperandRole_externalSecretsLeaderelectionYml() (*asset, error) {
	bytes, err := externalSecretsOperandRole_externalSecretsLeaderelectionYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/role_external-secrets-leaderelection.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandRolebinding_externalSecretsLeaderelectionYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: external-secrets-leaderelection
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: external-secrets-leaderelection
subjects:
  - kind: ServiceAccount
    name: external-secrets
    namespace: external-secrets
`)

func externalSecretsOperandRolebinding_externalSecretsLeaderelectionYmlBytes() ([]byte, error) {
	return _externalSecretsOperandRolebinding_externalSecretsLeaderelectionYml, nil
}

func externalSecretsOperandRolebinding_externalSecretsLeaderelectionYml() (*asset, error) {
	bytes, err := externalSecretsOperandRolebinding_externalSecretsLeaderelectionYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/rolebinding_external-secrets-leaderelection.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandSecret_externalSecretsWebhookYml = []byte(`---
apiVersion: v1
kind: Secret
metadata:
  name: external-secrets-webhook
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
    external-secrets.io/component: webhook
`)

func externalSecretsOperandSecret_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _externalSecretsOperandSecret_externalSecretsWebhookYml, nil
}

func externalSecretsOperandSecret_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := externalSecretsOperandSecret_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/secret_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandService_bitwardenSdkServerYml = []byte(`---
apiVersion: v1
kind: Service
metadata:
  name: bitwarden-sdk-server
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: bitwarden-sdk-server
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.6.0"
    app.kubernetes.io/managed-by: external-secrets-operator
spec:
  type: ClusterIP
  ports:
    - port: 9998
      targetPort: http
      name: http
  selector:
    app.kubernetes.io/name: bitwarden-sdk-server
    app.kubernetes.io/instance: external-secrets
`)

func externalSecretsOperandService_bitwardenSdkServerYmlBytes() ([]byte, error) {
	return _externalSecretsOperandService_bitwardenSdkServerYml, nil
}

func externalSecretsOperandService_bitwardenSdkServerYml() (*asset, error) {
	bytes, err := externalSecretsOperandService_bitwardenSdkServerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/service_bitwarden-sdk-server.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandService_externalSecretsCertControllerMetricsYml = []byte(`---
apiVersion: v1
kind: Service
metadata:
  name: external-secrets-cert-controller-metrics
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-cert-controller
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
spec:
  type: ClusterIP
  ports:
    - port: 8080
      protocol: TCP
      targetPort: metrics
      name: metrics
  selector:
    app.kubernetes.io/name: external-secrets-cert-controller
    app.kubernetes.io/instance: external-secrets
`)

func externalSecretsOperandService_externalSecretsCertControllerMetricsYmlBytes() ([]byte, error) {
	return _externalSecretsOperandService_externalSecretsCertControllerMetricsYml, nil
}

func externalSecretsOperandService_externalSecretsCertControllerMetricsYml() (*asset, error) {
	bytes, err := externalSecretsOperandService_externalSecretsCertControllerMetricsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/service_external-secrets-cert-controller-metrics.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandService_externalSecretsMetricsYml = []byte(`---
apiVersion: v1
kind: Service
metadata:
  name: external-secrets-metrics
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
spec:
  type: ClusterIP
  ports:
    - port: 8080
      protocol: TCP
      targetPort: metrics
      name: metrics
  selector:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
`)

func externalSecretsOperandService_externalSecretsMetricsYmlBytes() ([]byte, error) {
	return _externalSecretsOperandService_externalSecretsMetricsYml, nil
}

func externalSecretsOperandService_externalSecretsMetricsYml() (*asset, error) {
	bytes, err := externalSecretsOperandService_externalSecretsMetricsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/service_external-secrets-metrics.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandService_externalSecretsWebhookYml = []byte(`---
apiVersion: v1
kind: Service
metadata:
  name: external-secrets-webhook
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
    external-secrets.io/component: webhook
spec:
  type: ClusterIP
  ports:
    - port: 443
      targetPort: webhook
      protocol: TCP
      name: webhook
    - port: 8080
      protocol: TCP
      targetPort: metrics
      name: metrics
  selector:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
`)

func externalSecretsOperandService_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _externalSecretsOperandService_externalSecretsWebhookYml, nil
}

func externalSecretsOperandService_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := externalSecretsOperandService_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/service_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandServiceaccount_bitwardenSdkServerYml = []byte(`---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: bitwarden-sdk-server
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: bitwarden-sdk-server
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.6.0"
    app.kubernetes.io/managed-by: external-secrets-operator
`)

func externalSecretsOperandServiceaccount_bitwardenSdkServerYmlBytes() ([]byte, error) {
	return _externalSecretsOperandServiceaccount_bitwardenSdkServerYml, nil
}

func externalSecretsOperandServiceaccount_bitwardenSdkServerYml() (*asset, error) {
	bytes, err := externalSecretsOperandServiceaccount_bitwardenSdkServerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/serviceaccount_bitwarden-sdk-server.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandServiceaccount_externalSecretsCertControllerYml = []byte(`---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: external-secrets-cert-controller
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-cert-controller
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
`)

func externalSecretsOperandServiceaccount_externalSecretsCertControllerYmlBytes() ([]byte, error) {
	return _externalSecretsOperandServiceaccount_externalSecretsCertControllerYml, nil
}

func externalSecretsOperandServiceaccount_externalSecretsCertControllerYml() (*asset, error) {
	bytes, err := externalSecretsOperandServiceaccount_externalSecretsCertControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/serviceaccount_external-secrets-cert-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandServiceaccount_externalSecretsWebhookYml = []byte(`---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: external-secrets-webhook
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
`)

func externalSecretsOperandServiceaccount_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _externalSecretsOperandServiceaccount_externalSecretsWebhookYml, nil
}

func externalSecretsOperandServiceaccount_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := externalSecretsOperandServiceaccount_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/serviceaccount_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandServiceaccount_externalSecretsYml = []byte(`---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: external-secrets
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
`)

func externalSecretsOperandServiceaccount_externalSecretsYmlBytes() ([]byte, error) {
	return _externalSecretsOperandServiceaccount_externalSecretsYml, nil
}

func externalSecretsOperandServiceaccount_externalSecretsYml() (*asset, error) {
	bytes, err := externalSecretsOperandServiceaccount_externalSecretsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/serviceaccount_external-secrets.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandValidatingwebhookconfiguration_externalsecretValidateYml = []byte(`---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: externalsecret-validate
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
    external-secrets.io/component: webhook
webhooks:
  - name: "validate.externalsecret.external-secrets.io"
    rules:
      - apiGroups: ["external-secrets.io"]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE", "DELETE"]
        resources: ["externalsecrets"]
        scope: "Namespaced"
    clientConfig:
      service:
        namespace: external-secrets
        name: external-secrets-webhook
        path: /validate-external-secrets-io-v1-externalsecret
    admissionReviewVersions: ["v1", "v1beta1"]
    sideEffects: None
    timeoutSeconds: 5
    failurePolicy: Fail
`)

func externalSecretsOperandValidatingwebhookconfiguration_externalsecretValidateYmlBytes() ([]byte, error) {
	return _externalSecretsOperandValidatingwebhookconfiguration_externalsecretValidateYml, nil
}

func externalSecretsOperandValidatingwebhookconfiguration_externalsecretValidateYml() (*asset, error) {
	bytes, err := externalSecretsOperandValidatingwebhookconfiguration_externalsecretValidateYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/validatingwebhookconfiguration_externalsecret-validate.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsOperandValidatingwebhookconfiguration_secretstoreValidateYml = []byte(`---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: secretstore-validate
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v2.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
    external-secrets.io/component: webhook
webhooks:
  - name: "validate.secretstore.external-secrets.io"
    rules:
      - apiGroups: ["external-secrets.io"]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE", "DELETE"]
        resources: ["secretstores"]
        scope: "Namespaced"
    clientConfig:
      service:
        namespace: external-secrets
        name: external-secrets-webhook
        path: /validate-external-secrets-io-v1-secretstore
    admissionReviewVersions: ["v1", "v1beta1"]
    sideEffects: None
    timeoutSeconds: 5
    failurePolicy: Fail
  - name: "validate.clustersecretstore.external-secrets.io"
    rules:
      - apiGroups: ["external-secrets.io"]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE", "DELETE"]
        resources: ["clustersecretstores"]
        scope: "Cluster"
    clientConfig:
      service:
        namespace: external-secrets
        name: external-secrets-webhook
        path: /validate-external-secrets-io-v1-clustersecretstore
    admissionReviewVersions: ["v1", "v1beta1"]
    sideEffects: None
    timeoutSeconds: 5
    failurePolicy: Fail
`)

func externalSecretsOperandValidatingwebhookconfiguration_secretstoreValidateYmlBytes() ([]byte, error) {
	return _externalSecretsOperandValidatingwebhookconfiguration_secretstoreValidateYml, nil
}

func externalSecretsOperandValidatingwebhookconfiguration_secretstoreValidateYml() (*asset, error) {
	bytes, err := externalSecretsOperandValidatingwebhookconfiguration_secretstoreValidateYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/operand/validatingwebhookconfiguration_secretstore-validate.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"external-secrets/networkpolicies/allow-api-server-egress-for-bitwarden-sever.yml":         externalSecretsNetworkpoliciesAllowApiServerEgressForBitwardenSeverYml,
	"external-secrets/networkpolicies/allow-api-server-egress-for-cert-controller-traffic.yml": externalSecretsNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYml,
	"external-secrets/networkpolicies/allow-api-server-egress-for-main-controller-traffic.yml": externalSecretsNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYml,
	"external-secrets/networkpolicies/allow-api-server-egress-for-webhook-traffic.yml":         externalSecretsNetworkpoliciesAllowApiServerEgressForWebhookTrafficYml,
	"external-secrets/networkpolicies/allow-dns.yml":                                           externalSecretsNetworkpoliciesAllowDnsYml,
	"external-secrets/networkpolicies/deny-all.yml":                                            externalSecretsNetworkpoliciesDenyAllYml,
	"external-secrets/operand/certificate_bitwarden-tls-certs.yml":                             externalSecretsOperandCertificate_bitwardenTlsCertsYml,
	"external-secrets/operand/certificate_external-secrets-webhook.yml":                        externalSecretsOperandCertificate_externalSecretsWebhookYml,
	"external-secrets/operand/clusterrole_external-secrets-cert-controller.yml":                externalSecretsOperandClusterrole_externalSecretsCertControllerYml,
	"external-secrets/operand/clusterrole_external-secrets-controller.yml":                     externalSecretsOperandClusterrole_externalSecretsControllerYml,
	"external-secrets/operand/clusterrole_external-secrets-edit.yml":                           externalSecretsOperandClusterrole_externalSecretsEditYml,
	"external-secrets/operand/clusterrole_external-secrets-servicebindings.yml":                externalSecretsOperandClusterrole_externalSecretsServicebindingsYml,
	"external-secrets/operand/clusterrole_external-secrets-view.yml":                           externalSecretsOperandClusterrole_externalSecretsViewYml,
	"external-secrets/operand/clusterrolebinding_external-secrets-cert-controller.yml":         externalSecretsOperandClusterrolebinding_externalSecretsCertControllerYml,
	"external-secrets/operand/clusterrolebinding_external-secrets-controller.yml":              externalSecretsOperandClusterrolebinding_externalSecretsControllerYml,
	"external-secrets/operand/deployment_bitwarden-sdk-server.yml":                             externalSecretsOperandDeployment_bitwardenSdkServerYml,
	"external-secrets/operand/deployment_external-secrets-cert-controller.yml":                 externalSecretsOperandDeployment_externalSecretsCertControllerYml,
	"external-secrets/operand/deployment_external-secrets-webhook.yml":                         externalSecretsOperandDeployment_externalSecretsWebhookYml,
	"external-secrets/operand/deployment_external-secrets.yml":                                 externalSecretsOperandDeployment_externalSecretsYml,
	"external-secrets/operand/namespace_external-secrets.yml":                                  externalSecretsOperandNamespace_externalSecretsYml,
	"external-secrets/operand/role_external-secrets-leaderelection.yml":                        externalSecretsOperandRole_externalSecretsLeaderelectionYml,
	"external-secrets/operand/rolebinding_external-secrets-leaderelection.yml":                 externalSecretsOperandRolebinding_externalSecretsLeaderelectionYml,
	"external-secrets/operand/secret_external-secrets-webhook.yml":                             externalSecretsOperandSecret_externalSecretsWebhookYml,
	"external-secrets/operand/service_bitwarden-sdk-server.yml":                                externalSecretsOperandService_bitwardenSdkServerYml,
	"external-secrets/operand/service_external-secrets-cert-controller-metrics.yml":            externalSecretsOperandService_externalSecretsCertControllerMetricsYml,
	"external-secrets/operand/service_external-secrets-metrics.yml":                            externalSecretsOperandService_externalSecretsMetricsYml,
	"external-secrets/operand/service_external-secrets-webhook.yml":                            externalSecretsOperandService_externalSecretsWebhookYml,
	"external-secrets/operand/serviceaccount_bitwarden-sdk-server.yml":                         externalSecretsOperandServiceaccount_bitwardenSdkServerYml,
	"external-secrets/operand/serviceaccount_external-secrets-cert-controller.yml":             externalSecretsOperandServiceaccount_externalSecretsCertControllerYml,
	"external-secrets/operand/serviceaccount_external-secrets-webhook.yml":                     externalSecretsOperandServiceaccount_externalSecretsWebhookYml,
	"external-secrets/operand/serviceaccount_external-secrets.yml":                             externalSecretsOperandServiceaccount_externalSecretsYml,
	"external-secrets/operand/validatingwebhookconfiguration_externalsecret-validate.yml":      externalSecretsOperandValidatingwebhookconfiguration_externalsecretValidateYml,
	"external-secrets/operand/validatingwebhookconfiguration_secretstore-validate.yml":         externalSecretsOperandValidatingwebhookconfiguration_secretstoreValidateYml,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//
//	data/
//	  foo.txt
//	  img/
//	    a.png
//	    b.png
//
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"external-secrets": {nil, map[string]*bintree{
		"networkpolicies": {nil, map[string]*bintree{
			"allow-api-server-egress-for-bitwarden-sever.yml":         {externalSecretsNetworkpoliciesAllowApiServerEgressForBitwardenSeverYml, map[string]*bintree{}},
			"allow-api-server-egress-for-cert-controller-traffic.yml": {externalSecretsNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYml, map[string]*bintree{}},
			"allow-api-server-egress-for-main-controller-traffic.yml": {externalSecretsNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYml, map[string]*bintree{}},
			"allow-api-server-egress-for-webhook-traffic.yml":         {externalSecretsNetworkpoliciesAllowApiServerEgressForWebhookTrafficYml, map[string]*bintree{}},
			"allow-dns.yml": {externalSecretsNetworkpoliciesAllowDnsYml, map[string]*bintree{}},
			"deny-all.yml":  {externalSecretsNetworkpoliciesDenyAllYml, map[string]*bintree{}},
		}},
		"operand": {nil, map[string]*bintree{
			"certificate_bitwarden-tls-certs.yml":                        {externalSecretsOperandCertificate_bitwardenTlsCertsYml, map[string]*bintree{}},
			"certificate_external-secrets-webhook.yml":                   {externalSecretsOperandCertificate_externalSecretsWebhookYml, map[string]*bintree{}},
			"clusterrole_external-secrets-cert-controller.yml":           {externalSecretsOperandClusterrole_externalSecretsCertControllerYml, map[string]*bintree{}},
			"clusterrole_external-secrets-controller.yml":                {externalSecretsOperandClusterrole_externalSecretsControllerYml, map[string]*bintree{}},
			"clusterrole_external-secrets-edit.yml":                      {externalSecretsOperandClusterrole_externalSecretsEditYml, map[string]*bintree{}},
			"clusterrole_external-secrets-servicebindings.yml":           {externalSecretsOperandClusterrole_externalSecretsServicebindingsYml, map[string]*bintree{}},
			"clusterrole_external-secrets-view.yml":                      {externalSecretsOperandClusterrole_externalSecretsViewYml, map[string]*bintree{}},
			"clusterrolebinding_external-secrets-cert-controller.yml":    {externalSecretsOperandClusterrolebinding_externalSecretsCertControllerYml, map[string]*bintree{}},
			"clusterrolebinding_external-secrets-controller.yml":         {externalSecretsOperandClusterrolebinding_externalSecretsControllerYml, map[string]*bintree{}},
			"deployment_bitwarden-sdk-server.yml":                        {externalSecretsOperandDeployment_bitwardenSdkServerYml, map[string]*bintree{}},
			"deployment_external-secrets-cert-controller.yml":            {externalSecretsOperandDeployment_externalSecretsCertControllerYml, map[string]*bintree{}},
			"deployment_external-secrets-webhook.yml":                    {externalSecretsOperandDeployment_externalSecretsWebhookYml, map[string]*bintree{}},
			"deployment_external-secrets.yml":                            {externalSecretsOperandDeployment_externalSecretsYml, map[string]*bintree{}},
			"namespace_external-secrets.yml":                             {externalSecretsOperandNamespace_externalSecretsYml, map[string]*bintree{}},
			"role_external-secrets-leaderelection.yml":                   {externalSecretsOperandRole_externalSecretsLeaderelectionYml, map[string]*bintree{}},
			"rolebinding_external-secrets-leaderelection.yml":            {externalSecretsOperandRolebinding_externalSecretsLeaderelectionYml, map[string]*bintree{}},
			"secret_external-secrets-webhook.yml":                        {externalSecretsOperandSecret_externalSecretsWebhookYml, map[string]*bintree{}},
			"service_bitwarden-sdk-server.yml":                           {externalSecretsOperandService_bitwardenSdkServerYml, map[string]*bintree{}},
			"service_external-secrets-cert-controller-metrics.yml":       {externalSecretsOperandService_externalSecretsCertControllerMetricsYml, map[string]*bintree{}},
			"service_external-secrets-metrics.yml":                       {externalSecretsOperandService_externalSecretsMetricsYml, map[string]*bintree{}},
			"service_external-secrets-webhook.yml":                       {externalSecretsOperandService_externalSecretsWebhookYml, map[string]*bintree{}},
			"serviceaccount_bitwarden-sdk-server.yml":                    {externalSecretsOperandServiceaccount_bitwardenSdkServerYml, map[string]*bintree{}},
			"serviceaccount_external-secrets-cert-controller.yml":        {externalSecretsOperandServiceaccount_externalSecretsCertControllerYml, map[string]*bintree{}},
			"serviceaccount_external-secrets-webhook.yml":                {externalSecretsOperandServiceaccount_externalSecretsWebhookYml, map[string]*bintree{}},
			"serviceaccount_external-secrets.yml":                        {externalSecretsOperandServiceaccount_externalSecretsYml, map[string]*bintree{}},
			"validatingwebhookconfiguration_externalsecret-validate.yml": {externalSecretsOperandValidatingwebhookconfiguration_externalsecretValidateYml, map[string]*bintree{}},
			"validatingwebhookconfiguration_secretstore-validate.yml":    {externalSecretsOperandValidatingwebhookconfiguration_secretstoreValidateYml, map[string]*bintree{}},
		}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
