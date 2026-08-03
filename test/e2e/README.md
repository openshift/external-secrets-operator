# E2E test labels

Run tests from the repo root:

```bash
make test-e2e E2E_GINKGO_LABEL_FILTER="<expression>"
```

See [Ginkgo label filters](https://onsi.github.io/ginkgo/#spec-labels) for expression syntax. An empty filter runs every spec:

```bash
make test-e2e E2E_GINKGO_LABEL_FILTER=""
```

**No runtime skips.** If a spec is selected by the label filter, its prerequisites must be present or the spec **fails**. Use label filters to opt in to suites whose prerequisites you have configured.

## Label keys

| Key | Values | Meaning |
|-----|--------|---------|
| `Platform` | `AWS`, `GCP`, `Generic` | Cluster or portability requirement |
| `Provider` | `AWS`, `Bitwarden`, `Vault` | External secret backend integration |
| `API` | `Bitwarden` | Direct HTTP tests against bitwarden-sdk-server |
| `Feature` | see below | Optional capability or functional area |

### Feature values

| Value | Test area |
|-------|-----------|
| `OverrideEnv` | Custom env vars on operand deployments |
| `RevisionHistoryLimit` | Deployment revision history limits |
| `UnsafeAllowGenericTargets` | ExternalSecretsManager feature gate propagation |
| `CustomAnnotations` | Annotation apply/remove and managed-annotation restoration |
| `CustomLabels` | Custom and managed label lifecycle |
| `NetworkPolicy` | Static and custom network policy naming |
| `Proxy` | Proxy egress network policy (requires cluster-wide OpenShift proxy) |
| `TrustedCABundle` | trustedCABundle ConfigMap mounting/validation and Vault TLS failure→recovery |
| `ExternalSecretsTemplating` | ExternalSecret template merge (Kubernetes + Vault → dockerconfigjson) |
| `Upgrade` | Post-upgrade migration checks (temporary) |

## Default filter

`make test-e2e` uses:

```
Platform: isSubsetOf {AWS,Generic} && !(Feature: containsAny {Proxy, Upgrade}) && !(Provider: containsAny Bitwarden)
```

This runs portable tests plus AWS provider tests, and **API:Bitwarden** health/auth (bitwarden-sdk-server is deployed automatically — no pre-provisioned secrets). Excludes `Feature:Proxy`, `Feature:Upgrade`, and all `Provider:Bitwarden` specs (including Bitwarden provider sync and API Secrets API).

## Prerequisites

### Secrets you provision

| Secret | Namespace | Keys | Required when filter includes |
|--------|-----------|------|-------------------------------|
| `aws-creds` | `kube-system` | `aws_access_key_id`, `aws_secret_access_key` | `Provider:AWS` (`Platform:AWS` or `Platform:GCP`) |
| `bitwarden-creds` | `external-secrets-operator` | `token`, `organization_id`, `project_id` | `Provider:Bitwarden` |

### Secrets created by tests

| Secret | Namespace | Keys | Created by |
|--------|-----------|------|------------|
| `bitwarden-tls-certs` | `external-secrets` | `tls.crt`, `tls.key`, `ca.crt` | `API:Bitwarden` and `Provider:Bitwarden` via `ensureBitwardenOperandReady` |

The bitwarden-sdk-server plugin uses **`bitwarden-tls-certs`** (TLS materials for the in-cluster SDK server). That is separate from **`bitwarden-creds`**, which holds a live Bitwarden Secrets Manager API token for provider sync and Secrets API tests.

### Other environment requirements

| Requirement | Required when filter includes |
|-------------|-------------------------------|
| Cluster-wide OpenShift proxy (`proxy.config.openshift.io/cluster`) | `Feature:Proxy` |
| OpenShift with `redhat-operators` catalog (`openshift-marketplace`) | Main e2e Describe (`e2e_test.go`) — root `BeforeAll` installs Red Hat cert-manager Operator via OLM if not already present |

If a prerequisite is missing, the affected spec **fails** with a message pointing here — it does not skip.

## Filter recipes

| Filter | What runs |
|--------|-----------|
| *(default, omitted)* | `Platform:AWS` + `Platform:Generic` except `Feature:Proxy`, `Feature:Upgrade`, `Provider:Bitwarden`; includes `API:Bitwarden` health/auth |
| `""` | Entire suite |
| `Platform:AWS` | AWS Secret Manager sync, push, and refresh |
| `Platform:GCP && Provider:AWS` | GCP cluster using AWS Secrets Manager |
| `Provider:AWS` | Any AWS Secrets Manager integration (`Platform:AWS` or `Platform:GCP`) |
| `Provider:Bitwarden` | Bitwarden provider sync and API Secrets API |
| `Provider:Vault` | Vault HTTPS + trustedCABundle failure/recovery (uses suite-installed Red Hat cert-manager Operator) |
| `API:Bitwarden` | bitwarden-sdk-server HTTP API (deploys plugin + `bitwarden-tls-certs` automatically) |
| `API:Bitwarden \|\| Provider:Bitwarden` | All Bitwarden HTTP and provider tests (requires `bitwarden-creds` for Secrets API / provider sync) |
| `Feature:TrustedCABundle` | Trusted CA bundle suite (mount/validation + Vault TLS path when combined with `Provider:Vault`) |
| `Feature:ExternalSecretsTemplating` | ExternalSecret templating merge (Kubernetes + Vault dockerconfig) |
| `Feature:Proxy` | Proxy egress network policy |
| `Feature:Upgrade` | Post-upgrade network policy migration check |
| `Feature:NetworkPolicy` | Static and custom network policy naming |
| `Feature:OverrideEnv` | Component override env vars |
| `Feature:RevisionHistoryLimit` | Revision history limit defaults and overrides |
| `Feature:UnsafeAllowGenericTargets` | UnsafeAllowGenericTargets feature propagation |
| `Feature:CustomAnnotations` | Annotation lifecycle tests |
| `Feature:CustomLabels` | Label lifecycle tests |
| `Platform: isSubsetOf {GCP,Generic} && !(Feature: containsAny {Proxy, Upgrade}) && !(Provider: containsAny Bitwarden)` | Portable + GCP/AWS cross-platform tests on a GCP cluster |

### Combining labels

```bash
# Bitwarden provider and API together (requires bitwarden-creds)
make test-e2e E2E_GINKGO_LABEL_FILTER="Provider:Bitwarden || API:Bitwarden"

# All network-policy-related tests
make test-e2e E2E_GINKGO_LABEL_FILTER="Feature:NetworkPolicy || Feature:Proxy"

# AWS integration only (any platform label that uses AWS SM)
make test-e2e E2E_GINKGO_LABEL_FILTER="Provider:AWS"

# Vault TLS + trustedCABundle (uses cert-manager from root BeforeAll)
make test-e2e E2E_GINKGO_LABEL_FILTER="Provider:Vault"
```

## Specs by label

### `Platform:AWS` + `Provider:AWS`

File: `e2e_test.go` — **AWS Secret Manager**

- ClusterSecretStore, ExternalSecret sync, PushSecret, secret refresh
- Requires `aws-creds` in `kube-system`

### `Platform:GCP` + `Provider:AWS`

File: `e2e_test.go` — **Cross-platform: GCP cluster and AWS Secrets Manager**

- Same AWS SM flows on a non-AWS cluster
- Requires `aws-creds` in `kube-system`

### `Platform:Generic` feature specs

File: `e2e_test.go`

| Feature | Context |
|---------|---------|
| `OverrideEnv` | Environment Variables |
| `RevisionHistoryLimit` | Deployment Revision History Limit |
| `UnsafeAllowGenericTargets` | UnsafeAllowGenericTargets feature |
| `CustomAnnotations` | Annotations; Managed Annotation Restoration |
| `CustomLabels` | Custom Labels; Managed Label Restoration |
| `NetworkPolicy` | Static Network Policy Naming; Custom Network Policy Naming |
| `NetworkPolicy` + `Upgrade` | Post-upgrade skip-np-cleanup-check annotation (also tagged `Feature:Upgrade`) |
| `Proxy` | Proxy Egress Network Policy |

File: `trusted_ca_bundle_test.go`

| Feature | Describe |
|---------|----------|
| `TrustedCABundle` | Trusted CA Bundle (mount / `SSL_CERT_DIR` / Degraded / watch-label restore) |

### `Provider:Vault` + `Feature:TrustedCABundle`

File: `e2e_test.go` — **Vault Secret Manager**

- **BeforeAll:** cert-manager CA (`isCA`) + server Certificate for Vault HTTPS; Vault deploy/init
- Uses Red Hat cert-manager Operator installed by the root `BeforeAll` of the main e2e Describe (`ensureCertManagerOperatorReady`)
- Asserts SecretStore `Ready=False` / `InvalidProviderConfig` when `trustedCABundle` points at a valid but non-matching CA (`vault-e2e-sample-ca`), then Ready after switching to the Vault CA ConfigMap (`vault-server-ca` from the isCA Certificate)
- After trustedCABundle switch: SecretStore Ready → PushSecret → ExternalSecret → verify synced Secret
- Excluded from the default label filter (longer Vault HTTPS setup)
- Does not uninstall cert-manager on teardown (shared for other suites)

### `Provider:Vault` + `Feature:ExternalSecretsTemplating`

File: `e2e_test.go` — **Vault Secret Manager** (same Context)

- Creates a namespaced Kubernetes SecretStore (`kubernetes-backend`) with dedicated `eso-secret-reader` SA/Role/RoleBinding (no existing e2e Kubernetes-provider fixture)
- Pushes a dockerconfigjson Secret to Vault, reads another from Kubernetes, merges both via ExternalSecret `target.template` into `merged-registry-pull-secret`
- Asserts the target Secret is `kubernetes.io/dockerconfigjson` and contains both registry auth hosts

### `Provider:Bitwarden`

File: `bitwarden_es_test.go` — **Bitwarden Provider**

- Creates `bitwarden-tls-certs`, enables Bitwarden plugin on ExternalSecretsConfig, runs PushSecret/ExternalSecret flows
- Requires `bitwarden-creds` in `external-secrets-operator`

File: `bitwarden_api_test.go` — **Secrets API** context (also labeled `Provider:Bitwarden`)

- Live Bitwarden cloud API CRUD via bitwarden-sdk-server
- Requires `bitwarden-creds`

### `API:Bitwarden`

File: `bitwarden_api_test.go` — **Bitwarden SDK Server API**

- **BeforeAll:** generates TLS materials, creates `bitwarden-tls-certs` in `external-secrets`, enables `plugins.bitwardenSecretManagerProvider.secretRef`, waits for bitwarden-sdk-server
- **Health / Auth:** no `bitwarden-creds` required
- **Secrets API:** labeled `Provider:Bitwarden`; requires `bitwarden-creds` (excluded from default filter)

## Test output and failure artifacts

Ginkgo JUnit and JSON reports are always written under the output directory:

| Environment | Output directory |
|-------------|------------------|
| OpenShift CI | `$ARTIFACT_DIR` (set by CI) |
| Local (`make test-e2e` from repo root) | `_output/` at the repo root |

Files written on every run:

- `_output/e2e-junit.xml`
- `_output/e2e-report.json`

When a spec in the main e2e describe fails, a snapshot is also written to:

```
_output/e2e-artifacts/failure-<timestamp>/
├── pods/       # last 500 log lines and describe YAML per pod (operator, operand, test namespaces)
├── events/     # recent events per namespace
└── resources/  # ExternalSecretsConfig, ClusterSecretStores, ExternalSecrets, PushSecrets (YAML)
```

Override the base directory by setting `ARTIFACT_DIR` before running tests:

```bash
ARTIFACT_DIR=/tmp/eso-e2e make test-e2e
```
