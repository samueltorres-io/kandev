---
id: "05-mcp-and-ws-actions"
title: "publish_objective_assessment_kandev MCP tool + task.objective.* WS actions"
status: pending
wave: 4
depends_on: ["04-runner-branch-and-utility-agent"]
plan: "plan.md"
requirements:
  - REQ-AGENTS-OBJECTIVE-ASSESSMENT-001
acceptance_criteria:
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.1
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.6
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.10
system_design: "../../specs/agents/system-design/objective-assessment.md"
---

# Task 05: MCP publish tool + on-demand WS actions

## Summary

The two non-workflow triggers: an agent MCP tool and the on-demand
run/cancel/get WS actions, plus DTOs and the E2E reset cascade. Both are thin
handler files in the style of `internal/mcp/handlers/review.go`.

## Scope

### MCP tool

- `internal/mcp/server/server.go` — `registerObjectiveTools()` adding `publish_objective_assessment_kandev`. Schema: `task_id?`, `summary?`, `criteria[]` with `text`, `status`, `source_ref?`, `rationale`, `evidence?`. **No `verdict` argument** — the service computes it.
- `internal/mcp/server/handlers.go` — `publishObjectiveAssessmentHandler` resolving `task_id` via `s.resolveTaskID`, relaying over the agent WS stream via `ws.ActionMCPPublishObjectiveAssessment`.
- `pkg/websocket/actions.go` — `ActionMCPPublishObjectiveAssessment = "mcp.publish_objective_assessment"`.
- `internal/mcp/handlers/objective.go` — `handlePublishObjectiveAssessment` maps to `service.ObjectiveCriterionInput`, calls `ReviewService.PublishAssessment(Trigger: models.ReviewTriggerAgent, RunID: "")`. `ErrInvalidObjectiveCriterion` / `ErrNoCriteria` → `ws.ErrorCodeValidation`. Returns `{run_id, criterion_count, verdict}`.

### On-demand WS actions

- `pkg/websocket/actions.go` — `task.objective.run`, `task.objective.cancel`, `task.objective.get`.
- `internal/mcp/handlers/objective.go` —
  - `handleRunObjectiveAssessment` → `review.Runner.Launch(Kind: objective_check, Trigger: manual, AgentProfileID?)`. Maps `review.CodeFor(err)` to `data.code` (`objective_agent_unavailable`, `objective_no_changes`, `objective_no_objective`).
  - `handleCancelObjectiveAssessment` → prefer `Runner.Cancel`, fall back to `ReviewService.CancelRun`.
  - `handleGetObjectiveAssessment` → `ReviewService.GetTaskAssessment`.
- `internal/mcp/handlers/handlers.go` — register all four handlers gated on `reviewService != nil` / `reviewRunner != nil`.

### DTOs + plumbing

- DTOs in `pkg/api/v1/` (review DTO home): `TaskObjectiveAssessment`, `TaskObjectiveCriterion`, `TaskObjectiveRun` + `ToAPI`.
- `internal/gateway/websocket/dispatch_scope.go` — confirm `task.objective.*` payloads carry `task_id` so the backstop scopes them (deep `task.*` namespace must carry `task_id`).
- `cmd/kandev/e2e_reset.go` — call `DeleteTaskObjectiveCriteriaByWorkspace` before task deletion; keep the goroutine/E2E reset invariant.

## Exclusions

Frontend (task 07), workflow action (task 06).

## Implementation acceptance conditions

- MCP: two valid criteria → a `completed` `objective_check` run, `trigger = agent`, visible via `GetTaskAssessment` with no reload; one criterion missing `text` → error to the agent, nothing stored.
- `task.objective.run` on a task with no changes returns `objective_no_changes` and creates no run.
- A foreign `task_id` is denied by the dispatch backstop before the handler runs.

## Verification

```
cd apps/backend && go test ./internal/mcp/... ./internal/gateway/websocket/... -run 'Objective'
cd apps/backend && go test ./cmd/kandev/... -run 'Reset'
```

## Likely files / risks

`internal/mcp/server/{server.go,handlers.go}`, `internal/mcp/handlers/{objective.go,handlers.go}`, `pkg/websocket/actions.go`, `pkg/api/v1/`, `internal/gateway/websocket/dispatch_scope.go`, `cmd/kandev/e2e_reset.go`.
Reference: `internal/mcp/handlers/review.go` (`handlePublishReviewFindings`, `handleRunTaskReview`, `reviewLaunchError`).

## Output contract

Summary, files changed, tests run with results, blockers, risks, `status: done`, plan checkbox.
