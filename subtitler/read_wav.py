# read_wav.py
# Trevor Pottinger
# Sat May 23 22:44:46 PDT 2020

import argparse
import sys

import pandas as pd

from misc import dict2packed, readData


def main() -> None:
  parser = argparse.ArgumentParser('Print audio features in TSV')
  parser.add_argument('file', help='A video ID')
  parser.add_argument('--limit', type=float, help='The number of seconds ' +
                      'to take from the beginning of each file')
  args = parser.parse_args()
  # use pd.concat if this needs to read multiple files
  df = dict2packed(readData(
    'audios/%s.wav' % args.file,
    'tsvs/%s.tsv' % args.file,
    limit_seconds=args.limit
  ))
  df.to_csv(sys.stdout, sep='\t', index=False)
  return


if __name__ == '__main__':
  main()
