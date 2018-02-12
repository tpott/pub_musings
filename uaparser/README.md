# UserAgent Parsing, in a less bad way

User Agents are very structured, so I wanted to try
parsing them.

Dependency: [ply](http://www.dabeaz.com/ply/)

You can run this by piping user agent strings, each on
a newline via stdin. Ex:

	cat examples | python ua_parser.py

Errors (+ syntax errors) will be printed to stderr. You can
filter our the syntax errors by grepping for ERROR. I 
recommend doing this, since one error means one or more
syntax errors.

Example 1, from [maul](https://raw.github.com/bholley/maul/master/data-raw/uas_example_20101014-01.csv)
