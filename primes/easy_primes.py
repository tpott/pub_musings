# easy_primes.py
# Trevor Pottinger
# Mon Oct 12 13:07:13 PDT 2015
# Sun Feb  4 23:33:26 PST 2018

from __future__ import print_function
from __future__ import division

import argparse
import functools
import json
import math
import random
import sys

if sys.version_info.major >= 3:
    from typing import (Dict, List, Optional, Tuple)


# For more random facts and info about prime numbers, checkout
# https://en.wikipedia.org/wiki/Prime_number and https://oeis.org/A000040

def getPrimes(known_primes=None, max_num=2**12, plus_one=False):
    # type: (Optional[List[int]], int, bool) -> List[int]
    """This is a really simple method for finding primes"""
    if known_primes is None:
        known_primes = [2]
    assert known_primes[0] == 2, '2 should always be the first prime'
    assert known_primes == sorted(known_primes), 'please sort your primes'

    # Why did I think this was to "avoid side effects" again?
    known_primes = list(known_primes)
    for i in range(3, max_num + 1, 2):
        is_prime = True
        for j in range(len(known_primes)):
            if i % known_primes[j] == 0:
                is_prime = False
                break
        if not is_prime:
            continue
        known_primes.append(i)

    if not plus_one:
        return known_primes

    found_one_more = False
    i = max_num - (max_num % 2) + 1
    while not found_one_more:
        is_prime = True
        for j in range(len(known_primes)):
            if i % known_primes[j] == 0:
                is_prime = False
                break
        if is_prime:
            found_one_more = True
            known_primes.append(i)
        i += 2

    return known_primes


def getPrimesWithSkips(primes=None, max_num=2**12, plus_one=False):
    # type: (Optional[List[int]], int, bool) -> List[int]
    """This is builds up a sieve, then uses the distances between potential
    primes to skip more numbers than just two at a time. Its a lot like
    wheel factorization, but instead we're using the "wheel" to efficiently
    generate prime candidates and then run through simple trial division."""
    if primes is None:
        primes = [2, 3, 5]
    assert primes[0] == 2, '2 should always be the first prime'
    assert len(primes) >= 2, 'primes too short, try getPrimes()'
    assert primes == sorted(primes), 'please sort your primes'

    # For 2,3,5 this is 30; for 2,3,5,7 it's 210; for 2,3,5,7,11 it's 2310.
    # This grows very fast. Adding the next few primes yields 30030, 510510,
    # 9699690, 223092870 and 6469693230. Yup, the product of the first ten
    # primes is almost 6.5B. Holding the `skips` list in memory at that point
    # is quite intense. Checkout https://oeis.org/A002110 for more info
    start_skipping = 1
    for i in range(len(primes)):
        start_skipping *= primes[i]
    # For 2,3,5 this yields [1,7,11,13,17,19,23,29]
    skip_steps = []
    for i in range(start_skipping):
        is_multiple = False
        for p in primes:
            if i % p == 0:
                is_multiple = True
                break
        if not is_multiple:
            skip_steps.append(i)

    # For 2,3,5 this yields [6,4,2,4,2,4,6,2]
    skips = []
    for i in range(1, len(skip_steps)):
        skips.append(skip_steps[i] - skip_steps[i-1])
    # This is because -1 mod start_skipping is two away from 1 mod
    skips.append(2)

    i = skips[0] + 1
    skip_i = 1
    while i <= max_num:
        is_prime = True
        for p in primes:
            if i % p == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
        i += skips[skip_i]
        skip_i = (skip_i + 1) % len(skips)

    if not plus_one:
        return primes

    found_one_more = False
    i = max_num - (max_num % 2) + 1
    while not found_one_more:
        is_prime = True
        for p in primes:
            if i % p == 0:
                is_prime = False
                break
        if is_prime:
            found_one_more = True
            primes.append(i)
        i += skips[skip_i]
        skip_i = (skip_i + 1) % len(skips)

    return primes


