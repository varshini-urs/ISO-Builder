from http.server import SimpleHTTPRequestHandler, HTTPServer

class MyHTTPRequestHandler(SimpleHTTPRequestHandler):
    def guess_type(self, path):
        if path.endswith(".wasm"):
            return 'application/wasm'
        return super().guess_type(path)

if __name__ == "__main__":
    server_address = ('', 8080)
    httpd = HTTPServer(server_address, MyHTTPRequestHandler)
    print("Serving on port 8080...")
    httpd.serve_forever()
