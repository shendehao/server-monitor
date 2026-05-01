#!/usr/bin/env python3
"""Login and push self_update to all Windows agents."""
import json, urllib.request, sys

BASE = "http://127.0.0.1:5000"
USERNAME = "admin"
PASSWORD = sys.argv[1] if len(sys.argv) > 1 else "Qa2007.8.30"

# Login
data = json.dumps({"username": USERNAME, "password": PASSWORD}).encode()
req = urllib.request.Request(BASE + "/api/login", data, {"Content-Type": "application/json"})
resp = json.loads(urllib.request.urlopen(req).read())
token = resp.get("data", {}).get("token", "")
if not token:
    print("Login failed:", resp)
    sys.exit(1)
print("Login OK, token:", token[:20] + "...")

# Push self_update to all
req2 = urllib.request.Request(
    BASE + "/api/servers/all/force-update-cs",
    b"{}",
    {"Content-Type": "application/json", "Authorization": "Bearer " + token, "Host": "47.115.222.73"}
)
req2.get_method = lambda: "POST"
resp2 = json.loads(urllib.request.urlopen(req2).read())
print("Result:", resp2)
