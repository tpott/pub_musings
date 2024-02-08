# tabularize.py
# Trevor Pottinger
# Sun Feb  4 21:52:43 PST 2024

import argparse
from io import StringIO
import sys
from typing import Dict

import pandas as pd


def parseArgs() -> argparse.Namespace:
  parser = argparse.ArgumentParser(description="Tabularize OHLC csv data")
  parser.add_argument("input_file", help="The input CSV file. Use \"-\" for stdin")
  parser.add_argument("--tickers", help="Comma separated list of tickers to select")
  return parser.parse_args()
  

def main(args: Dict[str, str]) -> None:
  tickers = []
  if args["tickers"] is not None:
    for ticker in args["tickers"].split(","):
      tickers.append(ticker)

  if args["input_file"] != "-":
    df = pd.read_csv(args["input_file"])
  else:
    df = pd.read_csv(sys.stdin)

  # assert Name and Date separately, cause we don't want it to end up in the result
  assert "Name" in df.columns
  assert "Date" in df.columns

  expected_columns = ["Volume", "Open", "High", "Low", "Close"]
  for col in expected_columns:
    assert col in df.columns

  # TODO undo
  # "Adj Close" is optional
  # if "Adj Close" in df.columns:
    # expected_columns.append("Adj_Close")

  existing_tickers = df.Name.unique()
  selectable = sorted(set(tickers).intersection(existing_tickers))
  print(f"found {len(existing_tickers)} distinct tickers", file=sys.stderr)
  print(f"only selecting {len(tickers)} of them... result: {selectable}", file=sys.stderr)
  df = df[df.Name.isin(selectable)]

  # inplace=False means we return a new series rather than modifying "in place"
  dates = df.Date.drop_duplicates().reset_index(drop=True, inplace=False)
  # Note: df.Date can have duplicates because multiple tickers may have data
  df.set_index("Date", inplace=True)
  res_df = pd.DataFrame(dates)
  res_df.set_index("Date", inplace=True)

  res_columns = []
  for ticker in selectable:
    for col in expected_columns:
      # values = df[df.Name == ticker]. \
        # sort_index(inplace=False)[col]. \
      values = df[df.Name == ticker][col]. \
        rename(f"{ticker}_{col}"). \
        reindex_like(res_df)
        # reindex(index=res_df.index)
        # reset_index(drop=True, inplace=False)
        # reset_index(drop=False, inplace=False)
        # reset_index(inplace=False)
      res_columns.append(values)

  res_df = res_df.join(res_columns, how="inner", sort=True)
  res_df.reset_index(inplace=True)

  output = StringIO()
  res_df.to_csv(output, index=False)
  output.seek(0)
  print(output.read())


if __name__ == "__main__":
  main(vars(parseArgs()))
