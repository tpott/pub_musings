# carve.py
# Wed May  9 14:18:46 IST 2018

from __future__ import print_function

import sys


def main(source, start, end, dest):
    # type: (str, int, int, str) -> None
    with open(source, 'rb') as sf:
        sf.seek(start)
        byte_str = sf.read(end)
    with open(dest, 'wb') as df:
        df.write(byte_str)
    return

if __name__ == '__main__':
    assert len(sys.argv) >= 5, 'too few arguments'
    source_file, start, end, dest_file = sys.argv[1:5]
    start_offset = int(start)
    end_offset = int(end)
    main(source_file, start_offset, end_offset, dest_file)
