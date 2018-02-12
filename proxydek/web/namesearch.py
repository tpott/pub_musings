# namesearch.py
# Trevor Pottinger
# Sun Jan 10 16:04:26 PST 2016

import json

from sketch_view import SketchCardHandler

class SearchHandler(SketchCardHandler):

  def get(self):
    # User-controlled
    search_name = self.get_argument('name', None)
    if search_name is None:
      self.write('No search name provided')
      return
    # TODO fuzzy search?
    if search_name not in self.db['names']:
      self.write("%s not found in names" % search_name)
      return
    self.write(self.renderPageCSS())
    id_matches = self.db['names'][search_name]
    card_matches = []
    for i in range(len(id_matches)):
      card_matches.append(self.fetchCard(
        id_matches[i][0], # set id
        id_matches[i][1], # card id
        id_matches[i][2], # card sub id
      ))

    sketch_type = self.get_argument('render', None)
    if sketch_type is None or sketch_type == 'json':
      # Makes newlines appear in HTML
      output = "<pre>\n%s\n</pre>" % json.dumps(card_matches, indent=4)
      self.write(output)
    elif sketch_type == 'originals':
      for card in card_matches:
        self.write(self.renderWizardsCard(card))
        self.write(" <br />\n")
    elif sketch_type == 'sketches':
      for card in card_matches:
        self.write(self.renderCard(card))
        self.write(" <br />\n")
    else:
      output = "<pre>\n%s\n</pre>" % json.dumps(card_matches, indent=4)
      self.write(output)
    # end get
