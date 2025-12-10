# DevQuiz Service - Implementation Plan

## Overview

DevQuiz is a quiz service supporting multiple quiz types with persistent URLs, admin result viewing, and score tracking. Inspired by Google Forms but focused on quiz/assessment functionality.

---

## Tech Stack

Following existing project patterns:

| Component | Technology |
|-----------|------------|
| Backend | Python 3.8+ with FastAPI |
| Server | Uvicorn (ASGI) |
| Frontend | Jinja2 templates + vanilla JS (simple, no React overhead) |
| Storage | File-based JSON (following pricecrawler pattern) |
| Authentication | Simple token-based auth for admins |

---

## Directory Structure

```
devquiz/
├── app.py                 # FastAPI application entry point
├── config.py              # Configuration management
├── models.py              # Pydantic models for data validation
├── storage.py             # File-based JSON storage layer
├── auth.py                # Simple authentication
├── quiz_types/
│   ├── __init__.py
│   ├── static.py          # Static quiz logic
│   ├── generated.py       # Generated quiz from question bank
│   └── infinite.py        # Infinite quiz logic
├── routes/
│   ├── __init__.py
│   ├── quiz.py            # Quiz taking endpoints
│   ├── admin.py           # Admin endpoints
│   └── auth.py            # Auth endpoints
├── templates/
│   ├── base.html          # Base template
│   ├── quiz_take.html     # Quiz taking page
│   ├── quiz_results.html  # Results page for quiz taker
│   ├── admin_login.html   # Admin login
│   ├── admin_dashboard.html   # Admin quiz list
│   ├── admin_results.html     # Admin view of quiz results
│   └── quiz_create.html   # Quiz creation form
├── static/
│   ├── style.css
│   └── quiz.js            # Frontend quiz logic
├── data/                  # JSON storage (gitignored)
│   ├── question_banks/    # Question bank files
│   ├── quizzes/           # Generated quiz instances
│   ├── submissions/       # Quiz submissions/results
│   └── admins/            # Admin user data
├── requirements.txt
└── README.md
```

---

## Data Models

### Question

```python
class Question(BaseModel):
    id: str                          # Unique question ID
    text: str                        # Question text
    question_type: Literal["multiple_choice", "true_false"]
    choices: list[Choice]            # Answer choices
    correct_answer_id: str           # ID of correct choice
    explanation: str | None = None   # Optional explanation
    tags: list[str] = []             # For filtering/categorization
    difficulty: Literal["easy", "medium", "hard"] | None = None
```

### Choice

```python
class Choice(BaseModel):
    id: str                # Unique choice ID
    text: str              # Choice text
```

### QuestionBank

```python
class QuestionBank(BaseModel):
    id: str
    name: str
    description: str | None = None
    questions: list[Question]
    created_at: datetime
    created_by: str        # Admin ID
```

### Quiz

```python
class Quiz(BaseModel):
    id: str                          # Unique quiz ID (used in URL)
    quiz_type: Literal["static", "generated", "infinite"]
    title: str
    description: str | None = None

    # For static/generated quizzes
    questions: list[Question] | None = None

    # For generated quizzes
    source_bank_id: str | None = None
    question_count: int | None = None

    # For infinite quizzes
    infinite_config: InfiniteConfig | None = None

    # Metadata
    created_at: datetime
    created_by: str                  # Admin ID
    is_active: bool = True
    time_limit_minutes: int | None = None
    shuffle_questions: bool = False
    shuffle_choices: bool = False
    show_correct_answers: bool = True  # Show after submission
```

### InfiniteConfig

```python
class InfiniteConfig(BaseModel):
    source_bank_ids: list[str]       # Question banks to pull from
    questions_per_fetch: int = 5     # Questions per batch
    allow_repeats: bool = False      # Can same question appear again
```

### QuizSubmission

```python
class QuizSubmission(BaseModel):
    id: str                          # Unique submission ID
    quiz_id: str
    taker_name: str | None = None    # Optional name
    taker_email: str | None = None   # Optional email

    answers: list[Answer]

    # Results
    score: int                       # Number correct
    total_questions: int
    percentage: float

    started_at: datetime
    submitted_at: datetime
    time_taken_seconds: int
```

