# clouddns

Just some helpers to keep my cloudflare DNS records up to date

## get dns_records

```
curl --request GET --url https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records --header 'Content-Type: application/json' --header 'Authorization: Bearer $API_KEY' | jq '.result[]'
```

Where `$ZONE_ID` is from https://dash.cloudflare.com/ --> click on domain --> lower right hand side "Zone ID"

And `$API_KEY` is from https://dash.cloudflare.com/profile/api-tokens

## dig

```
dig +short $DOMAIN a
```

Where `$DOMAIN` is the domain we want to keep up

## cdn trace

```
curl "https://cloudflare.com/cdn-cgi/trace" | grep ip=
```

Returns the residential IP address, but with a `ip=` string prefix

# running

`python3 run_forever.py 60` with all the necessary files. `cat .gitignore` lists them out
