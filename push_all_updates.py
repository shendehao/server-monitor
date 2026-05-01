#!/usr/bin/env python3
"""Login and push all agent updates: Linux + Windows DLL."""
import json, urllib.request, sys

BASE = "http://127.0.0.1:5000"
PASSWORD = sys.argv[1] if len(sys.argv) > 1 else "Qa2007.8.30"

# Login
data = json.dumps({"username": "admin", "password": PASSWORD}).encode()
req = urllib.request.Request(BASE + "/api/login", data, {"Content-Type": "application/json"})
resp = json.loads(urllib.request.urlopen(req).read())
token = resp.get("data", {}).get("token", "")
if not token:
    print("Login failed:", resp)
    sys.exit(1)
print("Login OK")

headers = {"Content-Type": "application/json", "Authorization": "Bearer " + token, "Host": "47.115.222.73"}

# 1) Push Linux agent update
print("\n[1] Pushing Linux agent update...")
req1 = urllib.request.Request(BASE + "/api/agent/force-update-linux", b"{}", headers)
req1.get_method = lambda: "POST"
try:
    r1 = json.loads(urllib.request.urlopen(req1).read())
    print("  Result:", r1)
except Exception as e:
    print("  Error:", e)

# 2) Push Windows DLL update (self_update to all)
print("\n[2] Pushing Windows DLL update (self_update all)...")
req2 = urllib.request.Request(BASE + "/api/servers/all/force-update-cs", b"{}", headers)
req2.get_method = lambda: "POST"
try:
    r2 = json.loads(urllib.request.urlopen(req2).read())
    print("  Result:", r2)
except Exception as e:
    print("  Error:", e)

print("\nDone!")
