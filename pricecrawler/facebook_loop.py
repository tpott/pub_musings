# facebook_loop.py

import asyncio
from datetime import datetime, timezone
import json
import os
import time
import traceback
from typing import Any, Dict, List, NewType, Tuple, Optional

import aiohttp

from pricechecker import price_summaries
from serve_config import getConfig, saveConfig
from session_storage import (
    create_payment_session,
    update_conversation_status,
    cache_messages,
)


seconds = float

t_id = NewType("t_id", str)
u_id = NewType("u_id", str)
conversation = NewType("conversation", Tuple[t_id, u_id])


SECONDS_IN_DAY = seconds(86400)


async def get_my_id(page_token: str) -> Optional[str]:
    """Get the page ID from Facebook Graph API."""
    async with aiohttp.ClientSession() as session:
        async with session.get(
            f"https://graph.facebook.com/me?metadata=true&access_token={page_token}"
        ) as resp:
            results = await resp.json()
            if "error" in results:
                print(results["error"])
                return None
            metadata = results.get("metadata", {})
            if "type" not in metadata:
                print(f"Results didnt contain metadata, keys={list(results.keys())}")
                return None
            if metadata.get("type") != "page":
                print(f"Expected page type, got: {metadata.get('type')}")
                return None
            return results["id"]


async def check_token_expiry(page_token: str, app_id: str, app_secret: str) -> int:
    """Check when a Facebook page token expires."""
    async with aiohttp.ClientSession() as session:
        async with session.get(
            f"https://graph.facebook.com/debug_token?input_token={page_token}&access_token={app_id}|{app_secret}"
        ) as resp:
            results = await resp.json()
            if "error" in results:
                print(results["error"])
                return 0
            data = results.get("data", {})
            expires_at = data.get("expires_at", 0)
            return expires_at


async def maybe_refresh_to_non_expiring_token(
    app_id: str, app_secret: str, page_token: str, token_expiry_cache: Dict[str, int]
) -> str:
    """Refresh an expiring Facebook token to a non-expiring one."""
    # Check cache first
    if page_token in token_expiry_cache:
        cached_expiry = token_expiry_cache[page_token]
        if cached_expiry == 0:
            print("Token already non-expiring (cached)")
            return page_token

    # Check if token expires, if not return early
    expires_at = await check_token_expiry(page_token, app_id, app_secret)

    # Cache the expiry time
    token_expiry_cache[page_token] = expires_at

    if expires_at == 0:
        print("Token already non-expiring")
        return page_token

    print(f"Token expires at {expires_at}, refreshing to non-expiring token")
    async with aiohttp.ClientSession() as session:
        async with session.get(
            f"https://graph.facebook.com/oauth/access_token?grant_type=fb_exchange_token&client_id={app_id}&client_secret={app_secret}&fb_exchange_token={page_token}"
        ) as resp:
            results = await resp.json()
            if "error" in results:
                print(results["error"])
                return page_token

            new_access_token = results.get("access_token")
            if new_access_token is None:
                print("No access_token in refresh response")
                return page_token

            # Cache the new token as non-expiring
            token_expiry_cache[new_access_token] = 0

            return new_access_token


