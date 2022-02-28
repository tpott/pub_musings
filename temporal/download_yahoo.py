# download_yahoo.py
# Trevor Pottinger
# Sun Feb 27 18:17:59 PST 2022

from datetime import datetime
from datetime import timedelta
import os
import time

YMD_FORMAT = "%Y-%m-%d"

def main() -> None:
  now = datetime.utcnow()
  # Removed AABA
  tickers = [
    "AAPL", "AMZN", "AXP", "BA", "CAT", "CSCO", "CVX", "DIS", "GE", "GOOGL",
    # "FB",
    "GS", "HD", "IBM", "INTC", "JNJ", "JPM", "KO", "MCD", "MMM", "MRK",
    "MSFT", "NKE", "PFE", "PG", "TRV", "UNH", "UTX", "VZ", "WMT", "XOM",
  ]
  for ticker in tickers:
    end = now
    for i in range(4):
      start = (end - timedelta(days=5 * 365 + 1))
      command = f"curl -o {ticker}_{i}.csv 'https://query1.finance.yahoo.com/v7/finance/download/{ticker}?period1={int(start.timestamp())}&period2={int(end.timestamp())}&interval=1d&events=history&includeAdjustedClose=true'"
      print(command)
      # os.system(command)
      end = start
  return

if __name__ == "__main__":
  main()
