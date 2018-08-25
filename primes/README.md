# easy_primes.py

A simple script for generating a list of primes. Has the additional
functionality of using said list to factor a range of numbers.

Example usage:
```
# Derives many primes, which are then used to factor every number from 2 to
# 1000, inclusive. Then prints the factors in JSON, where the key is the prime
# and the value is the power of that prime. The other pipes are for pretty
# printing the factors in a TSV.
$ pypy primes/easy_primes.py --factor-from 2 --factor-to 1000 --json-factor 2>/dev/null
  | mlr --ijson --otsv unsparsify --fill-with -
  | python tsv_transpose.py 2>/dev/null
  | sort -n
  | python tsv_transpose.py 2>/dev/null
  > primes/first_1k.tsv
```
