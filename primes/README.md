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

# rosetta.py

This is an implementation that has a similar, but not exactly the same, interface
as to easy_primes.py. The implementation is from rosettacode.org, and way faster
than easy_primes.py

# gen_big.py

A simple script for generating random, large primes. It uses repeated
Miller-Rabin tests to ensure the random integers are prime.

```
$ python gen_big.py 510 > probable_primes
$ python gen_big.py 20
{"log2": 19.44626955927234, "n": 714349}
{"log2": 19.59390351827008, "n": 791321}
```

# quadratic_sieve.py

```
$ calc '714349 * 791321'
565279365029
$ python quadratic_sieve.py 565279365029 --mult 10000
{791321L: 1, 714349L: 1}
$ python quadratic_sieve.py 565279365029 --max_prime 541 --mult 100
{791321L: 1, 714349L: 1}
$ python -m cProfile -o restats quadratic_sieve.py 10925004257145179 --mult 1000000
$ python -m pstats restats
restats% sort tottime
restats% stats 20
```

I learned the most expensive part of the quadratic sieve was generating the
list of b-smooth numbers. I didn't implement a proper sieve, but still managed
to enumerate numbers at a rate ~200K numbers/second.

Values to iterate on:
* max prime
* sieve interval size
* how may extra ints more than primes

Several of the implementations below will keep generating more b-smooth ints
until they have count(ints) >= count(primes) + some number (typically just 1).
The thinking is that a matrix with one more row than columns will have 50%
odds of being a non-trivial solution.

I wish there was a better way for determining these thresholds. If there was
an incremental row reduction, then that would allow for looking for solutions
after each new b-smooth int is found. And if we needed to add a new prime,
then that would mean some previously non-b-smooth ints would become b-smooth.

I'm also curious how Quadratic Sieve handles semi-primes where the factors are
"nearer" to each other? And how does "nearer" change as the size of the numbers
increases?

```
# default max_prime=229
$ time python quadratic_sieve.py 10925004257145179 --mult 1000000
{111158357L: 1, 98283247L: 1}
real    2m12.167s

$ time python quadratic_sieve.py 10925004257145179 --mult 100000
{}
real    0m13.088s

$ time python quadratic_sieve.py 10925004257145179 --mult 100000 --max_prime 541
{111158357L: 1, 98283247L: 1}
real    0m52.122s

$ time python quadratic_sieve.py 10925004257145179 --mult 10000 --max_prime 541
{111158357L: 1, 98283247L: 1}
real    0m4.819s

$ time python quadratic_sieve.py 10925004257145179 --mult 1000 --max_prime 541
{}
real    0m0.533s

$ time python quadratic_sieve.py 10925004257145179 --mult 1000 --max_prime 1583
{111158357L: 1, 98283247L: 1}
real    0m14.857s

$ time python quadratic_sieve.py 10925004257145179 --mult 100 --max_prime 1583
{}
real    0m0.564s
```

* What value of b should be used?
* How many primes are quadratic residues of n?
* How many ints to search through?
* How many b-smooth ints are found?

The approach expects `n = small * big`, such that `2 < small < big < sqrt(n)`
and if we look for `x` and `y` such that `n = (x - y) * (x + y)` then that
implies `n = x ** 2 - y ** 2` and `small = x - y` and `big = x + y`. Which then
implies `x = small + y`, `big = small + 2 * y`, and finally
`y = (big - small) / 2`, `x = (big + small) / 2`.

# Links

* [primenet](https://www.mersenne.org/primenet/)'s exponent status
* [OEIS list](https://oeis.org/A000043/list) of Mersenne primes
* [RSA240 factored](https://lists.gforge.inria.fr/pipermail/cado-nfs-discuss/2019-December/001139.html) and brief explanation
* https://github.com/NachiketUN/Quadratic-Sieve-Algorithm/blob/master/src/main.py and
* https://github.com/Maosef/Quadratic-Sieve/blob/master/Quadratic%20Sieve.py are very similar
* https://github.com/skollmann/PyFactorise/blob/master/factorise.py is more unique
* https://github.com/cramppet/quadratic-sieve/blob/master/QS.py is pretty minimal
* https://github.com/alexbers/quadratic_sieve/blob/master/quadratic_sieve.py has some test semiprimes
