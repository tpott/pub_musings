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

// https://stackoverflow.com/questions/3437404/min-and-max-in-c
i64 min(i64 a, i64 b) {
  if (a <= b) {
    return a;
  }
  return b;
}

i64 modPow(i64 base, i64 exp, i64 mod) {
  // TODO assert(mod > 1)
  i64 result = 1;
  // Does this line make sense?
  base = base % mod;
  while (exp > 0) {
    // This is logically equivalent to `exp % 2`
    if ((exp & 0x1) == 1) {
      result = (result * base) % mod;
    }
    exp = exp >> 1;
    base = (base * base) % mod;
  }
  return result;
}

bool slowFillPrimes(greater_than_one count, prime *arr) {
  // TODO implement count primes
  arr[0] = make_prime(2);
  arr[1] = make_prime(3);
  for (i64 i = 2; i < greater_than_one_val(count); ++i) {
    i64 prev_prime = prime_val(arr[i - 1]);
    // Keep iterating until we find the next prime
    for (i64 j = prev_prime + 2; true; j += 2) {
      bool is_prime = true;
      for (i64 k = 0; k < i; ++k) {
        if (j % prime_val(arr[k]) == 0) {
          is_prime = false;
          break;
        }
      }
      if (is_prime) {
        arr[i] = make_prime(j);
        break;
      }
    }
  }
  return true;
}

// Based on https://rosettacode.org/wiki/Integer_roots#C
// This currently rounds down, but I want it to round up.
i64 squareRoot(greater_than_one n) {
  i64 c = 1;
  i64 d = (1 + greater_than_one_val(n)) / 2;
  i64 e = (d + greater_than_one_val(n) / d) / 2;
  while (c != d && c != e) {
    c = d;
    d = e;
    e = (e + greater_than_one_val(n) / e) / 2;
  }
  if (d < e) {
    return d;
  }
  return e;
}

bool isBSmooth(positive num_primes, prime *primes, i64 n) {
  for (i64 i = 0; i < positive_val(num_primes); ++i) {
    while (n % prime_val(primes[i]) == 0) {
      n /= prime_val(primes[i]);
    }
    if (n == 1) {
      return true;
    }
  }
  // Idea: b-smoothish numbers
  // If n is a square number (i.e. `n == sqrt(n)**2`) then we could lie here
  // because it's large prime factors are all even. This means they would
  // contribute zero to the row reduction.
  return false;
}

bool bSmoothList(
  positive num_primes,
  prime *primes,
  greater_than_one n,
  i64 num_ints,
  i64 max_num_ints,
  i64 *num_ints_found,
  i64 *ints,
  greater_than_one *squares
) {
  *num_ints_found = 0;
  // TODO round the square root up, so we don't have to check if `squared`
  // is negative in the loop below
  i64 root = squareRoot(n);
  for (i64 i = root; i < root + num_ints && *num_ints_found < max_num_ints; ++i) {
    // If we had multiple polynomials, this is would need changing
    i64 squared = i * i - greater_than_one_val(n);
    if (squared < 0) {
      continue;
    }
    if (!isBSmooth(num_primes, primes, squared)) {
      continue;
    }
    ints[*num_ints_found] = i;
    squares[*num_ints_found] = make_greater_than_one(squared);
    ++*num_ints_found;
  }
  return true;
}

bool slowFactor(
  positive num_primes,
  prime *primes,
  greater_than_one n,
  i64 *num_factors,
  prime *factors,
  i64 *exponents
) {
  if (greater_than_one_val(n) == 2) {
    *num_factors = 1;
    factors[0] = make_prime(2);
    exponents[0] = 1;
    return true;
  }

  *num_factors = 0;
  for (i64 i = 0; i < positive_val(num_primes); ++i) {
    if (greater_than_one_val(n) % prime_val(primes[i]) != 0) {
      continue;
    }
    factors[*num_factors] = primes[i];
    i64 exponent = 0;
    do {
      ++exponent;
      n = make_greater_than_one(greater_than_one_val(n) / prime_val(primes[i]));
    } while (greater_than_one_val(n) % prime_val(primes[i]) == 0);
    exponents[*num_factors] = exponent;
    ++*num_factors;
    if (greater_than_one_val(n) == 1) {
      return true;
    }
  }

  return false;
}

void printInts(i64 num_ints, i64 *ints) {
  printf("[");
  for (i64 i = 0; i < num_ints; ++i) {
    printf("%lld", ints[i]);
    if (i < num_ints - 1) {
      printf(", ");
    }
  }
  printf("]");
}

void printFactors(i64 num_factors, i64 *factors, i64 *exponents) {
  printf("[");
  for (i64 j = 0; j < num_factors; ++j) {
    printf("(%lld, %lld)", factors[j], exponents[j]);
    if (j < num_factors - 1) {
      printf(", ");
    }
  }
  printf("]");
}

