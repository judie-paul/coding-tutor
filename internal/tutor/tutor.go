// Package tutor implements AI tutoring conversations.
package tutor

import (
	"context"
	"database/sql"
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

// ErrNotFound is returned for conversations that do not exist or belong to
// another user.
var ErrNotFound = errors.New("conversation not found")

// ErrInvalidQuestion is returned for empty or overly long questions.
var ErrInvalidQuestion = errors.New("invalid question")

// MaxQuestionLength is the longest question a student can send.
const MaxQuestionLength = 4000

// Conversation is a tutoring thread owned by one student.
type Conversation struct {
	ID           int64
	UserID       int64
	TopicID      int64 // 0 for general conversations
	TopicTitle   string
	TopicSlug    string
	Title        string
	MessageCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Message is one turn in a conversation.
type Message struct {
	ID        int64
	Role      string // user or assistant
	Content   string
	CreatedAt time.Time
}

// Service holds tutoring business logic.
type Service struct {
	db       *sql.DB
	ai       ai.Tutor
	progress *progress.Store
}

// NewService creates a tutoring service.
func NewService(db *sql.DB, tutor ai.Tutor, p *progress.Store) *Service {
	return &Service{db: db, ai: tutor, progress: p}
}

// LearnerFor builds the AI learner profile for a user.
func LearnerFor(u *users.User) ai.Learner {
	return ai.Learner{Language: u.LanguageName, Level: u.ExperienceLevel}
}

func validateQuestion(q string) (string, error) {
	q = strings.TrimSpace(q)
	if q == "" || utf8.RuneCountInString(q) > MaxQuestionLength {
		return "", ErrInvalidQuestion
	}
	return q, nil
}

// titleFrom derives a conversation title from the first question.
func titleFrom(q string) string {
	q = strings.Join(strings.Fields(q), " ")
	if utf8.RuneCountInString(q) <= 70 {
		return q
	}
	r := []rune(q)[:70]
	if i := strings.LastIndex(string(r), " "); i > 40 {
		return string(r)[:i] + "…"
	}
	return string(r) + "…"
}

// Start creates a conversation by asking the first question. Nothing is
// stored unless the AI answers successfully.
func (s *Service) Start(ctx context.Context, u *users.User, topicID int64, topicTitle, question string) (int64, error) {
	q, err := validateQuestion(question)
	if err != nil {
		return 0, err
	}
	answer, err := s.ai.Explain(ctx, ai.ExplainRequest{Learner: LearnerFor(u), Topic: topicTitle, Question: q})
	if err != nil {
		return 0, err
	}

	var convID int64
	err = database.InTx(ctx, s.db, func(tx *sql.Tx) error {
		now := database.Now()
		res, err := tx.ExecContext(ctx, `INSERT INTO conversations (user_id, topic_id, title, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)`, u.ID, nullID(topicID), titleFrom(q), now, now)
		if err != nil {
			return fmt.Errorf("create conversation: %w", err)
		}
		if convID, err = res.LastInsertId(); err != nil {
			return err
		}
		if err := addMessages(ctx, tx, convID, q, answer); err != nil {
			return err
		}
		if err := s.progress.Increment(ctx, tx, u.ID, topicID, progress.QuestionsAsked, 1); err != nil {
			return err
		}
		return s.progress.Log(ctx, tx, u.ID, topicID, progress.KindConversationStarted,
			"Started a conversation: "+titleFrom(q), fmt.Sprintf("/tutor/c/%d", convID))
	})
	return convID, err
}

// Ask sends a follow-up question in an existing conversation and returns
// the stored user and assistant messages.
func (s *Service) Ask(ctx context.Context, u *users.User, convID int64, question string) (Message, Message, error) {
	q, err := validateQuestion(question)
	if err != nil {
		return Message{}, Message{}, err
	}
	conv, err := s.Get(ctx, u.ID, convID)
	if err != nil {
		return Message{}, Message{}, err
	}
	history, err := s.Messages(ctx, u.ID, convID)
	if err != nil {
		return Message{}, Message{}, err
	}
	aiHistory := make([]ai.Message, 0, len(history))
	for _, m := range history {
		aiHistory = append(aiHistory, ai.Message{Role: m.Role, Content: m.Content})
	}

	answer, err := s.ai.FollowUp(ctx, ai.FollowUpRequest{
		Learner: LearnerFor(u), Topic: conv.TopicTitle, History: aiHistory, Question: q,
	})
	if err != nil {
		return Message{}, Message{}, err
	}

	now := time.Now().UTC().Truncate(time.Second)
	err = database.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := addMessages(ctx, tx, convID, q, answer); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE conversations SET updated_at = ? WHERE id = ? AND user_id = ?`,
			database.FormatTime(now), convID, u.ID); err != nil {
			return err
		}
		if err := s.progress.Increment(ctx, tx, u.ID, conv.TopicID, progress.QuestionsAsked, 1); err != nil {
			return err
		}
		return s.progress.Log(ctx, tx, u.ID, conv.TopicID, progress.KindQuestionAsked,
			"Asked a follow-up in “"+conv.Title+"”", fmt.Sprintf("/tutor/c/%d", convID))
	})
	if err != nil {
		return Message{}, Message{}, err
	}
	return Message{Role: "user", Content: q, CreatedAt: now},
		Message{Role: "assistant", Content: answer, CreatedAt: now}, nil
}

func addMessages(ctx context.Context, tx *sql.Tx, convID int64, question, answer string) error {
	now := database.Now()
	for _, m := range [][2]string{{"user", question}, {"assistant", answer}} {
		if _, err := tx.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, created_at) VALUES (?, ?, ?, ?)`,
			convID, m[0], m[1], now); err != nil {
			return fmt.Errorf("insert message: %w", err)
		}
	}
	return nil
}

