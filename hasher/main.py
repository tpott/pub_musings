# main.py

import argparse
import sys


# Other programs that generate large, random output:
# `python ../primes/gen_big.py 1000`

def main() -> None:
  parser = argparse.ArgumentParser(description='hash stuff')
  # func? md5/sha1/sha256/etc
  # hmac? set key? set phrase?
  # onion v3?
  args = parser.parse_args()
  print(args)
  sys.stdout.write('hello world')
  return


if __name__ == '__main__':
  main()
