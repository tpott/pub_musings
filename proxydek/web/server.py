# server.py
# Trevor Pottinger
# Mon Sep  7 14:51:07 PDT 2015

from __future__ import print_function

import json
import random
import re

import tornado.ioloop
import tornado.web

import base
import deck
import json_view
import multi
import namesearch
import set_view
import sketch_view

port = 8000
filenames = ['AllCards.json','AllSets.json']

def nameNormalize(raw_name):
  return raw_name

def parseAllSets(filename):
  """Parses the raw AllSets-x.json file downloaded from http://mtgjson.com/"""
  raw_db = json.loads(open(filename).read())
  my_db = {'cards': {}, 'sets': {}, 'names': {}}
  error_db = {'no number': [], 'weird number': []}
  for set_code_name in raw_db:
    if set_code_name not in my_db['cards']:
      my_db['cards'][set_code_name] = {}
    if set_code_name not in my_db['sets']:
      my_db['sets'][set_code_name] = {'set_id': set_code_name}
    if 'cards' not in raw_db[set_code_name]:
      print("No cards for set \"%s\"" % (set_code_name))
    else:
      for card in raw_db[set_code_name]['cards']:
        if 'number' not in card:
          error_db['no number'].append(card)
          continue
        maybe_has_sub_id = False
        try:
          index = int(card['number'])
        except:
          maybe_has_sub_id = True
        if maybe_has_sub_id:
          try:
            sub_id = card['number'][-1]
            index = int(card['number'][:-1])
          except Exception:
            error_db['weird number'].append(card)
            continue
        else:
          # Does this make sense as a default?
          sub_id = '0'
        my_db['cards'][set_code_name][index] = { sub_id: card }
        if card['name'] in my_db['names']:
          my_db['names'][card['name']].append( (set_code_name, index, sub_id) )
        else:
          my_db['names'][card['name']] = [ (set_code_name, index, sub_id) ]
      # end for
      my_db['sets'][set_code_name]['numCards'] = len(raw_db[set_code_name]['cards'])

    if 'releaseDate' not in raw_db[set_code_name]:
      print("No release date for %s" % (set_code_name))
    else:
      my_db['sets'][set_code_name]['release'] = raw_db[set_code_name]['releaseDate']
    if 'name' not in raw_db[set_code_name]:
      print("No name for %s" % (set_code_name))
    else:
      my_db['sets'][set_code_name]['name'] = raw_db[set_code_name]['name']
  checked_errors = ['no number', 'weird number']
  for err in checked_errors:
    if len(error_db[err]) == 0:
      continue
    print(
      "Failed on %d \"%s\" errors. Examples:\n%s\n%s" % (
        len(error_db[err]),
        err,
        error_db[err][0],
        random.choice(error_db[err])
      ),
    )
  # can't get 'cards' printed correctly
  checked_dbs = ['sets']
  for db in checked_dbs:
    if len(my_db[db]) == 0:
      continue
    print(
      "Found %d elements in %s. Examples:\n%s\n%s" % (
        len(my_db[db]),
        db,
        my_db[db][my_db[db].keys()[0]],
        my_db[db][random.choice(my_db[db].keys())]
      ),
    )

  return (my_db, raw_db)

if __name__ == "__main__":
  (my_db, raw_db) = parseAllSets(filenames[1])
  regex_obj = re.compile('^/([A-Z]{3})-([0-9]{3})$')
  application = tornado.web.Application([
    (r"/", base.BaseCardHandler, dict(db=my_db)),
    (r"/search", namesearch.SearchHandler, dict(db=my_db)),
    (r"/create", deck.CreateHandler, dict(db=my_db)),
    (r"/set/([A-Z0-9]{3})", set_view.SetHandler, dict(db=my_db)),
    (r"/json/([A-Z0-9]{3})-([0-9]{3})(.)?", json_view.JSONCardHandler, dict(db=my_db)),
    (r"/sketch/([A-Z0-9]{3})-([0-9]{3})(.)?", sketch_view.SketchCardHandler, dict(db=my_db)),
    (r"/(sketches)/([A-Z0-9]{3})-([0-9]{3})(.)?", multi.MultiSketchCardHandler, dict(db=my_db)),
    (r"/(originals)/([A-Z0-9]{3})-([0-9]{3})(.)?", multi.MultiSketchCardHandler, dict(db=my_db)),
    (r"/(compares)/([A-Z0-9]{3})-([0-9]{3})(.)?", multi.MultiSketchCardHandler, dict(db=my_db)),
  ])
  application.listen(port)
  print("Starting server on port %d" % (port))
  tornado.ioloop.IOLoop.current().start()
