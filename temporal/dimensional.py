# dimensional.py
# Trevor Pottinger
# Mon Mar  7 20:57:47 PST 2022

import datetime
import http.server
import random
import secrets
import socketserver
import ssl
import sys
import traceback
import urllib

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd

from deseasonalize import deseasonalize


EXTRA_WIDE = [12.8, 4.8]
HOSTNAME = "localhost"
PORT = 8000
YMD_FORMAT = "%Y-%m-%d"


# This class is necessary to use the below syntax of `with Server(..) as ..`
class TCPServer(socketserver.TCPServer):
  def __enter__(self):
    return self

  def __exit__(self, *args):
    self.server_close()


# primary entry point is through do_GET
class MyHandler(http.server.SimpleHTTPRequestHandler):

  def __init__(self, *args, **kwargs):
    # Runs once for every request
    super().__init__(*args, **kwargs)

  def getStatusPage(self):
    s = b"1-AM-ALIVE"
    self.send_response(http.server.HTTPStatus.OK)
    # Other potentially good headers: Content-type, Last-Modified
    self.send_header("Content-Length", len(s))
    self.send_header("Content-Type", "text/html; charset=utf-8")
    self.end_headers()
    self.wfile.write(s)
    return

  def getIndexPage(self):
    page = (
      "<html>\n"
      "<head>\n"
      f"<meta http-equiv=\"refresh\" content=\"0;url=https://{HOSTNAME}:{PORT}/explore?n=1&start=2004-01-01&ticker=AAPL\" />\n"
      "</head>\n"
      "</html>"
    )
    s = page.encode("utf-8")
    self.send_response(http.server.HTTPStatus.OK)
    # Other potentially good headers: Content-type, Last-Modified
    self.send_header("Content-Length", len(s))
    self.send_header("Content-Type", "text/html; charset=utf-8")
    self.end_headers()
    self.wfile.write(s)
    return

  def getNextPage(self, inc):
    request = urllib.parse.urlparse(self.path)
    query = urllib.parse.parse_qs(request.query)
    n = query["n"][0]
    ticker = query["ticker"][0]
    start = datetime.datetime.strptime(query["start"][0], YMD_FORMAT)
    next_start = (start + datetime.timedelta(days=inc * 30)).strftime(YMD_FORMAT)
    self.send_response(http.server.HTTPStatus.FOUND)
    self.send_header("Location", f"https://{HOSTNAME}:{PORT}/explore?n={n}&start={next_start}&ticker={ticker}")
    self.end_headers()
    return

  def getExplorePage(self):
    request = urllib.parse.urlparse(self.path)
    query = urllib.parse.parse_qs(request.query)
    n = int(query["n"][0])
    ticker = query["ticker"][0]
    start = datetime.datetime.strptime(query["start"][0], YMD_FORMAT)
    end = (start + datetime.timedelta(days=2 * 365)).strftime(YMD_FORMAT)
    img_url = f"https://{HOSTNAME}:{PORT}/image?n={n}&start={query['start'][0]}&ticker={ticker}&end={end}"

    page = """<html>
  <head>
    <title>Explore</title>
    <script>

// https://stackoverflow.com/questions/5597060/detecting-arrow-key-presses-in-javascript
function checkKey(e) {
  let sp = new URLSearchParams(window.location.search);
  let n = parseInt(sp.get('n'));
  console.log(n);
  e = e || window.event;
  console.log(e.keyCode);
  if (e.keyCode === 37) {
    // left arrow
    window.location = '/prev?' + sp.toString();
    return;
  } else if (e.keyCode === 38) {
    // up arrow
    sp.set('n', n + 1);
    console.log(n + 1);
  } else if (e.keyCode === 39) {
    // right arrow
    window.location = '/next?' + sp.toString();
    return;
  } else if (e.keyCode === 40) {
    // down arrow
    sp.set('n', Math.max(0, n - 1));
    console.log(n - 1);
  }
  window.location.search = sp.toString();
}

document.onkeydown = checkKey;
    </script>
  </head>
  <body>
    <span>Hi</span>
    <div><img src="%s" /></div>
  </body>
</html>""" % img_url
    s = page.encode("utf-8")
    self.send_response(http.server.HTTPStatus.OK)
    # Other potentially good headers: Content-type, Last-Modified
    self.send_header("Content-Length", len(s))
    self.send_header("Content-Type", "text/html; charset=utf-8")
    self.end_headers()
    self.wfile.write(s)
    return

  def getImagePage(self):
    request = urllib.parse.urlparse(self.path)
    query = urllib.parse.parse_qs(request.query)
    n = int(query["n"][0])
    start = query["start"][0]
    end = query["end"][0]
    ticker = query["ticker"][0]

    # TODO pass filename into MyHandler
    df = pd.read_csv("combined_4b5bf22c082afb4d.csv")
    df = df[(df.Name == ticker) & (start <= df.Date) & (df.Date < end)]
    # TODO pass metric into MyHandler
    metric = "Close"
    date_lambda = lambda x: pd.Series({"Values": dict(zip(x.Name, x[metric]))})
    date_df = df.groupby("Date").apply(date_lambda)
    big_df = pd.DataFrame(date_df.Values.values.tolist(), index=date_df.index)
    norm_arr = deseasonalize(n, big_df.transpose().values)
    norm_df = pd.DataFrame(np.transpose(norm_arr), index=big_df.index, columns=big_df.columns)
    # big_df = big_df.dropna(axis=0, how="any")
    axes = big_df[[ticker]].plot(figsize=EXTRA_WIDE)
    norm_df[[ticker]].plot(ax=axes)
    plt.legend(["original_df", "deseasonalized_df"])

    self.send_response(http.server.HTTPStatus.OK)
    self.send_header("Content-Type", "image/png")
    self.end_headers()
    plt.savefig(self.wfile, format="png")
    return

  def getHandler(self):
    request = urllib.parse.urlparse(self.path)
    if request.path == "/":
      self.getIndexPage()
      return
    if request.path == "/status":
      self.getStatusPage()
      return
    if request.path == "/explore":
      self.getExplorePage()
      return
    if request.path == "/next":
      self.getNextPage(1)
      return
    if request.path == "/prev":
      self.getNextPage(-1)
      return
    if request.path == "/image":
      self.getImagePage()
      return
    s = b"Unknown page"
    self.send_response(http.server.HTTPStatus.NOT_FOUND)
    self.send_header("Content-Length", len(s))
    self.send_header("Content-Type", "text/html; charset=utf-8")
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
      s = b"<div>Unexpected exception, %s: %s</div>\n" % (e_type.__name__.encode("utf-8"), str(e).encode("utf-8"))
      s += "\n".join(map(lambda s: "<div>%s</div>" % s, traceback.format_tb(trace))).encode("utf-8")
    finally:
      del e_type, e, trace
    if s is None:
      return
    self.send_response(http.server.HTTPStatus.INTERNAL_SERVER_ERROR)
    self.send_header("Content-Length", len(s))
    self.send_header("Content-Type", "text/html; charset=utf-8")
    self.end_headers()
    self.wfile.write(s)
    return


def main() -> None:
  seed = secrets.randbits(64)
  print(f"Seeded with: {seed}")
  random.seed(seed)

  # Generate a self signed cert by running the following:
  # `openssl req -nodes -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 30`
  context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
  context.load_cert_chain("cert.pem", "key.pem")
  
  # This avoids "Address already in use" errors while testing
  TCPServer.allow_reuse_address = True
  with TCPServer(("0.0.0.0", PORT), MyHandler) as httpd:
    print(f"Serving at {PORT}")
    with context.wrap_socket(httpd.socket, server_side=True) as sock:
      httpd.socket = sock
      httpd.serve_forever()
      # httpd.socket = ssl.wrap_socket(
        # httpd.socket,
        # server_side=True,
        # keyfile="key.pem",
        # certfile="cert.pem",
        # ssl_version=ssl.PROTOCOL_TLSv1_2
      # )
      # httpd.serve_forever()

  return


if __name__ == "__main__":
  main()
