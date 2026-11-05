package progress

import "sort"

// Score weights. A topic is worth 100 points:
//   - 30 for completing the lesson,
//   - 35 for completing an exercise (15 if attempted but not yet completed),
//   - up to 35 from the best quiz score.
const (
	lessonWeight         = 30
	exerciseWeight       = 35
	exerciseAttemptValue = 15
	quizWeight           = 35
)

// Score computes a 0-100 mastery score for a topic from recorded activity.
func Score(tp TopicProgress) int {
	score := 0
	if tp.LessonCompleted {
		score += lessonWeight
	}
	switch {
	case tp.ExercisesCompleted > 0:
		score += exerciseWeight
	case tp.ExercisesAttempted > 0:
		score += exerciseAttemptValue
	}
	if tp.HasQuiz {
		score += clamp(tp.BestQuizPercent, 0, 100) * quizWeight / 100
	}
	return clamp(score, 0, 100)
}

// Totals summarizes progress across all of a language's topics.
type Totals struct {
	Topics             int
	TopicsStudied      int
	TopicsMastered     int // score >= 80
	LessonsCompleted   int
	QuestionsAsked     int
	ExercisesAttempted int
	ExercisesCompleted int
	HintsRequested     int
	QuizzesCompleted   int
	AverageQuizPercent int // over topics with at least one quiz; 0 if none
	OverallPercent     int // average topic score
}

// Summarize computes totals from per-topic progress.
func Summarize(tps []TopicProgress) Totals {
	t := Totals{Topics: len(tps)}
	scoreSum, quizSum, quizTopics := 0, 0, 0
	for _, tp := range tps {
		scoreSum += tp.Score
		if tp.Started {
			t.TopicsStudied++
		}
		if tp.Score >= 80 {
			t.TopicsMastered++
		}
		if tp.LessonCompleted {
			t.LessonsCompleted++
		}
		t.QuestionsAsked += tp.QuestionsAsked
		t.ExercisesAttempted += tp.ExercisesAttempted
		t.ExercisesCompleted += tp.ExercisesCompleted
		t.HintsRequested += tp.HintsRequested
		t.QuizzesCompleted += tp.QuizzesTaken
		if tp.HasQuiz {
			quizSum += tp.LastQuizPercent
			quizTopics++
		}
	}
	if len(tps) > 0 {
		t.OverallPercent = scoreSum / len(tps)
	}
	if quizTopics > 0 {
		t.AverageQuizPercent = quizSum / quizTopics
	}
	return t
}

// Recommend suggests up to n topics to study next: unfinished topics the
// student has already started come first, then new topics in learning-path
// order.
func Recommend(tps []TopicProgress, n int) []TopicProgress {
	var inProgress, fresh []TopicProgress
	for _, tp := range tps {
		switch {
		case tp.Score >= 70:
			continue
		case tp.Started:
			inProgress = append(inProgress, tp)
		default:
			fresh = append(fresh, tp)
		}
	}
	out := append(inProgress, fresh...)
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// Struggle is a topic where recorded activity suggests difficulty.
type Struggle struct {
	TopicProgress
	Reason string
}

// Struggles finds topics where the student appears to be having difficulty,
// most serious first. Only started topics are considered.
func Struggles(tps []TopicProgress) []Struggle {
	var out []Struggle
	for _, tp := range tps {
		if !tp.Started {
			continue
		}
		switch {
		case tp.HasQuiz && tp.LastQuizPercent < 60:
			out = append(out, Struggle{tp, "Your last quiz score was below 60%."})
		case tp.ExercisesAttempted >= 2 && tp.ExercisesCompleted == 0:
			out = append(out, Struggle{tp, "You have attempted exercises here but not completed one yet."})
		case tp.HintsRequested >= 3 && tp.HintsRequested > 2*tp.ExercisesCompleted+2:
			out = append(out, Struggle{tp, "You have needed several hints on this topic."})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score < out[j].Score })
	return out
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
