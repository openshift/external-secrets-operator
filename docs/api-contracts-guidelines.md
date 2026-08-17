# API Contracts Guidelines

## API Group and Versioning

- Group: `operator.openshift.io`, Version: `v1alpha1`. All types live in `api/v1alpha1/`.
- Only one version exists. The package-level marker `+groupName=operator.openshift.io` in `groupversion_info.go` sets the group.
- Every new type must be registered via `SchemeBuilder.Register()` in its `init()` function within the types file.

## CRD Resources

Two cluster-scoped singletons exist:

| Kind | Plural | Short Names | Purpose |
|---|---|---|---|
| ExternalSecretsConfig | externalsecretsconfigs | esc, externalsecretsconfig, esconfig | Configures the external-secrets operand |
| ExternalSecretsManager | externalsecretsmanagers | esm, externalsecretsmanager, esmanager | Global operator-level config, auto-created at install |

Both are enforced singletons via CEL: `self.metadata.name == 'cluster'`. Both use `+genclient:nonNamespaced` and `scope=Cluster`.

## Required Kubebuilder Markers on CRD Types

Every root CRD type must carry these markers:
```
+genclient
+genclient:nonNamespaced
+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
+kubebuilder:object:root=true
+kubebuilder:subresource:status
+kubebuilder:resource:path=<plural>,scope=Cluster,categories={external-secrets-operator, external-secrets},shortName=<list>
+kubebuilder:metadata:labels={"app.kubernetes.io/name=<kind-lower>", "app.kubernetes.io/part-of=external-secrets-operator"}
+operator-sdk:csv:customresourcedefinitions:displayName="<Kind>"
```

List types need `+kubebuilder:object:root=true` and `+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object`.

## Field-Level Validation Conventions

### String Fields
- Set `+kubebuilder:validation:MinLength` and `+kubebuilder:validation:MaxLength` on spec string fields (e.g., 1-253 for names, 0-2048 for URLs, 0-4096 for noProxy).
- Status fields (e.g., `ExternalSecretsImage`, `BitwardenSDKServerImage`) and custom condition message fields typically omit validation markers.
- Use `+kubebuilder:validation:Pattern` for key-format constraints (e.g., ConfigMap keys: `^[-._a-zA-Z0-9]+$`).

### Map Fields
- Set `+kubebuilder:validation:MinProperties` and `+kubebuilder:validation:MaxProperties` on spec maps (typical max: 20 for labels/annotations, 50 for nodeSelector).
- Set `+mapType=granular` for strategic-merge-patchable maps; `+mapType=atomic` for replace-on-update maps.

### List Fields
- Set `+kubebuilder:validation:MinItems` and `+kubebuilder:validation:MaxItems` on spec lists.
- Status lists (e.g., `Conditions`) typically omit MinItems/MaxItems validation.
- For merge-patchable lists: use `+listType=map` with `+listMapKey=<field>`. Composite keys are supported (e.g., networkPolicies uses both `name` and `componentName`).
- For replace-on-update lists: use `+listType=atomic`.
- Status subresource lists use `+patchMergeKey=<key>` and `+patchStrategy=merge` in addition to listType markers for proper merge behavior.

### Numeric Fields
- Use `+kubebuilder:validation:Minimum` and `+kubebuilder:validation:Maximum` (e.g., logLevel: 1-5, revisionHistoryLimit: 1-50).

### Enum Fields
- Use `+kubebuilder:validation:Enum` for closed sets. Define a named Go type with exported constants (e.g., `Mode` with `Enabled`/`Disabled`, `ManagementState` with `Managed`/`Unmanaged`, `ComponentName` with four values).

## CEL Validation Rules (XValidation)

CEL rules via `+kubebuilder:validation:XValidation` are the primary mechanism for cross-field and immutability validation. Patterns used:

- **Singleton enforcement**: `rule="self.metadata.name == 'cluster'"` on the root type.
- **Immutability**: `rule="self == oldSelf"` on individual fields (e.g., certManager `mode`, `issuerRef`, `injectAnnotations`).
- **List key immutability**: `rule="oldSelf.all(op, self.exists(p, p.name == op.name && p.componentName == op.componentName))"` to prevent renaming list entries.
- **Cross-field dependencies**: Complex rules on spec structs to enforce "if X is enabled, then Y or Z must be configured."
- **Annotation domain blocklists**: Regex-based `self.all(key, !key.matches(...))` rules blocking `kubernetes.io/`, `openshift.io/`, `k8s.io/`, `cert-manager.io/` prefixes.
- **Reserved env var names**: `self.all(e, !['PREFIX_'].exists(p, e.name.startsWith(p)) && e.name != 'EXACT_NAME')`.

Always provide a `message` on every XValidation rule.

## kubeapilinter Nolint Directives

The repo uses `//nolint:kubeapilinter` comments for intentional deviations. Document the reason inline. Known patterns:
- `listMapKey` fields must NOT have `omitempty` (for proper patch merge identification). Apply `//nolint:kubeapilinter // <field> is a listMapKey and must not have omitempty`.
- Custom `Condition` type (on ExternalSecretsManager) that intentionally omits some `metav1.Condition` fields.
- `metav1.Duration` fields retained despite linter preference, annotated with `// Duration type retained to avoid breaking API change`.

