# train.py
# Trevor Pottinger
# Thu May 21 22:22:07 PDT 2020

import argparse
import os

import pandas as pd

from misc import dict2packed, readData, trainModel, urandom5


def main() -> None:
  parser = argparse.ArgumentParser('Train an audio model to detect speech')
  parser.add_argument('files', help='A comma separated list of audio IDs')
  parser.add_argument('--limit', type=float, help='The number of seconds ' +
                      'to take from the beginning of each file')
  parser.add_argument('--n_iter', type=int, default=10, help='The number of ' +
                      'iterations that RandomizedSearchCV should use')
  args = parser.parse_args()
  train_files = args.files.split(',')
  def _readWrapper(in_file: str) -> pd.DataFrame:
    tsv_file = 'tsvs/labeled_%s.tsv' % in_file
    if not os.path.isfile(tsv):
      tsv_file = 'tsvs/aws_%s.tsv' % in_file
    return dict2packed(readData(
      'audios/%s/vocals_left.wav' % in_file,
      tsv_file,
      limit_seconds=args.limit
    ))
  df = pd.concat(list(map(_readWrapper, train_files)))
  model_file = trainModel(df, 'models/%s' % urandom5(), n_iter=args.n_iter)
  print('Saving model file to %s' % model_file)
  return


if __name__ == '__main__':
  main()
