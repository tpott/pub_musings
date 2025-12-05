# session_storage.py

import json
import os
import secrets
import shutil
import time
from datetime import datetime
from typing import Dict, List, Optional

from serve_config import getConfig, PAYMENT_LINK_EXPIRY_HOURS


def create_payment_session(
    user_id: str, conversation_id: str, name: Optional[str] = None
) -> str:
    """
    Create a new payment session and update user/conversation status files.

    Args:
        user_id: Facebook user ID
        conversation_id: Facebook conversation/thread ID
        name: Optional user name to store

    Returns:
        session_id: 8-character hex string

    Side effects:
        - Creates payment_sessions_dir/{session_id}.json
        - Creates/updates users_dir/{user_id}/status.json
        - Creates/updates conversations_dir/{conversation_id}/status.json
    """
    config = getConfig()
    session_id = secrets.token_hex(4)  # 8 hex chars

    # Create session file
    session = {
        "user_id": user_id,
        "conversation_id": conversation_id,
        "status": "pending",
        "created_at": int(time.time()),
        "expires_at": int(time.time()) + (PAYMENT_LINK_EXPIRY_HOURS * 3600),
        "facebook_response": None,
    }
    session_path = os.path.join(config["payment_sessions_dir"], f"{session_id}.json")
    with open(session_path, "w") as f:
        json.dump(session, f, indent=2)

    # Update user status
    update_user_status(user_id, session_id, conversation_id, name)

    # Update conversation status
    update_conversation_status(conversation_id, session_id, [user_id])

    return session_id


def get_payment_session(session_id: str) -> Optional[Dict]:
    """
    Retrieve a payment session by ID, checking expiry.

    Args:
        session_id: 8-character hex string

    Returns:
        Session dict if found and not expired, None otherwise
    """
    config = getConfig()
    session_path = os.path.join(config["payment_sessions_dir"], f"{session_id}.json")

    if not os.path.exists(session_path):
        return None

    with open(session_path, "r") as f:
        session = json.load(f)

    # Check expiry
    if session["expires_at"] < int(time.time()):
        return None

    return session


def complete_payment_session(session_id: str, facebook_response: Dict) -> bool:
    """
    Mark a payment session as completed and store Facebook's response.

    Args:
        session_id: 8-character hex string
        facebook_response: Full response object from FB.ui() callback

    Returns:
        True if successful, False if session not found

    Side effects:
        - Updates payment_sessions_dir/{session_id}.json
    """
    config = getConfig()
    session_path = os.path.join(config["payment_sessions_dir"], f"{session_id}.json")

    if not os.path.exists(session_path):
        return False

    with open(session_path, "r") as f:
        session = json.load(f)

    session["status"] = "completed"
    session["facebook_response"] = facebook_response

    with open(session_path, "w") as f:
        json.dump(session, f, indent=2)

    return True


def update_user_status(
    user_id: str, session_id: str, conversation_id: str, name: Optional[str] = None
) -> None:
    """
    Add session and conversation to user's status file.

    Args:
        user_id: Facebook user ID
        session_id: 8-character hex string
        conversation_id: Facebook conversation ID
        name: Optional user name to store

    Side effects:
        - Creates users_dir/{user_id}/ if needed
        - Creates/updates users_dir/{user_id}/status.json
    """
    config = getConfig()
    user_dir = os.path.join(config["users_dir"], user_id)
    os.makedirs(user_dir, exist_ok=True)

    status_path = os.path.join(user_dir, "status.json")

    # Load existing status or create new
    if os.path.exists(status_path):
        with open(status_path, "r") as f:
            status = json.load(f)
    else:
        status = {"payment_sessions": [], "conversations": [], "payment": "pending"}

    # Add session and conversation if not already present
    if session_id not in status["payment_sessions"]:
        status["payment_sessions"].append(session_id)
    if conversation_id not in status["conversations"]:
        status["conversations"].append(conversation_id)

    # Set payment status if not present
    if "payment" not in status:
        status["payment"] = "pending"

    # Update name if provided
    if name is not None:
        status["name"] = name

    with open(status_path, "w") as f:
        json.dump(status, f, indent=2)


