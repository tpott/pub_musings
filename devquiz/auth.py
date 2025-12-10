"""Authentication utilities for DevQuiz."""
from datetime import datetime, timedelta
from fastapi import Request, HTTPException, Depends
from fastapi.responses import RedirectResponse
import bcrypt

from config import Config
from models import Admin, AdminSession, generate_id
from storage import AdminStorage, SessionStorage


def hash_password(password: str) -> str:
    """Hash a password using bcrypt."""
    return bcrypt.hashpw(password.encode(), bcrypt.gensalt()).decode()


def verify_password(password: str, password_hash: str) -> bool:
    """Verify a password against its hash."""
    return bcrypt.checkpw(password.encode(), password_hash.encode())


def create_admin(username: str, password: str) -> Admin:
    """Create a new admin user."""
    admin = Admin(
        id=generate_id(),
        username=username,
        password_hash=hash_password(password)
    )
    AdminStorage.save(admin)
    return admin


def authenticate_admin(username: str, password: str) -> Admin | None:
    """Authenticate an admin by username and password."""
    admin = AdminStorage.find_by_username(username)
    if admin and verify_password(password, admin.password_hash):
        admin.last_login = datetime.utcnow()
        AdminStorage.save(admin)
        return admin
    return None


def create_session(admin: Admin) -> AdminSession:
    """Create a new session for an admin."""
    session = AdminSession(
        admin_id=admin.id,
        expires_at=datetime.utcnow() + timedelta(hours=Config.SESSION_EXPIRY_HOURS)
    )
    SessionStorage.save(session)
    return session


def validate_session(token: str) -> AdminSession | None:
    """Validate a session token and return the session if valid."""
    session = SessionStorage.load(token)
    if session and session.expires_at > datetime.utcnow():
        return session
    if session:
        SessionStorage.delete(token)
    return None


def get_session_token(request: Request) -> str | None:
    """Extract session token from request cookies."""
    return request.cookies.get("session_token")


async def require_admin(request: Request) -> Admin:
    """Dependency that requires a valid admin session."""
    token = get_session_token(request)
    if not token:
        raise HTTPException(status_code=303, headers={"Location": "/admin/login"})

    session = validate_session(token)
    if not session:
        raise HTTPException(status_code=303, headers={"Location": "/admin/login"})

    admin = AdminStorage.load(session.admin_id)
    if not admin:
        raise HTTPException(status_code=303, headers={"Location": "/admin/login"})

    return admin


async def optional_admin(request: Request) -> Admin | None:
    """Dependency that optionally gets the current admin."""
    token = get_session_token(request)
    if not token:
        return None

    session = validate_session(token)
    if not session:
        return None

    return AdminStorage.load(session.admin_id)


def ensure_default_admin():
    """Ensure a default admin exists for development."""
    if not AdminStorage.list_all():
        create_admin("admin", "admin")
        print("Created default admin user: admin / admin")
