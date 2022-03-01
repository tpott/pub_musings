# backtest.py
# Trevor Pottinger
# Sat Feb 26 21:34:08 PST 2022

import argparse
from collections import defaultdict
from datetime import datetime
from datetime import timedelta
from datetime import timezone
import random
import secrets
import sys
from typing import Any, List

import numpy as np
import pandas as pd

from deseasonalize import deseasonalize

YMD_FORMAT = "%Y-%m-%d"

_now = datetime.fromtimestamp(0, timezone.utc)
_history = []
_trades = []

def estimate(ticker, date, df) -> float:
  ohlc = df.loc[date, ticker]
  # TODO better weight this according to Open and Close
  # TODO weight this by the hourly time window with the highes Volume
  spread = ohlc.High - ohlc.Low
  return round(random.random() * spread + ohlc.Low, 2)

def trades() -> List[Any]:
  return _trades

def buy(ticker, date, price) -> None:
  _trades.append(["buy", ticker, date, price])

def sell(ticker, date, price) -> None:
  _trades.append(["sell", ticker, date, price])

def main() -> None:
  """this is basically a mean reversion strategy"""
  parser = argparse.ArgumentParser(description="Toy backtest app")
  parser.add_argument("input_file", help="The input CSV file. Use \"-\" for stdin")
  parser.add_argument("n", type=int, help="The number of frequencies to deseasonalize")
  parser.add_argument("-m", "--metric", default="Open", help="Which metric to analyze")
  parser.add_argument("-r", "--seed", type=int, help="The pseudo random seed for simulations")
  parser.add_argument("-s", "--since", help="Starting time, inclusive")
  parser.add_argument("-u", "--until", help="Ending time, exclusive")
  args = parser.parse_args()

  seed = args.seed
  if seed is None:
    seed = secrets.randbits(64)
  random.seed(seed)
  print(f"Seeding pseudo random number generator with {seed}", file=sys.stderr)

  metrics = ["Open", "High", "Low", "Close"]
  if args.metric not in metrics:
    print(f"Expected metric, {args.metric}, to be in one of {metrics}", file=sys.stderr)
    sys.exit(1)

  if args.input_file != "-":
    df = pd.read_csv(args.input_file)
  else:
    df = pd.read_csv(sys.stdin)

  if args.since is not None:
    df = df[df.Date >= args.since]
  if args.until is not None:
    df = df[df.Date < args.until]

  date_lambda = lambda x: pd.Series({"Values": dict(zip(x.Name, x[args.metric]))})
  date_df = df.groupby("Date").apply(date_lambda)
  big_df = pd.DataFrame(date_df.Values.values.tolist(), index=date_df.index)
  # Drops rows where any ticker had a NaN metric value
  # big_df = big_df.dropna(axis=0, how="any")

  df.set_index(["Date", "Name"], inplace=True)

  num_trading_days = 0
  start = datetime.strptime(big_df.index[0], YMD_FORMAT)
  for date in big_df.index:
    num_trading_days += 1
    today = datetime.strptime(date, YMD_FORMAT)
    if (today - start).days <= 730:
      continue
    window_start = (today - timedelta(days=730)).strftime(YMD_FORMAT)
    for ticker in big_df.columns:
      if big_df[ticker].loc[window_start:date].isna().sum() > 0:
        continue
      # TODO pre-compute the history
      history = big_df[ticker].loc[window_start:date].to_numpy()
      # TODO pre-compute the deseasonalized history
      deseasonalized = deseasonalize(args.n, history)
      price = estimate(ticker, date, df)
      if history[-1] < np.percentile(deseasonalized, 5) * 0.80:
        buy(ticker, date, price)
      if history[-1] > np.percentile(deseasonalized, 99) * 1.4:
        sell(ticker, date, price)

  print(f"num_trading_days = {num_trading_days}, since = {big_df.index[0]}, until = {big_df.index[-1]}", file=sys.stderr)
  num_trades = 0
  currency_balance = 0.0
  ticker_balances = defaultdict(int)
  buy_counts = defaultdict(int)
  sell_counts = defaultdict(int)
  for trade in trades():
    num_trades += 1
    print("\t".join(map(str, trade)))
    if trade[0] == "buy":
      currency_balance -= trade[3]
      buy_counts[trade[1]] += 1
    else:
      currency_balance += trade[3]
      sell_counts[trade[1]] += 1

  print(f"num_trades = {num_trades}", file=sys.stderr)
  print(f"currency_balance = {round(currency_balance, 2)}", file=sys.stderr)
  value_balance = 0.0
  for ticker in sorted(big_df.columns):
    balance = buy_counts[ticker] - sell_counts[ticker]
    ohlc = df.loc[big_df.index[-1], ticker]
    value = round(ohlc.Close * balance, 2)
    value_balance += value
    print(f"{ticker} +{buy_counts[ticker]} -{sell_counts[ticker]} = {balance} * {ohlc.Close} ~= {value}", file=sys.stderr)
  print(f"value_balance = {value_balance}", file=sys.stderr)

  return

if __name__ == '__main__':
  main()
