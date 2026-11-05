package quizzes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"codingtutor/internal/ai"
	"codingtutor/internal/database"
	"codingtutor/internal/progress"
	"codingtutor/internal/users"
)

// ErrAttemptNotFound is returned for attempts that do not exist or belong
// to another user.
var ErrAttemptNotFound = errors.New("attempt not found")

// IsCorrect grades a single answer. Multiple-choice and true/false answers
// must match the correct option; short answers match the answer or any
// accepted alternative, ignoring case, spacing and trailing punctuation.
func IsCorrect(q Question, answer string) bool {
	given := ai.NormalizeAnswer(answer)
	if given == "" {
		return false
	}
	if given == ai.NormalizeAnswer(q.Answer) {
		return true
	}
	if q.Type == ai.ShortAnswer {
		for _, alt := range q.AcceptedAnswers {
			if given == ai.NormalizeAnswer(alt) {
				return true
			}
		}
	}
	return false
}

// Score holds the result of grading a set of answers.
type Score struct {
	Correct int
	Total   int
	Percent int
	Results map[int64]bool // question ID -> correct
}

// Grade scores answers (question ID -> answer) against the quiz's key.
// Unanswered questions count as incorrect.
func Grade(questions []Question, answers map[int64]string) Score {
	s := Score{Total: len(questions), Results: make(map[int64]bool, len(questions))}
	for _, q := range questions {
		ok := IsCorrect(q, answers[q.ID])
		s.Results[q.ID] = ok
		if ok {
			s.Correct++
		}
	}
	if s.Total > 0 {
		s.Percent = (s.Correct*100 + s.Total/2) / s.Total // rounded
	}
	return s
}

// Submit grades the student's answers and records the attempt and progress
// in one transaction.
func (s *Service) Submit(ctx context.Context, u *users.User, quizID int64, answers map[int64]string) (int64, error) {
	q, err := s.Get(ctx, u.ID, quizID)
	if err != nil {
		return 0, err
	}
	// Only keep answers to this quiz's questions, with a sane length.
	clean := make(map[int64]string, len(q.Questions))
	for _, qu := range q.Questions {
		a := answers[qu.ID]
		if len(a) > 500 {
			a = a[:500]
		}
		clean[qu.ID] = a
	}
	score := Grade(q.Questions, clean)

	var attemptID int64
	err = database.InTx(ctx, s.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO quiz_attempts (quiz_id, user_id, score, total, percent, completed_at)
			VALUES (?, ?, ?, ?, ?, ?)`, q.ID, u.ID, score.Correct, score.Total, score.Percent, database.Now())
		if err != nil {
			return fmt.Errorf("insert attempt: %w", err)
		}
		if attemptID, err = res.LastInsertId(); err != nil {
			return err
		}
		for _, qu := range q.Questions {
			if _, err := tx.ExecContext(ctx, `INSERT INTO quiz_answers (attempt_id, question_id, answer, is_correct) VALUES (?, ?, ?, ?)`,
				attemptID, qu.ID, clean[qu.ID], score.Results[qu.ID]); err != nil {
				return fmt.Errorf("insert answer: %w", err)
			}
		}
		if err := s.progress.RecordQuiz(ctx, tx, u.ID, q.TopicID, score.Percent); err != nil {
			return err
		}
		return s.progress.Log(ctx, tx, u.ID, q.TopicID, progress.KindQuizCompleted,
			fmt.Sprintf("Scored %d%% on “%s”", score.Percent, q.Title), fmt.Sprintf("/quizzes/attempts/%d", attemptID))
	})
	return attemptID, err
}

// QuestionResult pairs a question with the student's answer.
type QuestionResult struct {
	Question
	Given   string
	Correct bool
	// WhyWrong explains why the chosen answer is wrong, when known.
	WhyWrong string
}

// Attempt is a graded quiz attempt.
type Attempt struct {
	ID          int64
	Quiz        *Quiz
	Score       int
	Total       int
	Percent     int
	CompletedAt time.Time
	Results     []QuestionResult
}

// GetAttempt returns a graded attempt owned by userID.
func (s *Service) GetAttempt(ctx context.Context, userID, attemptID int64) (*Attempt, error) {
	var a Attempt
	var quizID int64
	var completed string
	err := s.db.QueryRowContext(ctx, `SELECT id, quiz_id, score, total, percent, completed_at
		FROM quiz_attempts WHERE id = ? AND user_id = ?`, attemptID, userID).
		Scan(&a.ID, &quizID, &a.Score, &a.Total, &a.Percent, &completed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAttemptNotFound
	}
	if err != nil {
		return nil, err
	}
	a.CompletedAt = database.ParseTime(completed)
	if a.Quiz, err = s.Get(ctx, userID, quizID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `SELECT question_id, answer, is_correct FROM quiz_answers WHERE attempt_id = ?`, a.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	given := map[int64]string{}
	correct := map[int64]bool{}
	for rows.Next() {
		var qid int64
		var ans string
		var ok bool
		if err := rows.Scan(&qid, &ans, &ok); err != nil {
			return nil, err
		}
		given[qid], correct[qid] = ans, ok
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, q := range a.Quiz.Questions {
		r := QuestionResult{Question: q, Given: given[q.ID], Correct: correct[q.ID]}
		if !r.Correct {
			for opt, why := range q.WrongExplanations {
				if ai.NormalizeAnswer(opt) == ai.NormalizeAnswer(r.Given) {
					r.WhyWrong = why
				}
			}
		}
		a.Results = append(a.Results, r)
	}
	return &a, nil
}

// AttemptSummary is a row in a list of recent quiz results.
type AttemptSummary struct {
	ID          int64
	QuizID      int64
	QuizTitle   string
	TopicTitle  string
	Percent     int
	Score       int
	Total       int
	CompletedAt time.Time
}

// RecentAttempts lists the user's latest quiz attempts.
func (s *Service) RecentAttempts(ctx context.Context, userID int64, limit int) ([]AttemptSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.id, q.id, q.title, t.title, a.percent, a.score, a.total, a.completed_at
		FROM quiz_attempts a JOIN quizzes q ON q.id = a.quiz_id JOIN topics t ON t.id = q.topic_id
		WHERE a.user_id = ? ORDER BY a.completed_at DESC, a.id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AttemptSummary
	for rows.Next() {
		var a AttemptSummary
		var completed string
		if err := rows.Scan(&a.ID, &a.QuizID, &a.QuizTitle, &a.TopicTitle, &a.Percent, &a.Score, &a.Total, &completed); err != nil {
			return nil, err
		}
		a.CompletedAt = database.ParseTime(completed)
		out = append(out, a)
	}
	return out, rows.Err()
}
