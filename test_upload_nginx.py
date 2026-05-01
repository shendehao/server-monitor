#!/usr/bin/env python3
"""Test upload through Nginx HTTPS."""
import json, urllib.request, ssl

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

BASE = "https://47.115.222.73"

# Login
d = json.dumps({"username": "admin", "password": "Qa2007.8.30"}).encode()
req = urllib.request.Request(BASE + "/api/login", d, {"Content-Type": "application/json"})
resp = json.loads(urllib.request.urlopen(req, context=ctx).read())
token = resp["data"]["token"]
print("Login OK")

# Upload
boundary = "----test123boundary"
body = (
    "--" + boundary + "\r\n"
    'Content-Disposition: form-data; name="file"; filename="test.bin"\r\n'
    "Content-Type: application/octet-stream\r\n\r\n"
    "TESTDATA1234567890\r\n"
    "--" + boundary + "--\r\n"
).encode()

req2 = urllib.request.Request(
    BASE + "/api/agent/upload?platform=linux",
    body,
    {
        "Content-Type": "multipart/form-data; boundary=" + boundary,
        "Authorization": "Bearer " + token,
    },
)
try:
    resp2 = urllib.request.urlopen(req2, context=ctx).read().decode()
    print("Response:", resp2)
except urllib.error.HTTPError as e:
    print(f"HTTP {e.code}: {e.read().decode()}")
except Exception as e:
    print(f"Error: {type(e).__name__}: {e}")
