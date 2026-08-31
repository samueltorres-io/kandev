package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/kandev/kandev/internal/events"
	"github.com/kandev/kandev/internal/task/models"
)

// ErrNoCriteria is returned when a PublishAssessment call carries no criteria.
// The runner turns this into a failed run; the MCP tool surfaces it to the agent.
var ErrNoCriteria = errors.New("at least one objective criterion is required")

// ErrInvalidCriterion is returned when a submitted criterion fails structural
// validation. Validation is all-or-nothing, matching PublishFindings.
var ErrInvalidCriterion = errors.New("invalid objective criterion")

const (
	maxCriterionText      = 2000
	maxCriterionRationale = 4000
	objFieldVerdict       = "verdict"
	objFieldSummary       = "summary"
	objFieldCriteria      = "criteria"
	gateDecisionApprove   = "approve"
	gateDecisionReject    = "reject"
)

// ObjectiveCriterionInput is one criterion result as submitted, before the row
// id and timestamps are attached. Mirrors the assessment JSON contract and the
// publish_objective_assessment_kandev MCP tool schema.
type ObjectiveCriterionInput struct {
	Text      string
	SourceRef string
	Source    string
	Status    string
	Rationale string
	Evidence  []models.EvidencePointer
}

// PublishAssessmentRequest carries a completed assessment to store. It has no
// Verdict field: the service always computes the stored verdict from the
// criterion statuses by the fixed rule.
type PublishAssessmentRequest struct {
	TaskID         string
	RunID          string
	SessionID      string
	Trigger        models.ReviewRunTrigger
	Summary        string
	Criteria       []ObjectiveCriterionInput
	WorkflowStepID string
	Gate           bool
}

// TaskAssessment is a task's objective-assessment state for the Review surface.
type TaskAssessment struct {
	Runs     []*models.TaskReviewRun          `json:"runs"`
	Criteria []*models.TaskObjectiveCriterion `json:"criteria"`
	Verdict  models.ObjectiveVerdict          `json:"verdict"`
}

// CreateAssessmentRun records a pending objective_check run. Thin wrapper over
// CreateRun so the runner does not have to remember to set Kind.
func (s *ReviewService) CreateAssessmentRun(ctx context.Context, req CreateRunRequest) (*models.TaskReviewRun, error) {
	req.Kind = models.ReviewKindObjectiveCheck
	return s.CreateRun(ctx, req)
}

// ActiveAssessmentRun returns the task's newest in-flight objective_check run,
// or nil. Callers rejoin it instead of starting a duplicate.
func (s *ReviewService) ActiveAssessmentRun(ctx context.Context, taskID string) (*models.TaskReviewRun, error) {
	if taskID == "" {
		return nil, ErrTaskIDRequired
	}
	runs, err := s.repo.ListActiveTaskObjectiveRuns(ctx, taskID)
	if err != nil || len(runs) == 0 {
		return nil, err
	}
	return runs[0], nil
}

// FindAssessmentRunByEntryID returns the objective_check run created for a
// step-entry ledger id, or nil. Returns nil for a code_review run carrying that
// entry id so the two step actions never rejoin each other's run.
func (s *ReviewService) FindAssessmentRunByEntryID(ctx context.Context, entryID string) (*models.TaskReviewRun, error) {
	if entryID == "" {
		return nil, nil
	}
	run, err := s.repo.FindTaskReviewRunByEntryID(ctx, entryID)
	if err != nil || run == nil {
		return nil, err
	}
	if run.Kind.Normalized() != models.ReviewKindObjectiveCheck {
		return nil, nil
	}
	return run, nil
}

