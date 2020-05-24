# utils.py

import os

DIRS = [
  'audios',
  'downloads',
  'final',
  'lyrics',
  'models',
  'outputs',
  'subtitles',
  'tmp',
  'tsvs',
  'video_ids',
  'video_names',
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
