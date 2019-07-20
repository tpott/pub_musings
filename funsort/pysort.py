# pysort.py

from __future__ import print_function

import argparse
import sys

def main():
    # type: () -> None
    parser = argparse.ArgumentParser(description='Sorts some data')

    args = parser.parse_args()

    d = None
    rows = []
    for line in sys.stdin:
        cols = line.rstrip("\n").split("\t")
        rows.append(tuple(map(float, cols)))
        if d is None:
            d = len(cols)

    rows.sort(key=lambda x: x[0])

    format_s = "\t".join(["%f"] * d)
    for row in rows:
        print(format_s % row)

if __name__ == '__main__':
    main()
