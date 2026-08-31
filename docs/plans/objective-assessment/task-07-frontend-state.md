---
id: "07-frontend-state"
title: "Frontend types, API client, store slice, WS handlers"
status: done
wave: 5
depends_on: ["05-mcp-and-ws-actions"]
plan: "plan.md"
requirements:
  - REQ-AGENTS-OBJECTIVE-ASSESSMENT-001
acceptance_criteria:
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.10
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.11
system_design: "../../specs/agents/system-design/objective-assessment.md"
---

# Task 07: Frontend state layer

## Summary

Types, WS API client, Zustand slice, and WS event handlers for objective
assessments. Mirrors `lib/state/slices/review/`.

## Scope

- `apps/web/lib/types/objective.ts` — `ObjectiveVerdict` (`'met' | 'partial' | 'unmet'`), `ObjectiveCriterionStatus`, `ObjectiveCriterionSource`, `ObjectiveErrorCode`, `TaskObjectiveRun`, `TaskObjectiveCriterion` (with `evidence: EvidencePointer[]`), `TaskObjectiveAssessment`, `TaskObjectiveSnapshot`, and the WS event payload map.
- `apps/web/lib/api/domains/objective-api.ts` — `runObjectiveAssessment({taskId, sessionId?, agentProfileId?})`, `cancelObjectiveAssessment(runId)`, `getObjectiveAssessment(taskId)`. `ObjectiveRequestError` carrying `.code`.
- `apps/web/lib/state/slices/objective/` — `types.ts` (`assessmentsByTaskId`, `runsByTaskId`, `loadedTaskIds`; actions `setTaskAssessment`, `upsertAssessmentRun`, `clearTaskAssessmentState`), `objective-slice.ts` (`RUN_HISTORY_LIMIT = 20`, newest-first). Register in `lib/state/store.ts` + `default-state.ts`.
- `apps/web/lib/ws/handlers/objective.ts` + `lib/ws/router.ts` — `task.objective.run_updated` → `upsertAssessmentRun`, `task.objective.published` → `setTaskAssessment`, `task.objective.cleared` → `clearTaskAssessmentState`.
- `apps/web/hooks/domains/objective/use-task-objective.ts` — one-shot `getObjectiveAssessment` backfill on mount (guarded by `loadedTaskIds`); returns `{runs, assessment, activeRun, verdict}`.
- `apps/web/lib/objective/verdict.ts` — pure helpers: `verdictTone(verdict)`, `criterionTone(status)`, `unmetCount(criteria)`, `evidenceNavigable(pointer, changedFiles)`.
- i18n: all copy through `t()`; add keys in the five required locales (or `pnpm run i18n:zh-hant` for the pair).

## Exclusions

Rendering components (task 09).

## Implementation acceptance conditions

- Slice reducers: `setTaskAssessment` replaces the current assessment; `upsertAssessmentRun` keeps history capped and newest-first.
- WS handlers update the slice from each of the three events.
- `evidenceNavigable` returns false for a file not in the changed set.

## Verification

```
cd apps/web && pnpm run typecheck
cd apps/web && pnpm vitest run lib/state/slices/objective lib/ws/handlers/objective.test lib/objective
cd apps/web && pnpm run i18n:check
```

## Likely files / risks

`apps/web/lib/types/objective.ts`, `apps/web/lib/api/domains/objective-api.ts`, `apps/web/lib/state/slices/objective/*`, `apps/web/lib/ws/handlers/objective.ts`, `apps/web/hooks/domains/objective/use-task-objective.ts`, `apps/web/lib/objective/verdict.ts`.

## Output contract

Summary, files changed, tests run with results, blockers, risks, `status: done`, plan checkbox.
