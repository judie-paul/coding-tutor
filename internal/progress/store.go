// Package progress records learning activity and computes progress from it.
// All statistics come from recorded student actions, never from the AI.
package progress

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"codingtutor/internal/database"
)

// Activity kinds written to activity_logs.
const (
	KindLessonCompleted     = "lesson_completed"
	KindConversationStarted = "conversation_started"
	KindQuestionAsked       = "question_asked"
	KindExerciseGenerated   = "exercise_generated"
	KindHintRequested       = "hint_requested"
	KindSolutionRevealed    = "solution_revealed"
	KindCodeSubmitted       = "code_submitted"
	KindExerciseCompleted   = "exercise_completed"
	KindQuizGenerated       = "quiz_generated"
	KindQuizCompleted       = "quiz_completed"
)

// Counter is a learning_progress column that can be incremented.
type Counter string

// Counters that can be incremented. Only these names are ever placed in SQL.
const (
	QuestionsAsked     Counter = "questions_asked"
	ExercisesAttempted Counter = "exercises_attempted"
	ExercisesCompleted Counter = "exercises_completed"
	HintsRequested     Counter = "hints_requested"
)

func (c Counter) valid() bool {
	switch c {
	case QuestionsAsked, ExercisesAttempted, ExercisesCompleted, HintsRequested:
		return true
	}
	return false
}

// Store reads and writes progress data.
type Store struct{ db *sql.DB }

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// DB exposes the database for callers that need a transaction.
func (s *Store) DB() *sql.DB { return s.db }

// Log appends an entry to the student's activity history. topicID may be 0.
func (s *Store) Log(ctx context.Context, tx database.DBTX, userID, topicID int64, kind, description, link string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO activity_logs (user_id, topic_id, kind, description, link, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, userID, nullID(topicID), kind, description, link, database.Now())
	if err != nil {
		return fmt.Errorf("log activity: %w", err)
	}
	return nil
}

// ensureRow creates the (user, topic) progress row if it does not exist.
func ensureRow(ctx context.Context, tx database.DBTX, userID, topicID int64) error {
	now := database.Now()
	_, err := tx.ExecContext(ctx, `INSERT INTO learning_progress (user_id, topic_id, last_activity_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT (user_id, topic_id) DO NOTHING`, userID, topicID, now, now, now)
	return err
}

// Increment adds n to a counter for the user's topic.
func (s *Store) Increment(ctx context.Context, tx database.DBTX, userID, topicID int64, c Counter, n int) error {
	if !c.valid() {
		return fmt.Errorf("unknown progress counter %q", c)
	}
	if topicID == 0 {
		return nil
	}
	if err := ensureRow(ctx, tx, userID, topicID); err != nil {
		return err
	}
	now := database.Now()
	_, err := tx.ExecContext(ctx, `UPDATE learning_progress SET `+string(c)+` = `+string(c)+` + ?,
		last_activity_at = ?, updated_at = ? WHERE user_id = ? AND topic_id = ?`, n, now, now, userID, topicID)
	if err != nil {
		return fmt.Errorf("increment %s: %w", c, err)
	}
	return nil
}

// SetLessonCompleted marks or unmarks the topic's lesson as completed.
func (s *Store) SetLessonCompleted(ctx context.Context, tx database.DBTX, userID, topicID int64, done bool) error {
	if err := ensureRow(ctx, tx, userID, topicID); err != nil {
		return err
	}
	now := database.Now()
	_, err := tx.ExecContext(ctx, `UPDATE learning_progress SET lesson_completed = ?, last_activity_at = ?, updated_at = ?
		WHERE user_id = ? AND topic_id = ?`, boolInt(done), now, now, userID, topicID)
	return err
}

// RecordQuiz stores a completed quiz score for the topic.
func (s *Store) RecordQuiz(ctx context.Context, tx database.DBTX, userID, topicID int64, percent int) error {
	if err := ensureRow(ctx, tx, userID, topicID); err != nil {
		return err
	}
	now := database.Now()
	_, err := tx.ExecContext(ctx, `UPDATE learning_progress SET
		quizzes_taken = quizzes_taken + 1,
		last_quiz_percent = ?,
		best_quiz_percent = MAX(COALESCE(best_quiz_percent, 0), ?),
		last_activity_at = ?, updated_at = ?
		WHERE user_id = ? AND topic_id = ?`, percent, percent, now, now, userID, topicID)
	return err
}

