# temporal

Fun temporal analysis

* From https://www.kaggle.com/szrlee/stock-time-series-20050101-to-20171231

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
