// gen_big.c
// Trevor Pottinger
// Fri Nov 29 17:21:53 PST 2019

// Compiling:
// gcc gen_big.c -lm
// clang -lm gen_big.c

#include <fcntl.h>
#include <math.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>

typedef int64_t i64;

#define INT64_NUM_BYTES 8

// Error codes
#define INCORRECT_NUMBER_OF_ARGS 1
#define N_BITS_TOO_LOW 2
#define SLEEP_TOO_LOW 3

i64 modPow(i64 base, i64 exp, i64 mod) {
  // TODO assert(mod > 1)
  i64 result = 1;
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

i64 randrange(i64 lo, i64 hi) {
  // TODO assert(lo < hi)
  // TODO don't open /dev/urandom each time
  int rand_fd = open("/dev/urandom", O_RDONLY);
  i64 n;
  size_t num_bytes = read(rand_fd, &n, INT64_NUM_BYTES);
  // TODO assert(num_bytes == INT64_NUM_BYTES)

  // This is to ensure `n % range` is positive
  if (n < 0) {
    n = -n;
  }

  i64 range = hi - lo;
  return (n % range) + lo;
}

bool millerRabin(i64 k, i64 n) {
  for (i64 i = 0; i < k; ++i) {
    i64 a = randrange(2L, n);
    if (modPow(a, n - 1, n) != 1) {
      return false;
    }
  }

  return true;
}

bool isPrime(i64 n) {
  switch (n) {
    case 2:
    case 3:
    case 5:
    case 7:
    case 11:
    case 13:
    case 17:
    case 19:
    case 23:
    case 29:
      return true;
  }

  if (millerRabin(200L, n)) {
    return true;
  }

  // TODO additional checks

  return false;
}

void printPrime(i64 n) {
  printf("{\"n\": %ld, \"log2\": %f}\n", n, log(n));
}

void printUsage(char *command) {
  printf("printUsage() %s\n", command);
  printf("  n_bits, must be greater than 1\n");
  printf("  sleep, must be greater than or equal to 0\n");
}

int main(int argc, char** argv) {
  // Read the used command and decrement argc to be more readable
  char *command = argv[0];
  --argc;
  if (argc != 2) {
    printUsage(command);
    return INCORRECT_NUMBER_OF_ARGS;
  }

  int n_bits = atoi(argv[1]);
  if (n_bits <= 1) {
    printUsage(command);
    return N_BITS_TOO_LOW;
  }

  int sleep_time = atoi(argv[2]);
  if (sleep_time < 0) {
    printUsage(command);
    return SLEEP_TOO_LOW;
  }

  bool running = true;
  while (running) {
    bool is_prime = false;
    while (!is_prime) {
      // TODO random.getrandbits(n_bits) instead of randrange
      i64 n = randrange(2L, 1L << 32);
      if (n <= 1 || !isPrime(n)) {
        continue;
      }

      is_prime = true;
      printPrime(n);
    }

    // TODO proper sleep/nanosleep sleep_time
    sleep(1);
  }

  return 0;
}
