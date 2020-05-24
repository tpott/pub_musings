# eval.py
# Trevor Pottinger
# Thu May 21 22:33:15 PDT 2020

import argparse
import sys

import pandas as pd

from misc import evalModel


def main() -> None:
  parser = argparse.ArgumentParser('Train an audio model to detect speech')
  parser.add_argument('model_file', help='The file name of the model')
  parser.add_argument('files', help='A comma separated list of audio IDs')
  parser.add_argument('--limit', type=float, help='The number of seconds ' +
                      'to take from the beginning of each file')
  parser.add_argument('--test', action='store_true', help='Prints perf')
  args = parser.parse_args()
  eval_files = args.files.split(',')
  tsv_files = [None] * len(eval_files)
  if args.test:
    tsv_files = list(map(lambda vid: 'tsvs/%s.tsv' % vid, eval_files))
  eval_df, predictions = evalModel(
    args.model_file,
    list(map(lambda vid: 'audios/%s.wav' % vid, eval_files)),
    tsv_files,
    limit_seconds=args.limit
  )
  # is_talking.astype(str) is so we include NaNs in the groupby
  comparison = pd.concat([
    eval_df.is_talking.astype(str),
    pd.Series(predictions, name='predicted_talking'),
  ], axis=1)
  stats = comparison.groupby([
    'is_talking',
    'predicted_talking',
  ]).size().to_frame('num_windows')
  stats.to_csv(sys.stderr, sep='\t')
  return


if __name__ == '__main__':
  main()
