# wikipedia.py
# Trevor Pottinger
# Fri Oct 18 21:10:08 PDT 2019

from typing import (Any, Dict, List)

import urllib.request

try:
    from bs4 import BeautifulSoup
except ImportError:
    BeautifulSoup = None


class Wikipedia(object):
    CHUNK_SIZE = 1000  # num bytes

    @classmethod
    async def gen_list_s_and_p_500(cls) -> List[Dict[str, Any]]:
        url = 'https://en.wikipedia.org/wiki/List_of_S%26P_500_companies'

        # want: https://aiohttp.readthedocs.io/en/stable/
        with open('source_data/s_and_p_500_list.html', 'wb') as out_f:
            with urllib.request.urlopen(url) as f:
                chunk = f.read(cls.CHUNK_SIZE)
                while len(chunk) > 0:
                    out_f.write(chunk)
                    chunk = f.read(cls.CHUNK_SIZE)

        soup = None
        rows = []
        if BeautifulSoup is not None:
            with open('source_data/s_and_p_500_list.html', 'rb') as out_f:
                soup = BeautifulSoup(out_f.read(), features='html.parser')
                tables = soup.find(id='constituents')
                for elem in tables:
                    if elem.name != 'tbody':
                        continue
                    # Skip the header row!
                    rows = list(elem.find_all('tr'))[1:]
                print('found', len(rows))

        ret = []
        for row in rows:
            cols = row.find_all('td')
            symbol = list(cols[0].strings)[0]
            security = list(cols[1].strings)[0]
            # filings url
            sector = cols[3].string
            sub_sector = cols[4].string
            location = list(cols[5].strings)[0]
            date_added = cols[6].string
            cik = cols[7].string.strip()
            founded = cols[8].string.strip()
            ret.append((symbol, security, 'reports', sector, sub_sector,
                location, date_added, cik, founded))

        # Examples:
        # ret = [
            # ('MMM', '3M Company', 'reports', 'Industrials', 'Industrial Conglomerates', 'St. Paul, Minnesota', '', '0000066740', '1902')
            # , ('ATVI', 'Activision Blizzard', 'reports', 'Communication Services', 'Interactive Home Entertainment', 'Santa Monica, California', '2015-08-31', '0000718877', '2008')
            # , ('ALK', 'Alaska Air Group Inc', 'reports', 'Industrials', 'Airlines', 'Seattle, Washington', '2016-05-13', '0000766421', '1985')
        # ]
        return ret
