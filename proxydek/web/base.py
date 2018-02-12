# base.py
# Trevor Pottinger
# Sun Jan 10 16:01:55 PST 2016

from __future__ import print_function

import json

import tornado.ioloop
import tornado.web

class BaseCardHandler(tornado.web.RequestHandler):

  def fourOhFour(self, msg):
    print("404 GET %s %s %s" % (self.request.path, "BaseCardHandler", msg))
    self.clear()
    self.set_status(404)
    self.finish("soo sad: " + msg)

  def initialize(self, db):
    """Expects the db to be processed"""
    self.db = db
    self.url_pattern = 'http://gatherer.wizards.com/Handlers/Image.ashx?multiverseid=%(multiverseid)s&type=card'
    self.sketch_pattern = '%s-%03d'

  def fetchCard(self, set_id, card_id_int, card_sub_id):
    # '0' is an annoying default
    card_id_str = "%03d%s" % (card_id_int, card_sub_id if card_sub_id != '0' else '')
    if card_id_int not in self.db['cards'][set_id] or \
        card_sub_id not in self.db['cards'][set_id][card_id_int]:
      return None
    card = dict(self.db['cards'][set_id][card_id_int][card_sub_id])
    # This needs to be done before setting default power and toughness
    card['pt_divider'] = '/' if 'power' in card and 'toughness' in card else ''
    card['power'] = card['power'] if 'power' in card else ''
    card['toughness'] = card['toughness'] if 'toughness' in card else ''
    # Order doesn't matter after this
    card['card_id'] = card_id_str
    card['flavor'] = card['flavor'] if 'flavor' in card else ''
    card['manaCost'] = card['manaCost'] if 'manaCost' in card else ''
    card['originalType'] = card['type'] if 'type' in card else ''
    card['set_id'] = set_id
    card['text'] = card['text'] if 'text' in card else ''
    # Future self, this is to make things easier to read
    card['text'] = card['text'].replace("\n", "\n</div><div>")
    card['url'] = self.url_pattern % card if 'multiverseid' in card else 'https://upload.wikimedia.org/wikipedia/en/a/aa/Magic_the_gathering-card_back.jpg'
    card['uniq_card_id'] = "%s-%03d" % (set_id, card_id_int)
    #print(self.sketch_pattern % (set_id, max(0, card_id_int - 1)))
    card['prev_sketch_url'] = self.sketch_pattern % (set_id, max(0, card_id_int - 1))
    # next url is unsafe since we don't know the last card id
    card['next_sketch_url'] = self.sketch_pattern % (set_id, (card_id_int + 1))
    return card

  def get(self):
    output = json.dumps(self.db['sets'].values(), indent=4)
    # Makes newlines appear in HTML
    output = "<pre>\n%s\n</pre>" % output
    self.write(output)
