# runall.py
# Trevor Pottinger
# Fri Oct 18 21:13:24 PDT 2019

import asyncio

from sources.edgar import Edgar
from sources.wikipedia import Wikipedia


async def main() -> None:
    companies = await Wikipedia.gen_list_s_and_p_500()

    for company in companies[:20]: # TODO remove [:20]
        print(company)
        cik = int(company[7])
        filings = await Edgar.gen_10ks(cik)
        for filing in filings[:4]:  # TODO remove [:4]
            print(filing)
            documents = await Edgar.gen_documents(filing['url'])
            for document in documents:
                if document['type'] != '10-K':
                    continue
                ten_k = await Edgar.gen_ten_k(document['url'])
                document['sha256'] = ten_k['sha256']
                print(document)


if __name__ == '__main__':
    loop = asyncio.get_event_loop()
    loop.run_until_complete(main())
