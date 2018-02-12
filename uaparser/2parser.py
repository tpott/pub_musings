# parser.py
# Trevor Pottinger
# Sun Jan  5 14:39:26 PST 2014

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
    featurelist : feature_or_recurse SEMICOLON SPACE featurelist 
                | feature_or_recurse SEMICOLON featurelist 
                | feature_or_recurse
    '''
    if len(t) == 5:
        t[0] = t[1] + t[4]
    elif len(t) == 4:
        if t[2] == ';':
            t[0] = t[1] + t[3]
        elif t[2] == ' ':
            t[0] = t[1] + t[3]
    elif len(t) == 2:
        t[0] = t[1]

# feature_or_recurse :- list of lists
def p_feature_or_recurse(t):
    '''
    feature_or_recurse : LPAREN featurelist RPAREN
                       | feature_with_version
    '''
    if len(t) == 4:
        t[0] = [t[2]]
    elif len(t) == 2:
        t[0] = [t[1]]

# feature_with_version :- list (first elem is str, then ints)
#  version is optional
def p_feature_with_version(t):
    '''
    feature_with_version : feature FORWARDSLASH version
                         | feature SPACE version
                         | feature version
                         | feature
    '''
    if len(t) == 4:
        if t[2] == '/':
            t[0] = [0,] * (len(t[3]) + 1)
            t[0][0] = t[1]
            for i in range(len(t[3])):
                t[0][i+1] = t[3][i]
        elif t[2] == ' ':
            # TODO this is the same as forward slash
            t[0] = [0,] * (len(t[3]) + 1)
            t[0][0] = t[1]
            for i in range(len(t[3])):
                t[0][i+1] = t[3][i]
    elif len(t) == 3:
        t[0] = [t[1]] + t[2]
    elif len(t) == 2:
        t[0] = [t[1]]

# feature :- str
def p_feature(t):
    '''
    feature : feature COMMA SPACE feature
            | feature COMMA feature
            | name_with_spaces
            | name_with_periods
    '''
    if len(t) == 5:
        t[0] = t[1] + ', ' + t[4]
    elif len(t) == 4:
        t[0] = t[1] + ', ' + t[4]
    elif len(t) == 2:
        # TODO delete comment. was for case 3..
        #t[0] = t[1] + ' ' + t[3]
        t[0] = str(t[1]) # cast just to be sure

def p_name_with_spaces(t):
    '''
    name_with_spaces : name_with_spaces SPACE metaname
                     | metaname
    '''
    if len(t) == 4:
        t[0] = t[1] + ' ' + t[3]
    elif len(t) == 2:
        t[0] = t[1]

# name_with_periods :- str
def p_name_with_periods(t):
    '''
    name_with_periods : name_with_periods PERIOD metaname
                      | name_with_periods PERIOD
                      | PERIOD name_with_periods
                      | metaname
    '''
    if len(t) == 4:
        t[0] = t[1] + '.' + t[3]
    elif len(t) == 3:
        if t[2] == '.':
            t[0] = t[1] + '.'
        elif t[1] == '.':
            t[0] = '.' + t[2]
    elif len(t) == 2:
        t[0] = t[1]

def p_meta_name(t):
    pass

# version :- list of ints
def p_version(t):
   '''
   version : version PERIOD NUMBER
           | NUMBER
   '''
   if len(t) == 4:
      t[1].append(t[3])
      t[0] = t[1]
   elif len(t) == 2:
      t[0] = [t[1]]

def p_error(t):
    sys.stderr.write("Syntax error at '%s'\n" % t.value)

import ply.yacc as yacc
yacc.yacc()

# exports
parse = yacc.parse
