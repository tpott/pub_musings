# facebook_loop.py

import json
import os
import time
from typing import (Any, Dict, List)

import requests

def getRecentConversations(
  page_id: str,
  page_token_file: str,
  last_run_file: str,
) -> List[str]:
  last_run_time = int(open(last_run_file).read())
  print(last_run_time)
  access_token = open(page_token_file).read()
  resp = requests.get(f'https://graph.facebook.com/{page_id}/conversations?fields=participants,updated_time&access_token={access_token}')
  results = resp.json()
  print(results)
  # TODO this doesn't filter nor sort for "recent"
  # TODO we aren't using the paging cursors
  # updated_time like "2023-07-21T06:15:35+0000"
  return list(map(lambda x: x['id'], results['data']))


def getRecentMessages(conversation_id: str, page_token_file: str) -> List[Dict[str, Any]]:
  access_token = open(page_token_file).read()
  resp = requests.get(f'https://graph.facebook.com/{conversation_id}?fields=messages{{message,created_time,from}}&access_token={access_token}')
  results = resp.json()
  print(results)
  # TODO this doesn't filter nor sort for "recent"
  # TODO we aren't using the paging cursors
  # created_time like "2023-01-12T03:24:13+0000"
  return results['messages']['data']

def loop() -> None:
  last_run_file = os.environ.get('LAST_RUN_FILE')
  page_id = os.environ.get('PAGE_ID')
  page_token_file = os.environ.get('PAGE_TOKEN_FILE')
  while True:
    conversation_ids = getRecentConversations(page_id, page_token_file, last_run_file)
    for t_id in conversation_ids:
      messages = getRecentMessages(t_id, page_token_file)
    open(last_run_file, 'w').write(str(int(time.time())))
    break # TODO remove
    time.sleep(1)


def main() -> None:
  loop()


if __name__ == '__main__':
  main()
