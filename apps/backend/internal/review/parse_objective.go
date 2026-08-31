package review

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kandev/kandev/internal/task/models"
)

// ParsedCriterion is one criterion result recovered from an assessment reply.
type ParsedCriterion struct {
	Text      string
	SourceRef string
	Status    models.ObjectiveCriterionStatus
	Rationale string
	Evidence  []models.EvidencePointer
}

// AssessmentResult is the outcome of reading one assessment reply. StatedVerdict
// is the agent's own verdict, kept for logging only: the stored verdict is
// always recomputed by ReviewService from the criterion statuses.
type AssessmentResult struct {
	StatedVerdict string
	Summary       string
	Criteria      []ParsedCriterion
	RejectedCount int
}

type assessmentEvidence struct {
	Repo    string `json:"repo"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	LineEnd int    `json:"line_end"`
}

type assessmentCriterion struct {
	Text      string               `json:"text"`
	SourceRef string               `json:"source_ref"`
	Status    string               `json:"status"`
	Rationale string               `json:"rationale"`
	Evidence  []assessmentEvidence `json:"evidence"`
}

type assessmentResponse struct {
	Verdict  string                `json:"verdict"`
	Summary  string                `json:"summary"`
	Criteria []assessmentCriterion `json:"criteria"`
}

// ParseAssessment extracts the assessment envelope from the agent's raw reply,
// tolerating fences and surrounding prose (same extraction as ParseFindings). A
// criterion missing text or status, or with an unknown status, is dropped and
// counted. ErrUnparseableResponse is returned when zero criteria are
// recoverable.
func ParseAssessment(response string) (AssessmentResult, error) {
	for _, candidate := range jsonCandidates(response) {
		var envelope assessmentResponse
		if err := json.Unmarshal([]byte(candidate), &envelope); err != nil {
			continue
		}
		if envelope.Criteria == nil && !strings.Contains(candidate, `"criteria"`) {
			continue
		}
		result := validateAssessment(envelope)
		if len(result.Criteria) == 0 {
			return AssessmentResult{}, fmt.Errorf("%w: no usable criteria in assessment response", ErrUnparseableResponse)
		}
		return result, nil
	}
	return AssessmentResult{}, fmt.Errorf("%w: no criteria array in assessment response", ErrUnparseableResponse)
}

func validateAssessment(envelope assessmentResponse) AssessmentResult {
	result := AssessmentResult{
		StatedVerdict: strings.ToLower(strings.TrimSpace(envelope.Verdict)),
		Summary:       strings.TrimSpace(envelope.Summary),
	}
	for _, c := range envelope.Criteria {
		text := strings.TrimSpace(c.Text)
		status := models.ObjectiveCriterionStatus(strings.ToLower(strings.TrimSpace(c.Status)))
		if text == "" || !models.ValidObjectiveCriterionStatus(status) {
			result.RejectedCount++
			continue
		}
		result.Criteria = append(result.Criteria, ParsedCriterion{
			Text:      text,
			SourceRef: strings.TrimSpace(c.SourceRef),
			Status:    status,
			Rationale: strings.TrimSpace(c.Rationale),
			Evidence:  normalizeAssessmentEvidence(c.Evidence),
		})
	}
	return result
}

func normalizeAssessmentEvidence(in []assessmentEvidence) []models.EvidencePointer {
	out := make([]models.EvidencePointer, 0, len(in))
	for _, e := range in {
		file := strings.TrimSpace(e.File)
		if file == "" {
			continue
		}
		out = append(out, models.EvidencePointer{
			Repo:    strings.TrimSpace(e.Repo),
			File:    file,
			Line:    e.Line,
			LineEnd: e.LineEnd,
		})
	}
	return out
}
