---
id: "08-objective-panel"
title: "Objective panel: verdict banner, criterion checklist, send-to-agent"
status: pending
wave: 5
depends_on: ["07-frontend-state"]
plan: "plan.md"
requirements:
  - REQ-AGENTS-OBJECTIVE-ASSESSMENT-001
acceptance_criteria:
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.1
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.4
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.5
system_design: "../../specs/agents/system-design/objective-assessment.md"
---

# Task 08: Objective assessment surface

## Summary

The Review-surface panel: a run control, a verdict banner, the criterion
checklist with per-criterion rationale/evidence, and Send-to-agent.

## Scope

- `apps/web/components/review/objective-run-button.tsx` — "Assess objective" control in `review-top-bar.tsx` (and the Changes panel header), idle / running / failed states, Cancel while running, and the `objective_agent_unavailable` message linking to Settings → Utility Agents. **Hidden or disabled with a tooltip** when the task has no description and no `plan` / `spec` document (a `useHasObjectiveSource(taskId)` selector) — so `objective_no_objective` is normally unreachable from the UI.
- `apps/web/components/review/objective-panel.tsx` — verdict banner (`met` / `partial` / `unmet` tone via `verdictTone`), summary, and a list of `objective-criterion-row.tsx`: status icon, criterion text, `source_ref` chip when present, collapsible rationale (Markdown), evidence pointers.
- `apps/web/components/review/objective-criterion-row.tsx` — evidence pointer that is navigable (file in the changed set) jumps to that file/line in the existing diff viewer via the review navigation helper; a non-navigable pointer renders as plain text. Per-row **Send to agent** → reuse `use-send-finding-to-agent` pattern (`use-send-criterion-to-agent.ts`) building context from criterion text + status + rationale + evidence.
- Wire the panel into `apps/web/components/task/task-center-panel.tsx` beside the findings overview; keyboard shortcut optional (reuse `use-task-review-shortcut` pattern only if trivial).
- No inline diff annotations — do not touch `use-diff-annotation-renderer`.

## Exclusions

Mobile presentation (task 10), workflow step editor (task 09).

## Implementation acceptance conditions

- A completed `partial` assessment renders the banner, every criterion row, and the unmet count.
- A navigable evidence pointer jumps to the diff; a non-navigable one does not render a link.
- Send-to-agent posts the criterion context to the active session.
- With no description and no `plan` / `spec` doc, the run control is hidden/disabled with a tooltip.
- The `objective_agent_unavailable` error renders inline with the Settings link and no run is shown.

## Verification

```
cd apps/web && pnpm vitest run components/review/objective
cd apps/web && pnpm --filter @kandev/web lint
```

## Likely files / risks

`apps/web/components/review/{objective-run-button.tsx,objective-panel.tsx,objective-criterion-row.tsx}`, `apps/web/hooks/domains/objective/use-send-criterion-to-agent.ts`, `apps/web/components/task/task-center-panel.tsx`.
Reference: `components/review/review-findings-overview.tsx`, `hooks/domains/review/use-send-finding-to-agent.ts`. Follow `/frontend-design` and `/mobile-parity`.

## Output contract

Summary, files changed, tests run with results, blockers, risks, `status: done`, plan checkbox.
