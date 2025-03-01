# loop.py
# Mon Feb 24 20:44:17 PST 2025

from email.utils import formatdate
import os
import re
import time

import asyncio
import aiohttp
from bs4 import BeautifulSoup
import requests


# note the keys get passed as values of an enum to chatgpt
# target urls should be sorted with approximate prices ascending
targets = {
    "ecoflow": [
        {"url": "https://us.ecoflow.com/collections/river-series", "kind": "div", "id": "Collection"},
        {"url": "https://us.ecoflow.com/collections/delta-series", "kind": "div", "id": "Collection"},
    ],
    "inverters": [
        {
            "url": "https://signaturesolar.com/shop-all/inverters/hybrid-inverters/?sort=priceasc",
            "kind": "div",
            "id": "product-listing-container",
        },
    ],
    "lifepo batteries": [
        # TODO find a url for dumfume besides amazon
        # https://camelcamelcamel.com/product/B0DLGNJH8P?context=search
        {"url": "https://www.wattcycle.com/collections/12v-batteries?sort_by=price-ascending", "kind": "div", "id": "filter-results"},
        {"url": "https://www.litime.com/collections/12v-batteries", "kind": "div", "id": "CollectionProductGrid"},
        {"url": "https://www.litime.com/collections/48v-batteries", "kind": "div", "id": "CollectionProductGrid"},
        {"url": "https://signaturesolar.com/all-products/batteries/?sort=priceasc", "kind": "div", "id": "product-listing-container"},
    ],
    "solar panels": [
        {"url": "https://signaturesolar.com/shop-all/solar-panels/pallets/?sort=priceasc", "kind": "div", "id": "product-listing-container"},
    ],
}


def query_target(prompt):
    """Send user input to ChatGPT and return the response."""

    # Set up API key and endpoint
    API_KEY_FILE = os.getenv("OPENAI_API_KEY_FILE")
    if API_KEY_FILE is None:
        raise Exception("Failed reading OPENAI_API_KEY_FILE env var")
    API_KEY = open(API_KEY_FILE).read().strip()

    # Headers for API request
    HEADERS = {
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json"
    }

    price_targets = list(targets.keys())
    joined_targets = ", ".join(price_targets)

    data = {
        "model": "gpt-4o",  # Change to "gpt-3.5-turbo" if needed
        "messages": [
            {"role": "system", "content": f"Which of the price targets is the user asking about. Only respond with exactly one of the following price targets: {joined_targets}"},
            {"role": "user", "content": prompt}
        ],
        "temperature": 0.7
    }

    URL = "https://api.openai.com/v1/chat/completions"
    response = requests.post(URL, headers=HEADERS, json=data)

    if response.status_code == 200:
        result = response.json()
        return result["choices"][0]["message"]["content"].lower()
    else:
        return f"Error {response.status_code}: {response.text}"


def query_products(filtered_html_text):
    """Send filtered HTML to ChatGPT and return its summary."""

    # TODO librarize
    # Set up API key and endpoint
    API_KEY_FILE = os.getenv("OPENAI_API_KEY_FILE")
    if API_KEY_FILE is None:
        raise Exception("Failed reading OPENAI_API_KEY_FILE env var")
    API_KEY = open(API_KEY_FILE).read().strip()

    # Headers for API request
    HEADERS = {
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json"
    }

    price_targets = list(targets.keys())
    joined_targets = ", ".join(price_targets)

    data = {
        "model": "gpt-4o",  # Change to "gpt-3.5-turbo" if needed
        "messages": [
            # {"role": "system", "content": f"Which of the price targets is the user asking about. Only respond with exactly one of the following price targets: {joined_targets}"},
            {"role": "system", "content": f"Please summarize the products listed in this HTML. Include their prices (prefer sales price over real price or regular price). Please sort the products with prices ascending. Please do not repeat products."},
            {"role": "user", "content": filtered_html_text}
        ],
        "temperature": 0.7
    }

    URL = "https://api.openai.com/v1/chat/completions"
    response = requests.post(URL, headers=HEADERS, json=data)

    if response.status_code == 200:
        result = response.json()
        return result["choices"][0]["message"]["content"].lower()
    else:
        return f"Error {response.status_code}: {response.text}"

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


def main():
    """Loop to continuously prompt the user, query ChatGPT, and process responses."""
    print("ChatGPT Interactive (type 'exit' to quit)\n")
    
    while True:
        # Get user input
        try:
            user_input = input("product? ")
        except EOFError:
            print("Exiting")
            break
        
        # Exit condition
        if user_input.lower() in ["exit", "quit"]:
            print("Goodbye!")
            break

        # TODO how to associate `summarized` with `user_input`?
        # future `user_input` could be questions about previous `summarized`
        # Query ChatGPT
        target = query_target(user_input)
        if target not in targets:
            target_keys = list(targets.keys())
            print(f"Did not find one of {target_keys}, ChatGPT response: {target}\n")
            continue

        print(f"ChatGPT recognized: {target}\n")
        # targets[target] is a list of []{url, kind, id}
        # TODO sort results by the order of urls from targets[target]
        results = asyncio.run(fetch_all(target, targets[target]))
        for target, target_obj, status, content in results:
            url = target_obj["url"]
            print(f"Start URL: {url}")
            print(f"Now in UTC: {formatdate(time.time())}")
            print(f"Status: {status}")
            print(f"Content-Length: {len(content)}")

            soup = BeautifulSoup(content, "html.parser")

            if "kind" not in target_obj:
                print(f"Target: {target} doesn't have dom element kind (type)\n")
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
                print(f"Failed to find \"{css_id}\" in the results. Content: {content[:1000]}")
                continue
            text = results.text.strip()
            output_text = re.sub(r' +', ' ', text)
            output_text = re.sub(r'[\n\t]+', '\n', output_text)
            print(f"Div length: {len(text)}\nDiv Content: {output_text[:20]}")

            # TODO how to associate `summarized` with `user_input`?
            # future `user_input` could be questions about previous `summarized`
            summarized = query_products(output_text)
            print(f"Summarized products: {summarized}\n")
            print(f"End URL: {url}\n")
       

if __name__ == "__main__":
    main()

