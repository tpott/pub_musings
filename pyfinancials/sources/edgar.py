# edgar.py
# Trevor Pottinger
# Fri Oct 18 21:48:50 PDT 2019

from typing import (Any, Dict, List)

import hashlib
import urllib.request


class Edgar(object):
    CHUNK_SIZE = 1000  # num bytes

    @classmethod
    async def gen_10ks(cls, cik: int) -> List[Dict[str, Any]]:
        cik_str = '%010d' % cik
        url = ('https://www.sec.gov/cgi-bin/browse-edgar' +
               '?action=getcompany&CIK=%s&type=10-k&owner=include' +
               '&count=40') % cik

        with open('source_data/%s' % cik_str, 'wb') as out_f:
            with urllib.request.urlopen(url) as f:
                chunk = f.read(cls.CHUNK_SIZE)
                while len(chunk) > 0:
                    out_f.write(chunk)
                    chunk = f.read(cls.CHUNK_SIZE)

        # Need to parse the table
        # Filings, Format (<a href>), Description, Filing Date, File/Film Number
        ret = [
            {'url': 'https://www.sec.gov/Archives/edgar/data/66740/000155837019000470/0001558370-19-000470-index.htm'}
            , {'url': 'https://www.sec.gov/Archives/edgar/data/66740/000155837018000535/0001558370-18-000535-index.htm'}
            , {'url': 'https://www.sec.gov/Archives/edgar/data/66740/000155837017000479/0001558370-17-000479-index.htm'}
        ]
        if cik != 66740:
            ret = []
        return ret

    @classmethod
    async def gen_documents(cls, url: str) -> List[Dict[str, Any]]:
        resp = b''
        hasher = hashlib.sha256()
        with urllib.request.urlopen(url) as f:
            chunk = f.read(cls.CHUNK_SIZE)
            while len(chunk) > 0:
                hasher.update(chunk)
                resp += chunk
                chunk = f.read(cls.CHUNK_SIZE)

        with open('source_data/%s' % hasher.hexdigest(), 'wb') as out_f:
            out_f.write(resp)

        # Need to parse the table
        # Seq, Description, Document (<a href>), Type, Size
        ret = [
            {'seq': 1, 'type': '10-K', 'url': 'https://www.sec.gov/Archives/edgar/data/66740/000155837018000535/mmm-20171231x10k.htm'}
        ]
        if url != 'https://www.sec.gov/Archives/edgar/data/66740/000155837018000535/0001558370-18-000535-index.htm':
            ret = []
        return ret

    @classmethod
    async def gen_ten_k(cls, url: str) -> List[Dict[str, Any]]:
        resp = b''
        hasher = hashlib.sha256()
        with urllib.request.urlopen(url) as f:
            chunk = f.read(cls.CHUNK_SIZE)
            while len(chunk) > 0:
                hasher.update(chunk)
                resp += chunk
                chunk = f.read(cls.CHUNK_SIZE)

        print('Fetching 10-K %s' % hasher.hexdigest())
        with open('source_data/%s' % hasher.hexdigest(), 'wb') as out_f:
            out_f.write(resp)


