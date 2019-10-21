# runall.py
# Trevor Pottinger
# Fri Oct 18 21:13:24 PDT 2019

import asyncio

from sources.edgar import Edgar
from sources.wikipedia import Wikipedia


async def main() -> None:
    companies = await Wikipedia.gen_list_s_and_p_500()

    for company in companies[26:28]: # TODO remove [26:28]
        cik = int(company['cik'])
        search_url = Edgar.get_search_url(cik)
        company['search_url'] = search_url
        print(company)

        filings = await Edgar.gen_10ks(cik)
        for filing in filings[:2]:  # TODO remove [:2]
            filing['cik'] = company['cik']
            print(filing)
            documents = await Edgar.gen_documents(filing['url'])
            for document in documents:
                document['cik'] = company['cik']
                document['filing_date'] = filing['filing_date']
                if document['type'] != '10-K':
                    continue
                ten_k = await Edgar.gen_ten_k(document['url'])
                document['sha256'] = ten_k['sha256']
                print(document)


if __name__ == '__main__':
    loop = asyncio.get_event_loop()
    loop.run_until_complete(main())
