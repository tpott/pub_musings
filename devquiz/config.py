"""Configuration management for DevQuiz."""
import os
import json
from pathlib import Path


class Config:
    DATA_DIR: Path = Path("./data")
    HOST: str = "0.0.0.0"
    PORT: int = 8080
    SECRET_KEY: str = "dev-secret-change-me"
    SESSION_EXPIRY_HOURS: int = 24

    @classmethod
    def load(cls, config_path: str | None = None):
        path = config_path or os.environ.get("DEVQUIZ_CONFIG")
        if path and os.path.exists(path):
            with open(path) as f:
                data = json.load(f)
            cls.DATA_DIR = Path(data.get("data_dir", "./data"))
            cls.HOST = data.get("host", "0.0.0.0")
            cls.PORT = data.get("port", 8080)
            cls.SECRET_KEY = data.get("secret_key", cls.SECRET_KEY)
            cls.SESSION_EXPIRY_HOURS = data.get("session_expiry_hours", 24)

        # Override with environment variables
        if os.environ.get("DEVQUIZ_SECRET"):
            cls.SECRET_KEY = os.environ["DEVQUIZ_SECRET"]
        if os.environ.get("DEVQUIZ_PORT"):
            cls.PORT = int(os.environ["DEVQUIZ_PORT"])

        # Ensure data directories exist
        cls.DATA_DIR.mkdir(parents=True, exist_ok=True)
        (cls.DATA_DIR / "question_banks").mkdir(exist_ok=True)
        (cls.DATA_DIR / "quizzes").mkdir(exist_ok=True)
        (cls.DATA_DIR / "submissions").mkdir(exist_ok=True)
        (cls.DATA_DIR / "admins").mkdir(exist_ok=True)
        (cls.DATA_DIR / "sessions").mkdir(exist_ok=True)
