# quadratic_sieve.py
# Trevor Pottinger
# Fri Dec  6 22:57:30 PST 2019

from __future__ import division
from __future__ import print_function

import argparse
import math
import sys

if sys.version_info >= (3, 3):
    from typing import (Callable, Dict, List, Optional, Tuple, TypeVar)
    greater_than_one = int
    greater_than_zero = int
    matrix = List[List[int]]
    maybe_prime = int
    nonnegative = int
    prime = int
    K = TypeVar('K')
    V = TypeVar('V')


def gcd(a, b):
    # type: (nonnegative, nonnegative) -> nonnegative
    """Based on https://en.wikipedia.org/wiki/Euclidean_algorithm#Procedure"""
    assert a >= 0, 'type violation, expected a >= 0'
    assert b >= 0, 'type violation, expected b >= 0'
    if b == 0:
        return a
    if a < b:
        return gcd(b, a)
    return gcd(b, a % b)


def pollardsRho(n, g):
    # type: (nonnegative, Optional[Callable[[nonnegative, nonnegative], nonnegative]]) -> Optional[nonnegative]
    # https://en.wikipedia.org/wiki/Pollard%27s_rho_algorithm
    if g is None:
        g = lambda x, mod: (x ** 2 + 1) % mod
    x = 2
    y = 2
    d = 1
    while d == 1:
        x = g(x, n)
        y = g(g(y, n), n)
        d = gcd(abs(x - y), n)
    if d == n:
        return None
    return d


