# fastprimes.py
# Trevor Pottinger
# Sun Sep 14 22:55:00 PDT 2014

import sys

from bloom_filter import BloomFilter

def n_primes(n):
  "Returns the first n primes"
  assert n < (1 << 35), 'Less than 2^35 primes please'
  assert n > 2, 'Less than 2 primes isnt interesting'
  primes = [2, 3]
  print "Starting the generation of primes with seed(s) %s" % str(primes)
  while len(primes) < n:
    possible_prime = primes[-1]
    while True:
      possible_prime += 2
      is_possible = True
      for p in primes:
        if (possible_prime % p) == 0:
          is_possible = False
          break
      if is_possible:
        primes.append(possible_prime)
        break
  print "Finished, generated a total of %d primes" % len(primes)
  return primes

class FastPrimes(object):
  def __init__(self, primes, num_prime_funcs, num_prime_bits, num_fp_funcs, num_fp_bits):
    self.primes_bloom_filter = BloomFilter(num_prime_funcs, num_prime_bits)
    self.fps_bloom_filter = BloomFilter(num_fp_funcs, num_fp_bits)
    print 'Adding primes'
    for p in primes[:-1]: # why ignore the last prime?
      self.primes_bloom_filter.add(p)
    print 'Adding false positives..'
    for i in range(primes[0], primes[-1]):
      true_prime = i in primes
      bf_prime = self.primes_bloom_filter.contains(i)
      if true_prime and not bf_prime:
        assert False, 'False negatives NEVER happen'
      elif not true_prime and bf_prime:
        self.fps_bloom_filter.add(i)

  def isPrime(self, n):
    bf_prime = self.primes_bloom_filter.contains(n)
    bf_composite = self.fps_bloom_filter.contains(n)
    if bf_prime and not bf_composite:
      return True
    else:
      return False

  def evaluate(self, primes):
    """Tests every number between the first and last primes, including the
    numbers not in primes. We use the primes array as the source of truth."""
    num_false_positives, num_double_fps, min_false_positive = (0, 0, primes[-1] + 1)
    for i in range(primes[0], primes[-1]):
      true_prime = i in primes
      my_prime = self.isPrime(i)
      if true_prime and not my_prime:
        num_double_fps += 1
      elif not true_prime and my_prime:
        num_false_positives += 1
        if i < min_false_positive:
          min_false_positive = i
    return (num_false_positives, num_double_fps, min_false_positive)

  def getCounts(self):
    return (self.primes_bloom_filter.num_adds, self.fps_bloom_filter.num_adds)

# main method, todo move to separate file
if __name__ == '__main__':
  if len(sys.argv) == 7:
    num_primes = int(sys.argv[1])
    num_funcs = int(sys.argv[2])
    num_bits = 1 << int(sys.argv[3])
    num_fp_funcs = int(sys.argv[4])
    num_fp_bits = 1 << int(sys.argv[5])
    num_iterations = int(sys.argv[6])
  else:
    num_primes = 10000
    num_funcs = 2
    num_bits = 1 << 16 # 64K
    num_fp_funcs = 5
    num_fp_bits = 1 << 10
    num_iterations = 10
  primes = n_primes(num_primes)
  for i in range(num_iterations):
    print "### Run %d ###" % (i+1)
    fprimes = FastPrimes(primes, num_funcs, num_bits, num_fp_funcs, num_fp_bits)
    print 'Evaluating results'
    (fps, in_both, min_fp) = fprimes.evaluate(primes)
    print 'False positives %d, in both %d, min false pos %d' % (fps, in_both, min_fp)
    print 'Counts=', fprimes.getCounts()
