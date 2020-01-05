// quadratic_sieve.c
// Trevor Pottinger
// Tue Dec 31 12:52:12 PST 2019

// This is mostly interesting to see how much faster c is than python. The
// slowest step is finding b-smooth numbers. So even if I don't finish
// implementing this completely, as long as we can output the list of
// b-smooth numbers, then maybe it can hand the remaining bits off to python. 

// Compiling:
// gcc quadratic_sieve.c -lm
// clang -lm quadratic_sieve.c

#include <stdio.h>

#include "quadratic_sieve.h"

// Error codes
#define INCORRECT_NUMBER_OF_ARGS 1
#define N_TOO_SMALL 2
#define MULT_TOO_SMALL 3
#define NUM_PRIMES_TOO_SMALL 4
#define FAILED_TO_FACTOR 5

void printUsage(char *command) {
  printf("printUsage() %s\n", command);
  printf("  n, must be greater than 1\n");
  printf("  mult, must be greater than 0\n");
  printf("  num_primes, must be greater than 1\n");
}

int main(int argc, char** argv) {
  // Read the used command and decrement argc to be more readable
  char *command = argv[0];
  --argc;
  if (argc != 3) {
    printUsage(command);
    return INCORRECT_NUMBER_OF_ARGS;
  }

  // TODO parse an int larger than 64bits
  i64 n = atoll(argv[1]);
  if (n <= 1) {
    printUsage(command);
    return N_TOO_SMALL;
  }

  i64 mult = atoll(argv[2]);
  if (mult <= 0) {
    printUsage(command);
    return MULT_TOO_SMALL;
  }

  i64 num_primes = atoll(argv[3]);
  if (num_primes <= 1) {
    printUsage(command);
    return NUM_PRIMES_TOO_SMALL;
  }

  i64 factors[2];
  bool success = quadraticSieve(n, mult, num_primes, factors);
  if (!success) {
    printf("Failed to factor %ld\n", n);
    return FAILED_TO_FACTOR;
  }

  return 0;
}
