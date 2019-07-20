# gendata.py

from __future__ import print_function

import argparse
import random

def main():
    # type: () -> None
    parser = argparse.ArgumentParser(description='Generates some random data')
    parser.add_argument('--count', type=int, default=5, help=(
        'The number of data points to generate'))
    parser.add_argument('--dimensions', type=int, default=1, help=(
        'The number of dimensions each data point should be in'))

    args = parser.parse_args()
    n = args.count
    d = args.dimensions

    assert n >= 0, '--count must be non-negative'
    assert d > 0, '--dimensions must be greater than zero'

    format_s = "\t".join(["%f"] * d)
    for _ in range(n):
        row = []
        for _ in range(d):
            row.append(random.random())
        print(format_s % tuple(row))

if __name__ == '__main__':
    main()
