package review

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBuildObjectiveContext_DeriveFromDescription(t *testing.T) {
	ctx, err := BuildObjectiveContext(ObjectiveTask{
		Title:       "Add login",
		Description: "Users must be able to sign in with email and password.",
	}, nil)
	if err != nil {
		t.Fatalf("BuildObjectiveContext: %v", err)
	}
	if !ctx.DeriveCriteria || len(ctx.Criteria) != 0 {
		t.Fatalf("bare description should derive criteria, got %+v", ctx)
	}
	if ctx.ObjectiveText == "" {
		t.Fatalf("expected objective text")
	}
}

func TestBuildObjectiveContext_NoObjective(t *testing.T) {
	_, err := BuildObjectiveContext(ObjectiveTask{}, []ObjectiveDoc{{Kind: "notes", Body: "irrelevant"}})
	if !errors.Is(err, ErrNoObjective) {
		t.Fatalf("expected ErrNoObjective, got %v", err)
	}
}

func TestBuildObjectiveContext_AcceptanceCriteriaFromPlan(t *testing.T) {
	plan := `# Plan

## Acceptance criteria

- **AC-AGENTS-X-001.1:** A user can sign in with email and password.
- AC-AGENTS-X-001.2: The endpoint is rate limited.
- A plain bullet that is still a criterion.

## Other section

- not a criterion
`
	ctx, err := BuildObjectiveContext(ObjectiveTask{Description: "d"}, []ObjectiveDoc{
		{Kind: "plan", Body: plan, UpdatedAt: time.Now()},
	})
	if err != nil {
		t.Fatalf("BuildObjectiveContext: %v", err)
	}
	if ctx.DeriveCriteria || len(ctx.Criteria) != 3 {
		t.Fatalf("expected 3 document criteria, got %+v", ctx.Criteria)
	}
	if ctx.Criteria[0].SourceRef != "AC-AGENTS-X-001.1" {
		t.Fatalf("expected AC ref extracted, got %q", ctx.Criteria[0].SourceRef)
	}
	if ctx.Criteria[0].Text != "A user can sign in with email and password." {
		t.Fatalf("expected label stripped from text, got %q", ctx.Criteria[0].Text)
	}
	if ctx.Criteria[2].SourceRef != "" {
		t.Fatalf("plain bullet under the heading should have no ref, got %q", ctx.Criteria[2].SourceRef)
	}
}

func TestBuildObjectiveContext_NewestDocWins(t *testing.T) {
	old := ObjectiveDoc{Kind: "spec", Body: "old spec body", UpdatedAt: time.Now().Add(-time.Hour)}
	fresh := ObjectiveDoc{Kind: "spec", Body: "fresh spec body", UpdatedAt: time.Now()}
	ctx, err := BuildObjectiveContext(ObjectiveTask{Description: "d"}, []ObjectiveDoc{old, fresh})
	if err != nil {
		t.Fatalf("BuildObjectiveContext: %v", err)
	}
	if !strings.Contains(ctx.ObjectiveText, "fresh spec body") || strings.Contains(ctx.ObjectiveText, "old spec body") {
		t.Fatalf("expected only the newest spec, got %q", ctx.ObjectiveText)
	}
}
