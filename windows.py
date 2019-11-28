# windows.py
# Thu May 30 21:41:25 PDT 2019
# Trevor Pottinger

from __future__ import division
from __future__ import print_function

import sys


def main():
    rows = []
    for l in sys.stdin:
        date, val = l.rstrip("\n").split("\t")
        rows.append((date, float(val)))

    # Print window sizes, in number of trading days
    cols = ['Date', 'Value']
    print("\t".join(cols + ['%dd' % (i + 1) for i in range(len(rows))]))

    for i in range(len(rows)):
        cols = [rows[i][0], str(rows[i][1])]
        num_skips = 0
        for j in range(len(rows)):
            if j <= i:
                num_skips += 1
                continue
            cols.append(str(rows[i][1] / rows[j][1]))
        for j in range(num_skips):
            cols.append('')
        print("\t".join(cols))


if __name__ == '__main__':
    main()
