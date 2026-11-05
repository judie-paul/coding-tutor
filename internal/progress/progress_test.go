package progress

import (
	"context"
	"testing"

	"codingtutor/internal/testutil"
)

func TestScore(t *testing.T) {
	tests := []struct {
		name string
		tp   TopicProgress
		want int
	}{
		{"nothing", TopicProgress{}, 0},
		{"lesson only", TopicProgress{LessonCompleted: true}, 30},
		{"attempted exercise", TopicProgress{ExercisesAttempted: 1}, 15},
		{"completed exercise", TopicProgress{ExercisesAttempted: 3, ExercisesCompleted: 1}, 35},
		{"quiz 80%", TopicProgress{HasQuiz: true, BestQuizPercent: 80}, 28},
		{"everything perfect", TopicProgress{LessonCompleted: true, ExercisesCompleted: 2, HasQuiz: true, BestQuizPercent: 100}, 100},
		{"bad quiz data is clamped", TopicProgress{HasQuiz: true, BestQuizPercent: 250}, 35},
	}
	for _, tt := range tests {
		if got := Score(tt.tp); got != tt.want {
			t.Errorf("%s: Score = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestSummarize(t *testing.T) {
	tps := []TopicProgress{
		{Started: true, LessonCompleted: true, ExercisesAttempted: 2, ExercisesCompleted: 1, HasQuiz: true, BestQuizPercent: 100, LastQuizPercent: 80, QuizzesTaken: 2},
		{Started: true, HintsRequested: 3, ExercisesAttempted: 1},
		{},
	}
	for i := range tps {
		tps[i].Score = Score(tps[i])
	}
	got := Summarize(tps)
	if got.Topics != 3 || got.TopicsStudied != 2 || got.LessonsCompleted != 1 ||
		got.ExercisesAttempted != 3 || got.ExercisesCompleted != 1 || got.QuizzesCompleted != 2 ||
		got.HintsRequested != 3 || got.TopicsMastered != 1 {
		t.Errorf("unexpected totals: %+v", got)
	}
	if got.OverallPercent != (100+15+0)/3 {
		t.Errorf("OverallPercent = %d", got.OverallPercent)
	}
	if got.AverageQuizPercent != 80 {
		t.Errorf("AverageQuizPercent = %d", got.AverageQuizPercent)
	}
	if Summarize(nil).OverallPercent != 0 {
		t.Error("empty summary should be zero")
	}
}

func TestRecommendAndStruggles(t *testing.T) {
	tps := []TopicProgress{
		{TopicID: 1, Started: true, Score: 100},
		{TopicID: 2},
		{TopicID: 3, Started: true, Score: 30, HasQuiz: true, LastQuizPercent: 40},
		{TopicID: 4},
		{TopicID: 5, Started: true, Score: 15, ExercisesAttempted: 2},
	}
	rec := Recommend(tps, 3)
	if len(rec) != 3 || rec[0].TopicID != 3 || rec[1].TopicID != 5 || rec[2].TopicID != 2 {
		t.Errorf("unexpected recommendations: %+v", rec)
	}
	st := Struggles(tps)
	if len(st) != 2 || st[0].TopicID != 5 || st[1].TopicID != 3 {
		t.Errorf("unexpected struggles: %+v", st)
	}
}

func TestStoreCountersAndActivity(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewDB(t)
	db.Exec(`INSERT INTO users (name, email, password_hash, language_id, created_at, updated_at) VALUES ('U', 'u@x.io', 'h', 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	s := NewStore(db)

	var topicID int64
	db.QueryRow(`SELECT id FROM topics WHERE slug = 'maps'`).Scan(&topicID)

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.Increment(ctx, db, 1, topicID, ExercisesAttempted, 1))
	must(s.Increment(ctx, db, 1, topicID, HintsRequested, 2))
	must(s.SetLessonCompleted(ctx, db, 1, topicID, true))
	must(s.RecordQuiz(ctx, db, 1, topicID, 90))
	must(s.RecordQuiz(ctx, db, 1, topicID, 50))
	must(s.Log(ctx, db, 1, topicID, KindQuizCompleted, "Scored 50%", "/quizzes/1"))
	if err := s.Increment(ctx, db, 1, topicID, Counter("password_hash"), 1); err == nil {
		t.Fatal("unknown counters must be rejected")
	}

	tp, err := s.ForTopic(ctx, 1, 1, topicID)
	must(err)
	if !tp.Started || !tp.LessonCompleted || tp.ExercisesAttempted != 1 || tp.HintsRequested != 2 ||
		tp.QuizzesTaken != 2 || tp.BestQuizPercent != 90 || tp.LastQuizPercent != 50 {
		t.Errorf("unexpected topic progress: %+v", tp)
	}
	if tp.Score != 30+15+31 {
		t.Errorf("score = %d", tp.Score)
	}

	acts, err := s.RecentActivity(ctx, 1, 10, 0)
	must(err)
	if len(acts) != 1 || acts[0].TopicTitle != "Maps" {
		t.Errorf("unexpected activity: %+v", acts)
	}
	// Another user's activity must not be visible.
	if acts, _ := s.RecentActivity(ctx, 2, 10, 0); len(acts) != 0 {
		t.Error("activity leaked across users")
	}
}