## Status Subresource Patterns

### ExternalSecretsConfig Status
- Embeds `ConditionalStatus` (from `meta.go`) which holds `[]metav1.Condition` with standard merge markers (`+listType=map`, `+listMapKey=type`, `+patchMergeKey=type`, `+patchStrategy=merge`).
- Uses standard `metav1.Condition` with `ObservedGeneration` set to `esc.GetGeneration()`.
- Condition types: `Ready`, `Degraded`, and `UpdateAnnotation` (all defined as constants in `conditions.go`). The `UpdateAnnotation` condition is set by the `crd-annotator` controller on the `ExternalSecretsConfig` status.
- Reasons: `Failed`, `Ready`, `Progressing`, `Completed` (also constants, with `Progressing` defined as constant `ReasonInProgress`).
- Both conditions are set atomically before a single status update call.
- Additional status fields: `externalSecretsImage`, `bitwardenSDKServerImage`.

### ExternalSecretsManager Status
- Uses a custom `ControllerStatus` list (`+listType=map`, `+listMapKey=name`) tracking per-controller conditions.
- Each `ControllerStatus` contains a custom `[]Condition` type (Type, Status, Message -- no Reason or LastTransitionTime) and `ObservedGeneration`.
- Top-level `lastTransitionTime` is set on every condition change.

### Status Update Pattern
All controllers follow the same retry-on-conflict pattern:
1. Fetch current object from API server.
2. DeepCopy the changed status into the fetched object.
3. Call `StatusUpdate` (status subresource update).
4. Wrap in `retry.RetryOnConflict(retry.DefaultRetry, ...)`.

## Defaults

Use `+kubebuilder:default` for server-side defaulting:
- `logLevel`: 1
- `mode`: `Disabled`
- `injectAnnotations`: `"false"`
- `certificateCheckInterval`: `"5m"`
- `certificateDuration`: `"8760h"`
- `certificateRenewBefore`: `"30m"`
- `revisionHistoryLimit`: 10
- `networkPolicyProvisioning`: `Managed`
- `key` (ConfigMapKeyReference): `"ca-bundle.crt"`

## Shared Types and Composition

- `CommonConfigs` (in `meta.go`) is embedded via `json:",inline"` in both `ApplicationConfig` and `GlobalConfig`. It provides `logLevel`, `resources`, `affinity`, `tolerations`, `nodeSelector`, `proxy`.
- `ObjectReference`, `SecretReference`, `ConfigMapKeyReference`, `ConditionalStatus` are reusable building blocks in `meta.go`.
- Named types (`Mode`, `ManagementState`, `ComponentName`, `FeatureName`) with constants must be used instead of raw strings for enum fields.

## Code Generation Pipeline

After any API type change, run:
1. `make generate` -- regenerates `zz_generated.deepcopy.go` via controller-gen.
2. `make manifests` -- regenerates CRD YAMLs in `config/crd/bases/`, RBAC, and webhook configs.
3. `make update` -- full pipeline: generate + manifests + operand manifests + bindata + bundle + docs.
4. `make docs` -- regenerates `docs/api_reference.md` from `api/v1alpha1/` using `crd-ref-docs`.
5. `make verify` -- runs `check-git-diff` to ensure all generated files are committed.

Never edit `zz_generated.deepcopy.go` or CRD YAML files by hand.

## API Integration Tests

API validation is tested via declarative YAML test suites in `api/v1alpha1/tests/<crd-api-group-domain>/` (e.g., `externalsecretsconfig.operator.openshift.io/`, `externalsecretsmanager.operator.openshift.io/`). Each `.testsuite.yaml` file defines `onCreate` and `onUpdate` test cases specifying `initial`, `expected`, and `expectedError` YAML. The test framework (in `test/apis/`) installs CRDs into a real envtest API server (requires Kube >= 1.25 for CEL).

When adding a new CEL validation rule or field constraint, add corresponding test cases to the relevant `.testsuite.yaml` covering:
- Valid creation (initial + expected)
- Invalid creation (initial + expectedError with substring match)
- Immutability on update (initial + updated + expectedError)
- Boundary values (min/max lengths, min/max items)

Run API tests with `make test-apis` or via the general `make test`.

## Webhook Architecture

This operator does NOT use kubebuilder-generated admission webhooks for its own CRDs. CEL-based CRD validation handles all admission validation. The webhook code in `pkg/controller/external_secrets/` manages the upstream external-secrets operand's `ValidatingWebhookConfiguration` resources (not the operator's own API validation).

## Adding New API Fields Checklist

1. Add the Go field to the appropriate struct in `api/v1alpha1/`.
2. Add validation markers (min/max, enum, XValidation) for spec fields; status fields typically omit validation.
3. Use pointer types for optional sub-structs; value types for required scalars with defaults.
4. Add `+optional` or `+required` marker. Use `json:"fieldName,omitempty"` for optional fields.
5. Exception: listMapKey fields must NOT have `omitempty`; add `//nolint:kubeapilinter` with reason.
6. Write test cases in the corresponding `.testsuite.yaml` for both valid and invalid inputs.
7. Run `make update && make verify` to regenerate and validate all artifacts.
8. If the field adds a new condition type or reason, add constants in `conditions.go`.
