// main.c
// Sun May 20 12:56:17 IST 2018

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef long long i64;

// These strings are not zeroed out.
typedef struct str {
  i64 length;
  char *data;
} str;

typedef struct list_node {
  struct list_node *next;
  void *data;
} list_node;

#define IN_BUFFER_SIZE 1024
#define LINE_BUFFER_SIZE 16384
#define NULLCHAR '\0'
#define TAB '\t'
#define NEWLINE '\n'

/**
 * You always have at least one column, even if line is the empty string.
 */
int line_callback(i64 length, char *line) {
  printf("callback: %d, ", (int) length);

  // i64 num_cols = split(line, &cols, TAB);

  i64 num_cols = 0;
  list_node *cols = NULL;
  i64 col_start = 0;
  for (i64 i = 0; i < length; ++i) {
    if (line[i] != TAB) {
      continue;
    }

    // Add the column of data before the tab
    str *col = malloc(sizeof(str));
    col->length = i - col_start;
    col->data = &line[col_start];
    col_start = i + 1;
    ++num_cols;

    list_node *col_node = malloc(sizeof(list_node));
    col_node->next = cols;
    col_node->data = col;
    cols = col_node;
  }

  // Add the last column
  str *col = malloc(sizeof(str));
  col->length = length - col_start;
  col->data = &line[col_start];
  ++num_cols;

  list_node *col_node = malloc(sizeof(list_node));
  col_node->next = cols;
  col_node->data = col;
  cols = col_node;

  printf("%d, %s\n", (int) num_cols, line);

  // Cleanup data structures
  for (i64 i = 0; i < num_cols; ++i) {
    list_node *col_node = cols;
    str *col = cols->data;
    cols = cols->next;
    col_node->next = NULL;
    col_node->data = NULL;
    free(col_node);
    free(col);
  }

  return 0;
}

int main(int argc, char **argv) {
  printf("Hello from main!\n");

  char in_buffer[IN_BUFFER_SIZE], line_buffer[LINE_BUFFER_SIZE]; 

  // When is line_i not zero? When line_buffer has some part of a line, but
  // in_buffer is emptied.
  i64 bytes_read, line_i = 0;
  do {
    bytes_read = fread(in_buffer, sizeof(char), IN_BUFFER_SIZE, stdin);
    printf("Read %d bytes\n", (int) bytes_read);

    i64 line_start = 0;
    for (i64 i = 0; i < bytes_read; ++i) {
      if (in_buffer[i] != NEWLINE) {
        continue;
      }

      // Copy to where we left off. Copy from the last line ending.
      memcpy(&line_buffer[line_i], &in_buffer[line_start], i - line_start);
      line_buffer[line_i + i - line_start] = NULLCHAR;

      // Ignore warning of:
      // "passing 'char (*)[16384]' to parameter of type 'char *'"
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wincompatible-pointer-types"
      line_callback(line_i + i - line_start, &line_buffer);
#pragma GCC diagnostic pop

      line_start = i + 1;
      line_i = 0;
    }

    // Copy the remaining bytes in in_buffer to line_buffer
    memcpy(&line_buffer[line_i], &in_buffer[line_start], bytes_read - line_start);
    line_i += bytes_read - line_start;
  } while (bytes_read > 0);

  if (line_i == 0) {
    return 0;
  }

  line_buffer[line_i] = NULLCHAR;

#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wincompatible-pointer-types"
  line_callback(line_i, &line_buffer);
#pragma GCC diagnostic pop

  return 0;
}
