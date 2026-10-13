// Code generated for package assets by go-bindata DO NOT EDIT. (@generated)
// sources:
// bindata/operands/external-secrets/certificate_bitwarden-tls-certs.yml
// bindata/operands/external-secrets/certificate_external-secrets-webhook.yml
// bindata/operands/external-secrets/clusterrole_external-secrets-cert-controller.yml
// bindata/operands/external-secrets/clusterrole_external-secrets-controller.yml
// bindata/operands/external-secrets/clusterrole_external-secrets-edit.yml
// bindata/operands/external-secrets/clusterrole_external-secrets-servicebindings.yml
// bindata/operands/external-secrets/clusterrole_external-secrets-view.yml
// bindata/operands/external-secrets/clusterrolebinding_external-secrets-cert-controller.yml
// bindata/operands/external-secrets/clusterrolebinding_external-secrets-controller.yml
// bindata/operands/external-secrets/deployment_bitwarden-sdk-server.yml
// bindata/operands/external-secrets/deployment_external-secrets-cert-controller.yml
// bindata/operands/external-secrets/deployment_external-secrets-webhook.yml
// bindata/operands/external-secrets/deployment_external-secrets.yml
// bindata/operands/external-secrets/namespace_external-secrets.yml
// bindata/operands/external-secrets/role_external-secrets-leaderelection.yml
// bindata/operands/external-secrets/rolebinding_external-secrets-leaderelection.yml
// bindata/operands/external-secrets/secret_external-secrets-webhook.yml
// bindata/operands/external-secrets/service_bitwarden-sdk-server.yml
// bindata/operands/external-secrets/service_external-secrets-cert-controller-metrics.yml
// bindata/operands/external-secrets/service_external-secrets-metrics.yml
// bindata/operands/external-secrets/service_external-secrets-webhook.yml
// bindata/operands/external-secrets/serviceaccount_bitwarden-sdk-server.yml
// bindata/operands/external-secrets/serviceaccount_external-secrets-cert-controller.yml
// bindata/operands/external-secrets/serviceaccount_external-secrets-webhook.yml
// bindata/operands/external-secrets/serviceaccount_external-secrets.yml
// bindata/operands/external-secrets/validatingwebhookconfiguration_externalsecret-validate.yml
// bindata/operands/external-secrets/validatingwebhookconfiguration_secretstore-validate.yml
// bindata/operator/networkpolicies/allow-api-server-egress-for-bitwarden-sever.yml
// bindata/operator/networkpolicies/allow-api-server-egress-for-cert-controller-traffic.yml
// bindata/operator/networkpolicies/allow-api-server-egress-for-main-controller-traffic.yml
// bindata/operator/networkpolicies/allow-api-server-egress-for-webhook-traffic.yml
// bindata/operator/networkpolicies/allow-dns.yml
// bindata/operator/networkpolicies/deny-all.yml
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

var _operandsExternalSecretsCertificate_bitwardenTlsCertsYml = []byte(`apiVersion: cert-manager.io/v1
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

func operandsExternalSecretsCertificate_bitwardenTlsCertsYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsCertificate_bitwardenTlsCertsYml, nil
}

func operandsExternalSecretsCertificate_bitwardenTlsCertsYml() (*asset, error) {
	bytes, err := operandsExternalSecretsCertificate_bitwardenTlsCertsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/certificate_bitwarden-tls-certs.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsCertificate_externalSecretsWebhookYml = []byte(`---
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

func operandsExternalSecretsCertificate_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsCertificate_externalSecretsWebhookYml, nil
}

func operandsExternalSecretsCertificate_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := operandsExternalSecretsCertificate_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/certificate_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsClusterrole_externalSecretsCertControllerYml = []byte(`---
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

func operandsExternalSecretsClusterrole_externalSecretsCertControllerYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsClusterrole_externalSecretsCertControllerYml, nil
}

func operandsExternalSecretsClusterrole_externalSecretsCertControllerYml() (*asset, error) {
	bytes, err := operandsExternalSecretsClusterrole_externalSecretsCertControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/clusterrole_external-secrets-cert-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsClusterrole_externalSecretsControllerYml = []byte(`---
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

func operandsExternalSecretsClusterrole_externalSecretsControllerYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsClusterrole_externalSecretsControllerYml, nil
}

