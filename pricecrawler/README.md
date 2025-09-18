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
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
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
  * `/tmp/key` originally came from https://platform.openai.com/settings/organization/api-keys
  * but openai doesn't allow for re-downloading the key more than once
  * I saved it in lastpass
* I haven't gotten webhooks to work yet though... If it just triggers the loop, then it will work, so I literally just need any webhook.

# Saturday Fun

Playing with `cat prompts.txt | pbcopy` -> chatgpt o3. The result is python scavengerhunt.py:

```
GCLOUD_API_KEY_FILE=/tmp/gcloud OPENAI_API_KEY_FILE=/tmp/openai python scavengerhunt.py
```

Adding `-v` at the end includes some extra debugging bits...

This isn't really working after a night of tinkering. I've tried prompting
`coffee near {home}` and `handle_search` prints _something_. But it doesn't seem
to find the right search results, when I compare with https://www.google.com/maps

* I wish the `tool_*` functions were using a python function decorator. I'm not sure what
  it would be useful for, but I think it would better separate out what the tool functions
  are.
* The `handle_chat_loop` function should include the tool function names, parameters, types
  and descriptions in the "system" prompt.
* The `text_search` action case in `handle_chat_loop` is overly nested...
* The "remember" tool should remember a location, write it to a file. "Recall" would then
  read the file to include previously remembered locations.
* https://developers.google.com/maps/documentation/places/web-service/supported_types#table1
  these enums seem nice, but idk if I want to include them in a prompt?

It's crazy that I'm basically asking chatgpt to re-write this script from scratch
every time. It's not _completely_ from scratch, but almost. It uses the chat history
to see previous versions of the python. Older versions of the python script can fall
out of context. But I re-state the entire prompt every time so that it doesn't fall
out of context.

Moving to scavengerhunt/

# Operationalize

Skip the above saturday fun... it's quite different from base pricechecker

I started in the [Notes](/#Notes) section, but that required me to run [Commands](#Commands).

I logged into cloudflare.com, clicked "Zero Trust" on the left, then "Networks", then "Tunnels".
I found my `prices1` tunnel still running o.O but `brew services list` and 
`brew services info cloudflared` didn't show it as running. I tried `sudo cloudflared service install ...`
but it failed because it was already running (?). So I ran `sudo cloudflared service uninstall` and
tried again, and that worked.

I had to add `model='gpt-4.1'` to my `priceSummaries(...)` call in `facebook_loop.py`.

# Extending FB access tokens

I added `https://localhost:8443/trigger-callback` to work around FB's webhooks not working.

## Starting with a user token

Get a long lived user token:
```
curl -X GET "https://graph.facebook.com/oauth/access_token?grant_type=fb_exchange_token&client_id=$APP_ID&client_secret=$APP_SECRET&fb_exchange_token=$SHORT_LIVED_USER_TOKEN"
```

Then get a never expiring page token:
```
curl -X GET "https://graph.facebook.com/$PAGE_ID?fields=access_token&access_token=$LONG_LIVED_USER_TOKEN"
```

## Starting with a page token

Get a long lived page token:
```
curl -X GET "https://graph.facebook.com/oauth/access_token?grant_type=fb_exchange_token&client_id=$APP_ID&client_secret=$APP_SECRET&fb_exchange_token=$SHORT_LIVED_PAGE_TOKEN"
```

## Verify

You can verify qualities of your token with:
```
curl -X GET "https://graph.facebook.com/debug_token?input_token=$YOUR_TOKEN&access_token=$APP_ID|$APP_SECRET"
```

And just check your page ID:
```
curl -X GET "https://graph.facebook.com/me/accounts?access_token=$USER_TOKEN"
```

## Adding deletion

```
OPENAI_API_KEY_FILE=/tmp/openai LAST_RUN_FILE=/tmp/last_run_file PAGE_TOKEN_FILE=/tmp/page_token python3 facebook_loop.py
```

runs in a loop, so it avoids the whole broken Facebook webhooks issue. Ideally I would be running

```
WEBHOOK_CERT_FILE=cert.crt WEBHOOK_KEY_FILE=cert.key OPENAI_API_KEY_FILE=/tmp/openai LAST_RUN_FILE=/tmp/last_run_file PAGE_TOKEN_FILE=/tmp/page_token python3 facebook_loop.py
```

I need to test:
```
SIGNATURE="sha256=$(echo -n '{"user_id": "test_user_123"}' | openssl dgst -sha256 -hmac "$(cat $FACEBOOK_APP_SECRET_FILE)" | cut -d' ' -f2)"
curl -X POST -k -H "Content-Type: application/json" -H "X-Hub-Signature-256: $SIGNATURE" -d '{"user_id": "test_user_123"}' "https://localhost:8443/delete-me"
```

I set a debugger breakpoint with:
```
FACEBOOK_APP_ID=4040664086219677 FACEBOOK_APP_SECRET_FILE=/tmp/facebook WEBHOOK_CERT_FILE=cert.crt WEBHOOK_KEY_FILE=cert.key OPENAI_API_KEY_FILE=/tmp/openai LAST_RUN_FILE=/tmp/last_run_file PAGE_TOKEN_FILE=/tmp/page_token2 python3 -m pdb facebook_loop.py
```

## State of affairs

GET Request Handlers (in getHandler() method in webhooks.py):

1. /privacy-policy - serves privacy policy text file
2. /robots.txt - serves robots.txt file
3. /status - returns "1-AM-ALIVE" status response
4. /terms - serves terms of service text file
5. /trigger-callback - triggers a callback function if set
6. /validation - handles Facebook webhook validation
7. /deleted - handles deletion confirmation requests
8. Default handler for unknown pages - returns "Unknown page" 404

POST Request Handlers (in postHandler() method):

9. /delete-me - handles user deletion requests with signature verification
10. /validation - returns error for POST requests (validation should be GET only)
11. Default handler for unknown POST pages - returns "Unknown page" 404

I don't actually receive any webhooks, even test ones, because the app isn't yet published.
To publish the app, I need to take it through App Review. I don't know if I need to add a
static web page (ex: /setup-page) so that I can authorize other pages to leverage the app?
This would mean replacing all uses of PAGE_TOKEN_FILE to use a dynamic page token... Do I
want/need to add another static web page that enables people to pay (ex: /pay?id=abcd)?
https://developers.facebook.com/docs/games_payments looks reasonable. Based on
https://developers.facebook.com/docs/games_payments/taking-payments#setting_up I think the
static web page needs to define the "product" that the customer is paying for. 
