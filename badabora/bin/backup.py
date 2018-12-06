# backup.py
# Trevor Pottinger
# Sat Oct 27 21:52:47 PDT 2018

import sys

# allows importing of other modules in bodabora, but only when run from inside
# bodabora/bin/. Maybe import os.path.abspath(sys.path[0] + '/..')?
# sys.path.insert(0, os.path.abspath('..'))
from lib.operators import (BashOperator, PythonOperator, SelectOperator)


# To support `mypy --strict {file}` type checking
if sys.version_info >= (3, 4):
    from typing import Callable


def split_large_files(min_size, max_size):
    # type: (int, int) -> Callable[[], None]
    def _splitter():
        # type: () -> None
        pass
    return _splitter


def collect_small_files(min_size, max_size):
    # type: (int, int) -> Callable[[], None]
    def _collector():
        # type: () -> None
        pass
    return _collector


def main():
    # type: () -> None
    max_size = 2 ** 29  # ~512MB
    min_size = 2 ** 27  # ~128MB
    hasher = 'sha256'  # optionally make this a list separated by commas
    output_dir = 'test_out'

    # TODO: assert output_dir exists

    # Writes to <TABLE:files>, <TABLE:file2fragments>
    fs_walk = PythonOperator(
        deps=[],
    )

    # TODO: de-duplicate existing files by file hash
    # TODO: diff existing files against existing backup

    select_large = SelectOperator(
        deps=[fs_walk],
        where='size >= %d' % max_size,
    )

    # Writes to <TABLE:file2fragments>, <TMP_TABLE:plain_fragments>
    chunkify_large = PythonOperator(
        deps=[select_large],
        func=split_large_files(min_size, max_size),
    )

    select_small = SelectOperator(
        deps=[fs_walk, chunkify_large],
        where='size < %d' % min_size,
    )

    # Writes to <TABLE:file2fragments>, <TMP_TABLE:plain_fragments>
    collect_small = PythonOperator(
        deps=[select_small],
        func=collect_small_files(min_size, max_size),
    )

    select_medium = SelectOperator(
        deps=[fs_walk, chunkify_large, collect_small],
        where='%d <= size and size < %d' % (min_size, max_size),
    )

    encrypt_final = BashOperator(
        deps=[select_medium],
        script='./<BIN:encrypt> %s' % output_dir,
    )

    encrypt_final.run()


if __name__ == '__main__':
    main()
