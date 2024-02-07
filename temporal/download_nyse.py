# download_nyse.py
# Trevor Pottinger
# Thu Dec  1 09:12:50 PST 2022

import argparse
import json
import os
import secrets
import shutil
import tempfile
import time

YMD_FORMAT = "%Y-%m-%d"

def main() -> None:
  parser = argparse.ArgumentParser(description="Fetch some tickers")
  parser.add_argument("-n", type=int, default=100000, help="Number of tickers to fetch")
  args = parser.parse_args()

  data_dir = tempfile.mkdtemp()

  page = 1
  chunk_size = 10
  while True:
    # --fail-with-body ensures curl returns an error when we get a non 200 response
    command = f"curl 'https://www.nyse.com/api/quotes/filter' --silent -H 'Content-Type: application/json' -H 'Referer: https://www.nyse.com/listings_directory/stock' --data-raw '{{\"instrumentType\":\"EQUITY\",\"pageNumber\":{page},\"sortColumn\":\"NORMALIZED_TICKER\",\"sortOrder\":\"ASC\",\"maxResultsPerPage\":{chunk_size},\"filterToken\":\"\"}}' --fail-with-body > {data_dir}/{page}"
    ret = os.system(command)
    print(f"`{command}` = {ret}")
    if ret != 0:
      break
    if page == args.n:
      break
    time.sleep(0.2)
    page += 1

  all_tickers = []
  for f_name in os.listdir(data_dir):
    tickers = []
    with open(os.path.join(data_dir, f_name)) as f:
      tickers = json.loads(f.read())
    for t in tickers:
      # I also saw 'PREFERRED_STOCK'
      if t['instrumentType'] != 'COMMON_STOCK':
        continue
      # maybe use symbolExchangeTicker or normalizedTicker or symbolEsignalTicker
      all_tickers.append(t['symbolTicker'])

  tickers_file = f"tickers_{secrets.token_hex(8)}.csv"
  with open(tickers_files, "wb") as f:
    f.write("ticker\n".encode("utf-8"))
    for t in all_tickers:
      f.write(f"{t}\n".encode("utf-8"))
  shutil.rmtree(data_dir)

  return

if __name__ == "__main__":
  main()
