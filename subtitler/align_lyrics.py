# align_lyrics.py
# Trevor Pottinger
# Sun Jun  7 23:06:18 PDT 2020

import argparse


def align() -> None:
  return


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
  remove_table = dict.fromkeys(map(ord, '!.,'), None)
  lyrics = []
  with open(lyric_file, 'rb') as in_f:
    for line in in_f:
      for word in line.decode('utf-8').rstrip('\n').split(' '):
        lyrics.append(word.translate(remove_table).lower())
  print(transcribed[:30])
  print(lyrics[:30])
  return


if __name__ == '__main__':
  parser = argparse.ArgumentParser('Try to align timestamps with text')
  parser.add_argument('tsv', help='The TSV file with labeled (guessed) lyrics')
  parser.add_argument('lyrics', help='The un-timestamped lyrics')
  args = parser.parse_args()
  alignLyrics(args.tsv, args.lyrics)
