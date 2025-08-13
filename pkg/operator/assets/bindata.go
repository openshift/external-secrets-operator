// Code generated for package assets by go-bindata DO NOT EDIT. (@generated)
// sources:
// bindata/external-secrets/certificate_bitwarden-tls-certs.yml
// bindata/external-secrets/external-secrets-namespace.yaml
// bindata/external-secrets/resources/certificate_external-secrets-webhook.yml
// bindata/external-secrets/resources/clusterrole_external-secrets-cert-controller.yml
// bindata/external-secrets/resources/clusterrole_external-secrets-controller.yml
// bindata/external-secrets/resources/clusterrole_external-secrets-edit.yml
// bindata/external-secrets/resources/clusterrole_external-secrets-servicebindings.yml
// bindata/external-secrets/resources/clusterrole_external-secrets-view.yml
// bindata/external-secrets/resources/clusterrolebinding_external-secrets-cert-controller.yml
// bindata/external-secrets/resources/clusterrolebinding_external-secrets-controller.yml
// bindata/external-secrets/resources/deployment_bitwarden-sdk-server.yml
// bindata/external-secrets/resources/deployment_external-secrets-cert-controller.yml
// bindata/external-secrets/resources/deployment_external-secrets-webhook.yml
// bindata/external-secrets/resources/deployment_external-secrets.yml
// bindata/external-secrets/resources/role_external-secrets-leaderelection.yml
// bindata/external-secrets/resources/rolebinding_external-secrets-leaderelection.yml
// bindata/external-secrets/resources/secret_external-secrets-webhook.yml
// bindata/external-secrets/resources/service_bitwarden-sdk-server.yml
// bindata/external-secrets/resources/service_external-secrets-webhook.yml
// bindata/external-secrets/resources/serviceaccount_bitwarden-sdk-server.yml
// bindata/external-secrets/resources/serviceaccount_external-secrets-cert-controller.yml
// bindata/external-secrets/resources/serviceaccount_external-secrets-webhook.yml
// bindata/external-secrets/resources/serviceaccount_external-secrets.yml
// bindata/external-secrets/resources/validatingwebhookconfiguration_externalsecret-validate.yml
// bindata/external-secrets/resources/validatingwebhookconfiguration_secretstore-validate.yml
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

var _externalSecretsCertificate_bitwardenTlsCertsYml = []byte(`apiVersion: cert-manager.io/v1
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

func externalSecretsCertificate_bitwardenTlsCertsYmlBytes() ([]byte, error) {
	return _externalSecretsCertificate_bitwardenTlsCertsYml, nil
}

func externalSecretsCertificate_bitwardenTlsCertsYml() (*asset, error) {
	bytes, err := externalSecretsCertificate_bitwardenTlsCertsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/certificate_bitwarden-tls-certs.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsExternalSecretsNamespaceYaml = []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: external-secrets
`)

func externalSecretsExternalSecretsNamespaceYamlBytes() ([]byte, error) {
	return _externalSecretsExternalSecretsNamespaceYaml, nil
}

func externalSecretsExternalSecretsNamespaceYaml() (*asset, error) {
	bytes, err := externalSecretsExternalSecretsNamespaceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/external-secrets-namespace.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesCertificate_externalSecretsWebhookYml = []byte(`---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: external-secrets-webhook
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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
  duration: "8760h"
  secretName: external-secrets-webhook
`)

func externalSecretsResourcesCertificate_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesCertificate_externalSecretsWebhookYml, nil
}

func externalSecretsResourcesCertificate_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := externalSecretsResourcesCertificate_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/certificate_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesClusterrole_externalSecretsCertControllerYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-secrets-cert-controller
  labels:
    app.kubernetes.io/name: external-secrets-cert-controller
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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

func externalSecretsResourcesClusterrole_externalSecretsCertControllerYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesClusterrole_externalSecretsCertControllerYml, nil
}

