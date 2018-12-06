# selectoperator.py
# Trevor Pottinger
# Sat Oct 27 22:41:48 PDT 2018

import sys

from lib.baseoperator import BaseOperator


if sys.version_info >= (3, 4):
    from typing import List


class SelectOperator(BaseOperator):

    def __init__(self, deps, where):
        # type: (List[BaseOperator], str) -> None
        super(SelectOperator, self).__init__(deps)
        self.where = where