### Answer

```python
class Answer(BaseModel):
    question_id: str
    selected_choice_id: str
    is_correct: bool
```

### Admin

```python
class Admin(BaseModel):
    id: str
    username: str
    password_hash: str               # bcrypt hashed
    created_at: datetime
    last_login: datetime | None = None
```

### Session

```python
class AdminSession(BaseModel):
    token: str                       # Random hex token
    admin_id: str
    created_at: datetime
    expires_at: datetime
```

---

## API Endpoints

### Public Quiz Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/quiz/{quiz_id}` | Render quiz taking page |
| `POST` | `/quiz/{quiz_id}/start` | Start a quiz session |
| `GET` | `/quiz/{quiz_id}/questions` | Get quiz questions (hides correct answers) |
| `POST` | `/quiz/{quiz_id}/submit` | Submit quiz answers |
| `GET` | `/quiz/{quiz_id}/result/{submission_id}` | View submission results |

### Infinite Quiz Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/quiz/{quiz_id}/infinite/next` | Fetch next batch of questions |
| `POST` | `/quiz/{quiz_id}/infinite/answer` | Submit single answer, get next question |

### Admin Authentication

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/admin/login` | Login page |
| `POST` | `/admin/login` | Authenticate admin |
| `POST` | `/admin/logout` | End admin session |

### Admin Quiz Management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/admin` | Admin dashboard |
| `GET` | `/admin/quiz/new` | Create quiz form |
| `POST` | `/admin/quiz` | Create new quiz |
| `GET` | `/admin/quiz/{quiz_id}` | View/edit quiz |
| `PUT` | `/admin/quiz/{quiz_id}` | Update quiz |
| `DELETE` | `/admin/quiz/{quiz_id}` | Delete quiz |
| `POST` | `/admin/quiz/{quiz_id}/toggle` | Activate/deactivate quiz |

### Admin Question Bank Management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/admin/banks` | List question banks |
| `GET` | `/admin/bank/new` | Create bank form |
| `POST` | `/admin/bank` | Create question bank |
| `GET` | `/admin/bank/{bank_id}` | View/edit bank |
| `PUT` | `/admin/bank/{bank_id}` | Update bank |
| `DELETE` | `/admin/bank/{bank_id}` | Delete bank |
| `POST` | `/admin/bank/{bank_id}/import` | Import questions (JSON/CSV) |

### Admin Results Viewing

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/admin/quiz/{quiz_id}/results` | View all submissions for a quiz |
| `GET` | `/admin/submission/{submission_id}` | View single submission detail |
| `GET` | `/admin/quiz/{quiz_id}/export` | Export results as CSV |

---

## URL Scheme

Quiz URLs will follow this pattern:
```
/quiz/{quiz_id}
```

Where `quiz_id` is an 8-character alphanumeric string (e.g., `abc12xyz`).

Example URLs:
- Take quiz: `https://example.com/quiz/abc12xyz`
- View result: `https://example.com/quiz/abc12xyz/result/def45uvw`

---

## Storage Structure

Following the file-based JSON pattern from pricecrawler:

```
data/
├── question_banks/
│   ├── {bank_id}.json           # QuestionBank data
│   └── ...
├── quizzes/
│   ├── {quiz_id}.json           # Quiz data
│   └── ...
├── submissions/
│   ├── {quiz_id}/
│   │   ├── {submission_id}.json # Individual submissions
│   │   └── ...
│   └── ...
├── admins/
│   ├── {admin_id}.json          # Admin user data
│   └── ...
└── sessions/
    ├── {token}.json             # Active sessions
    └── ...
```

---

## Authentication Design

### Admin Authentication

Simple session-based authentication:

1. Admin logs in with username/password
2. Server verifies password hash (bcrypt)
3. Server creates session token (32-char hex via `secrets.token_hex(16)`)
4. Token stored as HTTP-only cookie
5. Token expires after 24 hours
6. Each request validates token against sessions storage

