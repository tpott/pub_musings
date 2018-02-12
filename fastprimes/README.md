fastprimes
==========

Constant time checks if a small number is a prime or composite

Example Output
==============

    $ python fastprimes.py 20000 5 17 10 17 2
    Starting the generation of primes with seed(s) [2, 3]
    Finished, generated a total of 20000 primes
    ### Run 1 ###
    Building 5 hash functions...
    Building an array of length 2^17 (131072)...
    Building 10 hash functions...
    Building an array of length 2^17 (131072)...
    Adding primes
    Adding false positives..
    Evaluating results
    False positives 0, in both 513, min false pos 224738
    Counts= (19999, 10400)
    ### Run 2 ###
    Building 5 hash functions...
    Building an array of length 2^17 (131072)...
    Building 10 hash functions...
    Building an array of length 2^17 (131072)...
    Adding primes
    Adding false positives..
    Evaluating results
    False positives 0, in both 419, min false pos 224738
    Counts= (19999, 10019)
