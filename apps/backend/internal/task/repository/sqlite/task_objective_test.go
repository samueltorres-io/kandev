package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/kandev/kandev/internal/task/models"
)

func newObjectiveRun(t *testing.T, ctx context.Context, repo *Repository, taskID string) *models.TaskReviewRun {
	t.Helper()
	run := &models.TaskReviewRun{
		TaskID:  taskID,
		Kind:    models.ReviewKindObjectiveCheck,
		Trigger: models.ReviewTriggerManual,
		AgentID: "claude-acp",
		Model:   "claude-haiku-4-5",
	}
	if err := repo.CreateTaskReviewRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskReviewRun (objective): %v", err)
	}
	return run
}

func criterion(runID, taskID, text string, ordinal int, status models.ObjectiveCriterionStatus) *models.TaskObjectiveCriterion {
	return &models.TaskObjectiveCriterion{
		RunID:     runID,
		TaskID:    taskID,
		Ordinal:   ordinal,
		Source:    models.ObjectiveSourceDerived,
		Text:      text,
		Status:    status,
		Rationale: "because",
		Evidence:  []models.EvidencePointer{{File: "a.go", Line: ordinal + 1}},
	}
}

func TestObjectiveRun_KindAndVerdictRoundTrip(t *testing.T) {
	repo := newRepoForSessionTests(t)
	ctx := context.Background()
	seedReviewTask(t, ctx, repo, "task-obj")

	run := newObjectiveRun(t, ctx, repo, "task-obj")
	if run.Kind != models.ReviewKindObjectiveCheck {
		t.Fatalf("expected objective_check kind, got %q", run.Kind)
	}

	run.Status = models.ReviewRunCompleted
	run.Verdict = models.ObjectiveVerdictPartial
	run.FindingCount = 2
	run.Summary = "one met, one not"
	if err := repo.UpdateTaskReviewRun(ctx, run); err != nil {
		t.Fatalf("UpdateTaskReviewRun: %v", err)
	}

	got, err := repo.GetTaskReviewRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetTaskReviewRun: %v", err)
	}
	if got.Kind != models.ReviewKindObjectiveCheck || got.Verdict != models.ObjectiveVerdictPartial {
		t.Fatalf("kind/verdict round-trip mismatch: %+v", got)
	}
}

func TestObjectiveCriteria_CreateListCascade(t *testing.T) {
	repo := newRepoForSessionTests(t)
	ctx := context.Background()
	seedReviewTask(t, ctx, repo, "task-crit")
	run := newObjectiveRun(t, ctx, repo, "task-crit")

	batch := []*models.TaskObjectiveCriterion{
		criterion(run.ID, "task-crit", "second", 1, models.ObjectiveCriterionUnmet),
		criterion(run.ID, "task-crit", "first", 0, models.ObjectiveCriterionMet),
	}
	if err := repo.CreateTaskObjectiveCriteria(ctx, batch); err != nil {
		t.Fatalf("CreateTaskObjectiveCriteria: %v", err)
	}

	got, err := repo.ListTaskObjectiveCriteria(ctx, "task-crit", run.ID)
	if err != nil {
		t.Fatalf("ListTaskObjectiveCriteria: %v", err)
	}
	if len(got) != 2 || got[0].Text != "first" || got[1].Text != "second" {
		t.Fatalf("expected ordinal ordering, got %+v", got)
	}
	if len(got[0].Evidence) != 1 || got[0].Evidence[0].File != "a.go" {
		t.Fatalf("evidence did not round-trip: %+v", got[0].Evidence)
	}

	// Deleting the run cascades its criteria.
	if err := repo.DeleteTaskReviewByTask(ctx, "task-crit"); err != nil {
		t.Fatalf("DeleteTaskReviewByTask: %v", err)
	}
	got, err = repo.ListTaskObjectiveCriteria(ctx, "task-crit", run.ID)
	if err != nil {
		t.Fatalf("ListTaskObjectiveCriteria after delete: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected criteria cleared, got %d", len(got))
	}
}

