package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/kandev/kandev/internal/task/models"
)

// ListTaskObjectiveRuns returns a task's objective-assessment runs, newest
// first, capped at limit. It is the objective sibling of ListTaskReviewRuns and
// filters kind = 'objective_check' so the two run kinds never mix in a read.
func (r *Repository) ListTaskObjectiveRuns(ctx context.Context, taskID string, limit int) ([]*models.TaskReviewRun, error) {
	if limit <= 0 {
		limit = defaultReviewRunHistory
	}
	rows, err := r.ro.QueryContext(ctx, r.ro.Rebind(
		`SELECT `+reviewRunColumns+` FROM task_review_runs
		 WHERE task_id = ? AND kind = 'objective_check'
		 ORDER BY created_at DESC, id DESC LIMIT ?`), taskID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list task objective runs: %w", err)
	}
	return collectReviewRuns(rows)
}

// ListActiveTaskObjectiveRuns returns the task's pending/running
// objective-assessment runs, newest first. The in-flight guard uses this so a
// second assessment request rejoins the existing pass.
func (r *Repository) ListActiveTaskObjectiveRuns(ctx context.Context, taskID string) ([]*models.TaskReviewRun, error) {
	rows, err := r.ro.QueryContext(ctx, r.ro.Rebind(
		`SELECT `+reviewRunColumns+` FROM task_review_runs
		 WHERE task_id = ? AND kind = 'objective_check' AND status IN ('pending', 'running')
		 ORDER BY created_at DESC, id DESC`), taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to list active task objective runs: %w", err)
	}
	return collectReviewRuns(rows)
}

// GetLatestCompletedTaskObjectiveRun returns the task's most recent completed
// objective-assessment run, or nil when the task has none. The Review surface
// shows this run's criteria as the current assessment.
func (r *Repository) GetLatestCompletedTaskObjectiveRun(ctx context.Context, taskID string) (*models.TaskReviewRun, error) {
	row := r.ro.QueryRowContext(ctx, r.ro.Rebind(
		`SELECT `+reviewRunColumns+` FROM task_review_runs
		 WHERE task_id = ? AND kind = 'objective_check' AND status = 'completed'
		 ORDER BY created_at DESC, id DESC LIMIT 1`), taskID)
	run, err := scanReviewRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return run, err
}

// CreateTaskObjectiveCriteria inserts a run's criterion checklist in one
// transaction so a partially-written assessment is never visible to a reader.
func (r *Repository) CreateTaskObjectiveCriteria(ctx context.Context, criteria []*models.TaskObjectiveCriterion) error {
	if len(criteria) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin task objective criteria tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	stmt := tx.Rebind(`INSERT INTO task_objective_criteria (` + objectiveCriterionColumns + `)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	for _, c := range criteria {
		if c.ID == "" {
			c.ID = uuid.New().String()
		}
		if c.CreatedAt.IsZero() {
			c.CreatedAt = now
		}
		if c.Source == "" {
			c.Source = models.ObjectiveSourceDerived
		}
		if c.Status == "" {
			c.Status = models.ObjectiveCriterionUnknown
		}
		evidence, marshalErr := marshalEvidence(c.Evidence)
		if marshalErr != nil {
			return marshalErr
		}
		if _, execErr := tx.ExecContext(ctx, stmt,
			c.ID, c.RunID, c.TaskID, c.Ordinal, string(c.Source), c.SourceRef,
			c.Text, string(c.Status), c.Rationale, evidence, c.CreatedAt,
		); execErr != nil {
			return fmt.Errorf("failed to insert task objective criterion: %w", execErr)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit task objective criteria: %w", err)
	}
	return nil
}

// ListTaskObjectiveCriteria returns a run's criteria in checklist order.
func (r *Repository) ListTaskObjectiveCriteria(ctx context.Context, taskID, runID string) ([]*models.TaskObjectiveCriterion, error) {
	rows, err := r.ro.QueryContext(ctx, r.ro.Rebind(
		`SELECT `+objectiveCriterionColumns+` FROM task_objective_criteria
		 WHERE task_id = ? AND run_id = ?
		 ORDER BY ordinal, id`), taskID, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to list task objective criteria: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var criteria []*models.TaskObjectiveCriterion
	for rows.Next() {
		c, scanErr := scanObjectiveCriterion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		criteria = append(criteria, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate task objective criteria: %w", err)
	}
	return criteria, nil
}

// GetTaskObjectiveCriterion retrieves a single criterion by ID.
func (r *Repository) GetTaskObjectiveCriterion(ctx context.Context, criterionID string) (*models.TaskObjectiveCriterion, error) {
	row := r.ro.QueryRowContext(ctx, r.ro.Rebind(
		`SELECT `+objectiveCriterionColumns+` FROM task_objective_criteria WHERE id = ?`), criterionID)
	c, err := scanObjectiveCriterion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", models.ErrTaskObjectiveCriterionNotFound, criterionID)
	}
	return c, err
}

// DeleteTaskObjectiveByTask removes a task's objective-assessment runs and their
// criteria, leaving Native Code Review state untouched. Backs
// ReviewService.ClearTaskAssessment.
func (r *Repository) DeleteTaskObjectiveByTask(ctx context.Context, taskID string) error {
	statements := []string{
		`DELETE FROM task_objective_criteria WHERE task_id = ?`,
		`DELETE FROM task_review_runs WHERE task_id = ? AND kind = 'objective_check'`,
	}
	for _, stmt := range statements {
		if _, err := r.db.ExecContext(ctx, r.db.Rebind(stmt), taskID); err != nil {
			return fmt.Errorf("failed to delete task objective state: %w", err)
		}
	}
	return nil
}

// DeleteTaskObjectiveCriteriaByRun removes every criterion of one run. Used when
// a re-run supersedes a prior assessment on a task that clears its state.
func (r *Repository) DeleteTaskObjectiveCriteriaByRun(ctx context.Context, runID string) error {
	if _, err := r.db.ExecContext(ctx, r.db.Rebind(
		`DELETE FROM task_objective_criteria WHERE run_id = ?`), runID); err != nil {
		return fmt.Errorf("failed to delete task objective criteria: %w", err)
	}
	return nil
}

func marshalEvidence(pointers []models.EvidencePointer) (string, error) {
	if len(pointers) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(pointers)
	if err != nil {
		return "", fmt.Errorf("failed to marshal criterion evidence: %w", err)
	}
	return string(b), nil
}

func scanObjectiveCriterion(s reviewRowScanner) (*models.TaskObjectiveCriterion, error) {
	c := &models.TaskObjectiveCriterion{}
	var source, status, evidence string
	err := s.Scan(&c.ID, &c.RunID, &c.TaskID, &c.Ordinal, &source, &c.SourceRef,
		&c.Text, &status, &c.Rationale, &evidence, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to scan task objective criterion: %w", err)
	}
	c.Source = models.ObjectiveCriterionSource(source)
	c.Status = models.ObjectiveCriterionStatus(status)
	if evidence != "" && evidence != "[]" {
		if err := json.Unmarshal([]byte(evidence), &c.Evidence); err != nil {
			return nil, fmt.Errorf("failed to unmarshal criterion evidence: %w", err)
		}
	}
	return c, nil
}
