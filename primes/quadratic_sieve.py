# quadratic_sieve.py
# Trevor Pottinger
# Fri Dec  6 22:57:30 PST 2019

from __future__ import division
from __future__ import print_function

import math
import sys

if sys.version_info >= (3, 3):
    from typing import (Dict, List, Optional, Tuple)
    greater_than_one = int
    greater_than_zero = int
    matrix = List[List[int]]
    maybe_prime = int
    nonnegative = int
    prime = int


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


def is_square(n):
    # type: (nonnegative) -> Tuple[nonnegative, bool]
    # Alternative: factor `n`, and check that all the prime powers are even.
    # This may not work if n is large enough
    root = int(math.floor(math.sqrt(n)))
    return (root, root ** 2 == n)


def fermats_method(n):
    # type: (nonnegative) -> Dict[maybe_prime, nonnegative]
    """Implements Fermat's method for factoring numbers. The idea is to find
    two numbers, a, and b, such that a ** 2 - b ** 2 == n. This form can be
    represented as (a - b) * (a + b), which therefore are two factors of n."""
    assert n >= 0, 'type violation, expected n >= 0'
    s = int(math.ceil(math.sqrt(n)))
    while True:
        d = s ** 2 - n
        d_root, d_is_square = is_square(d)
        if d_is_square:
            return {s - d_root: 1, s + d_root: 1}
        s += 1
    return {}


def slow_primes(n):
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


def slow_factors(n, primes=None):
    # type: (greater_than_one, Optional[List[prime]]) -> Tuple[bool, Dict[prime, greater_than_zero]]
    assert n > 1, 'type violation, expected n > 1'
    if primes is None:
        # generate all primes up to n, instead of up to ceil(sqrt(n)) b/c
        # we don't have logic to treat the remainder as a prime
        primes = slow_primes(n)
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
    ret = []
    # second pass for constructing vectors
    for row in rows:
        vector = [default for _ in range(num_keys)]
        for k in row:
            vector[key_to_col[k]] = row[k]
        ret.append(vector)
    return ret


def modular_row_reduction(matrix, mod):
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


def mat_extend(mat_a, mat_b):
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


def find_square_product(primes, ints):
    # type: (Optional[List[prime]], List[nonnegative]) -> Tuple[List[List[nonnegative]], List[nonnegative]]
    """Given a list of distinct integers, finds products of a subset of them
    that is a square and returns those products."""
    if primes is None:
        primes = slow_primes(max(ints))
    rows = []
    for i in ints:
        factored, factors = slow_factors(i, primes)
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
    res = modular_row_reduction(mat_extend(even_exponents, identity(n_ints)), 2)
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


def is_b_smooth(primes, i):
    # type: (List[prime], greater_than_one) -> bool
    for p in primes:
        while i % p == 0:
            i //= p
        if i == 1:
            return True
    return False


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


def is_quadratic_residue(p, n):
    # type: (prime, nonnegative) -> bool
    success, _modular_sqrt = tonelli(n, p)
    return success


def quadratic_sieve(n):
    # type: (greater_than_one) -> Dict[maybe_prime, greater_than_zero]
    assert n > 1, 'type violation, expected n > 1'
    # 1. choose smoothness bound B
    # TODO how do we pick B?
    B = 229 # 50th prime, 541  # 100th prime
    primes = [p for p in slow_primes(B) if is_quadratic_residue(p, n)]

    # 2. find numbers that are B smooth
    ints = []
    squares = []
    n_root = int(math.ceil(math.sqrt(n)))
    # TODO sieve! also, how do we pick the threshold?
    for i in range(n_root, n_root + 2 * len(primes) + 1):
        # `i ** 2 - n` should be equivalent to `i ** 2 % n` because of the
        # range that i is in, and therefore what `i ** 2` is in.
        square_mod_n = i ** 2 - n
        if not is_b_smooth(primes, square_mod_n):
            continue
        ints.append(i)
        squares.append(square_mod_n)
    # 3. factor numbers and generate exponent vectors
    # 4. apply some linear algebra
    # 5. now we have a ** 2 mod n == b ** 2 mod n
    indices, products = find_square_product(primes, squares)

    # 6. now we have (a - b) * (a + b) mod n == 0
    for i, product in enumerate(products):
        # TODO save the product decomposition to avoid slow_factors
        _, factors = slow_factors(product, primes)
        product_root = 1
        for prime in factors:
            product_root *= prime ** (factors[prime] // 2)
        a = 1
        a_s = []
        for j, bit in enumerate(indices[i]):
            if bit == 0:
                continue
            a_s.append(ints[j])
            a *= ints[j]
        # TODO should this be max(..) - min(..)?
        divisor = gcd(a - product_root, n)
        if divisor == 1 or divisor == n:
            continue
        # equivalent to `other_divisor = n // divisor`
        other_divisor = gcd(a + product_root, n)
        return {divisor: 1, other_divisor: 1}

    # No solution found!
    return {}