def getFactorization(primes, n):
    # type: (List[int], int) -> Tuple[bool, Dict[int, int]]
    assert n > 1, 'n must be greater than 1'
    is_prime = False
    # This is just another way for computing an upper bound on the sqrt of n
    natural_log = math.log(n, 2) / 2
    max_num = int(math.ceil(2 ** natural_log))
    print("Searching for factors of %d up to %d (except not really)" % (
        n, max_num), file=sys.stderr)

    factors = {}
    for i in range(len(primes)):
        # TODO state this as n = a * primes[i] + b, and if b != 0 ...
        # See: Euclidean algorithm. Then factors[primes[i]] = a ?
        if n % primes[i] != 0:
            continue
        print("Found factor %d" % (primes[i]), file=sys.stderr)
        factors[primes[i]] = 1
        n = n // primes[i]
        while n % primes[i] == 0:
            factors[primes[i]] += 1
            n = n // primes[i]
        if n == 1:
            break

    if n != 1:
        # Re-compute the max_num. For composites like 308634695716 =
        # 2^2 * 77158673929, the largest number necessary to check for
        # 77158673929 is 277774. So as long as we checked all primes greater
        # than that, then the last value of n must also be prime.
        natural_log = math.log(n, 2) / 2
        max_num = int(math.ceil(2 ** natural_log))

    if primes[-1] >= max_num and n != 1:
        # It can be proven that n must be prime by contradiction. Assume that
        # n is not prime, then it must be a composite. If it's a composite,
        # then there must be two or more prime factors. Assume the min factor
        # is less than max_num; but this can not be, because the factor would
        # have been found above (and could not be unfound). So the min factor
        # must be greater than or equal to max_num, implying all unfound
        # factors must be greater than or equal to max_num. But this can not
        # be, because even if there are just two factors equal to max_num,
        # then n must be greater than the original n. Thus, the current n
        # cannot be composite, and therefore must be prime.
        print("Previously unfound/unknown prime: %d" % (n), file=sys.stderr)
        factors[n] = 1
        n = 1
    elif n != 1:
        print("FAIL partial factors. Final n: %d / %d %d" % (n, max_num,
            primes[-1]), file=sys.stderr)
        return (False, factors)

    return (True, factors)


def fermatPrimeTest(p, n):
    # type: (int, int) -> Tuple[bool, Optional[int]]
    """If we find an int that proves p is composite, then we'll return it"""
    assert n > 1, 'number of tests, n, must be positive'
    if p == 2 or p == 3:
        return (True, None)
    assert p > 1, 'please only test numbers greater than 1'

    for _ in range(n):
        a = random.randint(2, p - 2)
        m = pow(a, p - 1, p)
        if m != 1:
            return (False, a)

    return (True, None)


