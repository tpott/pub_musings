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


# TODO remove
def pi(n):
    # type: (nonnegative) -> nonnegative
    """Returns the number of primes less than or equal to n"""
    assert n >= 0, 'type violation, expected n >= 0'
    if n <= 1:
        return 0
    if n == 2:
        return 1
    # TODO
    return 0


def isSquare(n):
    # type: (nonnegative) -> Tuple[nonnegative, bool]
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
        d_root, is_square = isSquare(d)
        if is_square:
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
    # type: (greater_than_one, Optional[List[prime]]) -> Dict[prime, greater_than_zero]
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
            return ret
    return ret


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


def transpose(matrix):
    # type: (matrix) -> matrix
    nrows = len(matrix)
    assert nrows > 0, 'matrix must have at least one row'
    ncols = len(matrix[0])
    assert ncols > 0, 'matrix must have at least one column'
    cols = []
    for val in matrix[0]:
        cols.append([val])
    for row in matrix[1:]:
        for i, val in enumerate(row):
            cols[i].append(val)
    return cols


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


def find_square_product(ints):
    # type: (List[nonnegative]) -> List[nonnegative]
    """Given a list of distinct integers, finds products of a subset of them
    that is a square and returns those products."""
    primes = slow_primes(max(ints))
    rows = []
    for i in ints:
        # TODO limit factors to be B smooth
        rows.append(slow_factors(i, primes))
    exponents = vectorize(rows, default=0)
    even_exponents = []
    for vector in exponents:
        even_exponents.append([e % 2 for e in vector])

    # We might use less than all `primes`
    n_primes = len(exponents[0])
    n_ints = len(ints)
    res = modular_row_reduction(mat_extend(even_exponents, identity(n_ints)), 2)
    zero_row = [0 for _ in range(n_primes)]

    solutions = []
    for i in range(n_ints):
        if res[i][:n_primes] != zero_row:
            continue
        solution = 1
        for j, bit in enumerate(res[i][n_primes:]):
            if bit == 0:
                continue
            solution *= ints[j]
        solutions.append(solution)

    return solutions


def quadratic_sieve(n):
    # type: (greater_than_one) -> Dict[maybe_prime, greater_than_zero]
    assert n > 1, 'type violation, expected n > 1'
    # 1. choose smoothness bound B
    B = 59
    primes = slow_primes(B)
    # 2. find numbers that are B smooth
    # 3. factor numbers and generate exponent vectors
    # 4. apply some linear algebra
    # 5. now we have a ** 2 mod n == b ** 2 mod n
    # 6. now we have (a - b) * (a + b) mod n == 0

    # fermats_method
    # s = int(math.ceil(math.sqrt(n)))
    # while True:
        # d = s ** 2 - n
        # d_root, is_square = isSquare(d)
        # if is_square:
            # return {s - d_root: 1, s + d_root: 1}
        # s += 1

    # factor base
    # smooth relations
    # exponent matrix
    # gaussian elimination
    # solve congruences
    ret = {}  # type: Dict[maybe_prime, greater_than_zero]
    return ret
