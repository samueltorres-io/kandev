---
id: "01-schema-and-repository"
title: "Shared run kind/verdict + task_objective_criteria table + repository"
status: pending
wave: 1
depends_on: []
plan: "plan.md"
requirements:
  - REQ-AGENTS-OBJECTIVE-ASSESSMENT-001
acceptance_criteria:
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.3
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.10
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.11
system_design: "../../specs/agents/system-design/objective-assessment.md"
---

# Task 01: Shared run kind/verdict + criteria table + repository

## Summary

Add the `objective_check` run kind to the shared review-run table and a new
`task_objective_criteria` table, with repository methods and models.

## Scope

- `internal/task/models/models.go` — `ReviewRunKind` (`code_review` | `objective_check`, default `code_review`), `ObjectiveVerdict` (`met|partial|unmet`), `ObjectiveCriterionStatus` (`met|partial|unmet|unknown`), `ObjectiveCriterionSource` (`derived|document`). Structs `TaskObjectiveCriterion` and its `EvidencePointer`. Add `Kind` and `Verdict` fields to `TaskReviewRun`. Sentinel `ErrTaskObjectiveCriterionNotFound`.
- `internal/task/repository/sqlite/base_migrations.go` — idempotent `ADD COLUMN kind TEXT NOT NULL DEFAULT 'code_review'` and `ADD COLUMN verdict TEXT NOT NULL DEFAULT ''` on `task_review_runs`; then `task_objective_criteria` DDL is created in schema-init and its indexes `(task_id, run_id)`, `(run_id)` added here after the table.
- `internal/task/repository/sqlite/base_schema.go` — `task_objective_criteria` `CREATE TABLE IF NOT EXISTS` per the design's Data model.
- `internal/task/repository/sqlite/task_review.go` — set `kind`/`verdict` in `CreateTaskReviewRun` / `UpdateTaskReviewRun`; new `CreateTaskObjectiveCriteria(ctx, []*TaskObjectiveCriterion)` (single tx), `ListTaskObjectiveCriteria(taskID, runID)`, `GetTaskObjectiveCriterion`, `DeleteTaskObjectiveCriteriaByRun`. Extend `DeleteTaskReviewByTask` / `DeleteTaskReviewByWorkspace` / `CancelInFlightTaskReviewRuns` to cover the new table and stay kind-agnostic.
- Add `AND kind = 'code_review'` to the unfiltered `SELECT ... FROM task_review_runs WHERE task_id = ?` reads only (run list, active-run lookup, history). Objective reads are separate methods filtering `kind = 'objective_check'`. Do not touch finding queries.

## Exclusions

Service write path, events, runner, triggers, frontend — later work orders.

## Implementation acceptance conditions

- A fresh DB and a replayed existing DB both end with the two new columns and the new table; existing Native Code Review rows read back with `kind = 'code_review'`.
- `CreateTaskObjectiveCriteria` writes N rows in one transaction; deleting the run cascades them; deleting the task cascades them.
- **Migration audit test:** insert a mixed-kind run set for one task; assert every Native Code Review read method returns only `code_review` rows and the objective read methods return only `objective_check` rows. This test is the audit.

## Verification

```
cd apps/backend && go test ./internal/task/repository/sqlite/... -run 'Review|Objective'
cd apps/backend && KANDEV_TEST_POSTGRES_DSN=... go test ./internal/task/repository/sqlite/... -run 'Objective'
```

## Likely files / risks

`internal/task/models/models.go`, `internal/task/repository/sqlite/{base_schema.go,base_migrations.go,task_review.go,task_review_test.go}`.
Risk: missing a code-review query in the audit silently mixes kinds in the Native Code Review UI — grep for `task_review_runs` exhaustively.

## Output contract

Summary, files changed, tests run with results, blockers, risks, `status: done`, plan checkbox.
