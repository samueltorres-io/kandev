package repository

import (
	"context"
	"testing"

	"github.com/kandev/kandev/internal/workflow/models"
)

// TestDeleteStepDecisionsByRole_IsolatesRoles is the objective-check gate
// isolation check: deleting the reserved objective-check role's decisions must
// leave human quorum decisions under other roles untouched.
func TestDeleteStepDecisionsByRole_IsolatesRoles(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()
	step := newPhase2TestStep(t, repo, "Gate")
	const taskID = "task-gate"

	record := func(participant, role, decision string) {
		t.Helper()
		if err := repo.RecordStepDecision(ctx, &models.WorkflowStepDecision{
			TaskID: taskID, StepID: step.ID, ParticipantID: participant,
			Decision: decision, DeciderID: participant, Role: role,
		}); err != nil {
			t.Fatalf("record %s/%s: %v", participant, role, err)
		}
	}
	record("reviewer-1", "reviewer", "approved")
	record("objective-check:"+step.ID, "objective-check", "reject")

	n, err := repo.DeleteStepDecisionsByRole(ctx, taskID, step.ID, "objective-check")
	if err != nil {
		t.Fatalf("DeleteStepDecisionsByRole: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 objective-check decision deleted, got %d", n)
	}

	remaining, err := repo.ListStepDecisions(ctx, taskID, step.ID)
	if err != nil {
		t.Fatalf("ListStepDecisions: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Role != "reviewer" {
		t.Fatalf("expected only the human reviewer decision to survive, got %+v", remaining)
	}

	// Re-run replaces rather than appends: delete-then-insert leaves exactly one.
	record("objective-check:"+step.ID, "objective-check", "approve")
	if _, err := repo.DeleteStepDecisionsByRole(ctx, taskID, step.ID, "objective-check"); err != nil {
		t.Fatalf("second delete: %v", err)
	}
	record("objective-check:"+step.ID, "objective-check", "approve")
	after, _ := repo.ListStepDecisions(ctx, taskID, step.ID)
	objCount := 0
	for _, d := range after {
		if d.Role == "objective-check" {
			objCount++
		}
	}
	if objCount != 1 {
		t.Fatalf("expected exactly one active objective-check decision after a re-run, got %d", objCount)
	}
}

func TestDeleteStepDecisionsByRole_RejectsEmptyArgs(t *testing.T) {
	repo := setupTestRepo(t)
	if _, err := repo.DeleteStepDecisionsByRole(context.Background(), "", "s", "r"); err == nil {
		t.Fatal("expected an error for an empty task_id")
	}
}
