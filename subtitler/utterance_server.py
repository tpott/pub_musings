# utterance_server.py
# Trevor Pottinger
# Mon May  4 08:38:25 PDT 2020

import email.utils
import http.server
import os
import random
import socketserver
import shutil
import ssl
import sys
import time
import traceback
from typing import NewType, Optional, Tuple
import urllib

import matplotlib
matplotlib.use('agg')
import matplotlib.pyplot as plt
import numpy as np
import scipy.io.wavfile
import scipy.signal

# Run as `python3 utterance_server.py`

FloatSeconds = NewType('FloatSeconds', float)

# https://docs.python.org/3/tutorial/inputoutput.html#methods-of-file-objects
OFFSET_FROM_START = 0


def readAndSpectro(start: FloatSeconds, end: FloatSeconds, max_duration: FloatSeconds, video) -> Tuple[int, int, np.ndarray, np.ndarray, np.ndarray, np.ndarray]:
  """return spectro: np.ndarray[dtype=float64, shape=[Nrows, Nfreqs]]"""
  rate, data = scipy.io.wavfile.read('data/audios/%s.wav' % video)
  length = data.shape[0] / rate
  # TODO parameterize these limits
  if data.shape[1] > 1:
    # Take the first channel
    data = data[:, 0]
  start_i, end_i = 0, length
  mean = (start + end) / 2
  half_duration = max_duration / 2
  if half_duration < mean:
    start_i = int((mean - half_duration) * rate)
  min_time = start_i / rate
  if mean < length - half_duration:
    end_i = int((mean + half_duration) * rate)
  data = data[start_i : end_i].copy()
  window_size = int(rate) // 100
  step_size = window_size // 2
  windows_per_second = int(rate) // step_size
  freqs, times, spectro = scipy.signal.stft(
    data,
    rate,
    window='hann', # default, as specified by the documentation (listed above)
    nperseg=window_size,
    noverlap=window_size // 2
  )
  return windows_per_second, rate, data, times + min_time, freqs, np.abs(spectro).T


def parseRangeHeader(range_header: str) -> Optional[Tuple[int, Optional[int]]]:
  unit, range_str = range_header.split('=', 1)
  if unit != 'bytes':
    return None
  start_str, end_str = range_str.split('-', 2)
  if start_str == '0' and end_str == '':
    return None
  start = int(start_str)
  if end_str == '':
    return (start, None)
  end = int(end_str)
  assert end >= start, 'got an invalid byte range, end (%d) < start (%d)' % (end, start)
  return (start, end)


# This class is necessary to use the below syntax of `with Server(..) as ..`
class TCPServer(socketserver.TCPServer):
  def __enter__(self):
    return self

  def __exit__(self, *args):
    self.server_close()

  def setExample(self, obj):
    self.example = obj

  def getExample(self):
    return self.example


def urandom(n: int) -> bytes:
  return open('/dev/urandom', 'rb').read(n)


