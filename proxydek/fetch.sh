#!/bin/bash
# fetch.sh
# Sat Dec 31 23:24:08 PST 2016

mkdir -p download
pushd download
curl -O 'https://mtgjson.com/json/AllCards.json.zip'
curl -O 'https://mtgjson.com/json/AllCards-x.json.zip'
curl -O 'https://mtgjson.com/json/AllSets.json.zip'
curl -O 'https://mtgjson.com/json/AllSets-x.json.zip'
for i in *.zip; do unzip $i; done
rm *.zip
md5sum *
sha1sum *
sha256sum *
popd