async def get_recent_conversations(
    page_id: str,
    page_token: str,
    last_run_time: int,
) -> List[Dict[str, Any]]:
    """Get recent conversations from Facebook Graph API."""
    print(f"last_run_time = {last_run_time}")
    async with aiohttp.ClientSession() as session:
        async with session.get(
            f"https://graph.facebook.com/{page_id}/conversations?fields=participants,updated_time&access_token={page_token}"
        ) as resp:
            results = await resp.json()
            print(results)
            filtered = []
            if "data" not in results:
                print(f"No conversations at all for page {page_id}")
                return []
            conversations = results["data"]
            for res in conversations:
                # TODO taking `[0]` assumes conversations are always 1-1
                # 'participants': {'data': [{'name': 'Trevor Pottinger', 'email': '6397427050373172@facebook.com', 'id': '6397427050373172'}, {'name': 'Agent Dale Cooper', 'email': '108420048810326@facebook.com', 'id': '108420048810326'}]}
                other_participant = list(
                    filter(lambda x: x["id"] != page_id, res["participants"]["data"])
                )[0]
                res["other"] = other_participant["id"]
                res["other_name"] = other_participant.get("name")
                # updated_time like "2023-07-21T06:15:35+0000"
                updated_time = (
                    datetime.strptime(res["updated_time"], "%Y-%m-%dT%H:%M:%S%z")
                    .astimezone(timezone.utc)
                    .timestamp()
                )
                if last_run_time - updated_time < SECONDS_IN_DAY:
                    filtered.append(res)
            # TODO we aren't using the paging cursors
            print(
                f"returning recent conversations {len(filtered)} / {len(conversations)}"
            )
            return list(
                map(
                    lambda x: {
                        "conversation_id": x["id"],
                        "user_id": x["other"],
                        "user_name": x["other_name"],
                        "participants": x["participants"]["data"],
                    },
                    filtered,
                )
            )


async def get_recent_messages(
    conversation_id: str, page_token: str
) -> List[Dict[str, Any]]:
    """Get recent messages from a conversation."""
    async with aiohttp.ClientSession() as session:
        async with session.get(
            f"https://graph.facebook.com/{conversation_id}?fields=messages{{message,created_time,from}}&access_token={page_token}"
        ) as resp:
            results = await resp.json()
            print(results)
            # TODO this doesn't filter nor sort for "recent"
            # TODO we aren't using the paging cursors
            # created_time like "2023-01-12T03:24:13+0000"
            messages = sorted(
                results["messages"]["data"], key=lambda x: x["created_time"]
            )

            # Cache messages
            cache_messages(conversation_id, messages)

            return messages


async def post_message(
    page_id: str, page_token: str, conv: Tuple[str, str], resp: str
) -> None:
    """Post a message to a Facebook conversation."""
    # Replace newlines and tabs because the graph API doesnt like them if
    # you try copy-pasting this printed URL. Rely on aiohttp post(json=...)
    # formatting, because it more reliably encodes the data correctly
    resp_text = resp.replace("\n", "\\n").replace("\t", "\\t").replace('"', '\\"')
    print(
        f'POST to https://graph.facebook.com/{page_id}/messages?recipient={{id:{conv[1]}}}&message={{text:"{resp_text}"}}&messaging_type=RESPONSE&access_token={page_token}'
    )
    params = {
        "recipient": {"id": str(conv[1])},
        "message": {"text": resp},
        "messaging_type": "RESPONSE",
        "access_token": page_token,
    }
    async with aiohttp.ClientSession() as session:
        async with session.post(
            f"https://graph.facebook.com/{page_id}/messages", json=params
        ) as result:
            print(result)
            if 400 <= result.status and result.status < 600:
                print(
                    f"{result.status} error post to graph.facebook.com/{page_id}/messages"
                )
                print(result.headers)


