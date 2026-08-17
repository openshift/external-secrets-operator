# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records (ADRs) documenting significant architectural and design decisions made for the External Secrets Operator.

## What is an ADR?

An Architecture Decision Record captures an important architectural decision along with its context and consequences. ADRs help:
- New contributors understand why things are the way they are
- Prevent re-litigating settled decisions
- Document alternatives that were considered
- Explain trade-offs and consequences

## ADR Format

Each ADR follows this structure:
- **Status**: Accepted, Proposed, Deprecated, Superseded
- **Context**: The problem or situation requiring a decision
- **Decision**: What we decided to do
- **Rationale**: Why this decision was made (the "why" behind the "what")
- **Consequences**: Positive and negative outcomes, mitigations
- **Alternatives Considered**: Other options and why they were rejected
- **References**: Links to relevant code, docs, or external resources

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [001](001-bindata-over-helm.md) | Use Bindata Pattern Instead of Helm Charts | Accepted |
| [002](002-three-controller-design.md) | Three-Controller Architecture | Accepted |
| [003](003-cel-validation-only.md) | Use CEL Validation Instead of Admission Webhooks | Accepted |
| [004](004-singleton-cr-pattern.md) | Singleton Cluster-Scoped CRs Named "cluster" | Accepted |

## When to Write an ADR

Write an ADR when making decisions about:
- Architecture patterns (controller design, resource management)
- API design (CRD structure, validation approach)
- Technology choices (libraries, frameworks, tools)
- Security architecture (RBAC model, certificate management)
- Performance trade-offs (caching strategy, reconciliation patterns)
- Operational patterns (deployment model, upgrade strategy)

**Do NOT write an ADR for:**
- Implementation details that don't affect architecture
- Temporary workarounds or tactical fixes
- Decisions that are easily reversible
- Standard practices with no alternatives considered

## How to Propose a New ADR

1. Copy the template: `cp 000-template.md XXX-your-decision.md`
2. Fill in the sections (focus on "why" not just "what")
3. Get feedback from team/reviewers
4. Update status to "Accepted" when decision is made
5. Update this README.md index

## ADR Lifecycle

- **Proposed**: Decision under discussion
- **Accepted**: Decision has been made and is active
- **Deprecated**: Decision is no longer recommended but still in use
- **Superseded**: Decision has been replaced (link to superseding ADR)

## References

- ADR concept: https://adr.github.io/
- OpenShift operator patterns: https://docs.openshift.com/container-platform/latest/operators/
