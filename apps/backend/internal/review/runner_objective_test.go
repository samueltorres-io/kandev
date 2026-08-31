package review

import (
	"context"
	"errors"
	"testing"

	"github.com/kandev/kandev/internal/task/models"
	taskservice "github.com/kandev/kandev/internal/task/service"
)

// --- fakeStore extensions for the objective_check path ---

func (f *fakeStore) ActiveAssessmentRun(context.Context, string) (*models.TaskReviewRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.activeObjective, nil
}

func (f *fakeStore) FindAssessmentRunByEntryID(_ context.Context, entryID string) (*models.TaskReviewRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, run := range f.runs {
		if run.EntryID == entryID && run.Kind == models.ReviewKindObjectiveCheck {
			return run, nil
		}
	}
	return nil, nil
}

func (f *fakeStore) PublishAssessment(_ context.Context, req taskservice.PublishAssessmentRequest) (*models.TaskReviewRun, []*models.TaskObjectiveCriterion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.publishErr != nil {
		return nil, nil, f.publishErr
	}
	f.assessments = append(f.assessments, req)
	statuses := make([]models.ObjectiveCriterionStatus, 0, len(req.Criteria))
	crit := make([]*models.TaskObjectiveCriterion, 0, len(req.Criteria))
	for _, c := range req.Criteria {
		statuses = append(statuses, models.ObjectiveCriterionStatus(c.Status))
		crit = append(crit, &models.TaskObjectiveCriterion{Text: c.Text, Status: models.ObjectiveCriterionStatus(c.Status)})
	}
	run := f.runs[req.RunID]
	if run != nil {
		run.Verdict = models.RollupObjectiveVerdict(statuses)
	}
	return run, crit, nil
}

func (f *fakeStore) WriteFailedAssessmentGate(_ context.Context, run *models.TaskReviewRun, gate bool, note string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gateFailures = append(f.gateFailures, fakeGateFailure{runID: idOf(run), gate: gate, note: note})
}

func idOf(run *models.TaskReviewRun) string {
	if run == nil {
		return ""
	}
	return run.ID
}

type fakeGateFailure struct {
	runID string
	gate  bool
	note  string
}

// --- objective fakes ---

type fakeObjectivePrompts struct{ err error }

func (f *fakeObjectivePrompts) BuildObjective(_ context.Context, oc ObjectiveContext, _ []ChangedFile, _ PromptContext) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "assess: " + oc.ObjectiveText, nil
}

type fakeDocuments struct {
	docs []ObjectiveDoc
	err  error
}

func (f *fakeDocuments) ObjectiveDocuments(context.Context, string) ([]ObjectiveDoc, error) {
	return f.docs, f.err
}

func newObjectiveHarness(t *testing.T, files map[string]any, response string, taskCtx PromptContext, docs []ObjectiveDoc) *runnerHarness {
	t.Helper()
	store := newFakeStore()
	inference := &fakeInference{responses: []string{response}}
	changes := &fakeChangeSource{uncommitted: files}
	runner := NewRunner(RunnerDeps{
		Store:            store,
		Resolver:         NewResolver(nil, &fakeUtility{found: true, enabled: true, agentID: "claude-acp", model: "haiku"}, nil),
		Changes:          changes,
		Inference:        inference,
		Prompts:          &fakePrompts{},
		ObjectivePrompts: &fakeObjectivePrompts{},
		Documents:        &fakeDocuments{docs: docs},
		TaskContext:      &fakeTaskContext{ctx: taskCtx},
		Sessions:         &fakeSessions{sessionID: "sess-1"},
		Logger:           testLogger(t),
	})
	runner.Start(context.Background())
	t.Cleanup(runner.Stop)
	return &runnerHarness{runner: runner, store: store, inference: inference, changes: changes}
}

const objectiveResponse = "```json\n{\"verdict\":\"met\",\"summary\":\"done\"," +
	"\"criteria\":[{\"text\":\"signs in\",\"status\":\"met\",\"rationale\":\"ok\"}," +
	"{\"text\":\"rate limited\",\"status\":\"unmet\",\"rationale\":\"missing\"}]}\n```"

