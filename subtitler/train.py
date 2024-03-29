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
                      'iterations that RandomizedSearchCV should use. ' +
                      'Default is 10')
  args = parser.parse_args()

  def _labelSelector(in_file: str) -> str:
    tsv_file = 'data/tsvs/labeled_%s.tsv' % in_file
    if not os.path.isfile(tsv_file):
      tsv_file = 'data/tsvs/aws_%s.tsv' % in_file
    return tsv_file
  def _readWrapper(in_file: str) -> pd.DataFrame:
    return dict2packed(readData(
      'data/audios/%s/vocals_left.wav' % in_file,
      _labelSelector(in_file),
      limit_seconds=args.limit
    ))

  train_files = args.files.split(',')
  df = pd.concat(list(map(_readWrapper, train_files)))
  label_files = list(map(_labelSelector, train_files))
  audio_files = list(map(lambda f: 'data/audios/%s/vocals_left.wav' % f, train_files))

  # TODO seed rand_int
  scorer_file, model_file = trainModel(
    df,
    'data/models/%s' % urandom5(),
    'data/models/%s' % urandom5(),
    audio_files,
    label_files,
    n_iter=args.n_iter
  )
  print('Saving scorer file to %s' % scorer_file)
  print('Saving model file to %s' % model_file)
  return


if __name__ == '__main__':
  main()
