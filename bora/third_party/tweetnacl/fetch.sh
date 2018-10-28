# fetch.sh

printf "curl -O 'https://tweetnacl.cr.yp.to/20140427/tweetnacl.c'\n"
curl -O 'https://tweetnacl.cr.yp.to/20140427/tweetnacl.c'
printf "curl -O 'https://tweetnacl.cr.yp.to/20140427/tweetnacl.h'\n"
curl -O 'https://tweetnacl.cr.yp.to/20140427/tweetnacl.h'

printf "sha256sum tweetnacl.{c,h}\n"
sha256sum tweetnacl.{c,h}
