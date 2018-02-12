# tokenizer.py
# Trevor Pottinger
# Sun Jan  5 14:39:54 PST 2014

tokens = (
    'COMMA', 'FORWARDSLASH', 'LPAREN', 'NAME', 
    'NUMBER', 'PERIOD', 'PLUS', 'RPAREN', 
    'SEMICOLON', 'SPACE',
    )

# Tokens

t_COMMA         = r','
t_FORWARDSLASH  = r'/'
t_LPAREN        = r'\('
t_NAME          = r'[a-zA-Z_][a-zA-Z0-9_-]*'
t_PERIOD        = r'.'
t_PLUS          = r'\+'
t_RPAREN        = r'\)'
t_SEMICOLON     = r';'
t_SPACE         = r'\ '

def t_NUMBER(t):
    r'\d+'
    try:
        t.value = int(t.value)
    except ValueError:
        print("Integer value too large %d", t.value)
        t.value = 0
    return t

# Ignored characters
t_ignore = "\n\t"

def t_newline(t):
    r'\n+'
    t.lexer.lineno += t.value.count("\n")

def t_error(t):
    print("Illegal character '%s'" % t.value[0])
    t.lexer.skip(1)
    
# Build the lexer
import ply.lex as lex
lex.lex()
