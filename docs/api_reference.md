# API Reference

## Packages
- [operator.openshift.io/v1alpha1](#operatoropenshiftiov1alpha1)


## operator.openshift.io/v1alpha1

Package v1alpha1 contains API Schema definitions for the operator v1alpha1 API group

### Resource Types
- [ExternalSecretsConfig](#externalsecretsconfig)
- [ExternalSecretsConfigList](#externalsecretsconfiglist)
- [ExternalSecretsManager](#externalsecretsmanager)
- [ExternalSecretsManagerList](#externalsecretsmanagerlist)



#### BitwardenSecretManagerProvider



BitwardenSecretManagerProvider is for enabling the bitwarden secrets manager provider and
for setting up the additional service required for connecting with the bitwarden server.



_Appears in:_
- [ExternalSecretsConfigSpec](#externalsecretsconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _string_ | enabled is for enabling the bitwarden secrets manager provider, which can be indicated<br />by setting `true` or `false`. | false | Enum: [true false] <br />Optional: \{\} <br /> |
| `secretRef` _SecretReference_ | SecretRef is the kubernetes secret containing the TLS key pair to be used for the bitwarden server.<br />The issuer in CertManagerConfig will be utilized to generate the required certificate if the secret<br />reference is not provided and CertManagerConfig is configured. The key names in secret for certificate<br />must be `tls.crt`, for private key must be `tls.key` and for CA certificate key name must be `ca.crt`. |  | Optional: \{\} <br /> |


#### CertManagerConfig



CertManagerConfig is for configuring cert-manager specifics.



_Appears in:_
- [ExternalSecretsConfigSpec](#externalsecretsconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _string_ | enabled is for enabling the use of cert-manager for obtaining and renewing the<br />certificates used for webhook server, instead of built-in certificates.<br />Use `true` or `false` to indicate the preference. | false | Enum: [true false] <br />Required: \{\} <br /> |
| `addInjectorAnnotations` _string_ | addInjectorAnnotations is for adding the `cert-manager.io/inject-ca-from` annotation to the<br />webhooks and CRDs to automatically setup webhook to the cert-manager CA. This requires<br />CA Injector to be enabled in cert-manager. Use `true` or `false` to indicate the preference. | false | Enum: [true false] <br />Optional: \{\} <br /> |
| `issuerRef` _ObjectReference_ | issuerRef contains details to the referenced object used for<br />obtaining the certificates. It must exist in the external-secrets<br />namespace if not using a cluster-scoped cert-manager issuer. |  | Required: \{\} <br /> |
| `certificateDuration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#duration-v1-meta)_ | certificateDuration is the validity period of the webhook certificate. | 8760h | Optional: \{\} <br /> |
| `certificateRenewBefore` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#duration-v1-meta)_ | certificateRenewBefore is the ahead time to renew the webhook certificate<br />before expiry. | 30m | Optional: \{\} <br /> |


#### CommonConfigs



CommonConfigs are the common configurations available for all the operands managed
by the operator.



_Appears in:_
- [ExternalSecretsConfigSpec](#externalsecretsconfigspec)
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `logLevel` _integer_ | logLevel supports value range as per [kubernetes logging guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#what-method-to-use). | 1 | Maximum: 5 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | labels to apply to all resources created by the operator. |  | MaxProperties: 20 <br />MinProperties: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#resourcerequirements-v1-core)_ | resources is for defining the resource requirements.<br />Cannot be updated.<br />ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ |  | Optional: \{\} <br /> |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#affinity-v1-core)_ | affinity is for setting scheduling affinity rules.<br />ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/ |  | Optional: \{\} <br /> |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#toleration-v1-core) array_ | tolerations is for setting the pod tolerations.<br />ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |  | MaxItems: 10 <br />MinItems: 0 <br />Optional: \{\} <br /> |
| `nodeSelector` _object (keys:string, values:string)_ | nodeSelector is for defining the scheduling criteria using node labels.<br />ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/ |  | MaxProperties: 10 <br />MinProperties: 0 <br />Optional: \{\} <br /> |
| `proxy` _[ProxyConfig](#proxyconfig)_ | proxy is for setting the proxy configurations which will be made available<br />in operand containers managed by the operator as environment variables. |  | Optional: \{\} <br /> |


#### Condition







_Appears in:_
- [ControllerStatus](#controllerstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | type of the condition |  | Required: \{\} <br /> |
| `status` _[ConditionStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#conditionstatus-v1-meta)_ | status of the condition |  |  |
| `message` _string_ | message provides details about the state. |  |  |


#### ConditionalStatus



ConditionalStatus holds information of the current state of the external-secrets deployment
indicated through defined conditions.



_Appears in:_
- [ExternalSecretsConfigStatus](#externalsecretsconfigstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions holds information of the current state of deployment. |  |  |


#### ControllerStatus



ControllerStatus holds the observed conditions of the controllers part of the operator.



_Appears in:_
- [ExternalSecretsManagerStatus](#externalsecretsmanagerstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name of the controller for which the observed condition is recorded. |  | Required: \{\} <br /> |
| `conditions` _[Condition](#condition) array_ | conditions holds information of the current state of the external-secrets-operator controllers. |  |  |
| `observedGeneration` _integer_ | observedGeneration represents the .metadata.generation on the observed resource. |  | Minimum: 0 <br /> |


#### ExternalSecretsConfig



ExternalSecretsConfig describes configuration and information about the managed external-secrets
deployment. The name must be `cluster` as ExternalSecretsConfig is a singleton,
allowing only one instance per cluster.

When an ExternalSecretsConfig is created, a new deployment is created which manages the
external-secrets and keeps it in the desired state.



_Appears in:_
- [ExternalSecretsConfigList](#externalsecretsconfiglist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `operator.openshift.io/v1alpha1` | | |
| `kind` _string_ | `ExternalSecretsConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ExternalSecretsConfigSpec](#externalsecretsconfigspec)_ | spec is the specification of the desired behavior of the ExternalSecretsConfig. |  |  |
| `status` _[ExternalSecretsConfigStatus](#externalsecretsconfigstatus)_ | status is the most recently observed status of the ExternalSecretsConfig. |  |  |


#### ExternalSecretsConfigList



ExternalSecretsConfigList is a list of ExternalSecretsConfig objects.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `operator.openshift.io/v1alpha1` | | |
| `kind` _string_ | `ExternalSecretsConfigList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ExternalSecretsConfig](#externalsecretsconfig) array_ |  |  |  |


#### ExternalSecretsConfigSpec



ExternalSecretsConfigSpec is for configuring the external-secrets operand behavior.



_Appears in:_
- [ExternalSecretsConfig](#externalsecretsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `namespace` _string_ | namespace is for configuring the namespace to install the external-secret operand. | external-secrets | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `operatingNamespace` _string_ | operatingNamespace is for restricting the external-secrets operations to provided namespace.<br />And when enabled `ClusterSecretStore` and `ClusterExternalSecret` are implicitly disabled. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `bitwardenSecretManagerProvider` _[BitwardenSecretManagerProvider](#bitwardensecretmanagerprovider)_ | bitwardenSecretManagerProvider is for enabling the bitwarden secrets manager provider and<br />for setting up the additional service required for connecting with the bitwarden server. |  | Optional: \{\} <br /> |
| `webhookConfig` _[WebhookConfig](#webhookconfig)_ | webhookConfig is for configuring external-secrets webhook specifics. |  | Optional: \{\} <br /> |
| `certManagerConfig` _[CertManagerConfig](#certmanagerconfig)_ | CertManagerConfig is for configuring cert-manager specifics, which will be used for generating<br />certificates for webhook and bitwarden-sdk-server components. |  | Optional: \{\} <br /> |
| `logLevel` _integer_ | logLevel supports value range as per [kubernetes logging guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#what-method-to-use). | 1 | Maximum: 5 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | labels to apply to all resources created by the operator. |  | MaxProperties: 20 <br />MinProperties: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#resourcerequirements-v1-core)_ | resources is for defining the resource requirements.<br />Cannot be updated.<br />ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ |  | Optional: \{\} <br /> |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#affinity-v1-core)_ | affinity is for setting scheduling affinity rules.<br />ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/ |  | Optional: \{\} <br /> |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#toleration-v1-core) array_ | tolerations is for setting the pod tolerations.<br />ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |  | MaxItems: 10 <br />MinItems: 0 <br />Optional: \{\} <br /> |
| `nodeSelector` _object (keys:string, values:string)_ | nodeSelector is for defining the scheduling criteria using node labels.<br />ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/ |  | MaxProperties: 10 <br />MinProperties: 0 <br />Optional: \{\} <br /> |
| `proxy` _[ProxyConfig](#proxyconfig)_ | proxy is for setting the proxy configurations which will be made available<br />in operand containers managed by the operator as environment variables. |  | Optional: \{\} <br /> |


#### ExternalSecretsConfigStatus



ExternalSecretsConfigStatus is the most recently observed status of the ExternalSecretsConfig.



_Appears in:_
- [ExternalSecretsConfig](#externalsecretsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions holds information of the current state of deployment. |  |  |
| `externalSecretsImage` _string_ | externalSecretsImage is the name of the image and the tag used for deploying external-secrets. |  |  |
| `bitwardenSDKServerImage` _string_ | BitwardenSDKServerImage is the name of the image and the tag used for deploying bitwarden-sdk-server. |  |  |


#### ExternalSecretsManager



ExternalSecretsManager describes configuration and information about the deployments managed by
the external-secrets-operator. The name must be `cluster` as this is a singleton object allowing
only one instance of ExternalSecretsManager per cluster.

It is mainly for configuring the global options and enabling optional features, which
serves as a common/centralized config for managing multiple controllers of the operator. The object
is automatically created during the operator installation.



_Appears in:_
- [ExternalSecretsManagerList](#externalsecretsmanagerlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `operator.openshift.io/v1alpha1` | | |
| `kind` _string_ | `ExternalSecretsManager` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ExternalSecretsManagerSpec](#externalsecretsmanagerspec)_ | spec is the specification of the desired behavior |  |  |
| `status` _[ExternalSecretsManagerStatus](#externalsecretsmanagerstatus)_ | status is the most recently observed status of controllers used by<br />External Secrets Operator. |  |  |


#### ExternalSecretsManagerList



ExternalSecretsManagerList is a list of ExternalSecretsManager objects.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `operator.openshift.io/v1alpha1` | | |
| `kind` _string_ | `ExternalSecretsManagerList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ExternalSecretsManager](#externalsecretsmanager) array_ |  |  |  |


#### ExternalSecretsManagerSpec



ExternalSecretsManagerSpec is the specification of the desired behavior of the ExternalSecretsManager.



_Appears in:_
- [ExternalSecretsManager](#externalsecretsmanager)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `globalConfig` _[GlobalConfig](#globalconfig)_ | globalConfig is for configuring the behavior of deployments that are managed<br />by external secrets-operator. |  | Optional: \{\} <br /> |
| `features` _[Feature](#feature) array_ | features is for enabling the optional operator features. |  | Optional: \{\} <br /> |


#### ExternalSecretsManagerStatus



ExternalSecretsManagerStatus is the most recently observed status of the ExternalSecretsManager.



_Appears in:_
- [ExternalSecretsManager](#externalsecretsmanager)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `controllerStatuses` _[ControllerStatus](#controllerstatus) array_ | controllerStatuses holds the observed conditions of the controllers part of the operator. |  |  |
| `lastTransitionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#time-v1-meta)_ | lastTransitionTime is the last time the condition transitioned from one status to another. |  | Format: date-time <br />Type: string <br /> |


#### Feature



Feature is for enabling the optional features.
Feature is for enabling the optional features.



_Appears in:_
- [ExternalSecretsManagerSpec](#externalsecretsmanagerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name of the optional feature. |  | Required: \{\} <br /> |
| `enabled` _boolean_ | enabled determines if feature should be turned on. |  | Enum: [true false] <br />Required: \{\} <br /> |


#### GlobalConfig



GlobalConfig is for configuring the external-secrets-operator behavior.



_Appears in:_
- [ExternalSecretsManagerSpec](#externalsecretsmanagerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `logLevel` _integer_ | logLevel supports value range as per [kubernetes logging guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#what-method-to-use). | 1 | Maximum: 5 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | labels to apply to all resources created by the operator. |  | MaxProperties: 20 <br />MinProperties: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#resourcerequirements-v1-core)_ | resources is for defining the resource requirements.<br />Cannot be updated.<br />ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ |  | Optional: \{\} <br /> |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#affinity-v1-core)_ | affinity is for setting scheduling affinity rules.<br />ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/ |  | Optional: \{\} <br /> |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#toleration-v1-core) array_ | tolerations is for setting the pod tolerations.<br />ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |  | MaxItems: 10 <br />MinItems: 0 <br />Optional: \{\} <br /> |
| `nodeSelector` _object (keys:string, values:string)_ | nodeSelector is for defining the scheduling criteria using node labels.<br />ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/ |  | MaxProperties: 10 <br />MinProperties: 0 <br />Optional: \{\} <br /> |
| `proxy` _[ProxyConfig](#proxyconfig)_ | proxy is for setting the proxy configurations which will be made available<br />in operand containers managed by the operator as environment variables. |  | Optional: \{\} <br /> |


#### ObjectReference



ObjectReference is a reference to an object with a given name, kind and group.



_Appears in:_
- [CertManagerConfig](#certmanagerconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the resource being referred to. |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `kind` _string_ | Kind of the resource being referred to. |  | MaxLength: 253 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `group` _string_ | Group of the resource being referred to. |  | MaxLength: 253 <br />MinLength: 1 <br />Optional: \{\} <br /> |


#### ProxyConfig



ProxyConfig is for setting the proxy configurations which will be made available
in operand containers managed by the operator as environment variables.



_Appears in:_
- [CommonConfigs](#commonconfigs)
- [ExternalSecretsConfigSpec](#externalsecretsconfigspec)
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `httpProxy` _string_ | httpProxy is the URL of the proxy for HTTP requests. |  | MaxLength: 4096 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `httpsProxy` _string_ | httpsProxy is the URL of the proxy for HTTPS requests. |  | MaxLength: 4096 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `noProxy` _string_ | noProxy is a comma-separated list of hostnames and/or CIDRs and/or IPs for which the proxy should not be used. |  | MaxLength: 4096 <br />MinLength: 1 <br />Optional: \{\} <br /> |


#### SecretReference



SecretReference is a reference to the secret with the given name, which should exist
in the same namespace where it will be utilized.



_Appears in:_
- [BitwardenSecretManagerProvider](#bitwardensecretmanagerprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the secret resource being referred to. |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |


#### WebhookConfig



WebhookConfig is for configuring external-secrets webhook specifics.



_Appears in:_
- [ExternalSecretsConfigSpec](#externalsecretsconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `certificateCheckInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#duration-v1-meta)_ | CertificateCheckInterval is for configuring the polling interval to check the certificate<br />validity. | 5m | Optional: \{\} <br /> |