# primary entry point is through do_GET
class MyHandler(http.server.SimpleHTTPRequestHandler):

  def __init__(self, *args, **kwargs):
    # Runs once for every request
    super().__init__(*args, **kwargs)

  def setup(self):
    if type(self.server) == TCPServer:
      self.example = self.server.getExample()
    else:
      self.example = None
    super().setup()

  def _myRedirect(self, url: str) -> None:
    self.send_response(http.server.HTTPStatus.FOUND)
    self.send_header('Location', url)
    self.end_headers()
    return

  def getStatusPage(self):
    s = b'1-AM-ALIVE'
    self.send_response(http.server.HTTPStatus.OK)
    # Other potentially good headers: Content-type, Last-Modified
    self.send_header('Content-Length', len(s))
    self.send_header('Content-Type', 'text/html; charset=utf-8')
    self.end_headers()
    self.wfile.write(s)
    return

  # TODO rename this to not sound so generic...
  def redirect(self, video_id: str, utterance_id: int, duration: FloatSeconds) -> None:
    self._myRedirect('/label?video={video_id}&utterance={utterance_id}&max_duration={duration}'.format(
      duration=duration,
      video_id=video_id,
      utterance_id=utterance_id,
    ))
    return

  def getLabelPage(self, query_string):
    data = urllib.parse.parse_qs(query_string)
    # TODO check 'video' in data
    video_id = data['video'][0]
    utterance_id = int(data['utterance'][0])
    max_duration = FloatSeconds(3.0)
    if 'max_duration' in data:
      max_duration = FloatSeconds(float(data['max_duration'][0]))
    utterance = None
    tsv_file = 'data/tsvs/labeled_%s.tsv' % video_id
    if not os.path.exists(tsv_file):
      tsv_file = 'data/tsvs/%s.tsv' % video_id

    with open(tsv_file, 'rb') as f:
      line_number = -1
      for line in f:
        cols = line.decode('utf-8').rstrip('\n').split('\t')
        if len(cols) == 0 or cols[0] == '':
          continue
        line_number += 1
        if line_number != utterance_id:
          continue
        start = FloatSeconds(float(cols[0]))
        end = FloatSeconds(float(cols[1]))
        duration = FloatSeconds(float(cols[2]))
        if abs(end - start - duration) > 0.0001:
          print('Utterance %d for video %s has an incorrect duration, %f. It should be %f' % (
            utterance_id,
            video_id,
            duration,
            end - start,
          ))
          duration = end - start
        # TODO parameterize these limits
        if duration + 0.01 > max_duration:
          self.redirect(video_id, utterance_id, duration + 0.01)
          return
        assert duration < max_duration, 'Duration, %s, too long for labeling' % cols[2]
        utterance = {
          'start': start,
          'end': end,
          'duration': duration,
          'content': cols[3],
        }

    # TODO make this deterministic to avoid re-creating it
    tmp_file_id = '%s-%d' % (video_id, utterance_id)
    image_file = 'data/tmp/%s.png' % tmp_file_id
    audio_file = 'data/tmp/%s.wav' % tmp_file_id
    _windows_per_sec, rate, data, times, freqs, spectro = readAndSpectro(
      utterance['start'],
      utterance['end'],
      max_duration,
      video_id
    )

    # TODO add a GET parameter to toggle this
    # Problem is when changing the TSV, the start and end times may change,
    # but the image already got cached.
    ignore_cache = True
    if ignore_cache or not os.path.exists(image_file):
      # TODO parameterize these limits
      max_freq = 210
      smaller_freqs = np.arange(freqs.shape[0])[:max_freq]
      spectro_display = np.log2(np.maximum(np.abs(spectro)[:, :max_freq].T, 0.001))
      plt.pcolormesh(times, smaller_freqs, spectro_display)
      plt.axvline(x=utterance['start'], color='#d62728')
      plt.axvline(x=utterance['end'], color='#d62728')
      plt.ylabel('Frequency')
      plt.xlabel('Time (seconds)')
      plt.gcf().set_size_inches([15, 4]) # default is 6 x 4
      plt.savefig(image_file)
      plt.clf()

    if ignore_cache or not os.path.exists(audio_file):
      scipy.io.wavfile.write(audio_file, rate, data)

    webpage_s = """<html>
  <head>
    <title>Utterance Labeler</title>
    <link rel="icon" href="/tmp/favicon.png" />
    <script type="text/javascript">

// Double curly braces are because this webpage is from python. It's a big
// template string with `.format(...)` called on it.
function redirect(n) {{
  // .slice(1) to drop the leading '?'
  let query_string = window.location.search.slice(1);
  let new_params = [];
  // TODO surely there is a better library for doing this in JS
  query_string.split('&').forEach((param) => {{
    let [param_name, param_value] = param.split('=');
    if (param_name !== 'utterance') {{
      new_params.push(param);
      return;
    }}
    let utterance_id = parseInt(param_value) + n;
    new_params.push(([param_name, utterance_id.toString()]).join('='));
  }});
  let new_query_string = '?' + new_params.join('&');
  let dest = window.location.origin + window.location.pathname + new_query_string;
  window.location = dest;
}}

function play() {{
  document.getElementsByTagName('audio')[0].play();
}}

document.addEventListener('keydown', event => {{
  let LEFT = 37;
  let RIGHT = 39;
  let N = 78;
  let P = 80;
  let R = 82;
  let Y = 89;
  switch (event.keyCode) {{
    case RIGHT:
    case N:
      redirect(1);
      break;
    case LEFT:
    case P:
      redirect(-1);
      break;
    case R:
      redirect(0);
      break;
    case Y:
      play();
      break;
  }}
}});

    </script>
  </head>
  <body>
    <p>Video = {video_id}</p>
    <img src="{image_file}" />
    <audio controls src="{audio_file}">
      Your browser doesn't support audio
    </audio>
    <!-- Other options are a <div> with <span>s -->
    <!-- Or just a plain <p> with JSON -->
    <!-- But I'd like to make this editable -->
    <table>
      <tr>
        <td>"start": {start},</td>
        <td>"end": {end},</td>
        <td>"duration": {duration},</td>
        <td>"content": "{content}",</td>
      </tr>
      <tr>
        <td>"num_rows": {num_rows},</td>
        <td>"num_cols": {num_cols},</td>
        <td></td>
        <td></td>
      </tr>
    </table>
  </body>
</html>""".format(
      audio_file=audio_file,
      content=utterance['content'],
      duration=utterance['duration'],
      end=utterance['end'],
      image_file=image_file,
      num_rows=spectro.shape[0],
      num_cols=spectro.shape[1],
      start=utterance['start'],
      video_id=video_id,
    )

    s_bytes = webpage_s.encode('utf-8')
    self.send_response(http.server.HTTPStatus.OK)
    self.send_header('Content-Length', len(s_bytes))
    self.send_header('Content-Type', 'text/html; charset=utf-8')
    self.end_headers()
    self.wfile.write(s_bytes)
    return
    # end getLabelPage

  def getAlignPage(self, query_string):
    data = urllib.parse.parse_qs(query_string)
    # TODO check 'video' in data
    video_id = data['video'][0]

    # TODO read data/tsvs/labeled_{video_id}
    # TODO read data/lyrics/{video_id}
    # TODO call alignLyrics from align_lyrics
    webpage_s = """<html>
  <head>
    <title>Utterance Aligner</title>
    <link rel="icon" href="/tmp/favicon.png" />
    <script type="text/javascript">
    </script>
  </head>
  <body>
    <p>Video = {video_id}</p>
  </body>
</html>
""".format(video_id=video_id)

    s_bytes = webpage_s.encode('utf-8')
    self.send_response(http.server.HTTPStatus.OK)
    self.send_header('Content-Length', len(s_bytes))
    self.send_header('Content-Type', 'text/html; charset=utf-8')
    self.end_headers()
    self.wfile.write(s_bytes)
    return
    # end getAlignPage

  def getAnchorPage(self, query_string):
    data = urllib.parse.parse_qs(query_string)
    # TODO check 'video' in data
    video_id = data['video'][0]
    audio_file = 'tmp/%s.wav' % video_id

    if not os.path.exists(audio_file):
      # TODO make this configurable
      shutil.copyfile('data/audios/%s.wav' % video_id, audio_file)
      # shutil.copyfile('data/audios/%s/vocals_left.wav' % video_id, audio_file)

    i = 0
    utts = []
    with open('data/tsvs/labeled_{video_id}.tsv'.format(video_id=video_id), 'r') as f:
      for line in f:
        start_s, end_s, duration_s, content = line.rstrip('\n').split('\t')
        utts.append((
          i,
          float(start_s),
          float(end_s),
          float(duration_s),
          content
        ))
        i += 1

    def _rowFormat(utt):
      label_url = '/label?video={video_id}&utterance={i}'.format(
        video_id=video_id,
        i=utt[0]
      )
      return """<tr>
  <td><a href="{label_url}">{i}</a></td>
  <td><input type="radio" name="select_start" value="{i}" /><!-- onClick=disableRadios --></td>
  <td><input type="radio" name="select_end" value="{i}" /><!-- onClick=disableRadios --></td>
  <td>{start:0.3f}</td>
  <td>{end:0.3f}</td>
  <td>{duration:0.3f}</td>
  <td><a href="{label_url}">{content}</a></td>
</tr>""".format(
        i=utt[0],
        label_url=label_url,
        start=utt[1],
        end=utt[2],
        duration=utt[3],
        content=utt[4]
      )
    utt_rows = "\n".join(map(_rowFormat, utts))

    # TODO static link to full wav audio
    # TODO javascript to auto pause every N seconds
    # TODO select last uttered word
    # TODO javascript to redistribute
    webpage_s = """<html>
  <head>
    <title>Utterance Anchorizer</title>
    <link rel="icon" href="/tmp/favicon.png" />
    <style>
.time_text_input {{
  width: 60px;
}}
    </style>
    <script type="text/javascript">

function toggleAudio() {{
  let audio_elem = document.getElementsByTagName('audio')[0];
  if (audio_elem.paused) {{
    audio_elem.play();
  }} else {{
    audio_elem.pause();
  }}
}}

function removeAllChildren(elem) {{
  for (let child = elem.firstElementChild;
	   child !== null;
	   child = elem.firstElementChild) {{
    elem.removeChild(child);
  }}
}}

function makeInputRow(row) {{
  // This would be a lot easier if we were using React..
  let start_input = document.createElement('input');
  let end_input = document.createElement('input');
  let duration_input = document.createElement('input');
  start_input.type = 'text';
  end_input.type = 'text';
  duration_input.type = 'text';
  start_input.className = 'time_text_input';
  end_input.className = 'time_text_input';
  duration_input.className = 'time_text_input';
  start_input.placeholder = row.children[3].textContent;
  end_input.placeholder = row.children[4].textContent;
  duration_input.placeholder = row.children[5].textContent;
  // Clear out the text before inserting the new dom nodes
  row.children[3].textContent = '';
  row.children[4].textContent = '';
  row.children[5].textContent = '';
  // Alternatively, try using `old_node.replaceWith(new_node)`
  row.children[3].appendChild(start_input);
  row.children[4].appendChild(end_input);
  row.children[5].appendChild(duration_input);
}}

function makeTextRow(row) {{
  // TODO make sure this row does not have editable text
  let start = row.children[3].firstElementChild.placeholder;
  let end = row.children[4].firstElementChild.placeholder;
  let duration = row.children[5].firstElementChild.placeholder;
  removeAllChildren(row.children[3]);
  removeAllChildren(row.children[4]);
  removeAllChildren(row.children[5]);
  row.children[3].textContent = start;
  row.children[4].textContent = end;
  row.children[5].textContent = duration;
}}

// Globals to track if either column has had any boxes checked
let start_selected = null;
let end_selected = null;

// This function will disable radio buttons based on which ones are
// currently selected. It gets called whenever one is clicked
function disableRadios(evt) {{
  let target_name = evt.target.name;
  let radio_name = null;
  let radio_value = parseInt(evt.target.value);

  if (target_name === 'select_start') {{
    radio_name = 'select_end';
    start_selected = radio_value;
  }} else if (target_name === 'select_end') {{
    radio_name = 'select_start';
    end_selected = radio_value;
  }} else {{
    throw 'Expected radio name to be select_start or select_end';
  }}

  let inputs = Array.from(document.getElementsByTagName('input'));
  inputs.forEach((elem) => {{
    if (elem.type !== 'radio') {{
      return;
    }}
    if (elem.name !== radio_name) {{
      return;
    }}
    if ((radio_name === 'select_start' && radio_value < parseInt(elem.value))
        || (radio_name === 'select_end' && radio_value > parseInt(elem.value))) {{
      elem.disabled = true;
    }} else {{
      elem.disabled = false;
    }}
    // TODO <input type="text" placeholder="start/end" />
    // TODO ajax submit button that writes to data/tsvs/labeled_video_id.tsv
  }});

  // TODO get the data into a const JS array
  if (start_selected === null || end_selected === null) {{
    return;
  }}
  let rows = Array.from(document.getElementsByTagName('tr'));

  // slice(1) skips over the header row
  rows.slice(1).forEach((row, i) => {{
    // Skip rows where it already has <input> elements
    let has_input_elements = row.children[3].children.length > 0;
    let selected_rows = start_selected <= i && i <= end_selected;
    if (!has_input_elements && selected_rows) {{
      makeInputRow(row);
    }} else if (has_input_elements && !selected_rows) {{
      makeTextRow(row);
    }}
  }});
}}

function submitAsync(evt) {{
  let xhttp = new XMLHttpRequest();
  xhttp.onreadystatechange = () => {{
    console.log('hi!');
    console.log(this.readyState);
    console.log(this.status);
    console.log(this.responseText);
  }};
  xhttp.open('POST', '/post_anchors', true);
  xhttp.send();
}}

document.addEventListener('keydown', event => {{
  let enter = 13; // enter key
  let space = 32; // space bar
  let Y = 89; // 'y'
  switch (event.keyCode) {{
    case space:
      toggleAudio();
      if (event.target === document.body) {{
        event.preventDefault();
      }}
      break;
    case Y:
      toggleAudio();
      break;
    case enter:
      submitAsync(event);
      break;
  }}
}});

// TODO setInterval and set background-color: #dedede

    </script>
  </head>
  <body>
    <p>Video = {video_id}</p>
    <audio controls src="{audio_file}" style="width: 600px">
      Your browser doesn't support audio
    </audio>
    <form>
      <table>
        <tr>
        <th>utt_id</th>
        <th><!-- radio select_start --></th>
        <th><!-- radio select_end --></th>
        <th>start</th>
        <th>end</th>
        <th>duration</th>
        <th>content</th>
        </tr>
{utt_rows}
      </table>
    </form>
    <script type="text/javascript">
let inputs = Array.from(document.getElementsByTagName('input'));
inputs.forEach((elem) => {{
  if (elem.type !== 'radio') {{
    return;
  }}
  elem.addEventListener('click', disableRadios);
}});
    </script>
  </body>
</html>
""".format(
        audio_file=audio_file,
        video_id=video_id,
        utt_rows=utt_rows
    )

    s_bytes = webpage_s.encode('utf-8')
    self.send_response(http.server.HTTPStatus.OK)
    self.send_header('Content-Length', len(s_bytes))
    self.send_header('Content-Type', 'text/html; charset=utf-8')
    self.end_headers()
    self.wfile.write(s_bytes)
    return
    # end getAnchorPage


  def getStatic(self, path):
    # Copied from https://github.com/python/cpython/blob/3.8/Lib/http/server.py
    parts = path.split('/')
    assert len(parts) == 3, 'Got a longer image filename than expected'
    assert parts[0] == '', 'Expected the first part to be the empty string'
    assert parts[1] == 'tmp'
    filename = parts[2]
    parts = filename.split('.')
    assert len(parts) == 2
    assert parts[1] in set(['png', 'wav'])
    byte_range = None
    range_header = self.headers.get('Range')
    if range_header is not None:
      byte_range = parseRangeHeader(range_header)
    local_path = 'data/tmp/%s.%s' % (parts[0], parts[1])
    with open(local_path, 'rb') as f:
      # TODO fix this byte range servicing
      if False and byte_range is not None:
        f.seek(byte_range[0], OFFSET_FROM_START)
        if byte_range[1] is not None:
          f_bytes = f.read(byte_range[1] - byte_range[0] + 1)
        else:
          f_bytes = f.read()
        self.send_header('Accept-Ranges', 'bytes')
        self.send_header('Content-Length', str(len(f_bytes)))
        self.send_header('Content-Type', 'application/octet-stream')
        # TODO check file stats last modified
        self.send_header(
          'Last-Modified',
          email.utils.formatdate(time.time(), usegmt=True)
        )
        self.end_headers()
        try:
          self.wfile.write(f_bytes)
        except Exception as e:
          print('got an exception!')
          print(e)
        return
      fs = os.fstat(f.fileno())
      self.send_response(http.server.HTTPStatus.OK)
      # self.send_header('Accept-Ranges', 'bytes')
      self.send_header('Content-Length', str(fs[6]))
      self.send_header('Content-Type', 'application/octet-stream')
      # and/or check X-Content-Duration and sync with audio.currentTime
      # TODO check file stats last modified
      self.send_header(
        'Last-Modified',
        email.utils.formatdate(time.time(), usegmt=True)
      )
      self.end_headers()
      shutil.copyfileobj(f, self.wfile)
    return


  def getHandler(self):
    request = urllib.parse.urlparse(self.path)
    # path in {'/' => 'start button', '/label' => 'image + audio'}
    # TODO move this check after the status check
    if request.path == '/status':
      self.getStatusPage()
      return
    if request.path == '/align':
      self.getAlignPage(request.query)
      return
    if request.path == '/anchor':
      self.getAnchorPage(request.query)
      return
    if request.path == '/label':
      self.getLabelPage(request.query)
      return
    if request.path.startswith('/tmp/'):
      self.getStatic(request.path)
      return
    s = b'Unknown page'
    self.send_response(http.server.HTTPStatus.NOT_FOUND)
    self.send_header('Content-Length', len(s))
    self.send_header('Content-Type', 'text/html; charset=utf-8')
    self.end_headers()
    self.wfile.write(s)
    return


  def do_GET(self):
    # This gets called because this is a http.server.SimpleHTTPRequestHandler
    # parse_request calls http.client.parse_headers and sets self.headers
    s, e_type, e, trace = None, None, None, None
    try:
      self.getHandler()
    except Exception:
      e_type, e, trace = sys.exc_info()
      # TODO add stack trace and exception message
      s = b'<div>Unexpected exception, %s: %s</div>\n' % (e_type.__name__.encode('utf-8'), str(e).encode('utf-8'))
      s += "\n".join(map(lambda s: '<div>%s</div>' % s, traceback.format_tb(trace))).encode('utf-8')
    finally:
      del e_type, e, trace
    if s is None:
      return
    self.send_response(http.server.HTTPStatus.INTERNAL_SERVER_ERROR)
    self.send_header('Content-Length', len(s))
    self.send_header('Content-Type', 'text/html; charset=utf-8')
    self.end_headers()
    self.wfile.write(s)
    return


  def postHandler(self):
    request = urllib.parse.urlparse(self.path)
    # path in {'/' => 'start button', '/label' => 'image + audio'}
    # TODO move this check after the status check
    if request.path == '/post_anchor':
      print(request)
      s = b'1-AM-ALIVE'
      self.send_response(http.server.HTTPStatus.OK)
      # Other potentially good headers: Content-type, Last-Modified
      self.send_header('Content-Length', len(s))
      self.send_header('Content-Type', 'text/html; charset=utf-8')
      self.end_headers()
      self.wfile.write(s)
    return


  def do_POST(self):
    s, e_type, e, trace = None, None, None, None
    try:
      self.postHandler()
    except Exception:
      e_type, e, trace = sys.exc_info()
      # TODO add stack trace and exception message
      s = b'<div>Unexpected exception, %s: %s</div>\n' % (e_type.__name__.encode('utf-8'), str(e).encode('utf-8'))
      s += "\n".join(map(lambda s: '<div>%s</div>' % s, traceback.format_tb(trace))).encode('utf-8')
    finally:
      del e_type, e, trace
    if s is None:
      return
    self.send_response(http.server.HTTPStatus.INTERNAL_SERVER_ERROR)
    self.send_header('Content-Length', len(s))
    self.send_header('Content-Type', 'text/html; charset=utf-8')
    self.end_headers()
    self.wfile.write(s)
    return


  # handlers: [start, stop, label result, next (utterance), random (utterance)]
  # length = self.headers.get('content-length')
  # data = self.rfile.read(int(length))
  # datastr = data.decode('utf-8')
  # oh, and `for(;;);`


def main():
  seed = urandom(7)
  shutil.copyfile('favicon.png', 'data/tmp/favicon.png')
  print('Seeded with:', int(seed.hex(), 16))
  random.seed(int(seed.hex(), 16))

  PORT = 8000
  if len(sys.argv) > 1:
    PORT = int(sys.argv[1])

  # Generate a self signed cert by running the following:
  # `openssl req -nodes -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 30`
  context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
  context.load_cert_chain('cert.pem', 'key.pem')

  # This avoids "Address already in use" errors while testing
  TCPServer.allow_reuse_address = True
  with TCPServer(('0.0.0.0', PORT), MyHandler) as httpd:
    httpd.setExample({
      'wut': 'example data!',
    })
    print('Serving at port:', PORT)
    print('Try https://localhost:%d/label?video=wr2sVPTacTE&utterance=0' % PORT)
    httpd.socket = context.wrap_socket(httpd.socket, server_side=True)
    httpd.serve_forever()


if __name__ == '__main__':
  main()
