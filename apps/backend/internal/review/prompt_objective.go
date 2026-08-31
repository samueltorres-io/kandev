package review

import (
	"context"
	"fmt"
	"strings"
)

// ObjectivePromptSentinel is the marker the objective-check prompt template must
// contain. The mock agent matches on it to return a deterministic assessment in
// dev and E2E runs.
const ObjectivePromptSentinel = "KANDEV_OBJECTIVE_CHECK_REQUEST"

// objectiveDiffBudgetBytes caps the combined diff sent to the assessing agent.
// The changed-file list is always included in full; the diff is tail-trimmed
// with a marker when it exceeds this.
const objectiveDiffBudgetBytes = 120_000

// ObjectiveTemplateSource serves the objective-check utility agent's stored
// prompt, so a user who edits that prompt changes how assessments are performed.
type ObjectiveTemplateSource interface {
	ObjectivePromptTemplate(ctx context.Context) (string, error)
	ResolveTemplate(ctx context.Context, template string, values map[string]string) (string, error)
}

// ObjectivePromptBuilder renders the assessment prompt for one pass over a
// task's whole changed set.
type ObjectivePromptBuilder interface {
	BuildObjective(ctx context.Context, oc ObjectiveContext, files []ChangedFile, promptCtx PromptContext) (string, error)
}

// ObjectiveTemplatePromptBuilder renders the assessment prompt from the
// objective-check utility agent's template.
type ObjectiveTemplatePromptBuilder struct {
	source ObjectiveTemplateSource
	budget int
}

// NewObjectiveTemplatePromptBuilder builds an objective prompt builder.
func NewObjectiveTemplatePromptBuilder(source ObjectiveTemplateSource) *ObjectiveTemplatePromptBuilder {
	return &ObjectiveTemplatePromptBuilder{source: source, budget: objectiveDiffBudgetBytes}
}

// BuildObjective renders the assessment prompt.
func (b *ObjectiveTemplatePromptBuilder) BuildObjective(ctx context.Context, oc ObjectiveContext, files []ChangedFile, promptCtx PromptContext) (string, error) {
	if b.source == nil {
		return "", fmt.Errorf("no objective-check prompt template is configured")
	}
	template, err := b.source.ObjectivePromptTemplate(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(template) == "" {
		return "", fmt.Errorf("the objective-check prompt template is empty")
	}

	values := map[string]string{
		"Objective":    oc.ObjectiveText,
		"Criteria":     FormatObjectiveCriteria(oc),
		"ChangedFiles": FormatChangedFileList(files),
		"GitDiff":      trimToBudget(FormatBatchDiff(files), b.budget),
		"DiffSummary":  FormatDiffSummary(files),
		"TaskTitle":    promptCtx.TaskTitle,
		"BranchName":   promptCtx.BranchName,
		"BaseBranch":   promptCtx.BaseBranch,
	}
	resolved, err := b.source.ResolveTemplate(ctx, template, values)
	if err != nil {
		return "", err
	}
	if !strings.Contains(resolved, ObjectivePromptSentinel) {
		resolved = ObjectivePromptSentinel + "\n\n" + resolved
	}
	return resolved, nil
}

// FormatObjectiveCriteria renders the predefined criteria list, or the
// derive-your-own instruction when none were found in a document.
func FormatObjectiveCriteria(oc ObjectiveContext) string {
	if oc.DeriveCriteria || len(oc.Criteria) == 0 {
		return "No acceptance criteria were provided. Derive 1 to 12 testable criteria from the objective above and evaluate each."
	}
	lines := make([]string, 0, len(oc.Criteria))
	for i, c := range oc.Criteria {
		prefix := fmt.Sprintf("%d.", i+1)
		if c.SourceRef != "" {
			lines = append(lines, fmt.Sprintf("%s [%s] %s", prefix, c.SourceRef, c.Text))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s %s", prefix, c.Text))
	}
	return strings.Join(lines, "\n")
}

// trimToBudget tail-trims s to at most budget bytes on a rune boundary, adding a
// marker when it cut. ponytail: naive tail cut, upgrade to per-file relevance
// trimming if assessments start missing large changes.
func trimToBudget(s string, budget int) string {
	if len(s) <= budget {
		return s
	}
	cut := s[:budget]
	for len(cut) > 0 && cut[len(cut)-1]&0xC0 == 0x80 {
		cut = cut[:len(cut)-1]
	}
	return cut + "\n\n[diff truncated to fit the assessment budget; the changed-file list above is complete]"
}
