# html.py
# Trevor Pottinger
# Fri Oct 18 23:24:31 PDT 2019

from typing import (Any, List, Optional)

class HTML(object):
    def __init__(self, byte_str: bytes) -> None:
        self.byte_str = byte_str

    def select(self, selector: bytes) -> List[object]:
        ret = []
        return ret

    def select_one(self, selector: bytes) -> Optional[object]:
        return None


if __name__ == '__main__':
    page = HTML(b'<table><tbody><tr><td>A</td><td>B</td></tr><tr><td>1</td><td>2</td></tr></tbody></table>')
    # #examples > div:nth-child(4) > div > pre > span:nth-child(158)
    table = page.select_one('table')
    assert table is not None, 'table not found!'
    header = table.select('tr')[0]
    rows = table.select('tr')[1:]
    for row in rows:
        cols = [elem.innerText for elem in row.select('td')]
        print(cols)
