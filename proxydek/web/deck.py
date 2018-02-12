# deck.py
# Trevor Pottinger
# Sun Jan 10 16:16:30 PST 2016

import base64
import json
import math
import os

from sketch_view import SketchCardHandler

class CreateHandler(SketchCardHandler):

  def get(self):
    print("200 GET %s %s" % (self.request.path, "CreateHandler"))
    # User-controlled
    text = self.get_argument('text', '')
    self.write(self.renderPageCSS())
    tokens = filter(lambda x: x != '', text.split("\n"))
    #print(tokens)
    tuples = map(lambda x: x.strip().split(' ', 1), tokens)
    #print(tuples)
    counted_name_tuples = map(lambda x: (int(x[0].strip(), 10), x[1].strip()), tuples)
    print(counted_name_tuples)
    deck_id = base64.urlsafe_b64encode(os.urandom(6))
    print('deck=%s' % deck_id)
    if text != '':
      # re-encode text so it's easier to read
      text = "\n".join(map(lambda tup: "%d %s" % (tup[0], tup[1]), counted_name_tuples))
      self.write("""
      <form action="/create">
        <textarea name="text" rows="20" cols="80">%s</textarea>
        <br />
        <input type="submit" name="deck_action" value="View" />
        <input type="submit" name="deck_action" value="Create" />
      </form>
      """ % text)

    matched_tuples = filter(lambda x: x[1] in self.db['names'], counted_name_tuples)
    #print(matched_tuples)
    id_matches = []
    for tup in matched_tuples:
      # search name by tup[1], take first item
      id_matches.extend([ self.db['names'][tup[1]][0] ] * tup[0])
    # TODO num_cards should be equal to len(id_matches)
    print(id_matches)
    if text == '':
      # No more results to write
      return
    action = self.get_argument('deck_action', 'View')
    card_matches = []
    for i in range(len(id_matches)):
      card_matches.append(self.fetchCard(
        id_matches[i][0], # set id
        id_matches[i][1], # card id
        id_matches[i][2], # card sub id
      ))
    #print(card_matches)
    (num_rows, num_cols) = (int(math.ceil(float(len(card_matches)) / 3)), 3)
    print("Rows x cols, %d %d = %d" % (num_rows, num_cols, len(card_matches)))
    self.write('<table>')
    sketch_type = self.get_argument('render', None)
    for i in range(num_rows):
      self.write('<tr style="page-break-inside: avoid; page-break-before: always;">')
      for j in range(num_cols):
        self.write('<td>')
        current_index = i * num_cols + j
        if current_index >= len(card_matches):
          break
        card = card_matches[current_index]
        if card is None:
          continue
        if sketch_type == 'originals':
          self.write(self.renderWizardsCard(card))
        elif sketch_type == 'sketches':
          self.write(self.renderCard(card))
        elif sketch_type == 'json' or sketch_type is None:
          output = "<pre>\n%s\n</pre>" % json.dumps(card_matches, indent=4)
          self.write(output)
        else:
          self.fourOhFour("WTF: %s" % (sketch_type))
          return
        self.write('</td>')
      self.write('</tr>')
    self.write('</table>')
  # end get
