# pricecrawler

I want an easy http service to prompt some LLM to crawl some sites and check
prices of some products on those pages. The LLM should then provide a simple
message summarizing those prices.

Example 1:
```
Hey pricecrawler, can you check ecoflow prices?
```

And that should crawl
* https://us.ecoflow.com/collections/delta-series and
* https://us.ecoflow.com/collections/river-series

Example 2:
```
Can you check lifepo prices for me?
```

And that should crawl
* https://www.litime.com/collections/12v-batteries
* https://www.litime.com/collections/48v-batteries
* https://signaturesolar.com/all-products/batteries/?sort=priceasc
* https://www.wattcycle.com/collections/12v-batteries?sort_by=price-ascending

Example 3:
```
What about inverters?
```

I'm less familiar...
* https://signaturesolar.com/shop-all/inverters/hybrid-inverters/?sort=priceasc

## price target

I guess the first step is to identify the price target.

Each price target should have multiple URLs. Each URL can return multiple products.

# Commands

Setup
```
python3 -m venv .
source bin/activate
python3 -m pip install -r requirements.txt
```

Running
```
OPENAI_API_KEY_FILE=/tmp/key python3 loop.py
```
