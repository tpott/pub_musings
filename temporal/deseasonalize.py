# deseasonalize.py
# Trevor Pottinger
# Wed Feb  2 21:45:41 PST 2022

import argparse
import sys

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import scipy.fftpack

# Default figsize is [6.4, 4.8]
EXTRA_WIDE = [12.8, 4.8]

# Note: idct of type 2 == dct of type 3
# np.flip reverses the order of the np array
# dropna is to drop NaN values

def deseasonalize(n: int, arr: np.array) -> np.array:
  """deseasonalize treats `arr` as a row indexed array. Each row
  is assumed to have columns of values."""
  wrapped = False
  if len(arr.shape) == 1:
    arr = np.array([arr])
    wrapped = True
  max_i = np.argmax(arr, axis=1)
  # np.roll does not play nicely with multiple axis
  # rotated = np.roll(arr, -max_i, axis=1)
  rotated = np.copy(arr)
  for row_i in range(arr.shape[0]):
    rotated[row_i, 0:(arr.shape[1] - max_i[row_i])] = arr[row_i, max_i[row_i]:]
    rotated[row_i, (arr.shape[1] - max_i[row_i]):] = arr[row_i, 0:max_i[row_i]]
  freqs = scipy.fftpack.dct(rotated, type=2, norm=None, axis=1)
  top_freqs = np.flip(np.argsort(np.abs(freqs), axis=1), axis=1)
  # TODO properly Skip freqs[0] because it's the constant frequency
  for row_i in range(arr.shape[0]):
    for i in range(1, n + 1):
      freqs[row_i][top_freqs[row_i][i]] = 0
  normalized = 1.0 / (2 * arr.shape[1]) * scipy.fftpack.idct(freqs, type=2, norm=None, axis=1)
  # return np.roll(normalized, max_i)
  unrotated = np.copy(normalized)
  for row_i in range(arr.shape[0]):
    unrotated[row_i, max_i[row_i]:] = normalized[row_i, 0:(arr.shape[1] - max_i[row_i])]
    unrotated[row_i, 0:max_i[row_i]] = normalized[row_i, (arr.shape[1] - max_i[row_i]):]
  if wrapped:
    return unrotated[0]
  return unrotated


def main() -> None:
  plt.close("all")

  parser = argparse.ArgumentParser(description="Basic deseasonalizer")
  parser.add_argument("input_file", help="The input CSV file. Use \"-\" for stdin")
  parser.add_argument("n", type=int, help="The number of frequencies to deseasonalize")
  parser.add_argument("-m", "--metric", default="Volume", help="Which metric to analyze")
  parser.add_argument("-t", "--ticker", default="MMM", help="Example ticker to test")
  parser.add_argument("-s", "--since", help="Starting time, inclusive")
  parser.add_argument("-u", "--until", help="Ending time, exclusive")
  args = parser.parse_args()

  metrics = ["Volume", "Open", "High", "Low", "Close"]
  if args.metric not in metrics:
    print(f"Expected metric, {args.metric}, to be in one of {metrics}")
    sys.exit(1)

  if args.input_file != "-":
    df = pd.read_csv(args.input_file)
  else:
    df = pd.read_csv(sys.stdin)

  if args.since is not None:
    df = df[df.Date >= args.since]
  if args.until is not None:
    df = df[df.Date < args.until]

  print("summary stats")
  print(df.describe(include="all"))
  print(df.head())
  print(df[["Date", args.metric, "Name"]])

  names = df.Name.value_counts().index.tolist()
  if args.ticker not in names:
    print(f"Expected ticker, {args.ticker}, to be in list of Name's, {str(names)}")
    sys.exit(1)

  print(f"Names: {sorted(names)}")
  print()

  # TODO figure out why NaN values are showing up
  series = df[df.Name == args.ticker][args.metric].dropna().to_numpy()
  # This prints the entire series... which is a bit verbose
  # print(f"{args.ticker} example")
  # print(series)
  # print(deseasonalize(args.n, series))
  # print()

  # History per Name aggregation
  # hist_lambda = lambda x: pd.Series({"History": list(zip(x.Date, x.Volume))})
  # hist_df = df.groupby("Name").apply(hist_lambda)
  # print(hist_df)

  # Values per Date aggregation
  date_lambda = lambda x: pd.Series({"Values": dict(zip(x.Name, x[args.metric]))})
  date_df = df.groupby("Date").apply(date_lambda)
  big_df = pd.DataFrame(date_df.Values.values.tolist(), index=date_df.index)
  big_df = big_df.dropna(axis=0, how="any")
  # Now big_df.transpose() == hist_df from above
  print("big_df and it's transpose")
  print(big_df)
  print(big_df.transpose().values)
  print()

  print("big result")
  norm_arr = deseasonalize(args.n, big_df.transpose().values)
  norm_df = pd.DataFrame(np.transpose(norm_arr), index=big_df.index, columns=big_df.columns)
  print(norm_arr)
  print(norm_df)

  axes = big_df[[args.ticker]].plot(figsize=EXTRA_WIDE)
  norm_df[[args.ticker]].plot(ax=axes)
  plt.legend(["original_df", "deseasonalized_df"])
  plt.show()

  # Is `x` a row, a column, or something else?
  # hist_lambda = lambda x: pd.Series({"History": list(zip(x.Date, x.Volume))})
  # df = df[df.Date < "2007-01-01"]
  # hist_df = df.groupby("Name").apply(hist_lambda)
  # big_df = df.join(hist_df, on="Name", how="left")
  # Tried:
  # big_df.apply(lambda row: pd.Series({"Date": row.Date, "Volume": row.Volume, "History": list(filter(lambda x: x[0] < row.Date, row.History))}), axis=1)
  # big_df.apply(lambda row: pd.Series({**row, **{"History": list(filter(lambda x: x[0] < row.Date, row.History))}}), axis=1)
  return


if __name__ == "__main__":
  main()
