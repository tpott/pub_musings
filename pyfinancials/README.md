# pyfinancials

Lets see how far we can get without any external deps

# Goal

Deterministically fetch a bunch of source data, do some transformation. And
then the final data should be easily presentable.

# Open Questions

How to parse HTML without a complex HTML parsing library?

# Notes

  # Download from https://finance.yahoo.com/quote/%5EGSPC/history?period1=1413529200&period2=1571295600&interval=1d&filter=history&frequency=1d
  ~/Github/miller/c/mlr --icsv --from source_data/\^GSPC.csv --ojson cat | jq '.["Adj Close"]'
