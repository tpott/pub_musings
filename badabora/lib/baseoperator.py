# baseoperator.py
# Trevor Pottinger
# Sat Oct 27 21:47:49 PDT 2018

from __future__ import print_function

import sys

if sys.version_info >= (3, 4):
    from typing import List


class BaseOperator(object):

    def __init__(self, deps):
        # type: (List[BaseOperator]) -> None
        self.deps = deps
        self.finished = False

    def run(self):
        # type: () -> None
        if self.finished:
            return
        for dep in self.deps:
            dep.run()
        print(self.runMessage())
        self.finished = True

    def runMessage(self):
        # type: () -> str
        return 'Running %s!' % self.__class__.__name__
