# devbot

A simple chatbot inspired by claude.ai/code with the biggest difference is that
devbot can leverage my local compute resources.

I'm leveraging some patterns I established in pricecrawler/. Namely:
1. loop.py should be the simplest way I build, debug, iterate.
2. matrix_loop.py should be one step up that gets me something that looks like
   a chatbot to most people.
3. whatsapp_loop.py will mean game time. I could eventually also expand to
   slack, discord, SMS, etc, but whatsapp is what I prefer and use so I'll use
   it as my level 3 goal.

## Install

```bash
# assumes `python` resolves to python3.9+ and that venv is installed
python -m venv .venv

source .venv/bin/activate

# CFLAGS and LDFLAGS are to help pip find libolm on the system
CFLAGS="-I/opt/homebrew/include" LDFLAGS="-L/opt/homebrew/lib" python -m pip install -r requirements.txt
```

## Running

This assumes you have written your config.json to data/ ahead of time.

```bash
SERVE_CONFIG=data/config.json python loop.py
```
