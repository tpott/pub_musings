// decrypt.c
// Trevor Pottinger
// Sat Oct 27 22:07:26 PDT 2018

#include <stdio.h>

#include "tweetnacl.h"

typedef long long i64;

#define NULLCHAR '\0'
#define TAB '\t'
#define NEWLINE '\n'

int main(int argc, char **argv) {
    i64 hi = 24;
    printf("hello world %lld\n", hi);
    return 0;
}
