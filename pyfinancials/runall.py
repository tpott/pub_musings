# runall.py
# Trevor Pottinger
# Fri Oct 18 21:13:24 PDT 2019

import asyncio

from sources.edgar import Edgar
from sources.wikipedia import Wikipedia


async def main() -> None:
    companies = await Wikipedia.gen_list_s_and_p_500()

    for company in companies[:7]: # TODO remove [:7]
        print(company)
        cik = int(company[7])
        filings = await Edgar.gen_10ks(cik)
        for filing in filings[:3]:  # TODO remove [:3]
            print(filing)
            documents = await Edgar.gen_documents(filing['url'])
            for document in documents:
                if document['type'] != '10-K':
                    continue
                ten_k = await Edgar.gen_ten_k(document['url'])


if __name__ == '__main__':
    loop = asyncio.get_event_loop()
    loop.run_until_complete(main())
