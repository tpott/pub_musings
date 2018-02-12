# sketch_view.py
# Trevor Pottinger
# Sun Jan 10 16:05:30 PST 2016

from base import BaseCardHandler

class SketchCardHandler(BaseCardHandler):

  def renderCard(self, card):
    # TODO copy card into a temp variable, and add values to that
    if card['pt_divider'] != '':
      card['power_float'] = """
  <div id="powerFloat" class="bordered" >
    <span class="innerText" style="float: right; font-size: 9pt; margin-right: 3px;">%(power)s%(pt_divider)s%(toughness)s</span>
  </div>
""" % card
    elif 'loyalty' in card and card['loyalty'] is not None:
      card['power_float'] = """
  <div id="powerFloat" class="bordered" >
    <span class="innerText" style="float: right; font-size: 9pt; margin-right: 3px;">%(loyalty)d</span>
  </div>
""" % card
    else:
      card['power_float'] = ''
    if len(card['text']) + len(card['flavor']) > 340:
      card['text_section'] = """
  <div class="bordered inner" style="top: 4px; height: 87px;">
    <span class="innerText" style="float: left; font-size: 6pt"><div>%(text)s</div></span>
    <span class="innerText" style="float: left; font-style: italic; font-size: 5pt;">%(flavor)s</span>
  </div>
""" % card
    elif len(card['text']) + len(card['flavor']) > 230:
      card['text_section'] = """
  <div class="bordered inner" style="top: 4px; height: 87px;">
    <span class="innerText" style="float: left; font-size: 7pt"><div>%(text)s</div></span>
    <span class="innerText" style="float: left; font-style: italic; font-size: 6pt;">%(flavor)s</span>
  </div>
""" % card
    else:
      card['text_section'] = """
  <div class="bordered inner" style="top: 4px; height: 87px;">
    <span class="innerText" style="float: left; font-size: 8pt"><div>%(text)s</div></span>
    <span class="innerText" style="float: left; font-style: italic; font-size: 7pt;">%(flavor)s</span>
  </div>
""" % card
    # Passes in card via `%`
    return """
<!-- 2.50" ==> 223px, 3.50" ==> 311px -->
<!-- actual ==> 203px, 291px -->
<div class="bordered" style="width: 203px; height: 291px; clear: left; float: left;">
  <!-- name and cost -->
  <div class="bordered inner" style="top: 5px; height: 18px; font-size: 9pt;">
    <span class="innerText" style="float: left; font-size: 9pt;">%(name)s</span>
    <span class="innerText" style="float: right; font-size: 8pt;">%(manaCost)s</span>
  </div>
  <!-- photo, top relative to previous div -->
  <div class="bordered inner" style="top: 2px; height: 135px;">
    <!--<img src="%%(art_url)s" />-->
  </div>
  <!-- type bar -->
  <div class="bordered inner" style="top: 3px; height: 17px; font-size: 8pt;">
    <span class="innerText" style="float: left;">%(originalType)s</span>
    <span class="innerText" style="float: right; margin-right: 3px;">%(set_id)s</span>
  </div>
  <!-- text section -->
  %(text_section)s
  <!-- why does the `top` attribute need to keep increasing? -->
  <!-- copyright, index, power + toughness -->
  <div class="bordered inner" style="top: 5px; height: 12px;">
    <span class="innerText" style="margin: 3px; float: left; font-size: 5pt;">%(set_id)s-%(card_id)s %(artist)s</span>
  </div>
  %(power_float)s
</div>
""" % card

  def renderFooterLinks(self, card):
    # Passes in card via `%`
    return """
<br>
<div style="float: left;">
  <a href="%(prev_sketch_url)s">Previous</a>
</div>
<!-- the destination page may not exist -->
<div style="padding-left: 180px;">
  <a href="%(next_sketch_url)s">Next</a>
</div>
""" % card

  def renderWizardsCard(self, card):
    # TODO width and height should be unified
    return """<img style="width: 203px; height: 291px;" src="%(url)s" />""" % card
    #return """<img style="padding-left: 5px;" src="%(url)s" />""" % card

  def renderPageCSS(self):
    return """
<style>
tr, td, table {
  padding: 0px;
  border-spacing: 0px;
}
.bordered {
  border: 1px solid black;
  color: black;
}
.inner {
  position: relative;
  left: 5px;
  width: 193px;
}
.innerText {
  margin-left: 5px;
}
#powerFloat {
  position: relative;
  width: 24px;
  height: 15px;
  float: right;
  right: 3px;
  bottom: 20px;
  background: white;
}
</style>
"""

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
    print("200 GET %s %s" % (self.request.path, "SketchCardHandler"))
    # This is terrible HTML
    self.write(self.renderPageCSS())
    card = self.fetchCard(set_id, card_index, card_sub_id)
    if card is None:
      self.fourOhFour("Unexpected None from %s-%s" % (set_id, card_id))
      return
    self.write(self.renderCard(card))
    self.write(self.renderWizardsCard(card))
    self.write(self.renderFooterLinks(card))
    # return get
