# test_subtitler.py
# Trevor Pottinger
# Sun Jun  7 22:32:51 PDT 2020

import unittest

from subtitler import align, normalizeTextContent


class TestSubtitler(unittest.TestCase):
  def test_align(self):
    utts = [
      (17.54, 18.15, 0.6099999999999994, 'um'),
      (26.24, 26.37, 0.13000000000000256, 'we'),
      (26.38, 26.64, 0.26000000000000156, 'went'),
      (26.64, 26.73, 0.08999999999999986, 'to'),
      (26.73, 26.82, 0.08999999999999986, 'the'),
      (26.82, 27.01, 0.19000000000000128, 'more'),
      (27.01, 27.21, 0.1999999999999993, 'pet'),
      (27.21, 27.64, 0.4299999999999997, 'store'),
      (28.36, 29.06, 0.6999999999999993, 'salesman'),
      (29.06, 29.16, 0.10000000000000142, 'is'),
      (29.16, 29.38, 0.21999999999999886, 'like'),
      (29.39, 29.8, 0.41000000000000014, 'what'),
      (29.81, 30.06, 0.25, "what's"),
      (30.06, 30.16, 0.10000000000000142, 'your'),
      (30.16, 30.52, 0.35999999999999943, 'budget'),
      (30.52, 30.64, 0.120000000000001, 'and'),
      (30.64, 30.73, 0.08999999999999986, "i'm"),
      (30.73, 30.96, 0.23000000000000043, 'like'),
      (30.97, 31.45, 0.4800000000000004, 'honestly'),
      (31.45, 31.49, 0.03999999999999915, 'i'),
      (31.49, 31.66, 0.1700000000000017, "don't"),
      (31.66, 31.74, 0.0799999999999983, 'know'),
      (31.74, 32.11, 0.370000000000001, 'nothing'),
      (32.11, 32.34, 0.23000000000000398, 'about'),
      (32.34, 32.48, 0.13999999999999346, 'my'),
      (32.48, 32.8, 0.3200000000000003, 'pets'),
      (32.8, 32.91, 0.10999999999999943, 'he'),
      (32.91, 33.15, 0.240000000000002, 'said'),
      (33.16, 33.29, 0.13000000000000256, 'i'),
      (33.29, 33.46, 0.1700000000000017, 'got'),
    ]
    lyrics = [
      'i', 'went', 'to', 'the', 'moped', 'store', 'said', 'fuck', 'it',
      "salesman's", 'like', 'what', 'up', "what's", 'your', 'budget?', 'and',
      "i'm", 'like', 'honestly', 'i', "don't", 'know', 'nothing', 'about',
      'mopeds', 'he', 'said', 'i', 'got',
    ]
    results = align(utts, lyrics)
    # check that all lyrics got assigned
    self.assertEqual(len(results), len(lyrics))
    for i in range(len(results)):
      self.assertEqual(
        results[i][3],
        lyrics[i],
        'Expected word %i to be "%s" but got: %s' % (i, lyrics[i], results[i][3])
      )
    # TODO fix selecting the first element
    # self.assertEqual(results[0][0], 26.38)
    self.assertEqual(results[-1][0], 33.29)

    utts = [
      (3.94, 4.24, 0.30000000000000027, 'कौन'),
      (41.54, 41.86, 0.3200000000000003, 'वही'),
      (41.86, 42.01, 0.14999999999999858, 'जो'),
      (42.01, 42.29, 0.28000000000000114, 'मिला'),
      (42.29, 42.43, 0.14000000000000057, 'तो'),
      (42.43, 42.77, 0.3400000000000034, 'मुझे'),
      (42.77, 43.06, 0.28999999999999915, 'ऐसा'),
      (43.06, 43.49, 0.4299999999999997, 'लगता'),
      (43.49, 43.63, 0.14000000000000057, 'था'),
      (43.63, 43.94, 0.30999999999999517, 'जैसे'),
      (43.94, 44.25, 0.3100000000000023, 'मेरी'),
      (44.25, 44.57, 0.3200000000000003, 'सारे'),
      (44.57, 44.71, 0.14000000000000057, 'जो'),
      (44.71, 44.95, 0.240000000000002, 'नहीं'),
      (44.95, 47.15, 2.1999999999999957, 'आॅफर'),
      (47.15, 47.43, 0.28000000000000114, 'लिखा'),
      (47.44, 47.57, 0.13000000000000256, 'है'),
      (47.57, 47.81, 0.240000000000002, 'खुश'),
      (47.81, 48.01, 0.19999999999999574, 'हूँ'),
      (48.01, 48.2, 0.19000000000000483, 'कि'),
      (48.21, 48.59, 0.38000000000000256, 'आती'),
      (48.6, 48.77, 0.1700000000000017, 'है'),
      (48.77, 49.1, 0.3299999999999983, 'मैं'),
      (49.25, 50.58, 1.3299999999999983, 'फॅमिली'),
      (50.58, 51.0, 0.4200000000000017, 'चाहे'),
      (51.0, 51.2, 0.20000000000000284, 'है'),
      (51.2, 52.84, 1.6400000000000006, 'फॅमिली'),
      (52.84, 52.97, 0.12999999999999545, 'है'),
      (52.97, 53.31, 0.3400000000000034, 'वहाँ'),
      (53.31, 53.57, 0.259999999999998, 'हैं'),
    ]
    lyrics = [
      'koi', 'jo', 'mila', 'to', 'mujhe', 'aisa', 'lagta', 'tha', 'jaise',
      'meri', 'saari', 'duniya', 'main', 'geeton', 'ki', 'rut', 'aur',
      'rangon', 'ki', 'barkha', 'hai', 'khushbu', 'ki', 'aandhi', 'hai',
      'mehki', 'hui', 'si', 'ab', 'saari',
    ]
    results = align(utts, lyrics)
    self.assertEqual(len(results), len(lyrics))
    for i in range(len(results)):
      self.assertEqual(
        results[i][3],
        lyrics[i],
        'Expected word %i to be "%s" but got: %s' % (i, lyrics[i], results[i][3])
      )

  def test_normalizeTextContent(self):
    self.assertEqual('a', normalizeTextContent('A'))
    self.assertEqual('mila', normalizeTextContent('मिला'))
    self.assertEqual('jo', normalizeTextContent('जो'))
    # TODO don't delete the last char for a modifier if the last char was not a vowel
    # self.assertEqual('mujhe', normalizeTextContent('मुझे'))


if __name__ == '__main__':
  unittest.main()
