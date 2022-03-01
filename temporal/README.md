# temporal

Fun temporal analysis

* From https://www.kaggle.com/szrlee/stock-time-series-20050101-to-20171231
* From `curl -o {ticker}.csv https://query1.finance.yahoo.com/v7/finance/download/{ticker}?period1={start}&period2={end}&interval=1d&events=history&includeAdjustedClose=true`

# Timing

`time python3 backtest.py all_stocks_2006-01-01_to_2018-01-01.csv 4 --until 2010-01-01 | head -n 20`

or

`python3 -m cProfile -o profile backtest.py all_stocks_2006-01-01_to_2018-01-01.csv 4 --until 2010-01-01 | head -n 20`

and then

```
import pstats
from pstats import SortKey
p = pstats.Stats("profile")
p.sort_stats(SortKey.CUMULATIVE).print_stats(20)
```

# Running

```
python3 download_yahoo.py tickers.csv
python3 backtest.py combined_9d6c044089bbee84.csv 4 --since 2012-01-01 --until 2020-01-01 > /tmp/trades
python3 deseasonalize.py combined_a392a33ee14c7203.csv 4 --metric Open --since 2014-01-01 --until 2022-01-01 --ticker GE
```
