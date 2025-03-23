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
mkdir -p tmp
python3 -m venv tmp
source tmp/bin/activate
python3 -m pip install -r requirements.txt
```

And
```
brew install cloudflared
```

Running
```
OPENAI_API_KEY_FILE=/tmp/key python3 loop.py
```

Cleanup
```
deactivate
```

# Notes

* I copied `webhooks.py`, `chatgpt.py`, and `facebook_loop.py` from my pub_musings/chatbot project
* I was able to create a self signed cert via `openssl req -new -x509 -nodes -newkey ec:<(openssl ecparam -name secp384r1) -keyout cert.key -out cert.crt -days 30`
* I was able to create a new cloudflare tunnel and set it up with `https://localhost:8443`
* I then ran `python3 webhooks.py 8443 cert.crt cert.key` in order to verify ownership with facebook
* I used the graph API explorer to figure out my app scoped page ID https://developers.facebook.com/tools/explorer/ by calling `/me` with a page token
* I was then able to plug everything together with `WEBHOOK_CERT_FILE=cert.crt WEBHOOK_KEY_FILE=cert.key OPENAI_API_KEY_FILE=/tmp/key LAST_RUN_FILE=/tmp/last_run_file PAGE_ID=506755539197493 PAGE_TOKEN_FILE=/tmp/page_token_file python3 facebook_loop.py`
* I haven't gotten webhooks to work yet though... If it just triggers the loop, then it will work, so I literally just need any webhook.

# Saturday Fun

Playing with `cat prompts.txt | pbcopy` -> chatgpt o3. The result is python scavengerhunt.py:

```
GCLOUD_API_KEY_FILE=/tmp/gcloud OPENAI_API_KEY_FILE=/tmp/openai python scavengerhunt.py
```
