---
status: draft
system: agents
requirements:
  - REQ-AGENTS-OBJECTIVE-ASSESSMENT-001
---

# Task Objective Assessment System Design

## Purpose and boundaries

This design adds an agent-driven pass that judges a task's current changes
against its stated objective and produces a verdict plus a criterion checklist.
It is a sibling of [Native Code Review](native-code-review.md) and reuses that
system's run orchestration, utility-agent profile resolution, inference
substrate, diff acquisition, workflow-step trigger plumbing, and task MCP publish
path.

Owned here: the `objective-check` run kind, the assessment/criterion data model,
the assessment prompt contract, the `publish_objective_assessment_kandev` MCP
tool, the `run_objective_check` workflow-step action, and the Review-surface
verdict/checklist presentation.

Consumed but not owned:

- **Native Code Review runtime** (`internal/review/`, `internal/task/service/review_service.go`, `task_review_runs`) — extended, not forked.
- **[Profile-backed Utility Agents](utility-agent-profiles.md)** — effective-profile resolution.
- **Task and workflow quorum-decision contract** (`internal/workflow/engine` guards, `workflow_step_decisions`) — the gate records a synthetic decision so the existing `WaitForQuorumGuard` blocks the transition. No new engine guard type.
- **Task documents** (`internal/task/service/document_service.go`, `TaskDocument` keys `plan` / `spec`) — objective and acceptance-criteria source.

## Requirement mapping