# Translated from https://rosettacode.org/wiki/Integer_roots#C
def intSqrt(n):
    # type: (nonnegative) -> nonnegative
    """Returns the square root of n, rounded down"""
    assert n >= 0, 'type violation, expected n >= 0'
    c = 1
    d = (1 + n) // 2
    e = (d + n // d) // 2
    while c != d and c != e:
        c = d
        d = e
        e = (e + n // e) // 2
    if d < e:
        return d
    return e


def isSquare(n):
    # type: (nonnegative) -> Tuple[nonnegative, bool]
    # Alternative: factor `n`, and check that all the prime powers are even.
    root = intSqrt(n)
    return (root, root * root == n)


def fermatsMethod(n):
    # type: (nonnegative) -> Dict[maybe_prime, nonnegative]
    """Implements Fermat's method for factoring numbers. The idea is to find
    two numbers, a, and b, such that a ** 2 - b ** 2 == n. This form can be
    represented as (a - b) * (a + b), which therefore are two factors of n."""
    assert n >= 0, 'type violation, expected n >= 0'
    s = int(math.ceil(math.sqrt(n)))
    while True:
        d = s ** 2 - n
        d_root, d_is_square = isSquare(d)
        if d_is_square:
            return {s - d_root: 1, s + d_root: 1}
        s += 1
    return {}


def slowPrimes(n):
    # type: (greater_than_one) -> List[prime]
    """Returns all primes less than or equal to n"""
    assert n > 1, 'type violation, expected n > 1'
    if n == 2:
        return [2]
    primes = [2]
    for i in range(3, n + 1, 2):
        is_prime = True
        for p in primes:
            if i % p == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
    return primes


def slowFactors(n, primes=None):
    # type: (greater_than_one, Optional[List[prime]]) -> Tuple[bool, Dict[prime, greater_than_zero]]
    assert n > 1, 'type violation, expected n > 1'
    if primes is None:
        # generate all primes up to n, instead of up to ceil(sqrt(n)) b/c
        # we don't have logic to treat the remainder as a prime
        primes = slowPrimes(n)
    ret = {}
    for p in primes:
        exponent = 0
        while n % p == 0:
            n //= p
            exponent += 1
        if exponent == 0:
            continue
        ret[p] = exponent
        if n == 1:
            return (True, ret)
    # Return false because we didn't finish factoring
    return (False, ret)


def vectorize(rows, default=None):
    # type: (List[Dict[K, V]], Optional[V]) -> List[List[V]]
    """Given a list of dicts, returns a list of vectors. Where each vector is
    of the same length, and each key in the input dicts corresponds to the same
    column in the output. If the input is from JSON, this is effectively
    "unsparsifying" the input."""
    key_to_col = {}  # type: Dict[K, nonnegative]
    num_keys = 0
    # first pass for key discovery
    for row in rows:
        for k in row:
            if k not in key_to_col:
                key_to_col[k] = num_keys
                num_keys += 1
    ret = []  # type: List[List[V]]
    # second pass for constructing vectors
    for row in rows:
        vector = [default for _ in range(num_keys)]
        for k in row:
            vector[key_to_col[k]] = row[k]
        ret.append(vector)
    return ret


def modularRowReduction(matrix, mod):
    # type: (matrix, Optional[greater_than_one]) -> matrix
    # Based on https://rosettacode.org/wiki/Reduced_row_echelon_form#Python
    assert mod is None or mod > 1, 'type violation, expected mod > 1'
    nrows = len(matrix)
    assert nrows > 0, 'matrix must have at least one row'
    ncols = len(matrix[0])
    assert ncols > 0, 'matrix must have at least one column'

    lead = 0
    for i in range(nrows):
        if lead >= ncols:
            break
        j = i

        # find the first row, j, where col `lead` is non-zero
        while matrix[j][lead] == 0:
            j += 1
            if j < nrows:
                continue
            # there are no non-zero values in column `lead`!
            j = i
            lead += 1
            if lead == ncols:
                return matrix

        # swap rows i and j
        matrix[i], matrix[j] = matrix[j], matrix[i]
        lead_value = matrix[i][lead]
        # divide the row by lead_value so the lead value becomes 1
        matrix[i] = [val // lead_value for val in matrix[i]]

        # cancel out column `lead` for each row j, s.t. j != i
        for j in range(nrows):
            if i == j:
                continue
            lead_value = matrix[j][lead]
            row = []
            for j_value, i_value in zip(matrix[i], matrix[j]):
                if mod is not None:
                    row.append((i_value - lead_value * j_value) % mod)
                else:
                    row.append(i_value - lead_value * j_value)
            matrix[j] = row
        lead += 1

    return matrix


def matstr(mat):
    # type: (matrix) -> str
    return  "[" + ",\n ".join(map(str, mat)) + "]"


def matExtend(mat_a, mat_b):
    # type: (matrix, matrix) -> matrix
    nrows = len(mat_a)
    assert nrows > 0, 'mat_a must have at least one row'
    assert nrows == len(mat_b), 'mat_b must have the same nrows as mat_a'
    ncols = len(mat_a[0]) + len(mat_b[0])
    assert ncols > 0, 'mat_a + mat_b must have at least one column'
    ret = []
    for i in range(nrows):
        ret.append(mat_a[i] + mat_b[i])
    return ret


def identity(nrows, ncols=None):
    # type: (greater_than_zero, Optional[greater_than_zero]) -> matrix
    assert nrows > 0, 'type violation, expected nrows > 0'
    assert ncols is None or ncols > 0, 'type violation, expected ncols > 0'
    if ncols is None:
        ncols = nrows
    ret = []
    for i in range(nrows):
        ret.append([0 for _ in range(i)])
        if i == ncols:
            continue
        ret[i].append(1)
        ret[i].extend([0 for _ in range(ncols - i - 1)])
    return ret


def findSquareProduct(primes, ints):
    # type: (Optional[List[prime]], List[nonnegative]) -> Tuple[List[List[nonnegative]], List[nonnegative]]
    """Given a list of distinct integers, finds products of a subset of them
    that is a square and returns those products."""
    if primes is None:
        primes = slowPrimes(max(ints))
    rows = []
    for i in ints:
        factored, factors = slowFactors(i, primes)
        if not factored:
            continue
        rows.append(factors)
    exponents = vectorize(rows, default=0)
    even_exponents = []
    for vector in exponents:
        even_exponents.append([e % 2 for e in vector])

    # We might use less than all `primes`, as well as less than all ints
    n_primes = len(even_exponents[0])
    n_ints = len(even_exponents)
    res = modularRowReduction(matExtend(even_exponents, identity(n_ints)), 2)
    zero_row = [0 for _ in range(n_primes)]

    indices = []
    solutions = []
    for i in range(n_ints):
        if res[i][:n_primes] != zero_row:
            continue
        solution = 1
        for j, bit in enumerate(res[i][n_primes:]):
            if bit == 0:
                continue
            solution *= ints[j]
        indices.append(res[i][n_primes:])
        solutions.append(solution)

    return (indices, solutions)


def isBSmooth(primes, i):
    # type: (List[prime], greater_than_one) -> bool
    for p in primes:
        while i % p == 0:
            i //= p
        if i == 1:
            return True
    return False


def bSmoothList(primes, n, count):
    # type: (List[prime], greater_than_one, greater_than_one) -> Tuple[List[greater_than_one], List[greater_than_one]]
    """This isn't a real sieve. It's just really simple. Given an int, n,
    and the number of smooth numbers to try getting, count, derive the list
    of smooth ints."""
    n_root = intSqrt(n)
    # This ensures we take the ceiling of the sqrt
    if n_root * n_root < n:
        n_root += 1
    ints = []
    squares = []
    for i in range(n_root, n_root + count + 1):
        # `i ** 2 - n` should be equivalent to `i ** 2 % n` because of the
        # range that i is in, and therefore what `i ** 2` is in.
        square_mod_n = i ** 2 - n
        if not isBSmooth(primes, square_mod_n):
            continue
        ints.append(i)
        squares.append(square_mod_n)
    return (ints, squares)


# From https://rosettacode.org/wiki/Tonelli-Shanks_algorithm#Python
def legendre(a, p):
    return pow(a, (p - 1) // 2, p)


# From https://rosettacode.org/wiki/Tonelli-Shanks_algorithm#Python
def tonelli(n, p):
    if not legendre(n, p) == 1:
        # previously this asserted "not a square (mod p)"
        return (False, None)
    q = p - 1
    s = 0
    while q % 2 == 0:
        q //= 2
        s += 1
    if s == 1:
        return (True, pow(n, (p + 1) // 4, p))
    z = 2
    for i in range(2, p):
        z = i
        if p - 1 == legendre(z, p):
            break
    c = pow(z, q, p)
    r = pow(n, (q + 1) // 2, p)
    t = pow(n, q, p)
    m = s
    t2 = 0
    while (t - 1) % p != 0:
        t2 = (t * t) % p
        for i in range(1, m):
            if (t2 - 1) % p == 0:
                break
            t2 = (t2 * t2) % p
        b = pow(c, 1 << (m - i - 1), p)
        r = (r * b) % p
        c = (b * b) % p
        t = (t * c) % p
        m = i
    return (True, r)


def isQuadraticResidue(p, n):
    # type: (prime, nonnegative) -> bool
    success, _modular_sqrt = tonelli(n, p)
    return success


def quadraticSieve(n, interval_mult=2, max_prime=229, verbosity=0):
    # type: (greater_than_one, nonnegative, greater_than_one, nonnegative) -> Dict[maybe_prime, greater_than_zero]
    assert n > 1, 'type violation, expected n > 1'
    assert interval_mult > 0, 'type violation, expected interval_mult > 0'
    # 1. choose smoothness bound B
    # TODO how do we pick B?
    B = max_prime
    # `primes` here is commonly referred to as a "factor_base" in other
    # implementations of the quadratic sieve
    primes = [p for p in slowPrimes(B) if isQuadraticResidue(p, n)]
    if verbosity > 0:
        print('Found %d primes less than or equal to %d, max is %d' % (len(primes), max_prime, primes[-1]))
    if verbosity > 2:
        print('Primes: %s' % (str(primes)))

    # 2. find numbers that are B smooth
    # TODO how do we pick the threshold for bSmoothSieve?
    ints, squares = bSmoothList(primes, n, interval_mult * len(primes))
    if verbosity > 0:
        print('Found %d b-smooth squares out of %d ints' % (len(squares), interval_mult * len(primes)))
    if verbosity > 2:
        print('Ints: %s' % (str(ints)))

    # 3. factor numbers and generate exponent vectors
    # 4. apply some linear algebra
    indices, products = findSquareProduct(primes, squares)
    if verbosity > 0:
        print('Found %d solutions to the mod-2 matrix' % (len(products)))

    for i, product in enumerate(products):
        used_ints = None  # type: Optional[List[nonnegative]]
        if verbosity > 1:
            used_ints = []
            print('Checking gcd of %d' % (product))
        # TODO move this computation to findSquareProduct
        a = 1
        for j, bit in enumerate(indices[i]):
            if bit == 0:
                continue
            a *= ints[j]
            if used_ints is not None:
                used_ints.append(ints[j])
        if used_ints is not None:
            print('Used ints: %s' % (str(used_ints)))
        product_root = intSqrt(product)
        # 5. now we have a ** 2 mod n == b ** 2 mod n
        # TODO should this be max(..) - min(..)?
        divisor = gcd(a - product_root, n)
        if divisor == 1 or divisor == n:
            continue
        # equivalent to `other_divisor = n // divisor`
        other_divisor = gcd(a + product_root, n)
        # 6. now we have (x - y) * (x + y) mod n == 0
        if verbosity > 1:
            x = (divisor + other_divisor) / 2
            y = (divisor - other_divisor) / 2
            print('%d = x ** 2 - y ** 2 = (x - y) * (x + y), x = %d, y = %d' % (n, x, y))
        return {divisor: 1, other_divisor: 1}

    # No solution found!
    return {}


def main():
    # type: () -> None
    parser = argparse.ArgumentParser(description='Factors a number using ' +
        'a quadratic sieve (kinda)')
    parser.add_argument('n', type=int, help='The number to factor')
    # TODO add help for mult (aka interval_mult)
    parser.add_argument('--mult', type=int, default=1)
    parser.add_argument('--max_prime', type=int, default=229, help='The ' +
        'max prime number to use for deriving B-smoothness. 229 is the ' +
        '50th prime, and 541 is the 100th.')
    parser.add_argument('-v', '--verbose', default=0, action='count')
    args = parser.parse_args()

    assert args.n > 3, 'expected n > 3'
    assert args.mult >= 1, 'expected mult >= 1'
    assert args.max_prime >= 2, 'expected max_prime >= 2'

    print(quadraticSieve(args.n, args.mult, args.max_prime, args.verbose))


if __name__ == '__main__':
    main()
