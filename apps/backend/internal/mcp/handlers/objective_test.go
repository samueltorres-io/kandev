package handlers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kandev/kandev/internal/review"
	"github.com/kandev/kandev/internal/task/models"
	ws "github.com/kandev/kandev/pkg/websocket"
)

func TestHandlePublishObjectiveAssessment_StoresValidBatch(t *testing.T) {
	h, reviewSvc, _ := newReviewHandlers(t)
	seedReviewHandlerTask(t, h, "task-obj")

	msg := makeWSMessage(t, ws.ActionMCPPublishObjectiveAssessment, map[string]interface{}{
		"task_id": "task-obj",
		"summary": "one met, one not",
		"criteria": []map[string]interface{}{
			{"text": "signs in", "status": "met", "rationale": "handler added"},
			{"text": "rate limited", "status": "unmet", "rationale": "missing"},
		},
	})
	resp, err := h.handlePublishObjectiveAssessment(context.Background(), msg)
	require.NoError(t, err)
	requireWSSuccess(t, resp)

	got, err := reviewSvc.GetTaskAssessment(context.Background(), "task-obj")
	require.NoError(t, err)
	require.Len(t, got.Runs, 1)
	require.Equal(t, models.ObjectiveVerdictPartial, got.Verdict)
	require.Len(t, got.Criteria, 2)
}

func TestHandlePublishObjectiveAssessment_MalformedRejectsWholeBatch(t *testing.T) {
	h, reviewSvc, _ := newReviewHandlers(t)
	seedReviewHandlerTask(t, h, "task-obj2")

	msg := makeWSMessage(t, ws.ActionMCPPublishObjectiveAssessment, map[string]interface{}{
		"task_id": "task-obj2",
		"criteria": []map[string]interface{}{
			{"text": "ok", "status": "met", "rationale": "r"},
			{"text": "", "status": "met", "rationale": "r"},
		},
	})
	resp, err := h.handlePublishObjectiveAssessment(context.Background(), msg)
	require.NoError(t, err)
	require.Equal(t, ws.MessageTypeError, resp.Type)

	got, err := reviewSvc.GetTaskAssessment(context.Background(), "task-obj2")
	require.NoError(t, err)
	require.Empty(t, got.Runs)
}

func TestHandleRunObjectiveAssessment_MapsNoChangesCode(t *testing.T) {
	h, _, runner := newReviewHandlers(t)
	seedReviewHandlerTask(t, h, "task-obj3")
	runner.err = review.ErrNoChanges

	msg := makeWSMessage(t, ws.ActionTaskObjectiveRun, map[string]interface{}{"task_id": "task-obj3"})
	resp, err := h.handleRunObjectiveAssessment(context.Background(), msg)
	require.NoError(t, err)
	require.Equal(t, ws.MessageTypeError, resp.Type)
	require.Equal(t, review.CodeObjectiveNoChanges, wsErrorDetails(t, resp)["code"])

	require.Len(t, runner.launched, 1)
	require.Equal(t, models.ReviewKindObjectiveCheck, runner.launched[0].Kind)
}
