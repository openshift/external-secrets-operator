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
| `mode` _[Mode](#mode)_ | mode indicates whether to use cert-manager for certificate management, instead of built-in cert-controller.<br />Enabled: Makes use of cert-manager for obtaining the certificates for webhook server and other components.<br />Disabled: Makes use of in-built cert-controller for obtaining the certificates for webhook server, which is the default behavior.<br />This field is immutable once set. |  | Enum: [Enabled Disabled] <br />Required: \{\} <br /> |
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


#### ComponentConfig



ComponentConfig defines configuration overrides for a specific external-secrets component.



_Appears in:_
- [ControllerConfig](#controllerconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `componentName` _[ComponentName](#componentname)_ | componentName identifies which external-secrets component this configuration applies to.<br />Valid component names: ExternalSecretsCoreController, Webhook, CertController, BitwardenSDKServer. |  | Enum: [ExternalSecretsCoreController Webhook CertController BitwardenSDKServer] <br />Required: \{\} <br /> |
| `deploymentConfigs` _[DeploymentConfig](#deploymentconfig)_ | deploymentConfigs specifies overrides for the Kubernetes Deployment resource of this component. |  | Optional: \{\} <br /> |
| `overrideEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#envvar-v1-core) array_ | overrideEnv specifies custom environment variables for this component's container. These are merged with operator-managed environment variables, with user-defined values taking precedence.<br />Keys starting with 'HOSTNAME', 'KUBERNETES_', or 'EXTERNAL_SECRETS_' are reserved and will be rejected. |  | MaxItems: 50 <br />Optional: \{\} <br /> |


#### ComponentName

_Underlying type:_ _string_

ComponentName represents the different external-secrets components that can have network policies applied.



_Appears in:_
- [ComponentConfig](#componentconfig)
- [NetworkPolicy](#networkpolicy)

| Field | Description |
| --- | --- |
| `ExternalSecretsCoreController` | CoreController represents the external-secrets component<br /> |
| `BitwardenSDKServer` | BitwardenSDKServer represents the bitwarden-sdk-server component<br /> |
| `Webhook` | Webhook represents the external-secrets webhook component<br /> |
| `CertController` | CertController represents the cert-controller component<br /> |


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
| `annotations` _object (keys:string, values:string)_ | annotations are for adding custom annotations to all the resources created for external-secrets deployment.<br />The annotations are merged with any default annotations set by the operator. User-specified annotations take precedence over defaults in case of conflicts.<br />Annotation keys with prefixes `kubernetes.io/`, `app.kubernetes.io/`, `openshift.io/`, or `k8s.io/` are not allowed. |  | MaxProperties: 20 <br />MinProperties: 0 <br />Optional: \{\} <br /> |
| `networkPolicies` _[NetworkPolicy](#networkpolicy) array_ | networkPolicies specifies the list of network policy configurations<br />to be applied to external-secrets pods.<br />Each entry allows specifying a name for the generated NetworkPolicy object,<br />along with its full Kubernetes NetworkPolicy definition.<br />If this field is not provided, external-secrets components will be isolated<br />with deny-all network policies, which will prevent proper operation. |  | MaxItems: 50 <br />MinItems: 0 <br />Optional: \{\} <br /> |
| `componentConfigs` _[ComponentConfig](#componentconfig) array_ | componentConfigs allows specifying deployment-level configuration overrides for individual external-secrets components. This field enables fine-grained control over deployment settings for each component independently.<br />Each component can only have one configuration entry. |  | MaxItems: 4 <br />MinItems: 0 <br />Optional: \{\} <br /> |


#### ControllerStatus



ControllerStatus holds the observed conditions of the controllers part of the operator.



_Appears in:_
- [ExternalSecretsManagerStatus](#externalsecretsmanagerstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name of the controller for which the observed condition is recorded. |  | Required: \{\} <br /> |
| `conditions` _[Condition](#condition) array_ | conditions holds information of the current state of the external-secrets-operator controllers. |  |  |
| `observedGeneration` _integer_ | observedGeneration represents the .metadata.generation on the observed resource. |  | Minimum: 0 <br /> |


#### DeploymentConfig



DeploymentConfig defines configuration overrides for a Kubernetes Deployment resource.



_Appears in:_
- [ComponentConfig](#componentconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `revisionHistoryLimit` _integer_ | revisionHistoryLimit specifies the number of old ReplicaSets to retain for rollback purposes.<br />This allows rolling back to previous deployment versions using 'kubectl rollout undo'.<br />Must be at least 1 to ensure rollback capability. Maximum value is 50 to limit resource usage.<br />If not specified, defaults to 10. | 10 | Maximum: 50 <br />Minimum: 1 <br />Optional: \{\} <br /> |


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


#### NetworkPolicy



NetworkPolicy represents a custom network policy configuration for operator-managed components.
It includes a name for identification and the network policy rules to be enforced.



_Appears in:_
- [ControllerConfig](#controllerconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name is a unique identifier for this network policy configuration.<br />This name will be used as part of the generated NetworkPolicy resource name. |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `componentName` _[ComponentName](#componentname)_ | componentName specifies which external-secrets component this network policy applies to. |  | Enum: [ExternalSecretsCoreController BitwardenSDKServer] <br />Required: \{\} <br /> |
| `egress` _[NetworkPolicyEgressRule](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#networkpolicyegressrule-v1-networking) array_ | egress is a list of egress rules to be applied to the selected pods. Outgoing traffic<br />is allowed if there are no NetworkPolicies selecting the pod (and cluster policy<br />otherwise allows the traffic), OR if the traffic matches at least one egress rule<br />across all the NetworkPolicy objects whose podSelector matches the pod. If<br />this field is empty then this NetworkPolicy limits all outgoing traffic (and serves<br />solely to ensure that the pods it selects are isolated by default).<br />The operator will automatically handle ingress rules based on the current running ports. |  | Required: \{\} <br /> |


#### ObjectReference

_Underlying type:_ _[struct{Name string "json:\"name\""; Kind string "json:\"kind,omitempty\""; Group string "json:\"group,omitempty\""}](#struct{name-string-"json:\"name\"";-kind-string-"json:\"kind,omitempty\"";-group-string-"json:\"group,omitempty\""})_

ObjectReference is a reference to an object with a given name, kind and group.



_Appears in:_
- [CertManagerConfig](#certmanagerconfig)



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


