# Stage evals (quality gates)

Operator-owned eval rubrics used by `/opsx-continue` and `/opsx-apply`.

| File | Used by |
|------|---------|
| `repo-assessment_eval.yaml` | `/opsx-continue` (repo-assessment) |
| `plan_eval.yaml` | `/opsx-continue` (plan) |
| `tasks_eval.yaml` | `/opsx-continue` (tasks) |
| `code-generation_eval.yaml` | `/opsx-apply` (per-task, ai-helpers mode) |

- Empty `evals: []` would mean the stage gate is present but has no cases yet.
- Current stage/code-generation files are populated; `/eval-loop` accumulates further cases from completed features into this directory.
- Do not edit schema package evals for forward workflow; this directory is the source of truth.

See: https://github.com/sujkini/openspec/blob/v2-restructured/README.md