func externalSecretsResourcesClusterrole_externalSecretsCertControllerYml() (*asset, error) {
	bytes, err := externalSecretsResourcesClusterrole_externalSecretsCertControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/clusterrole_external-secrets-cert-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesClusterrole_externalSecretsControllerYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-secrets-controller
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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

func externalSecretsResourcesClusterrole_externalSecretsControllerYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesClusterrole_externalSecretsControllerYml, nil
}

func externalSecretsResourcesClusterrole_externalSecretsControllerYml() (*asset, error) {
	bytes, err := externalSecretsResourcesClusterrole_externalSecretsControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/clusterrole_external-secrets-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesClusterrole_externalSecretsEditYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-secrets-edit
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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

func externalSecretsResourcesClusterrole_externalSecretsEditYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesClusterrole_externalSecretsEditYml, nil
}

func externalSecretsResourcesClusterrole_externalSecretsEditYml() (*asset, error) {
	bytes, err := externalSecretsResourcesClusterrole_externalSecretsEditYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/clusterrole_external-secrets-edit.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesClusterrole_externalSecretsServicebindingsYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-secrets-servicebindings
  labels:
    servicebinding.io/controller: "true"
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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

func externalSecretsResourcesClusterrole_externalSecretsServicebindingsYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesClusterrole_externalSecretsServicebindingsYml, nil
}

func externalSecretsResourcesClusterrole_externalSecretsServicebindingsYml() (*asset, error) {
	bytes, err := externalSecretsResourcesClusterrole_externalSecretsServicebindingsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/clusterrole_external-secrets-servicebindings.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesClusterrole_externalSecretsViewYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-secrets-view
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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

func externalSecretsResourcesClusterrole_externalSecretsViewYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesClusterrole_externalSecretsViewYml, nil
}

func externalSecretsResourcesClusterrole_externalSecretsViewYml() (*asset, error) {
	bytes, err := externalSecretsResourcesClusterrole_externalSecretsViewYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/clusterrole_external-secrets-view.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesClusterrolebinding_externalSecretsCertControllerYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: external-secrets-cert-controller
  labels:
    app.kubernetes.io/name: external-secrets-cert-controller
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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

func externalSecretsResourcesClusterrolebinding_externalSecretsCertControllerYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesClusterrolebinding_externalSecretsCertControllerYml, nil
}

func externalSecretsResourcesClusterrolebinding_externalSecretsCertControllerYml() (*asset, error) {
	bytes, err := externalSecretsResourcesClusterrolebinding_externalSecretsCertControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/clusterrolebinding_external-secrets-cert-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesClusterrolebinding_externalSecretsControllerYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: external-secrets-controller
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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

func externalSecretsResourcesClusterrolebinding_externalSecretsControllerYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesClusterrolebinding_externalSecretsControllerYml, nil
}

func externalSecretsResourcesClusterrolebinding_externalSecretsControllerYml() (*asset, error) {
	bytes, err := externalSecretsResourcesClusterrolebinding_externalSecretsControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/clusterrolebinding_external-secrets-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesDeployment_bitwardenSdkServerYml = []byte(`---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bitwarden-sdk-server
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: bitwarden-sdk-server
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.5.0"
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
          image: "ghcr.io/external-secrets/bitwarden-sdk-server:v0.5.0"
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
            items:
              - key: tls.crt
                path: cert.pem
              - key: tls.key
                path: key.pem
              - key: ca.crt
                path: ca.pem
            secretName: bitwarden-tls-certs
`)

func externalSecretsResourcesDeployment_bitwardenSdkServerYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesDeployment_bitwardenSdkServerYml, nil
}

func externalSecretsResourcesDeployment_bitwardenSdkServerYml() (*asset, error) {
	bytes, err := externalSecretsResourcesDeployment_bitwardenSdkServerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/deployment_bitwarden-sdk-server.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesDeployment_externalSecretsCertControllerYml = []byte(`---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: external-secrets-cert-controller
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-cert-controller
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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
        app.kubernetes.io/version: "v0.19.0"
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
          image: oci.external-secrets.io/external-secrets/external-secrets:v0.19.0
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
          readinessProbe:
            httpGet:
              port: 8081
              path: /readyz
            initialDelaySeconds: 20
            periodSeconds: 5
`)

func externalSecretsResourcesDeployment_externalSecretsCertControllerYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesDeployment_externalSecretsCertControllerYml, nil
}

