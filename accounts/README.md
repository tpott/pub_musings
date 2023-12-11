# accounts

Some scripts to help dump Plaid data into CSVs because Mint is shutting down

# Getting started

https://plaid.com/docs/api/tokens/#linktokencreate
`curl -X POST https://development.plaid.com/link/token/create -H 'Content-Type: application/json' -d '{"client_id": "'$CLIENT_ID'", "secret": "'$SECRET'", "user": {"client_user_id": "1"}, "client_name": "Trevors Budgeting App", "language": "en", "country_codes": ["US"], "products": ["transactions"]}'`

`python3 -m http.server 8080`

https://plaid.com/docs/api/tokens/#itempublic_tokenexchange
`curl -X POST https://development.plaid.com/item/public_token/exchange -H 'Content-Type: application/json' -d '{"client_id": "'$CLIENT_ID'", "secret": "'$SECRET'", "public_token": "public-development-5c224a01-8314-4491-a06f-39e193d5cddc"}'`
