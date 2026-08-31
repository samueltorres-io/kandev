package service

import (
	"context"
	"errors"
	"testing"

	"github.com/kandev/kandev/internal/events"
	"github.com/kandev/kandev/internal/task/models"
)

func crit(text string, status models.ObjectiveCriterionStatus) ObjectiveCriterionInput {
	return ObjectiveCriterionInput{Text: text, Status: string(status), Rationale: "why"}
}

type fakeGateWriter struct {
	taskID, stepID, decision, note string
	calls                          int
}

func (f *fakeGateWriter) WriteObjectiveGateDecision(_ context.Context, taskID, stepID, decision, note string) error {
	f.calls++
	f.taskID, f.stepID, f.decision, f.note = taskID, stepID, decision, note
	return nil
}

func TestPublishAssessment_MCPPathComputesVerdict(t *testing.T) {
	svc, eventBus, repo := createTestReviewService(t)
	ctx := context.Background()
	seedTask(t, ctx, repo, "task-o1")

	run, criteria, err := svc.PublishAssessment(ctx, PublishAssessmentRequest{
		TaskID:  "task-o1",
		Trigger: models.ReviewTriggerAgent,
		Summary: "one met, one not",
		Criteria: []ObjectiveCriterionInput{
			crit("signs in", models.ObjectiveCriterionMet),
			crit("rate limited", models.ObjectiveCriterionUnmet),
		},
	})
	if err != nil {
		t.Fatalf("PublishAssessment: %v", err)
	}
	if run.Kind != models.ReviewKindObjectiveCheck || run.Status != models.ReviewRunCompleted {
		t.Fatalf("expected completed objective run, got %+v", run)
	}
	if run.Verdict != models.ObjectiveVerdictPartial {
		t.Fatalf("expected computed partial verdict, got %q", run.Verdict)
	}
	if len(criteria) != 2 || run.FindingCount != 2 {
		t.Fatalf("expected 2 criteria counted, got %d / %d", len(criteria), run.FindingCount)
	}
	if countEvents(eventBus.GetPublishedEvents(), events.TaskObjectivePublished) != 1 {
		t.Fatalf("expected one published event, got %v", eventTypes(eventBus.GetPublishedEvents()))
	}
}

func TestPublishAssessment_VerdictRollupTable(t *testing.T) {
	cases := []struct {
		name string
		in   []ObjectiveCriterionInput
		want models.ObjectiveVerdict
	}{
		{"all met", []ObjectiveCriterionInput{crit("a", models.ObjectiveCriterionMet), crit("b", models.ObjectiveCriterionMet)}, models.ObjectiveVerdictMet},
		{"mixed", []ObjectiveCriterionInput{crit("a", models.ObjectiveCriterionMet), crit("b", models.ObjectiveCriterionUnknown)}, models.ObjectiveVerdictPartial},
		{"none met", []ObjectiveCriterionInput{crit("a", models.ObjectiveCriterionPartial), crit("b", models.ObjectiveCriterionUnmet)}, models.ObjectiveVerdictUnmet},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, _, repo := createTestReviewService(t)
			ctx := context.Background()
			seedTask(t, ctx, repo, "task-"+c.name)
			run, _, err := svc.PublishAssessment(ctx, PublishAssessmentRequest{TaskID: "task-" + c.name, Criteria: c.in})
			if err != nil {
				t.Fatalf("PublishAssessment: %v", err)
			}
			if run.Verdict != c.want {
				t.Fatalf("got %q want %q", run.Verdict, c.want)
			}
		})
	}
}

func TestPublishAssessment_RejectsEmptyAndMalformed(t *testing.T) {
	svc, _, repo := createTestReviewService(t)
	ctx := context.Background()
	seedTask(t, ctx, repo, "task-o2")

	if _, _, err := svc.PublishAssessment(ctx, PublishAssessmentRequest{TaskID: "task-o2"}); !errors.Is(err, ErrNoCriteria) {
		t.Fatalf("expected ErrNoCriteria, got %v", err)
	}

	_, _, err := svc.PublishAssessment(ctx, PublishAssessmentRequest{
		TaskID: "task-o2",
		Criteria: []ObjectiveCriterionInput{
			crit("ok", models.ObjectiveCriterionMet),
			{Text: "bad", Status: "definitely-not-a-status"},
		},
	})
	if !errors.Is(err, ErrInvalidCriterion) {
		t.Fatalf("expected ErrInvalidCriterion, got %v", err)
	}
	// Nothing persisted on rejection.
	got, err := svc.GetTaskAssessment(ctx, "task-o2")
	if err != nil {
		t.Fatalf("GetTaskAssessment: %v", err)
	}
	if len(got.Runs) != 0 {
		t.Fatalf("expected no runs after a rejected publish, got %d", len(got.Runs))
	}
}

