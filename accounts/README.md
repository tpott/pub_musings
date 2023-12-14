# accounts

Some scripts to help dump Plaid data into CSVs because Mint is shutting down

# Getting started

https://plaid.com/docs/api/tokens/#linktokencreate
`curl -X POST https://development.plaid.com/link/token/create -H 'Content-Type: application/json' -d '{"client_id": "'$CLIENT_ID'", "secret": "'$SECRET'", "user": {"client_user_id": "1"}, "client_name": "Trevors Budgeting App", "language": "en", "country_codes": ["US"], "products": ["transactions"]}'`

`cd static && python3 -m http.server 8080`

https://plaid.com/docs/api/tokens/#itempublic_tokenexchange
`curl -X POST https://development.plaid.com/item/public_token/exchange -H 'Content-Type: application/json' -d '{"client_id": "'$CLIENT_ID'", "secret": "'$SECRET'", "public_token": "public-development-5c224a01-8314-4491-a06f-39e193d5cddc"}'`

# Self signed cert

`openssl ecparam -list_curves`

`openssl req -new -x509 -nodes -newkey ec:<(openssl ecparam -name secp384r1) -keyout cert.key -out cert.crt -days 30`

`cd static && python3 ../https_server.py 8443 ../cert.crt ../cert.key`

Maybe just a https://letsencrypt.org/docs/challenge-types/#http-01-challenge ?

# Certbot

From one terminal:
`ssh -vNTR 80:localhost:80 $user@$domain`
* `-v` is for verbose output, which includes each request logging
* `-N` prevents us from running a remote command, cause we just want to create a proxy
* `-T` prevents pseudo-terminal from being created
* `-R` is the magic

From another terminal:
`sudo certbot certonly --standalone`
* `sudo` because certbot opens up port 80 -__-

Then, copy the cert out... And change the owner.

`cd static && python3 ../https_server.py 8443 ../fullchain.pem ../privkey.pem`

And from the first terminal, `ssh -vNTR 443:localhost:8443 root@home2.pottingers.us`
