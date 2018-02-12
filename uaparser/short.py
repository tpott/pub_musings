# short.py
# Trevor Pottinger
# Fri Jan 17 21:46:25 PST 2014

import ply.lex as lex
import ply.yacc as yacc
import sys

tokens = (
    'COMMA',
    'FEATURE',
    'LPAREN',
    'RPAREN', 
    'SEMI',
    'SPACE',
)

# Tokens

t_COMMA     = r','
t_FEATURE   = r'[^ ,;()]+'
t_LPAREN    = r'\('
t_RPAREN    = r'\)'
t_SEMI      = r';'
t_SPACE     = r'\ '

# Ignored characters
t_ignore = "\n\t"

def t_newline(t):
    r'\n+'
    t.lexer.lineno += t.value.count("\n")

def t_error(t):
    print("Illegal character '%s'" % t.value[0])
    t.lexer.skip(1)
    
# Build the lexer
lex.lex()

# Parsing rules

def p_user_agent(t):
    '''
    useragent : featurelist
    '''
    t[0] = t[1]

# featurelist :- list of (strings or lists of strings)
def p_featurelist(t):
    '''
    featurelist : FEATURE break featurelist
                | LPAREN featurelist RPAREN break featurelist
                | LPAREN featurelist RPAREN
                | FEATURE break
                | FEATURE
    '''
    if len(t) == 6:
        t[0] = t[2] + t[5]
    elif len(t) == 4:
        if t[1] == '(' and t[3] == ')':
            t[0] = t[2]
        elif type(t[1]) == str:
            t[0] = [t[1]] + t[3]
    elif len(t) == 3:
        t[0] = [t[1]]
    elif len(t) == 2:
        t[0] = [t[1]]

def p_break(t):
    '''
    break : break break
          | COMMA
          | SEMI
          | SPACE
    '''
    # does it even matter?
    t[0] = "BREAK"

def p_error(t):
    sys.stderr.write("Syntax error at '%s'\n" % t.value)

yacc.yacc()

# exports
parse = yacc.parse
