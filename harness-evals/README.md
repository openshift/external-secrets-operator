# Harness evals (operator-owned)

Operator-specific inputs for the [OpenSpec agile workflow](https://github.com/sujkini/openspec/blob/v2-restructured/README.md).

```text
harness-evals/
├── harness-docs/       # Operator docs (source for /opsx-constitute)
├── constitution.md     # Guardrails (required before plan.md)
└── evals/              # Stage eval YAMLs (quality gates for /opsx-continue and /opsx-apply)
```

| Path | Command | Notes |
|------|---------|-------|
| `harness-docs/` | `/opsx-constitute` | Already populated with ESO guidelines |
| `constitution.md` | `/opsx-continue` (before plan) | Present — regenerate with `/opsx-constitute` if needed |
| `evals/*_eval.yaml` | `/opsx-continue`, `/opsx-apply` | Populated stage/code-generation cases; `/eval-loop` accumulates more |

## Accumulate more eval cases

1. Fill `eval-generation/input/feature-bundle.yaml` from a **completed** feature (EP, epic, stories, PRs, bugs).
2. Run `/eval-loop`.
3. Review `eval-generation/eval-generation-workflow/template-gaps/` and `eval-generation/output-refined-templates/`.
4. Generated cases sync automatically into `harness-evals/evals/`.
