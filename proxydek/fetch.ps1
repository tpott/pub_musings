# fetch.ps1
# Tue Apr 11 21:47:32 PDT 2017

pushd download
curl 'https://mtgjson.com/json/AllCards.json.zip' -OutFile 'AllCards.json.zip'
curl 'https://mtgjson.com/json/AllCards-x.json.zip' -OutFile 'AllCards-x.json.zip'
curl 'https://mtgjson.com/json/AllSets.json.zip' -OutFile 'AllSets.json.zip'
curl 'https://mtgjson.com/json/AllSets-x.json.zip' -OutFile 'AllSets-x.json.zip'
# unzip, rm, md5sum, sha1sum, sha256sum
ForEach ($i in (Get-ChildItem *.zip)) {
  unzip -o $i
  rm $i
}
# Iterate over each file for each command to be the same as fetch.sh
ForEach ($i in (Get-ChildItem *)) { md5sum $i }
ForEach ($i in (Get-ChildItem *)) { sha1sum $i }
ForEach ($i in (Get-ChildItem *)) { sha256sum $i }
popd
