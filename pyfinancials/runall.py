# runall.py
# Trevor Pottinger
# Fri Oct 18 21:13:24 PDT 2019

import argparse
import asyncio
import json
from typing import Optional

from sources.edgar import Edgar
from sources.wikipedia import Wikipedia


async def main(min_i: int, max_i: Optional[int], num_filings: int) -> None:
    accepted_docs = ['10-K', '10-K405']
    companies = await Wikipedia.gen_list_s_and_p_500()
    if max_i is None:
        max_i = len(companies)

    for company in companies[min_i:max_i]:
        cik = int(company['cik'])
        search_url = Edgar.get_search_url(cik)
        company['search_url'] = search_url
        print(json.dumps(company))

        filings = await Edgar.gen_10ks(cik)
        for filing in filings[:num_filings]:
            filing['cik'] = company['cik']
            print(json.dumps(filing))
            documents = await Edgar.gen_documents(filing['url'])
            for document in documents:
                document['cik'] = company['cik']
                document['filing_date'] = filing['filing_date']
                if document['type'] not in accepted_docs:
                    continue
                ten_k = await Edgar.gen_ten_k(document['url'])
                document['sha256'] = ten_k['sha256']
                print(json.dumps(document))
            # end document loop
        # end filing loop
    # end company loop


if __name__ == '__main__':
    desc = 'Fetch 10-Ks from EDGAR for the S&P 500'
    parser = argparse.ArgumentParser(description=desc)
    parser.add_argument(
        '--min_i',
        type=int,
        default=0,
        help='Where to slice the list of S&P 500 from'
    )
    parser.add_argument(
        '--max_i',
        type=int,
        help='Where to slice the list of S&P 500 to'
    )
    parser.add_argument(
        '--num_filings',
        type=int,
        default=5,
        help='The max number of filings per company to fetch'
    )
    args = parser.parse_args()

    loop = asyncio.get_event_loop()
    loop.run_until_complete(main(
        args.min_i,
        args.max_i,
        args.num_filings
    ))
