# E2E Suggestions: external-secrets-operator (EP-1898)

## Detected Operator Structure
- **Framework**: controller-runtime (operator-sdk v1+ / kubebuilder v4)
- **Managed CRDs**: ExternalSecretsConfig (cluster-scoped singleton), ExternalSecretsManager (cluster-scoped singleton)
- **E2E Pattern**: Ginkgo v2 with Gomega assertions, dynamic resource loader, embed testdata
- **Operator Namespace**: external-secrets-operator
- **Operand Namespace**: external-secrets

## Changes Detected in Diff (ai-staging-release-1.0...HEAD)

### API Type Changes
| File | Changes |
|------|---------|
| `api/v1alpha1/meta.go` | Added `KVPair` and `Annotation` types |
| `api/v1alpha1/external_secrets_config_types.go` | Added `ComponentConfig`, `DeploymentConfig` types; extended `ControllerConfig` with `annotations` and `componentConfigs` fields; extended `ComponentName` enum |
| `api/v1alpha1/zz_generated.deepcopy.go` | Auto-generated deepcopy for new types |

### Controller Changes
| File | Changes |
|------|---------|
| `pkg/controller/external_secrets/deployments.go` | Added `applyAnnotationsToDeployment`, `applyComponentConfig`, `mergeEnvVars` functions |
| `pkg/controller/external_secrets/utils.go` | Added helper functions for ComponentName ↔ deployment asset mapping |

### CRD Schema Changes
| File | Changes |
|------|---------|
| `config/crd/bases/operator.openshift.io_externalsecretsconfigs.yaml` | New schema fields for annotations, componentConfigs, deploymentConfig, overrideEnv with CEL validation |

### Integration Test Changes
| File | Changes |
|------|---------|
| `api/v1alpha1/tests/.../externalsecretsconfig.testsuite.yaml` | 30+ new CRD validation test cases |

## Highly Recommended E2E Scenarios

### 1. ✅ Custom Annotation Propagation (CRITICAL)
**Why**: Core new feature — must verify annotations actually appear on Deployments
**Coverage**: Verify annotations on Deployment metadata AND Pod template metadata for all operand components

### 2. ✅ Annotation Update Propagation
**Why**: Ensures the controller reacts to annotation changes and re-applies them
**Coverage**: Update annotations and verify changes are reflected on Deployments

### 3. ✅ Per-Component RevisionHistoryLimit
**Why**: Tests that the component config mapping correctly targets the right Deployment
**Coverage**: Set revisionHistoryLimit for one component, verify only that Deployment is affected

### 4. ✅ Per-Component Override Environment Variables
**Why**: Tests that custom env vars are merged into the correct containers
**Coverage**: Set overrideEnv and verify env vars appear in container spec

### 5. ✅ Combined Annotations + ComponentConfigs
**Why**: Verifies both features work together without interference
**Coverage**: Apply both annotations and componentConfigs, verify all are applied

### 6. ✅ Multiple Independent Component Configs
**Why**: Tests that different components get their own independent config
**Coverage**: Set different revisionHistoryLimit for CoreController vs Webhook

## Optional/Nice-to-Have Scenarios

### 7. 🔄 Annotation Removal
**Why**: Verify that removing annotations from the CR also removes them from Deployments
**Note**: May require clearing annotations vs removing the annotations field

### 8. 🔄 EnvVar Override Precedence
**Why**: Verify that user-specified env vars override defaults when names conflict
**Note**: Need to know the default env vars to test override behavior

### 9. 🔄 Network Policy with New ComponentNames
**Why**: EP-1898 adds Webhook and CertController to ComponentName enum for network policies
**Note**: Requires more complex setup with network policy verification

## Gaps / Hard to Test Automatically

1. **CRD Validation (CEL rules)**: The reserved prefix validation for annotations and env vars is tested at the CRD level in `.testsuite.yaml` integration tests. E2E tests would duplicate this coverage. Only include if CRD validation behavior needs to be verified on a live cluster.

2. **Race Conditions**: If annotations and componentConfigs are updated simultaneously by different clients, the reconciler should handle this gracefully. This is extremely hard to test in e2e.

3. **Upgrade Scenarios**: Testing that existing ExternalSecretsConfig CRs without the new fields continue to work after an operator upgrade. This requires a multi-version test setup.
