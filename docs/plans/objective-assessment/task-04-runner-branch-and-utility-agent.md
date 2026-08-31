---
id: "04-runner-branch-and-utility-agent"
title: "Runner objective_check branch + objective-check utility agent"
status: pending
wave: 3
depends_on: ["02-service-and-events", "03-objective-extraction-and-parser"]
plan: "plan.md"
requirements:
  - REQ-AGENTS-OBJECTIVE-ASSESSMENT-001
acceptance_criteria:
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.1
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.8
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.9
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.10
system_design: "../../specs/agents/system-design/objective-assessment.md"
---

# Task 04: Runner branch + utility agent

## Summary

Give `internal/review.Runner` a `Kind` field and an `objective_check` execution
path that runs one inference call and publishes an assessment. Add the
`objective-check` built-in utility agent.

## Scope

- `internal/review/runner.go` — `RunRequest.Kind` (default `code_review`), plus `Gate bool` carried through from the workflow callback. `execute()` passes `Gate` + `WorkflowStepID` into `store.PublishAssessment` so the gate decision writer (task 06) fires. In `launch()` / `execute()`, branch on kind:
  - `code_review`: unchanged.
  - `objective_check`: after `CollectChanges` and `resolver.Resolve`, call `BuildObjectiveContext` (needs a `TaskContextLookup` extension returning the task's `plan`/`spec` docs — add a `DocumentsLookup` dep or extend the existing one). Build the prompt from the `objective-check` template with `{Objective, PredefinedCriteria, GitDiff, DiffSummary, ChangedFiles, TaskTitle, BranchName, BaseBranch}`. Trim per-file diffs to the ~120 KB budget, always keeping the full changed-file list. One `inference.Run`. `ParseAssessment`. `store.PublishAssessment(RunID=runID, ...)`. `store.CompleteAssessmentRun(runID, verdict, criterionCount)`. Cancel check before publish. `fail()` maps to `objective_*` codes.
- `internal/review/errors.go` — `objective_agent_unavailable`, `objective_workspace_unavailable`, `objective_no_changes`, `objective_no_objective`, `objective_unparseable_response`; extend `CodeFor`.
- `execute()` `fail()` path: for a **gated** `workflow_step` run, call the gate decision writer with a `reject` — **except** `objective_no_objective`, which writes no decision (task 06 owns the writer and this carve-out).
- `internal/review/resolver.go` — parametrize the utility-agent id (`builtin-code-review` vs `builtin-objective-check`) instead of the hard-coded constant; keep the 3-tier precedence.
- `config/utilityagents/objective-check.md` — the assessment prompt: strict JSON contract from the design, instruction to evaluate predefined criteria verbatim or derive 1–12, and the sentinel line `KANDEV_OBJECTIVE_CHECK_REQUEST`.
- `internal/utility/store/builtins.go` + `internal/prompts/store/sqlite.go` — `builtin-objective-check` named `objective-check`, `enabled = 0`, `profile_binding_state = 'inherit'`, `agent_id = "claude-acp"`.
- `internal/backendapp/review_wiring.go` — extend the wiring so one `Runner` serves both kinds (utility lookup for `objective-check`, documents lookup).

## Exclusions

WS request handlers, MCP tool, workflow callback — later work orders call `Runner.Launch`.

## Implementation acceptance conditions

- Happy path: changed files + objective → `completed` run with a verdict and N criteria persisted, one `TaskObjectivePublished` event.
- `objective_no_changes` (no diff) creates no run; `objective_no_objective` (empty objective) fails the run; `objective_agent_unavailable` (passthrough-only profile) fails immediately.
- A second `Launch` for a live run returns the in-flight run.

## Verification

```
cd apps/backend && go test ./internal/review/... -run 'Objective|Assessment'
cd apps/backend && go test ./internal/utility/... ./internal/backendapp/...
```

## Likely files / risks

`internal/review/{runner.go,resolver.go,errors.go,runner_objective_test.go}`, `config/utilityagents/objective-check.md`, `internal/utility/store/builtins.go`, `internal/prompts/store/sqlite.go`, `internal/backendapp/review_wiring.go`.
Risk: `internal/review` must not import agent runtime — keep new deps behind local interfaces defined in the package, adapters in `review_wiring.go`.

## Output contract

Summary, files changed, tests run with results, blockers, risks, `status: done`, plan checkbox.
