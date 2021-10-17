# server.py
# Trevor Potttinger
# Sun Nov 29 14:41:03 PST 2020

# Based off of https://github.com/tpott/labyrinth/blob/master/server.py
# Expects to be run in python3.5+

import http.server
import socketserver
import ssl
import sys
from threading import Thread


# TCPServer is necessary to use the below syntax of `with Server(..) as ..`
class TCPServer(socketserver.TCPServer):
  def __enter__(self):
    return self

  def __exit__(self, *args):
    self.server_close()


# Redirector makes it easier to run http + https
class Redirector(http.server.BaseHTTPRequestHandler):
  def do_GET(self):
    self.send_response(302)
    self.send_header('Location', 'https://localhost:8443')
    self.end_headers()



def serve_from_thread(host: str, http_port: int, redirector_class) -> None:
  with http.server.HTTPServer(('', http_port), redirector_class) as httpd:
    httpd.serve_forever()


def main() -> None:
  http_port, https_port = 8080, 8443
  if len(sys.argv) > 1:
    http_port = int(sys.argv[1])
  if len(sys.argv) > 2:
    https_port = int(sys.argv[2])
  assert http_port > 0, 'Expected http_port to be positive, got %d' % http_port
  assert https_port > 0, 'Expected https_port to be positive, got %d' % https_port

  Thread(target=serve_from_thread, args=['', http_port, Redirector]).start()

  # This avoids "Address already in use" errors while testing
  TCPServer.allow_reuse_address = True
  # Empty string means listen for 0.0.0.0
  with TCPServer(('', https_port), http.server.SimpleHTTPRequestHandler) as httpd:
    print('Serving at http on port %d and https on %d' % (http_port, https_port))
    # Generate an unsigned cert by running the following:
    # `openssl req -nodes -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 30`
    # Note that on OS X, you will need to import the cert.pem into the system's Keychain
    # and then mark the certificate as trusted for SSL
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
