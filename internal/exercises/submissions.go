package exercises

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"codingtutor/internal/ai"
	"codingtutor/internal/database"
	"codingtutor/internal/progress"
	"codingtutor/internal/users"
)

// MaxCodeLength limits the size of a code submission (in characters).
const MaxCodeLength = 20000

// Submission errors.
var (
	ErrInvalidCode        = errors.New("invalid code submission")
	ErrSubmissionNotFound = errors.New("submission not found")
)

// Submission is a piece of student code and its AI review. The student's
// code is stored exactly as submitted and is never executed.
type Submission struct {
	ID            int64
	ExerciseID    int64 // 0 for free-form reviews
	ExerciseTitle string
	Code          string
	Review        ai.Review
	LooksCorrect  bool
	CreatedAt     time.Time
}

// Lines splits the submitted code into lines for display with numbers.
func (s *Submission) Lines() []string {
	return strings.Split(strings.ReplaceAll(s.Code, "\r\n", "\n"), "\n")
}

func validateCode(code string) error {
	if strings.TrimSpace(code) == "" || utf8.RuneCountInString(code) > MaxCodeLength {
		return ErrInvalidCode
	}
	return nil
}

// Submit sends code for an AI review and stores the result. exerciseID may
// be 0 for a free-form review not tied to an exercise.
func (s *Service) Submit(ctx context.Context, u *users.User, exerciseID int64, code string) (int64, error) {
	if err := validateCode(code); err != nil {
		return 0, err
	}
	if u.LanguageID == 0 {
		return 0, ErrInvalidInput
	}
	var ex *Exercise
	if exerciseID != 0 {
		var err error
		if ex, err = s.Get(ctx, u.ID, exerciseID); err != nil {
			return 0, err
		}
	}

	req := ai.ReviewRequest{Learner: learner(u), Code: code}
	if ex != nil {
		req.Exercise = &ex.Content
	}
	review, err := s.ai.ReviewCode(ctx, req)
	if err != nil {
		return 0, err
	}
	reviewJSON, err := json.Marshal(review)
	if err != nil {
		return 0, err
	}

	var id int64
	err = database.InTx(ctx, s.db, func(tx *sql.Tx) error {
		var exArg any
		var topicID int64
		if ex != nil {
			exArg, topicID = ex.ID, ex.TopicID
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO exercise_submissions
			(user_id, exercise_id, language_id, code, review_json, looks_correct, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, u.ID, exArg, u.LanguageID, code, string(reviewJSON),
			review.LooksCorrect, database.Now())
		if err != nil {
			return fmt.Errorf("insert submission: %w", err)
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}

		link := fmt.Sprintf("/review/%d", id)
		if ex == nil {
			return s.progress.Log(ctx, tx, u.ID, 0, progress.KindCodeSubmitted, "Requested a code review", link)
		}
		// The first submission counts as an attempt at the exercise.
		if ex.SubmissionCount == 0 {
			if err := s.progress.Increment(ctx, tx, u.ID, topicID, progress.ExercisesAttempted, 1); err != nil {
				return err
			}
		}
		ex.SubmissionCount++
		if err := s.progress.Log(ctx, tx, u.ID, topicID, progress.KindCodeSubmitted,
			"Submitted a solution to “"+ex.Title+"”", link); err != nil {
			return err
		}
		if review.LooksCorrect {
			return s.complete(ctx, tx, u.ID, ex)
		}
		return nil
	})
	return id, err
}

const selectSubmission = `SELECT s.id, COALESCE(s.exercise_id, 0), COALESCE(e.title, ''), s.code, s.review_json,
	s.looks_correct, s.created_at
	FROM exercise_submissions s LEFT JOIN exercises e ON e.id = s.exercise_id`

func scanSubmission(row interface{ Scan(...any) error }) (*Submission, error) {
	var sub Submission
	var reviewJSON, created string
	if err := row.Scan(&sub.ID, &sub.ExerciseID, &sub.ExerciseTitle, &sub.Code, &reviewJSON, &sub.LooksCorrect, &created); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(reviewJSON), &sub.Review); err != nil {
		return nil, fmt.Errorf("decode review %d: %w", sub.ID, err)
	}
	sub.CreatedAt = database.ParseTime(created)
	return &sub, nil
}

// GetSubmission returns a submission owned by userID.
func (s *Service) GetSubmission(ctx context.Context, userID, id int64) (*Submission, error) {
	sub, err := scanSubmission(s.db.QueryRowContext(ctx, selectSubmission+` WHERE s.id = ? AND s.user_id = ?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSubmissionNotFound
	}
	return sub, err
}

// Submissions lists the user's submissions, newest first. With exerciseID 0
// it lists all submissions; limit <= 0 means no limit.
func (s *Service) Submissions(ctx context.Context, userID, exerciseID int64, limit int) ([]*Submission, error) {
	q := selectSubmission + ` WHERE s.user_id = ?`
	args := []any{userID}
	if exerciseID != 0 {
		q += ` AND s.exercise_id = ?`
		args = append(args, exerciseID)
	}
	if limit <= 0 {
		limit = -1
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY s.created_at DESC, s.id DESC LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Submission
	for rows.Next() {
		sub, err := scanSubmission(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}
