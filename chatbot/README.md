# Self reminders

`./src/app/tor -f src/app/torrc` runs tor using the specified torrc file.
* Listening port should be 80 or 443.
* Connecting local port can be whatever.
* Assuming `HiddenServiceDir` is set properly, it will look for `hs_ed25519_public_key`, `hs_ed25519_secret_key`, and `hostname`

Inside a `static` dir, create `.well-known/pki-validation` for harica DV cert validation

`python3 -m http.server 8080` serves local dir without any TLS

`openssl req -new -x509 -nodes -newkey ec:<(openssl ecparam -name secp384r1) -keyout cert.key -out cert.crt -days 30` generates a new cert
* `openssl ecparam -list_curves` didn't have all the curves I wanted.. secp384r1 seemed okay
* secp384r1 was based on https://soatok.blog/2022/05/19/guidance-for-choosing-an-elliptic-curve-signature-algorithm-in-2022/ and being available

`python3 ../https_server.py 8443 ../cert.crt ../cert.key` serves the local dir contents for static cert validation
* Mostly copied. I added the `getpass` bits so the TLS cert can be encrypted at rest

You can run `curl -k --socks5-hostname 127.0.0.1:9050 -v "YOUR_ONION_URL"`
* `-k` turns off cert validation, which is useful when testing TLS with a local cert prior to getting a harica cert

`python3 webhooks.py 8443 wut.pem privateKey.pem` runs the webhook consumer service
* Based off of https_server.py
* wut.pem was the PEM bundle from harica, including their intermediate certs, but manually removed the non-PEM contents

`OPENAI_API_KEY_FILE=/tmp/openai_api_key_file python3.11 chatgpt.py`
* Minimal example exercising https://platform.openai.com/docs/guides/gpt/chat-completions-api

`OPENAI_API_KEY_FILE=/tmp/openai_api_key_file LAST_RUN_FILE=/tmp/last_run_file PAGE_ID=108420048810326 PAGE_TOKEN_FILE=/tmp/page_token_file python3.11 facebook_loop.py`
* These env vars can be exported, this is just a single line command that is copy-pastable and runnable
