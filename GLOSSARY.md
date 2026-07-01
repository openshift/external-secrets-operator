# Glossary

This glossary defines domain-specific terminology used throughout the External Secrets Operator codebase and documentation.

## Core Concepts

**Bindata**  
Static assets (YAML manifests) compiled into the Go binary via the `go-bindata` tool. Located in `bindata/` directory and compiled to `pkg/operator/assets/bindata.go`. These are templates that are decoded and mutated at runtime.

**ESC**  
Short for `ExternalSecretsConfig`, the primary user-facing CRD. A cluster-scoped singleton CR named "cluster" that controls operand installation, cert-manager integration, Bitwarden plugin, proxy configuration, and network policies.

**ESM**  
Short for `ExternalSecretsManager`, the global operator-level configuration CRD. A cluster-scoped singleton CR named "cluster" that provides lower-priority defaults and optional features. Auto-created at operator install.

**ESO**  
Ambiguous term that can mean:
1. **External Secrets Operator** - This project (the OpenShift operator that manages the operand)
2. **External-Secrets.io** - The upstream open-source project we deploy as the operand

Context determines which meaning applies. In code comments and docs, prefer "operator" or "operand" for clarity.

**Operand**  
The upstream [external-secrets](https://github.com/external-secrets/external-secrets) application that this operator manages. The operand consists of deployments, services, RBAC, webhooks, and CRDs that end users interact with (`ExternalSecret`, `SecretStore`, `ClusterSecretStore`, etc.).

**Operator**  
This codebase (`github.com/openshift/external-secrets-operator`). The operator manages the operand's lifecycle (installation, upgrades, configuration, drift detection).

## Resource Types

**Managed Resources**  
Kubernetes resources owned and reconciled by the operator, labeled with `app=external-secrets`. Includes operand Deployments, Services, RBAC, NetworkPolicies, Secrets, ConfigMaps, Certificates, and ValidatingWebhookConfigurations. The operator drift-detects and reverts manual changes to these resources.

**Singleton CR**  
A custom resource limited to a single instance cluster-wide. Both `ExternalSecretsConfig` and `ExternalSecretsManager` are singletons enforced via CEL validation (`self.metadata.name == 'cluster'`).

**Watched Resources**  
Kubernetes resources that trigger reconciliation when changed, but are not owned by the operator. Examples: user-provided ConfigMaps (trusted CA bundle), cert-manager Issuers/ClusterIssuers, upstream ESO CRDs.

## Architectural Patterns

**Drift Detection**  
The process of comparing actual cluster state against desired state and reverting manual changes. Implemented via `common.HasObjectChanged()` which uses type-specific field comparison (not full `DeepEqual`).

**Reconciliation Order**  
The strict dependency order in which resources are created:  
`Namespace → NetworkPolicies → ServiceAccounts → Certificates → Secrets → TrustedCA ConfigMap → RBAC → Services → Deployments → ValidatingWebhooks → CR annotation tracking`

**Conditional Resources**  
Resources created only when specific configuration is enabled. Examples: cert-controller deployment (when cert-manager is NOT enabled), Bitwarden deployment (when Bitwarden IS enabled), Certificates (when cert-manager IS enabled).

**Three-Controller Architecture**  
The operator runs three separate controllers in a single binary:
- `external-secrets-controller` - main operand reconciliation
- `external-secrets-manager` - status aggregation
- `crd-annotator` - cert-manager CA injection (conditional)

## Code Generation

**Generated Files**  
Files created by code generators that must NOT be edited by hand:
- `zz_generated.deepcopy.go` - DeepCopy methods (from `make generate`)
- `config/crd/bases/*.yaml` - CRD manifests (from `make manifests`)
- `pkg/operator/assets/bindata.go` - Compiled YAML assets (from `make update-bindata`)
- `config/rbac/role.yaml` - Operator RBAC (from `make manifests`)
- `docs/api_reference.md` - API documentation (from `make docs`)

Always use `make update` after changes that affect these files.

**Kubebuilder Markers**  
Special Go comments (`// +kubebuilder:...`) that control code generation. Examples:
- `+kubebuilder:validation:XValidation` - CEL validation rules
- `+kubebuilder:rbac` - RBAC role generation
- `+kubebuilder:printcolumn` - kubectl output columns
- `+kubebuilder:resource` - CRD metadata

## Error Handling

**ReconcileError**  
Custom error type in `pkg/controller/common/errors.go` with three reasons:
- `IrrecoverableError` - No retry (config validation failures, permission errors)
- `RetryRequiredError` - Requeue after 30s (transient failures, conflicts)
- `UserConfigurationError` - No retry, recovery driven by watches (invalid user config)

**FromClientError**  
Helper function that auto-classifies Kubernetes API errors into the appropriate `ReconcileError` reason. `Unauthorized`, `Forbidden`, `Invalid`, and `BadRequest` become irrecoverable; everything else becomes retry-required.

## Testing

**envtest**  
Kubernetes API server testing environment used for API integration tests. Runs a real API server (etcd + kube-apiserver) without kubelet. Requires Kubernetes >= 1.25 for CEL validation support.

**FakeCtrlClient**  
Counterfeiter-generated mock implementation of the `CtrlClient` interface used in unit tests. Located in `pkg/controller/client/fakes/`.

**Table-Driven Tests**  
Unit test pattern where test cases are defined as a slice of structs, then iterated with `t.Run()`. Standard pattern across all `*_test.go` files in `pkg/`.

**Test Tiers**  
Three levels of testing:
1. **Unit** - `pkg/**/*_test.go`, stdlib testing, no cluster
2. **API** - `test/apis/`, Ginkgo + envtest, declarative YAML test suites
3. **E2E** - `test/e2e/`, Ginkgo + live cluster, labeled by platform/provider

## Integrations

**cert-manager**  
Optional dependency for automated TLS certificate provisioning. When installed, the operator can create `Certificate` resources instead of using the built-in cert-controller. Detected via CRD discovery at startup.

**Bitwarden Plugin**  
The only currently supported external secrets provider plugin. Runs as a separate `bitwarden-sdk-server` deployment when enabled via `spec.plugins.bitwardenSecretManagerProvider.mode: Enabled`.

**CNO (Cluster Network Operator)**  
OpenShift component that injects cluster CA certificates into ConfigMaps labeled with `config.openshift.io/inject-trusted-cabundle: "true"`. Used for proxy CA bundle injection.

**OLM (Operator Lifecycle Manager)**  
OpenShift framework for managing operator installation, upgrades, and permissions. Sets environment variables like `RELATED_IMAGE_EXTERNAL_SECRETS` on the operator pod.

## Network & Security

**Network Policy Architecture**  
Default-deny base policy with layered allow-policies:
- System policies (prefixed `eso-sys-`) for API server, webhook, DNS, cert-controller, Bitwarden
- User policies (prefixed `eso-user-`) for custom egress rules
- Deny-all base applied first, allow-policies layered on top

**Hardened Security Context**  
Programmatically enforced container security settings:
- `AllowPrivilegeEscalation: false`
- `Capabilities.Drop: ["ALL"]`
- `ReadOnlyRootFilesystem: true`
- `RunAsNonRoot: true`
- `SeccompProfile.Type: RuntimeDefault`

Applied via `updateContainerSecurityContext()` to all operand containers.

**HTTP/2 Disabled**  
Default configuration (`--enable-http2=false`) to mitigate known HTTP/2 vulnerabilities. Applies to operator metrics and webhook servers.

## Tooling

**go.work (Go Workspace)**  
Multi-module workspace configuration with four modules: `.`, `./cmd/external-secrets-operator`, `./test`, `./tools`. Requires special handling in Makefile (clearing `GOFLAGS` for test/fmt targets).

**controller-runtime**  
Kubernetes controller framework providing manager, cache, client, reconciler primitives. The operator uses version specified in `go.mod`.

**Ginkgo**  
BDD-style testing framework used for API and E2E tests. Provides `Describe`, `Context`, `It`, `BeforeEach`, `AfterEach`, labels, and table-driven specs.

**counterfeiter**  
Mock/fake code generator used to create `FakeCtrlClient` from the `CtrlClient` interface. Run via `go generate ./pkg/controller/client/...`.

## Abbreviations

- **API**: Application Programming Interface (or API integration tests)
- **CA**: Certificate Authority
- **CEL**: Common Expression Language (Kubernetes validation)
- **CR**: Custom Resource
- **CRD**: Custom Resource Definition
- **CSV**: ClusterServiceVersion (OLM bundle metadata)
- **E2E**: End-to-End (integration tests on live cluster)
- **FIPS**: Federal Information Processing Standards
- **RBAC**: Role-Based Access Control
- **TLS**: Transport Layer Security
- **YAML**: YAML Ain't Markup Language

## Field Naming Conventions

**Asset Name Constants**  
Pattern: `<resourceKind>_<resourceName>AssetName` in camelCase  
Example: `controllerDeploymentAssetName`, `webhookServiceAssetName`

**Environment Variable Constants**  
Pattern: all-caps with suffix `EnvVarName`  
Example: `externalSecretsImageEnvVarName`, `bitwardenSDKServerImageEnvVarName`

**Finalizer Format**  
Pattern: `<crd-plural>.<api-group>/<controller-name>`  
Example: `externalsecretsconfigs.operator.openshift.io/external-secrets-controller`

**Bindata YAML Files**  
Pattern: `<kind-lowercase>_<resource-name>.yml` in `bindata/external-secrets/resources/`  
Example: `deployment_external-secrets.yml`, `clusterrole_external-secrets-controller.yml`

## See Also

- **AGENTS.md** - Entry point for AI agents and developers
- **ARCHITECTURE.md** - High-level system architecture
- **docs/decisions/** - Architecture Decision Records explaining why things are designed as they are
- **CONTRIBUTING.md** - Contribution process and coding conventions
