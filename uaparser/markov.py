# markov.py
# Trevor Pottinger
# Fri Jan 10 23:13:26 PST 2014

def ngram(s, n):
    pass

def nchar(s, n):
    char_counts = {}
    for i in range(len(s) - n + 1):
        if s[i:i+n] in char_counts:
            char_counts[s[i:i+n]] += 1
        else:
            char_counts[s[i:i+n]] = 1
    return char_counts

markov1char = lambda s: nchar(s, 1)
markov2char = lambda s: nchar(s, 2)
markov3char = lambda s: nchar(s, 3)
markovNchar = lambda s, n: nchar(s, n)
