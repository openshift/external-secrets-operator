# External Secrets Operator for Red Hat OpenShift

This repository contains the External Secrets Operator for Red Hat OpenShift. The operator runs in the `external-secrets-operator` namespace and deploys and manages the upstream [external-secrets](https://github.com/external-secrets/external-secrets) application on OpenShift clusters using static YAML manifests embedded as bindata.

The External Secrets Operator operates as a cluster-wide service that integrates external secrets management systems -- such as AWS Secrets Manager, HashiCorp Vault, Google Secrets Manager, Azure Key Vault, IBM Cloud Secrets Manager, and AWS Systems Manager Parameter Store -- with the OpenShift Container Platform, performing secret fetching, refreshing, and provisioning within the cluster.

Using the External Secrets Operator ensures the following:
- Decouples applications from the secret-lifecycle management.
- Centralizes secret storage to support compliance requirements.
- Enables secure and automated secret rotation.
- Supports multi-cloud secret sourcing with fine-grained access control.
- Centralizes and audits access control.

## Architecture

The operator uses two singleton custom resources (both named `cluster`) for configuration:

| Custom Resource | Purpose |
|-----------------|---------|
| `ExternalSecretsConfig` (`esc`) | Operand installation and configuration |
| `ExternalSecretsManager` (`esm`) | Status aggregation and global settings |

Three controllers handle the operator lifecycle:

| Controller | Responsibility |
|------------|----------------|
| `external_secrets` | Installs and manages the external-secrets application based on user-defined configuration in the `ExternalSecretsConfig` CR. |
| `external_secrets_manager` | Reconciles the `ExternalSecretsManager` CR, provides aggregated status from other controllers, and auto-creates the default `cluster` CR. |
| `crd_annotator` | Adds `cert-manager.io/inject-ca-from` annotations to external-secrets CRDs. Activates only when [cert-manager](https://cert-manager.io/) is installed. |

The operand manifests are pre-rendered from upstream Helm charts at build time (`hack/update-external-secrets-manifests.sh`) and embedded via `openshift/build-machinery-go` into `pkg/operator/assets/bindata.go`.

## Tech Stack

| Component | Version |
|-----------|---------|
| Go | 1.26 (workspace mode via `go.work`) |
| Kubernetes libraries | v0.35.6 |
| controller-runtime | v0.23.3 |
| cert-manager | v1.18.5 |
| External Secrets (operand) | v2.5.0 |
| Container tool | podman (default; override with `CONTAINER_TOOL=docker`) |

## Project Structure

```text
api/                  API type definitions (ExternalSecretsConfig, ExternalSecretsManager)
cmd/                  Operator entry point
pkg/
  controller/         Controller implementations (external_secrets, external_secrets_manager, crd_annotator)
  operator/           Operand lifecycle, asset management, bindata
config/               Kustomize overlays, CRD bases, RBAC, samples
bindata/              Embedded operand manifests (source for bindata.go)
test/                 Test suites (e2e/, apis/, utils/)
hack/                 Build and codegen scripts
docs/                 Product/user docs, anti-patterns, OpenSpec notes
harness-evals/harness-docs/  Domain guidelines, architecture, ADRs, development workflows
bundle/               OLM bundle manifests
tools/                Build tooling module
images/               Container image definitions
vendor/               Vendored Go dependencies
```

## Getting Started

### Prerequisites

- Go 1.26+
- podman (or docker) 17.03+
- kubectl v1.32.1+ / oc
- Access to a Kubernetes v1.32.1+ / OpenShift cluster

### Building

```sh
make build              # Full build: manifests, generate, fmt, vet, then compile
make build-operator     # Compile the operator binary only (no codegen or checks)
make image-build        # Build the container image with podman
```

To build and push a custom image:

```sh
make image-build image-push IMG=<registry>/external-secrets-operator:<tag>
```

### Deploying to a Cluster

Install CRDs and deploy the operator:

```sh
make install                                            # Install CRDs
make deploy IMG=<registry>/external-secrets-operator:<tag>  # Deploy the operator
kubectl apply -k config/samples/                        # Create sample CRs
```

To uninstall:

```sh
kubectl delete -k config/samples/
make uninstall
make undeploy
```

### Generating a Standalone Installer

```sh
make build-installer IMG=<registry>/external-secrets-operator:<tag>
# Produces dist/install.yaml containing all resources
kubectl apply -f dist/install.yaml
```

## Usage

Once the operator is running, configure the operand through the two singleton custom resources (both named `cluster`).

### Quickstart Example

```sh
# Create the default ExternalSecretsConfig (installs the external-secrets operand). The
# ExternalSecretsManager CR is auto-created by the operator at startup, no action needed.
# `config/samples/` also contains provider-specific example CRs (ClusterSecretStore, ExternalSecret,
# etc.) with placeholder values that require editing before use -- apply only the CR below for a
# working quickstart; see `config/samples/kustomization.yaml` for the full list.
oc apply -f config/samples/operator_v1alpha1_externalsecretsconfig.yaml

# Check aggregated status from the ExternalSecretsManager CR
oc get esm cluster -o yaml

# Check the ExternalSecretsConfig CR
oc get esc cluster -o yaml

# Verify the operand pods are running
oc get pods -n external-secrets
```

See `config/samples/` for example CRs and the [domain guidelines](harness-evals/harness-docs/domain/) for the full `ExternalSecretsConfig` and `ExternalSecretsManager` field reference.

## Testing

| Make Target | Description |
|-------------|-------------|
| `make test` | Run all non-e2e tests (`test-apis` + `test-unit`); no cluster required. |
| `make test-unit` | Run unit tests (excludes `test/e2e`, `test/apis`, `test/utils`). |
| `make test-apis` | Run API integration tests (Ginkgo + envtest). |
| `make test-e2e` | Run end-to-end tests against a live cluster. |

E2E tests support label filtering for provider-specific or scenario-specific runs:

```sh
make test-e2e E2E_GINKGO_LABEL_FILTER="Provider:Vault"
```

For full E2E details including prerequisites, suite-specific commands, and cross-platform labels, see [test/e2e/README.md](test/e2e/README.md).

Detailed testing conventions (unit test style, table-driven patterns, envtest setup) are documented in [harness-evals/harness-docs/testing-guidelines.md](harness-evals/harness-docs/testing-guidelines.md).

## Verification and Linting

Before submitting a PR, run the CI gate check:

```sh
make verify          # Runs vet, fmt, deps, bindata, generated files, govulncheck, markdownlint, and git diff
make lint            # Run golangci-lint
make lint-fix        # Run golangci-lint with auto-fix
make lint-markdown   # Run markdownlint-cli2 on docs (also invoked by make verify)
```

`make verify` is the single gate that CI enforces. It catches generated-file drift, dependency inconsistencies, formatting issues, markdown style problems, and vulnerability scan failures.

## Code Generation

Several files in this repository are generated and must not be hand-edited:

| File | Regenerate With |
|------|-----------------|
| `pkg/operator/assets/bindata.go` | `make update-bindata` |
| `pkg/controller/client/fakes/fake_ctrl_client.go` | `go generate ./pkg/controller/client/...` |
| `**/zz_generated.deepcopy.go` | `make generate` |
| CRD YAML in `config/crd/bases/` | `make manifests` |

To regenerate everything at once:

```sh
make update    # generate + manifests + update-operand-manifests + update-bindata + bundle + docs
```

## Further Documentation

### Domain Guidelines and Agentic Documentation

Detailed rules, architecture deep-dives, and development workflows are in `harness-evals/harness-docs/`; component-specific ADRs are in `docs/decisions/` (see the list below):

| Guideline | Scope |
|-----------|-------|
| [Security](harness-evals/harness-docs/security-guidelines.md) | CEL validation, annotation/label restrictions, container hardening, RBAC, network policies, TLS |
| [Performance](harness-evals/harness-docs/performance-guidelines.md) | Label-filtered caches, change detection, event predicates, requeue strategy, concurrency |
| [Error Handling](harness-evals/harness-docs/error-handling-guidelines.md) | Error classification, status conditions, requeue matrix, events |
| [API Contracts](harness-evals/harness-docs/api-contracts-guidelines.md) | Singleton enforcement, field immutability, CEL rules, `.testsuite.yaml` patterns |
| [Testing](harness-evals/harness-docs/testing-guidelines.md) | Unit tests, API integration tests, E2E with Ginkgo labels |
| [Integration](harness-evals/harness-docs/integration-guidelines.md) | cert-manager, OLM, proxy, trusted CA, console, metrics, multi-arch, webhooks |

- `harness-evals/harness-docs/ESO_DEVELOPMENT.md` -- Development workflows, build targets, common tasks
- `harness-evals/harness-docs/ESO_TESTING.md` -- Test suites, patterns, E2E labels
- `harness-evals/harness-docs/architecture/` -- Controller internals, resource management, bindata pipeline
- `harness-evals/harness-docs/domain/` -- API documentation for ExternalSecretsConfig and ExternalSecretsManager
- `docs/decisions/` -- Component-specific architectural decision records
- `harness-evals/harness-docs/references/` -- Enhancement proposals catalog and ecosystem links

## For AI Agents

If you are an AI agent or LLM-based tool working on this repository, start with [AGENTS.md](AGENTS.md). It indexes critical patterns and deeper documentation in `harness-evals/harness-docs/`. Claude Code–specific build commands and behavioral preferences are in [CLAUDE.md](CLAUDE.md).

Recommended reading order: `AGENTS.md` → relevant `harness-evals/harness-docs/*-guidelines.md` → `domain/` → `architecture/` → `decisions/` → `ESO_DEVELOPMENT.md`

## Contributing

We welcome contributions from the community! See [CONTRIBUTING.md](CONTRIBUTING.md) for the full workflow (setup, conventions, PR checklist, and testing expectations).

In short: fork, branch from `main`, run `make verify` / `make test` / `make lint`, and open a PR that explains what changed and why.

## Security

To report a vulnerability, see [SECURITY.md](SECURITY.md). Do not open a public GitHub issue for security reports.

## External References

- [External Secrets Operator on OpenShift (Red Hat docs)](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/security_and_compliance/external-secrets-operator-for-red-hat-openshift)
- [external-secrets upstream project](https://external-secrets.io/latest/)
- [cert-manager Operator for Red Hat OpenShift](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/security_and_compliance/cert-manager-operator-for-red-hat-openshift)
- [Enhancement: ESO on OpenShift](https://github.com/openshift/enhancements/blob/master/enhancements/external-secrets-operator/external-secrets-operator.md)
- [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025-2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
