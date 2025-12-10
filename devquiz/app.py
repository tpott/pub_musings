"""DevQuiz - A quiz service with multiple quiz types."""
import random
from datetime import datetime
from contextlib import asynccontextmanager
from fastapi import FastAPI, Request, Form, Depends, HTTPException
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.staticfiles import StaticFiles
from fastapi.templating import Jinja2Templates
from pathlib import Path

from config import Config
from models import (
    Quiz, QuestionBank, Question, Choice, Admin,
    generate_id, QuizCreate, QuestionCreate
)
from storage import (
    QuizStorage, QuestionBankStorage, SubmissionStorage, AdminStorage
)
from auth import (
    authenticate_admin, create_session, require_admin,
    ensure_default_admin, get_session_token, validate_session,
    SessionStorage, hash_password
)
from quiz_logic import (
    start_quiz_session, submit_quiz, get_submission_results,
    get_questions_for_taker, generate_quiz_from_bank
)


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup
    Config.load()
    ensure_default_admin()
    create_sample_data()
    yield
    # Shutdown
    pass


app = FastAPI(title="DevQuiz", lifespan=lifespan)

# Templates
templates_dir = Path(__file__).parent / "templates"
templates_dir.mkdir(exist_ok=True)
templates = Jinja2Templates(directory=str(templates_dir))

# Static files
static_dir = Path(__file__).parent / "static"
static_dir.mkdir(exist_ok=True)
app.mount("/static", StaticFiles(directory=str(static_dir)), name="static")


def create_sample_data():
    """Create sample question bank and quiz for demo."""
    # Check if sample data already exists
    if QuestionBankStorage.list_all():
        return

    # Create a sample question bank
    bank = QuestionBank(
        id="sample_bank",
        name="Programming Fundamentals",
        description="Basic programming questions for developers",
        created_by="admin",
        questions=[
            Question(
                id="q1",
                text="What does HTML stand for?",
                choices=[
                    Choice(id="q1a", text="Hyper Text Markup Language"),
                    Choice(id="q1b", text="High Tech Modern Language"),
                    Choice(id="q1c", text="Home Tool Markup Language"),
                    Choice(id="q1d", text="Hyperlink Text Management Language"),
                ],
                correct_answer_id="q1a",
                explanation="HTML stands for Hyper Text Markup Language, the standard markup language for web pages.",
                difficulty="easy",
                tags=["web", "html"]
            ),
            Question(
                id="q2",
                text="Which of the following is NOT a programming language?",
                choices=[
                    Choice(id="q2a", text="Python"),
                    Choice(id="q2b", text="Java"),
                    Choice(id="q2c", text="HTML"),
                    Choice(id="q2d", text="JavaScript"),
                ],
                correct_answer_id="q2c",
                explanation="HTML is a markup language, not a programming language. It doesn't have logic or programming constructs.",
                difficulty="easy",
                tags=["basics"]
            ),
            Question(
                id="q3",
                text="What is the time complexity of binary search?",
                choices=[
                    Choice(id="q3a", text="O(n)"),
                    Choice(id="q3b", text="O(log n)"),
                    Choice(id="q3c", text="O(n²)"),
                    Choice(id="q3d", text="O(1)"),
                ],
                correct_answer_id="q3b",
                explanation="Binary search has O(log n) time complexity because it halves the search space with each comparison.",
                difficulty="medium",
                tags=["algorithms", "complexity"]
            ),
            Question(
                id="q4",
                text="What does CSS stand for?",
                choices=[
                    Choice(id="q4a", text="Computer Style Sheets"),
                    Choice(id="q4b", text="Cascading Style Sheets"),
                    Choice(id="q4c", text="Creative Style System"),
                    Choice(id="q4d", text="Colorful Style Sheets"),
                ],
                correct_answer_id="q4b",
                explanation="CSS stands for Cascading Style Sheets, used to style HTML elements.",
                difficulty="easy",
                tags=["web", "css"]
            ),
            Question(
                id="q5",
                text="Which data structure uses LIFO (Last In, First Out)?",
                choices=[
                    Choice(id="q5a", text="Queue"),
                    Choice(id="q5b", text="Stack"),
                    Choice(id="q5c", text="Array"),
                    Choice(id="q5d", text="Linked List"),
                ],
                correct_answer_id="q5b",
                explanation="A Stack uses LIFO - the last element added is the first one to be removed.",
                difficulty="medium",
                tags=["data-structures"]
            ),
            Question(
                id="q6",
                text="What is the purpose of a constructor in OOP?",
                choices=[
                    Choice(id="q6a", text="To destroy objects"),
                    Choice(id="q6b", text="To initialize objects"),
                    Choice(id="q6c", text="To copy objects"),
                    Choice(id="q6d", text="To compare objects"),
                ],
                correct_answer_id="q6b",
                explanation="A constructor is a special method used to initialize objects when they are created.",
                difficulty="medium",
                tags=["oop"]
            ),
            Question(
                id="q7",
                text="Which HTTP method is used to update a resource?",
                choices=[
                    Choice(id="q7a", text="GET"),
                    Choice(id="q7b", text="POST"),
                    Choice(id="q7c", text="PUT"),
                    Choice(id="q7d", text="DELETE"),
                ],
                correct_answer_id="q7c",
                explanation="PUT is the HTTP method typically used to update an existing resource.",
                difficulty="medium",
                tags=["web", "http"]
            ),
            Question(
                id="q8",
                text="What is Git primarily used for?",
                choices=[
                    Choice(id="q8a", text="Database management"),
                    Choice(id="q8b", text="Version control"),
                    Choice(id="q8c", text="Web hosting"),
                    Choice(id="q8d", text="Code compilation"),
                ],
                correct_answer_id="q8b",
                explanation="Git is a distributed version control system used to track changes in source code.",
                difficulty="easy",
                tags=["tools", "git"]
            ),
        ]
    )
    QuestionBankStorage.save(bank)

    # Create a sample static quiz
    quiz = Quiz(
        id="demo_quiz",
        quiz_type="static",
        title="Programming Basics Quiz",
        description="Test your knowledge of programming fundamentals! This quiz covers HTML, CSS, data structures, and more.",
        questions=bank.questions[:5],  # First 5 questions
        created_by="admin",
        show_correct_answers=True,
        shuffle_questions=False
    )
    QuizStorage.save(quiz)
    print("Created sample question bank and quiz")


