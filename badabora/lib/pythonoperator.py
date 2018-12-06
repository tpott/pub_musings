# pythonoperator.py
# Trevor Pottinger
# Sat Oct 27 22:42:46 PDT 2018

from lib.baseoperator import BaseOperator


import sys
if sys.version_info >= (3, 4):
    from typing import (Any, Callable)


class PythonOperator(BaseOperator):

    def __init__(self, func, **kwargs):
        # type: (Callable[[], None], **Any) -> None
        super(PythonOperator, self).__init__(**kwargs)
        self.func = func

    def runMessage(self):
        # type: () -> str
        return 'Running %s: %s' % (self.__class__.__name__, self.func.__name__)
