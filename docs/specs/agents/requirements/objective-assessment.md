---
status: draft
system: agents
created: 2026-08-31
owners:
  - samueltorres-io
---

# Task Objective Assessment Requirements

## Overview

Kandev can review a task's changed code for quality, security, and architecture
(see [Native Code Review](native-code-review.md)), and external GitHub bots add a
semantic pass after a pull request opens. Nothing checks the one question a
reviewer asks first: **does this change actually accomplish what the task asked
for?** Scope creep, a half-implemented acceptance criterion, or a diff that
solves a different problem than the one stated all pass every existing gate.

This capability adds an agent-driven pass that reads the task's stated objective,
derives a checklist of acceptance criteria, and judges the task's current changes
against each one, producing a single verdict (`met` / `partial` / `unmet`) plus a
per-criterion breakdown with evidence. It reuses the Native Code Review runtime
(run entity, utility-agent profile resolution, inference substrate, workflow-step
trigger, task MCP publish path) and adds an objective-specific output shape and an
optional workflow gate.

The agent system owns this contract because it owns agent-driven review
capabilities and the utility-agent profile contract both passes share. The
optional workflow gate consumes the [task and workflow](../../tasks/README.md)
quorum-decision contract; it does not own it.

## Terminology

- **Objective:** The outcome the task is meant to produce, taken from the task
  description and any task-attached `plan` / `spec` document.
- **Criterion:** One testable statement the change must satisfy. Either derived
  by the assessing agent from the objective, or copied verbatim from an
  acceptance-criteria list found in a task-attached document.
- **Assessment:** One run of the pass. Produces a verdict and a set of criterion
  results.
- **Verdict:** `met` (every criterion met), `partial` (at least one met and at
  least one not), or `unmet` (no criterion met). Computed from the criterion
  results; the agent's own stated verdict is advisory and not stored. An
  assessment the agent cannot produce at all is a failed run, not a fourth
  verdict.
- **Criterion result:** `met` / `partial` / `unmet` / `unknown`, each with a
  one-paragraph rationale and optional file/line evidence pointers. `unknown`
  counts as not met for the verdict rollup.
- **Gate:** A workflow-step configuration where a non-`met` verdict blocks the
  transition out of the step until re-assessed.

## Requirements

### REQ-AGENTS-OBJECTIVE-ASSESSMENT-001: Task Objective Assessment

**Intent:** A reviewer's first question — "did this do what was asked?" — has no
automated answer in Kandev today. Give users and workflows a repeatable,
evidence-backed verdict on whether a task's current changes satisfy its stated
objective, before a human review or a pull request.

**User story:** As a Kandev user, I want an agent to check my task's changes
against what the task asked for, so that I catch a missed or misread requirement
before I open the PR.

#### Acceptance criteria

- **AC-AGENTS-OBJECTIVE-ASSESSMENT-001.1:** From the Review/Changes surface a user
  SHALL be able to run an objective assessment of a task's current changes,
  without a pull request, a remote, or a Git host. The run control is hidden or
  disabled with an explanatory tooltip when the task has neither a description nor
  a `plan` / `spec` document, since there is no objective to assess.
- **AC-AGENTS-OBJECTIVE-ASSESSMENT-001.2:** An assessment reads the objective from
  the task description plus any task-attached document keyed `plan` or `spec`.
  When such a document contains an explicit acceptance-criteria list, those items
  are used verbatim as criteria; otherwise the agent derives 1 to 12 criteria
  from the objective text.
- **AC-AGENTS-OBJECTIVE-ASSESSMENT-001.3:** A completed assessment produces
  exactly one verdict (`met` / `partial` / `unmet`), computed from the criterion
  results by a fixed rule (all `met` → `met`; at least one `met` and at least one
  not → `partial`; no `met` → `unmet`); an optional one-paragraph summary; and one
  criterion result per criterion. Each criterion result carries a status (`met` /
  `partial` / `unmet` / `unknown`), a rationale, and zero or more evidence
  pointers (repository, file path, optional line range).
