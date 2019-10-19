# wikipedia.py
# Trevor Pottinger
# Fri Oct 18 21:10:08 PDT 2019

from typing import (Any, Dict, List)

import urllib.request


class Wikipedia(object):
    CHUNK_SIZE = 1000  # num bytes

    @classmethod
    async def gen_list_s_and_p_500(cls) -> List[Dict[str, Any]]:
        url = 'https://en.wikipedia.org/wiki/List_of_S%26P_500_companies'

        # want: https://aiohttp.readthedocs.io/en/stable/
        with open('source_data/s_and_p_500_list', 'wb') as out_f:
            with urllib.request.urlopen(url) as f:
                chunk = f.read(cls.CHUNK_SIZE)
                while len(chunk) > 0:
                    out_f.write(chunk)
                    chunk = f.read(cls.CHUNK_SIZE)

        # Need to parse the table
        # Symbol, Security, SEC filings (<a href>), GICS Sector, GICS Sub Industry, Headquarters Location, Date first added, CIK, Founded
        ret = [
            ('MMM', '3M Company', 'reports', 'Industrials', 'Industrial Conglomerates', 'St. Paul, Minnesota', '', '0000066740', '1902')
            , ('ATVI', 'Activision Blizzard', 'reports', 'Communication Services', 'Interactive Home Entertainment', 'Santa Monica, California', '2015-08-31', '0000718877', '2008')
            , ('ALK', 'Alaska Air Group Inc', 'reports', 'Industrials', 'Airlines', 'Seattle, Washington', '2016-05-13', '0000766421', '1985')
        ]
        return ret
