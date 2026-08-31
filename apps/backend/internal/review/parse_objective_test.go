package review

import (
	"errors"
	"testing"

	"github.com/kandev/kandev/internal/task/models"
)

const goodAssessment = `Here is my assessment.

` + "```json" + `
{
  "verdict": "partial",
  "summary": "Login works; rate limiting is missing.",
  "criteria": [
    {"text": "A user can sign in", "source_ref": "AC-X-1", "status": "met",
     "rationale": "handler added", "evidence": [{"file": "auth/login.go", "line": 40}]},
    {"text": "Rate limited", "status": "unmet", "rationale": "not implemented"}
  ]
}
` + "```" + `
`

func TestParseAssessment_FencedJSON(t *testing.T) {
	res, err := ParseAssessment(goodAssessment)
	if err != nil {
		t.Fatalf("ParseAssessment: %v", err)
	}
	if res.StatedVerdict != "partial" || res.Summary == "" || len(res.Criteria) != 2 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Criteria[0].Status != models.ObjectiveCriterionMet || res.Criteria[0].Evidence[0].Line != 40 {
		t.Fatalf("criterion 0 not parsed: %+v", res.Criteria[0])
	}
}

func TestParseAssessment_BareAndProseWrapped(t *testing.T) {
	bare := `{"criteria":[{"text":"x","status":"met"}]}`
	if _, err := ParseAssessment(bare); err != nil {
		t.Fatalf("bare JSON: %v", err)
	}
	prose := "I think:\n{\"summary\":\"s\",\"criteria\":[{\"text\":\"x\",\"status\":\"unmet\"}]}\nThat's all."
	if _, err := ParseAssessment(prose); err != nil {
		t.Fatalf("prose-wrapped JSON: %v", err)
	}
}

func TestParseAssessment_DropsMalformedCriterion(t *testing.T) {
	in := `{"criteria":[
		{"text":"ok","status":"met"},
		{"text":"","status":"met"},
		{"text":"bad status","status":"maybe"}
	]}`
	res, err := ParseAssessment(in)
	if err != nil {
		t.Fatalf("ParseAssessment: %v", err)
	}
	if len(res.Criteria) != 1 || res.RejectedCount != 2 {
		t.Fatalf("expected 1 kept / 2 rejected, got %+v", res)
	}
}

func TestParseAssessment_NoJSONIsUnparseable(t *testing.T) {
	if _, err := ParseAssessment("The task looks done to me."); !errors.Is(err, ErrUnparseableResponse) {
		t.Fatalf("expected ErrUnparseableResponse, got %v", err)
	}
	if _, err := ParseAssessment(`{"criteria":[]}`); !errors.Is(err, ErrUnparseableResponse) {
		t.Fatalf("expected ErrUnparseableResponse for zero criteria, got %v", err)
	}
}
