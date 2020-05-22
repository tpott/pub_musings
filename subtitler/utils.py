# utils.py

import os

DIRS = [
  'audios',
  'downloads',
  'final',
  'models',
  'outputs',
  'subtitles',
  'tmp',
  'tsvs',
]


def createDirs() -> None:
  for d in DIRS:
    try:
      os.mkdir(d)
    except os.exists:
      continue
  return


if __name__ == '__main__':
  createDirs()
