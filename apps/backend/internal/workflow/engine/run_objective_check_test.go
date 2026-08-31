package engine

import (
	"testing"

	wfmodels "github.com/kandev/kandev/internal/workflow/models"
)

func compiledObjectiveCheckAction(t *testing.T, config map[string]any) (Action, bool) {
	t.Helper()
	step := &wfmodels.WorkflowStep{
		ID:   "step-1",
		Name: "Assess",
		Events: wfmodels.StepEvents{
			OnEnter: []wfmodels.OnEnterAction{
				{Type: wfmodels.OnEnterRunObjectiveCheck, Config: config},
			},
		},
	}
	for _, action := range CompileStep(step).Events[TriggerOnEnter] {
		if action.Kind == ActionRunObjectiveCheck {
			return action, true
		}
	}
	return Action{}, false
}

func TestCompileStep_RunObjectiveCheckWithProfileAndGate(t *testing.T) {
	action, ok := compiledObjectiveCheckAction(t, map[string]any{
		wfmodels.ReviewAgentProfileConfigKey: "profile-9",
		wfmodels.ObjectiveGateConfigKey:      true,
	})
	if !ok || action.RunObjectiveCheck == nil {
		t.Fatalf("expected a typed run_objective_check action, got %+v ok=%v", action, ok)
	}
	if action.RunObjectiveCheck.AgentProfileID != "profile-9" {
		t.Fatalf("expected profile-9, got %q", action.RunObjectiveCheck.AgentProfileID)
	}
	if !action.RunObjectiveCheck.Gate {
		t.Fatal("expected Gate=true")
	}
}

func TestCompileStep_RunObjectiveCheckDefaultsUngated(t *testing.T) {
	action, ok := compiledObjectiveCheckAction(t, nil)
	if !ok || action.RunObjectiveCheck == nil {
		t.Fatal("expected a run_objective_check action to compile with no config")
	}
	if action.RunObjectiveCheck.Gate || action.RunObjectiveCheck.AgentProfileID != "" {
		t.Fatalf("expected ungated, no profile, got %+v", action.RunObjectiveCheck)
	}
}
