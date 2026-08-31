package handlers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/kandev/kandev/internal/review"
	"github.com/kandev/kandev/internal/task/models"
	"github.com/kandev/kandev/internal/task/service"
	ws "github.com/kandev/kandev/pkg/websocket"
)

// objectiveCriterionPayload is the per-criterion wire shape shared by the MCP
// tool and any future client publisher. Field names match the tool schema.
type objectiveCriterionPayload struct {
	Text      string                     `json:"text"`
	SourceRef string                     `json:"source_ref"`
	Status    string                     `json:"status"`
	Rationale string                     `json:"rationale"`
	Evidence  []objectiveEvidencePayload `json:"evidence"`
}

type objectiveEvidencePayload struct {
	Repo    string `json:"repo"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	LineEnd int    `json:"line_end"`
}

// handlePublishObjectiveAssessment stores an agent-authored assessment. Like the
// findings publish path, a malformed criterion rejects the whole call so the
// agent can correct and retry.
func (h *Handlers) handlePublishObjectiveAssessment(ctx context.Context, msg *ws.Message) (*ws.Message, error) {
	var req struct {
		TaskID    string                      `json:"task_id"`
		SessionID string                      `json:"session_id"`
		Summary   string                      `json:"summary"`
		Criteria  []objectiveCriterionPayload `json:"criteria"`
	}
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeBadRequest, "Invalid payload: "+err.Error(), nil)
	}
	if req.TaskID == "" {
		return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeValidation, "task_id is required", nil)
	}
	if len(req.Criteria) == 0 {
		return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeValidation, "at least one criterion is required", nil)
	}

	inputs := make([]service.ObjectiveCriterionInput, 0, len(req.Criteria))
	for _, c := range req.Criteria {
		evidence := make([]models.EvidencePointer, 0, len(c.Evidence))
		for _, e := range c.Evidence {
			evidence = append(evidence, models.EvidencePointer{Repo: e.Repo, File: e.File, Line: e.Line, LineEnd: e.LineEnd})
		}
		inputs = append(inputs, service.ObjectiveCriterionInput{
			Text: c.Text, SourceRef: c.SourceRef, Status: c.Status, Rationale: c.Rationale, Evidence: evidence,
		})
	}

	run, criteria, err := h.reviewService.PublishAssessment(ctx, service.PublishAssessmentRequest{
		TaskID:    req.TaskID,
		SessionID: req.SessionID,
		Trigger:   models.ReviewTriggerAgent,
		Summary:   req.Summary,
		Criteria:  inputs,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidCriterion) || errors.Is(err, service.ErrNoCriteria) {
			return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeValidation, err.Error(), nil)
		}
		if errors.Is(err, service.ErrTaskIDRequired) {
			return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeValidation, "task_id is required", nil)
		}
		return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeInternalError, "Failed to publish assessment: "+err.Error(), nil)
	}

	return ws.NewResponse(msg.ID, msg.Action, map[string]any{
		"run_id":          run.ID,
		"criterion_count": len(criteria),
		"verdict":         run.Verdict,
	})
}

// handleRunObjectiveAssessment starts an on-demand assessment pass.
func (h *Handlers) handleRunObjectiveAssessment(ctx context.Context, msg *ws.Message) (*ws.Message, error) {
	var req struct {
		TaskID         string `json:"task_id"`
		SessionID      string `json:"session_id"`
		RepositoryID   string `json:"repository_id"`
		AgentProfileID string `json:"agent_profile_id"`
	}
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeBadRequest, "Invalid payload: "+err.Error(), nil)
	}
	if req.TaskID == "" {
		return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeValidation, "task_id is required", nil)
	}
	run, err := h.reviewRunner.Launch(ctx, review.RunRequest{
		TaskID:         req.TaskID,
		Kind:           models.ReviewKindObjectiveCheck,
		SessionID:      req.SessionID,
		RepositoryID:   req.RepositoryID,
		AgentProfileID: req.AgentProfileID,
		Trigger:        models.ReviewTriggerManual,
	})
	if err != nil {
		return objectiveLaunchError(msg, err)
	}
	return ws.NewResponse(msg.ID, msg.Action, map[string]any{"run": run})
}

// objectiveLaunchError maps a launch failure onto a WS error carrying the
// objective_* code the Review surface branches on.
func objectiveLaunchError(msg *ws.Message, err error) (*ws.Message, error) {
	code := review.CodeForObjective(err)
	data := map[string]any{"code": code}
	switch code {
	case review.CodeObjectiveNoChanges, review.CodeObjectiveAgentUnavailable,
		review.CodeObjectiveWorkspaceUnavailable, review.CodeObjectiveNoObjective:
		return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeValidation, err.Error(), data)
	default:
		if errors.Is(err, service.ErrTaskIDRequired) {
			return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeValidation, "task_id is required", nil)
		}
		return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeInternalError, err.Error(), data)
	}
}

// handleCancelObjectiveAssessment cancels a non-terminal assessment run.
func (h *Handlers) handleCancelObjectiveAssessment(ctx context.Context, msg *ws.Message) (*ws.Message, error) {
	var req struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeBadRequest, "Invalid payload: "+err.Error(), nil)
	}
	if req.RunID == "" {
		return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeValidation, "run_id is required", nil)
	}
	cancel := h.reviewService.CancelRun
	if h.reviewRunner != nil {
		cancel = h.reviewRunner.Cancel
	}
	run, err := cancel(ctx, req.RunID)
	if err != nil {
		if errors.Is(err, service.ErrReviewRunNotFound) {
			return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeNotFound, "Assessment run not found", nil)
		}
		return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeInternalError, "Failed to cancel assessment: "+err.Error(), nil)
	}
	return ws.NewResponse(msg.ID, msg.Action, map[string]any{"run": run})
}

// handleGetObjectiveAssessment returns a task's assessment run history and the
// latest completed run's criteria, to backfill the store on mount.
func (h *Handlers) handleGetObjectiveAssessment(ctx context.Context, msg *ws.Message) (*ws.Message, error) {
	taskID, errMsg, errErr := parseTaskIDPayload(msg)
	if errMsg != nil || errErr != nil {
		return errMsg, errErr
	}
	result, err := h.reviewService.GetTaskAssessment(ctx, taskID)
	if err != nil {
		if errors.Is(err, service.ErrTaskIDRequired) {
			return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeValidation, "task_id is required", nil)
		}
		return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeInternalError, "Failed to get task assessment", nil)
	}
	return ws.NewResponse(msg.ID, msg.Action, result)
}

// handleClearObjectiveAssessment removes a task's assessment runs and criteria.
func (h *Handlers) handleClearObjectiveAssessment(ctx context.Context, msg *ws.Message) (*ws.Message, error) {
	taskID, errMsg, errErr := parseTaskIDPayload(msg)
	if errMsg != nil || errErr != nil {
		return errMsg, errErr
	}
	if err := h.reviewService.ClearTaskAssessment(ctx, taskID); err != nil {
		if errors.Is(err, service.ErrTaskIDRequired) {
			return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeValidation, "task_id is required", nil)
		}
		return ws.NewError(msg.ID, msg.Action, ws.ErrorCodeInternalError, "Failed to clear task assessment: "+err.Error(), nil)
	}
	return ws.NewResponse(msg.ID, msg.Action, map[string]any{"success": true})
}
