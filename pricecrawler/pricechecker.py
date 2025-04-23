# pricechecker.py
# Sun Mar  2 06:38:18 PST 2025

from email.utils import formatdate
import re
import time

import asyncio
import aiohttp
from bs4 import BeautifulSoup

from chatgpt import chatCompletitions


# note the keys get passed as values of an enum to chatgpt
# target urls should be sorted with approximate prices ascending

# kind div --> find a div, usually by id
# this should be the top level HTML DOM element that contains all the products on that page
# it will ultimately be passed to bsoup.find(...)
targets = {
    "3.2v lifepo cells": [
        # class product-list--collection, class product-list, class boost-pfs-filter-products
        # all seemed like possible options
        {"url": "https://www.18650batterystore.com/collections/lifepo4-prismatic-cells", "kind": "div", "class": "product-list--collection"},
    ],
    "12v lifepo batteries": [
        # TODO find a url for dumfume besides amazon
        # https://camelcamelcamel.com/product/B0DLGNJH8P?context=search
        {"url": "https://www.wattcycle.com/collections/12v-batteries?sort_by=price-ascending", "kind": "div", "id": "filter-results"},
        {"url": "https://www.litime.com/collections/12v-batteries", "kind": "div", "id": "CollectionProductGrid"},
    ],
    "48v lifepo batteries": [
        {"url": "https://www.litime.com/collections/48v-batteries", "kind": "div", "id": "CollectionProductGrid"},
        {"url": "https://signaturesolar.com/all-products/batteries/?sort=priceasc", "kind": "div", "id": "product-listing-container"},
    ],
    # TODO chatgpt requires pupeteer
    "openai api": [
        # {"url": "https://openai.com/api/pricing/", "kind": "main", "id": "main"},
        # {"url": "https://openai.com/api/pricing/", "kind": "div", "class": "flex"},
        # {"url": "https://platform.openai.com/docs/pricing", "kind": "class", "class": "mb-8"},
        {"url": "https://platform.openai.com/docs/pricing"},
    ],
    "ecoflow river": [
        {"url": "https://us.ecoflow.com/collections/river-series", "kind": "div", "id": "Collection"},
    ],
    "ecoflow delta": [
        {"url": "https://us.ecoflow.com/collections/delta-series", "kind": "div", "id": "Collection"},
    ],
    "inverters": [
        {
            "url": "https://signaturesolar.com/shop-all/inverters/hybrid-inverters/?sort=priceasc",
            "kind": "div",
            "id": "product-listing-container",
        },
    ],
    "priority bicycles": [
        {"url": "https://www.prioritybicycles.com/collections/bicycles-1", "kind": "div", "class": "collection-grid_products"},
    ],
    "solar panels": [
        {"url": "https://signaturesolar.com/shop-all/solar-panels/pallets/?sort=priceasc", "kind": "div", "id": "product-listing-container"},
    ],
}


async def fetch_url(target, session, target_obj):
    """Fetch URL using async HTTP request."""
    try:
        async with session.get(target_obj["url"], timeout=10) as response:
            text = await response.text()
            return target, target_obj, response.status, text
    except Exception as e:
        return url, None, str(e)


async def fetch_all(target, target_obj):
    """Fetch multiple URLs asynchronously."""
    async with aiohttp.ClientSession() as session:
        tasks = [fetch_url(target, session, obj) for obj in target_obj]
        return await asyncio.gather(*tasks)


def priceSummaries(user_input, model):
    """Return a price summary object given a user message asking about a price target.
    The summary object will contain a summaries list. One object for each crawled URL."""
    ret = {'user_input': user_input}

    # TODO how to associate `summarized` with `user_input`?
    # future `user_input` could be questions about previous `summarized`
    # Query ChatGPT
    price_targets = list(targets.keys())
    joined_targets = ", ".join(price_targets)
    messages = [
        {"role": "system", "content": f"Which of the price targets is the user asking about. Only respond with exactly one of the following price targets: {joined_targets}"},
        {"role": "user", "content": user_input}
    ]
    # gpt-4.1-mini is 1/5 the price of gpt-4.1
    target = chatCompletitions(messages, model if model is not None else "gpt-4.1-mini")
    if target not in targets:
        ret['error'] = f"Did not find one of {price_targets}, ChatGPT response: {target}\n"
        return ret

    ret['target'] = target
    ret['summaries'] = []
    print(f"ChatGPT recognized: {target}\n")
    # targets[target] is a list of []{url, kind, id}
    # TODO sort results by the order of urls from targets[target]
    results = asyncio.run(fetch_all(target, targets[target]))
    for target, target_obj, status, content in results:
        url = target_obj["url"]
        print(f"Start URL: {url}")
        print(f"Now in UTC: {formatdate(time.time(), localtime=False)}")
        print(f"Now in localtime: {formatdate(time.time(), localtime=True)}")
        print(f"Status: {status}")
        print(f"Content-Length: {len(content)}")

        soup = BeautifulSoup(content, "html.parser")

        url = None
        if "url" in target_obj:
            url = target_obj["url"]

        if "kind" not in target_obj:
            print(f"Target: {target} doesn't have dom element kind (type), maybe just url {url}\n")
            continue

        kind = target_obj["kind"]
        kwargs = {}
        if "id" in target_obj:
            kwargs["id"] = target_obj["id"]
        if "class" in target_obj:
            kwargs["class"] = target_obj["class"]

        # TODO configure this, filter-results for wattcycle, Collection for ecoflow
        # results = soup.find("div", id="filter-results") # wattcycle
        # results = soup.find("div", id="Collection") # ecoflow
        results = soup.find(target_obj["kind"], **kwargs)
        if results is None:
            # TODO we return a list of summaries, but this is writing to one shared error...
            ret['error'] = f"Failed to find HTML id/class corresponding to {target} for {url}"
            print(f"Failed to find \"{kwargs}\" in the results. Content: {content[:1000]}")
            continue  # maybe some other URL will work
        text = results.text.strip()
        output_text = re.sub(r' +', ' ', text)
        output_text = re.sub(r'[\n\t]+', '\n', output_text)
        print(f"Div length: {len(text)}\nDiv Content: {output_text[:20]}")

        # TODO how to associate `summarized` with `user_input`?
        # future `user_input` could be questions about previous `summarized`
        messages = [
            {"role": "system", "content": f"Please summarize the products listed in this HTML. Include their prices (prefer sales price over real price or regular price). Please sort the products with prices ascending. Please do not repeat products."},
            {"role": "user", "content": output_text}
        ]
        summarized = chatCompletitions(messages, model)
        print(f"Summarized products: {summarized}\n")
        print(f"End URL: {url}\n")
        ret['summaries'].append({
            'summary_text': summarized,
            'target': target,
            'url': url,
        })
    return ret
