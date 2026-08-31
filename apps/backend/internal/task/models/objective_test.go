package models

import "testing"

func TestRollupObjectiveVerdict(t *testing.T) {
	met := ObjectiveCriterionMet
	partial := ObjectiveCriterionPartial
	unmet := ObjectiveCriterionUnmet
	unknown := ObjectiveCriterionUnknown

	cases := []struct {
		name string
		in   []ObjectiveCriterionStatus
		want ObjectiveVerdict
	}{
		{"empty is unmet", nil, ObjectiveVerdictUnmet},
		{"all met", []ObjectiveCriterionStatus{met, met}, ObjectiveVerdictMet},
		{"some met some not", []ObjectiveCriterionStatus{met, unmet}, ObjectiveVerdictPartial},
		{"met plus partial is partial", []ObjectiveCriterionStatus{met, partial}, ObjectiveVerdictPartial},
		{"met plus unknown is partial", []ObjectiveCriterionStatus{met, unknown}, ObjectiveVerdictPartial},
		{"none met is unmet", []ObjectiveCriterionStatus{partial, unmet, unknown}, ObjectiveVerdictUnmet},
	}
	for _, c := range cases {
		if got := RollupObjectiveVerdict(c.in); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestReviewRunKindNormalized(t *testing.T) {
	if ReviewRunKind("").Normalized() != ReviewKindCodeReview {
		t.Fatal("empty kind should normalize to code_review")
	}
	if ReviewKindObjectiveCheck.Normalized() != ReviewKindObjectiveCheck {
		t.Fatal("objective_check kind should be unchanged")
	}
}
