# easy_primes.py

A simple script for generating a list of primes. Has the additional
functionality of using said list to factor a range of numbers.

Example usage:
```
# Derives many primes, which are then used to factor every number from 2 to
# 1000, inclusive. Then prints the factors in JSON, where the key is the prime
# and the value is the power of that prime. The other pipes are for pretty
# printing the factors in a TSV.
$ pypy primes/easy_primes.py --factor-to 1000 --json-factor 2>/dev/null
  | mlr --ijson --otsv unsparsify --fill-with -
  | python tsv_transpose.py 2>/dev/null
  | sort -n
  | python tsv_transpose.py 2>/dev/null
  > primes/first_1k.tsv

# To get each number and it's prime decomposition in the --json-factor format,
# this will first get the decompositions and then pipe the results through `jq`
# to format the TSV output
$ pypy primes/easy_primes.py --factor-to 1000000 --json-factors 2>/dev/null
  | jq -cr '[
    (to_entries | map(pow((.key | tonumber); .value)) | reduce .[] as $f (1; . * $f)),
    length,
    (length | . == 1) and (to_entries | .[0].value == 1),
    (. | tojson)] | @tsv'
```

# Links

* [primenet](https://www.mersenne.org/primenet/)'s exponent status
* [OEIS list](https://oeis.org/A000043/list) of Mersenne primes
* [RSA240 factored](https://lists.gforge.inria.fr/pipermail/cado-nfs-discuss/2019-December/001139.html) and brief explanation
