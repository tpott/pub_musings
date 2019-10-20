# n_squared.py
# Trevor Pottinger
# Sat Oct 19 20:59:09 PDT 2019

from typing import List

import sys


def square(data: List[float]) -> List[List[float]]:
    ret = []
    for i in range(len(data) - 1):
        row = []
        for j in range(i + 1, len(data)):
            # Example: divide today's data over yesterday's data
            row.append(data[j] / data[i])
        ret.append(row)
    return ret



if __name__ == '__main__':
    # Questions:
    # 1) How many columns are in the resulting data? Does the average of the
    #    first column mean the average of 1 day returns?
    # 2) What order should the input be in? What happens if it's descending
    #    (i.e. most recent first) instead of ascending (i.e. oldest first)?
    data = []
    for line in sys.stdin:
        data.append(float(line))
    for day in square(data):
        print("\t".join(map(str, day)))

    # Goal: given a long history (100 years?) of returns, what is the expected
    # returns for a target time frame? For example, typically investments are
    # for ~approximately six months, ten years, twenty years, etc. So the
    # number of days, assuming a target of six months, is approximately 180,
    # but probably not exactly 180 days. Assuming 180 +/- 10 days, then that
    # should be 20 columns of output.
