# hashcrypt.py
# Trevor Pottinger
# Sat Mar 28 11:13:14 PDT 2015

from __future__ import print_function

import binascii
import hashlib
import hmac
import os
import struct

# keysize is in number of bytes
keysize = 32
hash_func = hashlib.sha256

# Realistically iv size and signature size don't have to be the same size
# as the encryption key.

hexB = lambda s: binascii.hexlify(s)
unHexB = lambda s: binascii.unhexlify(s)

def pad(unpadded):
  """Takes a string and adds at least one byte, and at most /keysize/
  bytes to the end of the unpadded string. Each byte value will be the
  number of padded bytes."""
  n_padded_bytes = keysize - (len(unpadded) % keysize)
  padding = struct.pack('B', n_padded_bytes) * n_padded_bytes
  return unpadded + padding

def unpad(padded):
  n_padded_bytes = ord(padded[-1])
  assert n_padded_bytes > 0, '%d is not greater than zero' % n_padded_bytes
  assert n_padded_bytes <= keysize, '%d is not less than or equal /keysize/' % keysize
  return padded[:-n_padded_bytes]

def isPadded(text):
  n_padding = ord(text[-1])
  maybe_padded = n_padding > 0 and n_padding < keysize
  if not maybe_padded:
    return False
  return plaintext[-n_padding:] == (chr(n_padding) * n_padding)

def encrypt(plaintext, key):
  """Defines the implementation for hash-based symetric encryption. Note that
  this is intentionally non-deterministic, it uses urandom for an iv."""
  encryption_key = key[:keysize]
  signing_key = key[keysize:]

  if not isPadded(plaintext):
    plaintext = pad(plaintext)

  iv = os.urandom(keysize)
  signer = hmac.HMAC(signing_key, iv, hash_func)
  source = hmac.HMAC(encryption_key, '', hash_func)

  def nextKey(curkey):
    "Helper function - closure for source"
    hash_obj = source.copy()
    hash_obj.update(curkey)
    return hash_obj.digest()

  ciphertext = iv
  keystream = nextKey(iv)
  for i in range(0, len(plaintext), keysize):
    block = plaintext[i:i+keysize]
    encrypted_block = ''.join(
        [ chr(ord(block[i]) ^ ord(keystream[i])) for i in range(keysize) ]
    )
    ciphertext += encrypted_block
    signer.update(encrypted_block)
    keystream = nextKey(keystream)

  ciphertext += signer.digest()
  # TODO key rotation?? 

  return ciphertext

def decrypt(ciphertext, key):
  encryption_key = key[:keysize]
  signing_key = key[keysize:]

  iv = ciphertext[:keysize]
  cipherbody = ciphertext[keysize:-keysize]
  assert len(cipherbody) % keysize == 0, 'Incorrect cipherbody len %d' % len(cipherbody)
  signature = ciphertext[-keysize:]

  signer = hmac.HMAC(signing_key, iv, hash_func)
  source = hmac.HMAC(encryption_key, '', hash_func)

  def nextKey(curkey):
    "Helper function - closure including source"
    hash_obj = source.copy()
    hash_obj.update(curkey)
    return hash_obj.digest()

  plaintext = ''
  keystream = nextKey(iv)
  for i in range(0, len(cipherbody), keysize):
    block = cipherbody[i:i+keysize]
    plaintext_block = ''.join(
        [ chr(ord(block[i]) ^ ord(keystream[i])) for i in range(keysize) ]
    )
    plaintext += plaintext_block
    signer.update(block)
    keystream = nextKey(keystream)

  derived_signature = signer.digest()
  # Ideally this would use a constant-time comparison
  assert derived_signature == signature, 'Signature mismatch: %s' % hexB(derived_signature)
  
  return unpad(plaintext)
  
if __name__ == '__main__':
  # derived from hexB(os.urandom(keysize * 2)
  key = unHexB('8f7af27fe1175d348334578b58f4a8cafec0ec076b383c2cf6b6e9c710998463dc4716feb7d012ed4d94464725e0fad6de36243d7e82290e393503d2177c8e84')
  print(hexB(key))
  ciphertext = unHexB('38eebde7aaf4a68cd1ef1c468845ed233ddcf260708d4ac47978ffd9f05f74515347d6b105ea54b339400957b4c501b34584d4dab1f5e3f781e7d7995edcd4c6200f1018df52b7a409f16ceb08156b18640be0efb352fa5b7a3a245dd9b5fce235e6e0433d61be0842f837f07d1c069a511d1833e71c1e5fae7beda63d25055bd1f2fd72d2f9f8388077f3c1abf957b41fbca3cea447a7ac5ca960b972d0b8858875369198801263f485e08f8450b0d9644b6bebf6c1ecc6d5b4995a2ef3d41f7da4500a2f20224e45fc04c38199b4faa5408d833883874fae5190619e95a537b92c1df869b3988ee8ebb3a4d84d139b6965339520fca0874eb09fce7929178b2402fdfcd25b43b058cf9ec439af449d287af6dead62789f76cd1c014a3c56e96e104bde4d7246e421b291c9dbdbdf641dd1e95519caf905b6b6a47116fdef98')
  print(hexB(ciphertext))
  long_text = """Here's some long text to encrypt. We would like to know
  what the ciphertext looks like when it's encrypting more than 32 bytes. What will
  be really interesting is the newlines and spacing. I'd like to include using zlib."""
  assert decrypt(ciphertext, key) == long_text
  print(decrypt(ciphertext, key))
