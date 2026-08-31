# Feature issue draft: objective-assessment review node

Filled against the "Feature request" form fields. Copy each section into the
matching form field.

---

## Title

Add a final "was the task objective actually met?" review node

---

## Problem

Kandev has no automated pass that answers a reviewer's very first question:
**did this change do what the task asked for?**

Today a task's changes go through:

- **Native Code Review** (`docs/specs/agents/requirements/native-code-review.md`)
  produces line-anchored findings on quality, security, and architecture.
- External GitHub review bots add a semantic pass, but only *after* a PR opens,
  outside Kandev, and only when a Git host is configured.
- Workflow steps can gate on human quorum decisions.

None of these compares the diff against the task's stated objective. A change
that is clean, secure, well-structured, and solves a *different* problem than the
one in the task description passes every existing gate. Scope creep, a
half-implemented acceptance criterion, or a misread requirement are all invisible
until a human notices, usually late.

There is also no node positioned as the *final* check between "implement" and
"human review / open PR" that asserts completeness. A user running an autonomous
or multi-step workflow has no way to stop a task from advancing to review when
the work is only half done.

---

## Proposed solution

Add an agent-driven **objective assessment** pass that:

1. Reads the task's objective from the task description plus any task-attached
   `plan` / `spec` document.
2. Derives a checklist of acceptance criteria (or copies them verbatim when the
   document already lists `AC-*` items).
3. Judges the task's current changed set against each criterion.
4. Produces a single computed verdict (`met` / `partial` / `unmet`) plus a
   per-criterion breakdown with rationale and file/line evidence, rendered on the
   task's Review surface.

Outcome for the user: before a human review or a PR, they get a repeatable,
evidence-backed answer to "is this task actually done?", and can optionally make
a workflow enforce it.

It is **advisory by default**, with an **optional workflow-step gate** that
blocks a step's outbound transition until the verdict is `met`. Triggerable three
ways: on demand from the Review/Changes surface, as a workflow-step `on_enter`
action (so it sits between an implement step and a human review step), and by an
agent session via task MCP.

### Reuse, not a new subsystem

The design extends the Native Code Review runtime rather than forking it:

- Same run entity (`task_review_runs`), with a new `kind` column
  (`code_review` | `objective_check`) and a `verdict` column.
- Same utility-agent profile resolution (a new `objective-check` built-in
  utility agent).
- Same inference substrate, workflow-step trigger, and task-MCP publish path.
- New: a `task_objective_criteria` table for the checklist output (line-anchored
  findings do not fit a "met / not met" checklist), and the optional gate.

The gate reuses the existing `workflow_step_decisions` + `WaitForQuorumGuard`
contract by writing a synthetic decision under a reserved role, so the workflow
engine needs no new guard type.

---

## Affected area

Agents / review (extends Native Code Review), with workflow-engine and frontend
surface area.

---

## Who needs this?

- Users who run tasks through multi-step or autonomous workflows and want a
  completeness gate before the human-review step.
- Solo users who want a sanity check that a task's diff matches its description
  before opening a PR.
- Teams reviewing agent-authored PRs, where "clean code, wrong problem" is the
  most common failure mode.

---

## Target workflow

```
implement step  ->  [ run_objective_check ]  ->  human review step  ->  open PR
```

- The assessment runs as a workflow-step `on_enter` action after the implement
  step completes, using the step's configured agent profile (or the
  `objective-check` built-in utility agent).
- When the step's action is configured as a **gate**, a non-`met` verdict writes
  a synthetic `reject` decision under the reserved `objective-check` role; the
  existing `WaitForQuorumGuard` on the outbound transition then holds the task in
  the step until a re-run returns `met`.
- On demand: from the Review/Changes surface of any task, no PR or Git host
  required.
- Via MCP: an agent session holding task MCP calls
  `publish_objective_assessment_kandev` with its criteria breakdown.

---

## Alternatives considered

- **Fold it into Native Code Review as another finding category.** Rejected:
  the output shape is a verdict + checklist, not anchored findings; overloading
  the findings table and UI would blur both.
- **A brand-new review subsystem.** Rejected: duplicates run lifecycle, profile
  resolution, restart handling, and the MCP path for no benefit.