def mark_user_payment_completed(user_id: str) -> bool:
    """
    Mark a user's payment status as completed.

    Args:
        user_id: Facebook user ID

    Returns:
        True if successful, False if user status file doesn't exist

    Side effects:
        - Updates users_dir/{user_id}/status.json
    """
    config = getConfig()
    user_dir = os.path.join(config["users_dir"], user_id)
    status_path = os.path.join(user_dir, "status.json")

    if not os.path.exists(status_path):
        return False

    with open(status_path, "r") as f:
        status = json.load(f)

    status["payment"] = "completed"

    with open(status_path, "w") as f:
        json.dump(status, f, indent=2)

    return True


def update_conversation_status(
    conversation_id: str, session_id: Optional[str], participants: List[str]
) -> None:
    """
    Update conversation status with session and/or participants.

    Args:
        conversation_id: Facebook conversation ID
        session_id: 8-character hex string (can be None if just updating participants)
        participants: List of user IDs (excluding page_id)

    Side effects:
        - Creates conversations_dir/{conversation_id}/ if needed
        - Creates/updates conversations_dir/{conversation_id}/status.json
    """
    config = getConfig()
    conv_dir = os.path.join(config["conversations_dir"], conversation_id)
    os.makedirs(conv_dir, exist_ok=True)

    status_path = os.path.join(conv_dir, "status.json")

    # Load existing status or create new
    if os.path.exists(status_path):
        with open(status_path, "r") as f:
            status = json.load(f)
    else:
        status = {"payment_sessions": [], "participants": []}

    # Add session if provided
    if session_id is not None and session_id not in status["payment_sessions"]:
        status["payment_sessions"].append(session_id)

    # Update participants (replace with new list to handle changes)
    for participant in participants:
        if participant not in status["participants"]:
            status["participants"].append(participant)

    with open(status_path, "w") as f:
        json.dump(status, f, indent=2)


def cache_messages(conversation_id: str, messages: List[Dict]) -> None:
    """
    Cache messages to conversation directory.

    Args:
        conversation_id: Facebook conversation ID
        messages: List of message objects from Facebook API

    Side effects:
        - Creates conversations_dir/{conversation_id}/messages/ if needed
        - Writes message files with timestamp names
    """
    config = getConfig()
    messages_dir = os.path.join(
        config["conversations_dir"], conversation_id, "messages"
    )
    os.makedirs(messages_dir, exist_ok=True)

    for message in messages:
        # Parse created_time to get timestamp
        # Format: "2023-11-03T10:20:00+0000"
        created_time = datetime.strptime(message["created_time"], "%Y-%m-%dT%H:%M:%S%z")
        timestamp = int(created_time.timestamp())

        message_path = os.path.join(messages_dir, f"{timestamp}.json")
        with open(message_path, "w") as f:
            json.dump(message, f, indent=2)


def delete_user_data(user_id: str) -> None:
    """
    Delete all data for a user (for privacy compliance).

    Args:
        user_id: Facebook user ID

    Side effects:
        - Deletes users_dir/{user_id}/ directory
        - Deletes payment sessions belonging to this user
        - Deletes 1-1 conversations (where user is the only participant)
    """
    config = getConfig()
    user_dir = os.path.join(config["users_dir"], user_id)

    # Read user status first to get payment sessions and conversations
    status_path = os.path.join(user_dir, "status.json")
    payment_sessions = []
    conversations = []

    if os.path.exists(status_path):
        with open(status_path, "r") as f:
            status = json.load(f)
            payment_sessions = status.get("payment_sessions", [])
            conversations = status.get("conversations", [])

    # Delete user's payment sessions
    for session_id in payment_sessions:
        session_path = os.path.join(
            config["payment_sessions_dir"], f"{session_id}.json"
        )
        if os.path.exists(session_path):
            os.remove(session_path)

    # Delete 1-1 conversations (where this user is the only participant)
    for conversation_id in conversations:
        conv_dir = os.path.join(config["conversations_dir"], conversation_id)
        conv_status_path = os.path.join(conv_dir, "status.json")

        if os.path.exists(conv_status_path):
            with open(conv_status_path, "r") as f:
                conv_status = json.load(f)
                participants = conv_status.get("participants", [])

                # Only delete if this is a 1-1 conversation (one participant)
                if len(participants) == 1 and user_id in participants:
                    shutil.rmtree(conv_dir)

    # Delete user directory
    if os.path.exists(user_dir):
        shutil.rmtree(user_dir)
