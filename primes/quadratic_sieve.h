// quadratic_sieve.h
// Trevor Pottinger
// Tue Dec 31 13:03:41 PST 2019

// Compiling:
// gcc quadratic_sieve.c -lm
// clang -lm quadratic_sieve.c

#include <fcntl.h>
#include <math.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>

// Copied from https://blog.nelhage.com/2010/10/using-haskells-newtype-in-c/
#define NEWTYPE(tag, repr)                  \
    typedef struct { repr val; } tag;       \
    static inline tag make_##tag(repr v) {  \
            return (tag){.val = v};         \
    }                                       \
    static inline repr tag##_val(tag v) {   \
            return v.val;                   \
    }

typedef int16_t i16;
typedef int64_t i64;

NEWTYPE(greater_than_one, int64_t);
NEWTYPE(positive, int64_t);
NEWTYPE(prime, int64_t);

#define INT64_NUM_BYTES 8

// Error codes
#define INCORRECT_NUMBER_OF_ARGS 1

// All numbers _less than_ 2**64.8 have 15 or fewer prime factors.
// All numbers _less than_ 2**134.1 have 26 or fewer prime factors.
// All numbers _less than_ 2**256.7 have 43 or fewer prime factors.
// All numbers _less than_ 2**517.6 have 75 or fewer prime factors.
// All numbers _less than_ 2**1028 have 131 or fewer prime factors.
// All numbers _less than_ 2**2056.7 have 233 or fewer prime factors.
// http://oeis.org/wiki/Omega(n),_number_of_distinct_primes_dividing_n
// http://oeis.org/A001221
// Easily generate this by multiplying the first N primes until the
// accumulative product is greater than the threshold you seek.
#define MAX_NUM_DISTINCT_PRIME_FACTORS 26

i64 min(i64 a, i64 b);

i64 modPow(i64 base, i64 exp, i64 mod);

bool slowFillPrimes(greater_than_one count, prime *arr);

i64 squareRoot(greater_than_one n);

bool isBSmooth(positive num_primes, prime *primes, i64 n);

bool bSmoothList(
  positive num_primes,
  prime *primes,
  greater_than_one n,
  i64 num_ints,
  i64 max_num_ints,
  i64 *num_ints_found,
  i64 *ints,
  greater_than_one *squares
);

bool slowFactor(
  positive num_primes,
  prime *primes,
  greater_than_one n,
  i64 *num_factors,
  prime *factors,
  i64 *exponents
);

void printInts(i64 num_ints, i64 *ints);

void printFactors(i64 num_factors, i64 *factors, i64 *exponents);

bool findSquareProducts(
  positive num_primes,
  prime *primes,
  i64 num_ints,
  greater_than_one *ints
);

i64 legendre(i64 a, i64 p);

bool tonelli(i64 n, i64 p);

bool isQuadraticResidue(i64 p, i64 n);

bool quadraticSieve(
  greater_than_one n,
  positive mult,
  greater_than_one all_num_primes,
  i64 *results
);
