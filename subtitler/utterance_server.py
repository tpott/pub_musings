# utterance_server.py
# Trevor Pottinger
# Mon May  4 08:38:25 PDT 2020

import http.server
import random
import socketserver
import ssl
import sys
import urllib

# Run as `python3 utterance_server.py`


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

  def myRedirect(self, url: str) -> None:
    self.send_response(http.server.HTTPStatus.FOUND)
    self.send_header('Location', '/%s' % game_id)
    self.end_headers()
    return

  def getLabelPage(self, query_string):
    data = urllib.parse.parse_qs(query_string)
    # TODO check 'video' in data
    video_id = data['video'][0]
    utterance_id = int(data['utterance'][0])
    utterance = None
    with open('tsvs/%s.tsv' % video_id, 'rb') as f:
      line_number = -1
      for line in f:
        line_number += 1
        if line_number != utterance_id:
          continue
        cols = [col.decode('utf-8') for col in line.rstrip(b'\n').split(b'\t')]
        utterance = {
          'start': float(cols[0]),
          'end': float(cols[1]),
          'duration': float(cols[2]),
          'content': cols[3],
        }
    print('Labeling', utterance)
    s = """<html>
  <head>
    <title>Utterance Labeler</title>
  </head>
  <body>
    <p>Video = {video_id}</p>
  </body>
</html>""".format(
      video_id=video_id,
    )
    s_bytes = s.encode('utf-8')
    self.send_response(http.server.HTTPStatus.OK)
    self.send_header('Content-Length', len(s_bytes))
    self.end_headers()
    self.wfile.write(s_bytes)
    return

  def do_GET(self):
    request = urllib.parse.urlparse(self.path)
    # path in {'/' => 'start button', '/label' => 'image + audio'}
    # TODO move this check after the status check
    if request.path == '/label':
      self.getLabelPage(request.query)
      return
    s = b'hello world'
    if request.path == '/status':
      s = b'1-AM-ALIVE'
    self.send_response(http.server.HTTPStatus.OK)
    # Other potentially good headers: Content-type, Last-Modified
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
  # shutil.copyfile(source, dest) can be handy

  # This avoids "Address already in use" errors while testing
  TCPServer.allow_reuse_address = True
  with TCPServer(('0.0.0.0', PORT), MyHandler) as httpd:
    httpd.setExample({
      'wut': 'example data!',
    })
    print('Serving at port:', PORT)
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