def lucasPrimeTest(p, n=100, primes=None):
    # type: (int, int, Optional[List[int]]) -> Tuple[bool, Optional[int]]
    """If we find an int that proves p is prime, then we'll return it"""
    if p == 2 or p == 3:
        return (True, None)
    assert p > 1, 'please only test numbers greater than 1'

    if primes is None:
        primes = getPrimes()
    # Ugh, I forgot that getFactorization prints to stderr
    success, factors = getFactorization(primes, p - 1)
    # What to do when success is False?
    divs = list(map(lambda q: (p - 1) // q, factors.keys()))

    for _ in range(n):
        a = random.randint(2, p - 2)
        if pow(a, p - 1, p) != 1:
            return (False, None)  # Should we include `a` in the return here?

        all_not_one = True
        for d in divs:
            if pow(a, d, p) == 1:
                all_not_one = False
                break
        if all_not_one:
            return (True, a)

    # TODO wut to explain here?
    return (False, None)


def prettyPrint(factors):
    # type: (Dict[int, int]) -> str
    assert len(factors) > 0
    keys = list(factors.keys())
    keys.sort(reverse=False)  # ascending
    if factors[keys[0]] == 1:
        eq = "%d" % (keys[0])
    else:
        eq = "%d^%d" % (keys[0], factors[keys[0]])
    for k in keys[1:]:
        assert factors[k] >= 1
        if factors[k] == 1:
            eq = eq + " * %d" % k
        else:
            eq = eq + " * %d^%d" % (k, factors[k])
    return eq


def main():
    # type: () -> int
    parser = argparse.ArgumentParser(description='Easily generate primes')
    # Or list primes less than args.less_than?
    parser.add_argument('-l', '--list', action='store_true',
        help='List the first 1000 primes')
    parser.add_argument('-t', '--less-than', type=int, action='store',
        help='Find primes less than N')
    parser.add_argument('-f', '--factor', type=int, action='store',
        help='The number to try factoring based on primes --less-than N')
    parser.add_argument('--factor-from', type=int, action='store',
        help='The inclusive starting range for factoring')
    parser.add_argument('--factor-to', type=int, action='store',
        help='The inclusive ending range for factoring')
    parser.add_argument('-j', '--json-factors', action='store_true',
        help='Enables JSON printing of factors')
    parser.add_argument('-s', action='store_true', help='With skips')

    args = parser.parse_args()

    factoring = False
    factor_from, factor_to = 0, 0
    if args.factor is not None:
        assert args.factor_from is None, 'Can\'t factor and factor-from'
        assert args.factor_to is None, 'Can\'t factor and factor-to'
        assert not args.list, 'Can\'t list and factor'
        assert args.factor >= 2
        factoring = True
        factor_from, factor_to = args.factor, args.factor
    elif args.factor_from is not None:
        assert args.factor_to is not None, 'Can\'t factor-from and not factor-to'
        assert not args.list, 'Can\'t list and factor-from'
        assert args.factor_from >= 2
        assert args.factor_to >= args.factor_from
        factoring = True
        factor_from, factor_to = args.factor_from, args.factor_to
    elif args.factor_to is not None:
        assert not args.list, 'Can\'t list and factor-to'
        assert args.factor_to >= 2
        factoring = True
        factor_from, factor_to = 2, args.factor_to

    if args.json_factors:
        assert factoring, 'Can\'t print JSON factors if not factoring'

    # All primes must be less than or equal to max_num. We don't make this
    # a default in the ArgumentParser because we want the default for
    # --list to be different than for --factor
    max_num = args.less_than
    if max_num is None and factoring:
        # This is just another way for computing an upper bound on the sqrt of n
        natural_log = math.log(factor_to, 2) / 2
        max_num = int(math.ceil(2 ** natural_log))
    elif max_num is None:
        # 7919 is the 1000th prime number. Not helpful here, but 104729 is the
        # 10,000th prime number and 1299709 is the 100,000th. See list of first
        # 10K primes at https://oeis.org/A000040/b000040.txt and first 100K
        # primes at https://oeis.org/A000040/a000040.txt . For similar output
        # try `python easy_primes.py -l -t 104729 | awk '{ print NR " " $0 }'`
        max_num = 7919

    print("Searching for primes less than or equal to %d" % (max_num),
        file=sys.stderr)
    if args.s:
        primes = getPrimesWithSkips([2, 3, 5], max_num, factoring)
    else:
        primes = getPrimes([2], max_num, factoring)

    if args.list:
        for p in primes:
            print("%d" % (p))
        return 0

    print("Largest prime %d" % (primes[-1]), file=sys.stderr)
    if factor_from == 0:
        return 0  # nothing to compute?

    for i in range(factor_from, factor_to + 1):
        success, factors = getFactorization(primes, i)
        if not success:
            continue  # print error message?
        if args.json_factors:
            print("%s" % (json.dumps(factors, sort_keys=True)))
            continue
        print("Successfully found factors: %s" % (prettyPrint(factors)))
    return 0


if __name__ == '__main__':
    ret = main()
    sys.exit(ret)
