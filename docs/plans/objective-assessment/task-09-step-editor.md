---
id: "09-step-editor"
title: "Workflow step editor control + gate toggle"
status: pending
wave: 6
depends_on: ["06-workflow-action-and-gate", "08-objective-panel"]
plan: "plan.md"
requirements:
  - REQ-AGENTS-OBJECTIVE-ASSESSMENT-001
acceptance_criteria:
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.6
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.7
system_design: "../../specs/agents/system-design/objective-assessment.md"
---

# Task 09: Step editor control

## Summary

Let a user add `run_objective_check` to a workflow step, pick an agent profile,
and toggle the transition gate.

## Scope

- `apps/web/components/settings/workflows/` — next to the `run_code_review` control: a "Assess task objective on entry" checkbox, an agent-profile picker (portable ref, same component as `run_code_review` uses), and a "Block leaving this step until the verdict is met" toggle.
- When the gate toggle is on, the editor writes the outbound-transition `Guard` (`WaitForQuorumGuard` role = the reserved `ObjectiveCheckRole`, threshold `n_approve:1`) and a matching `clear_decisions` for that role on step re-entry so a re-run can flip the decision; when off, it removes both. `ObjectiveCheckRole` never appears in the human role picker.
- Self-documenting copy per frontend conventions; all through `t()`.
- Export/import round-trip already handled backend-side (task 06) — verify the editor reads back an imported config correctly.

## Exclusions

Backend action contract (task 06).

## Implementation acceptance conditions

- Adding the action with a profile and saving persists `{agent_profile_id, gate}`; reopening the editor shows them.
- Turning the gate on adds the guard + `clear_decisions`; turning it off removes them.
- An imported workflow with the action renders the control populated.

## Verification

```
cd apps/web && pnpm vitest run components/settings/workflows
cd apps/web && pnpm run typecheck && pnpm --filter @kandev/web lint
```

## Likely files / risks

`apps/web/components/settings/workflows/*` (step editor + action list), workflow-config types in `apps/web/lib/types/`.
Reference: the existing `run_code_review` step-editor control.

## Output contract

Summary, files changed, tests run with results, blockers, risks, `status: done`, plan checkbox.
