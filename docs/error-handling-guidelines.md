# Error Handling Guidelines

## Custom Error Type: `ReconcileError`

Resource reconciliation errors flow through the `ReconcileError` type defined in `pkg/controller/common/errors.go`. It carries a `Reason` (`ErrorReason` string) that determines requeue behavior, a human-readable `Message`, and the underlying `Err`. Setup errors (finalizers, client construction, CR fetching) use plain `fmt.Errorf` wrapping as described below.

### Three Error Reasons

| Reason | Constant | Requeue? | When to use |
|---|---|---|---|
| `IrrecoverableError` | `common.IrrecoverableError` | No | Invalid config, missing env vars, permission errors, bad requests |
| `RetryRequiredError` | `common.RetryRequiredError` | Yes (30s) | Transient network issues, resource conflicts, timeouts, not-found |
| `UserConfigurationError` | `common.UserConfigurationError` | No (unless NotFound) | Invalid or incomplete user-provided configuration (e.g., missing referenced Secret or Issuer). Sets Degraded. Recovery is driven by watches on the affected resource; NotFound errors still use periodic requeue (30s) until the referenced object exists. |

### Constructor Functions

- `common.NewIrrecoverableError(err, messageFmt, args...)` -- for errors that cannot be fixed by retrying.
- `common.NewRetryRequiredError(err, messageFmt, args...)` -- for transient errors worth retrying.
- `common.NewUserConfigurationError(err, messageFmt, args...)` -- for errors caused by invalid or incomplete user configuration. The operator sets Degraded but generally does not requeue (relying on watches instead), except when the underlying error is NotFound.
- `common.FromClientError(err, messageFmt, args...)` -- auto-classifies Kubernetes API errors: `Unauthorized`, `Forbidden`, `Invalid`, and `BadRequest` become irrecoverable; everything else becomes retry-required.
- All four return `nil` when the input `err` is `nil`. Always check this at call sites when the constructor is the last expression before return.

### Checking Error Type

- `common.IsIrrecoverableError(err)` -- checks whether an error is irrecoverable. Uses `errors.As` to traverse wrapped error chains.
- `common.IsRetryRequiredError(err)` -- checks whether an error is retry-required.
- `common.IsUserConfigurationError(err)` -- checks whether an error is a user configuration error.
- `common.IsUserConfigurationNotFound(err)` -- checks whether a user configuration error is caused by a missing object (NotFound). Used to decide whether to requeue.

## Error-to-Reconcile-Result Mapping

The main reconcile dispatcher in `processReconcileRequest` (in `pkg/controller/external_secrets/controller.go`) maps errors to `ctrl.Result` as follows:

```
err == nil                    -> Result{}, nil                          (success, no requeue)
IsIrrecoverableError          -> Result{}, errUpdate                    (no requeue; only status update error propagates)
IsUserConfigurationError      -> Result{}, errUpdate                    (no requeue; Degraded=True; watches drive recovery)
  IsUserConfigurationNotFound -> Result{RequeueAfter: 30s}, nil         (requeue because watches won't fire for unmanaged objects)
RetryRequiredError            -> Result{RequeueAfter: 30s}, nil         (manual requeue, no error to controller-runtime)
status update failure         -> Result{}, errUpdate                    (let controller-runtime handle backoff)
NotFound on primary CR        -> Result{}, nil                          (skip reconciliation silently)
```

Key rule: never return both `RequeueAfter` and a non-nil error. For recoverable errors, return `RequeueAfter` with `nil` error. For irrecoverable and user-configuration errors, return empty `Result` so no requeue happens (except user-configuration NotFound, which requeues).

The default requeue interval is `common.DefaultRequeueTime` (30 seconds), defined in `pkg/controller/common/constants.go`.

## Wrapping Errors from Kubernetes API Calls

For any Kubernetes client operation (Get, Create, Update, Delete, Patch, Exists), wrap the error with `common.FromClientError`:

```go
if err := r.Create(r.ctx, obj); err != nil {
    return common.FromClientError(err, "failed to create %s deployment resource", name)
}
```

This is the standard pattern across all resource reconciliation files (rbacs.go, deployments.go, services.go, secret.go, certificate.go, configmap.go, networkpolicy.go, validatingwebhook.go, serviceaccounts.go). `FromClientError` auto-classifies the API error so callers do not manually pick irrecoverable vs retryable.

For non-API errors that are definitively unrecoverable (e.g., missing environment variable, invalid configuration, failed validation), use `common.NewIrrecoverableError` directly:

```go
return common.NewIrrecoverableError(
    fmt.Errorf("ENV_VAR not set"),
    "failed to update image in %s deployment object", name,
)
```

## `fmt.Errorf` with `%w` for Non-Reconcile Errors

For errors outside the reconcile-result classification path (setup, client construction, utility functions), use standard `fmt.Errorf` with `%w` wrapping:

