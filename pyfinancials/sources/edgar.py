# edgar.py
# Trevor Pottinger
# Fri Oct 18 21:48:50 PDT 2019

from typing import (Any, Dict, List)

import hashlib
import urllib.request

try:
    from bs4 import BeautifulSoup
except ImportError:
    BeautifulSoup = None

class Edgar(object):
    CHUNK_SIZE = 1000  # num bytes

    @classmethod
    def get_search_url(cls, cik: int) -> str:
        cik_str = '%010d' % cik
        return ('https://www.sec.gov/cgi-bin/browse-edgar' +
                '?action=getcompany&CIK=%s&type=10-k&owner=include' +
                '&count=40') % cik_str

    @classmethod
    async def gen_10ks(cls, cik: int) -> List[Dict[str, Any]]:
        url = cls.get_search_url(cik)
        cik_str = '%010d' % cik
        hasher = hashlib.sha256()
        with open('source_data/%s.html' % cik_str, 'wb') as out_f:
            with urllib.request.urlopen(url) as f:
                chunk = f.read(cls.CHUNK_SIZE)
                while len(chunk) > 0:
                    hasher.update(chunk)
                    out_f.write(chunk)
                    chunk = f.read(cls.CHUNK_SIZE)

        rows = []
        if BeautifulSoup is not None:
            with open('source_data/%s.html' % cik_str, 'rb') as out_f:
                soup = BeautifulSoup(out_f.read(), features='html.parser')
                tables = soup.select('#seriesDiv > table')
                for table in tables:
                    if table.name != 'table':
                        continue
                    # Skip the header
                    rows = list(table.find_all('tr'))[1:]

        ret = []
        for row in rows:
            cols = list(row.find_all('td'))
            filings = cols[0].string
            rel_url = cols[1].find(id='documentsbutton')['href']
            documents_url = 'https://www.sec.gov%s' % rel_url
            # description
            filing_date = cols[3].string
            # file_num
            ret.append({
                'filings': filings,
                'filing_date': filing_date,
                'source_file': hasher.hexdigest(),
                'url': documents_url
            })

        # Example data looks like:
        # ret = [
            # {'url': 'https://www.sec.gov/Archives/edgar/data/66740/000155837019000470/0001558370-19-000470-index.htm'}
            # , {'url': 'https://www.sec.gov/Archives/edgar/data/66740/000155837018000535/0001558370-18-000535-index.htm'}
            # , {'url': 'https://www.sec.gov/Archives/edgar/data/66740/000155837017000479/0001558370-17-000479-index.htm'}
        # ]
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

        with open('source_data/%s.html' % hasher.hexdigest(), 'wb') as out_f:
            out_f.write(resp)

        rows = []
        if BeautifulSoup is not None:
            soup = BeautifulSoup(resp, features='html.parser')
            tables = soup.select('#formDiv > div > table')
            for table in tables:
                if table.name != 'table':
                    continue
                # Drop the header row
                rows = list(table.find_all('tr'))[1:]
                # And stop, because there's two tables that match and we just
                # want the first one.
                break

        ret = []
        for row in rows:
            cols = list(row.find_all('td'))
            seq = cols[0].string
            description = cols[1].string
            document_url = 'https://www.sec.gov%s' % cols[2].find('a')['href']
            document_type = cols[3].string
            size = cols[4].string
            ret.append({
                'description': description,
                'seq': seq,
                'size': size,
                'source_file': hasher.hexdigest(),
                'type': document_type,
                'url': document_url
            })

        # Example:
        # ret = [
            # {'seq': 1, 'type': '10-K', 'url': 'https://www.sec.gov/Archives/edgar/data/66740/000155837018000535/mmm-20171231x10k.htm'}
        # ]
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

        with open('source_data/%s.html' % hasher.hexdigest(), 'wb') as out_f:
            out_f.write(resp)

        return {'sha256': hasher.hexdigest()}


