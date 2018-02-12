# main.py
# Trevor Pottinger
# Sun Jun 15 10:26:02 PDT 2014

from bloom_filter import bloom_filter, test_bf
from pdbf import pdbf, pdbf_ord, test_pdbf

def main():
  # Makes two, disjoint bloom filters. They share three hash functions
  # and have 2^8 or 256 bits. Lastly, the max number is 2^2^3 or 256
  primes = pdbf_ord(2, 8, 3) # num funcs, table size, max num
  print primes
  print "primes", primes.primes.getArrayRepr()
  print "composites", primes.composites.getArrayRepr()

  max_num = 256 # must correspond to the third arg above
  nums = {}
  for i in range(1, max_num):
    h = primes.primes.get_hash(i)
    if h in nums:
      nums[h] += 1
    else:
      nums[h] = 1
  nums_list = [ (k, nums[k]) for k in nums ]
  def cmp(x,y):
    if x[1] < y[1]: return 1
    elif x[1] > y[1]: return -1
    else: return 0
  nums_list.sort(cmp)
  print nums_list[:10]
  #return [ n for n in range(1, max_num) if primes.isPrime(n) ]

if __name__ == '__main__':
  main()
  #test_bf()
