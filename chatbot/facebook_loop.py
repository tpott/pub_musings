# facebook_loop.py

from datetime import datetime, timezone
import json
import os
import time
from typing import (Any, Dict, List, NewType, Tuple)

import requests

from chatgpt import chatCompletitions
from webhooks import serve


seconds = float

t_id = NewType('t_id', str)
conversation = NewType('conversation', Tuple[t_id, str])


SECONDS_IN_DAY = seconds(86400)


def getRecentConversations(
  page_id: str,
  page_token_file: str,
  last_run_file: str,
) -> List[conversation]:
  last_run_time = int(open(last_run_file).read())
  print(last_run_time)
  access_token = open(page_token_file).read()
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
  return list(map(lambda x: (x['id'], x['other']), filtered))


def getRecentMessages(conversation_id: str, page_token_file: str) -> List[Dict[str, Any]]:
  access_token = open(page_token_file).read()
  resp = requests.get(f'https://graph.facebook.com/{conversation_id}?fields=messages{{message,created_time,from}}&access_token={access_token}')
  results = resp.json()
  print(results)
  # TODO this doesn't filter nor sort for "recent"
  # TODO we aren't using the paging cursors
  # created_time like "2023-01-12T03:24:13+0000"
  return sorted(results['messages']['data'], key=lambda x: x['created_time'])


def runOnce() -> None:
  # get env vars
  last_run_file = os.environ.get('LAST_RUN_FILE')
  page_id = os.environ.get('PAGE_ID')
  page_token_file = os.environ.get('PAGE_TOKEN_FILE')

  # loop over recent conversations
  conversations = getRecentConversations(page_id, page_token_file, last_run_file)
  for conv in conversations:
    messages = getRecentMessages(conv[0], page_token_file)
    # skip if the last message was from the bot
    if messages[-1]['from']['id'] == page_id:
      print(f'skipping conversation t_id {conv[0]} with {conv[1]}')
      continue 

    # construct our messages for calling openai for chatgpt
    context_messages = [{"role": "system", "content": "You are Agent Dale Cooper, from hit TV series Twin Peaks. You are a lover of damn fine coffee. You are not a fan of Bob. You will respond in character, as Dale Cooper would. You will help others on their scavenger hunt around Seattle. You will keep responses less than 200 characters."}]
    for msg in messages:
      if msg['from']['id'] == page_id:
        context_messages.append({"role": "assistant", "content": msg['message']})
      else:
        context_messages.append({"role": "user", "content": msg['message']})
      print(context_messages[-1])
      # end for loop over messages

    # call openai and post the message it generates
    resp = chatCompletitions(context_messages)
    result = requests.post(f'https://graph.facebook.com/{page_id}/messages?recipient={{id:{conv[1]}}}&message={{text:"{resp}"}}&messaging_type=RESPONSE&access_token={open(page_token_file).read()}')
    print(result)
    if 400 <= result.status_code and result.status_code < 600:
      print(f'{result.status_code} error post to graph.facebook.com/{page_id}/messages')
      print(result.headers)
    # end for loop over conversations
  open(last_run_file, 'w').write(str(int(time.time())))


def runLoop(sleep_time: seconds) -> None:
  while True:
    runOnce()
    break # TODO remove
    time.sleep(sleep_time)


def main() -> None:
  cert_path = os.environ.get('WEBHOOK_CERT_FILE')
  privkey_path = os.environ.get('WEBHOOK_KEY_FILE')
  if cert_path is not None or privkey_path is not None:
    runOnce()
    serve('0.0.0.0', 8443, cert_path, privkey_path, runOnce)
    # TODO async call loop with a 1 min sleep, just as a fallback
    # in case webhooks stop working
  else:
    runLoop(seconds(5))


if __name__ == '__main__':
  main()
