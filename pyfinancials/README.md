# pyfinancials

Lets see how far we can get without any external deps

# Goal

Deterministically fetch a bunch of source data, do some transformation. And
then the final data should be easily presentable.

# Open Questions

## How to parse HTML without a complex HTML parsing library?

Instead of answering this, I cheated and used [BeautifulSoup](https://github.com/edouardswiac/python-edgar). I would
like to wrap this inside the API in html_parser.py so that I can work on
removing this dependency.

## How best to cache the fetched data and avoid re-fetching?

# Notes

```
# Download from https://finance.yahoo.com/quote/%5EGSPC/history?period1=1413529200&period2=1571295600&interval=1d&filter=history&frequency=1d
~/Github/miller/c/mlr --icsv --from source_data/\^GSPC.csv --ojson cat \
  | jq '.["Adj Close"]' \
  | python3 n_squared.py \
  | python3 ~/tsv_summarize.py --cast_types f --agg_types n
```

S&P 500 fetching ends up functioning like this:
1) fetch company list from wikipedia (`Wikipedia.gen_list_s_and_p_500`)
2) for each company, fetch list of 10-Ks' metadata (`Edgar.gen_10ks`)
3) for each 10-K, fetch it's metadata (`Edgar.gen_documents`). Then fetch it's content (`Edgar.gen_ten_k`)

# Other Projects

* https://github.com/joeyism/py-edgar
* https://github.com/ryansmccoy/py-sec-edgar
* https://github.com/coyo8/sec-edgar
* https://github.com/andrewkittredge/financial_fundamentals
* https://github.com/lukerosiak/pysec
* https://github.com/edouardswiac/python-edgar
