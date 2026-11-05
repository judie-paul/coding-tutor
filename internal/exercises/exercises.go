// Package exercises implements AI-generated programming exercises,
// progressive hints and code submissions.
package exercises

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"codingtutor/internal/ai"
	"codingtutor/internal/database"
	"codingtutor/internal/progress"
	"codingtutor/internal/topics"
	"codingtutor/internal/users"
)

// Errors returned by the service.
var (
	ErrNotFound     = errors.New("exercise not found")
	ErrInvalidInput = errors.New("invalid exercise options")
)

// Status values.
const (
	StatusOpen      = "open"
	StatusCompleted = "completed"
)

// Exercise is a stored exercise owned by one student.
type Exercise struct {
	ID               int64
	UserID           int64
	TopicID          int64
	TopicTitle       string
	TopicSlug        string
	Difficulty       string
	Type             string
	Title            string
	Content          ai.Exercise
	Status           string
	SolutionRevealed bool
	HintCount        int
	SubmissionCount  int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// TypeLabel returns a friendly name for the exercise type.
func (e *Exercise) TypeLabel() string { return ai.ExerciseTypeLabel(e.Type) }

// Completed reports whether the exercise is completed.
func (e *Exercise) Completed() bool { return e.Status == StatusCompleted }

// Service holds exercise business logic.
type Service struct {
	db       *sql.DB
	ai       ai.Tutor
	progress *progress.Store
}

// NewService creates an exercise service.
func NewService(db *sql.DB, tutor ai.Tutor, p *progress.Store) *Service {
	return &Service{db: db, ai: tutor, progress: p}
}

func learner(u *users.User) ai.Learner {
	return ai.Learner{Language: u.LanguageName, Level: u.ExperienceLevel}
}

// Generate asks the AI for a new exercise and stores it.
func (s *Service) Generate(ctx context.Context, u *users.User, t topics.Topic, difficulty, typ string) (int64, error) {
	if !users.ValidLevel(difficulty) || !ai.ValidExerciseType(typ) || t.LanguageID != u.LanguageID {
		return 0, ErrInvalidInput
	}
	ex, err := s.ai.GenerateExercise(ctx, ai.ExerciseRequest{
		Learner: learner(u), Topic: t.Title, Difficulty: difficulty, Type: typ,
	})
	if err != nil {
		return 0, err
	}
	content, err := json.Marshal(ex)
	if err != nil {
		return 0, err
	}

	var id int64
	err = database.InTx(ctx, s.db, func(tx *sql.Tx) error {
		now := database.Now()
		res, err := tx.ExecContext(ctx, `INSERT INTO exercises
			(user_id, topic_id, difficulty, exercise_type, title, content_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, u.ID, t.ID, difficulty, typ, ex.Title, string(content), now, now)
		if err != nil {
			return fmt.Errorf("insert exercise: %w", err)
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		return s.progress.Log(ctx, tx, u.ID, t.ID, progress.KindExerciseGenerated,
			"New exercise: "+ex.Title, fmt.Sprintf("/exercises/%d", id))
	})
	return id, err
}

const selectExercise = `SELECT e.id, e.user_id, e.topic_id, t.title, t.slug, e.difficulty, e.exercise_type,
	e.title, e.content_json, e.status, e.solution_revealed,
	(SELECT COUNT(*) FROM exercise_hints h WHERE h.exercise_id = e.id AND h.kind = 'hint'),
	(SELECT COUNT(*) FROM exercise_submissions s WHERE s.exercise_id = e.id),
	e.created_at, e.updated_at
	FROM exercises e JOIN topics t ON t.id = e.topic_id`

func scanExercise(row interface{ Scan(...any) error }) (*Exercise, error) {
	var e Exercise
	var content, created, updated string
	err := row.Scan(&e.ID, &e.UserID, &e.TopicID, &e.TopicTitle, &e.TopicSlug, &e.Difficulty, &e.Type,
		&e.Title, &content, &e.Status, &e.SolutionRevealed, &e.HintCount, &e.SubmissionCount, &created, &updated)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(content), &e.Content); err != nil {
		return nil, fmt.Errorf("decode exercise %d: %w", e.ID, err)
	}
	e.CreatedAt, e.UpdatedAt = database.ParseTime(created), database.ParseTime(updated)
	return &e, nil
}

// Get returns an exercise owned by userID.
func (s *Service) Get(ctx context.Context, userID, id int64) (*Exercise, error) {
	e, err := scanExercise(s.db.QueryRowContext(ctx, selectExercise+` WHERE e.id = ? AND e.user_id = ?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return e, err
}

// List returns the user's exercises, newest first, optionally for one topic.
func (s *Service) List(ctx context.Context, userID, topicID int64) ([]*Exercise, error) {
	q := selectExercise + ` WHERE e.user_id = ?`
	args := []any{userID}
	if topicID != 0 {
		q += ` AND e.topic_id = ?`
		args = append(args, topicID)
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY e.created_at DESC, e.id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Exercise
	for rows.Next() {
		e, err := scanExercise(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// MarkCompleted marks an exercise as completed/understood. It is
// idempotent: progress is only counted the first time.
func (s *Service) MarkCompleted(ctx context.Context, u *users.User, id int64) error {
	e, err := s.Get(ctx, u.ID, id)
	if err != nil {
		return err
	}
	return database.InTx(ctx, s.db, func(tx *sql.Tx) error {
		return s.complete(ctx, tx, u.ID, e)
	})
}

// complete transitions an open exercise to completed inside tx.
func (s *Service) complete(ctx context.Context, tx *sql.Tx, userID int64, e *Exercise) error {
	res, err := tx.ExecContext(ctx, `UPDATE exercises SET status = ?, updated_at = ? WHERE id = ? AND user_id = ? AND status = ?`,
		StatusCompleted, database.Now(), e.ID, userID, StatusOpen)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil // already completed
	}
	if err := s.progress.Increment(ctx, tx, userID, e.TopicID, progress.ExercisesCompleted, 1); err != nil {
		return err
	}
	return s.progress.Log(ctx, tx, userID, e.TopicID, progress.KindExerciseCompleted,
		"Completed the exercise “"+e.Title+"”", fmt.Sprintf("/exercises/%d", e.ID))
}
