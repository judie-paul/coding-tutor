# Code Tutor — your personal AI programming tutor

Code Tutor is a web application that helps you learn to program with the help of an AI tutor. It explains concepts in plain language, writes practice exercises at your level, gives you hints when you're stuck, reviews the code you write and quizzes you on what you've learned. It also keeps track of your progress so you always know what to study next.

The first version teaches **Go**. More languages can be added later.

- [Part 1 — Using Code Tutor](#part-1--using-code-tutor) (for students and anyone using the app)
- [Part 2 — Technical guide](#part-2--technical-guide) (for people installing, running or developing it)

---

# Part 1 — Using Code Tutor

## What you can do

| Feature | What it does for you |
|---|---|
| **AI Tutor** | Ask any programming question and get a clear explanation with examples. Ask follow-up questions; the tutor remembers the conversation. |
| **Topics** | A step-by-step learning path, from the basics to advanced subjects like goroutines and channels. |
| **Exercises** | Practice problems written for your chosen topic and difficulty. |
| **Hints** | Three hints per exercise, each a little more helpful than the last. They guide you without giving the answer away. |
| **Code review** | Paste your code and get friendly feedback: what it does, what's wrong and why, and how to improve it. |
| **Quizzes** | Short quizzes that are marked instantly, with an explanation for every answer. |
| **Progress** | See how far you've come, which topics need more work and what to study next. |

## Getting started

1. Open Code Tutor in your web browser (for example `http://localhost:8080` if you are running it on your own computer).
2. Click **Start learning for free** and create an account with your name, email and a password of at least 8 characters.
3. Choose the language you want to learn and your experience level:
   - **Beginner**: you're new to programming or to this language. Explanations are simple and go step by step.
   - **Intermediate**: you know the basics. Explanations focus on the "why" and common pitfalls.
   - **Advanced**: you're comfortable with the language. Explanations are concise and cover trade-offs.

   You can change both later in **Settings**.

You'll land on your **Dashboard**, your home page.

## The dashboard

The dashboard shows at a glance:

- how many lessons, exercises and quizzes you've completed,
- your overall progress through the learning path,
- your recent activity (click any item to go back to it),
- **Recommended next**: topics to study next, based on what you've actually done,
- quick links to every part of the app.

Use the menu on the left (or the ☰ button on a phone) to move around.

## Learning a topic

1. Open **Topics** to see the learning path. Each card shows the topic's difficulty and your progress on it.
2. Click a topic to see what you'll learn and its key points.
3. From the topic page you can:
   - **Learn with the tutor**: starts a conversation about the topic,
   - **Practice exercise**: creates an exercise for this topic,
   - **Take a quiz**: tests your understanding.
4. When you feel you understand the topic, click **Mark lesson as completed**. (Changed your mind? Click it again to undo.)

A good rhythm for each topic: **ask the tutor → practice an exercise → take a quiz**.

## Asking the AI tutor

1. Go to **AI Tutor**.
2. Optionally pick a topic, then type your question. You can also click one of the example questions.
3. Click **Ask the tutor**. The answer usually takes a few seconds.
4. Keep the conversation going by typing follow-up questions at the bottom. Press **Enter** to send, or **Shift + Enter** for a new line.

**Tips for great answers**

- Say what you want: *"Explain goroutines like I'm a beginner"*, *"Give me a real-world analogy for interfaces"*.
- Paste code and ask *"Explain this code line by line"*.
- Ask *"What are common mistakes with this?"* or *"Show me another example"*. The buttons under the chat box do this for you.
- Use the **Copy** button on any code example to copy it into your editor.

All your conversations are saved. Open **Conversations** to see them grouped by topic and pick up where you left off. You can delete a conversation from its page.

## Practicing with exercises

1. Go to **Exercises → New exercise** (or use **Practice exercise** on a topic page).
2. Choose a topic, a difficulty and an exercise type:
   - **Write a function**: implement a single function,
   - **Build a small program**: write a complete short program,
   - **Fix the bug**: find and fix mistakes in existing code,
   - **Improve existing code**: make working code cleaner or better.
3. Click **Generate exercise**. You'll get a problem description, a learning objective, requirements, examples, constraints and sometimes starter code.
4. Write your solution in your own code editor and run it there (for Go: `go run main.go`).

### When you're stuck: hints

On the exercise page, click **Get hint 1 of 3**.

- **Hint 1** is a small clue.
- **Hint 2** explains the approach in more detail.
- **Hint 3** is a strong nudge towards the solution.

For a hint that fits *your* attempt, open **Share your current attempt** and paste your code before asking.

If you're still stuck after the hints, you can open **Still stuck? Reveal the full solution**, tick the confirmation box and click **Show the full solution**. The solution is only ever shown when you explicitly ask for it, because working it out yourself is how you learn.

### Finishing an exercise

- **Submit for review** (see below). If the review finds no errors, the exercise is marked **Completed** automatically.
- Or click **Mark as understood** if you've solved it and don't need a review.

## Getting your code reviewed

There are two ways:

- **For an exercise**: paste your code in **Submit your solution** at the bottom of the exercise page.
- **For any code**: open **Code Review** in the menu, paste your code and click **Review my code**.

The review page shows:

| Section | Meaning |
|---|---|
| **Your code** | Exactly what you submitted, with line numbers. |
| **What your code does** | A plain-language summary. |
| **Errors** | Things that are actually wrong: code that won't compile, crashes, gives the wrong result or misses a requirement. Each one explains *why* and gives a hint on how to fix it. |
| **Edge cases** | Inputs or situations your code might not handle. |
| **Suggestions** | Optional improvements to readability and style. Your code can be correct without them. |
| **A corrected example** | Shown only when it really helps. Compare it with your code, then fix your own version. |

> **Important:** Code Tutor never runs your code. The AI *reads* it as text, the way a teacher reads your homework. Always test your code yourself.

All your reviews are listed on the **Code Review** page and on each exercise.

## Taking quizzes

1. Go to **Quizzes → New quiz** (or **Take a quiz** on a topic page).
2. Choose a topic, a difficulty and the number of questions (5, 8 or 10).
3. Answer the questions. There are multiple-choice, true/false and short-answer questions. For short answers, type a single word or short phrase (for example `append`).
4. Click **Submit answers**. Answers stay hidden until you submit.

The results page shows your score, the correct answer to every question, why your answer was wrong and an explanation. You can **Retake** a quiz as often as you like; your best and latest scores are both kept.

## Tracking your progress

Open **Progress** to see:

- your **overall mastery** and how many topics you've mastered,
- a **topic-by-topic** table,
- **areas to work on**: topics where quiz scores are low, exercises stay unfinished or you've needed lots of hints,
- topics **suggested to revisit**,
- your **recent quiz results** and your complete **learning history**.

**How mastery is calculated.** Each topic is worth 100%:

- 30% for completing the lesson,
- 35% for completing an exercise (15% for attempting one),
- up to 35% from your best quiz score.

Your progress is calculated only from what you've actually done. Click **Get study advice** for personal suggestions from the AI study coach. The advice is based only on your recorded progress and never changes your statistics.

## Account settings

In **Settings** you can:

- change your name, language and experience level,
- change your password (this signs you out on your other devices),
- delete your account. This permanently removes your account and all your learning history.

## Frequently asked questions

**Does Code Tutor run my code?**
No. Your code is sent to the AI as text to be read and reviewed. It is never compiled or executed by the site.

**Can other people see my conversations, exercises or scores?**
No. Everything you create is private to your account.

**Can the AI be wrong?**
Yes, occasionally. Treat its answers like advice from a knowledgeable friend: test the code yourself, and ask a follow-up question if something doesn't make sense.

**It says "The AI tutor is unavailable" or "took too long".**
The AI service may be busy or temporarily unreachable. Wait a moment and try again. Your question is kept in the box so you don't have to retype it.

**It says I'm sending requests too quickly.**
To keep the service fair and affordable, there's a limit on AI requests per minute. Wait a minute and continue.

**I logged in but got sent back to the login page.**
Your session may have expired. Log in again; nothing you saved is lost.

---

# Part 2 — Technical guide

## Technology

| Area | Choice |
|---|---|
| Backend | Go (standard library: `net/http`, `html/template`, `database/sql`, `log/slog`) |
| Database | SQLite via [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) |
| Passwords | bcrypt via [`golang.org/x/crypto`](https://pkg.go.dev/golang.org/x/crypto/bcrypt) |
| Frontend | Server-rendered Go templates, plain CSS, a little vanilla JavaScript (no frameworks, no Node.js) |
| AI | Any **OpenAI-compatible** chat-completions API, called with `net/http` |

There are only two third-party dependencies:

- **`modernc.org/sqlite`** is a pure-Go SQLite driver. It needs no C compiler (cgo/gcc), so the app builds the same way on Windows, macOS and Linux.
- **`golang.org/x/crypto/bcrypt`** provides the standard, well-reviewed password hashing that Go's standard library does not include.

## Prerequisites

- **Go 1.26 or newer**: <https://go.dev/dl/>
- **An AI provider** with an OpenAI-compatible API. This can be a hosted service (OpenAI, OpenRouter, Groq, …) or a local model server such as [Ollama](https://ollama.com).
- On Windows, **PowerShell** to use `build.ps1` (PowerShell 7 recommended).

No C compiler, Node.js or separate database server is needed.

## Quick start

**Windows (PowerShell)**

```powershell
Copy-Item .env.example .env      # then edit .env and set your AI settings
.\build.ps1 -Run                 # checks, tests, builds and starts the server
```

**macOS / Linux**

```bash
cp .env.example .env             # then edit .env and set your AI settings
go run ./cmd/web
```

Open <http://localhost:8080>, create an account and start learning.

The database file (`data/tutor.db` by default) and its tables are created automatically on first start.

## Configuring the AI provider

The app talks to any service that implements the OpenAI **`/chat/completions`** API. Set these values in `.env` or as environment variables:

| Variable | Description |
|---|---|
| `AI_BASE_URL` | Base URL of the API, **without** `/chat/completions` |
| `AI_API_KEY` | Your API key (leave empty for local servers that don't need one) |
| `AI_MODEL` | Model name to use |

Examples:

| Provider | `AI_BASE_URL` | Example `AI_MODEL` |
|---|---|---|
| OpenAI | `https://api.openai.com/v1` | `gpt-4o-mini` |
| OpenRouter | `https://openrouter.ai/api/v1` | `meta-llama/llama-3.1-70b-instruct` |
| Groq | `https://api.groq.com/openai/v1` | `llama-3.3-70b-versatile` |
| Ollama (local) | `http://localhost:11434/v1` | `llama3.1` |

Notes:

- The API key is used only by the server. It is never sent to the browser or written to logs.
- For structured tasks (exercises, reviews, quizzes, study advice) the app asks for JSON output with `response_format: {"type": "json_object"}`. If your provider rejects that parameter, set `AI_JSON_MODE=false`. The prompts still ask for JSON, and every response is validated either way.
- Use a reasonably capable model. Very small models often produce invalid quizzes or reviews. The app retries once and then shows a friendly error.
- If the AI is not configured, the rest of the app still works, and AI features show a clear message.

## Configuration reference

All settings come from environment variables. For local development you can put them in a `.env` file in the project root. Real environment variables always win over `.env`.

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | `development` or `production`. Production uses JSON logs and requires secure cookies. |
| `APP_ADDR` | `:8080` | Address the server listens on. |
| `DB_PATH` | `data/tutor.db` | SQLite database file. The directory is created if needed. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `SESSION_TTL` | `168h` | How long a login lasts (Go duration, e.g. `12h`). |
| `COOKIE_SECURE` | `true` in production | Send cookies only over HTTPS. Must be `true` in production. |
| `TRUST_PROXY` | `false` | Use `X-Forwarded-For` for client IPs. Enable **only** behind a trusted reverse proxy. |
| `AI_BASE_URL` | `https://api.openai.com/v1` | See above. |
| `AI_API_KEY` | *(empty)* | See above. |
| `AI_MODEL` | `gpt-4o-mini` | See above. |
| `AI_TIMEOUT` | `60s` | Timeout for a single AI request. |
| `AI_JSON_MODE` | `true` | Send `response_format=json_object` for structured requests. |
| `AI_RATE_LIMIT` | `10` | AI requests allowed per student per minute. |

## The database

### Initializing the database

There is nothing to install. On startup the server opens (or creates) the SQLite file at `DB_PATH` and applies any pending migrations. Foreign keys, WAL journaling and a busy timeout are enabled on every connection.

### Running migrations

Migrations run automatically every time the server starts. To apply them without starting the server (for example as a deployment step):

```powershell
.\build.ps1 -Migrate            # Windows: build, then migrate
```

```bash
go run ./cmd/web -migrate       # any platform
./tutor -migrate                # with a built binary
```

How migrations work:

- They live in `migrations/` as numbered SQL files (`001_initial_schema.sql`, `002_seed_go_topics.sql`, …) and are embedded into the binary.
- Applied migrations are recorded in the `schema_migrations` table, so each runs exactly once, inside its own transaction.
- **To change the schema**, add a new file with the next number, such as `003_add_something.sql`. Never edit a migration that has already been applied.
- **To add a new programming language**, add a migration that inserts a row into `programming_languages` and its `topics` (see `002_seed_go_topics.sql`). The prompts already use the student's chosen language.

### Backups

SQLite stores everything in one file. To back it up safely while the server is running, use the SQLite CLI:

```bash
sqlite3 data/tutor.db ".backup 'backup-2026-01-01.db'"
```

Or stop the server and copy `tutor.db` together with any `tutor.db-wal` and `tutor.db-shm` files.

## Building and running

`build.ps1` is the main entry point on Windows:

| Command | What it does |
|---|---|
| `.\build.ps1` | Check formatting, run `go vet`, run all tests, build `bin\tutor.exe` |
| `.\build.ps1 -Run` | All of the above, then start the server |
| `.\build.ps1 -Migrate` | Build, then apply database migrations and exit |
| `.\build.ps1 -SkipTests` | Build without running tests |
| `.\build.ps1 -Release` | Smaller, production-ready binary (`-trimpath -ldflags "-s -w"`) |
| `.\build.ps1 -Release -OS linux -Arch amd64` | Cross-compile for a Linux server (produces `bin/tutor`) |
| `.\build.ps1 -Clean` | Remove the `bin` folder |

Without PowerShell:

```bash
go run ./cmd/web                       # run directly
go build -o bin/tutor ./cmd/web        # build a binary
./bin/tutor                            # run it
```

The binary is self-contained: templates, CSS, JavaScript and migrations are all embedded. Only the database file lives outside it.

## Running the tests

```powershell
.\build.ps1                 # Windows: includes formatting, vet and tests
```

```bash
go test ./...               # all tests
go test -race ./...         # with the race detector (needs cgo)
go test ./internal/ai -v    # one package, verbose
```

The tests never call a real AI provider. They use:

- a mocked HTTP server (`httptest`) to test the AI client: requests, timeouts and error handling,
- a fake AI completer to drive full HTTP flows through the real router, middleware, templates and a real temporary SQLite database.

What's covered: authentication, sessions, CSRF, rate limiting, authorization (users can't reach each other's data), database migrations and constraints, every feature's HTTP handlers, exercise generation, progressive hints, code review, quiz grading, progress calculations, AI response parsing and validation, and HTML escaping of AI output.

## Running in production

Checklist:

1. **Build a release binary** for your server, e.g. `.\build.ps1 -Release -OS linux -Arch amd64`.
2. **Set `APP_ENV=production`.** This turns on JSON logs and requires `COOKIE_SECURE=true`.
3. **Serve over HTTPS.** Put the app behind a reverse proxy such as Caddy or nginx that terminates TLS. Secure cookies and HSTS headers are only sent correctly over HTTPS.
4. **Set `TRUST_PROXY=true`** only when the app is reachable *exclusively* through that proxy. This makes per-IP login rate limiting use the real client address.
5. **Store secrets in the environment** (for example a systemd `EnvironmentFile` with restricted permissions), not in the repository.
6. **Put the database on persistent storage** (`DB_PATH`) and schedule regular backups.
7. **Run migrations** as part of each deployment (`./tutor -migrate`). They also run automatically on start.
8. **Health checks**: `GET /healthz` returns `200 ok` when the server and database are reachable.

Example systemd unit:

```ini
[Unit]
Description=Code Tutor
After=network.target

[Service]
User=tutor
WorkingDirectory=/opt/tutor
EnvironmentFile=/opt/tutor/tutor.env
ExecStart=/opt/tutor/tutor
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Example `tutor.env`:

```ini
APP_ENV=production
APP_ADDR=127.0.0.1:8080
DB_PATH=/var/lib/tutor/tutor.db
TRUST_PROXY=true
AI_BASE_URL=https://api.openai.com/v1
AI_API_KEY=sk-...
AI_MODEL=gpt-4o-mini
```

Example Caddyfile (automatic HTTPS):

```
tutor.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

The server shuts down gracefully on `SIGINT`/`SIGTERM`, letting in-flight requests finish. Expired sessions are cleaned up hourly.

## Project structure

```
cmd/web/            entry point: config, logging, database, HTTP server
internal/
  ai/               all AI code: OpenAI-compatible client, prompts, JSON validation
  app/              wires stores, services and handlers into the router (+ HTTP tests)
  auth/             sessions, login, registration, logout
  config/           environment-based configuration
  dashboard/        dashboard page
  database/         SQLite connection, migrations, transaction helper
  exercises/        exercises, progressive hints, code submissions and reviews
  middleware/       logging, recovery, security headers, CSRF, rate limiting, auth guard
  progress/         activity log, progress counters, mastery calculations, progress page
  quizzes/          quiz generation, grading, results
  settings/         account settings page
  testutil/         shared test helpers
  topics/           languages, topics and lesson completion
  tutor/            AI tutor conversations
  users/            accounts, password hashing, validation
  view/             template rendering, safe Markdown, flash messages
migrations/         numbered SQL migrations (embedded)
web/
  templates/        Go HTML templates (layouts, partials, one folder per feature)
  static/           CSS, JavaScript, images (embedded)
build.ps1           build/test/run script
.env.example        configuration template
```

Submissions and reviews live in `internal/exercises` rather than a separate package, because a submission belongs to an exercise and shares its ownership checks.

## Architecture

**Request flow:**

```
browser → middleware (recover → security headers → body limit → load session
→ log → CSRF) → router → auth guard (+ AI rate limit) → handler → service → store → SQLite
                                                            └→ ai.Tutor → AI provider
```

- **Handlers** read forms, check the logged-in user and render templates. They contain no SQL or AI calls.
- **Services** (`tutor`, `exercises`, `quizzes`, …) hold the business rules and use transactions where several writes must succeed together, such as storing a quiz attempt, its answers and the progress update.
- **The AI layer** (`internal/ai`) is the only code that talks to the model:
  - `ai.Completer` is the network interface, so tests swap in a fake.
  - `ai.Tutor` exposes one method per task: explain, follow-up, exercise, hint, solution, code review, quiz, study advice.
  - Every structured response is parsed, validated and size-limited before use. Invalid responses are retried once.
- **Ownership** is enforced in SQL: every query for private data includes `user_id = ?`. Another user's records return 404, exactly like records that don't exist.

**Data model (simplified):**

```
users ─┬─ sessions
       ├─ conversations ── messages
       ├─ exercises ─┬─ exercise_hints
       │             └─ exercise_submissions (exercise_id is NULL for free-form reviews)
       ├─ quizzes ── quiz_questions
       │      └── quiz_attempts ── quiz_answers
       ├─ learning_progress (one row per user × topic)
       └─ activity_logs
programming_languages ── topics
```

All timestamps are stored as UTC RFC 3339 text. Deleting a user cascades to all of their data.

## Security

- Passwords hashed with bcrypt. Login takes the same time whether or not an email exists.
- Server-side sessions. Only a SHA-256 hash of each session token is stored. Cookies are `HttpOnly`, `SameSite=Lax` and `Secure` in production. Sessions expire, are replaced on login (preventing session fixation) and are revoked on logout and on password change for other devices.
- CSRF protection on every state-changing request: a double-submit token plus Go's `http.CrossOriginProtection` (Origin / `Sec-Fetch-Site` checks).
- A strict Content-Security-Policy (no inline scripts or styles), `X-Frame-Options: DENY`, `nosniff`, a referrer policy, and HSTS when HTTPS is used.
- Parameterized SQL everywhere. The only dynamic SQL fragment (a progress counter name) is checked against an allow-list.
- Template auto-escaping. AI output is rendered by a small Markdown renderer that escapes all text first, so a model can't inject HTML or scripts.
- Rate limits on login and registration (per IP) and on AI requests (per user). Request bodies are capped at 1 MB.
- Open-redirect protection on the post-login `next` parameter.
- Student code is **never executed**. It is sent to the AI as text, wrapped in delimiters, with instructions to treat it as data.
- Internal errors, provider messages and API keys never reach users. They are logged server-side with `slog`.

## Troubleshooting

| Problem | Solution |
|---|---|
| `go: command not found` / build fails on version | Install Go 1.26+ and reopen your terminal. |
| "The AI tutor is not configured yet" | Set `AI_BASE_URL` and `AI_MODEL` (and `AI_API_KEY` if your provider needs one). |
| "The AI tutor is unavailable right now" | Check the server logs (`level=ERROR msg="ai provider error"`). Common causes are a wrong API key (status 401), a wrong model name (404) or a wrong base URL. |
| "returned an answer we couldn't understand" | The model produced invalid JSON twice. Try a more capable model, or set `AI_JSON_MODE=true` if your provider supports it. |
| Provider error about `response_format` | Set `AI_JSON_MODE=false`. |
| Can't stay logged in when testing over plain HTTP on another machine's address | Secure cookies are only sent over HTTPS (browsers make an exception for `localhost`). For HTTP testing use `APP_ENV=development` and `COOKIE_SECURE=false`; in production, use HTTPS. |
| "Your form expired or came from another site" | Refresh the page and resubmit. This is the CSRF protection working, often after a server restart or a long-open tab. |
| `database is locked` | Make sure only one server instance uses the database file. |

## Limitations and future enhancements

- Code is reviewed by reading only. A future version could compile and run submissions in an isolated sandbox.
- Only Go is seeded. Other languages can be added with a migration (see [Running migrations](#running-migrations)).
- Rate limits are kept in memory per server instance. A multi-instance deployment would need a shared store.
- AI answers are returned in full rather than streamed token by token.