async def run_once(token_expiry_cache: Optional[Dict[str, int]] = None) -> None:
    """Run one polling cycle to check for new Facebook messages."""
    if token_expiry_cache is None:
        token_expiry_cache = {}
    # Get config
    config = getConfig()
    app_id = config["facebook_app_id"]
    app_secret = config["facebook_app_secret"]
    webhook_hostname = config["webhook_hostname"]

    # Loop through all pages
    for page_config in config["pages"]:
        page_id = page_config["page_id"]
        page_token = page_config["page_token"]
        last_run_time = page_config["last_run"]

        print(f"Processing page {page_id}")

        # Refresh to non-expiring token if needed
        new_token = await maybe_refresh_to_non_expiring_token(
            app_id, app_secret, page_token, token_expiry_cache
        )
        if new_token != page_token:
            print(f"Token was refreshed for page {page_id}")
            page_config["page_token"] = new_token
            page_token = new_token

        # Loop over recent conversations
        conversations = await get_recent_conversations(
            page_id, page_token, last_run_time
        )
        for conv in conversations:
            conversation_id = conv["conversation_id"]
            user_id = conv["user_id"]
            user_name = conv["user_name"]

            messages = await get_recent_messages(conversation_id, page_token)

            # skip if the last message was from the bot
            if messages[-1]["from"]["id"] == page_id:
                print(f"skipping conversation t_id {conversation_id} with {user_id}")
                continue

            # Update conversation participants with user name
            participants = [user_id]
            update_conversation_status(conversation_id, None, participants)

            # Check payment status for all participants
            unpaid_users = []
            for participant in conv["participants"]:
                participant_id = participant["id"]
                # Skip the page itself
                if participant_id == page_id:
                    continue

                # Check user status
                user_dir = os.path.join(config["users_dir"], participant_id)
                status_path = os.path.join(user_dir, "status.json")

                if os.path.exists(status_path):
                    with open(status_path, "r") as f:
                        user_status = json.load(f)
                        payment_status = user_status.get("payment", "pending")
                        if payment_status != "completed":
                            unpaid_users.append(
                                {
                                    "id": participant_id,
                                    "name": participant.get("name", participant_id),
                                }
                            )
                else:
                    # No status file means no payment
                    unpaid_users.append(
                        {
                            "id": participant_id,
                            "name": participant.get("name", participant_id),
                        }
                    )

            # If any user hasn't paid, post payment message
            if len(unpaid_users) > 0:
                for unpaid_user in unpaid_users:
                    session_id = create_payment_session(
                        unpaid_user["id"], conversation_id, unpaid_user["name"]
                    )
                    payment_url = f"https://{webhook_hostname}/pay?r={session_id}"
                    await post_message(
                        page_id,
                        page_token,
                        (conversation_id, user_id),
                        f"@{unpaid_user['name']} in order to use price checker, please complete payment at {payment_url}",
                    )
                continue

            # construct our messages for calling openai for chatgpt
            context_messages = []
            for msg in messages:
                if msg["from"]["id"] == page_id:
                    context_messages.append(
                        {"role": "assistant", "content": msg["message"]}
                    )
                else:
                    context_messages.append({"role": "user", "content": msg["message"]})
                print(context_messages[-1])
                # end for loop over messages

            # call openai and post the message it generates
            # TODO utilize more of historical message context
            summary_obj = await price_summaries(
                context_messages[-1]["content"], model="gpt-5"
            )
            if "error" in summary_obj:
                await post_message(
                    page_id,
                    page_token,
                    (conversation_id, user_id),
                    summary_obj["error"],
                )
                continue
            for summary in summary_obj["summaries"]:
                target = summary["target"]
                url = summary["url"]
                # some targets don't have <div> element attributes configured
                if "summary_text" not in summary:
                    await post_message(
                        page_id,
                        page_token,
                        (conversation_id, user_id),
                        f"{target}\nURL: {url}",
                    )
                    continue
                summary_text = summary["summary_text"]
                await post_message(
                    page_id,
                    page_token,
                    (conversation_id, user_id),
                    f"{target}\n{summary_text}\nURL: {url}",
                )
            # end for loop over conversations

        # Update last_run for this page
        page_config["last_run"] = int(time.time())

    # Save updated config
    saveConfig(config)


async def polling_loop() -> None:
    """Background task that polls Facebook every 15 seconds."""
    token_expiry_cache: Dict[str, int] = {}
    while True:
        try:
            await run_once(token_expiry_cache)
            await asyncio.sleep(15.0)
        except Exception as e:
            print(f"Error in polling loop: {e}")
            traceback.print_exc()
            await asyncio.sleep(15.0)  # Continue despite errors
