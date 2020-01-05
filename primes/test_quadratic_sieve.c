// test_quadratic_sieve.c
// Trevor Pottinger
// Tue Dec 31 13:01:24 PST 2019

// Compiling:
// gcc test_quadratic_sieve.c -lm
// clang -lm test_quadratic_sieve.c

#include <stdio.h>

#include "quadratic_sieve.h"

void test_factor(i64 n_primes, i64 *primes, i64 n) {
  i64 num_factors = -1;
  i64 factors[MAX_NUM_DISTINCT_PRIME_FACTORS];
  i64 exponents[MAX_NUM_DISTINCT_PRIME_FACTORS];
  bool _factored = slowFactor(n_primes, primes, n, &num_factors, factors, exponents);
  printf("Factored %ld: [", n);
  for (i64 i = 0; i < num_factors; ++i) {
    printf("(%ld, %ld)", factors[i], exponents[i]);
    if (i < num_factors - 1) {
      printf(", ");
    }
  }
  printf("]\n");
}

int main(int argc, char **argv) {
  i64 n_primes = 30;
  i64 primes[n_primes];
  bool _success = slowFillPrimes(n_primes, primes);
  printf("The %ld-th prime is %ld\n", n_primes, primes[n_primes - 1]);

  printf("sqrt(%ld) = %ld, expected %ld\n", 64L, squareRoot(64L), 8L);
  printf("sqrt(%ld) = %ld, expected %ld\n", 55L, squareRoot(55L), 7L);
  printf("sqrt(%ld) = %ld, expected %ld\n", 61782576236670853L, squareRoot(61782576236670853L), 248561011L);

  test_factor(n_primes, primes, 5959);

  printf("isQuadraticResidue(2, 15347) = %d, expected 1\n", isQuadraticResidue(2L, 15347L));
  printf("isQuadraticResidue(3, 15347) = %d, expected 0\n", isQuadraticResidue(3L, 15347L));
  printf("isQuadraticResidue(73, 90283) = %d, expected 1\n", isQuadraticResidue(73L, 90283L));
  printf("isQuadraticResidue(1223, 463001234381863703) = %d, expected 0\n", isQuadraticResidue(1223L, 463001234381863703L));
  printf("isQuadraticResidue(1217, 463001234381863703) = %d, expected 1\n", isQuadraticResidue(1217L, 463001234381863703L));
  return 0;
}
