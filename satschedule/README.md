# satschedule

Lets try using Z3 SMT solver to find a schedule for projects that works

## C++ Install

https://github.com/Z3Prover/z3 did not work for me on MacOS 14. I tried upgrading to
MacOS 15 (Sequoia) and it's still not working. I'm missing some dependencies...

```
CXX=clang++ CC=clang python3 scripts/mk_make.py
cd build
make
```

fails with
```
error: invalid value 'c++20' in '-std=c++20'
```

## Python Install

This was pretty quick and easy. I added `z3-solver` to `requirements.txt` and then
ran the following commands:
```
python3 -m venv venv # create a new venv in the directory called "venv"
source venv/bin/activate
python -m pip install -r requirements.txt
```

Eventually I can exit the venv by running
```
deactivate
```

Then I tried running https://github.com/Z3Prover/z3/blob/master/examples/python/example.py

Note that I added `black` to `requirements.txt`. This makes running `black *.py`
easier.
