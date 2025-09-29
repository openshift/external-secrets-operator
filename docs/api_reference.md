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



#### ApplicationConfig



ApplicationConfig is for specifying the configurations for the external-secrets operand.



_Appears in:_
- [ExternalSecretsConfigSpec](#externalsecretsconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `operatingNamespace` _string_ | operatingNamespace is for restricting the external-secrets operations to the provided namespace.<br />When configured `ClusterSecretStore` and `ClusterExternalSecret` are implicitly disabled. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `webhookConfig` _[WebhookConfig](#webhookconfig)_ | webhookConfig is for configuring external-secrets webhook specifics. |  | Optional: \{\} <br /> |
| `logLevel` _integer_ | logLevel supports value range as per [Kubernetes logging guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#what-method-to-use). | 1 | Maximum: 5 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#resourcerequirements-v1-core)_ | resources is for defining the resource requirements.<br />Cannot be updated.<br />ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ |  | Optional: \{\} <br /> |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#affinity-v1-core)_ | affinity is for setting scheduling affinity rules.<br />ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/ |  | Optional: \{\} <br /> |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#toleration-v1-core) array_ | tolerations is for setting the pod tolerations.<br />ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/<br />This field can have a maximum of 50 entries. |  | MaxItems: 50 <br />MinItems: 0 <br />Optional: \{\} <br /> |
| `nodeSelector` _object (keys:string, values:string)_ | nodeSelector is for defining the scheduling criteria using node labels.<br />ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/<br />This field can have a maximum of 50 entries. |  | MaxProperties: 50 <br />MinProperties: 0 <br />Optional: \{\} <br /> |
| `proxy` _[ProxyConfig](#proxyconfig)_ | proxy is for setting the proxy configurations which will be made available in operand containers managed by the operator as environment variables. |  | Optional: \{\} <br /> |


#### BitwardenSecretManagerProvider



BitwardenSecretManagerProvider is for enabling the bitwarden secrets manager provider and for setting up the additional service required for connecting with the bitwarden server.



_Appears in:_
- [PluginsConfig](#pluginsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `mode` _[Mode](#mode)_ | mode indicates bitwarden secrets manager provider state, which can be indicated by setting Enabled or Disabled.<br />Enabled: Enables the Bitwarden provider plugin. The operator will ensure the plugin is deployed and its state is synchronized.<br />Disabled: Disables reconciliation of the Bitwarden provider plugin. The plugin and its resources will remain in their current state and will not be managed by the operator. | Disabled | Enum: [Enabled Disabled] <br />Optional: \{\} <br /> |
| `secretRef` _SecretReference_ | SecretRef is the Kubernetes secret containing the TLS key pair to be used for the bitwarden server.<br />The issuer in CertManagerConfig will be utilized to generate the required certificate if the secret reference is not provided and CertManagerConfig is configured.<br />The key names in secret for certificate must be `tls.crt`, for private key must be `tls.key` and for CA certificate key name must be `ca.crt`. |  | Optional: \{\} <br /> |


#### CertManagerConfig



CertManagerConfig is for configuring cert-manager specifics.



_Appears in:_
- [CertProvidersConfig](#certprovidersconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `mode` _[Mode](#mode)_ | mode indicates whether to use cert-manager for certificate management, instead of built-in cert-controller.<br />Enabled: Makes use of cert-manager for obtaining the certificates for webhook server and other components.<br />Disabled: Makes use of in-built cert-controller for obtaining the certificates for webhook server, which is the default behavior.<br />This field is immutable once set. | Disabled | Enum: [Enabled Disabled] <br />Required: \{\} <br /> |
| `injectAnnotations` _string_ | injectAnnotations is for adding the `cert-manager.io/inject-ca-from` annotation to the webhooks and CRDs to automatically setup webhook to use the cert-manager CA. This requires CA Injector to be enabled in cert-manager.<br />Use `true` or `false` to indicate the preference. This field is immutable once set. | false | Enum: [true false] <br />Optional: \{\} <br /> |
| `issuerRef` _ObjectReference_ | issuerRef contains details of the referenced object used for obtaining certificates.<br />When `issuerRef.Kind` is `Issuer`, it must exist in the `external-secrets` namespace.<br />This field is immutable once set. |  | Optional: \{\} <br /> |
| `certificateDuration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#duration-v1-meta)_ | certificateDuration is the validity period of the webhook certificate. | 8760h | Optional: \{\} <br /> |
| `certificateRenewBefore` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#duration-v1-meta)_ | certificateRenewBefore is the ahead time to renew the webhook certificate before expiry. | 30m | Optional: \{\} <br /> |


#### CertProvidersConfig



CertProvidersConfig defines the configuration for certificate providers used to manage TLS certificates for webhook and plugins.



_Appears in:_
- [ControllerConfig](#controllerconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `certManager` _[CertManagerConfig](#certmanagerconfig)_ | certManager is for configuring cert-manager provider specifics. |  | Optional: \{\} <br /> |


#### CommonConfigs



CommonConfigs are the common configurations available for all the operands managed by the operator.



_Appears in:_
- [ApplicationConfig](#applicationconfig)
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `logLevel` _integer_ | logLevel supports value range as per [Kubernetes logging guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#what-method-to-use). | 1 | Maximum: 5 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#resourcerequirements-v1-core)_ | resources is for defining the resource requirements.<br />Cannot be updated.<br />ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ |  | Optional: \{\} <br /> |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#affinity-v1-core)_ | affinity is for setting scheduling affinity rules.<br />ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/ |  | Optional: \{\} <br /> |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#toleration-v1-core) array_ | tolerations is for setting the pod tolerations.<br />ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/<br />This field can have a maximum of 50 entries. |  | MaxItems: 50 <br />MinItems: 0 <br />Optional: \{\} <br /> |
| `nodeSelector` _object (keys:string, values:string)_ | nodeSelector is for defining the scheduling criteria using node labels.<br />ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/<br />This field can have a maximum of 50 entries. |  | MaxProperties: 50 <br />MinProperties: 0 <br />Optional: \{\} <br /> |
| `proxy` _[ProxyConfig](#proxyconfig)_ | proxy is for setting the proxy configurations which will be made available in operand containers managed by the operator as environment variables. |  | Optional: \{\} <br /> |


#### Condition







_Appears in:_
- [ControllerStatus](#controllerstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | type of the condition |  | Required: \{\} <br /> |
| `status` _[ConditionStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#conditionstatus-v1-meta)_ | status of the condition |  |  |
| `message` _string_ | message provides details about the state. |  |  |


#### ConditionalStatus



ConditionalStatus holds information of the current state of the external-secrets deployment indicated through defined conditions.



_Appears in:_
- [ExternalSecretsConfigStatus](#externalsecretsconfigstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions holds information of the current state of deployment. |  |  |


#### ControllerConfig



ControllerConfig is for specifying the configurations for the controller to use while installing the `external-secrets` operand and the plugins.



_Appears in:_
- [ExternalSecretsConfigSpec](#externalsecretsconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `certProvider` _[CertProvidersConfig](#certprovidersconfig)_ | certProvider is for defining the configuration for certificate providers used to manage TLS certificates for webhook and plugins. |  | Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | labels to apply to all resources created for the external-secrets operand deployment.<br />This field can have a maximum of 20 entries. |  | MaxProperties: 20 <br />MinProperties: 0 <br />Optional: \{\} <br /> |
| `periodicReconcileInterval` _integer_ | periodicReconcileInterval specifies the time interval in seconds for periodic reconciliation by the operator.<br />This controls how often the operator checks resources created for external-secrets operand to ensure they remain in desired state.<br />Interval can have value between 120-18000 seconds (2 minutes to 5 hours). Defaults to 300 seconds (5 minutes) if not specified. | 300 | Maximum: 18000 <br />Minimum: 120 <br />Optional: \{\} <br /> |


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



ExternalSecretsConfig describes configuration and information about the managed external-secrets deployment.
The name must be `cluster` as ExternalSecretsConfig is a singleton, allowing only one instance per cluster.

When an ExternalSecretsConfig is created, the controller installs the external-secrets and keeps it in the desired state.



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
| `appConfig` _[ApplicationConfig](#applicationconfig)_ | appConfig is for specifying the configurations for the `external-secrets` operand. |  | Optional: \{\} <br /> |
| `plugins` _[PluginsConfig](#pluginsconfig)_ | plugins is for configuring the optional provider plugins. |  | Optional: \{\} <br /> |
| `controllerConfig` _[ControllerConfig](#controllerconfig)_ | controllerConfig is for specifying the configurations for the controller to use while installing the `external-secrets` operand and the plugins. |  | Optional: \{\} <br /> |


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



ExternalSecretsManager describes configuration and information about the deployments managed by the external-secrets-operator.
The name must be `cluster` as this is a singleton object allowing only one instance of ExternalSecretsManager per cluster.

It is mainly for configuring the global options and enabling optional features, which serves as a common/centralized config for managing multiple controllers of the operator.
The object is automatically created during the operator installation.



_Appears in:_
- [ExternalSecretsManagerList](#externalsecretsmanagerlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `operator.openshift.io/v1alpha1` | | |
| `kind` _string_ | `ExternalSecretsManager` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ExternalSecretsManagerSpec](#externalsecretsmanagerspec)_ | spec is the specification of the desired behavior |  |  |
| `status` _[ExternalSecretsManagerStatus](#externalsecretsmanagerstatus)_ | status is the most recently observed status of controllers used by External Secrets Operator. |  |  |


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
| `globalConfig` _[GlobalConfig](#globalconfig)_ | globalConfig is for configuring the behavior of deployments that are managed by external secrets-operator. |  | Optional: \{\} <br /> |


#### ExternalSecretsManagerStatus



ExternalSecretsManagerStatus is the most recently observed status of the ExternalSecretsManager.



_Appears in:_
- [ExternalSecretsManager](#externalsecretsmanager)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `controllerStatuses` _[ControllerStatus](#controllerstatus) array_ | controllerStatuses holds the observed conditions of the controllers part of the operator. |  |  |
| `lastTransitionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#time-v1-meta)_ | lastTransitionTime is the last time the condition transitioned from one status to another. |  | Format: date-time <br />Type: string <br /> |


#### GlobalConfig



GlobalConfig is for configuring the external-secrets-operator behavior.



_Appears in:_
- [ExternalSecretsManagerSpec](#externalsecretsmanagerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `labels` _object (keys:string, values:string)_ | labels to apply to all resources created by the operator.<br />This field can have a maximum of 20 entries. |  | MaxProperties: 20 <br />MinProperties: 0 <br />Optional: \{\} <br /> |
| `logLevel` _integer_ | logLevel supports value range as per [Kubernetes logging guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#what-method-to-use). | 1 | Maximum: 5 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#resourcerequirements-v1-core)_ | resources is for defining the resource requirements.<br />Cannot be updated.<br />ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ |  | Optional: \{\} <br /> |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#affinity-v1-core)_ | affinity is for setting scheduling affinity rules.<br />ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/ |  | Optional: \{\} <br /> |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#toleration-v1-core) array_ | tolerations is for setting the pod tolerations.<br />ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/<br />This field can have a maximum of 50 entries. |  | MaxItems: 50 <br />MinItems: 0 <br />Optional: \{\} <br /> |
| `nodeSelector` _object (keys:string, values:string)_ | nodeSelector is for defining the scheduling criteria using node labels.<br />ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/<br />This field can have a maximum of 50 entries. |  | MaxProperties: 50 <br />MinProperties: 0 <br />Optional: \{\} <br /> |
| `proxy` _[ProxyConfig](#proxyconfig)_ | proxy is for setting the proxy configurations which will be made available in operand containers managed by the operator as environment variables. |  | Optional: \{\} <br /> |


#### Mode

_Underlying type:_ _string_

Mode indicates the operational state of the optional features.



_Appears in:_
- [BitwardenSecretManagerProvider](#bitwardensecretmanagerprovider)
- [CertManagerConfig](#certmanagerconfig)

| Field | Description |
| --- | --- |
| `Enabled` | Enabled indicates the optional configuration is enabled.<br /> |
| `Disabled` | Disabled indicates the optional configuration is disabled.<br /> |


#### ObjectReference



ObjectReference is a reference to an object with a given name, kind and group.



_Appears in:_
- [CertManagerConfig](#certmanagerconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the resource being referred to. |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `kind` _string_ | Kind of the resource being referred to. |  | MaxLength: 253 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `group` _string_ | Group of the resource being referred to. |  | MaxLength: 253 <br />MinLength: 1 <br />Optional: \{\} <br /> |


#### PluginsConfig



PluginsConfig is for configuring the optional plugins.



_Appears in:_
- [ExternalSecretsConfigSpec](#externalsecretsconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bitwardenSecretManagerProvider` _[BitwardenSecretManagerProvider](#bitwardensecretmanagerprovider)_ | bitwardenSecretManagerProvider is for enabling the bitwarden secrets manager provider plugin for connecting with the bitwarden secrets manager. |  | Optional: \{\} <br /> |


#### ProxyConfig



ProxyConfig is for setting the proxy configurations which will be made available in operand containers managed by the operator as environment variables.



_Appears in:_
- [ApplicationConfig](#applicationconfig)
- [CommonConfigs](#commonconfigs)
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `httpProxy` _string_ | httpProxy is the URL of the proxy for HTTP requests.<br />This field can have a maximum of 2048 characters. |  | MaxLength: 2048 <br />MinLength: 0 <br />Optional: \{\} <br /> |
| `httpsProxy` _string_ | httpsProxy is the URL of the proxy for HTTPS requests.<br />This field can have a maximum of 2048 characters. |  | MaxLength: 2048 <br />MinLength: 0 <br />Optional: \{\} <br /> |
| `noProxy` _string_ | noProxy is a comma-separated list of hostnames and/or CIDRs and/or IPs for which the proxy should not be used.<br />This field can have a maximum of 4096 characters. |  | MaxLength: 4096 <br />MinLength: 0 <br />Optional: \{\} <br /> |


#### SecretReference



SecretReference is a reference to the secret with the given name, which should exist in the same namespace where it will be utilized.



_Appears in:_
- [BitwardenSecretManagerProvider](#bitwardensecretmanagerprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the secret resource being referred to. |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |


#### WebhookConfig



WebhookConfig is for configuring external-secrets webhook specifics.



_Appears in:_
- [ApplicationConfig](#applicationconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `certificateCheckInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#duration-v1-meta)_ | CertificateCheckInterval is for configuring the polling interval to check the certificate validity. | 5m | Optional: \{\} <br /> |


