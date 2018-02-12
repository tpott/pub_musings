# multi.py
# Trevor Pottinger
# Sun Jan 10 16:03:33 PST 2016

from sketch_view import SketchCardHandler

# Uh-oh..
class MultiSketchCardHandler(SketchCardHandler):
  # This should ideally live in the Application Router
  ACCEPTED_TYPES = ['compares', 'originals', 'sketches']

  def get(self, sketch_type, set_id, card_id, card_sub_id):
    if card_sub_id is None:
      # Does this make sense as the default sub ID?
      card_sub_id = '0'
    if sketch_type not in MultiSketchCardHandler.ACCEPTED_TYPES:
      self.fourOhFour("\"%s\" not in ACCEPTED_TYPES" % (sketch_type))
      return
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
    print("200 GET %s %s %s" % (self.request.path, "MultiSketchCardHandler", sketch_type))
    # This is terrible HTML
    self.write(self.renderPageCSS())
    self.write('<table>')
    if sketch_type in ['originals', 'sketches']:
      (num_rows, num_cols) = (3, 3)
    else:
      assert sketch_type == 'compares'
      (num_rows, num_cols) = (2, 2)
    for i in range(num_rows):
      self.write('<tr>')
      for j in range(num_cols):
        self.write('<td>')
        current_index = card_index + i * num_cols + j
        card = self.fetchCard(set_id, current_index, card_sub_id)
        if card is None:
          continue
        if sketch_type == 'compares':
          self.write(self.renderCard(card))
          self.write(self.renderWizardsCard(card))
        elif sketch_type == 'originals':
          self.write(self.renderWizardsCard(card))
        elif sketch_type == 'sketches':
          self.write(self.renderCard(card))
        else:
          self.fourOhFour("WTF: %s" % (sketch_type))
          return
        self.write('</td>')
      self.write('</tr>')
    self.write('</table>')
    # return get

if __name__ == "__main__":
  (my_db, raw_db) = parseAllSets(filenames[1])
  regex_obj = re.compile('^/([A-Z]{3})-([0-9]{3})$')
  application = tornado.web.Application([
    (r"/", BaseCardHandler, dict(db=my_db)),
    (r"/search", SearchHandler, dict(db=my_db)),
    (r"/set/([A-Z0-9]{3})", SetHandler, dict(db=my_db)),
    (r"/json/([A-Z0-9]{3})-([0-9]{3})(.)?", JSONCardHandler, dict(db=my_db)),
    (r"/sketch/([A-Z0-9]{3})-([0-9]{3})(.)?", SketchCardHandler, dict(db=my_db)),
    (r"/(sketches)/([A-Z0-9]{3})-([0-9]{3})(.)?", MultiSketchCardHandler, dict(db=my_db)),
    (r"/(originals)/([A-Z0-9]{3})-([0-9]{3})(.)?", MultiSketchCardHandler, dict(db=my_db)),
    (r"/(compares)/([A-Z0-9]{3})-([0-9]{3})(.)?", MultiSketchCardHandler, dict(db=my_db)),
  ])
  application.listen(port)
  print("Starting server on port %d" % (port))
  tornado.ioloop.IOLoop.current().start()
