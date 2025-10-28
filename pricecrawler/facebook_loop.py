# facebook_loop.py

from datetime import datetime, timezone
import json
import os
import threading
import time
from typing import (Any, Dict, List, NewType, Tuple)

import requests

from chatgpt import chatCompletitions
from pricechecker import priceSummaries
from serve_config import getConfig, saveConfig
from webhooks import serve


seconds = float

t_id = NewType('t_id', str)
u_id = NewType('u_id', str)
conversation = NewType('conversation', Tuple[t_id, u_id])


SECONDS_IN_DAY = seconds(86400)


def getMyId(page_token: str) -> str:
    resp = requests.get(f'https://graph.facebook.com/me?metadata=true&access_token={page_token}')
    results = resp.json()
    if 'error' in results:
      print(results['error'])
      return None
    metadata = results.get('metadata', {})
    if 'type' not in metadata:
      print(f"Results didnt contain metadata, keys={list(results.keys())}")
      return None
    if metadata.get('type') != 'page':
      print(f"Expected page type, got: {metadata.get('type')}")
      return None
    return results['id']


def checkTokenExpiry(page_token: str, app_id: str, app_secret: str) -> int:
    resp = requests.get(f'https://graph.facebook.com/debug_token?input_token={page_token}&access_token={app_id}|{app_secret}')
    results = resp.json()
    if 'error' in results:
        print(results['error'])
        return 0
    data = results.get('data', {})
    expires_at = data.get('expires_at', 0)
    return expires_at


def maybeRefreshToNonExpiringToken(app_id: str, app_secret: str, page_token: str) -> str:
    # Check if token expires, if not return early
    expires_at = checkTokenExpiry(page_token, app_id, app_secret)
    if expires_at == 0:
        print("Token already non-expiring")
        return page_token

    print(f"Token expires at {expires_at}, refreshing to non-expiring token")
    resp = requests.get(f'https://graph.facebook.com/oauth/access_token?grant_type=fb_exchange_token&client_id={app_id}&client_secret={app_secret}&fb_exchange_token={page_token}')
    results = resp.json()
    if 'error' in results:
        print(results['error'])
        return page_token

    new_access_token = results.get('access_token')
    if new_access_token is None:
        print("No access_token in refresh response")
        return page_token

    return new_access_token


def getRecentConversations(
    page_id: str,
    page_token: str,
    last_run_time: int,
) -> List[conversation]:
    print(f"last_run_time = {last_run_time}")
    resp = requests.get(f'https://graph.facebook.com/{page_id}/conversations?fields=participants,updated_time&access_token={page_token}')
    results = resp.json()
    print(results)
    filtered = []
    for res in results['data']:
        # 'participants': {'data': [{'name': 'Trevor Pottinger', 'email': '6397427050373172@facebook.com', 'id': '6397427050373172'}, {'name': 'Agent Dale Cooper', 'email': '108420048810326@facebook.com', 'id': '108420048810326'}]}
        res['other'] = list(filter(lambda x: x['id'] != page_id, res['participants']['data']))[0]['id']
        # updated_time like "2023-07-21T06:15:35+0000"
        updated_time = datetime.strptime(res['updated_time'], '%Y-%m-%dT%H:%M:%S%z').astimezone(timezone.utc).timestamp()
        if last_run_time - updated_time < SECONDS_IN_DAY:
            filtered.append(res)
    # TODO we aren't using the paging cursors
    print(f'returning {len(filtered)} recent conversations')
    return list(map(lambda x: conversation((t_id(x['id']), u_id(x['other']))), filtered))


def getRecentMessages(conversation_id: str, page_token: str) -> List[Dict[str, Any]]:
    resp = requests.get(f'https://graph.facebook.com/{conversation_id}?fields=messages{{message,created_time,from}}&access_token={page_token}')
    results = resp.json()
    print(results)
    # TODO this doesn't filter nor sort for "recent"
    # TODO we aren't using the paging cursors
    # created_time like "2023-01-12T03:24:13+0000"
    return sorted(results['messages']['data'], key=lambda x: x['created_time'])


