package store

import (
	"context"
	"fmt"
	"time"
)

// OnboardingDoneSteps returns the dashboard onboarding steps an admin marked
// done by hand. The set overrides only the derived state: it persists a
// bypass, never proof the step was really completed.
func (s *Store) OnboardingDoneSteps(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT step FROM onboarding_done`)
	if err != nil {
		return nil, fmt.Errorf("query onboarding_done: %w", err)
	}

	defer func() { _ = rows.Close() }()

	out := map[string]struct{}{}

	for rows.Next() {
		var step string
		if err := rows.Scan(&step); err != nil {
			return nil, fmt.Errorf("scan onboarding_done: %w", err)
		}

		out[step] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate onboarding_done: %w", err)
	}

	return out, nil
}

// MarkOnboardingDone records steps as manually completed by actor, in one
// transaction. Re-marking a step refreshes marked_by and marked_at.
func (s *Store) MarkOnboardingDone(ctx context.Context, actor string, steps []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin onboarding_done tx: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().UTC().Format(time.RFC3339)

	for _, step := range steps {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO onboarding_done (step, marked_by, marked_at) VALUES (?, ?, ?)
			 ON CONFLICT(step) DO UPDATE SET marked_by = excluded.marked_by, marked_at = excluded.marked_at`,
			step, actor, now); err != nil {
			return fmt.Errorf("mark onboarding step %q: %w", step, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit onboarding_done: %w", err)
	}

	committed = true

	return nil
}
