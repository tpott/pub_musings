# tsv2srt.py
# Trevor Pottinger
# Sat May 16 16:09:19 PDT 2020

import argparse
import math
import os
from typing import NewType, Tuple

SuccessOrFailure = NewType('SuccessOrFailure', bool)
Success = SuccessOrFailure(True)
Failure = SuccessOrFailure(False)


class Utterance(object):
  def __init__(self, start: float, duration: float, content: str) -> None:
    self.start = start
    self.duration = duration
    self.content = content
    self.end = start + duration


# https://wiki.videolan.org/SubViewer/
def timeize(seconds: float) -> str:
  hours = int(math.floor(seconds / 3600))
  seconds -= hours * 3600
  minutes = int(math.floor(seconds / 60))
  seconds -= minutes * 60
  return "{hours:02d}:{minutes:02d}:{seconds:02.03f}".format(
    hours=hours,
    minutes=minutes,
    seconds=seconds
  )


def textize(index_result_tuple: Tuple[int, Utterance]) -> str:
  index, result = index_result_tuple
  end = timeize(result.end)
  start = timeize(result.start)
  return """{index}
{start} --> {end}
{text}""".format(index=index + 1, start=start, end=end, text=result.content)


def tsv2srt(in_file: str, out_file: str) -> SuccessOrFailure:
  if not os.path.exists(in_file):
    print('Input file, %s, does not exist' % in_file)
    return Failure

  results = []
  with open(in_file, 'rb') as f:
    for line in f:
      cols = line.decode('utf-8').rstrip('\n').split('\t')
      if cols[0] == '':
        continue
      try:
        duration = float(cols[1])
        results.append(Utterance(float(cols[0]), duration, cols[2]))
      except ValueError as e:
        print('Failed to parse row %d. %s: %s' % (len(results), type(e).__name__, str(e)))
        continue
      if results[-1].duration - duration > 0.0001:
        print('Incorrect duration in row %d. Read %f, but derived %f' % (
          len(results), duration, results[-1].duration))

  text = "\n\n".join(list(map(textize, enumerate(results))))
  with open(out_file, 'wb') as f:
    f.write(text.encode('utf-8'))
  return Success


def main() -> None:
  parser = argparse.ArgumentParser('Convert TSV data to SRT subtitles')
  parser.add_argument('in_file', help='The TSV file')
  parser.add_argument('out_file', help='The output SRT file')
  args = parser.parse_args()
  _ = tsv2srt(args.in_file, args.out_file)


if __name__ == '__main__':
  main()