### Quiz Taker Identity

Quiz takers don't need accounts. Options:

1. **Anonymous**: No identification required
2. **Named**: Optional name/email before starting
3. **Required**: Must provide name/email (configurable per quiz)

---

## Quiz Type Implementation Details

### 1. Static Quiz

The simplest type. Questions are defined directly in the quiz:

```python
def create_static_quiz(title: str, questions: list[Question], admin_id: str) -> Quiz:
    return Quiz(
        id=generate_quiz_id(),
        quiz_type="static",
        title=title,
        questions=questions,
        created_at=datetime.utcnow(),
        created_by=admin_id
    )
```

### 2. Generated Quiz

Creates a quiz by randomly selecting from a question bank:

```python
def generate_quiz_from_bank(
    bank_id: str,
    question_count: int,
    title: str,
    admin_id: str,
    difficulty: str | None = None,
    tags: list[str] | None = None
) -> Quiz:
    bank = load_question_bank(bank_id)

    # Filter questions
    pool = bank.questions
    if difficulty:
        pool = [q for q in pool if q.difficulty == difficulty]
    if tags:
        pool = [q for q in pool if any(t in q.tags for t in tags)]

    # Random selection
    selected = random.sample(pool, min(question_count, len(pool)))

    return Quiz(
        id=generate_quiz_id(),
        quiz_type="generated",
        title=title,
        questions=selected,
        source_bank_id=bank_id,
        question_count=question_count,
        created_at=datetime.utcnow(),
        created_by=admin_id
    )
```

### 3. Infinite Quiz

Questions are fetched dynamically. Server tracks which questions have been shown:

```python
class InfiniteQuizSession:
    quiz_id: str
    session_id: str
    questions_seen: set[str]  # Question IDs already shown
    current_score: int
    total_answered: int
    started_at: datetime

def get_next_questions(session: InfiniteQuizSession, count: int = 5) -> list[Question]:
    quiz = load_quiz(session.quiz_id)
    config = quiz.infinite_config

    # Gather questions from source banks
    all_questions = []
    for bank_id in config.source_bank_ids:
        bank = load_question_bank(bank_id)
        all_questions.extend(bank.questions)

    # Filter out already seen (unless repeats allowed)
    if not config.allow_repeats:
        available = [q for q in all_questions if q.id not in session.questions_seen]
    else:
        available = all_questions

    # Select random subset
    selected = random.sample(available, min(count, len(available)))

    # Track seen questions
    session.questions_seen.update(q.id for q in selected)

    return selected
```

---

## Frontend Design

### Quiz Taking Flow

1. **Landing Page** (`/quiz/{id}`)
   - Quiz title and description
   - Optional name/email form
   - "Start Quiz" button

2. **Quiz Page**
   - Progress indicator (Question X of Y)
   - Question text
   - Multiple choice options (radio buttons)
   - Previous/Next navigation
   - Timer (if time limit set)
   - Submit button

3. **Results Page**
   - Overall score and percentage
   - Pass/fail indicator (if threshold set)
   - Question-by-question breakdown:
     - Question text
     - User's answer (highlighted red if wrong)
     - Correct answer (highlighted green)
     - Explanation (if provided)

### Infinite Quiz Flow

1. **Start Page**
   - Quiz title
   - "Begin" button

2. **Question Page**
   - Current question
   - Running score display
   - Submit answer → immediately show right/wrong
   - "Next Question" button
   - "End Quiz" button

3. **Final Results**
   - Total questions answered
   - Final score and percentage
   - Option to continue or finish

### Admin Dashboard

1. **Login Page**
   - Username/password form

2. **Dashboard**
   - List of quizzes with quick stats
   - Create new quiz button
   - Manage question banks link

3. **Quiz Results Page**
   - Table of submissions
   - Columns: Name, Score, Date, Time Taken
   - Click to view detailed submission
   - Export to CSV button

---

## Implementation Phases

