# selectoperator.py
# Trevor Pottinger
# Sat Oct 27 22:41:48 PDT 2018

from lib.baseoperator import BaseOperator


import sys
if sys.version_info >= (3, 4):
    from typing import (Any, List)


class SelectOperator(BaseOperator):

    def __init__(self, where, **kwargs):
        # type: (str, **Any) -> None
        super(SelectOperator, self).__init__(**kwargs)
        self.where = where

    def runMessage(self):
        # type: () -> str
        return 'Running %s: select * where %s' % (self.__class__.__name__, self.where)
