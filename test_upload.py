#!/usr/bin/env python3
"""Test agent upload endpoint."""
import json, urllib.request, sys, io, email.mime.multipart

BASE = "http://127.0.0.1:5000"

# Login
data = json.dumps({"username": "admin", "password": "Qa2007.8.30"}).encode()
req = urllib.request.Request(BASE + "/api/login", data, {"Content-Type": "application/json"})
resp = json.loads(urllib.request.urlopen(req).read())
token = resp["data"]["token"]
print("Token OK")

# Upload test file via multipart
import http.client, mimetypes

boundary = "----WebKitFormBoundary7MA4YWxkTrZu0gW"
body = (
    f"--{boundary}\r\n"
    f'Content-Disposition: form-data; name="file"; filename="test.bin"\r\n'
    f"Content-Type: application/octet-stream\r\n\r\n"
    f"TESTDATA1234567890\r\n"
    f"--{boundary}--\r\n"
).encode()

req2 = urllib.request.Request(
    BASE + "/api/agent/upload?platform=linux",
    body,
    {
        "Content-Type": f"multipart/form-data; boundary={boundary}",
        "Authorization": f"Bearer {token}",
    },
)
try:
    resp2 = urllib.request.urlopen(req2).read().decode()
    print("Response:", resp2)
except urllib.error.HTTPError as e:
    print(f"HTTP {e.code}:", e.read().decode())