# ============== Public Quiz Routes ==============

@app.get("/", response_class=HTMLResponse)
async def home(request: Request):
    """Home page with list of available quizzes."""
    quizzes = [q for q in QuizStorage.list_all() if q.is_active]
    return templates.TemplateResponse("home.html", {
        "request": request,
        "quizzes": quizzes
    })


@app.get("/quiz/{quiz_id}", response_class=HTMLResponse)
async def quiz_start_page(request: Request, quiz_id: str):
    """Quiz landing/start page."""
    quiz = QuizStorage.load(quiz_id)
    if not quiz or not quiz.is_active:
        raise HTTPException(status_code=404, detail="Quiz not found")
    return templates.TemplateResponse("quiz_start.html", {
        "request": request,
        "quiz": quiz
    })


@app.post("/quiz/{quiz_id}/start")
async def start_quiz(request: Request, quiz_id: str, taker_name: str = Form(None)):
    """Start a quiz session."""
    quiz = QuizStorage.load(quiz_id)
    if not quiz or not quiz.is_active:
        raise HTTPException(status_code=404, detail="Quiz not found")

    submission = start_quiz_session(quiz_id, taker_name=taker_name)
    return RedirectResponse(
        url=f"/quiz/{quiz_id}/take/{submission.id}",
        status_code=303
    )


@app.get("/quiz/{quiz_id}/take/{submission_id}", response_class=HTMLResponse)
async def take_quiz(request: Request, quiz_id: str, submission_id: str):
    """Quiz taking page."""
    quiz = QuizStorage.load(quiz_id)
    submission = SubmissionStorage.load(quiz_id, submission_id)

    if not quiz or not submission:
        raise HTTPException(status_code=404, detail="Quiz or session not found")

    if submission.submitted_at:
        return RedirectResponse(url=f"/quiz/{quiz_id}/result/{submission_id}")

    questions = get_questions_for_taker(quiz)
    return templates.TemplateResponse("quiz_take.html", {
        "request": request,
        "quiz": quiz,
        "submission_id": submission_id,
        "questions": questions
    })


