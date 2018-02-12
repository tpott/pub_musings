#!/bin/python3
# allsets2csv.py
# Mon Jan  2 21:09:51 PST 2017

import csv
import json
import os

inname = 'download/AllSets.json'
outname = 'download/allsets.tsv'
outname2 = 'download/allcards.tsv'

def readallsets(filename):
    sets = {}
    with open(filename, encoding='utf8') as setfile:
        sets = json.loads(setfile.read())
    if type(sets) != dict:
        raise Exception('Input file, %s, did not parse as json dict' % inname)
    if len(sets) == 0:
        raise Exception('Input file, %s, did not find any sets' % inname)
    return sets

def writesetdata():
    sets = readallsets(inname)

    # Find all the headers
    keys = {}
    for set_code in sets:
        if 'cards' in sets[set_code]:
            # Add data
            sets[set_code]['num_cards'] = len(sets[set_code]['cards'])
        for fieldname in sets[set_code]:
            if fieldname in keys:
                keys[fieldname] += 1
            else:
                keys[fieldname] = 1

    # Write the CSV
    # newline param because... python -__-
    # https://docs.python.org/3/library/csv.html#id3
    with open(outname, 'w', encoding='utf8', newline='') as csvfile:
        if 'cards' in keys:
            del keys['cards']
        writer = csv.DictWriter(csvfile, delimiter='\t', fieldnames=keys.keys(),
                restval='', extrasaction='ignore', lineterminator='\n')
        writer.writeheader()
        for set_code in sets:
            del sets[set_code]['cards']
            # Note that sets[set_code]['code'] should be equal to set_code
            writer.writerow(sets[set_code])

def writecarddata():
    sets = readallsets(inname)

    # Find all the headers and cards
    keys = {}
    cards = []
    for set_code in sets:
        if 'cards' not in sets[set_code]:
            print('Set %s doesnt have any "cards"' % set_code)
            continue
        if len(sets[set_code]['cards']) == 0:
            print('Set %s has zero "cards"' % set_code)
            continue
        for card in sets[set_code]['cards']:
            # Add data
            card['releaseDate'] = sets[set_code]['releaseDate']
            card['setCode'] = sets[set_code]['code']
            for fieldname in card:
                if type(card[fieldname]) == str:
                    # Escape newlines within an output cell
                    card[fieldname] = card[fieldname].replace('\n', '\\n')
                    card[fieldname] = card[fieldname].replace('\t', '\\t')
                if fieldname in keys:
                    keys[fieldname] += 1
                else:
                    keys[fieldname] = 1
            cards.append(card)

    # Write the CSV
    # newline param because... python -__-
    # https://docs.python.org/3/library/csv.html#id3
    with open(outname2, 'w', encoding='utf8', newline='') as csvfile:
        writer = csv.DictWriter(csvfile, delimiter='\t', fieldnames=keys.keys(),
                restval='', extrasaction='ignore', lineterminator='\n')
        writer.writeheader()
        for card in cards:
            writer.writerow(card)

def main():
    if os.path.isfile(outname):
        print('%s already exists, skipping writing AllSets csv' % outname)
    else:
        print('Writing AllSets csv to %s' % outname)
        writesetdata()
    if os.path.isfile(outname2):
        print('%s already exists, skipping writing AllSets csv' % outname2)
    else:
        print('Writing AllSets csv to %s' % outname2)
        writecarddata()

if __name__ == '__main__':
    main()