| Requirement / AC | Design section |
| --- | --- |
| `REQ-AGENTS-OBJECTIVE-ASSESSMENT-001` | [Components and responsibilities](#components-and-responsibilities) |
| AC-...-001.1, .6 (triggers) | [Triggers](#triggers), [Control flow](#control-flow) |
| AC-...-001.2 (objective source) | [Objective and criteria extraction](#objective-and-criteria-extraction) |
| AC-...-001.3, .9 (output shape) | [Data and contracts](#data-and-contracts) |
| AC-...-001.4, .5, .11, .12 (surface, advisory, history, mobile) | [Frontend](#frontend-components) |
| AC-...-001.7 (gate) | [Workflow gate](#workflow-gate) |
| AC-...-001.8 (runtime config) | [Profile resolution](#profile-resolution) |
| AC-...-001.10 (runs, restart) | [Persistence](#persistence), [Failure and recovery](#failure-and-recovery) |

## Components and responsibilities

### Backend

| Component | Responsibility |
| --- | --- |
| `task_review_runs.kind` column (`code_review` \| `objective_check`, default `code_review`) | Discriminates the two run kinds on the shared table. |
| `task_objective_criteria` table | One row per criterion result of an `objective_check` run. |
| `task_review_runs.verdict` column (`""` \| `met` \| `partial` \| `unmet`) | Rollup verdict computed from criterion statuses; empty for `code_review` runs and for `objective_check` runs that did not complete. |
| `ReviewService` (extended) | Adds `PublishAssessment`, `GetTaskAssessment`; `CreateRun`/`MarkRunRunning`/`CompleteRun`/`FailRun`/`CancelRun` already generic over `kind`. Single write path. |
| `internal/review` `Runner` (extended) | `Launch` gains `Kind`. For `objective_check`: builds the objective/criteria context, runs one inference call (no per-file batching — the whole diff summary plus changed-file list, budget-capped), parses the assessment JSON, calls `store.PublishAssessment`, completes the run with the verdict. |
| `internal/review` `objective.go` (new) | Objective + criteria extraction from task description and `plan`/`spec` documents; assessment-response parsing (`ParseAssessment`). |
| `objective-check` built-in utility agent | `internal/utility/store/builtins.go` entry + `config/utilityagents/objective-check.md` prompt. Prompt defines the strict JSON assessment contract and carries a sentinel line for the mock agent. |
| `run_objective_check` workflow action | `internal/workflow/models` + `internal/workflow/engine` action kind; `internal/orchestrator/workflow_callbacks.go` callback. Config: `agent_profile_id` (portable ref), `gate` (bool). |
| Gate decision writer | On a gated `objective_check` completion whose `trigger = workflow_step`, `ReviewService` **upserts** a `workflow_step_decisions` row: `approve` when verdict `met`, `reject` otherwise, under the reserved role `objective-check`. |
| `publish_objective_assessment_kandev` MCP tool | `internal/mcp/server` schema + `internal/mcp/handlers/objective.go` handler → `ReviewService.PublishAssessment` with `trigger = agent`. |
| WS actions/events | `task.objective.run`, `task.objective.cancel`, `task.objective.get`; events `task.objective.run_updated`, `task.objective.published`, `task.objective.cleared`. (`task.review.*` run status events are reused where the payload is just the run; the assessment payload needs its own event.) |

### Frontend

| Component | Responsibility |
| --- | --- |
| `lib/types/objective.ts` | `ObjectiveVerdict` (`met` \| `partial` \| `unmet`), `ObjectiveCriterionStatus`, `TaskObjectiveAssessment`, `TaskObjectiveCriterion`. |
| `lib/state/slices/objective/` | `assessmentsByTaskId`, `runsByTaskId`; `setTaskAssessment`, `upsertAssessmentRun`, `clearTaskAssessment`. Mirrors the review slice. |
| `lib/ws/handlers/objective.ts` | Maps the three server events into the slice. |
| `lib/api/domains/objective-api.ts` | `runObjectiveAssessment`, `cancelObjectiveAssessment`, `getObjectiveAssessment`. |
| `components/review/objective-panel.tsx` | Verdict banner (`met` / `partial` / `unmet`) + criterion checklist; per-criterion Send-to-agent and evidence jump. Lives beside the findings overview in the Review top bar. |
| `components/review/objective-run-button.tsx` | Run/cancel control. Hidden or disabled with a tooltip when the task has no description and no `plan` / `spec` document. |
| `components/settings/workflows/` | Step-editor control for `run_objective_check` (checkbox + agent-profile picker + "Gate transition on a met verdict" toggle). |
| `components/task/mobile/` | Bottom-sheet presentation of the verdict + checklist. |

## Data and contracts

### `task_objective_criteria`

```
id             string    PK
run_id         string    FK -> task_review_runs.id (cascade delete), indexed
task_id        string    FK -> tasks.id (cascade delete), indexed
ordinal        int       0-based position in the checklist
source         enum      derived | document
source_ref     string    e.g. "AC-AGENTS-X-001.2" when source = document; "" otherwise
text           string    the criterion statement, <= 2000 chars
status         enum      met | partial | unmet | unknown
rationale      string    Markdown, <= 4000 chars
evidence       json      [] of {repo, file, line?, line_end?}; repo "" for single-repo
created_at     timestamp
```

Index `(task_id, run_id)`. Deleting a task or its run cascades. A new completed
`objective_check` run does not delete prior runs' criteria; the frontend shows
the latest completed run's set.

### `task_review_runs` additions

- `kind` — `code_review` (default) | `objective_check`.
- `verdict` — `""` for code review and for an incomplete objective check; one of
  `met` / `partial` / `unmet` for a completed objective check.

`finding_count` is reused as the criterion count for `objective_check` runs (no
new column). `entry_id` unique-partial-index dedup for redelivered workflow step
entries applies unchanged.

### Assessment JSON contract (agent output)

```json
{
  "verdict": "partial",
  "summary": "Login works; the rate-limit criterion is unimplemented.",
  "criteria": [
    {
      "text": "A user can sign in with email and password.",
      "source_ref": "",
      "status": "met",
      "rationale": "Handler added in auth/login.go:40 with a passing test.",
      "evidence": [{ "file": "apps/backend/internal/auth/login.go", "line": 40 }]
    }
  ]
}
```

Parsing accepts a fenced ` ```json ` block or a bare object, tolerating prose
around it. The agent's `verdict` field is **ignored** — `ReviewService` always
computes the stored verdict from the criterion statuses by a fixed rule: all
`met` → `met`; at least one `met` and at least one not-`met` → `partial`; no
`met` → `unmet` (`unknown` counts as not-`met`). A response with zero parseable
criteria fails the run with `objective_unparseable_response`. A criterion missing
`text` or `status`, or with an unknown status, is dropped and counted; the run
still completes and the summary notes the count. Evidence entries naming a file
outside the changed set are kept but flagged non-navigable by the frontend.

### MCP tool `publish_objective_assessment_kandev`

```
task_id   string   optional, defaults to current task
summary    string   optional
criteria   array    required, >= 1 entry; each: {text, source_ref?, status, rationale, evidence?}
```

Returns `{run_id, criterion_count, verdict}`, where `verdict` is the value
`ReviewService` computed from the submitted criterion statuses — the tool takes no
`verdict` argument. Creates a `task_review_runs` row with `kind = objective_check`,
`trigger = agent`, `status = completed`. A malformed entry rejects the whole call
(the agent can retry), matching `publish_review_findings_kandev`.

### `run_objective_check` workflow-step action

`OnEnterActionType = "run_objective_check"`, config
`{ "agent_profile_id": string, "gate": bool }`. Compiles to
`engine.ActionRunObjectiveCheck` with `RunObjectiveCheckAction{AgentProfileID, Gate}`.
Exported/imported with the workflow, referencing the profile by portable agent
name/model/mode like every other step profile reference. Listed in
`sessionIndependentActionKinds` so the step-entry ledger runs it once per
committed arrival with an `entry_id`.

## Objective and criteria extraction

`internal/review/objective.go` `BuildObjectiveContext(task, documents)`:

1. Objective text = `task.Description` + the body of the newest `plan` document +
   the newest `spec` document (each capped, total capped at the prompt budget,
   ~60 KB).
2. If a document body contains an acceptance-criteria list — a Markdown list
   whose items match `AC-...:` or a `## Acceptance criteria` / `### Acceptance`
   heading followed by list items — those items become `source = document`
   criteria with `source_ref` set to the matched `AC-*` id when present. The
   agent then only *evaluates* them, it does not invent the list.
3. Otherwise the prompt instructs the agent to derive 1–12 criteria from the
   objective text (`source = derived`, `source_ref = ""`).

No new task/spec linkage table. This deliberately uses documents already attached
to the task by the handoff cascade or the user.

## Profile resolution

Identical to Native Code Review (`internal/review/resolver.go`), with the utility
agent id `builtin-objective-check` substituted for `builtin-code-review`:

1. Explicit `agent_profile_id` (workflow-step path) — resolve from the agent
   profile; reject CLI-passthrough.
2. Enabled `objective-check` utility agent — its bound or inherited profile.
3. User default utility agent/model.

Fails closed with `ErrAgentUnavailable` → run fails immediately with
`objective_agent_unavailable`; the surface links to Settings → Utility Agents and
does not retry.

## Control flow

### On-demand / MCP

```
UI "Assess objective"  ->  WS task.objective.run
  -> Runner.Launch(Kind=objective_check, Trigger=manual)
     claim(taskId) single-slot lock; rejoin an in-flight run if present
     resolver.Resolve(profileID)
     CollectChanges(sessionID, repositoryID?)   [reused; ErrNoChanges -> no run row]
     store.CreateRun(kind=objective_check)      -> pending, event task.objective.run_updated
     detached goroutine (10-min timeout):
       MarkRunRunning
       BuildObjectiveContext(task, documents)
       prompts.Build(objective-check template, {Objective, Criteria?, GitDiff, DiffSummary, ChangedFiles, TaskTitle, BranchName, BaseBranch})
       inference.Run(...)                        [reused substrate]
       ParseAssessment -> verdict, criteria
       store.PublishAssessment(runID, verdict, summary, criteria)  -> event task.objective.published
       store.CompleteRun(runID, verdict, criterionCount)           -> event task.objective.run_updated
```

Agent MCP path skips the runner: `publish_objective_assessment_kandev` →
`ReviewService.PublishAssessment(RunID="")` creates a synthetic completed run.

### Workflow step

`run_objective_check` `on_enter` → `runObjectiveCheckCallback.Execute` →
`Runner.Launch(Kind=objective_check, Trigger=workflow_step, WorkflowStepID, EntryID, Gate)`.
The callback returns immediately (`ActionResult{}, nil`) — it never blocks step
entry. Gating is applied *after* completion, see below.

### Workflow gate

`objective-check` is a **reserved role**: it is a package-level constant, excluded
from the human role picker and the step quorum-role configuration in the UI, so a
gate decision can never be conflated with a human quorum decision. A single
isolation test (a step with human quorum plus an objective gate) pins that the
two decision sets do not mix.

When `Gate` is true and `trigger = workflow_step`, on run completion
`ReviewService` **upserts** the `workflow_step_decisions` row for
`(task_id, workflow_step_id, role = "objective-check")` — deleting any prior
`objective-check` decision for that step first, so a re-run replaces rather than
appends:

- verdict `met` → decision `approve`, actor/role `objective-check`
- verdict `partial` or `unmet` → decision `reject`, same actor/role
- gated run that **failed** (agent unavailable, workspace unavailable, unparseable)
  → decision `reject` with the error as the note
- gated run with **no objective to assess** (`objective_no_objective`) → **no
  decision written**; the callback logs a warning and the step surfaces a
  non-blocking notice. A missing description must not pin a workflow.

The step's outbound transition carries
`Guard: WaitForQuorumGuard{Role: "objective-check", Threshold: "n_approve:1"}`
(configured by the step editor when the gate toggle is on) plus a
`clear_decisions` for that role on step re-entry. The existing engine guard
evaluation blocks the transition until an `approve` exists; re-running the
assessment with a `met` result replaces the `reject` with an `approve` and the
transition proceeds.

Rationale for reusing decisions rather than a new guard type: the quorum guard,
its thresholds, `clear_decisions`, and the read-only `Guards` projection already
exist and are tested; a dedicated `RequiresObjectiveMet` guard would duplicate
that evaluation path. Reserving one role string is a one-line mitigation for the
only real downside. Alternative considered and rejected for that reason.

## Failure and recovery

| Condition | Behavior |
| --- | --- |
| No effective utility profile / passthrough-only / no model | Run fails immediately, `objective_agent_unavailable`, surface links to Settings → Utility Agents, no criteria written. Gated: writes `reject`. |
| Task workspace not materialized / agentctl unreachable | Run fails `objective_workspace_unavailable`; prior assessment untouched. Gated: `reject`. |
| Task has no changed files | `task.objective.run` returns `objective_no_changes`, no run created. |
| Objective text empty (no description, no `plan`/`spec` doc) | The on-demand run control is hidden/disabled with a tooltip, so this is normally unreachable from the UI. If still triggered (workflow step, MCP), the run fails `objective_no_objective` with an actionable message. A **gated** workflow step in this state writes no decision and does not block (see [Workflow gate](#workflow-gate)). |
| Response has zero parseable criteria | Run fails `objective_unparseable_response`; raw response truncated onto `error_message`. Gated: `reject`. |
| Single malformed criterion in an otherwise valid response | Dropped and counted; run completes; summary notes the count. MCP path rejects the whole call instead. |
| Diff exceeds budget | Changed-file list is always included; per-file diffs are trimmed newest/most-relevant first to fit ~120 KB; the summary names files whose diff was trimmed. Run still completes. |
| Backend restart while `pending`/`running` | On boot marked `cancelled`, `error_message = "interrupted by restart"` (reuses `CancelInFlightTaskReviewRuns`, now kind-agnostic). Never resumed. Gated: no decision written, transition stays blocked until re-run. |
| Second `task.objective.run` while one is live | Returns the in-flight run unchanged (shared `claim` lock). |

## Persistence

`task_review_runs` (kind/verdict columns via idempotent `ADD COLUMN` migrations,
then any index) and `task_objective_criteria` (new table in
`internal/task/repository/sqlite/`, `CREATE TABLE IF NOT EXISTS` in schema-init,
indexes in `runMigrations()` per `apps/backend/CLAUDE.md`). Runs against SQLite
and PostgreSQL — add the env-gated PG behavior test for every new dialect-
sensitive method.

`kind` defaults to `code_review`, so existing rows and existing Native Code Review
inserts stay correct without change. Only the unfiltered
`SELECT ... FROM task_review_runs WHERE task_id = ?` reads (the run list, active-run
lookup, history) gain an explicit `AND kind = 'code_review'`; objective reads are
separate methods filtering `kind = 'objective_check'`. One repository test inserts
a mixed-kind set for a task and asserts each Native Code Review method returns only
`code_review` rows — that test **is** the migration audit and fails if a query was
missed.

Runs and criteria survive restart; in-flight runs do not. A task keeps all its
assessment runs and criteria until the task is deleted or the user clears the
task's assessment (`task.objective.clear` → delete `objective_check` runs +
criteria for the task, event `task.objective.cleared`). `DeleteTaskReviewByTask`
/ `DeleteTaskReviewByWorkspace` and the `e2e_reset.go` cascade extend to cover
the new table.

## Security

Same trust model as Native Code Review. `task.objective.*` WS actions carry
`task_id` and are scoped by the gateway dispatch backstop plus the handler-level
`authorizeTask`. The MCP publish path is scoped to the task owner via
`internal/mcp/scope` and `ReviewService.authorize` (already wired via
`SetTaskAuthorizer`). The assessing agent runs with the resolved utility
profile's permission policy; it reads the workspace diff and task documents only.
No new secret or credential surface.

## Observability

- Structured logs on run create / running / complete / fail with `run_kind=objective_check`, `verdict`, `criterion_count`, `duration_ms`, `trigger`.
- Reuse the review run's token/duration columns.
- Gate decisions are visible in the existing workflow step decision log and the read-only `Guards` projection on the step.
- No new expvar metric in this iteration; the run list is the diagnostic surface.

## Related decisions

- Extends [Native Code Review](native-code-review.md) — no ADR; it is a sibling
  capability under the same runtime. Record an ADR only if the workflow-gate
  decision-reuse pattern is contested in review.
