-- main.hs
-- Compute the hash functions and bit filter
-- Trevor Pottinger
-- Sat Jun 14 14:21:20 PDT 2014

module Main where

import Text.Printf

main :: IO ()
main = do
  generateConfig 3 7 4

generateConfig :: Integer -> Integer -> Integer -> IO ()
generateConfig nFuncs filterMagnitude maxNumMagnitude = 
  do
    -- printf "Creating a Bloom filter with %d hash functions\n" nFuncs
    printf "  with a filter of size %d" $ (2 ^ filterMagnitude :: Int)
    -- printf " and max number of %d\n" $ (2 ^ (2 ^ maxNumMagnitude)) - 1
