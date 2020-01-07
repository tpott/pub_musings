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

typedef int16_t i16;
typedef int64_t i64;
typedef int64_t greater_than_one;

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

bool slowFillPrimes(greater_than_one count, i64 *arr) {
  // TODO assert count > 1
  // TODO implement count primes
  arr[0] = 2;
  arr[1] = 3;
  for (i64 i = 2; i < count; ++i) {
    i64 prev_prime = arr[i - 1];
    // Keep iterating until we find the next prime
    for (i64 j = prev_prime + 2; true; j += 2) {
      bool is_prime = true;
      for (i64 k = 0; k < i; ++k) {
        if (j % arr[k] == 0) {
          is_prime = false;
          break;
        }
      }
      if (is_prime) {
        arr[i] = j;
        break;
      }
    }
  }
  return true;
}

// Based on https://rosettacode.org/wiki/Integer_roots#C
// This currently rounds down, but I want it to round up.
i64 squareRoot(i64 n) {
  // assert n > 1
  i64 c = 1;
  i64 d = (1 + n) / 2;
  i64 e = (d + n / d) / 2;
  while (c != d && c != e) {
    c = d;
    d = e;
    e = (e + n / e) / 2;
  }
  if (d < e) {
    return d;
  }
  return e;
}

bool isBSmooth(i64 num_primes, i64 *primes, i64 n) {
  for (i64 i = 0; i < num_primes; ++i) {
    while (n % primes[i] == 0) {
      n /= primes[i];
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

bool bSmoothList(i64 num_primes, i64 *primes, i64 n, i64 num_ints, i64 max_num_ints, i64 *num_ints_found, i64 *ints, i64 *squares) {
  // assert n > 1
  *num_ints_found = 0;
  // TODO round the square root up, so we don't have to check if `squared`
  // is negative in the loop below
  i64 root = squareRoot(n);
  for (i64 i = root; i < root + num_ints && *num_ints_found < max_num_ints; ++i) {
    // If we had multiple polynomials, this is would need changing
    i64 squared = i * i - n;
    if (squared < 0) {
      continue;
    }
    if (!isBSmooth(num_primes, primes, squared)) {
      continue;
    }
    ints[*num_ints_found] = i;
    squares[*num_ints_found] = squared;
    ++*num_ints_found;
  }
  return true;
}

bool slowFactor(i64 num_primes, i64 *primes, i64 n, i64 *num_factors, i64 *factors, i64 *exponents) {
  // assert n > 1
  if (n == 2) {
    *num_factors = 1;
    factors[0] = 2;
    exponents[0] = 1;
    return true;
  }
  *num_factors = 0;
  for (i64 i = 0; i < num_primes; ++i) {
    if (n % primes[i] != 0) {
      continue;
    }
    factors[*num_factors] = primes[i];
    i64 exponent = 0;
    do {
      ++exponent;
      n /= primes[i];
    } while (n % primes[i] == 0);
    exponents[*num_factors] = exponent;
    ++*num_factors;
    if (n == 1) {
      return true;
    }
  }
  return false;
}

void printFactors(i64 num_factors, i64 *factors, i64 *exponents) {
  printf("[");
  for (i64 j = 0; j < num_factors; ++j) {
    printf("(%ld, %ld)", factors[j], exponents[j]);
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
bool findSquareProducts(i64 num_primes, i64 *primes, i64 num_ints, i64 *ints) {
  // Should we assert num_ints > num_primes? Most implementations search for
  // num_ints = num_primes + 1. Supposedly, each additional int more than prime
  // improves the likelyhood that a solution will be found.
  i64 num_factors[num_ints];
  i64 factors[MAX_NUM_DISTINCT_PRIME_FACTORS][num_ints];
  i64 exponents[MAX_NUM_DISTINCT_PRIME_FACTORS][num_ints];
  for (i64 i = 0; i < num_ints; ++i) {
    bool factored = slowFactor(num_primes, primes, ints[i], &num_factors[i], factors[i], exponents[i]);
    // printf("Factored %ld: ", ints[i]);
    // printFactors(num_factors[i], factors[i], exponents[i]);
    // printf("\n");
    if (!factored) {
      // This should never happen when all the `ints` are b-smooth, where
      // `b = primes[num_primes - 1]` (i.e. the max prime). We fail here
      // because the code below doesn't handle un-factored ints.
      printf("WTF! Unable to factor: %ld\n", ints[i]);
      return false;
    }
    // rows.append((num_factors, factors, exponents));
  }
  // exponents = vectorize(rows, default=0)
  // even_exponents = matrixMap(exponents, lambda x: x % 2)
  // TODO implement factors_to_col: prime -> column index
  i16 exponent_matrix[num_ints][num_primes];
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

bool quadraticSieve(i64 n, i64 mult, i64 all_num_primes, i64 *results) {
  // assert n > 1
  // assert mult > 0
  // assert all_num_primes > 1

  i64 all_primes[all_num_primes];
  bool success = slowFillPrimes(all_num_primes, all_primes);
  if (!success) {
    printf("Failed to find some small primes\n");
    return false;
  }
  i64 primes[all_num_primes];
  i64 num_primes = 0;
  for (i64 i = 0; i < all_num_primes; ++i) {
    if (!isQuadraticResidue(all_primes[i], n)) {
      continue;
    }
    primes[num_primes] = all_primes[i];
    ++num_primes;
  }
  printf("Successfully found %ld primes, max = %ld, out of all %ld primes considered\n", num_primes, primes[num_primes - 1], all_num_primes);

  printf("Primes: [");
  for (i64 i = 0; i < num_primes; ++i) {
    printf("%ld", primes[i]);
    if (i < num_primes - 1) {
      printf(", ");
    }
  }
  printf("]\n");

  // find b-smooth squares and their respective ints using n and primes
  i64 num_search_ints = mult * num_primes;
  i64 max_num_ints = min(num_search_ints, num_primes + 100);
  i64 ints[max_num_ints];
  i64 squares[max_num_ints];
  i64 num_ints_found = -1;
  success = bSmoothList(num_primes, primes, n, num_search_ints, max_num_ints, &num_ints_found, ints, squares);
  if (!success) {
    printf("Failed to find a list of b-smooth squares\n");
    return false;
  }
  printf("Successfully found %ld b-smooth squares out of %ld\n", num_ints_found, num_search_ints);

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
