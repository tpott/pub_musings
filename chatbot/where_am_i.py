# where_am_i.py

import argparse
import getpass
import json
import os
from typing import (Dict, List)

from mealprep import chatCompletitions


def main() -> None:
  # What's the difference between identity match and authenticating someone over chat?
  parser = argparse.ArgumentParser(description="Where am I?")
  parser.add_argument("location_file", help="Path to locations, descriptions, and addresses. START HERE!")
  args = parser.parse_args()

  if not os.path.isfile(args.location_file):
    print("no location_file file detected")
    return

  while True:
    visuals = input('What do you see? ')
    print(visuals)
    resp = chatCompletitions([
      {"role": "system", "content": "Do you think the following sounds like one of the following locations? Please only respond with \"yes\" or \"no\"."},
      {"role": "user", "content": open(args.location_file).read()},
      {"role": "user", "content": visuals},
    ])
    print(resp)

    if resp.lower() != "yes":
      continue

    print(chatCompletitions([
      {"role": "system", "content": "Which of the following locations do you think I'm describing? Please respond with just the locations name."},
      {"role": "user", "content": open(args.location_file).read()},
      {"role": "user", "content": visuals},
    ]))
    break


if __name__ == '__main__':
    main()
