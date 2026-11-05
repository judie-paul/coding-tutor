package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
)

// ---------------------------------------------------------------- Exercises

// Exercise types students can choose from.
var ExerciseTypes = []string{"function", "program", "debugging", "refactoring"}

// ExerciseTypeLabel returns a friendly label for an exercise type.
func ExerciseTypeLabel(t string) string {
	switch t {
	case "function":
		return "Write a function"
	case "program":
		return "Build a small program"
	case "debugging":
		return "Fix the bug"
	case "refactoring":
		return "Improve existing code"
	}
	return t
}

// ValidExerciseType reports whether t is a supported exercise type.
func ValidExerciseType(t string) bool {
	for _, v := range ExerciseTypes {
		if t == v {
			return true
		}
	}
	return false
}

// Exercise is a validated, AI-generated programming exercise.
type Exercise struct {
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Objective    string    `json:"learning_objective"`
	Requirements []string  `json:"requirements"`
	Examples     []Example `json:"examples"`
	Constraints  []string  `json:"constraints"`
	Hints        []string  `json:"hints"`
	Concepts     []string  `json:"concepts"`
	StarterCode  string    `json:"starter_code"`
}

// Example shows an input and its expected output.
type Example struct {
	Input       string `json:"input"`
	Output      string `json:"output"`
	Explanation string `json:"explanation"`
}

// ExerciseRequest describes the exercise to generate.
type ExerciseRequest struct {
	Learner    Learner
	Topic      string
	Difficulty string
	Type       string
}

// GenerateExercise creates a new exercise.
func (s *Service) GenerateExercise(ctx context.Context, req ExerciseRequest) (*Exercise, error) {
	msgs := []Message{
		{Role: "system", Content: exerciseSystemPrompt(req.Learner)},
		{Role: "user", Content: exerciseUserPrompt(req)},
	}
	return structured(ctx, s, "exercise", msgs, maxTokensExercise, ParseExercise)
}

// ParseExercise decodes and validates an exercise document.
func ParseExercise(raw string) (*Exercise, error) {
	var ex Exercise
	if err := decodeJSON(raw, &ex); err != nil {
		return nil, err
	}
	ex.Title = clean(ex.Title, 120)
	ex.Description = clean(ex.Description, 4000)
	ex.Objective = clean(ex.Objective, 600)
	ex.Requirements = cleanList(ex.Requirements, 12, 400)
	ex.Constraints = cleanList(ex.Constraints, 10, 400)
	ex.Hints = cleanList(ex.Hints, 3, 800)
	ex.Concepts = cleanList(ex.Concepts, 10, 80)
	ex.StarterCode = clip(strings.TrimRight(ex.StarterCode, " \n\t"), 4000)
	var examples []Example
	for _, e := range ex.Examples {
		e.Input, e.Output = clip(strings.TrimSpace(e.Input), 1000), clip(strings.TrimSpace(e.Output), 1000)
		e.Explanation = clean(e.Explanation, 400)
		if e.Input != "" || e.Output != "" {
			examples = append(examples, e)
		}
		if len(examples) == 4 {
			break
		}
	}
	ex.Examples = examples

	switch {
	case ex.Title == "":
		return nil, invalid("exercise has no title")
	case ex.Description == "":
		return nil, invalid("exercise has no description")
	case ex.Objective == "":
		return nil, invalid("exercise has no learning objective")
	case len(ex.Requirements) == 0:
		return nil, invalid("exercise has no requirements")
	}
	return &ex, nil
}

// ---------------------------------------------------------------- Code review

// Finding is a single problem or suggestion in a code review.
type Finding struct {
	Kind        string `json:"kind"`
	Line        int    `json:"line"`
	Title       string `json:"title"`
	Explanation string `json:"explanation"`
	FixHint     string `json:"fix_hint"`
}

// Review is a validated, educational code review.
type Review struct {
	Summary          string    `json:"summary"`
	LooksCorrect     bool      `json:"looks_correct"`
	Errors           []Finding `json:"errors"`
	EdgeCases        []string  `json:"edge_cases"`
	Suggestions      []Finding `json:"suggestions"`
	CorrectedExample string    `json:"corrected_example"`
	Encouragement    string    `json:"encouragement"`
}

// ReviewRequest asks for a review of student code.
type ReviewRequest struct {
	Learner  Learner
	Code     string
	Exercise *Exercise // optional: the exercise being solved
}

// ReviewCode reviews student code as text. The code is never executed.
func (s *Service) ReviewCode(ctx context.Context, req ReviewRequest) (*Review, error) {
	msgs := []Message{
		{Role: "system", Content: reviewSystemPrompt(req.Learner)},
		{Role: "user", Content: reviewUserPrompt(req)},
	}
	return structured(ctx, s, "review", msgs, maxTokensReview, ParseReview)
}

