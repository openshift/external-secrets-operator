# Anti-Patterns Documentation

This directory documents anti-patterns found in the external-secrets-operator codebase, their root causes, impacts, and solutions.

## Purpose

Documenting anti-patterns helps:
- **Prevent reintroduction** of problematic patterns
- **Share knowledge** across the team
- **Guide future development** with lessons learned
- **Reference solutions** from similar projects

## Current Anti-Patterns

### 1. Dual Cache Race Condition ✅ FIXED

**Documents:**
- [`DUAL_CACHE_FIX.md`](./DUAL_CACHE_FIX.md) - Detailed technical explanation
- [`DUAL_CACHE_FIX_SUMMARY.md`](./DUAL_CACHE_FIX_SUMMARY.md) - Quick reference

**Problem:** Controller used two separate caches (manager's cache for watches, custom cache for reads), creating:
- Race conditions during startup
- 2× memory usage
- 2× API server connections
- Potential "object not found" errors

**Solution:** Unified cache approach - configure manager's cache with label selectors via `NewCacheBuilder()`.

**Inspiration:** [cert-manager-operator PR #324](https://github.com/openshift/cert-manager-operator/pull/324)

**Status:** Fixed in commit [ref]

---

## Potential Future Anti-Patterns

Based on common Kubernetes operator issues, watch for:

### Controller Patterns
- [ ] **Unbounded Reconciliation** - Missing rate limiting or exponential backoff
- [ ] **Status Update Loops** - Status updates triggering unnecessary reconciliations
- [ ] **Missing Finalizers** - Resources not properly cleaned up on deletion
- [ ] **Blocking Reconciliation** - Long-running operations without context timeouts

### Cache & Client Patterns
- [ ] **Cache Stampede** - All controllers resyncing simultaneously
- [ ] **Over-caching** - Watching resources not actually used
- [ ] **Direct API Calls** - Bypassing cache unnecessarily (use `UncachedClient` intentionally)
- [ ] **Stale Reads** - Not handling cache sync properly

### Resource Management
- [ ] **Resource Leaks** - Not cleaning up created resources
- [ ] **Owner Reference Missing** - Manual cleanup instead of garbage collection
- [ ] **Unbounded Resource Creation** - No limits on child resources

### Error Handling
- [ ] **Silent Failures** - Errors not surfaced to status or events
- [ ] **Panic in Reconciliation** - Unhandled panics crashing controller
- [ ] **Error Shadowing** - Generic errors hiding root cause

## How to Document a New Anti-Pattern

When you discover an anti-pattern:

1. **Create a new document** following this structure:
   ```
   # Anti-Pattern: [Name]

   ## Problem
   - What is the issue?
   - Why is it problematic?
   - What are the symptoms?

   ## Root Cause
   - Why does this happen?
   - What makes it easy to introduce?

   ## Impact
   - Performance impact
   - Reliability impact
   - Resource usage impact

   ## Solution
   - How to fix it
   - Code examples
   - References to similar fixes

   ## Prevention
   - How to avoid reintroducing
   - Linting rules
   - Code review checklist
   ```

2. **Update this README** with a link to the new document

3. **Tag the commit** that fixes it for easy reference

## References

### Best Practices
- [Controller-Runtime Best Practices](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
- [Kubernetes Operator Patterns](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Kubebuilder Book](https://book.kubebuilder.io/)

## Contributing

Found an anti-pattern? Document it here! Include:
- Clear problem statement
- Reproducible example (if possible)
- Impact assessment
- Proposed solution with references
- Prevention strategy

---

**Maintained by:** External Secrets Operator Team  
**Last Updated:** 2025-10-19

