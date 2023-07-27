# facebook_loop.py

from datetime import datetime, timezone
import json
import os
import time
from typing import (Any, Dict, List, NewType)

import requests

from chatgpt import chatCompletitions


seconds = float

t_id = NewType('t_id', str)


SECONDS_IN_DAY = seconds(86400)


def getRecentConversations(
  page_id: str,
  page_token_file: str,
  last_run_file: str,
) -> List[t_id]:
  last_run_time = int(open(last_run_file).read())
  print(last_run_time)
  access_token = open(page_token_file).read()
  resp = requests.get(f'https://graph.facebook.com/{page_id}/conversations?fields=participants,updated_time&access_token={access_token}')
  results = resp.json()
  print(results)
  filtered = []
  for res in results['data']:
    # updated_time like "2023-07-21T06:15:35+0000"
    updated_time = datetime.strptime(res['updated_time'], '%Y-%m-%dT%H:%M:%S%z').astimezone(timezone.utc).timestamp()
    if last_run_time - updated_time < SECONDS_IN_DAY:
      filtered.append(res)
  # TODO we aren't using the paging cursors
  print(f'returning {len(filtered)} recent conversations')
  return list(map(lambda x: x['id'], filtered))


def getRecentMessages(conversation_id: str, page_token_file: str) -> List[Dict[str, Any]]:
  access_token = open(page_token_file).read()
  resp = requests.get(f'https://graph.facebook.com/{conversation_id}?fields=messages{{message,created_time,from}}&access_token={access_token}')
  results = resp.json()
  print(results)
  # TODO this doesn't filter nor sort for "recent"
  # TODO we aren't using the paging cursors
  # created_time like "2023-01-12T03:24:13+0000"
  return sorted(results['messages']['data'], key=lambda x: x['created_time'])

def loop() -> None:
  last_run_file = os.environ.get('LAST_RUN_FILE')
  page_id = os.environ.get('PAGE_ID')
  page_token_file = os.environ.get('PAGE_TOKEN_FILE')
  while True:
    conversation_ids = getRecentConversations(page_id, page_token_file, last_run_file)
    for tid in conversation_ids:
      messages = getRecentMessages(tid, page_token_file)
      # if messages[-1]['from']['id'] == page_id: continue # skip if the last message was from the bot
      context_messages = [{"role": "system", "content": "You are Agent Dale Cooper, from hit TV series Twin Peaks. You are a lover of damn fine coffee. You are not a fan of Bob."}]
      for msg in messages:
        if msg['from']['id'] == page_id:
          context_messages.append({"role": "assistant", "content": msg['message']})
        else:
          context_messages.append({"role": "user", "content": msg['message']})
        print(context_messages[-1])
      chatCompletitions(context_messages)
      # TODO POST /{page_id}/messages?recipient={id:6397427050373172}&message={text:'You did it!'}&messaging_type=RESPONSE&acess_token=$page_token
      # end for loop over conversation_ids
    open(last_run_file, 'w').write(str(int(time.time())))
    break # TODO remove
    time.sleep(seconds(5))


def main() -> None:
  loop()


if __name__ == '__main__':
  main()
