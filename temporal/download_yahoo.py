# download_yahoo.py
# Trevor Pottinger
# Sun Feb 27 18:17:59 PST 2022

import argparse
from datetime import datetime
from datetime import timedelta
import os
import secrets
import shutil
import tempfile
import time

import pandas as pd

YMD_FORMAT = "%Y-%m-%d"

def main() -> None:
  parser = argparse.ArgumentParser(description="Fetch some open, high, low, close data for the given tickers. Defaults to last 20 years")
  parser.add_argument("ticker_file", help="A CSV file with tickers. Use \"-\" for stdin")
  args = parser.parse_args()

  if args.ticker_file != "-":
    df = pd.read_csv(args.ticker_file)
  else:
    df = pd.read_csv(sys.stdin)

  data_dir = tempfile.mkdtemp()

  now = datetime.utcnow()
  for ticker in df.ticker.values:
    end = now
    for i in range(4):
      start = (end - timedelta(days=5 * 365 + 1))
      command = f"curl --fail-with-body --silent -o {data_dir}/{ticker}_{i}.csv 'https://query1.finance.yahoo.com/v7/finance/download/{ticker}?period1={int(start.timestamp())}&period2={int(end.timestamp())}&interval=1d&events=history&includeAdjustedClose=true'"
      ret = os.system(command)
      print(f"`{command}` = {ret}")
      time.sleep(0.2)
      end = start

  dfs = []
  for file_name in os.listdir(data_dir):
    df = pd.read_csv(os.path.join(data_dir, file_name))
    if "Date" not in df.columns:
      print(f"Found weird data in {file_name}, skipping")
      continue
    ticker = file_name.split("_")[0]
    df["Name"] = [ticker] * len(df.index)
    dfs.append(df)

  combined_df = pd.concat([df for df in dfs])
  combined_df.to_csv(f"combined_{secrets.token_hex(8)}.csv", index=False)

  shutil.rmtree(data_dir)

  return

if __name__ == "__main__":
  main()
