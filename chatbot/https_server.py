# https_server.py
# Copied from https://gist.github.com/SeanPesce/af5f6b7665305b4c45941634ff725b7a

import getpass
import http.server
import ssl
import sys


def serve(host, port, cert_fpath, privkey_fpath):
    context = ssl.SSLContext(ssl.PROTOCOL_TLS)
    pword = ''
    if 'BEGIN ENCRYPTED PRIVATE KEY' in open(privkey_fpath).read():
        pword = getpass.getpass(prompt='cert password: ')
    context.load_cert_chain(certfile=cert_fpath, keyfile=privkey_fpath, password=pword)
    server_address = (host, port)
    httpd = http.server.HTTPServer(server_address, http.server.SimpleHTTPRequestHandler)
    httpd.socket = context.wrap_socket(httpd.socket, server_side=True)
    httpd.serve_forever()


if __name__ == '__main__':
    if len(sys.argv) < 4:
        print(f'Usage: {sys.argv[0]} <port> <PEM certificate file> <private key file>')
        sys.exit()
    
    port = int(sys.argv[1])
    cert_path = sys.argv[2]
    privkey_path = sys.argv[3]
    
    serve('0.0.0.0', port, cert_path, privkey_path)
