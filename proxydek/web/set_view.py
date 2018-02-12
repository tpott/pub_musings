# set_view.py
# Trevor Pottinger
# Sun Jan 10 16:04:58 PST 2016

import json

from base import BaseCardHandler

class SetHandler(BaseCardHandler):

  def get(self, set_id):
    if set_id not in self.db['cards']:
      self.fourOhFour("\"%s\" not a valid set code name" % (set_id))
      return
    print("200 GET %s %s" % (self.request.path, "SetHandler"))
    set_summary = self.db['sets'][set_id]
    # SketchController.uri..
    set_summary['first'] = '/sketch/%s-001' % set_summary['set_id']
    set_summary['cards'] = []
    for i in range(1, set_summary['numCards']):
      if set_id in self.db['cards'] and i in self.db['cards'][set_id]:
        possible_sub_ids = ['0', 'a', 'b', 'c']
        for j in range(len(possible_sub_ids)):
          sub_id = possible_sub_ids[j]
          if sub_id not in self.db['cards'][set_id][i]:
            # no such sub_id, so skip
            continue
          set_summary['cards'].append({
            'card_id': "%s-%03d%s" % (set_id, i, sub_id if sub_id != '0' else ''),
            'name': self.db['cards'][set_id][i][sub_id]['name'],
          })
          # we found what we we're looking for
          break
      else:
        pass
    output = json.dumps(set_summary, indent=4)
    # Makes newlines appear in HTML
    output = "<pre>\n%s\n</pre>" % output
    self.write(output)