var errorKinds = map[string]bool{"syntax": true, "logic": true, "runtime": true, "requirement": true, "compile": true}
var suggestionKinds = map[string]bool{"readability": true, "practice": true, "performance": true, "style": true, "naming": true, "safety": true}

// ParseReview decodes and validates a code review.
func ParseReview(raw string) (*Review, error) {
	var r Review
	if err := decodeJSON(raw, &r); err != nil {
		return nil, err
	}
	r.Summary = clean(r.Summary, 2000)
	if r.Summary == "" {
		return nil, invalid("review has no summary")
	}
	r.Errors = cleanFindings(r.Errors, errorKinds, "logic")
	r.Suggestions = cleanFindings(r.Suggestions, suggestionKinds, "practice")
	r.EdgeCases = cleanList(r.EdgeCases, 8, 400)
	r.CorrectedExample = clip(strings.TrimSpace(r.CorrectedExample), 6000)
	r.Encouragement = clean(r.Encouragement, 400)
	if len(r.Errors) > 0 {
		r.LooksCorrect = false // never call code correct while listing actual errors
	}
	return &r, nil
}

func cleanFindings(in []Finding, kinds map[string]bool, fallback string) []Finding {
	var out []Finding
	for _, f := range in {
		f.Kind = strings.ToLower(strings.TrimSpace(f.Kind))
		if !kinds[f.Kind] {
			f.Kind = fallback
		}
		f.Title = clean(f.Title, 160)
		f.Explanation = clean(f.Explanation, 1500)
		f.FixHint = clean(f.FixHint, 800)
		if f.Line < 0 || f.Line > 100000 {
			f.Line = 0
		}
		if f.Title == "" && f.Explanation == "" {
			continue
		}
		if f.Title == "" {
			f.Title = strings.ToUpper(f.Kind[:1]) + f.Kind[1:] + " issue"
		}
		out = append(out, f)
		if len(out) == 12 {
			break
		}
	}
	return out
}

// ---------------------------------------------------------------- Quizzes

// Question types.
const (
	MultipleChoice = "multiple_choice"
	TrueFalse      = "true_false"
	ShortAnswer    = "short_answer"
)

// Question is one validated quiz question.
type Question struct {
	Type              string            `json:"type"`
	Prompt            string            `json:"prompt"`
	Options           []string          `json:"options"`
	Answer            string            `json:"answer"`
	AcceptedAnswers   []string          `json:"accepted_answers"`
	Explanation       string            `json:"explanation"`
	WrongExplanations map[string]string `json:"wrong_explanations"`
}

// Quiz is a validated quiz.
type Quiz struct {
	Title     string     `json:"title"`
	Questions []Question `json:"questions"`
}

// QuizRequest describes the quiz to generate.
type QuizRequest struct {
	Learner    Learner
	Topic      string
	Difficulty string
	Count      int
}

// GenerateQuiz creates a quiz with a mix of question types.
func (s *Service) GenerateQuiz(ctx context.Context, req QuizRequest) (*Quiz, error) {
	if req.Count < 3 || req.Count > 15 {
		req.Count = 5
	}
	msgs := []Message{
		{Role: "system", Content: quizSystemPrompt(req.Learner)},
		{Role: "user", Content: quizUserPrompt(req)},
	}
	return structured(ctx, s, "quiz", msgs, maxTokensQuiz, ParseQuiz)
}

// ParseQuiz decodes and validates a quiz. Invalid questions are dropped;
// at least three valid questions are required. Multiple-choice options are
// shuffled so the correct answer's position is unpredictable.
func ParseQuiz(raw string) (*Quiz, error) {
	var q Quiz
	if err := decodeJSON(raw, &q); err != nil {
		return nil, err
	}
	q.Title = clean(q.Title, 120)
	var valid []Question
	for _, qu := range q.Questions {
		if v, ok := validateQuestion(qu); ok {
			valid = append(valid, v)
		}
		if len(valid) == 15 {
			break
		}
	}
	if len(valid) < 3 {
		return nil, invalid(fmt.Sprintf("quiz has only %d valid questions", len(valid)))
	}
	if q.Title == "" {
		q.Title = "Quiz"
	}
	q.Questions = valid
	return &q, nil
}