// PublishAssessment is the single write path for a completed objective
// assessment. It validates the criteria, computes the verdict, persists the
// criterion rows and the run's verdict, and fans out the published event. When
// RunID is empty it creates a synthetic completed run (the MCP path).
func (s *ReviewService) PublishAssessment(ctx context.Context, req PublishAssessmentRequest) (*models.TaskReviewRun, []*models.TaskObjectiveCriterion, error) {
	if req.TaskID == "" {
		return nil, nil, ErrTaskIDRequired
	}
	if err := s.authorize(ctx, req.TaskID); err != nil {
		return nil, nil, err
	}
	if len(req.Criteria) == 0 {
		return nil, nil, ErrNoCriteria
	}

	// Validate before creating any row so a malformed batch leaves no orphan run.
	criteria, statuses, err := buildObjectiveCriteria(req.TaskID, "", req.Criteria)
	if err != nil {
		return nil, nil, err
	}
	verdict := models.RollupObjectiveVerdict(statuses)

	run, err := s.resolveAssessmentRun(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range criteria {
		c.RunID = run.ID
	}

	if err := s.repo.DeleteTaskObjectiveCriteriaByRun(ctx, run.ID); err != nil {
		return nil, nil, err
	}
	if err := s.repo.CreateTaskObjectiveCriteria(ctx, criteria); err != nil {
		s.logger.Error("create objective criteria", zap.String(rvFieldTaskID, req.TaskID), zap.Error(err))
		return nil, nil, err
	}

	summary := strings.TrimSpace(req.Summary)
	run, err = s.mutateRun(ctx, run.ID, func(r *models.TaskReviewRun) {
		r.Verdict = verdict
		r.Summary = summary
		r.FindingCount = len(criteria)
	})
	if err != nil {
		return nil, nil, err
	}

	s.publishEvent(ctx, events.TaskObjectivePublished, map[string]any{
		rvFieldTaskID:    req.TaskID,
		rvFieldRunID:     run.ID,
		objFieldVerdict:  verdict,
		objFieldSummary:  summary,
		objFieldCriteria: criteria,
	})
	s.maybeWriteGateDecision(ctx, req, run, verdict, "")
	return run, criteria, nil
}

// resolveAssessmentRun returns the run the assessment belongs to, creating a
// completed synthetic one when the caller has none (the MCP path).
func (s *ReviewService) resolveAssessmentRun(ctx context.Context, req PublishAssessmentRequest) (*models.TaskReviewRun, error) {
	if req.RunID != "" {
		return s.repo.GetTaskReviewRun(ctx, req.RunID)
	}
	now := time.Now().UTC()
	trigger := req.Trigger
	if trigger == "" {
		trigger = models.ReviewTriggerAgent
	}
	run := &models.TaskReviewRun{
		TaskID:      req.TaskID,
		Kind:        models.ReviewKindObjectiveCheck,
		SessionID:   req.SessionID,
		Trigger:     trigger,
		Status:      models.ReviewRunCompleted,
		CompletedAt: &now,
	}
	if err := s.repo.CreateTaskReviewRun(ctx, run); err != nil {
		return nil, err
	}
	s.publishRun(ctx, run)
	return run, nil
}

// maybeWriteGateDecision records the synthetic step decision for a gated
// objective_check triggered by a workflow step. verdict == met -> approve,
// otherwise reject. failureNote is set only for a failed run.
func (s *ReviewService) maybeWriteGateDecision(ctx context.Context, req PublishAssessmentRequest, run *models.TaskReviewRun, verdict models.ObjectiveVerdict, failureNote string) {
	if !req.Gate || req.WorkflowStepID == "" || s.gateWriter == nil {
		return
	}
	if run.Trigger != models.ReviewTriggerWorkflowStep {
		return
	}
	decision := gateDecisionApprove
	if verdict != models.ObjectiveVerdictMet {
		decision = gateDecisionReject
	}
	if err := s.gateWriter.WriteObjectiveGateDecision(ctx, req.TaskID, req.WorkflowStepID, decision, failureNote); err != nil {
		s.logger.Error("write objective gate decision",
			zap.String(rvFieldTaskID, req.TaskID), zap.String("step_id", req.WorkflowStepID), zap.Error(err))
	}
}

// WriteFailedAssessmentGate records a reject decision for a gated objective_check
// that failed to complete (agent unavailable, unparseable result). The runner
// calls this after FailRun. A run with no objective (objective_no_objective)
// must NOT reach here — the runner skips it so a missing description never pins a
// workflow.
func (s *ReviewService) WriteFailedAssessmentGate(ctx context.Context, run *models.TaskReviewRun, gate bool, note string) {
	if run == nil {
		return
	}
	s.maybeWriteGateDecision(ctx, PublishAssessmentRequest{
		TaskID:         run.TaskID,
		WorkflowStepID: run.WorkflowStepID,
		Gate:           gate,
	}, run, models.ObjectiveVerdictUnmet, note)
}

// GetTaskAssessment returns a task's objective run history (newest first) plus
// the criteria of the latest completed run.
func (s *ReviewService) GetTaskAssessment(ctx context.Context, taskID string) (*TaskAssessment, error) {
	if taskID == "" {
		return nil, ErrTaskIDRequired
	}
	if err := s.authorize(ctx, taskID); err != nil {
		return nil, err
	}
	runs, err := s.repo.ListTaskObjectiveRuns(ctx, taskID, 0)
	if err != nil {
		return nil, err
	}
	out := &TaskAssessment{Runs: nonNilRuns(runs), Criteria: []*models.TaskObjectiveCriterion{}}
	latest, err := s.repo.GetLatestCompletedTaskObjectiveRun(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if latest != nil {
		criteria, listErr := s.repo.ListTaskObjectiveCriteria(ctx, taskID, latest.ID)
		if listErr != nil {
			return nil, listErr
		}
		if criteria != nil {
			out.Criteria = criteria
		}
		out.Verdict = latest.Verdict
	}
	return out, nil
}

// ClearTaskAssessment removes a task's objective runs and criteria, leaving
// Native Code Review state untouched.
func (s *ReviewService) ClearTaskAssessment(ctx context.Context, taskID string) error {
	if taskID == "" {
		return ErrTaskIDRequired
	}
	if err := s.authorize(ctx, taskID); err != nil {
		return err
	}
	if err := s.repo.DeleteTaskObjectiveByTask(ctx, taskID); err != nil {
		return err
	}
	s.publishEvent(ctx, events.TaskObjectiveCleared, map[string]any{rvFieldTaskID: taskID})
	return nil
}

// buildObjectiveCriteria validates and normalizes the submitted criteria,
// returning the rows to store and the parallel status slice for the verdict
// rollup. One malformed entry rejects the whole batch.
func buildObjectiveCriteria(taskID, runID string, in []ObjectiveCriterionInput) ([]*models.TaskObjectiveCriterion, []models.ObjectiveCriterionStatus, error) {
	criteria := make([]*models.TaskObjectiveCriterion, 0, len(in))
	statuses := make([]models.ObjectiveCriterionStatus, 0, len(in))
	for i, c := range in {
		row, err := buildObjectiveCriterion(taskID, runID, i, c)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: criterion %d: %s", ErrInvalidCriterion, i+1, err)
		}
		criteria = append(criteria, row)
		statuses = append(statuses, row.Status)
	}
	return criteria, statuses, nil
}

func buildObjectiveCriterion(taskID, runID string, idx int, c ObjectiveCriterionInput) (*models.TaskObjectiveCriterion, error) {
	text := strings.TrimSpace(c.Text)
	if text == "" {
		return nil, errors.New("text is required")
	}
	if len(text) > maxCriterionText {
		return nil, fmt.Errorf("text exceeds %d chars", maxCriterionText)
	}
	status := models.ObjectiveCriterionStatus(strings.ToLower(strings.TrimSpace(c.Status)))
	if !models.ValidObjectiveCriterionStatus(status) {
		return nil, fmt.Errorf("unknown status %q", c.Status)
	}
	rationale := strings.TrimSpace(c.Rationale)
	if len(rationale) > maxCriterionRationale {
		rationale = rationale[:maxCriterionRationale]
	}
	source := models.ObjectiveCriterionSource(strings.ToLower(strings.TrimSpace(c.Source)))
	if source != models.ObjectiveSourceDocument {
		source = models.ObjectiveSourceDerived
	}
	evidence := make([]models.EvidencePointer, 0, len(c.Evidence))
	for _, e := range c.Evidence {
		file := strings.TrimSpace(e.File)
		if file == "" {
			continue
		}
		evidence = append(evidence, models.EvidencePointer{
			Repo: strings.TrimSpace(e.Repo), File: file, Line: e.Line, LineEnd: e.LineEnd,
		})
	}
	return &models.TaskObjectiveCriterion{
		RunID:     runID,
		TaskID:    taskID,
		Ordinal:   idx,
		Source:    source,
		SourceRef: strings.TrimSpace(c.SourceRef),
		Text:      text,
		Status:    status,
		Rationale: rationale,
		Evidence:  evidence,
	}, nil
}
