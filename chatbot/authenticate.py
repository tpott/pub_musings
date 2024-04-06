# authenticate.py

import argparse
import getpass
import json
import os
from typing import (Dict, List)

from mealprep import chatCompletitions


def main() -> None:
  # What's the difference between identity match and authenticating someone over chat?
  parser = argparse.ArgumentParser(description="Authenticate an identity")
  parser.add_argument("identity_file", help="Path to identity description. START HERE!")
  args = parser.parse_args()

  if not os.path.isfile(args.identity_file):
    print("no identity_file file detected")
    return

  who = input('Who are you? ')
  print(who)
  # print(chatCompletitions([
	# {"role": "user", "content": f"Hello, I am: "{open(args.identity_file).read()}"."},
    # {"role": "system", "content": "Can you tell"},
    # {"role": "user", "content": getpass.getuser()},
  # ]))


if __name__ == '__main__':
    main()
