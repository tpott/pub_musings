# backtest.py
# Trevor Pottinger
# Sat Feb 26 21:34:08 PST 2022

import argparse
from datetime import datetime
from datetime import timedelta
from datetime import timezone
from typing import Any, List

import numpy as np
import pandas as pd

from deseasonalize import deseasonalize

YMD_FORMAT = "%Y-%m-%d"

_now = datetime.fromtimestamp(0, timezone.utc)
_history = []
_trades = []

def trades() -> List[Any]:
  return _trades

def buy(ticker, date) -> None:
  _trades.append(["buy", ticker, date])

def sell(ticker, date) -> None:
  _trades.append(["sell", ticker, date])

def main() -> None:
  """this is basically a mean reversion strategy"""
  parser = argparse.ArgumentParser(description="Toy backtest app")
  parser.add_argument("input_file", help="The input CSV file. Use \"-\" for stdin")
  parser.add_argument("-m", "--metric", default="Open", help="Which metric to analyze")
  # TODO random seed
  args = parser.parse_args()

  metrics = ["Open", "High", "Low", "Close"]
  if args.metric not in metrics:
    print(f"Expected metric, {args.metric}, to be in one of {metrics}")
    sys.exit(1)

  df = pd.read_csv(args.input_file)

  date_lambda = lambda x: pd.Series({"Values": dict(zip(x.Name, x[args.metric]))})
  date_df = df.groupby("Date").apply(date_lambda)
  big_df = pd.DataFrame(date_df.Values.values.tolist(), index=date_df.index)
  big_df = big_df.dropna(axis=0, how="any")

  start = datetime.strptime(big_df.index[0], YMD_FORMAT)
  for date in big_df.index:
    today = datetime.strptime(date, YMD_FORMAT)
    if (today - start).days <= 730:
      continue
    window_start = (today - timedelta(days=730)).strftime(YMD_FORMAT)
    for ticker in big_df.columns:
      history = big_df[ticker].loc[window_start:date].dropna().to_numpy()
      deseasonalized = deseasonalize(4, history)
      # TODO pick random price between high and low, preferably between low and close
      if history[-1] < np.percentile(deseasonalized, 0.8) * 0.80:
        buy(ticker, date)
      if history[-1] > np.percentile(deseasonalized, 99.6) * 1.4:
        sell(ticker, date)

  for trade in trades():
    print(trade)

  return

if __name__ == '__main__':
  main()
