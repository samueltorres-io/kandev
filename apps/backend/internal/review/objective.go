package review

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// ErrNoObjective means the task has neither a description nor a plan/spec
// document, so there is nothing to assess. The on-demand run control is hidden
// in this state; a gated workflow step in this state does not block.
var ErrNoObjective = errors.New("objective_no_objective")

const (
	// maxObjectiveTextBytes caps the whole objective context passed to the
	// prompt (~60 KB, matching the system design's budget).
	maxObjectiveTextBytes = 60000
	// maxObjectiveDescBytes / maxObjectiveDocBytes cap each part before they
	// are concatenated.
	maxObjectiveDescBytes = 16000
	maxObjectiveDocBytes  = 24000
	// maxPredefinedCriteria matches AC-...-001.2's "1 to 12 criteria" ceiling.
	maxPredefinedCriteria = 12
)

// ObjectiveTask is the minimal task data BuildObjectiveContext needs.
type ObjectiveTask struct {
	Title       string
	Description string
}

// ObjectiveDoc is one task-attached document. Kind is the TaskDocument key
// ("plan", "spec", ...); only plan and spec are read.
type ObjectiveDoc struct {
	Kind      string
	Body      string
	UpdatedAt time.Time
}

// PredefinedCriterion is a criterion copied verbatim from a document's
// acceptance-criteria list. SourceRef is the AC-* id when one was matched.
type PredefinedCriterion struct {
	Text      string
	SourceRef string
}

// ObjectiveContext is the parsed objective input for the assessment prompt.
type ObjectiveContext struct {
	ObjectiveText string
	// Criteria is the verbatim acceptance-criteria list found in a document.
	// Empty when none was found.
	Criteria []PredefinedCriterion
	// DeriveCriteria is true when the agent must derive the checklist itself.
	DeriveCriteria bool
}

// BuildObjectiveContext assembles the objective text from the task description
// plus the newest plan and spec documents, and extracts a verbatim
// acceptance-criteria list when a document contains one.
func BuildObjectiveContext(task ObjectiveTask, docs []ObjectiveDoc) (ObjectiveContext, error) {
	plan := newestDoc(docs, "plan")
	spec := newestDoc(docs, "spec")

	parts := make([]string, 0, 3)
	if d := strings.TrimSpace(task.Description); d != "" {
		parts = append(parts, capBytes(d, maxObjectiveDescBytes))
	}
	for _, d := range []*ObjectiveDoc{plan, spec} {
		if d == nil {
			continue
		}
		if b := strings.TrimSpace(d.Body); b != "" {
			parts = append(parts, capBytes(b, maxObjectiveDocBytes))
		}
	}
	objectiveText := capBytes(strings.TrimSpace(strings.Join(parts, "\n\n---\n\n")), maxObjectiveTextBytes)
	if objectiveText == "" {
		return ObjectiveContext{}, ErrNoObjective
	}

	var criteria []PredefinedCriterion
	for _, d := range []*ObjectiveDoc{plan, spec} {
		if d == nil {
			continue
		}
		if found := extractAcceptanceCriteria(d.Body); len(found) > 0 {
			criteria = found
			break
		}
	}
	if len(criteria) > maxPredefinedCriteria {
		criteria = criteria[:maxPredefinedCriteria]
	}
	return ObjectiveContext{
		ObjectiveText:  objectiveText,
		Criteria:       criteria,
		DeriveCriteria: len(criteria) == 0,
	}, nil
}

func newestDoc(docs []ObjectiveDoc, kind string) *ObjectiveDoc {
	var best *ObjectiveDoc
	for i := range docs {
		if docs[i].Kind != kind {
			continue
		}
		if best == nil || docs[i].UpdatedAt.After(best.UpdatedAt) {
			best = &docs[i]
		}
	}
	return best
}

func capBytes(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := s[:limit]
	// Trim back to a valid rune boundary.
	for len(cut) > 0 && cut[len(cut)-1]&0xC0 == 0x80 {
		cut = cut[:len(cut)-1]
	}
	if len(cut) > 0 && cut[len(cut)-1]&0xC0 == 0xC0 {
		cut = cut[:len(cut)-1]
	}
	return cut
}

var (
	acHeadingRe  = regexp.MustCompile(`(?i)^#{1,6}\s*acceptance`)
	otherHeading = regexp.MustCompile(`^#{1,6}\s`)
	listItemRe   = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+(.*\S)\s*$`)
	acRefRe      = regexp.MustCompile(`AC-[A-Z0-9]+(?:-[A-Z0-9]+)*(?:\.\d+)*`)
	boldRe       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
)

// extractAcceptanceCriteria pulls a verbatim criteria list from a Markdown body:
// the list items under an "## Acceptance criteria" heading, plus any list item
// that itself names an AC-* id.
func extractAcceptanceCriteria(body string) []PredefinedCriterion {
	lines := strings.Split(body, "\n")
	var out []PredefinedCriterion
	seen := map[string]struct{}{}
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case acHeadingRe.MatchString(trimmed):
			inSection = true
			continue
		case otherHeading.MatchString(trimmed):
			inSection = false
			continue
		}
		m := listItemRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		item := m[1]
		ref := acRefRe.FindString(item)
		if !inSection && ref == "" {
			continue
		}
		c := PredefinedCriterion{Text: cleanCriterionText(item), SourceRef: ref}
		key := c.SourceRef + "\x00" + c.Text
		if _, dup := seen[key]; dup || c.Text == "" {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	return out
}

func cleanCriterionText(item string) string {
	s := boldRe.ReplaceAllString(item, "$1")
	// Drop a leading "AC-...:" label; the ref is carried separately.
	if idx := acRefRe.FindStringIndex(s); idx != nil && idx[0] <= 2 {
		rest := strings.TrimSpace(s[idx[1]:])
		rest = strings.TrimPrefix(rest, ":")
		if t := strings.TrimSpace(rest); t != "" {
			s = t
		}
	}
	return strings.TrimSpace(s)
}
