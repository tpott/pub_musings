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
from webhooks import serve


seconds = float

t_id = NewType('t_id', str)
u_id = NewType('u_id', str)
conversation = NewType('conversation', Tuple[t_id, u_id])


SECONDS_IN_DAY = seconds(86400)


def getPageId(page_token_file: str) -> str:
    access_token = open(page_token_file).read().strip()
    resp = requests.get(f'https://graph.facebook.com/me?fields=id&access_token={access_token}')
    results = resp.json()
    if 'error' in results:
      print(results['error'])
      return None
    return results['id']


def getRecentConversations(
    page_id: str,
    page_token_file: str,
    last_run_file: str,
) -> List[conversation]:
    last_run_str = open(last_run_file).read().strip()
    last_run_time = 0
    if last_run_str != "":
        last_run_time = int(last_run_str)
    print(last_run_time)
    access_token = open(page_token_file).read().strip()
    resp = requests.get(f'https://graph.facebook.com/{page_id}/conversations?fields=participants,updated_time&access_token={access_token}')
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


def getRecentMessages(conversation_id: str, page_token_file: str) -> List[Dict[str, Any]]:
    access_token = open(page_token_file).read().strip()
    resp = requests.get(f'https://graph.facebook.com/{conversation_id}?fields=messages{{message,created_time,from}}&access_token={access_token}')
    results = resp.json()
    print(results)
    # TODO this doesn't filter nor sort for "recent"
    # TODO we aren't using the paging cursors
    # created_time like "2023-01-12T03:24:13+0000"
    return sorted(results['messages']['data'], key=lambda x: x['created_time'])


def postMessage(page_id, page_token_file, conv, resp) -> None:
    # Replace newlines and tabs because the graph API doesnt like them if
    # you try copy-pasting this printed URL. Rely on requests.post(json=...)
    # formatting, because it more reliably encodes the data correctly
    resp_text = resp.replace('\n', '\\n').replace('\t', '\\t').replace('"', '\\"')
    print(f'POST to https://graph.facebook.com/{page_id}/messages?recipient={{id:{conv[1]}}}&message={{text:"{resp_text}"}}&messaging_type=RESPONSE&access_token={open(page_token_file).read().strip()}')
    params = {
        'recipient': {'id': str(conv[1])},
        'message': {'text': resp},
        'messaging_type': 'RESPONSE',
        'access_token': open(page_token_file).read().strip(),
    }
    result = requests.post(f'https://graph.facebook.com/{page_id}/messages', json=params)
    print(result)
    if 400 <= result.status_code and result.status_code < 600:
        print(f'{result.status_code} error post to graph.facebook.com/{page_id}/messages')
        print(result.headers)


def runOnce() -> None:
    # get env vars
    last_run_file = os.environ.get('LAST_RUN_FILE')
    assert last_run_file is not None, "Missing env var: LAST_RUN_FILE"
    page_token_file = os.environ.get('PAGE_TOKEN_FILE')
    assert page_token_file is not None, "Missing env var: PAGE_TOKEN_FILE"
    page_id = getPageId(page_token_file)
    assert page_id is not None, "Failed to load page ID"

    # loop over recent conversations
    conversations = getRecentConversations(page_id, page_token_file, last_run_file)
    for conv in conversations:
        messages = getRecentMessages(conv[0], page_token_file)
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
        summary_obj = priceSummaries(context_messages[-1]['content'], model='gpt-4.1')
        if len(summary_obj.get('summaries', [])) == 0 and 'error' in summary_obj:
            postMessage(page_id, page_token_file, conv, summary_obj['error'])
            continue
        for summary in summary_obj['summaries']:
            target = summary['target']
            summary_text = summary['summary_text']
            url = summary['url']
            postMessage(page_id, page_token_file, conv, f"{target}\n{summary_text}\nURL: {url}")
        # end for loop over conversations
    open(last_run_file, 'w').write(str(int(time.time())))


def runLoop(sleep_time: seconds) -> None:
    while True:
        runOnce()
        time.sleep(sleep_time)


def main() -> None:
    cert_path = os.environ.get('WEBHOOK_CERT_FILE')
    privkey_path = os.environ.get('WEBHOOK_KEY_FILE')
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