// All numbers less than 2**64.8 have 15 or fewer prime factors.
// All numbers less than 2**134.1 have 26 or fewer prime factors.
// All numbers less than 2**256.7 have 43 or fewer prime factors.
// All numbers less than 2**517.6 have 75 or fewer prime factors.
// All numbers less than 2**1028 have 131 or fewer prime factors.
// All numbers less than 2**2056.7 have 233 or fewer prime factors.
#define MAX_NUM_DISTINCT_PRIME_FACTORS 15
bool findSquareProducts(
  positive num_primes,
  prime *primes,
  i64 num_ints,
  greater_than_one *ints
) {
  // Should we assert num_ints > num_primes? Most implementations search for
  // num_ints = num_primes + 1. Supposedly, each additional int more than prime
  // improves the likelyhood that a solution will be found.
  i64 num_factors[num_ints];
  prime factors[num_ints][MAX_NUM_DISTINCT_PRIME_FACTORS];
  i64 exponents[num_ints][MAX_NUM_DISTINCT_PRIME_FACTORS];

  for (i64 i = 0; i < num_ints; ++i) {
    bool factored = slowFactor(
      num_primes,
      primes,
      ints[i],
      &num_factors[i],
      factors[i],
      exponents[i]
    );
    // printf("Factored %lld: ", ints[i]);
    // printFactors(num_factors[i], factors[i], exponents[i]);
    // printf("\n");
    if (!factored) {
      // This should never happen when all the `ints` are b-smooth, where
      // `b = primes[num_primes - 1]` (i.e. the max prime). We fail here
      // because the code below doesn't handle un-factored ints.
      printf("WTF! Unable to factor: %lld\n", greater_than_one_val(ints[i]));
      return false;
    }
    // rows.append((num_factors, factors, exponents));
  }

  // exponents = vectorize(rows, default=0)
  // even_exponents = matrixMap(exponents, lambda x: x % 2)
  // TODO implement factors_to_col: prime -> column index
  i16 exponent_matrix[num_ints][positive_val(num_primes)];
  for (i64 i = 0; i < num_ints; ++i) {
    for (i64 j = 0; j < num_factors[i]; ++j) {
      i64 col_index = 0; // factors_to_col[factors[i][j]]
      exponent_matrix[i][col_index] = exponents[i][j] % 2;
    }
  }

  // TODO finish implementing findSquareProducts
  // result_matrix = rowReduce(matrixExtend(even_exponents, identity(count(rows))), 2)
  // foreach row in result_matrix:
  //   if isAllZeros(row[some_point:]):
  //     solutions.append(row)
  return false;
}

// Translated from https://rosettacode.org/wiki/Tonelli-Shanks_algorithm#Python
i64 legendre(i64 a, i64 p) {
  return modPow(a, (p - 1) / 2, p);
}

// Translated from https://rosettacode.org/wiki/Tonelli-Shanks_algorithm#Python
bool tonelli(i64 n, i64 p) {
  if (legendre(n, p) != 1) {
    return false;
  }
  // Really, this should return a tuple of (success, sqrt), but we only care
  // about if it succeeds. So we just return a bool.
  // TODO finish implementing the tonelli algorithm
  return true;
}

bool isQuadraticResidue(i64 p, i64 n) {
  // TODO filter primes to quadratic residues
  return tonelli(n, p);
}

bool quadraticSieve(
  greater_than_one n,
  positive mult,
  i64 all_num_primes_untyped,
  i64 *results
) {
  if (all_num_primes_untyped <= 1) {
    printf("min all_num_primes is 2, got %lld\n", all_num_primes_untyped);
    return false;
  }
  greater_than_one all_num_primes = make_greater_than_one(all_num_primes_untyped);

  prime all_primes[all_num_primes_untyped];
  bool success = slowFillPrimes(all_num_primes, all_primes);
  if (!success) {
    printf("Failed to find some small primes\n");
    return false;
  }

  prime primes[all_num_primes_untyped];
  i64 num_primes_found = 0;
  for (i64 i = 0; i < all_num_primes_untyped; ++i) {
    if (!isQuadraticResidue(prime_val(all_primes[i]), greater_than_one_val(n))) {
      continue;
    }
    primes[num_primes_found] = all_primes[i];
    ++num_primes_found;
  }

  if (num_primes_found == 0) {
    printf("Failed to find find any primes\n");
    return false;
  }
  printf(
    "Successfully found %lld primes, max = %lld, out of all %lld primes considered\n",
    num_primes_found,
    prime_val(primes[num_primes_found - 1]),
    all_num_primes_untyped
  );
  positive num_primes = make_positive(num_primes_found);

  printf("Primes: ");
  printInts(num_primes_found, (i64*)(primes));
  printf("\n");

  // find b-smooth squares and their respective ints using n and primes
  i64 num_search_ints = positive_val(mult) * num_primes_found;
  i64 max_num_ints = min(num_search_ints, num_primes_found + 100);
  i64 ints[max_num_ints];
  greater_than_one squares[max_num_ints];
  i64 num_ints_found = -1;
  success = bSmoothList(
    num_primes,
    primes,
    n,
    num_search_ints,
    max_num_ints,
    &num_ints_found,
    ints,
    squares
  );

  if (!success) {
    printf("Failed to find a list of b-smooth squares\n");
    return false;
  }
  printf("Successfully found %lld b-smooth squares out of %lld\n", num_ints_found, num_search_ints);
  if (num_ints_found == max_num_ints) {
    printf("Found the maximum number of b-smooth squares, rather than searching all ints\n");
  }
  printf("Ints: ");
  printInts(num_ints_found, ints);
  printf("\n");

  // find square products using primes and squares
  i64 num_solutions = -1;
  success = findSquareProducts(num_primes, primes, num_ints_found, squares);
  if (!success) {
    printf("Failed to find square products\n");
    return false;
  }

  // TODO check each solution

  return false;
}
