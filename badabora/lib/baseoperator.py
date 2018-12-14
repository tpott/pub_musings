# baseoperator.py
# Trevor Pottinger
# Sat Oct 27 21:47:49 PDT 2018

from __future__ import print_function

import sys

if sys.version_info >= (3, 4):
    from typing import (Any, List, Tuple, Union)


class BaseOperator(object):

    def __init__(self, deps):
        # type: (List[BaseOperator]) -> None
        self.deps = deps
        self.finished = False
        self.subs = []  # type: List[BaseOperator]
        for dep in self.deps:
            dep.subs.append(self)

    def read(self, something):
        # type: (Union[str, Tuple[Any, ...]]) -> None
        pass

    def write(self, something):
        # type: (Union[str, Tuple[Any, ...]]) -> None
        for sub in self.subs:
            # Consider self.subs expected input types. If it wants str, but we
            # pass in a Tuple, then it will barf. We should be able to check
            # output types and input types before running. At least, the number
            # of elements in the Tuple, or columns in the TSV str. We could
            # even do these checks at __init__ time.
            sub.read(something)

    def run(self):
        # type: () -> None
        if self.finished:
            return
        for dep in self.deps:
            if dep.finished:
                continue
            dep.run()
        print(self.runMessage())
        self.finished = True

    def runMessage(self):
        # type: () -> str
        return 'Running %s!' % self.__class__.__name__
