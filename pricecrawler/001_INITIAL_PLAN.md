# Price Checker Facebook Messenger Bot

Build a Facebook Messenger chatbot that checks product prices by scraping websites and using ChatGPT to parse/summarize results.

## Core Architecture

### 1. Configuration (`serve_config.py`)
- Load config from `SERVE_CONFIG` env var (JSON file path)
- Required fields: `facebook_app_id`, `facebook_app_secret`, `open_api_key`, `webhook_hostname`, `webhook_cert_file`, `webhook_key_file`
- Support multiple pages: each with `page_id`, `page_token`, `last_run` timestamp
- Support directories: `payment_sessions_dir`, `users_dir`, `conversations_dir`
- Payment link expiry: 24 hours

### 2. ChatGPT Integration (`chatgpt.py`)
- Call OpenAI chat completions API with configurable model
- Support models: gpt-4o, gpt-4o-mini, gpt-4.1, gpt-4.1-mini, gpt-5, gpt-5-mini (default: gpt-5)
- Track token usage (prompt, completion, total)
- Calculate and print costs based on pricing per million tokens (input/output rates)

### 3. Price Checker (`pricechecker.py`)
- Define price targets dict with categories: "12v lifepo batteries", "48v lifepo batteries", "priority bicycles", "solar panels", "inverters", "ecoflow river", "ecoflow delta", "3.2v lifepo cells", "openai api"
- Each target has URLs with DOM selectors (`kind`, `id`, `class`) to extract product listings
- Async fetch all URLs using aiohttp
- Parse HTML with BeautifulSoup to extract specified div/element
- Call ChatGPT to: (1) identify which target user is asking about, (2) summarize products with prices ascending
- Return summary object with list of summaries (one per URL)

### 4. Payment System (`session_storage.py`)
- Create payment sessions with 8-char hex session IDs
- Track user status: payment status (pending/completed), sessions, conversations
- Track conversation status: participants, payment sessions
- Cache messages by timestamp in conversation directories
- Support user data deletion (for privacy compliance)
- Mark payment as completed after Facebook payment callback

### 5. Webhook Server (`webhooks.py`)
- HTTPS server using ssl.SSLContext with cert/key files
- GET endpoints:
  - `/status` - health check
  - `/app-validation` and `/page-validation` - Facebook webhook verification (return hub.challenge)
  - `/pay?r={session_id}` - payment page using pay.html.tmpl
  - `/og/price-checker` - Open Graph meta tags for purchasable item
  - `/privacy-policy`, `/terms`, `/robots.txt` - static files
  - `/deleted?id={code}` - deletion confirmation
  - `/trigger-callback` - manual callback trigger
- POST endpoints:
  - `/payment-callback` - receive Facebook payment result, complete session, mark user paid
  - `/delete-me` - GDPR deletion with signature verification using HMAC-SHA256
- Serve templates with .format() substitution

### 6. Facebook Loop (`facebook_loop.py`)
- Main entry point: runs loop every 15 seconds + webhook server in parallel thread
- Per iteration:
  - Refresh page tokens to non-expiring if needed (cache expiry checks to avoid rate limits)
  - Fetch conversations updated in last 24 hours
  - Get recent messages for each conversation
  - Skip if last message from bot
  - Check payment status for all participants
  - If unpaid: post payment link (`https://{hostname}/pay?r={session_id}`)
  - If paid: call priceSummaries, post each summary to conversation
- Update `last_run` timestamp and save config

### 7. CLI Tool (`loop.py`)
- Interactive REPL for testing price checker
- Accept `-m/--model` flag for ChatGPT model selection
- Loop: prompt for product, call priceSummaries, display results

## Templates

### pay.html.tmpl
- Facebook payments dialog using FB SDK v24.0
- Check session status: invalid/expired → error, completed → "already paid", pending → show payment dialog
- Use FB.ui() with method='pay', action='purchaseitem'
- POST result to `/payment-callback` endpoint
- Template vars: `{YOUR_APP_ID}`, `{YOUR_REQUEST_ID}`, `{YOUR_STATUS}`

### product-price-checker.html.tmpl
- Open Graph meta tags for Facebook payments
- Product: "Price Checker License", $0.99 USD
- Image: barcode scanner webp
- Template var: `{YOUR_APP_ID}`

## Static Files

### requirements.txt
```
asyncio
aiohttp
beautifulsoup4
requests
```

### robots.txt
```
User-agent: *
Allow: /
```

### privacy-policy.txt
Privacy policy covering: Facebook Graph API minimal data access, AI API usage for price parsing, web requests for pricing data, no long-term storage, third-party services disclosure.

### terms-of-service.txt
Terms covering: eligibility (13+), acceptable use, pricing disclaimer (estimates only, verify before purchase), AS-IS service, liability limitations, indemnification, Facebook platform dependency, termination rights, Washington state governing law.

## Execution
Run with: `SERVE_CONFIG=/path/to/config.json python3 facebook_loop.py`
Config must have valid Facebook app credentials, OpenAI API key, SSL cert/key paths, and initialized page tokens.
