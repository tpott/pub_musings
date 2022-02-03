# deseasonalize.py
# Trevor Pottinger
# Wed Feb  2 21:45:41 PST 2022

import sys

import numpy as np
import pandas as pd
import scipy.fftpack

# Note: idct of type 2 == dct of type 3
# np.flip reverses the order of the np array
# dropna is to drop NaN values

def deseasonalize(n: int, arr: np.array) -> np.array:
  max_i = np.argmax(arr)
  rotated = np.roll(arr, -max_i)
  freqs = scipy.fftpack.dct(rotated, type=2, norm=None)
  top_freqs = np.flip(np.argsort(np.abs(freqs)))
  # TODO properly Skip freqs[0] because it's the constant frequency
  for i in range(1, n + 1):
    freqs[top_freqs[i]] = 0
  normalized = scipy.fftpack.idct(freqs, type=2, norm=None)
  return np.roll(1.0 / (2 * arr.shape[0]) * normalized, max_i)


def main() -> None:
  df = pd.read_csv(sys.stdin)
  # TODO figure out why NaN values are showing up
  series = df[df.Name == 'MMM'].Open.dropna().to_numpy() 
  print(series)
  print(deseasonalize(int(sys.argv[1]), series))


if __name__ == '__main__':
  main()