func validateQuestion(q Question) (Question, bool) {
	q.Type = strings.ToLower(strings.TrimSpace(q.Type))
	q.Prompt = clean(q.Prompt, 1500)
	q.Answer = strings.TrimSpace(q.Answer)
	q.Explanation = clean(q.Explanation, 1500)
	if q.Prompt == "" || q.Answer == "" || q.Explanation == "" {
		return q, false
	}
	wrong := map[string]string{}
	for k, v := range q.WrongExplanations {
		if k, v = strings.TrimSpace(k), clean(v, 800); k != "" && v != "" {
			wrong[k] = v
		}
	}
	q.WrongExplanations = wrong

	switch q.Type {
	case MultipleChoice:
		opts := cleanList(q.Options, 6, 300)
		seen := map[string]bool{}
		var unique []string
		for _, o := range opts {
			if key := NormalizeAnswer(o); !seen[key] {
				seen[key] = true
				unique = append(unique, o)
			}
		}
		if len(unique) < 2 {
			return q, false
		}
		answer := ""
		for _, o := range unique {
			if NormalizeAnswer(o) == NormalizeAnswer(q.Answer) {
				answer = o
			}
		}
		if answer == "" {
			return q, false
		}
		rand.Shuffle(len(unique), func(i, j int) { unique[i], unique[j] = unique[j], unique[i] })
		q.Options, q.Answer, q.AcceptedAnswers = unique, answer, nil
	case TrueFalse:
		switch NormalizeAnswer(q.Answer) {
		case "true":
			q.Answer = "True"
		case "false":
			q.Answer = "False"
		default:
			return q, false
		}
		q.Options, q.AcceptedAnswers = []string{"True", "False"}, nil
	case ShortAnswer:
		if len(q.Answer) > 120 {
			return q, false
		}
		q.Options = nil
		q.AcceptedAnswers = cleanList(q.AcceptedAnswers, 8, 120)
	default:
		return q, false
	}
	return q, true
}

// NormalizeAnswer lowercases, trims and collapses whitespace and trailing
// punctuation so equivalent answers compare equal.
func NormalizeAnswer(s string) string {
	s = strings.ToLower(strings.Join(strings.Fields(s), " "))
	s = strings.TrimRight(s, ".!?;, ")
	s = strings.Trim(s, "`'\"")
	return s
}

// ---------------------------------------------------------------- Recommendations

// RecommendRequest carries real progress data for the model to interpret.
type RecommendRequest struct {
	Learner Learner
	Topics  []string // every topic title in the learning path, in order
	Summary string   // factual description of recorded progress
}

// Recommendation suggests one topic to focus on.
type Recommendation struct {
	Topic  string `json:"topic"`
	Reason string `json:"reason"`
	Action string `json:"action"`
}

// Recommendations is the model's study advice.
type Recommendations struct {
	Summary string           `json:"summary"`
	Items   []Recommendation `json:"recommendations"`
}

// Recommend asks the model to suggest what to study next based on progress.
func (s *Service) Recommend(ctx context.Context, req RecommendRequest) (*Recommendations, error) {
	msgs := []Message{
		{Role: "system", Content: recommendSystemPrompt(req.Learner)},
		{Role: "user", Content: recommendUserPrompt(req)},
	}
	return structured(ctx, s, "recommend", msgs, maxTokensRecs, func(raw string) (*Recommendations, error) {
		return ParseRecommendations(raw, req.Topics)
	})
}

// ParseRecommendations validates recommendations, keeping only items that
// refer to real topics.
func ParseRecommendations(raw string, topics []string) (*Recommendations, error) {
	var r Recommendations
	if err := decodeJSON(raw, &r); err != nil {
		return nil, err
	}
	known := map[string]string{}
	for _, t := range topics {
		known[NormalizeAnswer(t)] = t
	}
	r.Summary = clean(r.Summary, 1200)
	var items []Recommendation
	for _, it := range r.Items {
		title, ok := known[NormalizeAnswer(it.Topic)]
		if !ok {
			continue
		}
		it.Topic = title
		it.Reason, it.Action = clean(it.Reason, 500), clean(it.Action, 500)
		if it.Reason == "" {
			continue
		}
		items = append(items, it)
		if len(items) == 4 {
			break
		}
	}
	if r.Summary == "" && len(items) == 0 {
		return nil, invalid("recommendations are empty")
	}
	r.Items = items
	return &r, nil
}

// ---------------------------------------------------------------- JSON helpers

func invalid(msg string) error { return fmt.Errorf("%w: %s", ErrInvalidResponse, msg) }

// decodeJSON extracts the JSON object from a model reply (tolerating code
// fences or surrounding prose) and decodes it into v.
func decodeJSON(raw string, v any) error {
	s := strings.TrimSpace(raw)
	start, end := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return invalid("no JSON object found")
	}
	if err := json.Unmarshal([]byte(s[start:end+1]), v); err != nil {
		return invalid("malformed JSON: " + err.Error())
	}
	return nil
}

// clean trims whitespace and limits length.
func clean(s string, max int) string { return clip(strings.TrimSpace(s), max) }

// cleanList trims entries, drops empty ones and limits count and length.
func cleanList(in []string, maxItems, maxLen int) []string {
	var out []string
	for _, s := range in {
		if s = clean(s, maxLen); s != "" {
			out = append(out, s)
		}
		if len(out) == maxItems {
			break
		}
	}
	return out
}
