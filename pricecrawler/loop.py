# loop.py
# Mon Feb 24 20:44:17 PST 2025

import os
import re

import asyncio
import aiohttp
from bs4 import BeautifulSoup
import requests


targets = {
    "ecoflow": [
        "https://us.ecoflow.com/collections/delta-series",
        "https://us.ecoflow.com/collections/river-series",
    ],
    "lifepo batteries": [
        "https://www.litime.com/collections/12v-batteries",
        "https://www.litime.com/collections/48v-batteries",
        "https://signaturesolar.com/all-products/batteries/?sort=priceasc",
        "https://www.wattcycle.com/collections/12v-batteries?sort_by=price-ascending",
    ],
    "inverters": [
        "https://signaturesolar.com/shop-all/inverters/hybrid-inverters/?sort=priceasc",
    ],
}


def query_target(prompt):
    """Send user input to ChatGPT and return the response."""

    # Set up API key and endpoint
    API_KEY_FILE = os.getenv("OPENAI_API_KEY_FILE")
    if API_KEY_FILE is None:
        raise Exception("Failed reading OPENAI_API_KEY_FILE env var")
    API_KEY = open(API_KEY_FILE).read().strip()

    URL = "https://api.openai.com/v1/chat/completions"

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

    response = requests.post(URL, headers=HEADERS, json=data)

    if response.status_code == 200:
        result = response.json()
        return result["choices"][0]["message"]["content"].lower()
    else:
        return f"Error {response.status_code}: {response.text}"


async def fetch_url(session, url):
    """Fetch URL using async HTTP request."""
    try:
        async with session.get(url, timeout=10) as response:
            text = await response.text()
            return url, response.status, text
    except Exception as e:
        return url, None, str(e)


async def fetch_all(urls):
    """Fetch multiple URLs asynchronously."""
    async with aiohttp.ClientSession() as session:
        tasks = [fetch_url(session, url) for url in urls]
        return await asyncio.gather(*tasks)


def main():
    """Loop to continuously prompt the user, query ChatGPT, and process responses."""
    print("ChatGPT Interactive (type 'exit' to quit)\n")
    
    while True:
        # Get user input
        try:
            user_input = input("You: ")
        except EOFError:
            print("Exiting")
            break
        
        # Exit condition
        if user_input.lower() in ["exit", "quit"]:
            print("Goodbye!")
            break

        # Query ChatGPT
        target = query_target(user_input)
        print(f"ChatGPT: {target}\n")
        if target not in targets:
            continue

        results = asyncio.run(fetch_all(targets[target]))
        for url, status, content in results:
            print(f"URL: {url}\nStatus: {status}\nContent-Length: {len(content)}")
            soup = BeautifulSoup(content, "html.parser")
            # TODO configure this, filter-results for wattcycle, Collection for ecoflow
            # results = soup.find("div", id="filter-results")
            results = soup.find("div", id="Collection")
            if results is None:
                print(f"Failed to find filter-results. Content: {content[:1000]}")
                continue
            text = results.text.strip()
            output_text = re.sub(r' +', ' ', text)
            output_text = re.sub(r'[\n\t]+', '\n', output_text)
            print(f"Div length: {len(text)}\nDiv: {output_text[:1000]}...")
       

if __name__ == "__main__":
    main()

