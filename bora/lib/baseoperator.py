# baseoperator.py
# Trevor Pottinger
# Sat Oct 27 21:47:49 PDT 2018

import sys

if sys.version_info >= (3, 4):
    from typing import List


class BaseOperator(object):

    def __init__(self, deps):
        # type: (List[BaseOperator]) -> None
        self.deps = deps


    def run(self):
        # type: () -> None
        pass
