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
    self.assertEqual(results[0][0], 26.38)
    self.assertEqual(results[-1][0], 33.29)

  def test_normalizeTextContent(self):
    self.assertEqual('a', normalizeTextContent('A'))


if __name__ == '__main__':
  unittest.main()