```go
return fmt.Errorf("failed to create uncached client: %w", err)
```

Do not wrap these in `ReconcileError` -- they propagate as plain errors up through controller setup or `Reconcile` return.

## Retry Logic

### `UpdateWithRetry` (Client-Level Retry)

All resource updates that may hit conflicts use `r.UpdateWithRetry(ctx, obj)` instead of `r.Update(ctx, obj)`. This method (in `pkg/controller/client/client.go`) wraps `retry.RetryOnConflict(retry.DefaultRetry, ...)`, re-fetching the latest resource version before each attempt.

Use `UpdateWithRetry` for updating managed resources (Deployments, RBAC, Services, etc.) and for finalizer updates.

### `retry.RetryOnConflict` (Status Updates)

Status subresource updates use `retry.RetryOnConflict(retry.DefaultRetry, ...)` directly. The pattern is: fetch latest, deep-copy desired status into it, then call `StatusUpdate`. This appears identically in all three controllers.

### `retry.OnError` with Custom Predicate

The ESM default resource creation (`pkg/controller/external_secrets_manager/externalsecretsmanager.go`) uses `retry.OnError` with a custom `shouldRetryOnError` predicate that stops retrying on `AlreadyExists`, `Conflict`, `Invalid`, `BadRequest`, `Unauthorized`, `Forbidden`, and `TooManyRequests`.

## Status Condition Updates

### Condition Types

| Type | Defined In | Used By |
|---|---|---|
| `Degraded` | `api/v1alpha1/conditions.go` | external_secrets controller |
| `Ready` | `api/v1alpha1/conditions.go` | external_secrets controller |
| `UpdateAnnotation` | `api/v1alpha1/conditions.go` | crd_annotator controller |

### Condition Update Rules

1. On irrecoverable error: set `Degraded=True/Failed` and `Ready=False/Failed`.
2. On user configuration error: set `Degraded=True/Failed` and `Ready=False/Failed` with message `"user configuration is invalid: ..."`.
3. On retryable error: set `Degraded=False/Ready` and `Ready=False/Progressing` with the error message.
4. On success: set `Degraded=False/Ready` and `Ready=True/Ready`.
5. Set both conditions atomically via `apimeta.SetStatusCondition` before calling `updateStatus`.
6. Only call `updateStatus` when `SetStatusCondition` returns `true` (condition actually changed).
7. Always include `ObservedGeneration` from the CR's current `.metadata.generation`.

### Error Aggregation on Status Update Failure

When the status update itself fails alongside a reconciliation error, aggregate both using `utilerrors.NewAggregate([]error{err, errUpdate})`. This pattern exists in both the external_secrets and crd_annotator controllers.

## Kubernetes Event Recording

- Use `corev1.EventTypeNormal` + reason `"Reconciled"` for successful create/update of resources.
- Use `corev1.EventTypeWarning` + reason `"ResourceAlreadyExists"` when a resource exists during initial creation reconciliation.
- Use `corev1.EventTypeWarning` + reason `"RemoveDeployment"` on CR deletion.
- Use `corev1.EventTypeWarning` + reason `"Read"` on fetch failures (ESM controller).
- Event messages must include the resource name (namespace/name format).

## Logging Conventions

- `r.log.V(1).Info(...)` -- operational events (reconcile start, resource modifications, label/annotation changes).
- `r.log.V(4).Info(...)` -- detailed debug (resource already in expected state, status update attempts, metadata builds).
- `r.log.Error(err, "message")` -- always log the error at the point it occurs in `reconcileExternalSecretsDeployment`, before returning it upward. The top-level `processReconcileRequest` also logs the error again with `r.log.Error(err, "failed to reconcile external-secrets deployment")`.
- `ctrl.Log.V(1).WithName("subsystem")` -- for setup-time logging outside reconciler instances (cache setup, CRD discovery).
- Use structured key-value pairs (`"name"`, `"request"`, `"key"`, `"namespace"`, `"error"`, `"installed"`), never string interpolation in log messages.

## `Exists` Helper

The `CtrlClient.Exists(ctx, key, obj)` method converts `NotFound` errors to `(false, nil)`, passing all other errors through. Use this instead of raw `Get` + manual `IsNotFound` checks when you only need existence.

## Panics (Decode Helpers Only)

The `Decode*ObjBytes` functions in `pkg/controller/common/utils.go` panic on decode failure or type assertion failure. These are called only with compile-time-known static assets (`assets.MustAsset`), so panics indicate a build-time bug, not a runtime error. Never use panic for runtime error handling elsewhere.

## NotFound Handling in Reconcile

When the primary CR (`ExternalSecretsConfig` or `ExternalSecretsManager`) is not found, return `ctrl.Result{}, nil` -- do not requeue. Log at V(1) and skip reconciliation. When a secondary/dependent CR is not found, either skip gracefully (ESM looking for ESC) or wrap in `fmt.Errorf` and return for requeue.
