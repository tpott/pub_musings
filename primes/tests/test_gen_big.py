# test_gen_big.py
# Trevor Pottinger
# Fri Dec  6 21:23:27 PST 2019

import unittest

from gen_big import (isPowerOfTwo, isPrime, lucasLehmer, millerRabin)


class TestPrimeGen(unittest.TestCase):

    def test_isPowerOfTwo(self):
        # type: () -> None
        self.assertTrue(isPowerOfTwo(2**60))
        self.assertFalse(isPowerOfTwo(2**60 + 1))

    def test_isPrime(self):
        # type: () -> None
        def isOnePrime():
            return isPrime(1)
        self.assertRaises(AssertionError, isOnePrime)
        self.assertTrue(isPrime(2))
        self.assertTrue(isPrime(3))
        self.assertFalse(isPrime(4))
        self.assertTrue(isPrime(5))
        self.assertFalse(isPrime(6))
        self.assertTrue(isPrime(11))
        self.assertTrue(isPrime(29))
        self.assertFalse(isPrime(7918))
        self.assertTrue(isPrime(7919))

    def test_lucasLehmer(self):
        # type: () -> None
        def isTwoPrime():
            return lucasLehmer(2)
        self.assertRaises(AssertionError, isTwoPrime)
        def isNotMersenneNumber():
            return lucasLehmer(2 ** 3)
        self.assertRaises(AssertionError, isNotMersenneNumber)
        self.assertTrue(lucasLehmer(2 ** 3 - 1))
        self.assertFalse(lucasLehmer(2 ** 11 - 1))
        self.assertTrue(lucasLehmer(2 ** 31 - 1))

    def test_millerRabin(self):
        # type: () -> None
        def isOnePrime():
            return millerRabin(2, 1)
        self.assertRaises(AssertionError, isOnePrime)
        def isTwoPrime():
            return millerRabin(1, 2)
        self.assertRaises(AssertionError, isTwoPrime)
        self.assertTrue(millerRabin(20, 2 ** 19 - 1))
        self.assertTrue(millerRabin(2, 2866150047031629177512058644832339139))
        self.assertFalse(millerRabin(2, 286615004703162917751205864483233912))


if __name__ == '__main__':
    unittest.main()
