# parser.py
# Trevor Pottinger
# Wed Jan 15 23:03:11 PST 2014

import sys
from tokenizer import *

# Parsing rules

def p_user_agent(t):
    '''
    useragent : featurelist
    '''
    t[0] = t[1]

# featurelist :- list of lists
def p_featurelist(t):
    '''
    featurelist : feature_or_recurse SPACE featurelist
                | feature_or_recurse SEMICOLON featurelist
                | feature_or_recurse
    '''
    if len(t) == 4:
        t[0] = t[1] + t[2]
    elif len(t) == 2:
        t[0] = t[1]

# feature_or_recurse :- list of lists
def p_feature_or_recurse(t):
    '''
    feature_or_recurse : LPAREN featurelist RPAREN
                       | feature
    '''
    if len(t) == 4:
        t[0] = t[2]
    elif len(t) == 2:
        t[0] = t[1]

# feature_with_version :- list (first elem is str, then ints)
#  version is optional
def p_feature_with_version(t):
    pass

# feature :- str
def p_feature(t):
    pass

def p_name_with_spaces(t):
    pass

# name_with_periods :- str
def p_name_with_periods(t):
    pass

def p_meta_name(t):
    pass

# version :- list of ints
def p_version(t):
    pass

def p_error(t):
    sys.stderr.write("Syntax error at '%s'\n" % t.value)

import ply.yacc as yacc
yacc.yacc()

# exports
parse = yacc.parse
