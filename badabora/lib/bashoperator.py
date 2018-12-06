# bashoperator.py
# Trevor Pottinger
# Sat Oct 27 22:42:20 PDT 2018

from lib.baseoperator import BaseOperator


import sys
if sys.version_info >= (3, 4):
    from typing import (Any, Callable)


class BashOperator(BaseOperator):

    def __init__(self, script, **kwargs):
        # type: (str, **Any) -> None
        super(BashOperator, self).__init__(**kwargs)
        self.script = script
