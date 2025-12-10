"""File-based JSON storage layer for DevQuiz."""
import json
from pathlib import Path
from typing import TypeVar, Type
from pydantic import BaseModel

from config import Config
from models import Quiz, QuestionBank, QuizSubmission, Admin, AdminSession

T = TypeVar('T', bound=BaseModel)


class Storage:
    """Generic file-based JSON storage."""

    @staticmethod
    def _get_path(subdir: str, item_id: str) -> Path:
        return Config.DATA_DIR / subdir / f"{item_id}.json"

    @staticmethod
    def save(subdir: str, item_id: str, data: BaseModel) -> None:
        path = Storage._get_path(subdir, item_id)
        path.parent.mkdir(parents=True, exist_ok=True)
        with open(path, 'w') as f:
            f.write(data.model_dump_json(indent=2))

    @staticmethod
    def load(subdir: str, item_id: str, model: Type[T]) -> T | None:
        path = Storage._get_path(subdir, item_id)
        if not path.exists():
            return None
        with open(path) as f:
            return model.model_validate_json(f.read())

    @staticmethod
    def delete(subdir: str, item_id: str) -> bool:
        path = Storage._get_path(subdir, item_id)
        if path.exists():
            path.unlink()
            return True
        return False

    @staticmethod
    def list_all(subdir: str, model: Type[T]) -> list[T]:
        dir_path = Config.DATA_DIR / subdir
        if not dir_path.exists():
            return []
        items = []
        for path in dir_path.glob("*.json"):
            with open(path) as f:
                items.append(model.model_validate_json(f.read()))
        return items

    @staticmethod
    def exists(subdir: str, item_id: str) -> bool:
        return Storage._get_path(subdir, item_id).exists()


# Convenience functions for each model type
class QuizStorage:
    SUBDIR = "quizzes"

    @classmethod
    def save(cls, quiz: Quiz) -> None:
        Storage.save(cls.SUBDIR, quiz.id, quiz)

    @classmethod
    def load(cls, quiz_id: str) -> Quiz | None:
        return Storage.load(cls.SUBDIR, quiz_id, Quiz)

    @classmethod
    def delete(cls, quiz_id: str) -> bool:
        return Storage.delete(cls.SUBDIR, quiz_id)

    @classmethod
    def list_all(cls) -> list[Quiz]:
        return Storage.list_all(cls.SUBDIR, Quiz)


class QuestionBankStorage:
    SUBDIR = "question_banks"

    @classmethod
    def save(cls, bank: QuestionBank) -> None:
        Storage.save(cls.SUBDIR, bank.id, bank)

    @classmethod
    def load(cls, bank_id: str) -> QuestionBank | None:
        return Storage.load(cls.SUBDIR, bank_id, QuestionBank)

    @classmethod
    def delete(cls, bank_id: str) -> bool:
        return Storage.delete(cls.SUBDIR, bank_id)

    @classmethod
    def list_all(cls) -> list[QuestionBank]:
        return Storage.list_all(cls.SUBDIR, QuestionBank)


class SubmissionStorage:
    SUBDIR = "submissions"

    @classmethod
    def save(cls, submission: QuizSubmission) -> None:
        # Store under submissions/{quiz_id}/{submission_id}.json
        subdir = f"{cls.SUBDIR}/{submission.quiz_id}"
        Storage.save(subdir, submission.id, submission)

    @classmethod
    def load(cls, quiz_id: str, submission_id: str) -> QuizSubmission | None:
        subdir = f"{cls.SUBDIR}/{quiz_id}"
        return Storage.load(subdir, submission_id, QuizSubmission)

    @classmethod
    def list_for_quiz(cls, quiz_id: str) -> list[QuizSubmission]:
        subdir = f"{cls.SUBDIR}/{quiz_id}"
        return Storage.list_all(subdir, QuizSubmission)


class AdminStorage:
    SUBDIR = "admins"

    @classmethod
    def save(cls, admin: Admin) -> None:
        Storage.save(cls.SUBDIR, admin.id, admin)

    @classmethod
    def load(cls, admin_id: str) -> Admin | None:
        return Storage.load(cls.SUBDIR, admin_id, Admin)

    @classmethod
    def find_by_username(cls, username: str) -> Admin | None:
        for admin in Storage.list_all(cls.SUBDIR, Admin):
            if admin.username == username:
                return admin
        return None

    @classmethod
    def list_all(cls) -> list[Admin]:
        return Storage.list_all(cls.SUBDIR, Admin)


class SessionStorage:
    SUBDIR = "sessions"

    @classmethod
    def save(cls, session: AdminSession) -> None:
        Storage.save(cls.SUBDIR, session.token, session)

    @classmethod
    def load(cls, token: str) -> AdminSession | None:
        return Storage.load(cls.SUBDIR, token, AdminSession)

    @classmethod
    def delete(cls, token: str) -> bool:
        return Storage.delete(cls.SUBDIR, token)
