# align_lyrics.py
# Trevor Pottinger
# Sun Jun  7 23:06:18 PDT 2020

import argparse
from typing import Any, List, Tuple


Utterance = Tuple[float, float, float, str]


# Generated from `python3 -c 'print("\n".join(["%d: (\"?\", False, %s)," % (i, chr(i)) for i in range(2304, 2390)]))' >> align_lyrics.py`
CHAR_MAP = {
  # 2304 - 2307 are modifiers that are hard to print..
  2304: ('a', True, ''),  # Example: कऀ
  2305: ('an', True, ''),  # Example: कँ
  2306: ('an', True, ''),  # Example: कं
  2307: ('ah', True, ''),  # Example: कः

  # 2308 - 2361 are full chars
  2308: ('?', False, 'ऄ'),
  2309: ('a', False, 'अ'),
  2310: ('aa', False, 'आ'),
  2311: ('i', False, 'इ'),
  2312: ('ee', False, 'ई'),
  2313: ('u', False, 'उ'),
  2314: ('oo', False, 'ऊ'),
  2315: ('r', False, 'ऋ'),
  2316: ('?', False, 'ऌ'),
  2317: ('ai', False, 'ऍ'),
  2318: ('ai', False, 'ऎ'),
  2319: ('e', False, 'ए'),
  2320: ('ai', False, 'ऐ'),
  2321: ('o', False, 'ऑ'),
  2322: ('o', False, 'ऒ'),
  2323: ('o', False, 'ओ'),
  2324: ('au', False, 'औ'),
  2325: ('ka', False, 'क'),
  2326: ('kha', False, 'ख'),
  2327: ('ga', False, 'ग'),
  2328: ('gha', False, 'घ'),
  2329: ('na', False, 'ङ'),
  2330: ('cha', False, 'च'),
  2331: ('chha', False, 'छ'),
  2332: ('ja', False, 'ज'),
  2333: ('jha', False, 'झ'),
  2334: ('na', False, 'ञ'),
  2335: ('ta', False, 'ट'),
  2336: ('tha', False, 'ठ'),
  2337: ('da', False, 'ड'),
  2338: ('dha', False, 'ढ'),
  2339: ('na', False, 'ण'),
  2340: ('ta', False, 'त'),
  2341: ('tha', False, 'थ'),
  2342: ('da', False, 'द'),
  2343: ('dha', False, 'ध'),
  2344: ('na', False, 'न'),
  2345: ('na', False, 'ऩ'),
  2346: ('pa', False, 'प'),
  2347: ('pha', False, 'फ'),
  2348: ('ba', False, 'ब'),
  2349: ('bha', False, 'भ'),
  2350: ('ma', False, 'म'),
  2351: ('ya', False, 'य'),
  2352: ('ra', False, 'र'),
  2353: ('ra', False, 'ऱ'),
  2354: ('la', False, 'ल'),
  2355: ('la', False, 'ळ'),
  2356: ('la', False, 'ऴ'),
  2357: ('va', False, 'व'),
  2358: ('sha', False, 'श'),
  2359: ('sha', False, 'ष'),
  2360: ('sa', False, 'स'),
  2361: ('ha', False, 'ह'),

  # 2362 - 2389 are modifiers that don't render nicely
  2362: ('?', True, ''),
  2363: ('?', True, ''),
  2364: ('?', True, ''),
  2365: ('?', True, 'ऽ'),
  2366: ('a', True, ''),
  2367: ('i', True, ''),
  2368: ('?', True, ''),
  2369: ('u', True, ''),
  2370: ('u', True, ''),
  2371: ('?', True, ''),
  2372: ('?', True, ''),
  2373: ('e', True, ''),
  2374: ('?', True, ''),
  2375: ('e', True, ''),
  2376: ('ai', True, ''),
  2377: ('?', True, ''),
  2378: ('?', True, ''),
  2379: ('o', True, ''),
  2380: ('ie', True, ''),
  2381: ('i', True, ''),
  2382: ('?', True, ''),
  2383: ('?', True, ''),
  2384: ('?', True, 'ॐ'),
  2385: ('?', True, ''),
  2386: ('?', True, ''),
  2387: ('?', True, ''),
  2388: ('?', True, ''),
  2389: ('?', True, ''),
}

# mila mailaa
# 2350, 2367, 2354, 2366
def normalizeTextContent(s: str) -> str:
  s = s.lower()
  lang = None
  # Check that all chars are from the same language
  for c in s:
    o = ord(c)
    c_lang = None
    if 2304 <= o and o <= 2389:
      c_lang = 'hindi'
    if lang is None:
      lang = c_lang
      continue
    if lang == c_lang:
      continue
    raise ValueError('Multiple languages detected! Got %s, expected %s' % (c_lang, lang))
  # Now we know the lang
  if lang is None:
    return s
  new_s = ''
  # Construct a transliterated string
  for c in s:
    en_c, modifier, _ = CHAR_MAP[ord(c)]
    if modifier and len(new_s) > 0:
      new_s = new_s[:-1]
    new_s += en_c
  return new_s


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
    norm_text = normalizeTextContent(utt[3])
    row = []
    for word in lyrics:
      row.append(match_score if norm_text == word else miss_score)
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

  js = []
  ret = []
  for i in reversed(list(range(len(lyrics)))):
    row = scores[i]
    j = max_by(row)
    js.append(j)
    ret.append((
      utts[j][0],
      utts[j][1],
      utts[j][2],
      lyrics[i]
    ))
  print(list(reversed(js)))

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
