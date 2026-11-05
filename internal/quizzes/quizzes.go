// Package quizzes implements AI-generated quizzes, grading and results.
package quizzes

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
	ErrNotFound     = errors.New("quiz not found")
	ErrInvalidInput = errors.New("invalid quiz options")
)

// QuestionCounts are the quiz lengths students can choose.
var QuestionCounts = []int{5, 8, 10}

// Question is a stored quiz question including its answer key.
type Question struct {
	ID                int64
	Position          int
	Type              string
	Prompt            string
	Options           []string
	Answer            string
	AcceptedAnswers   []string
	Explanation       string
	WrongExplanations map[string]string
}

// Quiz is a stored quiz owned by one student.
type Quiz struct {
	ID          int64
	UserID      int64
	TopicID     int64
	TopicTitle  string
	TopicSlug   string
	Title       string
	Difficulty  string
	Questions   []Question
	Attempts    int
	BestPercent int
	CreatedAt   time.Time
}

// Service holds quiz business logic.
type Service struct {
	db       *sql.DB
	ai       ai.Tutor
	progress *progress.Store
}

// NewService creates a quiz service.
func NewService(db *sql.DB, tutor ai.Tutor, p *progress.Store) *Service {
	return &Service{db: db, ai: tutor, progress: p}
}

func validCount(n int) bool {
	for _, c := range QuestionCounts {
		if n == c {
			return true
		}
	}
	return false
}

// Generate asks the AI for a quiz and stores it with its answer key.
func (s *Service) Generate(ctx context.Context, u *users.User, t topics.Topic, difficulty string, count int) (int64, error) {
	if !users.ValidLevel(difficulty) || !validCount(count) || t.LanguageID != u.LanguageID {
		return 0, ErrInvalidInput
	}
	q, err := s.ai.GenerateQuiz(ctx, ai.QuizRequest{
		Learner:    ai.Learner{Language: u.LanguageName, Level: u.ExperienceLevel},
		Topic:      t.Title,
		Difficulty: difficulty,
		Count:      count,
	})
	if err != nil {
		return 0, err
	}

	var id int64
	err = database.InTx(ctx, s.db, func(tx *sql.Tx) error {
		now := database.Now()
		res, err := tx.ExecContext(ctx, `INSERT INTO quizzes (user_id, topic_id, title, difficulty, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`, u.ID, t.ID, q.Title, difficulty, now, now)
		if err != nil {
			return fmt.Errorf("insert quiz: %w", err)
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		for i, qu := range q.Questions {
			opts, _ := json.Marshal(nonNil(qu.Options))
			accepted, _ := json.Marshal(nonNil(qu.AcceptedAnswers))
			wrong, _ := json.Marshal(qu.WrongExplanations)
			if _, err := tx.ExecContext(ctx, `INSERT INTO quiz_questions
				(quiz_id, position, question_type, prompt, options_json, correct_answer, accepted_answers_json, explanation, wrong_explanations_json)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, i+1, qu.Type, qu.Prompt, string(opts), qu.Answer,
				string(accepted), qu.Explanation, string(wrong)); err != nil {
				return fmt.Errorf("insert question: %w", err)
			}
		}
		return s.progress.Log(ctx, tx, u.ID, t.ID, progress.KindQuizGenerated,
			"New quiz: "+q.Title, fmt.Sprintf("/quizzes/%d", id))
	})
	return id, err
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

const selectQuiz = `SELECT q.id, q.user_id, q.topic_id, t.title, t.slug, q.title, q.difficulty, q.created_at,
	(SELECT COUNT(*) FROM quiz_attempts a WHERE a.quiz_id = q.id),
	COALESCE((SELECT MAX(percent) FROM quiz_attempts a WHERE a.quiz_id = q.id), 0)
	FROM quizzes q JOIN topics t ON t.id = q.topic_id`

func scanQuiz(row interface{ Scan(...any) error }) (*Quiz, error) {
	var q Quiz
	var created string
	if err := row.Scan(&q.ID, &q.UserID, &q.TopicID, &q.TopicTitle, &q.TopicSlug, &q.Title, &q.Difficulty,
		&created, &q.Attempts, &q.BestPercent); err != nil {
		return nil, err
	}
	q.CreatedAt = database.ParseTime(created)
	return &q, nil
}

// Get returns a quiz owned by userID, including its questions.
func (s *Service) Get(ctx context.Context, userID, quizID int64) (*Quiz, error) {
	q, err := scanQuiz(s.db.QueryRowContext(ctx, selectQuiz+` WHERE q.id = ? AND q.user_id = ?`, quizID, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, position, question_type, prompt, options_json, correct_answer,
		accepted_answers_json, explanation, wrong_explanations_json
		FROM quiz_questions WHERE quiz_id = ? ORDER BY position`, q.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var qu Question
		var opts, accepted, wrong string
		if err := rows.Scan(&qu.ID, &qu.Position, &qu.Type, &qu.Prompt, &opts, &qu.Answer, &accepted,
			&qu.Explanation, &wrong); err != nil {
			return nil, err
		}
		if err := errors.Join(json.Unmarshal([]byte(opts), &qu.Options),
			json.Unmarshal([]byte(accepted), &qu.AcceptedAnswers),
			json.Unmarshal([]byte(wrong), &qu.WrongExplanations)); err != nil {
			return nil, fmt.Errorf("decode question %d: %w", qu.ID, err)
		}
		q.Questions = append(q.Questions, qu)
	}
	return q, rows.Err()
}

// List returns the user's quizzes, newest first.
func (s *Service) List(ctx context.Context, userID int64) ([]*Quiz, error) {
	rows, err := s.db.QueryContext(ctx, selectQuiz+` WHERE q.user_id = ? ORDER BY q.created_at DESC, q.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Quiz
	for rows.Next() {
		q, err := scanQuiz(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}
