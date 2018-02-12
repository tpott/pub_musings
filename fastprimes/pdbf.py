# pdbf.py
# Trevor Pottinger
# Sun Jun 15 10:27:47 PDT 2014

import random
from math import ceil, log

from bloom_filter import bloom_filter

def randD():
  """Returns a random number between 0 and 1"""
  return random.random()

def randB():
  return randD() < 0.5

def randSort():
  r = randD()
  if r < 0.5: return 1
  elif r > 0.5: return -1
  else: return 0

class pdbf():
  def __init__(self, k, m, maxNum, to_build=True):
    self.k = k
    self.m = m
    self.maxNum = maxNum
    self.primes = bloom_filter(k, m, maxNum)
    self.composites = self.primes.deep_copy()
    self.false_positives = { 'is_prime' : [], 'is_composite' : [] }
    self.built = False
    if to_build:
      self.__build(showProgress=False)

  def __build(self, showProgress=False):
    self.primes.add(1)
    self.primes.add(2)
    self.primes.add(3)
    self.primes.add(5)
    self.primes.add(7)
    self.composites.add(4)
    self.composites.add(6)
    ns = ( n for n in range(8, self.maxNum+1) ) # inclusive range
    if showProgress:
      from tqdm import tqdm
      ns = tqdm(ns, desc='Building disjoint sets', total=self.maxNum)
    for n in ns:
      #break # FIXME TODO REMOVE this stops the build
      #print "testing", n,
      if self.isPrime(n):
        #print "prime",
        self.primes.add(n)
      else:
        #print "composite",
        self.composites.add(n)
      #print bin(self.primes.getArrayVal()).count('1'), bin(self.composites.getArrayVal()).count('1')
    self.built = True

  def isPrime(self, n):
    if n <= 0 or n > self.maxNum:
      raise Exception("%d is out of the valid range [1,%d]" % (n, self.maxNum))
    if self.built:
      probably_composite = self.composites.contains(n)
      probably_prime = self.primes.contains(n)
      if not probably_composite and not probably_prime:
        raise Exception("Congratulations, you broke bloom filters: %d" % n)
      elif not probably_composite and probably_prime:
        return True
      elif probably_composite and not probably_prime:
        return False
      elif n in self.false_positives['is_prime']:
        return True
      elif n in self.false_positives['is_composite']:
        return False
    # at this point, n is not known, definitively
    result = self.isPrimeBruteForce(n)
    if self.built:
      print "New",
      if result:
        print "prime", n,
        found = False
        for j in range(len(self.false_positives['is_prime'])):
          if self.false_positives['is_prime'][j] > n:
            self.false_positives['is_prime'][j:j] = n
            break
        if not found:
          self.false_positives['is_prime'].append(n)
      else:
        print "composite", n,
        found = False
        for j in range(len(self.false_positives['is_composite'])):
          if self.false_positives['is_composite'][j] > n:
            self.false_positives['is_composite'][j:j] = n
            break
        if not found:
          self.false_positives['is_composite'].append(n)
      print "false positive discovered" # end if built
    return result

  def isPrimeBruteForce(self, n):
    for i in range(2, int(ceil(log(n, 2)))+1):
      if self.isPrime(i) and n % i == 0:
        return False
    return True

  def long_message(self):
    config = {
      'num_funcs' : self.k,
      'array_len' : self.m,
      'max_num' : self.maxNum
    }
    return """Prime disjoint bloom filter sets
    %(num_funcs)d number of hash functions that are shared for two
    %(array_len)d long bloom filters, where %(max_num)d is the
    maximum of the checked range""" % config

  def __repr__(self):
    return "pdbf(%d funcs, %d bits, %d max)" % ( self.k, self.m, self.maxNum )

def test_pdbf():
  pass

def pdbf_ord(k, m, N):
  """Returns an initialized pdbf where:
  k num hash funcs
  m order of magnitude of the length of the bit hash arrays
  N order of magnitude of max num"""
  return pdbf(k, 2**m, (2**(2**N)) - 1)
