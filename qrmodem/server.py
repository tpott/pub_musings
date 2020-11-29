# server.py
# Trevor Potttinger
# Sun Nov 29 14:41:03 PST 2020

# Based off of https://github.com/tpott/labyrinth/blob/master/server.py
# Expects to be run in python3.5+

import http.server
import socketserver
import ssl
import sys


# This class is necessary to use the below syntax of `with Server(..) as ..`
class TCPServer(socketserver.TCPServer):
  def __enter__(self):
    return self

  def __exit__(self, *args):
    self.server_close()


def main() -> None:
	port = 8000
	if len(sys.argv) > 1:
		port = int(sys.argv[1])
	assert port > 0, 'Expected port to be positive, got %d' % port
	# This avoids "Address already in use" errors while testing
	TCPServer.allow_reuse_address = True
	# Empty string means listen for 0.0.0.0
	with TCPServer(('', port), http.server.SimpleHTTPRequestHandler) as httpd:
		print('Serving at port %d' % port)
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