func externalSecretsResourcesDeployment_externalSecretsCertControllerYml() (*asset, error) {
	bytes, err := externalSecretsResourcesDeployment_externalSecretsCertControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/deployment_external-secrets-cert-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesDeployment_externalSecretsWebhookYml = []byte(`---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: external-secrets-webhook
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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
        app.kubernetes.io/version: "v0.19.0"
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
          image: oci.external-secrets.io/external-secrets/external-secrets:v0.19.0
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
          readinessProbe:
            httpGet:
              port: 8081
              path: /readyz
            initialDelaySeconds: 20
            periodSeconds: 5
          volumeMounts:
            - name: certs
              mountPath: /tmp/certs
              readOnly: true
      volumes:
        - name: certs
          secret:
            secretName: external-secrets-webhook
`)

func externalSecretsResourcesDeployment_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesDeployment_externalSecretsWebhookYml, nil
}

func externalSecretsResourcesDeployment_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := externalSecretsResourcesDeployment_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/deployment_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesDeployment_externalSecretsYml = []byte(`---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: external-secrets
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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
        app.kubernetes.io/version: "v0.19.0"
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
          image: oci.external-secrets.io/external-secrets/external-secrets:v0.19.0
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

func externalSecretsResourcesDeployment_externalSecretsYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesDeployment_externalSecretsYml, nil
}

func externalSecretsResourcesDeployment_externalSecretsYml() (*asset, error) {
	bytes, err := externalSecretsResourcesDeployment_externalSecretsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/deployment_external-secrets.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesRole_externalSecretsLeaderelectionYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: external-secrets-leaderelection
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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

func externalSecretsResourcesRole_externalSecretsLeaderelectionYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesRole_externalSecretsLeaderelectionYml, nil
}

func externalSecretsResourcesRole_externalSecretsLeaderelectionYml() (*asset, error) {
	bytes, err := externalSecretsResourcesRole_externalSecretsLeaderelectionYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/role_external-secrets-leaderelection.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesRolebinding_externalSecretsLeaderelectionYml = []byte(`---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: external-secrets-leaderelection
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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

func externalSecretsResourcesRolebinding_externalSecretsLeaderelectionYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesRolebinding_externalSecretsLeaderelectionYml, nil
}

func externalSecretsResourcesRolebinding_externalSecretsLeaderelectionYml() (*asset, error) {
	bytes, err := externalSecretsResourcesRolebinding_externalSecretsLeaderelectionYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/rolebinding_external-secrets-leaderelection.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesSecret_externalSecretsWebhookYml = []byte(`---
apiVersion: v1
kind: Secret
metadata:
  name: external-secrets-webhook
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
    app.kubernetes.io/managed-by: external-secrets-operator
    external-secrets.io/component: webhook
`)

func externalSecretsResourcesSecret_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesSecret_externalSecretsWebhookYml, nil
}

