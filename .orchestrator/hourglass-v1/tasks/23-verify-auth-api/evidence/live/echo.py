# SPDX-License-Identifier: MIT
# throwaway upstream that echoes the request line and headers (task 23, check 9)
import http.server, sys
class H(http.server.BaseHTTPRequestHandler):
    def do_ANY(self):
        body = (self.requestline + "\n" + str(self.headers)).encode()
        self.send_response(200); self.send_header("Set-Cookie", "a=b; Path=/; HttpOnly"); self.send_header("Content-Length", str(len(body))); self.end_headers(); self.wfile.write(body)
    do_GET = do_POST = do_PUT = do_DELETE = do_ANY
    def log_message(self, *a): pass
http.server.HTTPServer(("127.0.0.1", int(sys.argv[1])), H).serve_forever()
