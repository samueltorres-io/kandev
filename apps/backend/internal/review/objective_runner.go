package review

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/kandev/kandev/internal/task/models"
	taskservice "github.com/kandev/kandev/internal/task/service"
)

// ObjectiveDocumentLookup supplies the task's plan/spec documents for the
// objective context. Best-effort: a lookup failure degrades to
// description-only, not a run failure.
type ObjectiveDocumentLookup interface {
	ObjectiveDocuments(ctx context.Context, taskID string) ([]ObjectiveDoc, error)
}

// executeObjective runs one objective-assessment pass: build the objective
// context, one inference call, parse, publish the assessment, complete the run.
func (r *Runner) executeObjective(ctx context.Context, req RunRequest, runID string, identity ReviewerIdentity, files []ChangedFile) error {
	started := time.Now()
	if _, err := r.store.MarkRunRunning(ctx, runID); err != nil {
		return err
	}
	if r.objectivePrompts == nil {
		return r.failObjective(ctx, req, runID,
			fmt.Errorf("%w: no objective prompt builder is configured", ErrExecutionFailed), started)
	}

	promptCtx, _ := r.promptContext(ctx, req)
	oc, err := BuildObjectiveContext(
		ObjectiveTask{Title: promptCtx.TaskTitle, Description: promptCtx.TaskDescription},
		r.objectiveDocuments(ctx, req.TaskID),
	)
	if err != nil {
		return r.failObjective(ctx, req, runID, err, started)
	}

	prompt, err := r.objectivePrompts.BuildObjective(ctx, oc, files, promptCtx)
	if err != nil {
		return r.failObjective(ctx, req, runID, fmt.Errorf("%w: build prompt: %w", ErrExecutionFailed, err), started)
	}

	result, err := r.inference.Run(ctx, identity, req.SessionID, prompt)
	if err != nil {
		return r.failObjective(ctx, req, runID, fmt.Errorf("%w: %w", ErrExecutionFailed, err), started)
	}
	if result == nil {
		return r.failObjective(ctx, req, runID, fmt.Errorf("%w: assessor returned no result", ErrExecutionFailed), started)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		r.logger.Debug("objective run cancelled before publishing", zap.String("run_id", runID))
		return nil
	}

	parsed, err := ParseAssessment(result.Response)
	if err != nil {
		return r.failObjective(ctx, req, runID, err, started)
	}

	inputs := objectiveCriterionInputs(oc, parsed.Criteria)
	if _, _, err := r.store.PublishAssessment(ctx, taskservice.PublishAssessmentRequest{
		TaskID:         req.TaskID,
		RunID:          runID,
		SessionID:      req.SessionID,
		Trigger:        req.Trigger,
		Summary:        parsed.Summary,
		Criteria:       inputs,
		WorkflowStepID: req.WorkflowStepID,
		Gate:           req.Gate,
	}); err != nil {
		return r.failObjective(ctx, req, runID, err, started)
	}

	_, err = r.store.CompleteRun(ctx, taskservice.CompleteRunRequest{
		RunID:           runID,
		Summary:         parsed.Summary,
		FindingCount:    len(inputs),
		FileCount:       len(files),
		RepositoryCount: RepositoryCount(files),
		PromptTokens:    result.PromptTokens,
		ResponseTokens:  result.ResponseTokens,
		DurationMs:      int(time.Since(started).Milliseconds()),
	})
	return err
}

func (r *Runner) objectiveDocuments(ctx context.Context, taskID string) []ObjectiveDoc {
	if r.documents == nil {
		return nil
	}
	docs, err := r.documents.ObjectiveDocuments(ctx, taskID)
	if err != nil {
		r.logger.Warn("objective documents unavailable", zap.String("task_id", taskID), zap.Error(err))
		return nil
	}
	return docs
}

// failObjective records the failure with an objective_* code and, for a gated
// workflow-step run, writes the reject decision — except objective_no_objective,
// which must never pin a workflow.
func (r *Runner) failObjective(ctx context.Context, req RunRequest, runID string, cause error, started time.Time) error {
	if isReviewCanceled(cause) || isReviewCanceled(ctx.Err()) {
		if cause != nil {
			return cause
		}
		return ctx.Err()
	}
	code := CodeForObjective(cause)
	if errors.Is(cause, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		code = CodeObjectiveCancelled
	}
	failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	run, err := r.store.FailRun(failCtx, runID, code, cause.Error(), int(time.Since(started).Milliseconds()))
	if err != nil {
		r.logger.Error("record objective run failure", zap.String("run_id", runID), zap.Error(err))
	}
	if req.Gate && req.Trigger == models.ReviewTriggerWorkflowStep && !errors.Is(cause, ErrNoObjective) && run != nil {
		r.store.WriteFailedAssessmentGate(failCtx, run, req.Gate, cause.Error())
	}
	return cause
}

// objectiveCriterionInputs maps parsed criteria to service inputs, tagging the
// source: document when the objective carried a predefined list or the criterion
// names an AC id, derived otherwise.
func objectiveCriterionInputs(oc ObjectiveContext, parsed []ParsedCriterion) []taskservice.ObjectiveCriterionInput {
	predefined := !oc.DeriveCriteria && len(oc.Criteria) > 0
	out := make([]taskservice.ObjectiveCriterionInput, 0, len(parsed))
	for _, c := range parsed {
		source := string(models.ObjectiveSourceDerived)
		if predefined || c.SourceRef != "" {
			source = string(models.ObjectiveSourceDocument)
		}
		out = append(out, taskservice.ObjectiveCriterionInput{
			Text:      c.Text,
			SourceRef: c.SourceRef,
			Source:    source,
			Status:    string(c.Status),
			Rationale: c.Rationale,
			Evidence:  c.Evidence,
		})
	}
	return out
}
