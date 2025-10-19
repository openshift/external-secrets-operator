# Fix: Dual Cache Race Condition

## Summary

Fixed the dual cache synchronization issue reported in [ESO-203](https://issues.redhat.com/browse/ESO-203) by adopting a **unified cache approach** similar to the solution implemented in [cert-manager-operator PR #324](https://github.com/openshift/cert-manager-operator/pull/324) for the istio-csr controller race condition ([CM-735](https://issues.redhat.com/browse/CM-735)).

## Problem

The external-secrets-operator controller was using **two separate caches**:
1. **Manager's default cache** - For watching resources and triggering reconciliation
2. **Custom cache** - For reading objects during reconciliation

This created a race condition:
```
OLD (Race Condition):
Manager cache syncs → triggers reconcile → reads from different custom cache → might not be synced yet
```

### Consequences
- Potential "object not found" errors despite object existing
- Race condition during startup
- Unnecessary memory and network overhead (both caches watching same resources)
- Complexity in understanding cache synchronization

## Solution

Replaced the dual-cache pattern with a **single unified cache**:

```
NEW (No Race):
Manager cache syncs → triggers reconcile → reads from SAME manager cache → guaranteed synced
```

### Changes Made

####1. Configure Manager Cache with Label Selectors

**File:** `cmd/external-secrets-operator/main.go`

Added `NewCache` option to manager configuration:
```go
mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
    // ... existing options ...
    // Configure manager's cache with custom label selectors
    // This replaces the need for a separate custom cache
    NewCache: escontroller.NewCacheBuilder(),
})
```

#### 2. Implement Cache Builder

**File:** `pkg/controller/external_secrets/controller.go`

Created `NewCacheBuilder()` function that configures the manager's cache with the same label selectors previously used in the custom cache:

```go
func NewCacheBuilder() cache.NewCacheFunc {
    return func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
        // Build the object list with label selectors
        objectList := buildCacheObjectList()
        
        // Configure cache options with our label-filtered resources
        opts.ByObject = objectList
        
        // Create and return the cache using the standard cache constructor
        return cache.New(config, opts)
    }
}
```

#### 3. Simplified Client Creation

**Before:**
```go
func NewClient(m manager.Manager, r *Reconciler) (operatorclient.CtrlClient, error) {
    c, err := BuildCustomClient(m, r)  // Created separate custom cache
    if err != nil {
        return nil, err
    }
    return &operatorclient.CtrlClientImpl{Client: c}, nil
}
```

**After:**
```go
func NewClient(m manager.Manager, r *Reconciler) (operatorclient.CtrlClient, error) {
    // Use the manager's client directly - it reads from the manager's cache
    // which is now configured with the same label selectors
    return &operatorclient.CtrlClientImpl{
        Client: m.GetClient(),
    }, nil
}
```

#### 4. Dynamic Certificate Registration

Added `checkAndRegisterCertificates()` to register Certificate informers if cert-manager is installed:

```go
func checkAndRegisterCertificates(mgr ctrl.Manager, r *Reconciler) (bool, error) {
    exist, err := isCRDInstalled(mgr.GetConfig(), certificateCRDName, certificateCRDGroupVersion)
    if err != nil {
        return false, err
    }
    
    if exist {
        r.optionalResourcesList[certificateCRDGKV] = struct{}{}
        // Register Certificate informer with manager's cache
        _, err = mgr.GetCache().GetInformer(context.Background(), &certmanagerv1.Certificate{})
        if err != nil {
            return false, err
        }
    }
    
    return exist, nil
}
```

#### 5. Removed Deprecated Code

Deleted the entire `BuildCustomClient()` function and its associated logic (~100 lines).

## Benefits

### 1. Eliminates Race Condition ✅
- Controller-runtime guarantees cache sync before reconciliation starts
- No more potential for reading from an unsynced cache
- Deterministic behavior

### 2. Reduces Resource Usage ✅
- **Before:** 2 caches × N resources = 2N watch connections + 2N cached objects
- **After:** 1 cache × N resources = N watch connections + N cached objects
- **Memory saved:** ~50%
- **Network traffic saved:** ~50%

### 3. Simplifies Code ✅
- Removed ~100 lines of custom cache management code
- Clearer control flow: one cache, one source of truth
- Easier to understand and maintain

### 4. Follows Best Practices ✅
- Uses standard controller-runtime pattern
- Same solution as cert-manager-operator ([PR #324](https://github.com/openshift/cert-manager-operator/pull/324))
- Leverages controller-runtime's built-in cache synchronization guarantees

## Architecture Comparison

### Before (Dual Cache)
```
┌──────────────────────────────────────────┐
│          Kubernetes API Server            │
└────────────┬─────────────────┬───────────┘
             │                 │
             │ Watch           │ Watch
             ▼                 ▼
   ┌─────────────────┐  ┌─────────────────┐
   │ MANAGER CACHE   │  │  CUSTOM CACHE   │
   │                 │  │                 │
   │ - Watches all   │  │ - Label filter  │
   │ - Triggers recon│  │ - For reads     │
   └────────┬────────┘  └────────┬────────┘
            │                    │
            │ Event              │ Get/List
            ▼                    ▼
   ┌───────────────────────────────────────┐
   │         Controller/Reconciler          │
   │                                        │
   │  ⚠️  RACE: Custom cache might not be  │
   │      synced when reconciliation starts │
   └────────────────────────────────────────┘
```

### After (Unified Cache)
```
┌──────────────────────────────────────────┐
│          Kubernetes API Server            │
└────────────┬─────────────────────────────┘
             │
             │ Watch
             ▼
   ┌─────────────────────────────────────┐
   │      UNIFIED MANAGER CACHE          │
   │                                     │
   │ - Label-filtered resources          │
   │ - Both watches AND reads            │
   │ - Guaranteed synced before recon    │
   └────────────┬────────────────────────┘
                │
                │ Both events AND reads
                ▼
   ┌────────────────────────────────────┐
   │      Controller/Reconciler          │
   │                                     │
   │  ✅  NO RACE: Same cache for all   │
   └─────────────────────────────────────┘
```

## Testing

### Verification Steps

1. **Build the operator:**
   ```bash
   make build
   ```

2. **Deploy and observe:**
   ```bash
   # Check cache initialization logs
   kubectl logs -n external-secrets-operator deployment/external-secrets-operator-controller-manager | grep "cache-setup"
   
   # Verify no race conditions
   kubectl logs -n external-secrets-operator deployment/external-secrets-operator-controller-manager | grep -i "not found"
   ```

3. **Test reconciliation:**
   ```bash
   # Create/update ExternalSecretsConfig
   kubectl apply -f config/samples/operator_v1alpha1_externalsecretsconfig.yaml
   
   # Should reconcile immediately without errors
   kubectl get externalsecretsconfig cluster -o yaml
   ```

### Expected Behavior

✅ No "object not found" errors  
✅ Immediate reconciliation after resource creation  
✅ Consistent behavior across restarts  
✅ Reduced memory footprint  

## Migration Notes

### Breaking Changes
**None.** This is an internal implementation change with no API changes.

### Rollback
If issues occur, revert this commit to restore the dual-cache implementation.

## References

- **Inspiration:** [cert-manager-operator PR #324](https://github.com/openshift/cert-manager-operator/pull/324)
- **Related Issue:** [CM-735 - IstioCSR race condition](https://issues.redhat.com/browse/CM-735)
- **Controller-Runtime Cache Docs:** https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/cache
- **Original Discussion:** [Gist with AI dialogue](https://gist.github.com/lunarwhite/8928d1dc8e35d0d23e6cc7a364985215)

## Credits

Solution inspired by the fix implemented in cert-manager-operator by the OpenShift cert-manager team for a similar race condition in the istio-csr controller.

