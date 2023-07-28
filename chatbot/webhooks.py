# webhooks.py
# Inspired from https://gist.github.com/SeanPesce/af5f6b7665305b4c45941634ff725b7a
# and https://github.com/tpott/pub_musings/blob/trunk/subtitler/utterance_server.py

import getpass
import http.server
import socketserver
import ssl
import sys
import traceback
import urllib


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

  def setCallback(self, callback):
    self.callback = callback

  def getCallback(self):
    return self.callback


class MyHandler(http.server.SimpleHTTPRequestHandler):
  def __init__(self, *args, **kwargs):
    # Runs once for every request
    super().__init__(*args, **kwargs)

  def setup(self):
    if type(self.server) == TCPServer:
      self.example = self.server.getExample()
      self.callback = self.server.getCallback()
    else:
      self.example = None
      self.callback = None
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

  def getValidation(self, query):
    # query[hub.mode] == subscribe
    # query[hub.challenge] == what we want 
    # query[hub.verify_token] == what we sent
    params = urllib.parse.parse_qs(query)
    assert len(params['hub.challenge']) > 0, 'expected at least one hub.challenge'
    s = params['hub.challenge'][0].encode('utf-8')
    self.send_response(http.server.HTTPStatus.OK)
    # Other potentially good headers: Content-type, Last-Modified
    self.send_header('Content-Length', len(s))
    self.send_header('Content-Type', 'text/html; charset=utf-8')
    self.end_headers()
    self.wfile.write(s)
    return

  def getHandler(self):
    request = urllib.parse.urlparse(self.path)
    # path in {'/' => 'start button', '/label' => 'image + audio'}
    # TODO move this check after the status check
    if request.path == '/validation':
      self.getValidation(request.query)
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
      # This is a little taboo. We arguably shouldn't be returning detailed error messages
      # via the webserver... But it makes testing so much easier.
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
    if request.path == '/validation':
      print(request)
      print(self.rfile.read())
      print(self.headers.get('X-Hub-Signature-256'))
      s = b'1-AM-ALIVE'
      self.send_response(http.server.HTTPStatus.OK)
      # Other potentially good headers: Content-type, Last-Modified
      self.send_header('Content-Length', len(s))
      self.send_header('Content-Type', 'text/html; charset=utf-8')
      self.end_headers()
      self.wfile.write(s)
      if self.callback is not None:
        self.callback()
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


def serve(host, port, cert_fpath, privkey_fpath, callback=None):
    context = ssl.SSLContext(ssl.PROTOCOL_TLS)
    pword = ''
    if 'BEGIN ENCRYPTED PRIVATE KEY' in open(privkey_fpath).read():
        pword = getpass.getpass(prompt='cert password: ')
    context.load_cert_chain(certfile=cert_fpath, keyfile=privkey_fpath, password=pword)
    server_address = (host, port)
    TCPServer.allow_reuse_address = True
    with TCPServer(server_address, MyHandler) as httpd:
      httpd.setExample({
        'wut': 'example data!',
      })
      httpd.setCallback(callback)
      print('Serving at port:', port)
      httpd.socket = context.wrap_socket(httpd.socket, server_side=True)
      httpd.serve_forever()


if __name__ == '__main__':
    if len(sys.argv) < 4:
        print(f'Usage: {sys.argv[0]} <port> <PEM certificate file> <private key file>')
        sys.exit()
    
    port = int(sys.argv[1])
    cert_path = sys.argv[2]
    privkey_path = sys.argv[3]
    
    serve('0.0.0.0', port, cert_path, privkey_path, None)
