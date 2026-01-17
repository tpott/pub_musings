# reformat.py
# Trevor Pottinger
# Tue Sep 22 00:22:49 PDT 2020

import sys

def main() -> None:
    for line in sys.stdin:
        start_str, end_str, len_str, word = line.rstrip('\n').split('\t')
        print('{start:0.3f}\t{end:0.3f}\t{duration:0.3f}\t{content}'.format(
            start=float(start_str),
            end=float(end_str),
            duration=float(len_str),
            content=word.lower()
        ))
    return


if __name__ == '__main__':
    main()
