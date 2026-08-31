---
id: "03-objective-extraction-and-parser"
title: "Objective + criteria extraction and assessment-response parser"
status: pending
wave: 2
depends_on: ["01-schema-and-repository"]
plan: "plan.md"
requirements:
  - REQ-AGENTS-OBJECTIVE-ASSESSMENT-001
acceptance_criteria:
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.2
  - AC-AGENTS-OBJECTIVE-ASSESSMENT-001.3
system_design: "../../specs/agents/system-design/objective-assessment.md"
---

# Task 03: Objective extraction + assessment parser

## Summary

Pure functions: turn a task + its documents into an objective/criteria prompt
context, and parse the agent's assessment JSON back into typed values.

## Scope

- `internal/review/objective.go`:
  - `BuildObjectiveContext(task TaskContext, docs []TaskDoc) (ObjectiveContext, error)` — objective text = description + newest `plan` + newest `spec` doc bodies, each capped, total ≤ ~60 KB. Returns `ErrNoObjective` when the result is empty/whitespace.
  - Acceptance-criteria detection: a `## Acceptance criteria` / `### Acceptance` heading followed by list items, or list items matching `**AC-...:**` / `AC-...:`. Extracted items become `PredefinedCriterion{Text, SourceRef}` (`SourceRef` = the `AC-*` id when matched). Cap at 12; if none found, `ObjectiveContext.DeriveCriteria = true`.
- `internal/review/parse.go` (or a new `parse_objective.go`) — `ParseAssessment(response string) (AssessmentResult, error)`:
  - Accept a fenced ` ```json ` block or a bare `{...}` with prose around it (reuse the existing `ParseFindings` JSON-extraction helper).
  - `AssessmentResult{Verdict, Summary, Criteria []ParsedCriterion, RejectedCount int}`.
  - Drop a criterion missing `text` or `status`, or with an unknown status; count it.
  - `ErrUnparseableResponse` when zero criteria are recoverable.
  - Do NOT trust the response's `verdict` — leave rollup to the service (task 02); still surface the agent's stated verdict for logging.

## Exclusions

Prompt template content (task 04), inference call, persistence.

## Implementation acceptance conditions

- A `plan` doc with an `## Acceptance criteria` list yields `document`-source criteria with `AC-*` refs; a bare description yields `DeriveCriteria = true`.
- Fenced JSON, bare JSON, and prose-wrapped JSON all parse; a response with no JSON object returns `ErrUnparseableResponse`.
- One malformed criterion is dropped and counted; the rest parse.

## Verification

```
cd apps/backend && go test ./internal/review/... -run 'Objective|Assessment|ParseAssessment'
```

## Likely files / risks

`internal/review/{objective.go,objective_test.go,parse_objective.go,parse_objective_test.go}`.
Reuse the JSON-extraction helper from `internal/review/parse.go`; do not duplicate it.

## Output contract

Summary, files changed, tests run with results, blockers, risks, `status: done`, plan checkbox.