func TestPublishAssessment_RunnerPathKeepsVerdictThroughCompleteRun(t *testing.T) {
	svc, _, repo := createTestReviewService(t)
	ctx := context.Background()
	seedTask(t, ctx, repo, "task-o3")

	run, err := svc.CreateAssessmentRun(ctx, CreateRunRequest{TaskID: "task-o3", Trigger: models.ReviewTriggerManual})
	if err != nil {
		t.Fatalf("CreateAssessmentRun: %v", err)
	}
	if _, err := svc.MarkRunRunning(ctx, run.ID); err != nil {
		t.Fatalf("MarkRunRunning: %v", err)
	}
	if _, _, err := svc.PublishAssessment(ctx, PublishAssessmentRequest{
		TaskID: "task-o3", RunID: run.ID,
		Criteria: []ObjectiveCriterionInput{crit("a", models.ObjectiveCriterionMet)},
	}); err != nil {
		t.Fatalf("PublishAssessment: %v", err)
	}
	done, err := svc.CompleteRun(ctx, CompleteRunRequest{RunID: run.ID, FindingCount: 1})
	if err != nil {
		t.Fatalf("CompleteRun: %v", err)
	}
	if done.Verdict != models.ObjectiveVerdictMet || done.Status != models.ReviewRunCompleted {
		t.Fatalf("expected verdict preserved through CompleteRun, got %+v", done)
	}
}

func TestGetAndClearTaskAssessment(t *testing.T) {
	svc, eventBus, repo := createTestReviewService(t)
	ctx := context.Background()
	seedTask(t, ctx, repo, "task-o4")

	if _, _, err := svc.PublishAssessment(ctx, PublishAssessmentRequest{
		TaskID:   "task-o4",
		Criteria: []ObjectiveCriterionInput{crit("a", models.ObjectiveCriterionMet)},
	}); err != nil {
		t.Fatalf("PublishAssessment: %v", err)
	}

	got, err := svc.GetTaskAssessment(ctx, "task-o4")
	if err != nil {
		t.Fatalf("GetTaskAssessment: %v", err)
	}
	if len(got.Runs) != 1 || len(got.Criteria) != 1 || got.Verdict != models.ObjectiveVerdictMet {
		t.Fatalf("unexpected assessment: %+v", got)
	}

	// A native code review on the same task must survive ClearTaskAssessment.
	cr, err := svc.CreateRun(ctx, CreateRunRequest{TaskID: "task-o4", Trigger: models.ReviewTriggerManual})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	if err := svc.ClearTaskAssessment(ctx, "task-o4"); err != nil {
		t.Fatalf("ClearTaskAssessment: %v", err)
	}
	if countEvents(eventBus.GetPublishedEvents(), events.TaskObjectiveCleared) != 1 {
		t.Fatalf("expected one cleared event")
	}
	after, err := svc.GetTaskAssessment(ctx, "task-o4")
	if err != nil {
		t.Fatalf("GetTaskAssessment: %v", err)
	}
	if len(after.Runs) != 0 {
		t.Fatalf("expected assessment cleared, got %d runs", len(after.Runs))
	}
	review, err := svc.GetTaskReview(ctx, "task-o4")
	if err != nil {
		t.Fatalf("GetTaskReview: %v", err)
	}
	if len(review.Runs) != 1 || review.Runs[0].ID != cr.ID {
		t.Fatalf("code review run must survive assessment clear, got %+v", review.Runs)
	}
}

func TestPublishAssessment_GateDecisionWrittenOnWorkflowTrigger(t *testing.T) {
	svc, _, repo := createTestReviewService(t)
	ctx := context.Background()
	seedTask(t, ctx, repo, "task-o5")
	writer := &fakeGateWriter{}
	svc.SetObjectiveGateWriter(writer)

	run, err := svc.CreateAssessmentRun(ctx, CreateRunRequest{
		TaskID: "task-o5", Trigger: models.ReviewTriggerWorkflowStep, WorkflowStepID: "step-1",
	})
	if err != nil {
		t.Fatalf("CreateAssessmentRun: %v", err)
	}
	if _, _, err := svc.PublishAssessment(ctx, PublishAssessmentRequest{
		TaskID: "task-o5", RunID: run.ID, WorkflowStepID: "step-1", Gate: true,
		Criteria: []ObjectiveCriterionInput{crit("a", models.ObjectiveCriterionUnmet)},
	}); err != nil {
		t.Fatalf("PublishAssessment: %v", err)
	}
	if writer.calls != 1 || writer.decision != "reject" || writer.stepID != "step-1" {
		t.Fatalf("expected one reject decision for step-1, got %+v", writer)
	}

	// An ungated run writes nothing.
	writer.calls = 0
	run2, _ := svc.CreateAssessmentRun(ctx, CreateRunRequest{
		TaskID: "task-o5", Trigger: models.ReviewTriggerWorkflowStep, WorkflowStepID: "step-2",
	})
	if _, _, err := svc.PublishAssessment(ctx, PublishAssessmentRequest{
		TaskID: "task-o5", RunID: run2.ID, WorkflowStepID: "step-2", Gate: false,
		Criteria: []ObjectiveCriterionInput{crit("a", models.ObjectiveCriterionMet)},
	}); err != nil {
		t.Fatalf("PublishAssessment: %v", err)
	}
	if writer.calls != 0 {
		t.Fatalf("ungated run must not write a decision, got %d calls", writer.calls)
	}
}