func externalSecretsResourcesSecret_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := externalSecretsResourcesSecret_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/secret_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesService_bitwardenSdkServerYml = []byte(`---
apiVersion: v1
kind: Service
metadata:
  name: bitwarden-sdk-server
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: bitwarden-sdk-server
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.5.0"
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

func externalSecretsResourcesService_bitwardenSdkServerYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesService_bitwardenSdkServerYml, nil
}

func externalSecretsResourcesService_bitwardenSdkServerYml() (*asset, error) {
	bytes, err := externalSecretsResourcesService_bitwardenSdkServerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/service_bitwarden-sdk-server.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesService_externalSecretsWebhookYml = []byte(`---
apiVersion: v1
kind: Service
metadata:
  name: external-secrets-webhook
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
    app.kubernetes.io/managed-by: external-secrets-operator
    external-secrets.io/component: webhook
spec:
  type: ClusterIP
  ports:
    - port: 443
      targetPort: 10250
      protocol: TCP
      name: webhook
  selector:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
`)

func externalSecretsResourcesService_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesService_externalSecretsWebhookYml, nil
}

func externalSecretsResourcesService_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := externalSecretsResourcesService_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/service_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesServiceaccount_bitwardenSdkServerYml = []byte(`---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: bitwarden-sdk-server
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: bitwarden-sdk-server
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.5.0"
    app.kubernetes.io/managed-by: external-secrets-operator
`)

func externalSecretsResourcesServiceaccount_bitwardenSdkServerYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesServiceaccount_bitwardenSdkServerYml, nil
}

func externalSecretsResourcesServiceaccount_bitwardenSdkServerYml() (*asset, error) {
	bytes, err := externalSecretsResourcesServiceaccount_bitwardenSdkServerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/serviceaccount_bitwarden-sdk-server.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesServiceaccount_externalSecretsCertControllerYml = []byte(`---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: external-secrets-cert-controller
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-cert-controller
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
    app.kubernetes.io/managed-by: external-secrets-operator
`)

func externalSecretsResourcesServiceaccount_externalSecretsCertControllerYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesServiceaccount_externalSecretsCertControllerYml, nil
}

func externalSecretsResourcesServiceaccount_externalSecretsCertControllerYml() (*asset, error) {
	bytes, err := externalSecretsResourcesServiceaccount_externalSecretsCertControllerYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/serviceaccount_external-secrets-cert-controller.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesServiceaccount_externalSecretsWebhookYml = []byte(`---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: external-secrets-webhook
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
    app.kubernetes.io/managed-by: external-secrets-operator
`)

func externalSecretsResourcesServiceaccount_externalSecretsWebhookYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesServiceaccount_externalSecretsWebhookYml, nil
}

func externalSecretsResourcesServiceaccount_externalSecretsWebhookYml() (*asset, error) {
	bytes, err := externalSecretsResourcesServiceaccount_externalSecretsWebhookYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/serviceaccount_external-secrets-webhook.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesServiceaccount_externalSecretsYml = []byte(`---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: external-secrets
  namespace: external-secrets
  labels:
    app.kubernetes.io/name: external-secrets
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
    app.kubernetes.io/managed-by: external-secrets-operator
`)

func externalSecretsResourcesServiceaccount_externalSecretsYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesServiceaccount_externalSecretsYml, nil
}

func externalSecretsResourcesServiceaccount_externalSecretsYml() (*asset, error) {
	bytes, err := externalSecretsResourcesServiceaccount_externalSecretsYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/serviceaccount_external-secrets.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesValidatingwebhookconfiguration_externalsecretValidateYml = []byte(`---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: externalsecret-validate
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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

func externalSecretsResourcesValidatingwebhookconfiguration_externalsecretValidateYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesValidatingwebhookconfiguration_externalsecretValidateYml, nil
}

