# bloom_filter.py
# Trevor Pottinger
# Sun Jun 15 10:27:29 PDT 2014

import math
import random

from random import shuffle as rand_shuffle
from random import randint as rand_num
from fractions import gcd

def _is_coprime(a, b):
  return gcd(a, b) == 1

def _hash_builder(num_bits):
  'Returns a simple hash function that uses num_bits as the modulo'
  assert num_bits > 1, 'Hash functions with <2 modulo are just silly'
  c = rand_num(0, num_bits-1)
  while not _is_coprime(c, num_bits):
    c = rand_num(0, num_bits-1)
  # multiplicand
  a = rand_num(1, num_bits-1)
  while not _is_coprime(a, num_bits):
    a = rand_num(1, num_bits-1)
  def __chunks(n):
    'Breaks n up into smaller ints, so each one is smaller than num_bits'
    ns = []
    while n != 0:
      ns.append(n % num_bits)
      n = n / num_bits
    return ns
  def __hash(n):
    # does this work for strings or other types as well? 
    ns = __chunks(n)
    x = (a * ns[0] + c) % num_bits
    for i in range(1, len(ns)):
      # this doesnt quite smell right, b/c x is reused
      x = (a * x + ns[i]) % num_bits
    return x
  return __hash

class BloomFilter(object):
  """A naive implementation of bloom filters. Specifically targetting the use
  of two filters, such that they are disjoint subsets of one larger set. The
  result of this is that a bitwise OR of the two results in the bloom filter
  as though the sets were joined"""

  # INITIALIZERS

  def __init__(self, num_funcs, num_bits, to_build=True):
    assert num_bits > 64, 'Only more than 64 bits please'
    assert num_bits != 0 and ((num_bits & (num_bits - 1)) == 0), 'Only powers of two for now please'
    self.num_funcs = num_funcs
    self.num_bits = num_bits
    # begin needs building
    self.funcs = []
    self.array = 0
    self.array_mask = 0
    self.built = False
    self.num_adds = 0
    # end needs building
    if to_build:
      self.__build()

  def deep_copy(self):
    copy = BloomFilter(self.num_funcs, self.num_bits, False)
    copy.funcs = list(self.funcs) # is this really a deep copy?
    copy.array = self.array # number assignment is by value
    copy.array_mask = self.array_mask
    return copy

  def __build(self):
    """Generate the necessary hash functions for this bloom filter. A
    hash function takes a number and returns a number in the range 
    [0,num_bits]"""
    print "Building %d hash functions..." % self.num_funcs
    self.funcs = [ _hash_builder(self.num_bits) for _ in range(self.num_funcs) ]
    print "Building an array of length 2^%d (%d)..." % (math.log(self.num_bits, 2), self.num_bits)
    self.array = 1 << self.num_bits
    self.array_mask = (1 << self.num_bits) - 1
    self.built = True

  def load(funcs_str, arr_str):
    bf_set = BloomFilter(len(funcs_str.split(":")), len(arr_str) / 2, False)
    bf_set.funcs = [[]] * bf_set.num_funcs # TODO
    bf_set.array = int(arr_str, 16)

  # SETTERS

  def add(self, n):
    self.num_adds += 1
    self.array |= self.get_hash(n)
    return self

  def setArray(self, array):
    self.array = array
    return self

  # GETTERS

  def get_hash(self, n):
    # map hashes on n, results in list of indicies
    hashes = map(lambda f: f(n), self.funcs)
    # map indicies to integer with that bit set
    nums = map(lambda i: 1 << i, hashes)
    return reduce(lambda a, num: a | num, nums, 0)

  def contains(self, n):
    h = self.get_hash(n)
    return h  == (self.array & h)

  def getArrayVal(self):
    return self.array & self.array_mask

  def getArrayRepr(self):
    s = hex(self.getArrayVal()).lstrip('0x').rstrip('L')
    expected_len = self.num_bits / 4 # expected string length
    if len(s) < expected_len:
      return '0' * (expected_len - len(s)) + s
    elif len(s) > expected_len:
      raise Exception("Array longer than expected")
    else:
      return s

  def getFunctionRepr(self):
    return "<function_repr>"

  def __str__(self):
    return hex(self.getArrayVal())[2:-1]

  # end BloomFilter

def test_bf():
  tests = {}

  tests[0] = BloomFilter(3, 128, 255)
  tests[0].add(2**7 + 2**5)
  tests[0].add(2**5 + 2**3)
  print tests[0].getArrayRepr()

  print tests[0].contains(2**7 + 2**5)
  print tests[0].contains(2**5 + 2**3)
  print tests[0].contains(2**5)
