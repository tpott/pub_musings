# masher.py
# Trevor Pottinger
# Fri Sep  9 15:50:20 PDT 2022

import argparse
import collections
import csv
import math
from typing import Dict, List


Cols = List[str]
Rows = List[Cols]
Histogram = Dict[any, int]


def get_parser() -> argparse.ArgumentParser:
	parser = argparse.ArgumentParser(description='mashes up data from two different csvs')
	parser.add_argument('file1', help='the first csv file')
	parser.add_argument('file2', help='the second csv file')
	return parser


def read_csv(filename: str) -> Rows:
	rows = []
	# newline='' is per https://docs.python.org/3/library/csv.html#id3
	with open(filename, newline='') as csvfile:
		spamreader = csv.reader(csvfile)
		for row in spamreader:
			rows.append(row)
			# debugging
			# print(row)
			# print(', '.join(row))
			# print(type(row[0].encode('utf-8')))
	return rows


def distance(hist1: Histogram, hist2: Histogram) -> float:
	"returns math.nan if unimplemented, math.inf is the max, 0.0 is the min"
	dist = 0.0
	for k in hist1:
		if k in hist2:
			dist += (hist1[k] - hist2[k]) ** 2
		else:
			dist += hist1[k] ** 2
	for k in hist2:
		if k in hist1:
			continue
		dist += hist2[k] ** 2
	return math.sqrt(dist)


def cosine(hist1: Histogram, hist2: Histogram) -> float:
	"returns math.nan if unimplemented, math.pi is the max, 0.0 is the min"
	dot = 0.0
	for k in hist1:
		if k in hist2:
			dot += hist1[k] * hist2[k]
	norm1 = 0.0
	for k in hist1:
		norm1 += hist1[k] ** 2
	norm2 = 0.0
	for k in hist2:
		norm2 += hist2[k] ** 2
	return math.acos(dot / (math.sqrt(norm1) * math.sqrt(norm2)))


def compare(rows1: Rows, rows2: Rows) -> Dict[str, str]:
	n1 = len(rows1)
	n2 = len(rows2)
	if n1 == 0 and n2 == 0:
		return {'nrows': 'both tables empty'}
	elif n1 == 0:
		return {'nrows': 'first table empty, second table non-empty'}
	elif n2 == 0:
		return {'nrows': 'second table empty, first table non-empty'}
	ret = {'nrows1': n1, 'nrows2': n2}

	m1 = len(rows1[0])
	m2 = len(rows2[0])
	if m1 == 0 and m2 == 0:
		return {'mcols': 'both tables have no columns'}
	elif m1 == 0:
		return {'mcols': 'first table has no columns, second table has some'}
	elif m2 == 0:
		return {'mcols': 'second table has no columns, first table has some'}
	ret.update({'mcols1': m1, 'mcols2': m2})

	hists1 = []
	for i in range(m1):
		hists1.append(collections.defaultdict(int))
	for row in rows1:
		for i in range(m1):
			for c in row[i].encode('utf-8'):
				hists1[i][c] += 1

	# debugging
	# for hist in hists1:
		# print(hist)

	hists2 = []
	for i in range(m2):
		hists2.append(collections.defaultdict(int))
	for row in rows2:
		for i in range(m2):
			for c in row[i].encode('utf-8'):
				hists2[i][c] += 1

	# debugging
	# for hist in hists2:
		# print(hist)

	for i in range(m1):
		for j in range(m2):
			dist = distance(hists1[i], hists2[j])
			angle = cosine(hists1[i], hists2[j])
			print(f'table1 col {i}, table2 col {j}, dist {dist}, angle {angle}')

	return ret


def main() -> None:
	parser = get_parser()
	args = parser.parse_args()
	file1 = read_csv(args.file1)
	file2 = read_csv(args.file2)
	print(compare(file1, file2))


if __name__ == '__main__':
	main()
