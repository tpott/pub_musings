# operatorsoutput.py
# Trevor Pottinger
# Wed Dec 12 22:42:45 PST 2018

import sys

if sys.version_info >= (3, 4):
    from typing import (Any, Tuple, Union)

class OperatorOutput(object):

    def write(self, something):
        # type: (Union[str, Tuple[Any, ...]]) -> None
        pass
