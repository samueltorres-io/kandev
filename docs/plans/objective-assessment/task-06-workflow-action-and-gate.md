---
id: "06-workflow-action-and-gate"
title: "run_objective_check workflow action + gate decision writer"
status: done
wave: 4
depends_on: ["04-runner-branch-and-utility-agent"]
plan: "plan.md"
requirements:
  - REQ-AGENTS-OBJECTIVE-ASSESSMENT-001
acceptance_criteria:
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.6
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.7
system_design: "../../specs/agents/system-design/objective-assessment.md"
---

# Task 06: Workflow step action + optional gate

## Summary

A `run_objective_check` `on_enter` action that starts an assessment on step
entry, plus an optional gate that blocks the outbound transition on a non-`met`
verdict by writing a synthetic quorum decision.

## Scope

- `internal/workflow/models/models.go` — `OnEnterRunObjectiveCheck OnEnterActionType = "run_objective_check"`; config keys `agent_profile_id`, `gate`. `OnEnterAction` payload extension.
- `internal/workflow/engine/types.go` — `ActionRunObjectiveCheck ActionKind`; `RunObjectiveCheckAction{AgentProfileID string, Gate bool}`; `CompileStep` case; `readObjectiveCheckConfig`. Add to `sessionIndependentActionKinds`.
- `internal/workflow/engine/entrydispatch.go` / `stepentry` — runs once per committed arrival with `entry_id` (idempotency via the run's `entry_id`).
- `internal/orchestrator/workflow_callbacks.go` — `runObjectiveCheckCallback` registered when `svc.reviewRunner != nil`. `Execute` threads `AgentProfileID`, `Gate`, `WorkflowStepID`, `EntryID` into `Runner.Launch(Kind: objective_check, Trigger: workflow_step)`. Returns `ActionResult{}, nil` — never blocks step entry.
- Gate decision writer — implement the `stepDecisionWriter` seam from task 02, using the reserved `ObjectiveCheckRole` constant. On a gated `objective_check` **completion** with `trigger = workflow_step`: delete any prior `objective-check` decision for `(task_id, workflow_step_id)`, then insert one — `approve` if verdict `met`, else `reject` — actor/role `objective-check`. On a gated **failure** (from the runner's `fail()` path): insert `reject` with the error note — **except** `objective_no_objective`, which writes nothing and lets the callback log a warning + surface a non-blocking notice. Delete-then-insert, never append.
- `internal/workflow/models/export.go` + `internal/workflow/service/` sync-apply — round-trip `agent_profile_id` (portable ref) and `gate` on workflow import/export.
- Guard config: when `gate` is on the transition carries `WaitForQuorumGuard{Role: ObjectiveCheckRole, Threshold: "n_approve:1"}` + a `clear_decisions` for that role on step re-entry. The engine guard evaluation is unchanged. `ObjectiveCheckRole` is excluded from the human role picker / quorum-role config (enforced with a test).

## Exclusions

The step-editor UI control (task 09) — this WO ships the backend contract and asserts the guard blocks/unblocks.

## Implementation acceptance conditions

- Entering a step with an ungated `run_objective_check` action starts a `workflow_step`-trigger run and the transition is unaffected.
- With `gate` on: a `partial` verdict writes a `reject`, the `WaitForQuorumGuard` does not fire, the transition stays; a subsequent `met` re-run **replaces** the row with `approve` and the transition proceeds.
- A gated run that fails writes `reject` with the failure reason; a gated run with `objective_no_objective` writes no decision and does not block.
- Isolation test: a step with human role-based quorum plus an objective gate keeps the two decision sets separate; `ObjectiveCheckRole` is rejected by the human role picker.

## Verification

```
cd apps/backend && go test ./internal/workflow/... ./internal/orchestrator/... -run 'Objective|Guard|Quorum'
```

## Likely files / risks

`internal/workflow/models/{models.go,export.go}`, `internal/workflow/engine/{types.go,entrydispatch.go}`, `internal/orchestrator/workflow_callbacks.go`, `internal/workflow/service/sync_apply.go`.
Risk: decision overwrite semantics — ensure re-run replaces, not appends, the `objective-check` decision.

## Output contract

Summary, files changed, tests run with results, blockers, risks, `status: done`, plan checkbox.
