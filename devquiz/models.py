"""Pydantic models for DevQuiz."""
from datetime import datetime
from typing import Literal
from pydantic import BaseModel, Field
import secrets


def generate_id() -> str:
    """Generate an 8-character alphanumeric ID."""
    return secrets.token_hex(4)


class Choice(BaseModel):
    id: str = Field(default_factory=generate_id)
    text: str


class Question(BaseModel):
    id: str = Field(default_factory=generate_id)
    text: str
    question_type: Literal["multiple_choice", "true_false"] = "multiple_choice"
    choices: list[Choice]
    correct_answer_id: str
    explanation: str | None = None
    tags: list[str] = []
    difficulty: Literal["easy", "medium", "hard"] | None = None


class QuestionBank(BaseModel):
    id: str = Field(default_factory=generate_id)
    name: str
    description: str | None = None
    questions: list[Question] = []
    created_at: datetime = Field(default_factory=datetime.utcnow)
    created_by: str


class InfiniteConfig(BaseModel):
    source_bank_ids: list[str]
    questions_per_fetch: int = 5
    allow_repeats: bool = False


class Quiz(BaseModel):
    id: str = Field(default_factory=generate_id)
    quiz_type: Literal["static", "generated", "infinite"]
    title: str
    description: str | None = None
    questions: list[Question] | None = None
    source_bank_id: str | None = None
    question_count: int | None = None
    infinite_config: InfiniteConfig | None = None
    created_at: datetime = Field(default_factory=datetime.utcnow)
    created_by: str
    is_active: bool = True
    time_limit_minutes: int | None = None
    shuffle_questions: bool = False
    shuffle_choices: bool = False
    show_correct_answers: bool = True
    require_name: bool = False


class Answer(BaseModel):
    question_id: str
    selected_choice_id: str
    is_correct: bool = False


class QuizSubmission(BaseModel):
    id: str = Field(default_factory=generate_id)
    quiz_id: str
    taker_name: str | None = None
    taker_email: str | None = None
    answers: list[Answer] = []
    score: int = 0
    total_questions: int = 0
    percentage: float = 0.0
    started_at: datetime = Field(default_factory=datetime.utcnow)
    submitted_at: datetime | None = None
    time_taken_seconds: int = 0


class Admin(BaseModel):
    id: str = Field(default_factory=generate_id)
    username: str
    password_hash: str
    created_at: datetime = Field(default_factory=datetime.utcnow)
    last_login: datetime | None = None


class AdminSession(BaseModel):
    token: str = Field(default_factory=lambda: secrets.token_hex(16))
    admin_id: str
    created_at: datetime = Field(default_factory=datetime.utcnow)
    expires_at: datetime


# Form models for API requests
class QuizStartRequest(BaseModel):
    taker_name: str | None = None
    taker_email: str | None = None


class QuizSubmitRequest(BaseModel):
    submission_id: str
    answers: dict[str, str]  # question_id -> choice_id


class LoginRequest(BaseModel):
    username: str
    password: str


class QuestionBankCreate(BaseModel):
    name: str
    description: str | None = None


class QuestionCreate(BaseModel):
    text: str
    choices: list[str]  # List of choice texts
    correct_index: int  # Index of correct choice
    explanation: str | None = None
    difficulty: Literal["easy", "medium", "hard"] | None = None
    tags: list[str] = []


class QuizCreate(BaseModel):
    title: str
    description: str | None = None
    quiz_type: Literal["static", "generated", "infinite"] = "static"
    question_bank_id: str | None = None
    question_count: int | None = None
    shuffle_questions: bool = False
    shuffle_choices: bool = False
    show_correct_answers: bool = True
    require_name: bool = False
    time_limit_minutes: int | None = None
