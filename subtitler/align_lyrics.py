# align_lyrics.py
# Trevor Pottinger
# Sun Jun  7 23:06:18 PDT 2020

import argparse
from typing import Any, List, Tuple


Utterance = Tuple[float, float, float, str]


# TODO replace Any with Generic[T]
def transpose(rows: List[List[Any]]) -> List[List[Any]]:
  """Converts rows to cols or vice versa"""
  nrows = len(rows)
  assert nrows > 0, "Expected at least one row in input"
  ncols = len(rows[0])
  cols = [[None for _ in range(nrows)] for _ in range(ncols)]
  for i in range(nrows):
    for j in range(ncols):
      if j > len(rows[i]):
        break
      cols[j][i] = rows[i][j]
  return cols


def max_by(xs: List[Any]) -> int:
  assert len(xs) > 0, 'Expected at least one element in `xs`'
  max_i = 0
  max_val = xs[0]
  for i in range(1, len(xs)):
    if xs[i] < max_val:
      continue
    max_val = xs[i]
    max_i = i
  return max_i


def align(utts: List[Utterance], lyrics: List[str]) -> List[Utterance]:
  """Given a list of utterances (typically from aws transribe) and lyrics,
  try aligning the lyrics according to when the utterances occurred."""
  miss_score = -20
  match_score = 10
  vertical_discount = 15
  horizontal_discount = 15

  utt_mat = []
  for utt in utts:
    row = []
    for word in lyrics:
      row.append(match_score if utt[3] == word else miss_score)
    utt_mat.append(row)

  mat = transpose(utt_mat)
  scores = [[0 for _ in range(len(utts))] for _ in range(len(lyrics))]
  for i, row in enumerate(mat):
    for j, val in enumerate(row):
      vals = [0]
      if i > 0 and j > 0:
        vals.append(scores[i - 1][j - 1] + val)
      if i > 0:
        vals.append(scores[i - 1][j] - vertical_discount)
      if j > 0:
        vals.append(scores[i][j - 1] - horizontal_discount)
      scores[i][j] = max(vals)

  ret = []
  for i in reversed(list(range(len(lyrics)))):
    row = scores[i]
    j = 0
    j = max_by(row)
    ret.append((
      utts[j][0],
      utts[j][1],
      utts[j][2],
      lyrics[i]
    ))

  return list(reversed(ret))


def alignLyrics(tsv_file: str, lyric_file: str) -> None:
  transcribed = []
  with open(tsv_file, 'rb') as in_f:
      for line in in_f:
        cols = line.decode('utf-8').rstrip('\n').split('\t')
        start = float(cols[0])
        end = float(cols[1])
        duration = float(cols[2])
        # TODO maybe normalize (transliterate) this?
        text = cols[3].lower()
        transcribed.append((start, end, duration, text))
  # As suggested by https://stackoverflow.com/questions/3939361/remove-specific-characters-from-a-string-in-python
  remove_table = dict.fromkeys(map(ord, '?!-.,'), None)
  lyrics = []
  with open(lyric_file, 'rb') as in_f:
    for line in in_f:
      for word in line.decode('utf-8').rstrip('\n').split(' '):
        lyrics.append(word.translate(remove_table).lower())
  print('Len transcribed=%d, first N=%s' % (len(transcribed), str(transcribed[:30])))
  print('Len lyrics=%d, first N=%s' % (len(lyrics), str(lyrics[:30])))
  for item in align(transcribed, lyrics):
    print('{start}\t{end}\t{duration:0.3f}\t{content}'.format(
      start=item[0],
      end=item[1],
      duration=item[2],
      content=item[3]
    ))
  return


if __name__ == '__main__':
  parser = argparse.ArgumentParser('Try to align timestamps with text')
  parser.add_argument('tsv', help='The TSV file with labeled (guessed) lyrics')
  parser.add_argument('lyrics', help='The un-timestamped lyrics')
  args = parser.parse_args()
  alignLyrics(args.tsv, args.lyrics)
  # Problems:
  # 1) Misses jump to the end, or way off, because of max_by. ffmpeg just drops
  #    these because they're considered "duplicates".
  # 2) Streaks/high precision matches should be treated more like anchors
  # 3) We need to call `normalizeTextContent` from subtitler... And implement
  #    it's transliteration.
  # 4) Imprecise utterances that are several seconds long should be split.
