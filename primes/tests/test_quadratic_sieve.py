# test_quadratic_sieve.py
# Trevor Pottinger
# Sat Dec  7 15:20:28 PST 2019

import unittest

from quadratic_sieve import (
    slow_factors,
    fermats_method,
    find_square_product,
    gcd,
    modular_row_reduction)


class TestQuadraticSieve(unittest.TestCase):

    def test_gcd(self):
        # type: () -> None
        def negativeGcd():
            return gcd(-2, 2)
        self.assertRaises(AssertionError, negativeGcd)
        self.assertEqual(gcd(1071, 462), 21)
        self.assertEqual(gcd(6, 7), 1)

    def test_fermats(self):
        # type: () -> None
        # Example from
        # https://blogs.msdn.microsoft.com/devdev/2006/06/19/factoring-large-numbers-with-quadratic-sieve/
        self.assertEqual(fermats_method(5959), {59: 1, 101: 1})

    def test_row_reduction(self):
        # type: () -> None
        self.assertEquals(modular_row_reduction([
            [1, 2, -1, -4],
            [2, 3, -1, -11],
            [-2, 0, -3, 22]], None), [
            [1, 0, 0, -8],
            [0, 1, 0, 1],
            [0, 0, 1, -2]])
        # From https://blogs.msdn.microsoft.com/devdev/2006/06/19/factoring-large-numbers-with-quadratic-sieve/
        self.assertEquals(modular_row_reduction([
            #10 24 35 52 54 78
            [1, 1, 0, 0, 1, 1],  # 2
            [0, 1, 0, 0, 1, 1],  # 3
            [1, 0, 1, 0, 0, 0],  # 5
            [0, 0, 1, 0, 0, 0],  # 7
            [0, 0, 0, 0, 0, 0],  # 11
            [0, 0, 0, 1, 0, 1]], 2), [
            [1, 0, 0, 0, 0, 0],
            [0, 1, 0, 0, 1, 1],
            [0, 0, 1, 0, 0, 0],
            [0, 0, 0, 1, 0, 1],
            [0, 0, 0, 0, 0, 0],
            [0, 0, 0, 0, 0, 0]])

    def test_factors(self):
        # type: () -> None
        def factorsOfOne():
            return slow_factors(1)
        self.assertRaises(AssertionError, factorsOfOne)
        self.assertEquals(slow_factors(10), {2: 1, 5: 1})
        self.assertEquals(slow_factors(24), {2: 3, 3: 1})
        self.assertEquals(slow_factors(35), {5: 1, 7: 1})
        self.assertEquals(slow_factors(52), {2: 2, 13: 1})
        self.assertEquals(slow_factors(54), {2: 1, 3: 3})
        self.assertEquals(slow_factors(78), {2: 1, 3: 1, 13: 1})

    def test_square_products(self):
        # type: () -> None
        # 97344 == 312 ** 2. Other answer: 1296 (36 ** 2)
        self.assertEquals(
            find_square_product([10, 24, 35, 52, 54, 78]),
            [1296, 219024])


if __name__ == '__main__':
    unittest.main()
