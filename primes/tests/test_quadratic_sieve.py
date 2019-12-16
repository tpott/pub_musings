# test_quadratic_sieve.py
# Trevor Pottinger
# Sat Dec  7 15:20:28 PST 2019

import unittest

from quadratic_sieve import (
    fermatsMethod,
    findSquareProduct,
    gcd,
    isQuadraticResidue,
    modularRowReduction,
    pollardsRho,
    quadraticSieve,
    slowFactors)


class TestQuadraticSieve(unittest.TestCase):

    def test_gcd(self):
        # type: () -> None
        def negativeGcd():
            return gcd(-2, 2)
        self.assertRaises(AssertionError, negativeGcd)
        self.assertEqual(gcd(1071, 462), 21)
        self.assertEqual(gcd(6, 7), 1)

    def test_pollards(self):
        # type: () -> None
        self.assertEqual(pollardsRho(8051, None), 97)
        self.assertEqual(pollardsRho(10403, None), 101)

    def test_fermats(self):
        # type: () -> None
        # Example from
        # https://blogs.msdn.microsoft.com/devdev/2006/06/19/factoring-large-numbers-with-quadratic-sieve/
        self.assertEqual(fermatsMethod(5959), {59: 1, 101: 1})

    def test_row_reduction(self):
        # type: () -> None
        self.assertEqual(modularRowReduction([
            [1, 2, -1, -4],
            [2, 3, -1, -11],
            [-2, 0, -3, 22]], None), [
            [1, 0, 0, -8],
            [0, 1, 0, 1],
            [0, 0, 1, -2]])
        # From https://blogs.msdn.microsoft.com/devdev/2006/06/19/factoring-large-numbers-with-quadratic-sieve/
        self.assertEqual(modularRowReduction([
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
            return slowFactors(1)
        self.assertRaises(AssertionError, factorsOfOne)
        # TODO test passing in primes that aren't enough to factor
        self.assertEqual(slowFactors(10)[1], {2: 1, 5: 1})
        self.assertEqual(slowFactors(24)[1], {2: 3, 3: 1})
        self.assertEqual(slowFactors(35)[1], {5: 1, 7: 1})
        self.assertEqual(slowFactors(52)[1], {2: 2, 13: 1})
        self.assertEqual(slowFactors(54)[1], {2: 1, 3: 3})
        self.assertEqual(slowFactors(78)[1], {2: 1, 3: 1, 13: 1})

    def test_square_products(self):
        # type: () -> None
        # 1296 == 36 ** 2 == 24 * 54
        # 219024 == 468 ** 2 == 52 * 54 * 78
        # Note there's also 97344 == 312 **2 == 24 * 52 * 78, but the
        # first two solutions form a basis for the matrix we use, and
        # so it is a redundant solution.
        _, products = findSquareProduct(None, [10, 24, 35, 52, 54, 78])
        self.assertEqual(products, [1296, 219024])

    def test_quadratic_residue(self):
        # type: () -> None
        # 15347 is from https://en.wikipedia.org/wiki/Quadratic_sieve
        self.assertTrue(isQuadraticResidue(2, 15347))
        self.assertFalse(isQuadraticResidue(3, 15347))
        self.assertFalse(isQuadraticResidue(5, 15347))
        self.assertFalse(isQuadraticResidue(7, 15347))
        self.assertFalse(isQuadraticResidue(11, 15347))
        self.assertFalse(isQuadraticResidue(13, 15347))
        self.assertTrue(isQuadraticResidue(17, 15347))
        self.assertFalse(isQuadraticResidue(19, 15347))
        self.assertTrue(isQuadraticResidue(23, 15347))
        self.assertTrue(isQuadraticResidue(29, 15347))
        # 90283 is from https://blogs.msdn.microsoft.com/devdev/2006/06/19/factoring-large-numbers-with-quadratic-sieve/
        self.assertTrue(isQuadraticResidue(2, 90283))
        self.assertTrue(isQuadraticResidue(3, 90283))
        self.assertFalse(isQuadraticResidue(5, 90283))
        self.assertTrue(isQuadraticResidue(7, 90283))

    def test_quadratic_sieve(self):
        # type: () -> None
        self.assertEqual(quadraticSieve(5959), {59: 1, 101: 1})
        self.assertEqual(quadraticSieve(90283), {137: 1, 659: 1})
        self.assertEqual(quadraticSieve(1811706971, 14), {17299: 1, 104729: 1})


if __name__ == '__main__':
    unittest.main()