def postMessage(page_id, page_token, conv, resp) -> None:
    # Replace newlines and tabs because the graph API doesnt like them if
    # you try copy-pasting this printed URL. Rely on requests.post(json=...)
    # formatting, because it more reliably encodes the data correctly
    resp_text = resp.replace('\n', '\\n').replace('\t', '\\t').replace('"', '\\"')
    print(f'POST to https://graph.facebook.com/{page_id}/messages?recipient={{id:{conv[1]}}}&message={{text:"{resp_text}"}}&messaging_type=RESPONSE&access_token={page_token}')
    params = {
        'recipient': {'id': str(conv[1])},
        'message': {'text': resp},
        'messaging_type': 'RESPONSE',
        'access_token': page_token,
    }
    result = requests.post(f'https://graph.facebook.com/{page_id}/messages', json=params)
    print(result)
    if 400 <= result.status_code and result.status_code < 600:
        print(f'{result.status_code} error post to graph.facebook.com/{page_id}/messages')
        print(result.headers)


def runOnce() -> None:
    # Get config
    config = getConfig()
    app_id = config['facebook_app_id']
    app_secret = config['facebook_app_secret']

    # Loop through all pages
    for page_config in config['pages']:
        page_id = page_config['page_id']
        page_token = page_config['page_token']
        last_run_time = page_config['last_run']

        print(f"Processing page {page_id}")

        # Refresh to non-expiring token if needed
        new_token = maybeRefreshToNonExpiringToken(app_id, app_secret, page_token)
        if new_token != page_token:
            print(f"Token was refreshed for page {page_id}")
            page_config['page_token'] = new_token
            page_token = new_token

        # Loop over recent conversations
        conversations = getRecentConversations(page_id, page_token, last_run_time)
        for conv in conversations:
            messages = getRecentMessages(conv[0], page_token)
            # skip if the last message was from the bot
            if messages[-1]['from']['id'] == page_id:
                print(f'skipping conversation t_id {conv[0]} with {conv[1]}')
                continue

            # construct our messages for calling openai for chatgpt
            context_messages = []
            for msg in messages:
                if msg['from']['id'] == page_id:
                    context_messages.append({"role": "assistant", "content": msg['message']})
                else:
                    context_messages.append({"role": "user", "content": msg['message']})
                print(context_messages[-1])
                # end for loop over messages

            # call openai and post the message it generates
            # TODO utilize more of historical message context
            summary_obj = priceSummaries(context_messages[-1]['content'], model='gpt-5')
            if 'error' in summary_obj:
                postMessage(page_id, page_token, conv, summary_obj['error'])
                continue
            for summary in summary_obj['summaries']:
                target = summary['target']
                url = summary['url']
                # some targets don't have <div> element attributes configured
                if 'summary_text' not in summary:
                    postMessage(page_id, page_token, conv, f"{target}\nURL: {url}")
                    continue
                summary_text = summary['summary_text']
                postMessage(page_id, page_token, conv, f"{target}\n{summary_text}\nURL: {url}")
            # end for loop over conversations

        # Update last_run for this page
        page_config['last_run'] = int(time.time())

    # Save updated config
    saveConfig(config)


def runLoop(sleep_time: seconds) -> None:
    while True:
        runOnce()
        time.sleep(sleep_time)


def main() -> None:
    config = getConfig()
    cert_path = config.get('webhook_cert_file')
    privkey_path = config.get('webhook_key_file')
    if cert_path is not None or privkey_path is not None:
        runOnce()
        print('will now serve webhooks from port 8443 and run loop every 15 seconds in parallel')

        # Start the loop thread
        loop_thread = threading.Thread(target=runLoop, args=(seconds(15),), daemon=True)
        loop_thread.start()

        # Run the server in the main thread
        serve('0.0.0.0', 8443, cert_path, privkey_path, runOnce)
    else:
        print('will run in a loop, every 15 seconds')
        runLoop(seconds(15))


if __name__ == '__main__':
    main()
