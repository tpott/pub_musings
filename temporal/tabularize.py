# tabularize.py
# Trevor Pottinger
# Sun Feb  4 21:52:43 PST 2024

import argparse
from io import StringIO
import sys

import pandas as pd


def main() -> None:
  parser = argparse.ArgumentParser(description="Tabularize OHLC csv data")
  parser.add_argument("input_file", help="The input CSV file. Use \"-\" for stdin")
  parser.add_argument("--tickers", help="Comma separated list of tickers to select")
  args = parser.parse_args()

  tickers = []
  if args.tickers is not None:
    for ticker in args.tickers.split(","):
      tickers.append(ticker)

  if args.input_file != "-":
    df = pd.read_csv(args.input_file)
  else:
    df = pd.read_csv(sys.stdin)

  # assert Name and Date separately, cause we don't want it to end up in the result
  assert "Name" in df.columns
  assert "Date" in df.columns

  expected_columns = ["Volume", "Open", "High", "Low", "Close"]
  for col in expected_columns:
    assert col in df.columns

  # "Adj Close" is optional
  # if "Adj Close" in df.columns:
    # expected_columns.append("Adj_Close")

  existing_tickers = df.Name.unique()
  selectable = set(tickers).intersection(existing_tickers)
  print(f"found {len(existing_tickers)} distinct tickers")
  print(f"only selecting {len(tickers)} of them... result: {selectable}")
  df = df[df.Name.isin(selectable)]

  res_columns1 = ["Date"]
  res_columns1.extend([f"{ticker}_{col}" for col in expected_columns for ticker in selectable])
  print(res_columns1)

  # res_df = df.pivot(index="Date", columns=res_columns)
  # res_df = df.pivot(index="Date", columns="Name")
  # Reset index to make "Date" a regular column
  # res_df.reset_index(inplace=True)
  # print(res_df)

  # inplace=False means we return a new series rather than modifying "in place"
  dates = df.Date.sort_values().reset_index(drop=True, inplace=False)

  res_columns = [dates]
  for ticker in selectable:
    for col in expected_columns:
      res_columns.append(
        df[df.Name == ticker].sort_values(by="Date")[col].reset_index(drop=True, inplace=False),
      )

  res_df = pd.concat(res_columns, axis=1)

  output = StringIO()
  res_df.to_csv(output, index=False)
  output.seek(0)
  print(output.read())


if __name__ == "__main__":
  main()
