---
id: "02-service-and-events"
title: "ReviewService.PublishAssessment / GetTaskAssessment + bus/WS events"
status: pending
wave: 2
depends_on: ["01-schema-and-repository"]
plan: "plan.md"
requirements:
  - REQ-AGENTS-OBJECTIVE-ASSESSMENT-001
acceptance_criteria:
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.3
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.10
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.11
system_design: "../../specs/agents/system-design/objective-assessment.md"
---

# Task 02: Assessment write path + events

## Summary

Extend `ReviewService` with the single write path for objective assessments and
fan out the three new bus events.

## Scope

- `internal/task/service/review_service.go`:
  - `PublishAssessment(ctx, PublishAssessmentRequest{TaskID, RunID, Trigger, Summary, Criteria []ObjectiveCriterionInput, WorkflowStepID, Gate})`. No `Verdict` in the request — the agent's stated verdict is never trusted. Normalizes/validates each criterion (text required, ≤ 2000 chars; known status; rationale ≤ 4000 chars; evidence entries well-formed). **Computes** `verdict` from criterion statuses: all `met` → `met`; at least one `met` and at least one not-`met` → `partial`; no `met` → `unmet` (`unknown` counts as not-`met`). Empty criteria is rejected (`ErrNoCriteria`) — the runner fails the run instead. When `RunID == ""` creates a synthetic `completed` run with `kind = objective_check` and the given trigger (MCP path). Writes criteria, updates the run's `verdict` + `finding_count` (= criterion count), publishes `TaskObjectivePublished`.
  - `GetTaskAssessment(taskID)` → `{runs (objective_check only, newest first, cap 20), criteria (latest completed run)}`.
  - `ClearTaskAssessment(taskID)` → delete `objective_check` runs + criteria, publish `TaskObjectiveCleared`.
  - `CompleteRun` already generic; add a `verdict` argument path or a `CompleteAssessmentRun` wrapper that sets `verdict` before publishing `TaskReviewRunUpdated` (reused for run status).
- `internal/events/types.go` — `TaskObjectivePublished`, `TaskObjectiveCleared`. (Run status reuses `TaskReviewRunUpdated`.)
- `pkg/websocket/actions.go` — `task.objective.published`, `task.objective.cleared`, `task.objective.run_updated` (or reuse `task.review.run_updated` if the payload is identical — decide and document).
- `internal/gateway/websocket/task_notifications.go` — forward the new bus events, task-scoped.
- Gate hook seam: when `Gate` is set and `Trigger == workflow_step` and `WorkflowStepID != ""`, `PublishAssessment` calls a `stepDecisionWriter` interface (implemented in task 06) — define it here, keep it nil-safe. Add the reserved role constant `ObjectiveCheckRole = "objective-check"` in a package importable by both the service and the workflow step editor.

## Exclusions

Runner, MCP tool, WS request handlers, workflow callback.

## Implementation acceptance conditions

- A malformed criterion rejects the whole batch with an error naming the index and field; nothing is written.
- Verdict is always the computed rollup; the request carries no verdict field. Table-driven test pins every status combination.
- Each mutating method publishes exactly one event of the expected type with the expected payload keys.

## Verification

```
cd apps/backend && go test ./internal/task/service/... -run 'Assessment|Objective'
cd apps/backend && go test ./internal/gateway/websocket/...
```

## Likely files / risks

`internal/task/service/{review_service.go,review_service_test.go}`, `internal/events/types.go`, `pkg/websocket/actions.go`, `internal/gateway/websocket/task_notifications.go`.
Reference: the finding validation + supersede logic already in `review_service.go`.

## Output contract

Summary, files changed, tests run with results, blockers, risks, `status: done`, plan checkbox.