func externalSecretsResourcesValidatingwebhookconfiguration_externalsecretValidateYml() (*asset, error) {
	bytes, err := externalSecretsResourcesValidatingwebhookconfiguration_externalsecretValidateYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/validatingwebhookconfiguration_externalsecret-validate.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _externalSecretsResourcesValidatingwebhookconfiguration_secretstoreValidateYml = []byte(`---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: secretstore-validate
  labels:
    app.kubernetes.io/name: external-secrets-webhook
    app.kubernetes.io/instance: external-secrets
    app.kubernetes.io/version: "v0.19.0"
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
`)

func externalSecretsResourcesValidatingwebhookconfiguration_secretstoreValidateYmlBytes() ([]byte, error) {
	return _externalSecretsResourcesValidatingwebhookconfiguration_secretstoreValidateYml, nil
}

func externalSecretsResourcesValidatingwebhookconfiguration_secretstoreValidateYml() (*asset, error) {
	bytes, err := externalSecretsResourcesValidatingwebhookconfiguration_secretstoreValidateYmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "external-secrets/resources/validatingwebhookconfiguration_secretstore-validate.yml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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
	"external-secrets/certificate_bitwarden-tls-certs.yml":                                  externalSecretsCertificate_bitwardenTlsCertsYml,
	"external-secrets/external-secrets-namespace.yaml":                                      externalSecretsExternalSecretsNamespaceYaml,
	"external-secrets/resources/certificate_external-secrets-webhook.yml":                   externalSecretsResourcesCertificate_externalSecretsWebhookYml,
	"external-secrets/resources/clusterrole_external-secrets-cert-controller.yml":           externalSecretsResourcesClusterrole_externalSecretsCertControllerYml,
	"external-secrets/resources/clusterrole_external-secrets-controller.yml":                externalSecretsResourcesClusterrole_externalSecretsControllerYml,
	"external-secrets/resources/clusterrole_external-secrets-edit.yml":                      externalSecretsResourcesClusterrole_externalSecretsEditYml,
	"external-secrets/resources/clusterrole_external-secrets-servicebindings.yml":           externalSecretsResourcesClusterrole_externalSecretsServicebindingsYml,
	"external-secrets/resources/clusterrole_external-secrets-view.yml":                      externalSecretsResourcesClusterrole_externalSecretsViewYml,
	"external-secrets/resources/clusterrolebinding_external-secrets-cert-controller.yml":    externalSecretsResourcesClusterrolebinding_externalSecretsCertControllerYml,
	"external-secrets/resources/clusterrolebinding_external-secrets-controller.yml":         externalSecretsResourcesClusterrolebinding_externalSecretsControllerYml,
	"external-secrets/resources/deployment_bitwarden-sdk-server.yml":                        externalSecretsResourcesDeployment_bitwardenSdkServerYml,
	"external-secrets/resources/deployment_external-secrets-cert-controller.yml":            externalSecretsResourcesDeployment_externalSecretsCertControllerYml,
	"external-secrets/resources/deployment_external-secrets-webhook.yml":                    externalSecretsResourcesDeployment_externalSecretsWebhookYml,
	"external-secrets/resources/deployment_external-secrets.yml":                            externalSecretsResourcesDeployment_externalSecretsYml,
	"external-secrets/resources/role_external-secrets-leaderelection.yml":                   externalSecretsResourcesRole_externalSecretsLeaderelectionYml,
	"external-secrets/resources/rolebinding_external-secrets-leaderelection.yml":            externalSecretsResourcesRolebinding_externalSecretsLeaderelectionYml,
	"external-secrets/resources/secret_external-secrets-webhook.yml":                        externalSecretsResourcesSecret_externalSecretsWebhookYml,
	"external-secrets/resources/service_bitwarden-sdk-server.yml":                           externalSecretsResourcesService_bitwardenSdkServerYml,
	"external-secrets/resources/service_external-secrets-webhook.yml":                       externalSecretsResourcesService_externalSecretsWebhookYml,
	"external-secrets/resources/serviceaccount_bitwarden-sdk-server.yml":                    externalSecretsResourcesServiceaccount_bitwardenSdkServerYml,
	"external-secrets/resources/serviceaccount_external-secrets-cert-controller.yml":        externalSecretsResourcesServiceaccount_externalSecretsCertControllerYml,
	"external-secrets/resources/serviceaccount_external-secrets-webhook.yml":                externalSecretsResourcesServiceaccount_externalSecretsWebhookYml,
	"external-secrets/resources/serviceaccount_external-secrets.yml":                        externalSecretsResourcesServiceaccount_externalSecretsYml,
	"external-secrets/resources/validatingwebhookconfiguration_externalsecret-validate.yml": externalSecretsResourcesValidatingwebhookconfiguration_externalsecretValidateYml,
	"external-secrets/resources/validatingwebhookconfiguration_secretstore-validate.yml":    externalSecretsResourcesValidatingwebhookconfiguration_secretstoreValidateYml,
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
		"certificate_bitwarden-tls-certs.yml": {externalSecretsCertificate_bitwardenTlsCertsYml, map[string]*bintree{}},
		"external-secrets-namespace.yaml":     {externalSecretsExternalSecretsNamespaceYaml, map[string]*bintree{}},
		"resources": {nil, map[string]*bintree{
			"certificate_external-secrets-webhook.yml":                   {externalSecretsResourcesCertificate_externalSecretsWebhookYml, map[string]*bintree{}},
			"clusterrole_external-secrets-cert-controller.yml":           {externalSecretsResourcesClusterrole_externalSecretsCertControllerYml, map[string]*bintree{}},
			"clusterrole_external-secrets-controller.yml":                {externalSecretsResourcesClusterrole_externalSecretsControllerYml, map[string]*bintree{}},
			"clusterrole_external-secrets-edit.yml":                      {externalSecretsResourcesClusterrole_externalSecretsEditYml, map[string]*bintree{}},
			"clusterrole_external-secrets-servicebindings.yml":           {externalSecretsResourcesClusterrole_externalSecretsServicebindingsYml, map[string]*bintree{}},
			"clusterrole_external-secrets-view.yml":                      {externalSecretsResourcesClusterrole_externalSecretsViewYml, map[string]*bintree{}},
			"clusterrolebinding_external-secrets-cert-controller.yml":    {externalSecretsResourcesClusterrolebinding_externalSecretsCertControllerYml, map[string]*bintree{}},
			"clusterrolebinding_external-secrets-controller.yml":         {externalSecretsResourcesClusterrolebinding_externalSecretsControllerYml, map[string]*bintree{}},
			"deployment_bitwarden-sdk-server.yml":                        {externalSecretsResourcesDeployment_bitwardenSdkServerYml, map[string]*bintree{}},
			"deployment_external-secrets-cert-controller.yml":            {externalSecretsResourcesDeployment_externalSecretsCertControllerYml, map[string]*bintree{}},
			"deployment_external-secrets-webhook.yml":                    {externalSecretsResourcesDeployment_externalSecretsWebhookYml, map[string]*bintree{}},
			"deployment_external-secrets.yml":                            {externalSecretsResourcesDeployment_externalSecretsYml, map[string]*bintree{}},
			"role_external-secrets-leaderelection.yml":                   {externalSecretsResourcesRole_externalSecretsLeaderelectionYml, map[string]*bintree{}},
			"rolebinding_external-secrets-leaderelection.yml":            {externalSecretsResourcesRolebinding_externalSecretsLeaderelectionYml, map[string]*bintree{}},
			"secret_external-secrets-webhook.yml":                        {externalSecretsResourcesSecret_externalSecretsWebhookYml, map[string]*bintree{}},
			"service_bitwarden-sdk-server.yml":                           {externalSecretsResourcesService_bitwardenSdkServerYml, map[string]*bintree{}},
			"service_external-secrets-webhook.yml":                       {externalSecretsResourcesService_externalSecretsWebhookYml, map[string]*bintree{}},
			"serviceaccount_bitwarden-sdk-server.yml":                    {externalSecretsResourcesServiceaccount_bitwardenSdkServerYml, map[string]*bintree{}},
			"serviceaccount_external-secrets-cert-controller.yml":        {externalSecretsResourcesServiceaccount_externalSecretsCertControllerYml, map[string]*bintree{}},
			"serviceaccount_external-secrets-webhook.yml":                {externalSecretsResourcesServiceaccount_externalSecretsWebhookYml, map[string]*bintree{}},
			"serviceaccount_external-secrets.yml":                        {externalSecretsResourcesServiceaccount_externalSecretsYml, map[string]*bintree{}},
			"validatingwebhookconfiguration_externalsecret-validate.yml": {externalSecretsResourcesValidatingwebhookconfiguration_externalsecretValidateYml, map[string]*bintree{}},
			"validatingwebhookconfiguration_secretstore-validate.yml":    {externalSecretsResourcesValidatingwebhookconfiguration_secretstoreValidateYml, map[string]*bintree{}},
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
