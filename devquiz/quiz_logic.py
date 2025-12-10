"""Quiz business logic for DevQuiz."""
import random
from datetime import datetime
from models import (
    Quiz, QuestionBank, Question, Choice, QuizSubmission, Answer,
    generate_id, QuestionCreate
)
from storage import QuizStorage, QuestionBankStorage, SubmissionStorage


def create_question_from_form(data: QuestionCreate) -> Question:
    """Create a Question model from form data."""
    choices = [Choice(text=text) for text in data.choices]
    correct_id = choices[data.correct_index].id if 0 <= data.correct_index < len(choices) else choices[0].id

    return Question(
        text=data.text,
        choices=choices,
        correct_answer_id=correct_id,
        explanation=data.explanation,
        difficulty=data.difficulty,
        tags=data.tags
    )


def create_static_quiz(
    title: str,
    questions: list[Question],
    admin_id: str,
    description: str | None = None,
    **kwargs
) -> Quiz:
    """Create a static quiz with predefined questions."""
    quiz = Quiz(
        quiz_type="static",
        title=title,
        description=description,
        questions=questions,
        created_by=admin_id,
        **kwargs
    )
    QuizStorage.save(quiz)
    return quiz


def generate_quiz_from_bank(
    bank_id: str,
    question_count: int,
    title: str,
    admin_id: str,
    description: str | None = None,
    difficulty: str | None = None,
    tags: list[str] | None = None,
    **kwargs
) -> Quiz | None:
    """Generate a quiz by selecting random questions from a bank."""
    bank = QuestionBankStorage.load(bank_id)
    if not bank:
        return None

    pool = bank.questions

    # Filter by difficulty
    if difficulty:
        pool = [q for q in pool if q.difficulty == difficulty]

    # Filter by tags
    if tags:
        pool = [q for q in pool if any(t in q.tags for t in tags)]

    # Random selection
    count = min(question_count, len(pool))
    if count == 0:
        return None

    selected = random.sample(pool, count)

    quiz = Quiz(
        quiz_type="generated",
        title=title,
        description=description,
        questions=selected,
        source_bank_id=bank_id,
        question_count=question_count,
        created_by=admin_id,
        **kwargs
    )
    QuizStorage.save(quiz)
    return quiz


def get_quiz_questions(quiz: Quiz, shuffle: bool = False) -> list[Question]:
    """Get questions for a quiz, optionally shuffled."""
    questions = quiz.questions or []
    if shuffle or quiz.shuffle_questions:
        questions = random.sample(questions, len(questions))
    return questions


def get_questions_for_taker(quiz: Quiz) -> list[dict]:
    """Get questions formatted for quiz taker (without correct answers)."""
    questions = get_quiz_questions(quiz)
    result = []
    for q in questions:
        choices = q.choices
        if quiz.shuffle_choices:
            choices = random.sample(choices, len(choices))
        result.append({
            "id": q.id,
            "text": q.text,
            "choices": [{"id": c.id, "text": c.text} for c in choices]
        })
    return result


def start_quiz_session(quiz_id: str, taker_name: str | None = None, taker_email: str | None = None) -> QuizSubmission:
    """Start a new quiz submission session."""
    quiz = QuizStorage.load(quiz_id)
    submission = QuizSubmission(
        quiz_id=quiz_id,
        taker_name=taker_name,
        taker_email=taker_email,
        total_questions=len(quiz.questions) if quiz and quiz.questions else 0
    )
    SubmissionStorage.save(submission)
    return submission


def submit_quiz(quiz_id: str, submission_id: str, answers: dict[str, str]) -> QuizSubmission | None:
    """Submit quiz answers and calculate score."""
    quiz = QuizStorage.load(quiz_id)
    submission = SubmissionStorage.load(quiz_id, submission_id)

    if not quiz or not submission:
        return None

    # Build answer list and calculate score
    answer_list = []
    score = 0

    for question in quiz.questions or []:
        selected_id = answers.get(question.id, "")
        is_correct = selected_id == question.correct_answer_id
        if is_correct:
            score += 1
        answer_list.append(Answer(
            question_id=question.id,
            selected_choice_id=selected_id,
            is_correct=is_correct
        ))

    # Update submission
    submission.answers = answer_list
    submission.score = score
    submission.total_questions = len(quiz.questions) if quiz.questions else 0
    submission.percentage = (score / submission.total_questions * 100) if submission.total_questions > 0 else 0
    submission.submitted_at = datetime.utcnow()
    submission.time_taken_seconds = int((submission.submitted_at - submission.started_at).total_seconds())

    SubmissionStorage.save(submission)
    return submission


def get_submission_results(quiz_id: str, submission_id: str) -> dict | None:
    """Get detailed results for a submission."""
    quiz = QuizStorage.load(quiz_id)
    submission = SubmissionStorage.load(quiz_id, submission_id)

    if not quiz or not submission:
        return None

    # Build detailed results
    questions_results = []
    answers_map = {a.question_id: a for a in submission.answers}

    for question in quiz.questions or []:
        answer = answers_map.get(question.id)
        selected_choice = None
        correct_choice = None

        for choice in question.choices:
            if answer and choice.id == answer.selected_choice_id:
                selected_choice = choice
            if choice.id == question.correct_answer_id:
                correct_choice = choice

        questions_results.append({
            "question": question.text,
            "selected": selected_choice.text if selected_choice else "No answer",
            "correct": correct_choice.text if correct_choice else "",
            "is_correct": answer.is_correct if answer else False,
            "explanation": question.explanation
        })

    return {
        "quiz_title": quiz.title,
        "taker_name": submission.taker_name,
        "score": submission.score,
        "total": submission.total_questions,
        "percentage": submission.percentage,
        "time_taken": submission.time_taken_seconds,
        "questions": questions_results,
        "show_correct_answers": quiz.show_correct_answers
    }