// TopicProgress is a student's recorded progress on one topic.
type TopicProgress struct {
	TopicID            int64
	Slug               string
	Title              string
	Difficulty         string
	Position           int
	Started            bool
	LessonCompleted    bool
	QuestionsAsked     int
	ExercisesAttempted int
	ExercisesCompleted int
	HintsRequested     int
	QuizzesTaken       int
	BestQuizPercent    int  // valid when HasQuiz
	LastQuizPercent    int  // valid when HasQuiz
	HasQuiz            bool // at least one quiz completed
	LastActivity       time.Time
	Score              int // 0-100, see Score
}

// ByTopic returns progress for every topic of a language (including topics
// the student has not started) in learning-path order.
func (s *Store) ByTopic(ctx context.Context, userID, languageID int64) ([]TopicProgress, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.slug, t.title, t.difficulty, t.position,
		p.id IS NOT NULL, COALESCE(p.lesson_completed, 0), COALESCE(p.questions_asked, 0),
		COALESCE(p.exercises_attempted, 0), COALESCE(p.exercises_completed, 0),
		COALESCE(p.hints_requested, 0), COALESCE(p.quizzes_taken, 0),
		p.best_quiz_percent, p.last_quiz_percent, COALESCE(p.last_activity_at, '')
		FROM topics t
		LEFT JOIN learning_progress p ON p.topic_id = t.id AND p.user_id = ?
		WHERE t.language_id = ?
		ORDER BY t.position`, userID, languageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TopicProgress
	for rows.Next() {
		var tp TopicProgress
		var best, last sql.NullInt64
		var lastAct string
		if err := rows.Scan(&tp.TopicID, &tp.Slug, &tp.Title, &tp.Difficulty, &tp.Position,
			&tp.Started, &tp.LessonCompleted, &tp.QuestionsAsked, &tp.ExercisesAttempted,
			&tp.ExercisesCompleted, &tp.HintsRequested, &tp.QuizzesTaken, &best, &last, &lastAct); err != nil {
			return nil, err
		}
		tp.HasQuiz = best.Valid
		tp.BestQuizPercent = int(best.Int64)
		tp.LastQuizPercent = int(last.Int64)
		tp.LastActivity = database.ParseTime(lastAct)
		tp.Score = Score(tp)
		out = append(out, tp)
	}
	return out, rows.Err()
}

// ForTopic returns the student's progress on a single topic.
func (s *Store) ForTopic(ctx context.Context, userID, languageID, topicID int64) (TopicProgress, error) {
	all, err := s.ByTopic(ctx, userID, languageID)
	if err != nil {
		return TopicProgress{}, err
	}
	for _, tp := range all {
		if tp.TopicID == topicID {
			return tp, nil
		}
	}
	return TopicProgress{}, sql.ErrNoRows
}

// Activity is one entry in the student's learning history.
type Activity struct {
	Kind        string
	Description string
	Link        string
	TopicTitle  string
	CreatedAt   time.Time
}

// RecentActivity returns the student's latest activity, newest first.
func (s *Store) RecentActivity(ctx context.Context, userID int64, limit, offset int) ([]Activity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.kind, a.description, a.link, COALESCE(t.title, ''), a.created_at
		FROM activity_logs a LEFT JOIN topics t ON t.id = a.topic_id
		WHERE a.user_id = ? ORDER BY a.created_at DESC, a.id DESC LIMIT ? OFFSET ?`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Activity
	for rows.Next() {
		var a Activity
		var created string
		if err := rows.Scan(&a.Kind, &a.Description, &a.Link, &a.TopicTitle, &created); err != nil {
			return nil, err
		}
		a.CreatedAt = database.ParseTime(created)
		out = append(out, a)
	}
	return out, rows.Err()
}

// CountActivity returns the total number of activity entries for a user.
func (s *Store) CountActivity(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_logs WHERE user_id = ?`, userID).Scan(&n)
	return n, err
}

func nullID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