const selectConversation = `SELECT c.id, c.user_id, COALESCE(c.topic_id, 0), COALESCE(t.title, ''), COALESCE(t.slug, ''),
	c.title, (SELECT COUNT(*) FROM messages m WHERE m.conversation_id = c.id), c.created_at, c.updated_at
	FROM conversations c LEFT JOIN topics t ON t.id = c.topic_id`

func scanConversation(row interface{ Scan(...any) error }) (Conversation, error) {
	var c Conversation
	var created, updated string
	err := row.Scan(&c.ID, &c.UserID, &c.TopicID, &c.TopicTitle, &c.TopicSlug, &c.Title, &c.MessageCount, &created, &updated)
	c.CreatedAt, c.UpdatedAt = database.ParseTime(created), database.ParseTime(updated)
	return c, err
}

// Get returns a conversation owned by userID.
func (s *Service) Get(ctx context.Context, userID, convID int64) (Conversation, error) {
	c, err := scanConversation(s.db.QueryRowContext(ctx, selectConversation+` WHERE c.id = ? AND c.user_id = ?`, convID, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// List returns the user's conversations, most recently active first.
// limit <= 0 returns all of them.
func (s *Service) List(ctx context.Context, userID int64, limit int) ([]Conversation, error) {
	if limit <= 0 {
		limit = -1
	}
	rows, err := s.db.QueryContext(ctx, selectConversation+` WHERE c.user_id = ? ORDER BY c.updated_at DESC, c.id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		c, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Messages returns a conversation's messages, oldest first. The ownership
// check is part of the query.
func (s *Service) Messages(ctx context.Context, userID, convID int64) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT m.id, m.role, m.content, m.created_at FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE m.conversation_id = ? AND c.user_id = ? ORDER BY m.id`, convID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var created string
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &created); err != nil {
			return nil, err
		}
		m.CreatedAt = database.ParseTime(created)
		out = append(out, m)
	}
	return out, rows.Err()
}

// Delete removes a conversation owned by userID.
func (s *Service) Delete(ctx context.Context, userID, convID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM conversations WHERE id = ? AND user_id = ?`, convID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func nullID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}
