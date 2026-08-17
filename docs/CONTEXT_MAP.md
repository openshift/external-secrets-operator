# Context Documentation Map

This document maps all documentation in this repository, organized by audience and purpose.

## For Track 1: Contextualization

This repository implements **prescriptive context registration** following the Track 1 approach:

- **Prescriptive context** (official organizational direction): architecture, decisions, guidelines
- **Process context**: contributing workflows

When the Cyborg schema supports context registration (Track 1 Phase 2), this repo can register:
- **Entry point**: AGENTS.md
- **Architecture**: ARCHITECTURE.md + docs/decisions/
- **Domain guidelines**: docs/*-guidelines.md
- **Terminology**: GLOSSARY.md
- **Process**: CONTRIBUTING.md

---

## Entry Points by Audience

### 🤖 For AI Agents

**Start here**: [AGENTS.md](../AGENTS.md)

AGENTS.md provides:
- Quick index to domain-specific guidelines
- Project structure and build system
- Architectural patterns
- Common pitfalls

**Then read**:
- [GLOSSARY.md](../GLOSSARY.md) - Terminology definitions
- [docs/decisions/](decisions/) - Architecture Decision Records (why decisions were made)
- Domain-specific guidelines linked from AGENTS.md

### 👨‍💻 For New Contributors

**Start here**: [CONTRIBUTING.md](../CONTRIBUTING.md)

**Then read**:
- [AGENTS.md](../AGENTS.md) - Architecture and patterns
- [GLOSSARY.md](../GLOSSARY.md) - Terminology
- [docs/testing-guidelines.md](testing-guidelines.md) - Testing framework details

**Before first PR**:
- Run `make lint && make test && make update && make verify`

### 🏗️ For Architects & Tech Leads

**Start here**: [ARCHITECTURE.md](../ARCHITECTURE.md)

**Then read**:
- [docs/decisions/](decisions/) - ADRs explaining "why"
- Domain guidelines:
  - [api-contracts-guidelines.md](api-contracts-guidelines.md) - CRD design
  - [performance-guidelines.md](performance-guidelines.md) - Cache, reconciliation patterns
  - [security-guidelines.md](security-guidelines.md) - RBAC, network policies, hardening

---

## Documentation Hierarchy

### Organizational Level (Future - Track 1 Phase 2)

When registered in Cyborg, this repo will inherit context from:
- **Hybrid Platforms org-level** - Development standards, escalation procedures
- **Area/Group/Pillar-level** - Operational context

*Currently: Not yet registered (waiting for Cyborg schema changes)*

### Repo-Level Documentation

#### Entry Points

| Document | Audience | Purpose |
|---|---|---|
| [AGENTS.md](../AGENTS.md) | AI agents, developers | Technical entry point with architecture, patterns |
| [README.md](../README.md) | All | General overview |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | Contributors | PR process and coding conventions |
| [ARCHITECTURE.md](../ARCHITECTURE.md) | Architects | High-level system design |

#### Reference

| Document | Purpose |
|---|---|
| [GLOSSARY.md](../GLOSSARY.md) | Domain-specific terminology |
| [CLAUDE.md](../CLAUDE.md) | Claude Code quick reference |

### Domain-Specific Technical Docs

#### Guidelines (Prescriptive)

Located in `docs/`:

| Document | Domain |
|---|---|
| [api-contracts-guidelines.md](api-contracts-guidelines.md) | CRD types, kubebuilder markers, CEL validation |
| [error-handling-guidelines.md](error-handling-guidelines.md) | ReconcileError types, retry logic, status conditions |
| [integration-guidelines.md](integration-guidelines.md) | Bindata pattern, cert-manager, OpenShift platform |
| [performance-guidelines.md](performance-guidelines.md) | Cache architecture, watch predicates, reconciliation |
| [security-guidelines.md](security-guidelines.md) | Container security, RBAC, TLS, network policies |
| [testing-guidelines.md](testing-guidelines.md) | Unit/API/E2E test tiers, frameworks |

#### Architecture Decision Records

Located in `docs/decisions/`:

| ADR | Decision |
|---|---|
| [001-bindata-over-helm.md](decisions/001-bindata-over-helm.md) | Why static YAML + bindata instead of Helm |
| [002-three-controller-design.md](decisions/002-three-controller-design.md) | Why three controllers instead of monolith |
| [003-cel-validation-only.md](decisions/003-cel-validation-only.md) | Why CEL validation instead of webhooks |
| [004-singleton-cr-pattern.md](decisions/004-singleton-cr-pattern.md) | Why singleton CRs named "cluster" |
| [README.md](decisions/README.md) | ADR index |

### Generated Documentation

| Document | Generated From | Purpose |
|---|---|---|
| [api_reference.md](api_reference.md) | `api/v1alpha1/` types | API field documentation |
| `config/crd/bases/*.yaml` | Kubebuilder markers | CRD manifests |
| `config/rbac/role.yaml` | `+kubebuilder:rbac` markers | Operator RBAC |

**Never edit generated files manually** - use `make update`.

---

## Documentation Type Classification

For **Track 1 context registration**:

### Prescriptive Context (Authoritative)

Official decisions and architectural direction:
- AGENTS.md (patterns)
- ARCHITECTURE.md (design)
- docs/*-guidelines.md (how to implement correctly)
- docs/decisions/ (ADRs - why decisions were made)
- CONTRIBUTING.md (process requirements)
- api_reference.md (API contracts)

### Meta-Documentation

Navigation and terminology:
- GLOSSARY.md (defines terms)
- CONTEXT_MAP.md (this file - navigation)
- docs/decisions/README.md (ADR index)

---

## Finding Information

### "How do I...?"

| Question | Start Here |
|---|---|
| Understand the architecture? | [ARCHITECTURE.md](../ARCHITECTURE.md) → [AGENTS.md](../AGENTS.md) |
| Add a new CRD field? | [api-contracts-guidelines.md](api-contracts-guidelines.md) |
| Write tests? | [testing-guidelines.md](testing-guidelines.md) |
| Understand error handling? | [error-handling-guidelines.md](error-handling-guidelines.md) |
| Configure RBAC? | [security-guidelines.md](security-guidelines.md) |

### "Why is it designed this way?"

All "why" questions → [docs/decisions/](decisions/)

| Topic | ADR |
|---|---|
| Why bindata instead of Helm? | [ADR-001](decisions/001-bindata-over-helm.md) |
| Why three controllers? | [ADR-002](decisions/002-three-controller-design.md) |
| Why CEL validation? | [ADR-003](decisions/003-cel-validation-only.md) |
| Why singleton CRs? | [ADR-004](decisions/004-singleton-cr-pattern.md) |

### "What does X mean?"

[GLOSSARY.md](../GLOSSARY.md) - Comprehensive terminology reference

---

## Cross-References

### Most Referenced Docs

1. **AGENTS.md** - Referenced by README, CONTRIBUTING, CLAUDE.md, all guidelines
2. **ARCHITECTURE.MD** - Referenced by AGENTS, ADRs, guidelines
3. **GLOSSARY.md** - Referenced by technical docs for terminology
4. **CONTRIBUTING.md** - Referenced by README, AGENTS

### Documentation Dependencies

```
README.md
  ├── AGENTS.md (AI entry point)
  ├── CONTRIBUTING.md (process)
  └── ARCHITECTURE.md (design)

AGENTS.md
  ├── docs/*-guidelines.md (domain docs)
  ├── GLOSSARY.md (terminology)
  └── ARCHITECTURE.md (system design)

ARCHITECTURE.md
  └── docs/decisions/ (why decisions)

CONTRIBUTING.md
  ├── AGENTS.md (architecture context)
  └── docs/testing-guidelines.md (testing)
```

---

## Version History

**Track 1 Contextualization** (PR #150 + enhancements):
- **PR #150**: AGENTS.md, ARCHITECTURE.md, CONTRIBUTING.md, 6 guideline docs
- **Enhancements**: ADRs, GLOSSARY, CONTEXT_MAP

---

## Related: Track 1 Contextualization

This repository implements **prescriptive context registration** as described in the Agentic SDLC Strategy Track 1.

**What we've done**:
- ✅ Written prescriptive context (ADRs, guidelines, architecture)
- ✅ Centralized context in structured format
- ✅ Provided entry points for AI agents and developers
- ✅ Defined terminology via GLOSSARY

**What's next (Track 1 Phase 2)**:
- ⏳ Cyborg schema extension for context registration
- ⏳ Register this repo's context (AGENTS.md, decisions/, guidelines/)
- ⏳ Service account access for AI agents

**Context registry example**:
```yaml
# Future Cyborg YAML
team: external-secrets-operator
context:
  entry_point: github.com/.../AGENTS.md
  architecture: github.com/.../ARCHITECTURE.md
  decisions: github.com/.../docs/decisions/
  domain_guidelines:
    - github.com/.../docs/api-contracts-guidelines.md
    - github.com/.../docs/error-handling-guidelines.md
    # ... etc
  terminology: github.com/.../GLOSSARY.md
  process: github.com/.../CONTRIBUTING.md
```

This documentation is already usable by AI tools - registration makes it **discoverable** across the org.
