---
id: "10-mobile-e2e-docs"
title: "Mobile parity, E2E, mock-agent scenario, public docs"
status: pending
wave: 6
depends_on: ["08-objective-panel"]
plan: "plan.md"
requirements:
  - REQ-AGENTS-OBJECTIVE-ASSESSMENT-001
acceptance_criteria:
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.1
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.7
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.12
system_design: "../../specs/agents/system-design/objective-assessment.md"
---

# Task 10: Mobile, E2E, mock agent, docs

## Summary

Native mobile presentation of the assessment, hermetic E2E for both triggers, the
mock-agent assessment response, and public docs.

## Scope

- `apps/web/components/task/mobile/` — the verdict banner + criterion checklist as a bottom sheet from the mobile Changes panel, with the full desktop per-criterion action set (Send to agent, evidence jump). Run control in the mobile changes toolbar. Follow `/mobile-parity`.
- `apps/backend/cmd/mock-agent/handler.go` — `isObjectiveCheckRequest(prompt)` matching `KANDEV_OBJECTIVE_CHECK_REQUEST`, returning a deterministic fenced-JSON assessment (a `partial` verdict with one `met` and one `unmet` criterion). Rebuild with `make -C apps/backend build-mock-agent`.
- `apps/web/e2e/objective/objective-on-demand.spec.ts` — GIVEN changed files + a task description, WHEN "Assess objective", THEN a run completes and the verdict banner + criterion rows render; Send-to-agent posts context; the no-objective case shows the inline message.
- `apps/web/e2e/objective/objective-workflow-gate.spec.ts` — GIVEN a step with a gated `run_objective_check`, WHEN a task enters it and the mock returns `partial`, THEN the outbound transition is blocked and the step shows why; WHEN re-assessed and the mock returns all-`met`, THEN the `reject` is replaced by `approve` and the transition proceeds.
- `docs/public/**` — a section under the review/agents docs describing objective assessment: what it reads (task description + `plan` / `spec` docs, and the **v1 limitation** that there is no link to `docs/specs/**`), the three triggers, the three verdict meanings, the advisory nature, and the workflow gate (including that a gated step does not block when there is no objective). Use `/docs-maintainer`.

## Exclusions

None.

## Implementation acceptance conditions

- Both E2E specs pass hermetically with the mock agent.
- Mobile bottom sheet exposes every desktop per-criterion action.
- `pnpm run i18n:check` and `python3 scripts/lint-spec-files.py --all` pass.

## Verification

```
cd apps/web && pnpm e2e:raw -- objective
cd apps/web && pnpm run i18n:check
python3 scripts/lint-spec-files.py --all
```

## Likely files / risks

`apps/web/components/task/mobile/*`, `apps/backend/cmd/mock-agent/handler.go`, `apps/web/e2e/objective/*`, `docs/public/**`.
Risk: the gate E2E needs the mock to return different verdicts across runs — use a prompt-count or scenario switch in the mock handler.

## Output contract

Summary, files changed, tests run with results, blockers, risks, `status: done`, plan checkbox.
