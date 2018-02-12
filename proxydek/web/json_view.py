# json_view.py
# Trevor Pottinger
# Sun Jan 10 16:02:11 PST 2016

import json

from base import BaseCardHandler

class JSONCardHandler(BaseCardHandler):

  def get(self, set_id, card_id, card_sub_id):
    if card_sub_id is None:
      # Does this make sense as the default sub ID?
      card_sub_id = '0'
    try:
      card_index = int(card_id)
    except Exception:
      self.fourOhFour("\"%d\" not an int, in set %s" % (index, set_id))
      return
    if set_id not in self.db['cards']:
      self.fourOhFour("\"%s\" not a valid set code name" % (set_id))
      return
    if card_index not in self.db['cards'][set_id]:
      self.fourOhFour(
        "\"%d\" not a valid index in set %s" % (card_index, set_id)
      )
      return
    if card_sub_id not in self.db['cards'][set_id][card_index]:
      self.fourOhFour(
        "\"%s\" not a valid sub index in set %s-%d" % (card_sub_id, set_id, card_index)
      )
      return
    print("200 GET %s %s" % (self.request.path, "JSONCardHandler"))
    output = json.dumps(self.fetchCard(set_id, card_index, card_sub_id), indent=4)
    output = "<pre>\n%s\n</pre>" % output
    self.write(output)
