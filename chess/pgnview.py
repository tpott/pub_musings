# pgnview.py
# Trevor Pottinger
# Mon Feb 18 14:34:30 PST 2019

import argparse
import chess
import chess.pgn
import curses
import sys
import time

if sys.version_info >= (3, 5):
    from typing import (Any, Callable, Optional)


def main(pgn_file, freq, log_file):
    # type: (str, float, Optional[str]) -> Callable[[Any], None]
    f = None
    if log_file is not None:
        f = open(log_file, 'wb')

    # TODO a pgn_file can contain multiple games
    pgn_f = open(pgn_file)
    game = chess.pgn.read_game(pgn_f)
    moves = list(game.mainline_moves())
    if f is not None:
        f.write(b"Read %d moves\n" % len(moves))

    def render(stdscr, board):
        # type: (Any, chess.Board) -> None
        if f is not None:
            f.write(b"Rendering at %f\n" % time.time())
        x = 0
        y = 0
        for c in str(board.unicode()):
            stdscr.addstr(y, x, c)
            x += 1
            if c != '\n':
                continue
            x = 0
            y += 1
        return

    def _main(stdscr):
        # type: (Any) -> None
        stdscr.clear()

        b = chess.Board()
        render(stdscr, b)
        stdscr.refresh()

        prev = time.time()
        i = 0
        while i < len(moves):
            time.sleep(0.001)  # sleep for 1ms
            if time.time() - prev < freq:
                continue

            b.push(moves[i])
            i += 1
            prev = time.time()

            # k = stdscr.getch()
            render(stdscr, b)
            stdscr.refresh()

        return
    return _main


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Play pgn files via the terminal')
    parser.add_argument('-d', type=float, default=0.5, help='The frequency to play moves at')
    parser.add_argument('filename', help='The path of the .pgn file')
    parser.add_argument('--logfile', help='The path to a log file that will be overwritten')
    args = parser.parse_args()

    curses.wrapper(main(args.filename, args.d, args.logfile))
