-- Initial schema for the AI Programming Tutor.
-- All timestamps are stored as UTC RFC 3339 text, e.g. 2026-01-02T15:04:05Z.

CREATE TABLE programming_languages (
    id          INTEGER PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE users (
    id               INTEGER PRIMARY KEY,
    name             TEXT NOT NULL,
    email            TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash    TEXT NOT NULL,
    language_id      INTEGER REFERENCES programming_languages(id) ON DELETE SET NULL,
    experience_level TEXT NOT NULL DEFAULT 'beginner'
        CHECK (experience_level IN ('beginner', 'intermediate', 'advanced')),
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE TABLE sessions (
    token_hash   TEXT PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at   TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    last_seen_at TEXT NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

CREATE TABLE topics (
    id          INTEGER PRIMARY KEY,
    language_id INTEGER NOT NULL REFERENCES programming_languages(id) ON DELETE CASCADE,
    slug        TEXT NOT NULL,
    title       TEXT NOT NULL,
    summary     TEXT NOT NULL,
    key_points  TEXT NOT NULL DEFAULT '', -- newline-separated list
    difficulty  TEXT NOT NULL CHECK (difficulty IN ('beginner', 'intermediate', 'advanced')),
    position    INTEGER NOT NULL,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    UNIQUE (language_id, slug)
);
CREATE INDEX idx_topics_language_position ON topics(language_id, position);

CREATE TABLE conversations (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic_id   INTEGER REFERENCES topics(id) ON DELETE SET NULL,
    title      TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_conversations_user_updated ON conversations(user_id, updated_at DESC);

CREATE TABLE messages (
    id              INTEGER PRIMARY KEY,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content         TEXT NOT NULL,
    created_at      TEXT NOT NULL
);
CREATE INDEX idx_messages_conversation ON messages(conversation_id, id);

CREATE TABLE exercises (
    id                INTEGER PRIMARY KEY,
    user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic_id          INTEGER NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    difficulty        TEXT NOT NULL CHECK (difficulty IN ('beginner', 'intermediate', 'advanced')),
    exercise_type     TEXT NOT NULL,
    title             TEXT NOT NULL,
    content_json      TEXT NOT NULL, -- validated ai.Exercise document
    status            TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'completed')),
    solution_revealed INTEGER NOT NULL DEFAULT 0,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);
CREATE INDEX idx_exercises_user_created ON exercises(user_id, created_at DESC);
CREATE INDEX idx_exercises_topic ON exercises(topic_id);

-- exercise_id is NULL for free-form code reviews not tied to an exercise.
CREATE TABLE exercise_submissions (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exercise_id INTEGER REFERENCES exercises(id) ON DELETE CASCADE,
    language_id INTEGER NOT NULL REFERENCES programming_languages(id),
    code        TEXT NOT NULL, -- the student's original submission, never modified
    review_json TEXT NOT NULL, -- validated ai.Review document
    looks_correct INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL
);
CREATE INDEX idx_submissions_user_created ON exercise_submissions(user_id, created_at DESC);
CREATE INDEX idx_submissions_exercise ON exercise_submissions(exercise_id);

CREATE TABLE exercise_hints (
    id          INTEGER PRIMARY KEY,
    exercise_id INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('hint', 'solution')),
    level       INTEGER NOT NULL CHECK (level BETWEEN 1 AND 4),
    content     TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    UNIQUE (exercise_id, level)
);

CREATE TABLE quizzes (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic_id   INTEGER NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    difficulty TEXT NOT NULL CHECK (difficulty IN ('beginner', 'intermediate', 'advanced')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_quizzes_user_created ON quizzes(user_id, created_at DESC);

CREATE TABLE quiz_questions (
    id                     INTEGER PRIMARY KEY,
    quiz_id                INTEGER NOT NULL REFERENCES quizzes(id) ON DELETE CASCADE,
    position               INTEGER NOT NULL,
    question_type          TEXT NOT NULL CHECK (question_type IN ('multiple_choice', 'true_false', 'short_answer')),
    prompt                 TEXT NOT NULL,
    options_json           TEXT NOT NULL DEFAULT '[]',
    correct_answer         TEXT NOT NULL,
    accepted_answers_json  TEXT NOT NULL DEFAULT '[]',
    explanation            TEXT NOT NULL,
    wrong_explanations_json TEXT NOT NULL DEFAULT '{}',
    UNIQUE (quiz_id, position)
);

CREATE TABLE quiz_attempts (
    id           INTEGER PRIMARY KEY,
    quiz_id      INTEGER NOT NULL REFERENCES quizzes(id) ON DELETE CASCADE,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    score        INTEGER NOT NULL, -- number of correct answers
    total        INTEGER NOT NULL,
    percent      INTEGER NOT NULL,
    completed_at TEXT NOT NULL
);
CREATE INDEX idx_attempts_user_completed ON quiz_attempts(user_id, completed_at DESC);
CREATE INDEX idx_attempts_quiz ON quiz_attempts(quiz_id);

CREATE TABLE quiz_answers (
    id          INTEGER PRIMARY KEY,
    attempt_id  INTEGER NOT NULL REFERENCES quiz_attempts(id) ON DELETE CASCADE,
    question_id INTEGER NOT NULL REFERENCES quiz_questions(id) ON DELETE CASCADE,
    answer      TEXT NOT NULL,
    is_correct  INTEGER NOT NULL,
    UNIQUE (attempt_id, question_id)
);

CREATE TABLE learning_progress (
    id                  INTEGER PRIMARY KEY,
    user_id             INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic_id            INTEGER NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    lesson_completed    INTEGER NOT NULL DEFAULT 0,
    questions_asked     INTEGER NOT NULL DEFAULT 0,
    exercises_attempted INTEGER NOT NULL DEFAULT 0,
    exercises_completed INTEGER NOT NULL DEFAULT 0,
    hints_requested     INTEGER NOT NULL DEFAULT 0,
    quizzes_taken       INTEGER NOT NULL DEFAULT 0,
    best_quiz_percent   INTEGER,
    last_quiz_percent   INTEGER,
    last_activity_at    TEXT NOT NULL,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    UNIQUE (user_id, topic_id)
);

CREATE TABLE activity_logs (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic_id    INTEGER REFERENCES topics(id) ON DELETE SET NULL,
    kind        TEXT NOT NULL,
    description TEXT NOT NULL,
    link        TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
CREATE INDEX idx_activity_user_created ON activity_logs(user_id, created_at DESC, id DESC);
