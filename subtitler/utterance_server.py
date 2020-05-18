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
from typing import NewType, Tuple
import urllib

import matplotlib
matplotlib.use('agg')
import matplotlib.pyplot as plt
import numpy as np
import scipy.io.wavfile
import scipy.signal

# Run as `python3 utterance_server.py`

FloatSeconds = NewType('FloatSeconds', float)


def readAndSpectro(start: FloatSeconds, end: FloatSeconds, max_duration: FloatSeconds, video) -> Tuple[int, int, np.ndarray, np.ndarray, np.ndarray, np.ndarray]:
  """return spectro: np.ndarray[dtype=float64, shape=[Nrows, Nfreqs]]"""
  rate, data = scipy.io.wavfile.read('audios/%s.wav' % video)
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
    with open('tsvs/%s.tsv' % video_id, 'rb') as f:
      line_number = -1
      for line in f:
        line_number += 1
        if line_number != utterance_id:
          continue
        cols = [col.decode('utf-8') for col in line.rstrip(b'\n').split(b'\t')]
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
        if duration > max_duration:
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
    image_file = 'tmp/%s.png' % tmp_file_id
    audio_file = 'tmp/%s.wav' % tmp_file_id
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
      max_freq = 110
      smaller_freqs = np.arange(freqs.shape[0])[:max_freq]
      spectro_display = np.abs(spectro)[:, :max_freq].T
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
    self.end_headers()
    self.wfile.write(s_bytes)
    return

  def getImage(self, path):
    # Copied from https://github.com/python/cpython/blob/3.8/Lib/http/server.py
    parts = path.split('/')
    assert len(parts) == 3, 'Got a longer image filename than expected'
    assert parts[0] == '', 'Expected the first part to be the empty string'
    assert parts[1] == 'tmp'
    filename = parts[2]
    parts = filename.split('.')
    assert len(parts) == 2
    assert parts[1] in set(['png', 'wav'])
    local_path = 'tmp/%s.%s' % (parts[0], parts[1])
    with open(local_path, 'rb') as f:
      fs = os.fstat(f.fileno())
      self.send_response(http.server.HTTPStatus.OK)
      self.send_header('Content-type', 'application/octet-stream')
      self.send_header('Content-Length', str(fs[6]))
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
    if request.path == '/label':
      self.getLabelPage(request.query)
      return
    if request.path.startswith('/tmp/'):
      self.getImage(request.path)
      return
    s = b'Unknown page'
    self.send_response(http.server.HTTPStatus.NOT_FOUND)
    self.send_header('Content-Length', len(s))
    self.end_headers()
    self.wfile.write(s)
    return

  def do_GET(self):
    s, e_type, e, trace = None, None, None, None
    try:
      self.getHandler()
    except Exception:
      e_type, e, trace = sys.exc_info()
      # TODO add stack trace and exception message
      s = b'Unexpected exception, %s: %s\n' % (e_type.__name__.encode('utf-8'), str(e).encode('utf-8'))
      s += "\n".join(traceback.format_tb(trace)).encode('utf-8')
    finally:
      del e_type, e, trace
    if s is None:
      return
    self.send_response(http.server.HTTPStatus.INTERNAL_SERVER_ERROR)
    self.send_header('Content-Length', len(s))
    self.end_headers()
    self.wfile.write(s)
    return

  # there's also a do_POST
  # handlers: [start, stop, label result, next (utterance), random (utterance)]
  # length = self.headers.get('content-length')
  # data = self.rfile.read(int(length))
  # datastr = data.decode('utf-8')
  # oh, and `for(;;);`


def main():
  seed = urandom(7)
  print('Seeded with:', int(seed.hex(), 16))
  random.seed(int(seed.hex(), 16))

  PORT = 8000
  if len(sys.argv) > 1:
    PORT = int(sys.argv[1])

  # This avoids "Address already in use" errors while testing
  TCPServer.allow_reuse_address = True
  with TCPServer(('0.0.0.0', PORT), MyHandler) as httpd:
    httpd.setExample({
      'wut': 'example data!',
    })
    print('Serving at port:', PORT)
    print('Try https://localhost:%d/label?video=wr2sVPTacTE&utterance=0' % PORT)
    # Generate a self signed cert by running the following:
    # `openssl req -nodes -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 30`
    httpd.socket = ssl.wrap_socket(
      httpd.socket,
      server_side=True,
      keyfile='key.pem',
      certfile='cert.pem',
      ssl_version=ssl.PROTOCOL_TLSv1_2
    )
    httpd.serve_forever()


if __name__ == '__main__':
  main()