- **AC-AGENTS-OBJECTIVE-ASSESSMENT-001.4:** The verdict, summary, and criterion
  checklist render on the task's Review surface. Evidence pointers that resolve to
  a file in the current changed set link to that line in the existing diff
  viewer; the assessment does NOT create inline diff annotations.
- **AC-AGENTS-OBJECTIVE-ASSESSMENT-001.5:** An assessment is **advisory** by
  default. Kandev SHALL NOT apply, stage, commit, or revert any change based on an
  assessment. From a criterion result the user MAY **Send to agent**, which turns
  the criterion text, status, rationale, and evidence into ordinary agent context
  for the active session.
- **AC-AGENTS-OBJECTIVE-ASSESSMENT-001.6:** An assessment SHALL be triggerable
  three ways: on demand from the Review/Changes surface; as a workflow-step
  `on_enter` action so it sits between an implement step and a human review step;
  and by an agent session holding task MCP, which publishes the assessment
  directly.
- **AC-AGENTS-OBJECTIVE-ASSESSMENT-001.7:** The workflow-step action MAY be
  configured as a **gate**. When gated, a completed assessment whose verdict is
  not `met` blocks the step's outbound transition until a later assessment
  returns `met`; a gated assessment that fails to complete (agent unavailable,
  unparseable result) also blocks, and the step surfaces why. A gated assessment
  on a task with **no objective to assess** does NOT block: it logs a warning and
  the step surfaces a non-blocking notice, because a missing description is not an
  unmet objective. An ungated action never blocks a transition, matching
  `run_code_review` today.
- **AC-AGENTS-OBJECTIVE-ASSESSMENT-001.8:** The assessing runtime is configurable
  independently of the agent that wrote the code, using the same resolution as
  Native Code Review: the on-demand and MCP paths use the effective profile of
  the built-in `objective-check` utility agent (falling back to the default
  utility profile); the workflow-step path MAY additionally name an agent profile
  directly. Both paths use the profile's complete launch and permission
  configuration per [Profile-backed Utility Agents](utility-agent-profiles.md).
- **AC-AGENTS-OBJECTIVE-ASSESSMENT-001.9:** For a multi-repository task the
  assessment covers all repositories' changes in one pass; evidence pointers
  carry their repository and group per repository like the rest of the Changes
  panel.
- **AC-AGENTS-OBJECTIVE-ASSESSMENT-001.10:** Every assessment is visible as a
  **run** with a status (`pending` / `running` / `completed` / `failed` /
  `cancelled`), a verdict when completed, a criterion count, and a failure reason
  when it fails. Runs and their criterion results survive a Kandev restart; an
  in-flight run does not and is marked `cancelled` with reason
  `interrupted by restart` on boot.
- **AC-AGENTS-OBJECTIVE-ASSESSMENT-001.11:** A task keeps assessments from more
  than one run. The Review surface shows the latest completed assessment and a
  short run history; a new completed assessment supersedes the previous one for
  the default view without deleting it.
- **AC-AGENTS-OBJECTIVE-ASSESSMENT-001.12:** The objective-assessment surface has
  full capability parity on phones, using native mobile presentation for the
  verdict banner, criterion checklist, and per-criterion actions.

## Out of scope

- Inline per-line diff annotations. Objective assessment output is a checklist and
  a verdict, not anchored findings; anchored findings remain Native Code Review's
  contract.
- Auto-applying, staging, committing, or reverting a change from a criterion
  result.
- A first-class link between a Kandev task and a `docs/specs/**` requirements
  document. This iteration reads acceptance criteria only from documents already
  attached to the task (`plan` / `spec` keys). A structured spec-linkage contract
  is a separate future capability.
- Reconciling the assessment with a pull request's review threads, GitHub CI
  reviewers, or PR checks, in either direction.
- Cross-task or whole-repository objective evaluation. Scope is one task's changed
  set.
- Editing criteria by hand in the UI, or persisting a user-curated criteria list
  separate from an assessment run.
- Scheduled or automatic-on-every-turn assessment. Triggers are explicit: user
  action, workflow-step entry, or an agent MCP call.
- A dedicated assessment-history UI beyond the per-task run list.
