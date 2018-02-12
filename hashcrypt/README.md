# Hashcrypt

Hash-based symetric encryption

Disclaimer: This is meant to be a Proof-of-Concept and should not be used in any
production system.

The idea is to use a hash function, such as md5 or sha256, and cryptographically
secure key to create a keystream that can be used for symmetric encryption. I'm
using HMAC since it is well studied.
