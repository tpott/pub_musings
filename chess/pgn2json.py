# pgnview.py
# Trevor Pottinger
# Mon Feb 18 14:34:30 PST 2019

import argparse
import chess
import chess.pgn
import json
import sys

if sys.version_info >= (3, 5):
    from typing import (Any, Dict, List)


def list2counts(l):
    # type: (List[str]) -> Dict[str, int]
    counts = {}  # type: Dict[str, int]
    for item in l:
        if item in counts:
            counts[item] += 1
        else:
            counts[item] = 1
    return counts


def game2dict(game, include_piece_counts):
    # type: (chess.Game, bool) -> Dict[str, Any]
    ret = {}  # type: Dict[str, Any]

    # Fields to check for: Site, Date, EventDate, Round, Result,
    # White, Black, ECO, WhiteElo, BlackElo, PlyCount
    ret['event_name'] = game.headers['Event']
    ret['white'] = game.headers['White']
    ret['black'] = game.headers['Black']

    ret['result'] = 'unknown:%s' % game.headers['Result']
    if game.headers['Result'] == '1-0':
        ret['result'] = 'white_won'
    elif game.headers['Result'] == '0-1':
        ret['result'] = 'black_won'
    elif game.headers['Result'] == '1/2-1/2':
        ret['result'] = 'tie'

    moves = list(game.mainline_moves())
    ret['moves'] = list(map(str, moves))
    ret['num_moves'] = len(ret['moves'])
    if 'PlyCount' in game.headers:
        ret['ply_count'] = int(game.headers['PlyCount'])
        ret['correct_ply_count'] = ret['ply_count'] == len(moves)

    b = game.board()
    ret['num_legal_moves'] = []
    piece_counts = []
    for move in moves:
        ret['num_legal_moves'].append(len(list(b.legal_moves)))
        pieces = list(map(str, b.piece_map().values()))
        piece_counts.append(list2counts(pieces))
        b.push(move)

    if include_piece_counts:
        # Final piece counts not recorded? Oh well.
        ret['piece_counts'] = piece_counts

    return ret


def main(pgn_file, include_piece_counts):
    # type: (str, bool) -> None

    pgn_f = open(pgn_file)
    game = chess.pgn.read_game(pgn_f)
    while game is not None:
        ret = game2dict(game, include_piece_counts)
        print(json.dumps(ret))
        # Next game
        game = chess.pgn.read_game(pgn_f)
    # end while

    return


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Converts PGN files to JSON')
    parser.add_argument('filename', help='The path of the .pgn file')
    parser.add_argument('--piece_counts', action='store_true',
                        help=('Includes a dict of how many of each piece after '
                              'each move.'))
    args = parser.parse_args()

    main(args.filename, args.piece_counts)