func TestRunner_ObjectiveHappyPath(t *testing.T) {
	h := newObjectiveHarness(t,
		map[string]any{"a.go": fileEntry("a.go", "@@ -1 +1,2 @@\n old\n+new\n", "", "")},
		objectiveResponse,
		PromptContext{TaskTitle: "Login", TaskDescription: "Users sign in."},
		nil,
	)

	run, err := h.runner.Run(context.Background(), RunRequest{
		TaskID: "task-1", Kind: models.ReviewKindObjectiveCheck, Trigger: models.ReviewTriggerManual,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if h.store.statusOf(run.ID) != models.ReviewRunCompleted {
		t.Fatalf("expected completed, got %q", h.store.statusOf(run.ID))
	}
	if len(h.store.assessments) != 1 || len(h.store.assessments[0].Criteria) != 2 {
		t.Fatalf("expected one assessment with 2 criteria, got %+v", h.store.assessments)
	}
	if run.Verdict != models.ObjectiveVerdictPartial {
		t.Fatalf("expected computed partial verdict, got %q", run.Verdict)
	}
	completed, _ := h.store.lastCompleted()
	if completed.FindingCount != 2 {
		t.Fatalf("expected criterion count 2, got %d", completed.FindingCount)
	}
}

func TestRunner_ObjectiveNoChangesCreatesNoRun(t *testing.T) {
	h := newObjectiveHarness(t, nil, objectiveResponse, PromptContext{TaskDescription: "d"}, nil)
	_, err := h.runner.Run(context.Background(), RunRequest{
		TaskID: "task-1", Kind: models.ReviewKindObjectiveCheck, Trigger: models.ReviewTriggerManual,
	})
	if !errors.Is(err, ErrNoChanges) || CodeForObjective(err) != CodeObjectiveNoChanges {
		t.Fatalf("expected objective_no_changes, got %v", err)
	}
	if h.store.runCount() != 0 {
		t.Fatalf("expected no run created, got %d", h.store.runCount())
	}
}

func TestRunner_ObjectiveNoObjectiveFailsRun(t *testing.T) {
	h := newObjectiveHarness(t,
		map[string]any{"a.go": fileEntry("a.go", "d", "", "")},
		objectiveResponse,
		PromptContext{}, // no title, no description
		nil,
	)
	run, err := h.runner.Run(context.Background(), RunRequest{
		TaskID: "task-1", Kind: models.ReviewKindObjectiveCheck, Trigger: models.ReviewTriggerManual,
	})
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if h.store.statusOf(run.ID) != models.ReviewRunFailed {
		t.Fatalf("expected failed run, got %q", h.store.statusOf(run.ID))
	}
	f, _ := h.store.lastFailure()
	if f.code != CodeObjectiveNoObjective {
		t.Fatalf("expected objective_no_objective, got %q", f.code)
	}
}

func TestRunner_ObjectiveUnparseableFailsRun(t *testing.T) {
	h := newObjectiveHarness(t,
		map[string]any{"a.go": fileEntry("a.go", "d", "", "")},
		"I think the task looks done.",
		PromptContext{TaskDescription: "d"},
		nil,
	)
	run, _ := h.runner.Run(context.Background(), RunRequest{
		TaskID: "task-1", Kind: models.ReviewKindObjectiveCheck, Trigger: models.ReviewTriggerManual,
	})
	f, ok := h.store.lastFailure()
	if !ok || f.code != CodeObjectiveUnparseableResponse {
		t.Fatalf("expected objective_unparseable_response, got %+v (status %q)", f, h.store.statusOf(run.ID))
	}
}

func TestRunner_ObjectiveGateRejectOnFailure(t *testing.T) {
	// Unparseable + gated workflow-step run -> a reject gate decision.
	h := newObjectiveHarness(t,
		map[string]any{"a.go": fileEntry("a.go", "d", "", "")},
		"no json here",
		PromptContext{TaskDescription: "d"},
		nil,
	)
	_, _ = h.runner.Run(context.Background(), RunRequest{
		TaskID: "task-1", Kind: models.ReviewKindObjectiveCheck,
		Trigger: models.ReviewTriggerWorkflowStep, WorkflowStepID: "step-1", Gate: true,
	})
	if len(h.store.gateFailures) != 1 || !h.store.gateFailures[0].gate {
		t.Fatalf("expected one failed-gate write, got %+v", h.store.gateFailures)
	}

	// no-objective must NOT write a gate decision (a missing description never pins a workflow).
	h2 := newObjectiveHarness(t,
		map[string]any{"a.go": fileEntry("a.go", "d", "", "")},
		objectiveResponse,
		PromptContext{},
		nil,
	)
	_, _ = h2.runner.Run(context.Background(), RunRequest{
		TaskID: "task-1", Kind: models.ReviewKindObjectiveCheck,
		Trigger: models.ReviewTriggerWorkflowStep, WorkflowStepID: "step-1", Gate: true,
	})
	if len(h2.store.gateFailures) != 0 {
		t.Fatalf("no-objective must not write a gate decision, got %+v", h2.store.gateFailures)
	}
}