@app.post("/quiz/{quiz_id}/submit/{submission_id}")
async def submit_quiz_answers(request: Request, quiz_id: str, submission_id: str):
    """Submit quiz answers."""
    form_data = await request.form()
    answers = {}
    for key, value in form_data.items():
        if key.startswith("q_"):
            question_id = key[2:]
            answers[question_id] = value

    submission = submit_quiz(quiz_id, submission_id, answers)
    if not submission:
        raise HTTPException(status_code=404, detail="Submission not found")

    return RedirectResponse(
        url=f"/quiz/{quiz_id}/result/{submission_id}",
        status_code=303
    )


@app.get("/quiz/{quiz_id}/result/{submission_id}", response_class=HTMLResponse)
async def view_result(request: Request, quiz_id: str, submission_id: str):
    """View quiz results."""
    results = get_submission_results(quiz_id, submission_id)
    if not results:
        raise HTTPException(status_code=404, detail="Results not found")

    return templates.TemplateResponse("quiz_result.html", {
        "request": request,
        "results": results,
        "quiz_id": quiz_id
    })


# ============== Admin Auth Routes ==============

@app.get("/admin/login", response_class=HTMLResponse)
async def admin_login_page(request: Request):
    """Admin login page."""
    return templates.TemplateResponse("admin_login.html", {
        "request": request,
        "error": None
    })


@app.post("/admin/login")
async def admin_login(request: Request, username: str = Form(...), password: str = Form(...)):
    """Process admin login."""
    admin = authenticate_admin(username, password)
    if not admin:
        return templates.TemplateResponse("admin_login.html", {
            "request": request,
            "error": "Invalid username or password"
        })

    session = create_session(admin)
    response = RedirectResponse(url="/admin", status_code=303)
    response.set_cookie(
        key="session_token",
        value=session.token,
        httponly=True,
        max_age=Config.SESSION_EXPIRY_HOURS * 3600
    )
    return response


@app.get("/admin/logout")
async def admin_logout(request: Request):
    """Admin logout."""
    token = get_session_token(request)
    if token:
        SessionStorage.delete(token)
    response = RedirectResponse(url="/", status_code=303)
    response.delete_cookie("session_token")
    return response


# ============== Admin Dashboard Routes ==============

@app.get("/admin", response_class=HTMLResponse)
async def admin_dashboard(request: Request, admin: Admin = Depends(require_admin)):
    """Admin dashboard."""
    quizzes = QuizStorage.list_all()
    banks = QuestionBankStorage.list_all()

    # Get submission counts for each quiz
    quiz_stats = []
    for quiz in quizzes:
        submissions = SubmissionStorage.list_for_quiz(quiz.id)
        completed = [s for s in submissions if s.submitted_at]
        avg_score = sum(s.percentage for s in completed) / len(completed) if completed else 0
        quiz_stats.append({
            "quiz": quiz,
            "submission_count": len(completed),
            "avg_score": round(avg_score, 1)
        })

    return templates.TemplateResponse("admin_dashboard.html", {
        "request": request,
        "admin": admin,
        "quiz_stats": quiz_stats,
        "banks": banks
    })


@app.get("/admin/quiz/{quiz_id}/results", response_class=HTMLResponse)
async def admin_quiz_results(request: Request, quiz_id: str, admin: Admin = Depends(require_admin)):
    """View all submissions for a quiz."""
    quiz = QuizStorage.load(quiz_id)
    if not quiz:
        raise HTTPException(status_code=404, detail="Quiz not found")

    submissions = SubmissionStorage.list_for_quiz(quiz_id)
    completed = [s for s in submissions if s.submitted_at]
    completed.sort(key=lambda s: s.submitted_at, reverse=True)

    return templates.TemplateResponse("admin_results.html", {
        "request": request,
        "admin": admin,
        "quiz": quiz,
        "submissions": completed
    })


@app.get("/admin/quiz/new", response_class=HTMLResponse)
async def new_quiz_page(request: Request, admin: Admin = Depends(require_admin)):
    """Create new quiz page."""
    banks = QuestionBankStorage.list_all()
    return templates.TemplateResponse("admin_quiz_new.html", {
        "request": request,
        "admin": admin,
        "banks": banks
    })


