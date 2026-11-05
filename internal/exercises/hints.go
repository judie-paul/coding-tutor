package exercises

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"codingtutor/internal/ai"
	"codingtutor/internal/database"
	"codingtutor/internal/progress"
	"codingtutor/internal/users"
)

// MaxHints is the number of progressive hints per exercise.
const MaxHints = 3

// Hint-related errors.
var (
	ErrNoMoreHints          = errors.New("all hints already shown")
	ErrSolutionNotConfirmed = errors.New("solution must be explicitly requested")
)

// Hint is a hint or the revealed solution for an exercise.
type Hint struct {
	Kind      string // hint or solution
	Level     int    // 1-3 for hints, 4 for the solution
	Content   string
	CreatedAt time.Time
}

// IsSolution reports whether this entry is the full solution.
func (h Hint) IsSolution() bool { return h.Kind == "solution" }

// Hints returns the hints (and solution, if revealed) shown so far, in order.
func (s *Service) Hints(ctx context.Context, userID, exerciseID int64) ([]Hint, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT h.kind, h.level, h.content, h.created_at
		FROM exercise_hints h JOIN exercises e ON e.id = h.exercise_id
		WHERE h.exercise_id = ? AND e.user_id = ? ORDER BY h.level`, exerciseID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Hint
	for rows.Next() {
		var h Hint
		var created string
		if err := rows.Scan(&h.Kind, &h.Level, &h.Content, &created); err != nil {
			return nil, err
		}
		h.CreatedAt = database.ParseTime(created)
		out = append(out, h)
	}
	return out, rows.Err()
}

// RequestHint generates and stores the next progressive hint. currentCode is
// the student's optional current attempt, used to tailor the hint. If the AI
// is unavailable, the hint written when the exercise was generated is used.
func (s *Service) RequestHint(ctx context.Context, u *users.User, exerciseID int64, currentCode string) (Hint, error) {
	e, err := s.Get(ctx, u.ID, exerciseID)
	if err != nil {
		return Hint{}, err
	}
	existing, err := s.Hints(ctx, u.ID, exerciseID)
	if err != nil {
		return Hint{}, err
	}
	var previous []string
	for _, h := range existing {
		if !h.IsSolution() {
			previous = append(previous, h.Content)
		}
	}
	level := len(previous) + 1
	if level > MaxHints {
		return Hint{}, ErrNoMoreHints
	}

	content, err := s.ai.Hint(ctx, ai.HintRequest{
		Learner: learner(u), Exercise: &e.Content, Level: level,
		PreviousHints: previous, StudentCode: strings.TrimSpace(currentCode),
	})
	if err != nil {
		// Fall back to the hint written with the exercise, if there is one.
		if !ai.IsAIError(err) || len(e.Content.Hints) < level {
			return Hint{}, err
		}
		content = e.Content.Hints[level-1]
	}

	h := Hint{Kind: "hint", Level: level, Content: content, CreatedAt: time.Now().UTC()}
	err = database.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := insertHint(ctx, tx, e.ID, h); err != nil {
			return err
		}
		if err := s.progress.Increment(ctx, tx, u.ID, e.TopicID, progress.HintsRequested, 1); err != nil {
			return err
		}
		return s.progress.Log(ctx, tx, u.ID, e.TopicID, progress.KindHintRequested,
			fmt.Sprintf("Used hint %d on “%s”", level, e.Title), fmt.Sprintf("/exercises/%d#hints", e.ID))
	})
	return h, err
}

// RevealSolution shows the complete solution. confirmed must be true: the
// solution is only ever revealed when the student explicitly asks for it.
func (s *Service) RevealSolution(ctx context.Context, u *users.User, exerciseID int64, confirmed bool) (Hint, error) {
	if !confirmed {
		return Hint{}, ErrSolutionNotConfirmed
	}
	e, err := s.Get(ctx, u.ID, exerciseID)
	if err != nil {
		return Hint{}, err
	}
	existing, err := s.Hints(ctx, u.ID, exerciseID)
	if err != nil {
		return Hint{}, err
	}
	var previous []string
	for _, h := range existing {
		if h.IsSolution() {
			return h, nil // already revealed
		}
		previous = append(previous, h.Content)
	}

	content, err := s.ai.Solution(ctx, ai.HintRequest{Learner: learner(u), Exercise: &e.Content, PreviousHints: previous})
	if err != nil {
		return Hint{}, err
	}
	h := Hint{Kind: "solution", Level: MaxHints + 1, Content: content, CreatedAt: time.Now().UTC()}
	err = database.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := insertHint(ctx, tx, e.ID, h); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE exercises SET solution_revealed = 1, updated_at = ? WHERE id = ? AND user_id = ?`,
			database.Now(), e.ID, u.ID); err != nil {
			return err
		}
		return s.progress.Log(ctx, tx, u.ID, e.TopicID, progress.KindSolutionRevealed,
			"Viewed the solution to “"+e.Title+"”", fmt.Sprintf("/exercises/%d#hints", e.ID))
	})
	return h, err
}

func insertHint(ctx context.Context, tx *sql.Tx, exerciseID int64, h Hint) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO exercise_hints (exercise_id, kind, level, content, created_at)
		VALUES (?, ?, ?, ?, ?)`, exerciseID, h.Kind, h.Level, h.Content, database.FormatTime(h.CreatedAt))
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		// A concurrent request already stored this level.
		return ErrNoMoreHints
	}
	return err
}