- **A new workflow guard type (`RequiresObjectiveMet`).** Rejected: reusing the
  quorum-decision contract with a reserved role is a much smaller change and
  keeps one guard-evaluation path.
- **Link the task to a `docs/specs/**` requirements doc for criteria.** Deferred:
  no task-to-spec linkage table exists today. v1 reads only `plan` / `spec` task
  documents; structured spec linkage is a separate future capability.
- **Workaround today:** a human reads the task description and the diff manually,
  or adds a human quorum step. Neither is repeatable or evidence-backed, and
  neither works for autonomous runs.

---

## Acceptance criteria

- From the Review/Changes surface a user can run an objective assessment of a
  task's current changes without a PR, remote, or Git host. The control is
  hidden/disabled with a tooltip when the task has no description and no
  `plan` / `spec` document.
- An assessment reads the objective from the task description plus task-attached
  `plan` / `spec` documents; explicit `AC-*` lists are used verbatim, otherwise
  the agent derives 1 to 12 criteria.
- A completed assessment produces exactly one verdict (`met` / `partial` /
  `unmet`) computed by a fixed rule from per-criterion results (`met` / `partial`
  / `unmet` / `unknown`, each with rationale and optional file/line evidence).
- The verdict, summary, and criterion checklist render on the Review surface;
  navigable evidence pointers jump to the diff viewer; no inline diff
  annotations are created.
- The assessment is advisory: Kandev never applies, stages, commits, or reverts
  a change from it. "Send to agent" turns a criterion into agent context.
- Triggerable on demand, as a workflow-step `on_enter` action, and via task MCP.
- The workflow-step action can be configured as a gate: a non-`met` or failed
  assessment blocks the outbound transition and the step surfaces why; a gated
  step with no objective does NOT block (logs a warning, non-blocking notice);
  an ungated action never blocks.
- The assessing runtime is configurable independently of the coding agent, via
  the same resolution as Native Code Review.
- Multi-repository tasks are assessed in one pass; evidence groups per repo.
- Every assessment is a run with status/verdict/criterion-count/failure-reason;
  runs and criteria survive restart; in-flight runs become `cancelled` on boot.
- A task keeps more than one assessment; the Review surface shows the latest plus
  a short history.
- Full capability parity on phones (native mobile presentation).

---

## Risks and constraints

- **Persistence / migration:** two new columns on `task_review_runs`, one new
  table (`task_objective_criteria`). Idempotent `ADD COLUMN` migrations; `kind`
  defaults to `code_review` so existing rows and inserts stay correct. The shared
  table now holds two run kinds: code-review-only list reads add
  `AND kind = 'code_review'`, and a mixed-kind repository test acts as the
  migration audit.
- **Public API / protocol:** new WS actions (`task.objective.run` / `cancel` /
  `get`), events (`task.objective.run_updated` / `published` / `cleared`), and
  MCP tool `publish_objective_assessment_kandev`.
- **Workflow contract:** new `run_objective_check` `on_enter` action with
  `{agent_profile_id, gate}`; export/import round-trip; one reserved quorum role.
  Gate decisions share `workflow_step_decisions` with human quorum decisions:
  the `objective-check` role is excluded from the human role picker (test
  enforced) and decisions are delete-then-insert per `(task_id, step_id)`.
- **Deadlock avoidance:** a gated step on a task with no objective writes no
  decision and does not block.
- **Cost / latency:** each assessment is one extra inference call over the
  changed set; runs are explicit-trigger only (no per-turn or scheduled runs).
- **Privacy:** same data exposure as Native Code Review (diff + task text to the
  configured utility agent); no new external surface.

---

## References

- Sibling feature: `docs/specs/agents/requirements/native-code-review.md`
- Profile resolution: `docs/specs/agents/requirements/utility-agent-profiles.md`
- Draft design package (on a branch, linkable from the PR):
  - `docs/specs/agents/requirements/objective-assessment.md`
  - `docs/specs/agents/system-design/objective-assessment.md`
  - `docs/plans/objective-assessment/plan.md` (10 work orders, 6 waves)

---

## Before submitting

- [x] I checked existing issues and did not find this request already tracked.
- [x] This request describes user value, not only an implementation detail.
