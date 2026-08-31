---
spec: docs/specs/agents/requirements/objective-assessment.md
created: 2026-08-31
status: draft
---

# Implementation Plan: Task Objective Assessment

## Overview

Objective assessment is a second **run kind** on the existing Native Code Review
runtime. The shared `task_review_runs` table gains a `kind` discriminator and a
`verdict` column; a new `task_objective_criteria` table holds the checklist. The
`internal/review` `Runner` gains a `Kind` branch that, instead of per-file
finding batches, builds an objective/criteria context, runs one inference call,
parses an assessment JSON, and completes the run with a rollup verdict. The three
triggers (WS action, `run_objective_check` workflow action, MCP tool) and the
frontend Review-surface panel mirror their Native Code Review equivalents. The
optional workflow gate reuses `workflow_step_decisions` + `WaitForQuorumGuard` —
no new engine guard type.

Native Code Review (`docs/plans/native-code-review/`) is the end-to-end reference
for every layer; each work order names the specific sibling file to copy.

Dependency order: schema/models → service write path + events → objective
extraction + parser → runner branch → triggers (MCP + WS, workflow) → frontend →
mobile + E2E + docs.

Four early design risks were resolved toward less code, not more:

- **Gate.** Reuse `workflow_step_decisions` + `WaitForQuorumGuard` with a reserved
  role constant `objective-check` (excluded from the human role picker). No new
  engine guard type. Re-run upserts the decision. One isolation test.
- **Thin objective source.** Accepted for v1. The run control is hidden/disabled
  when the task has no description and no `plan` / `spec` doc; a gated workflow
  step in that state does **not** block (logs a warning). No spec-linkage table.
- **Shared-table migration.** `kind` defaults to `code_review`; only unfiltered
  run-list reads gain `AND kind = 'code_review'`. A mixed-kind repository test is
  the audit.
- **Verdict.** The agent's stated verdict is ignored; `ReviewService` computes it
  from criterion statuses (all `met` → `met`; some `met` + some not → `partial`;
  no `met` → `unmet`). No `inconclusive` state — an assessment the agent cannot
  produce is a failed run.

## Waves

| Wave | Work orders | Parallel-safe |
| --- | --- | --- |
| 1 | 01 | — |
| 2 | 02, 03 | 02 and 03 after 01 |
| 3 | 04 | after 02, 03 |
| 4 | 05, 06 | both after 04 |
| 5 | 07, 08 | 08 after 07 |
| 6 | 09, 10 | 09 and 10 after 08 |

## Work orders

| ID | Title | Wave | Depends on |
| --- | --- | --- | --- |
| 01 | Schema: `kind`/`verdict` columns + `task_objective_criteria` + repository | 1 | — |
| 02 | `ReviewService.PublishAssessment` / `GetTaskAssessment` + bus/WS events | 2 | 01 |
| 03 | Objective + criteria extraction and assessment-response parser | 2 | 01 |
| 04 | Runner `objective_check` branch + `objective-check` utility agent | 3 | 02, 03 |
| 05 | `publish_objective_assessment_kandev` MCP tool + `task.objective.*` WS actions | 4 | 04 |
| 06 | `run_objective_check` workflow action + gate decision writer | 4 | 04 |
| 07 | Frontend types, API client, store slice, WS handlers | 5 | 05 |
| 08 | Objective panel: verdict banner, criterion checklist, send-to-agent | 5 | 07 |
| 09 | Workflow step editor control + gate toggle | 6 | 06, 08 |
| 10 | Mobile parity, E2E, mock-agent scenario, public docs | 6 | 08 |

## Cross-cutting verification

```
cd apps/backend && go test ./internal/task/... ./internal/review/... ./internal/workflow/... ./internal/mcp/...
cd apps/backend && golangci-lint run ./... --new-from-rev="<base-sha>" --timeout=5m
cd apps/web && pnpm run typecheck && pnpm --filter @kandev/web lint
cd apps/web && pnpm e2e:raw -- objective
python3 scripts/lint-spec-files.py --all
```

## Residual risks

- **Gate role reuse.** The `objective-check` role must be a shared constant used
  by the writer (WO-06), the guard config (WO-09), and the human-role exclusion
  list. WO-06 owns the isolation test and the re-run-upsert test.
- **Objective source is thin.** Only `task.Description` + `plan` / `spec`
  documents. Documented as a v1 limitation in public docs (WO-10); a structured
  spec link is a separate future capability.
- **Verdict rule.** Fixed in the service, agent verdict ignored. WO-02 and WO-03
  tests must pin every criterion-status combination.
- **Shared-table reads.** WO-01's mixed-kind repository test is the audit; treat a
  failure there as a missed query, not a flaky test.
