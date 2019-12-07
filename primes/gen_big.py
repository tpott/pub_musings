# gen_big.py
# Trevor Pottinger
# Sat Nov 23 13:55:49 PST 2019

from __future__ import division
from __future__ import print_function

import argparse
import json
import math
import random
import sys
import time

if sys.version_info >= (3, 3):
    uint = int
    greater_than_one = int


def millerRabin(k, n):
    # type: (greater_than_one, greater_than_one) -> bool
    """Run k tests to check if n is a prime. Each test picks a random int from
    the range [2, n), and checks if `a ** (n - 1) % n == 1`. Note that
    Carmichael numbers are composite numbers that will pass all their tests."""
    assert k > 1, 'type violation, expected k > 1'
    assert n > 1, 'type violation, expected n > 1'
    for _ in range(k):
        a = random.randrange(2, n)
        if pow(a, n - 1, n) != 1:
            return False
    return True


def isPowerOfTwo(n):
    # type: (greater_than_one) -> bool
    return (n & (n - 1)) == 0


def lucasLehmer(n):
    # type: (greater_than_one) -> bool
    """Based on https://en.wikipedia.org/wiki/Lucas%E2%80%93Lehmer_primality_test"""
    assert n > 3, 'lucasLehmer only valid for n > 3'
    assert isPowerOfTwo(n + 1), 'lucasLehmer only valid for 2 ** p - 1'
    p = int(math.floor(math.log(n + 1, 2)))
    s = 4
    for _ in range(p - 2):
        s = pow(s, 2, n) - 2
    return s == 0


def isPrime(n):
    # type: (greater_than_one) -> bool
    assert n > 1, 'type violation, expected n > 1'
    if n == 2:
        return True
    elif n in set([3, 5, 7, 11, 13, 17, 19, 23, 29]):
        return True

    # TODO pick the number of iterations to run miller-rabin
    elif millerRabin(200, n):
        if not isPowerOfTwo(n + 1) and n > 3:
            # Lucas-Lehmer only works on `n` of the form `2 ** p - 1`,
            # for `p` > 2
            return True
        return lucasLehmer(n)

    # TODO try an AKS test, else we may incorrectly return False
    return False


def main():
    # type: () -> None
    parser = argparse.ArgumentParser(description='Generates a continuous ' +
        'stream of big primes')
    parser.add_argument('n_bits', type=int, help='The number of bits the ' +
        'primes should be')
    parser.add_argument('--sleep', type=int, default=500, help='The number ' +
        'of milliseconds to sleep between each number. Defaults to 500')
    args = parser.parse_args()

    assert args.n_bits > 1, 'Must have at least two bits'
    assert args.sleep >= 0, '--sleep must be non-negative'

    running = True
    while running:
        is_prime = False
        while not is_prime:
            n = random.getrandbits(args.n_bits)  # type: uint
            if n <= 1 or not isPrime(n):
                continue
            is_prime = True
            print(json.dumps({
                'n': n,
                'log2': math.log(n, 2),
            }))
        time.sleep(args.sleep / 1000)


if __name__ == '__main__':
    main()
