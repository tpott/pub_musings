# chess

This directory depends on [python-chess](https://github.com/niklasf/python-chess).
Both classes only work with python3 (b/c python-chess).

pgnview.py uses curses to watch pgn replays

pgn2json.py spits out a bunch of data in json

# install

```
pip install -r requirements.txt
```

# test it

```
echo 'e4 e6 2. d4 d5 3. Nd2 Nf6 4. e5 Nfd7 5. f4 c5 6. c3 Nc6 7. Ndf3 cxd4 8. cxd4 f6 9. Bd3 Bb4+ 10. Bd2 Qb6 11. Ne2 fxe5 12. fxe5 O-O 13. a3 Be7 14. Qc2 Rxf3 15. gxf3 Nxd4 16. Nxd4 Qxd4 17. O-O-O Nxe5 18. Bxh7+ Kh8 19. Kb1 Qh4 20. Bc3 Bf6 21. f4 Nc4 22. Bxf6 Qxf6 23. Bd3 b5 24. Qe2 Bd7 25. Rhg1 Be8 26. Rde1 Bf7 27. Rg3 Rc8 28. Reg1 Nd6 29. Rxg7 Nf5 30. R7g5 Rc7 31. Bxf5 exf5 32. Rh5+' > example.pgn.txt
python3 pgnview.py example.pgn.txt
```