### Phase 1: Core Infrastructure
- [ ] Project setup (directory structure, requirements.txt)
- [ ] Configuration management
- [ ] Storage layer (JSON file CRUD operations)
- [ ] Pydantic models
- [ ] FastAPI app skeleton

### Phase 2: Admin Authentication
- [ ] Admin model and storage
- [ ] Password hashing utilities
- [ ] Session management
- [ ] Login/logout endpoints
- [ ] Auth middleware/dependencies

### Phase 3: Question Bank Management
- [ ] Question bank CRUD endpoints
- [ ] Admin templates for bank management
- [ ] Question import (JSON format)
- [ ] Validation logic

### Phase 4: Static Quiz
- [ ] Quiz CRUD endpoints
- [ ] Quiz creation form
- [ ] Quiz taking page
- [ ] Answer submission
- [ ] Results calculation
- [ ] Results display page

### Phase 5: Generated Quiz
- [ ] Quiz generation logic
- [ ] Generation parameters (count, difficulty, tags)
- [ ] Generation form UI
- [ ] Integration with existing quiz flow

### Phase 6: Infinite Quiz
- [ ] Infinite session management
- [ ] Next question endpoint
- [ ] Real-time answer validation
- [ ] Running score tracking
- [ ] Infinite quiz UI (AJAX-based)

### Phase 7: Admin Results & Analytics
- [ ] Results listing page
- [ ] Detailed submission view
- [ ] CSV export
- [ ] Basic statistics (avg score, completion rate)

### Phase 8: Polish & Extras
- [ ] Responsive CSS styling
- [ ] Timer functionality
- [ ] Question/choice shuffling
- [ ] Error handling improvements
- [ ] Input validation refinement

---

## Configuration

Environment-based configuration following project patterns:

```python
# config.py
import os
import json
from pathlib import Path

class Config:
    DATA_DIR: Path
    HOST: str
    PORT: int
    SECRET_KEY: str  # For session signing
    SESSION_EXPIRY_HOURS: int

    @classmethod
    def load(cls, config_path: str | None = None):
        path = config_path or os.environ.get("DEVQUIZ_CONFIG")
        if path:
            with open(path) as f:
                data = json.load(f)
            # ... populate from JSON
        else:
            # Defaults
            cls.DATA_DIR = Path("./data")
            cls.HOST = "0.0.0.0"
            cls.PORT = 8080
            cls.SECRET_KEY = os.environ.get("DEVQUIZ_SECRET", "dev-secret-change-me")
            cls.SESSION_EXPIRY_HOURS = 24
```

Example config JSON:
```json
{
  "data_dir": "./data",
  "host": "0.0.0.0",
  "port": 8080,
  "secret_key": "your-production-secret",
  "session_expiry_hours": 24
}
```

---

## Security Considerations

1. **Password Storage**: Use bcrypt for admin password hashing
2. **Session Tokens**: Use `secrets.token_hex()` for cryptographic randomness
3. **CSRF Protection**: Include CSRF tokens in forms
4. **Input Validation**: Pydantic models validate all input
5. **Path Traversal**: Sanitize all file paths in storage layer
6. **Rate Limiting**: Consider adding rate limits for quiz submissions

---

## Future Enhancements (Out of Scope)

- User accounts for quiz takers
- Quiz scheduling (open/close dates)
- Question media (images, code blocks)
- Multiple correct answers
- Fill-in-the-blank questions
- Quiz templates
- Email notifications
- Detailed analytics/charts
- Question bank sharing/import from external sources
- API for programmatic quiz creation

---

## Running the Service

```bash
# Install dependencies
pip install -r requirements.txt

# Set configuration (optional)
export DEVQUIZ_CONFIG=./config.json

# Run development server
python -m uvicorn app:app --reload --port 8080

# Or production
python -m uvicorn app:app --host 0.0.0.0 --port 8080
```

---

## Dependencies

```
# requirements.txt
fastapi>=0.100.0
uvicorn[standard]>=0.23.0
pydantic>=2.0.0
bcrypt>=4.0.0
jinja2>=3.1.0
python-multipart>=0.0.6  # For form handling
```
