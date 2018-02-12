# ua_parser.py
# Trevor Pottinger
# Sun Jan  5 20:37:18 PST 2014

import sys
#from 2parser import parse
from markov import *
from short import parse as shortparse
from short import lex as lex

def Tokens(s):
    lex.input(s)
    return [ token.value for token in iter(lex.token, None) ]

def Tree(s):
    ret = {
        'tree' : None,
        'length' : -1,
        'error' : None,
    }
    try:
        ast = parse(s)
        if ast == None:
            raise Exception("Unable to parse: ")
        #ast.reverse()
        ret['tree'] = ast
    except Exception as e:
        ret['error'] = e
        sys.stderr.write("ERROR ")
        sys.stderr.write(str(e))
        sys.stderr.write(str(s))
        sys.stderr.write("\n")
    return ret

def Markov(s):
    return markov1char(s)

def ShortTree(s):
    ret = {
        'tree' : None,
        'length' : -1,
        'error' : None,
    }
    try:
        ast = shortparse(s)
        if ast == None:
            raise Exception("Unable to parse: ")
        #ast.reverse()
        ret['tree'] = ast
    except Exception as e:
        ret['error'] = e
        sys.stderr.write("ERROR ")
        sys.stderr.write(str(e))
        sys.stderr.write(str(s))
        sys.stderr.write("\n")
    return ret

def main():
    # init
    errors = 0
    noneErrors = 0
    lengths = {}
    ret = None 
    case = "raw" 
    options = "raw tokens trees markov short all".split(" ")

    if len(sys.argv) == 2 and sys.argv[1] in options:
        case = sys.argv[1]
    printAllRets = True
    ret = None

    # repeat
    while 1:
        try:
            # Use raw_input on Python 2
            # or input on Python 3
            s = raw_input()   
        except EOFError:
            break

        if case == "raw":
            ret = s
            printAllRets = True
        elif case == "tokens":
            ret = Tokens(s)
            if len(ret) in lengths:
                lengths[len(ret)] += 1
            else:
                lengths[len(ret)] = 1
        elif case == "trees":
            ret = Tree(s)
            if ret['error']:
                errors += 1
            elif len(ret['tree']) in lengths:
                lengths[len(ret['tree'])] += 1
            else:
                lengths[len(ret['tree'])] = 1
            ret = ret['tree']
        elif case == "markov":
            ret = Markov(s)
            for c in ret:
                if c in lengths:
                    lengths[c] += ret[c]
                else:
                    lengths[c] = ret[c]
        elif case == "short":
            ret = ShortTree(s)
            if ret['error']:
                errors += 1
            elif len(ret['tree']) in lengths:
                lengths[len(ret['tree'])] += 1
            else:
                lengths[len(ret['tree'])] = 1
            ret = ret['tree']
        elif case == "all":
            # TODO don't assume
            # assume printAllRets
            print s, "tokens=", Tokens(s), "trees=", Tree(s)['tree']

            
        if printAllRets and ret != None:
            print ret

        # end while

    if case == "markov":
        def cmpSecondItem(x,y):
            if x[1] > y[1]:
                return -1
            elif x[1] < y[1]:
                return 1
            else:
                return 0
        l = list(lengths.iteritems())
        l.sort(cmp=cmpSecondItem)
        print l, len(l)


    # results
    print
    print "There were", errors, "errors"
    #print "Unable to parse", noneErrors, "user agents"

    if case == "raw":
        print lengths # TODO sort...
    else:
        print lengths

s = "Mozilla/5.0 (IE 11.0; Windows NT 6.3) like Gecko"
if __name__ == '__main__':
    main()
else:
    ret = Tree(s)
    print ret['tree']