func TestObjectiveCriteria_CascadesOnTaskDelete(t *testing.T) {
	repo := newRepoForSessionTests(t)
	ctx := context.Background()
	seedReviewTask(t, ctx, repo, "task-crit-cascade")
	run := newObjectiveRun(t, ctx, repo, "task-crit-cascade")
	if err := repo.CreateTaskObjectiveCriteria(ctx, []*models.TaskObjectiveCriterion{
		criterion(run.ID, "task-crit-cascade", "c", 0, models.ObjectiveCriterionMet),
	}); err != nil {
		t.Fatalf("CreateTaskObjectiveCriteria: %v", err)
	}
	if err := repo.DeleteTask(ctx, "task-crit-cascade"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	got, err := repo.ListTaskObjectiveCriteria(ctx, "task-crit-cascade", run.ID)
	if err != nil {
		t.Fatalf("ListTaskObjectiveCriteria: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected criteria removed with the task, got %d", len(got))
	}
}

func TestObjectiveCriterion_GetMissingSentinel(t *testing.T) {
	repo := newRepoForSessionTests(t)
	if _, err := repo.GetTaskObjectiveCriterion(context.Background(), "nope"); !errors.Is(err, models.ErrTaskObjectiveCriterionNotFound) {
		t.Fatalf("expected ErrTaskObjectiveCriterionNotFound, got %v", err)
	}
}

// TestObjectiveReads_KindIsolation is the migration audit: with a mixed-kind run
// set for one task, every Native Code Review read must return only code_review
// rows and every objective read only objective_check rows.
func TestObjectiveReads_KindIsolation(t *testing.T) {
	repo := newRepoForSessionTests(t)
	ctx := context.Background()
	seedReviewTask(t, ctx, repo, "task-mix")

	cr := newReviewRun(t, ctx, repo, "task-mix") // pending code review
	obj := newObjectiveRun(t, ctx, repo, "task-mix")
	obj.Status = models.ReviewRunCompleted
	obj.Verdict = models.ObjectiveVerdictMet
	if err := repo.UpdateTaskReviewRun(ctx, obj); err != nil {
		t.Fatalf("complete objective run: %v", err)
	}
	activeObj := newObjectiveRun(t, ctx, repo, "task-mix") // pending objective

	crRuns, err := repo.ListTaskReviewRuns(ctx, "task-mix", 20)
	if err != nil {
		t.Fatalf("ListTaskReviewRuns: %v", err)
	}
	if len(crRuns) != 1 || crRuns[0].ID != cr.ID {
		t.Fatalf("code-review list leaked objective runs: %+v", crRuns)
	}

	crActive, err := repo.ListActiveTaskReviewRuns(ctx, "task-mix")
	if err != nil {
		t.Fatalf("ListActiveTaskReviewRuns: %v", err)
	}
	if len(crActive) != 1 || crActive[0].ID != cr.ID {
		t.Fatalf("code-review active list leaked objective runs: %+v", crActive)
	}

	objRuns, err := repo.ListTaskObjectiveRuns(ctx, "task-mix", 20)
	if err != nil {
		t.Fatalf("ListTaskObjectiveRuns: %v", err)
	}
	if len(objRuns) != 2 {
		t.Fatalf("expected 2 objective runs, got %d", len(objRuns))
	}
	for _, r := range objRuns {
		if r.Kind != models.ReviewKindObjectiveCheck {
			t.Fatalf("objective list leaked a %q run", r.Kind)
		}
	}

	objActive, err := repo.ListActiveTaskObjectiveRuns(ctx, "task-mix")
	if err != nil {
		t.Fatalf("ListActiveTaskObjectiveRuns: %v", err)
	}
	if len(objActive) != 1 || objActive[0].ID != activeObj.ID {
		t.Fatalf("objective active list mismatch: %+v", objActive)
	}

	latest, err := repo.GetLatestCompletedTaskObjectiveRun(ctx, "task-mix")
	if err != nil {
		t.Fatalf("GetLatestCompletedTaskObjectiveRun: %v", err)
	}
	if latest == nil || latest.ID != obj.ID {
		t.Fatalf("expected the completed objective run, got %+v", latest)
	}

	// CancelInFlight is kind-agnostic: it closes both pending kinds.
	cancelled, err := repo.CancelInFlightTaskReviewRuns(ctx)
	if err != nil {
		t.Fatalf("CancelInFlightTaskReviewRuns: %v", err)
	}
	if cancelled != 2 {
		t.Fatalf("expected the two pending runs cancelled, got %d", cancelled)
	}
}

func TestGetLatestCompletedTaskObjectiveRun_NoneReturnsNil(t *testing.T) {
	repo := newRepoForSessionTests(t)
	ctx := context.Background()
	seedReviewTask(t, ctx, repo, "task-none")
	newObjectiveRun(t, ctx, repo, "task-none") // pending, not completed

	got, err := repo.GetLatestCompletedTaskObjectiveRun(ctx, "task-none")
	if err != nil {
		t.Fatalf("GetLatestCompletedTaskObjectiveRun: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for a task with no completed objective run, got %+v", got)
	}
}
