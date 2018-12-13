# walk.py
# Trevor Pottinger
# Sat Oct 27 21:51:39 PDT 2018

import os
import sys

if sys.version_info >= (3, 4):
    from typing import (Generator, Tuple)


class Walk(object):

    def __init__(self, hash_func, target_dir):
        # type: (str, str) -> None
        self.hash_func = hash_func
        self.target_dir = target_dir

    def gen(self):
        # type: () -> Generator[Tuple[str, int, str], None, None]
        for _root, _dirs, files in os.walk(self.target_dir):
            for f in files:
                # yield (f, getsize(f), hasher(self.hash_func, f))
                yield (f, 0, '')
