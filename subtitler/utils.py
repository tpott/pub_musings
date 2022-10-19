# utils.py

import os

DIRS = [
  'data/audios',
  'data/downloads',
  'data/final',
  'data/lyrics',
  'data/models',
  'data/outputs',
  'data/subtitles',
  'data/tmp',
  'data/tsvs',
  'data/video_ids',
  'data/video_names',
]


def createDirs() -> None:
  for d in DIRS:
    try:
      os.mkdir(d)
    except FileExistsError:
      continue
  return


if __name__ == '__main__':
  createDirs()
