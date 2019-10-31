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

We have URLs, and their raw responses. We can save the time, but how long
should we cache the response for? And maybe keep the cache, just overwrite
the current "state" so we can look at historical changes.

But how do we differentiate the different cached data? And what about the
metadata? Knowing which 10-Ks are for AAPL and which are for GOOG is quite
important.

If the script can apply a filter, and effectively cache responses, then it
can fetch just the necessary bits. It would print out previously seen
responses, which could manually be inspected.

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

## Docs

```
# Fetch up to 10 filings for 10 companies
$ python3 runall.py --num_filings 10 --min_i 50 --max_i 60 | tee source_data/stdout
# Group all the stdout logs by company (`cik`)
$ tail -n +2 source_data/stdout | jq -sc 'group_by(.cik) | .[]'
# For each company, group their logs by source file
$ tail -n +2 source_data/stdout | jq -sc 'group_by(.cik) | .[] | group_by(.source_file)'
```

Base files: stdout, s_and_p_500_list.html
Each company: {cik}.html
Each filing: {filing_page_hash}.html, {filing_hash}.html

Example stdout:
* Company
* Filing Page
* Filing
* Filing Page
* Filing
* Company
* Filing Page
* Filing
* Filing Page
* Filing Page
* Filing

Could group by `cik` across all three. If `search_url` is present, then it's a
Company row. If `sha256` is present, then it's a Filing row. Else, it's a
Filing Page row.

# Other Projects

* https://github.com/joeyism/py-edgar
* https://github.com/ryansmccoy/py-sec-edgar
* https://github.com/coyo8/sec-edgar
* https://github.com/andrewkittredge/financial_fundamentals
* https://github.com/lukerosiak/pysec
* https://github.com/edouardswiac/python-edgar
