# ExternalSecretsConfig

**API Group**: `operator.openshift.io/v1alpha1`
**Kind**: `ExternalSecretsConfig`
**Scope**: Cluster (singleton, name must be `cluster`)
**Short Names**: `esc`, `externalsecretsconfig`, `esconfig`

**API Definition**: [`api/v1alpha1/external_secrets_config_types.go`](../../api/v1alpha1/external_secrets_config_types.go)

## Purpose

Primary CR that triggers installation and configuration of the external-secrets operand. Creating this resource causes the operator to deploy all external-secrets components into the `external-secrets` namespace.

**Key Principle**: Singleton pattern enforced via CEL rule — only one instance named `cluster` is permitted per cluster.

## Spec Structure

```go
type ExternalSecretsConfigSpec struct {
    ApplicationConfig  ApplicationConfig  // Operand behavior: logLevel, resources, affinity, tolerations, nodeSelector, proxy, operatingNamespace, webhookConfig
    Plugins            PluginsConfig       // Optional provider plugins (BitwardenSecretManagerProvider)
    ControllerConfig   ControllerConfig    // Deployment config: certProvider, labels, annotations, networkPolicies, componentConfigs, trustedCABundle, replicas
}
```

### ApplicationConfig

| Field | Type | Description |
|-------|------|-------------|
| `logLevel` | `int32` (1-5) | Kubernetes logging level, default 1 |
| `resources` | `ResourceRequirements` | CPU/memory requests and limits (immutable) |
| `affinity` | `Affinity` | Scheduling affinity rules |
| `tolerations` | `[]Toleration` (max 50) | Pod tolerations |
| `nodeSelector` | `map[string]string` (max 50) | Node label selectors |
| `proxy` | `ProxyConfig` | Proxy settings (httpProxy, httpsProxy, noProxy, networkPolicyProvisioning) |
| `operatingNamespace` | `string` (1-63 chars) | Restricts ESO to single namespace; disables ClusterSecretStore and ClusterExternalSecret |
| `webhookConfig` | `*WebhookConfig` | Webhook-specific settings (certificateCheckInterval, default "5m") |

### ControllerConfig

| Field | Type | Description |
|-------|------|-------------|
| `certProvider` | `*CertProvidersConfig` | Certificate management: cert-manager integration (mode, issuerRef, injectAnnotations) — **immutable once set** |
| `labels` | `map[string]string` (max 20) | Custom labels for all operand resources |
| `annotations` | `map[string]string` (max 20) | Custom annotations; reserved domains (`kubernetes.io/`, `openshift.io/`, `cert-manager.io/`, `k8s.io/`) blocked via CEL |
| `networkPolicies` | `[]NetworkPolicy` (max 50) | Custom egress rules per component; name+componentName immutable once added; operator prepends `eso-user-` prefix |
| `componentConfigs` | `[]ComponentConfig` (max 4) | Per-component overrides: `overrideEnv`, `revisionHistoryLimit` |
| `trustedCABundle` | `*ConfigMapKeyReference` | User CA bundle ConfigMap for outbound TLS; must exist in operand namespace |
| `replicas` | `*int32` (≥1) | Desired pod count for the **core controller** deployment (`external-secrets`); defaults to **1** when unset. Values less than 1 are rejected at admission. When greater than 1, the operator enables leader election so exactly one controller instance reconciles at a time. Does not scale the webhook or cert-controller deployments. |

### ComponentConfig

Per-component deployment-level overrides. Components: `ExternalSecretsCoreController`, `Webhook`, `CertController`, `BitwardenSDKServer`.

| Field | Type | Description |
|-------|------|-------------|
| `componentName` | `ComponentName` (enum) | Target component |
| `overrideEnv` | `[]EnvVar` (max 50) | Custom env vars; reserved prefixes (`KUBERNETES_`, `EXTERNAL_SECRETS_`) and names (`HOSTNAME`, `SSL_CERT_DIR`, `SSL_CERT_FILE`) blocked |
| `deploymentConfigs.revisionHistoryLimit` | `*int32` (1-50) | ReplicaSet history limit, default 10 |

### PluginsConfig

| Field | Type | Description |
|-------|------|-------------|
| `bitwardenSecretManagerProvider.mode` | `Mode` (Enabled/Disabled) | Plugin state, default Disabled |
| `bitwardenSecretManagerProvider.secretRef` | `*SecretReference` | TLS secret for bitwarden server; required when Bitwarden is Enabled and cert-manager `mode` is not `Enabled` |