@app.post("/admin/quiz/create")
async def create_quiz(
    request: Request,
    title: str = Form(...),
    description: str = Form(""),
    quiz_type: str = Form("generated"),
    question_bank_id: str = Form(...),
    question_count: int = Form(5),
    shuffle_questions: bool = Form(False),
    show_correct_answers: bool = Form(True),
    admin: Admin = Depends(require_admin)
):
    """Create a new quiz."""
    if quiz_type == "generated":
        quiz = generate_quiz_from_bank(
            bank_id=question_bank_id,
            question_count=question_count,
            title=title,
            description=description or None,
            admin_id=admin.id,
            shuffle_questions=shuffle_questions,
            show_correct_answers=show_correct_answers
        )
        if not quiz:
            raise HTTPException(status_code=400, detail="Could not generate quiz from bank")
    else:
        # Static quiz from entire bank
        bank = QuestionBankStorage.load(question_bank_id)
        if not bank:
            raise HTTPException(status_code=404, detail="Question bank not found")
        quiz = Quiz(
            quiz_type="static",
            title=title,
            description=description or None,
            questions=bank.questions,
            created_by=admin.id,
            shuffle_questions=shuffle_questions,
            show_correct_answers=show_correct_answers
        )
        QuizStorage.save(quiz)

    return RedirectResponse(url="/admin", status_code=303)


@app.post("/admin/quiz/{quiz_id}/toggle")
async def toggle_quiz(quiz_id: str, admin: Admin = Depends(require_admin)):
    """Toggle quiz active status."""
    quiz = QuizStorage.load(quiz_id)
    if not quiz:
        raise HTTPException(status_code=404, detail="Quiz not found")
    quiz.is_active = not quiz.is_active
    QuizStorage.save(quiz)
    return RedirectResponse(url="/admin", status_code=303)


@app.post("/admin/quiz/{quiz_id}/delete")
async def delete_quiz(quiz_id: str, admin: Admin = Depends(require_admin)):
    """Delete a quiz."""
    QuizStorage.delete(quiz_id)
    return RedirectResponse(url="/admin", status_code=303)


# ============== Question Bank Routes ==============

@app.get("/admin/bank/new", response_class=HTMLResponse)
async def new_bank_page(request: Request, admin: Admin = Depends(require_admin)):
    """Create new question bank page."""
    return templates.TemplateResponse("admin_bank_new.html", {
        "request": request,
        "admin": admin
    })


@app.post("/admin/bank/create")
async def create_bank(
    request: Request,
    name: str = Form(...),
    description: str = Form(""),
    admin: Admin = Depends(require_admin)
):
    """Create a new question bank."""
    bank = QuestionBank(
        name=name,
        description=description or None,
        created_by=admin.id
    )
    QuestionBankStorage.save(bank)
    return RedirectResponse(url=f"/admin/bank/{bank.id}", status_code=303)


@app.get("/admin/bank/{bank_id}", response_class=HTMLResponse)
async def view_bank(request: Request, bank_id: str, admin: Admin = Depends(require_admin)):
    """View and edit question bank."""
    bank = QuestionBankStorage.load(bank_id)
    if not bank:
        raise HTTPException(status_code=404, detail="Question bank not found")

    return templates.TemplateResponse("admin_bank_view.html", {
        "request": request,
        "admin": admin,
        "bank": bank
    })


@app.post("/admin/bank/{bank_id}/add-question")
async def add_question_to_bank(
    request: Request,
    bank_id: str,
    question_text: str = Form(...),
    choice_1: str = Form(...),
    choice_2: str = Form(...),
    choice_3: str = Form(""),
    choice_4: str = Form(""),
    correct_choice: int = Form(...),
    explanation: str = Form(""),
    admin: Admin = Depends(require_admin)
):
    """Add a question to a bank."""
    bank = QuestionBankStorage.load(bank_id)
    if not bank:
        raise HTTPException(status_code=404, detail="Question bank not found")

    choices = [Choice(text=c) for c in [choice_1, choice_2, choice_3, choice_4] if c.strip()]
    correct_idx = min(correct_choice - 1, len(choices) - 1)

    question = Question(
        text=question_text,
        choices=choices,
        correct_answer_id=choices[correct_idx].id,
        explanation=explanation or None
    )
    bank.questions.append(question)
    QuestionBankStorage.save(bank)

    return RedirectResponse(url=f"/admin/bank/{bank_id}", status_code=303)


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host=Config.HOST, port=Config.PORT)