func operandsExternalSecretsClusterrole_externalSecretsControllerYml() (*asset, error) {
	bytes, err := operandsExternalSecretsClusterrole_externalSecretsControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/clusterrole_external-secrets-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsClusterrole_externalSecretsEditYml = []byte(`---
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

func operandsExternalSecretsClusterrole_externalSecretsEditYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsClusterrole_externalSecretsEditYml, nil
}

func operandsExternalSecretsClusterrole_externalSecretsEditYml() (*asset, error) {
	bytes, err := operandsExternalSecretsClusterrole_externalSecretsEditYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/clusterrole_external-secrets-edit.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsClusterrole_externalSecretsServicebindingsYml = []byte(`---
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

func operandsExternalSecretsClusterrole_externalSecretsServicebindingsYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsClusterrole_externalSecretsServicebindingsYml, nil
}

func operandsExternalSecretsClusterrole_externalSecretsServicebindingsYml() (*asset, error) {
	bytes, err := operandsExternalSecretsClusterrole_externalSecretsServicebindingsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/clusterrole_external-secrets-servicebindings.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsClusterrole_externalSecretsViewYml = []byte(`---
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

func operandsExternalSecretsClusterrole_externalSecretsViewYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsClusterrole_externalSecretsViewYml, nil
}

func operandsExternalSecretsClusterrole_externalSecretsViewYml() (*asset, error) {
	bytes, err := operandsExternalSecretsClusterrole_externalSecretsViewYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/clusterrole_external-secrets-view.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsClusterrolebinding_externalSecretsCertControllerYml = []byte(`---
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

func operandsExternalSecretsClusterrolebinding_externalSecretsCertControllerYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsClusterrolebinding_externalSecretsCertControllerYml, nil
}

func operandsExternalSecretsClusterrolebinding_externalSecretsCertControllerYml() (*asset, error) {
	bytes, err := operandsExternalSecretsClusterrolebinding_externalSecretsCertControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/clusterrolebinding_external-secrets-cert-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsClusterrolebinding_externalSecretsControllerYml = []byte(`---
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

func operandsExternalSecretsClusterrolebinding_externalSecretsControllerYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsClusterrolebinding_externalSecretsControllerYml, nil
}

func operandsExternalSecretsClusterrolebinding_externalSecretsControllerYml() (*asset, error) {
	bytes, err := operandsExternalSecretsClusterrolebinding_externalSecretsControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/clusterrolebinding_external-secrets-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsDeployment_bitwardenSdkServerYml = []byte(`---
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

func operandsExternalSecretsDeployment_bitwardenSdkServerYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsDeployment_bitwardenSdkServerYml, nil
}

func operandsExternalSecretsDeployment_bitwardenSdkServerYml() (*asset, error) {
	bytes, err := operandsExternalSecretsDeployment_bitwardenSdkServerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/deployment_bitwarden-sdk-server.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsDeployment_externalSecretsCertControllerYml = []byte(`---
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

func operandsExternalSecretsDeployment_externalSecretsCertControllerYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsDeployment_externalSecretsCertControllerYml, nil
}

func operandsExternalSecretsDeployment_externalSecretsCertControllerYml() (*asset, error) {
	bytes, err := operandsExternalSecretsDeployment_externalSecretsCertControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/deployment_external-secrets-cert-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsDeployment_externalSecretsWebhookYml = []byte(`---
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

func operandsExternalSecretsDeployment_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsDeployment_externalSecretsWebhookYml, nil
}

func operandsExternalSecretsDeployment_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := operandsExternalSecretsDeployment_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/deployment_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsDeployment_externalSecretsYml = []byte(`---
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

func operandsExternalSecretsDeployment_externalSecretsYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsDeployment_externalSecretsYml, nil
}

func operandsExternalSecretsDeployment_externalSecretsYml() (*asset, error) {
	bytes, err := operandsExternalSecretsDeployment_externalSecretsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/deployment_external-secrets.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsNamespace_externalSecretsYml = []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: external-secrets
`)

func operandsExternalSecretsNamespace_externalSecretsYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsNamespace_externalSecretsYml, nil
}

func operandsExternalSecretsNamespace_externalSecretsYml() (*asset, error) {
	bytes, err := operandsExternalSecretsNamespace_externalSecretsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/namespace_external-secrets.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsRole_externalSecretsLeaderelectionYml = []byte(`---
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

func operandsExternalSecretsRole_externalSecretsLeaderelectionYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsRole_externalSecretsLeaderelectionYml, nil
}

func operandsExternalSecretsRole_externalSecretsLeaderelectionYml() (*asset, error) {
	bytes, err := operandsExternalSecretsRole_externalSecretsLeaderelectionYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/role_external-secrets-leaderelection.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsRolebinding_externalSecretsLeaderelectionYml = []byte(`---
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

func operandsExternalSecretsRolebinding_externalSecretsLeaderelectionYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsRolebinding_externalSecretsLeaderelectionYml, nil
}

func operandsExternalSecretsRolebinding_externalSecretsLeaderelectionYml() (*asset, error) {
	bytes, err := operandsExternalSecretsRolebinding_externalSecretsLeaderelectionYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/rolebinding_external-secrets-leaderelection.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsSecret_externalSecretsWebhookYml = []byte(`---
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

func operandsExternalSecretsSecret_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsSecret_externalSecretsWebhookYml, nil
}

func operandsExternalSecretsSecret_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := operandsExternalSecretsSecret_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/secret_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsService_bitwardenSdkServerYml = []byte(`---
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

func operandsExternalSecretsService_bitwardenSdkServerYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsService_bitwardenSdkServerYml, nil
}

func operandsExternalSecretsService_bitwardenSdkServerYml() (*asset, error) {
	bytes, err := operandsExternalSecretsService_bitwardenSdkServerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/service_bitwarden-sdk-server.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsService_externalSecretsCertControllerMetricsYml = []byte(`---
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

func operandsExternalSecretsService_externalSecretsCertControllerMetricsYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsService_externalSecretsCertControllerMetricsYml, nil
}

func operandsExternalSecretsService_externalSecretsCertControllerMetricsYml() (*asset, error) {
	bytes, err := operandsExternalSecretsService_externalSecretsCertControllerMetricsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/service_external-secrets-cert-controller-metrics.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsService_externalSecretsMetricsYml = []byte(`---
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

func operandsExternalSecretsService_externalSecretsMetricsYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsService_externalSecretsMetricsYml, nil
}

func operandsExternalSecretsService_externalSecretsMetricsYml() (*asset, error) {
	bytes, err := operandsExternalSecretsService_externalSecretsMetricsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/service_external-secrets-metrics.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsService_externalSecretsWebhookYml = []byte(`---
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

func operandsExternalSecretsService_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsService_externalSecretsWebhookYml, nil
}

func operandsExternalSecretsService_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := operandsExternalSecretsService_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/service_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsServiceaccount_bitwardenSdkServerYml = []byte(`---
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

func operandsExternalSecretsServiceaccount_bitwardenSdkServerYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsServiceaccount_bitwardenSdkServerYml, nil
}

func operandsExternalSecretsServiceaccount_bitwardenSdkServerYml() (*asset, error) {
	bytes, err := operandsExternalSecretsServiceaccount_bitwardenSdkServerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/serviceaccount_bitwarden-sdk-server.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsServiceaccount_externalSecretsCertControllerYml = []byte(`---
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

func operandsExternalSecretsServiceaccount_externalSecretsCertControllerYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsServiceaccount_externalSecretsCertControllerYml, nil
}

func operandsExternalSecretsServiceaccount_externalSecretsCertControllerYml() (*asset, error) {
	bytes, err := operandsExternalSecretsServiceaccount_externalSecretsCertControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/serviceaccount_external-secrets-cert-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsServiceaccount_externalSecretsWebhookYml = []byte(`---
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

func operandsExternalSecretsServiceaccount_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsServiceaccount_externalSecretsWebhookYml, nil
}

func operandsExternalSecretsServiceaccount_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := operandsExternalSecretsServiceaccount_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/serviceaccount_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsServiceaccount_externalSecretsYml = []byte(`---
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

func operandsExternalSecretsServiceaccount_externalSecretsYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsServiceaccount_externalSecretsYml, nil
}

func operandsExternalSecretsServiceaccount_externalSecretsYml() (*asset, error) {
	bytes, err := operandsExternalSecretsServiceaccount_externalSecretsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/serviceaccount_external-secrets.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsValidatingwebhookconfiguration_externalsecretValidateYml = []byte(`---
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

func operandsExternalSecretsValidatingwebhookconfiguration_externalsecretValidateYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsValidatingwebhookconfiguration_externalsecretValidateYml, nil
}

func operandsExternalSecretsValidatingwebhookconfiguration_externalsecretValidateYml() (*asset, error) {
	bytes, err := operandsExternalSecretsValidatingwebhookconfiguration_externalsecretValidateYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/validatingwebhookconfiguration_externalsecret-validate.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operandsExternalSecretsValidatingwebhookconfiguration_secretstoreValidateYml = []byte(`---
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

func operandsExternalSecretsValidatingwebhookconfiguration_secretstoreValidateYmlBytes() ([]byte, error) {
	return _operandsExternalSecretsValidatingwebhookconfiguration_secretstoreValidateYml, nil
}

func operandsExternalSecretsValidatingwebhookconfiguration_secretstoreValidateYml() (*asset, error) {
	bytes, err := operandsExternalSecretsValidatingwebhookconfiguration_secretstoreValidateYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operands/external-secrets/validatingwebhookconfiguration_secretstore-validate.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operatorNetworkpoliciesAllowApiServerEgressForBitwardenSeverYml = []byte(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: eso-sys-allow-api-server-egress-for-bitwarden-server
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: bitwarden-sdk-server
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v1.3.0"
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

func operatorNetworkpoliciesAllowApiServerEgressForBitwardenSeverYmlBytes() ([]byte, error) {
	return _operatorNetworkpoliciesAllowApiServerEgressForBitwardenSeverYml, nil
}

func operatorNetworkpoliciesAllowApiServerEgressForBitwardenSeverYml() (*asset, error) {
	bytes, err := operatorNetworkpoliciesAllowApiServerEgressForBitwardenSeverYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operator/networkpolicies/allow-api-server-egress-for-bitwarden-sever.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operatorNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYml = []byte(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: eso-sys-allow-api-server-egress-for-cert-controller
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-cert-controller
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v1.3.0"
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

func operatorNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYmlBytes() ([]byte, error) {
	return _operatorNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYml, nil
}

func operatorNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYml() (*asset, error) {
	bytes, err := operatorNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operator/networkpolicies/allow-api-server-egress-for-cert-controller-traffic.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operatorNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYml = []byte(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: eso-sys-allow-api-server-egress-for-main-controller
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v1.3.0"
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

func operatorNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYmlBytes() ([]byte, error) {
	return _operatorNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYml, nil
}

func operatorNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYml() (*asset, error) {
	bytes, err := operatorNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operator/networkpolicies/allow-api-server-egress-for-main-controller-traffic.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operatorNetworkpoliciesAllowApiServerEgressForWebhookTrafficYml = []byte(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: eso-sys-allow-api-server-egress-for-webhook
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v1.3.0"
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

func operatorNetworkpoliciesAllowApiServerEgressForWebhookTrafficYmlBytes() ([]byte, error) {
	return _operatorNetworkpoliciesAllowApiServerEgressForWebhookTrafficYml, nil
}

func operatorNetworkpoliciesAllowApiServerEgressForWebhookTrafficYml() (*asset, error) {
	bytes, err := operatorNetworkpoliciesAllowApiServerEgressForWebhookTrafficYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operator/networkpolicies/allow-api-server-egress-for-webhook-traffic.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operatorNetworkpoliciesAllowDnsYml = []byte(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v1.3.0"
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

func operatorNetworkpoliciesAllowDnsYmlBytes() ([]byte, error) {
	return _operatorNetworkpoliciesAllowDnsYml, nil
}

func operatorNetworkpoliciesAllowDnsYml() (*asset, error) {
	bytes, err := operatorNetworkpoliciesAllowDnsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operator/networkpolicies/allow-dns.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _operatorNetworkpoliciesDenyAllYml = []byte(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: eso-sys-deny-all-traffic
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v1.3.0"
    app.kubernetes.io/managed-by: external-secrets-operator
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress`)

func operatorNetworkpoliciesDenyAllYmlBytes() ([]byte, error) {
	return _operatorNetworkpoliciesDenyAllYml, nil
}

func operatorNetworkpoliciesDenyAllYml() (*asset, error) {
	bytes, err := operatorNetworkpoliciesDenyAllYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "operator/networkpolicies/deny-all.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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
	"operands/external-secrets/certificate_bitwarden-tls-certs.yml":                        operandsExternalSecretsCertificate_bitwardenTlsCertsYml,
	"operands/external-secrets/certificate_external-secrets-webhook.yml":                   operandsExternalSecretsCertificate_externalSecretsWebhookYml,
	"operands/external-secrets/clusterrole_external-secrets-cert-controller.yml":           operandsExternalSecretsClusterrole_externalSecretsCertControllerYml,
	"operands/external-secrets/clusterrole_external-secrets-controller.yml":                operandsExternalSecretsClusterrole_externalSecretsControllerYml,
	"operands/external-secrets/clusterrole_external-secrets-edit.yml":                      operandsExternalSecretsClusterrole_externalSecretsEditYml,
	"operands/external-secrets/clusterrole_external-secrets-servicebindings.yml":           operandsExternalSecretsClusterrole_externalSecretsServicebindingsYml,
	"operands/external-secrets/clusterrole_external-secrets-view.yml":                      operandsExternalSecretsClusterrole_externalSecretsViewYml,
	"operands/external-secrets/clusterrolebinding_external-secrets-cert-controller.yml":    operandsExternalSecretsClusterrolebinding_externalSecretsCertControllerYml,
	"operands/external-secrets/clusterrolebinding_external-secrets-controller.yml":         operandsExternalSecretsClusterrolebinding_externalSecretsControllerYml,
	"operands/external-secrets/deployment_bitwarden-sdk-server.yml":                        operandsExternalSecretsDeployment_bitwardenSdkServerYml,
	"operands/external-secrets/deployment_external-secrets-cert-controller.yml":            operandsExternalSecretsDeployment_externalSecretsCertControllerYml,
	"operands/external-secrets/deployment_external-secrets-webhook.yml":                    operandsExternalSecretsDeployment_externalSecretsWebhookYml,
	"operands/external-secrets/deployment_external-secrets.yml":                            operandsExternalSecretsDeployment_externalSecretsYml,
	"operands/external-secrets/namespace_external-secrets.yml":                             operandsExternalSecretsNamespace_externalSecretsYml,
	"operands/external-secrets/role_external-secrets-leaderelection.yml":                   operandsExternalSecretsRole_externalSecretsLeaderelectionYml,
	"operands/external-secrets/rolebinding_external-secrets-leaderelection.yml":            operandsExternalSecretsRolebinding_externalSecretsLeaderelectionYml,
	"operands/external-secrets/secret_external-secrets-webhook.yml":                        operandsExternalSecretsSecret_externalSecretsWebhookYml,
	"operands/external-secrets/service_bitwarden-sdk-server.yml":                           operandsExternalSecretsService_bitwardenSdkServerYml,
	"operands/external-secrets/service_external-secrets-cert-controller-metrics.yml":       operandsExternalSecretsService_externalSecretsCertControllerMetricsYml,
	"operands/external-secrets/service_external-secrets-metrics.yml":                       operandsExternalSecretsService_externalSecretsMetricsYml,
	"operands/external-secrets/service_external-secrets-webhook.yml":                       operandsExternalSecretsService_externalSecretsWebhookYml,
	"operands/external-secrets/serviceaccount_bitwarden-sdk-server.yml":                    operandsExternalSecretsServiceaccount_bitwardenSdkServerYml,
	"operands/external-secrets/serviceaccount_external-secrets-cert-controller.yml":        operandsExternalSecretsServiceaccount_externalSecretsCertControllerYml,
	"operands/external-secrets/serviceaccount_external-secrets-webhook.yml":                operandsExternalSecretsServiceaccount_externalSecretsWebhookYml,
	"operands/external-secrets/serviceaccount_external-secrets.yml":                        operandsExternalSecretsServiceaccount_externalSecretsYml,
	"operands/external-secrets/validatingwebhookconfiguration_externalsecret-validate.yml": operandsExternalSecretsValidatingwebhookconfiguration_externalsecretValidateYml,
	"operands/external-secrets/validatingwebhookconfiguration_secretstore-validate.yml":    operandsExternalSecretsValidatingwebhookconfiguration_secretstoreValidateYml,
	"operator/networkpolicies/allow-api-server-egress-for-bitwarden-sever.yml":             operatorNetworkpoliciesAllowApiServerEgressForBitwardenSeverYml,
	"operator/networkpolicies/allow-api-server-egress-for-cert-controller-traffic.yml":     operatorNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYml,
	"operator/networkpolicies/allow-api-server-egress-for-main-controller-traffic.yml":     operatorNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYml,
	"operator/networkpolicies/allow-api-server-egress-for-webhook-traffic.yml":             operatorNetworkpoliciesAllowApiServerEgressForWebhookTrafficYml,
	"operator/networkpolicies/allow-dns.yml":                                               operatorNetworkpoliciesAllowDnsYml,
	"operator/networkpolicies/deny-all.yml":                                                operatorNetworkpoliciesDenyAllYml,
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
	"operands": {nil, map[string]*bintree{
		"external-secrets": {nil, map[string]*bintree{
			"certificate_bitwarden-tls-certs.yml":                        {operandsExternalSecretsCertificate_bitwardenTlsCertsYml, map[string]*bintree{}},
			"certificate_external-secrets-webhook.yml":                   {operandsExternalSecretsCertificate_externalSecretsWebhookYml, map[string]*bintree{}},
			"clusterrole_external-secrets-cert-controller.yml":           {operandsExternalSecretsClusterrole_externalSecretsCertControllerYml, map[string]*bintree{}},
			"clusterrole_external-secrets-controller.yml":                {operandsExternalSecretsClusterrole_externalSecretsControllerYml, map[string]*bintree{}},
			"clusterrole_external-secrets-edit.yml":                      {operandsExternalSecretsClusterrole_externalSecretsEditYml, map[string]*bintree{}},
			"clusterrole_external-secrets-servicebindings.yml":           {operandsExternalSecretsClusterrole_externalSecretsServicebindingsYml, map[string]*bintree{}},
			"clusterrole_external-secrets-view.yml":                      {operandsExternalSecretsClusterrole_externalSecretsViewYml, map[string]*bintree{}},
			"clusterrolebinding_external-secrets-cert-controller.yml":    {operandsExternalSecretsClusterrolebinding_externalSecretsCertControllerYml, map[string]*bintree{}},
			"clusterrolebinding_external-secrets-controller.yml":         {operandsExternalSecretsClusterrolebinding_externalSecretsControllerYml, map[string]*bintree{}},
			"deployment_bitwarden-sdk-server.yml":                        {operandsExternalSecretsDeployment_bitwardenSdkServerYml, map[string]*bintree{}},
			"deployment_external-secrets-cert-controller.yml":            {operandsExternalSecretsDeployment_externalSecretsCertControllerYml, map[string]*bintree{}},
			"deployment_external-secrets-webhook.yml":                    {operandsExternalSecretsDeployment_externalSecretsWebhookYml, map[string]*bintree{}},
			"deployment_external-secrets.yml":                            {operandsExternalSecretsDeployment_externalSecretsYml, map[string]*bintree{}},
			"namespace_external-secrets.yml":                             {operandsExternalSecretsNamespace_externalSecretsYml, map[string]*bintree{}},
			"role_external-secrets-leaderelection.yml":                   {operandsExternalSecretsRole_externalSecretsLeaderelectionYml, map[string]*bintree{}},
			"rolebinding_external-secrets-leaderelection.yml":            {operandsExternalSecretsRolebinding_externalSecretsLeaderelectionYml, map[string]*bintree{}},
			"secret_external-secrets-webhook.yml":                        {operandsExternalSecretsSecret_externalSecretsWebhookYml, map[string]*bintree{}},
			"service_bitwarden-sdk-server.yml":                           {operandsExternalSecretsService_bitwardenSdkServerYml, map[string]*bintree{}},
			"service_external-secrets-cert-controller-metrics.yml":       {operandsExternalSecretsService_externalSecretsCertControllerMetricsYml, map[string]*bintree{}},
			"service_external-secrets-metrics.yml":                       {operandsExternalSecretsService_externalSecretsMetricsYml, map[string]*bintree{}},
			"service_external-secrets-webhook.yml":                       {operandsExternalSecretsService_externalSecretsWebhookYml, map[string]*bintree{}},
			"serviceaccount_bitwarden-sdk-server.yml":                    {operandsExternalSecretsServiceaccount_bitwardenSdkServerYml, map[string]*bintree{}},
			"serviceaccount_external-secrets-cert-controller.yml":        {operandsExternalSecretsServiceaccount_externalSecretsCertControllerYml, map[string]*bintree{}},
			"serviceaccount_external-secrets-webhook.yml":                {operandsExternalSecretsServiceaccount_externalSecretsWebhookYml, map[string]*bintree{}},
			"serviceaccount_external-secrets.yml":                        {operandsExternalSecretsServiceaccount_externalSecretsYml, map[string]*bintree{}},
			"validatingwebhookconfiguration_externalsecret-validate.yml": {operandsExternalSecretsValidatingwebhookconfiguration_externalsecretValidateYml, map[string]*bintree{}},
			"validatingwebhookconfiguration_secretstore-validate.yml":    {operandsExternalSecretsValidatingwebhookconfiguration_secretstoreValidateYml, map[string]*bintree{}},
		}},
	}},
	"operator": {nil, map[string]*bintree{
		"networkpolicies": {nil, map[string]*bintree{
			"allow-api-server-egress-for-bitwarden-sever.yml":         {operatorNetworkpoliciesAllowApiServerEgressForBitwardenSeverYml, map[string]*bintree{}},
			"allow-api-server-egress-for-cert-controller-traffic.yml": {operatorNetworkpoliciesAllowApiServerEgressForCertControllerTrafficYml, map[string]*bintree{}},
			"allow-api-server-egress-for-main-controller-traffic.yml": {operatorNetworkpoliciesAllowApiServerEgressForMainControllerTrafficYml, map[string]*bintree{}},
			"allow-api-server-egress-for-webhook-traffic.yml":         {operatorNetworkpoliciesAllowApiServerEgressForWebhookTrafficYml, map[string]*bintree{}},
			"allow-dns.yml": {operatorNetworkpoliciesAllowDnsYml, map[string]*bintree{}},
			"deny-all.yml":  {operatorNetworkpoliciesDenyAllYml, map[string]*bintree{}},
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