**CEL Validation**: When Bitwarden `mode` is `Enabled`, either `secretRef` must be set **or** `controllerConfig.certProvider.certManager.mode` must be `Enabled` (a present cert-manager config with `mode: Disabled` does not satisfy this rule).

## Status

```go
type ExternalSecretsConfigStatus struct {
    ConditionalStatus          // Embeds []metav1.Condition (patchMergeKey=type)
    ExternalSecretsImage       string  // Deployed external-secrets image
    BitwardenSDKServerImage    string  // Deployed bitwarden image (if applicable)
}
```

### Conditions

| Type | Status | Reason | Meaning |
|------|--------|--------|---------|
| `Ready` | True | `Ready` | Operand deployed and healthy |
| `Ready` | False | `Progressing` | Deployment in progress |
| `Ready` | False | `Failed` | Deployment failed |
| `Degraded` | True | `Failed` | Irrecoverable error (e.g., missing image env var) |
| `Degraded` | False | `Ready` | Operating normally |
| `UpdateAnnotation` | True/False | `Completed`/`Failed` | Annotation tracking status |

## Lifecycle

1. **Creation**: Controller installs operand namespace, network policies, RBAC, services, deployments, webhooks in strict dependency order
2. **Update**: Controller diffs each managed resource field-by-field (not SSA) and applies changes via `UpdateWithRetry`
3. **Deletion**: Finalizer-protected cleanup of operator-managed resources — namespace-scoped objects in `external-secrets`, plus cluster-scoped managed objects (ClusterRoles/Bindings, ValidatingWebhookConfigurations). Operand CRDs themselves are not deleted.

## Example: Minimal Installation

```yaml
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec: {}
```

## Example: High Availability (Multiple Controller Replicas)

```yaml
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  controllerConfig:
    replicas: 3
```

The operator sets `Deployment.spec.replicas` on the core controller to the declared value on every reconcile. Leader election is enabled automatically when `replicas` is greater than 1. Manual `kubectl scale` drift is corrected on the next reconcile cycle.

## Example: Full Configuration

```yaml
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  appConfig:
    logLevel: 2
    proxy:
      httpProxy: "http://proxy.example.com:3128"
      httpsProxy: "https://proxy.example.com:3128"
      noProxy: ".cluster.local,.svc,10.0.0.0/8"
      networkPolicyProvisioning: Managed
  controllerConfig:
    certProvider:
      certManager:
        mode: Enabled
        issuerRef:
          name: my-issuer
          kind: ClusterIssuer
          group: cert-manager.io
        injectAnnotations: "true"
    labels:
      team: platform
    annotations:
      custom.example.com/owner: "team-secrets"
    networkPolicies:
      - name: allow-vault-egress
        componentName: ExternalSecretsCoreController
        egress:
          - to:
              - ipBlock:
                  cidr: 10.0.1.0/24
            ports:
              - port: 8200
                protocol: TCP
    componentConfigs:
      - componentName: ExternalSecretsCoreController
        overrideEnv:
          - name: MY_CUSTOM_VAR
            value: "custom-value"
        deploymentConfigs:
          revisionHistoryLimit: 5
  plugins:
    bitwardenSecretManagerProvider:
      mode: Enabled
```

## Common Mistakes

1. **Name must be `cluster`** — CEL validation rejects any other name
2. **cert-manager fields are immutable** — `mode`, `injectAnnotations`, `issuerRef` cannot be changed after initial set; delete and recreate the CR to change
3. **Network policy entries cannot be removed** — CEL rule `oldSelf.all(op, self.exists(...))` prevents removal of existing entries
4. **Reserved annotation domains** — `kubernetes.io/`, `openshift.io/`, `cert-manager.io/`, `k8s.io/` prefixes are rejected
5. **Reserved env var names** — `KUBERNETES_*`, `EXTERNAL_SECRETS_*`, `HOSTNAME`, `SSL_CERT_DIR`, `SSL_CERT_FILE` are blocked in `overrideEnv`
6. **Bitwarden requires TLS** — When enabling bitwarden without cert-manager, `secretRef` is mandatory

## Related Concepts

- [ExternalSecretsManager](./external-secrets-manager.md) — Global config and feature toggles
