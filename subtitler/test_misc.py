# test_misc.py
# Trevor Pottinger
# Mon May 18 20:32:40 PDT 2020

import unittest

import pandas as pd

from misc import dict2packed, readData


class TestMisc(unittest.TestCase):
  def test_reading(self):
    df = dict2packed(readData(
      'audios/NrgmdOz227I.wav',
      'tsvs/NrgmdOz227I.tsv',
      200
    ))
    ndf = pd.DataFrame(data={
      'is_talking_now': df.is_talking[2:].to_numpy(),
      'was_talking': df.is_talking[1:-1].to_numpy(),
      'was_was_talking': df.is_talking[0:-2].to_numpy(),
    })
    counts = ndf.groupby([
      'is_talking_now',
      'was_talking',
      'was_was_talking',
    ]).size().to_frame('num_frames')
    # TODO this shouldn't require a sum(), these should both be scalars
    assert counts.loc[1, 1, 1].sum() > counts.loc[1, 0, 0].sum()

  # TODO test the following
  # >>> df = pd.concat([dict2packed(readData('audios/%s.wav' % in_file, 'tsvs/%s.tsv' % in_file, 180)) for in_file in ['NrgmdOz227I']])
  # >>> model_file = trainModel(df)
  # >>> evalModel(model_file, ['audios/W15KUnxvZ7A.wav'])


if __name__ == '__main__':
  unittest.main()
