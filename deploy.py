#!/usr/bin/env python3
"""Deploy script: upload artifacts and restart server via SSH/SFTP (paramiko)"""
import paramiko, time, sys, os

HOST = "47.115.222.73"
USER = "root"
PASS = "Qa2007.8.30"
REMOTE_DIR = "/www/wwwroot/goo"
AGENT_BIN_DIR = REMOTE_DIR + "/data/agent-bin"

def main():
    print(f"[*] Connecting to {HOST}...")
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASS, timeout=15)
    sftp = ssh.open_sftp()

    # 1. Upload DLL + mapping
    base = os.path.dirname(os.path.abspath(__file__))
    uploads = [
        (os.path.join(base, "agent-cs", "MiniAgent_obf.dll"), AGENT_BIN_DIR + "/MiniAgent.dll"),
        (os.path.join(base, "agent-cs", "obf_mapping.txt"),   AGENT_BIN_DIR + "/obf_mapping.txt"),
        (os.path.join(base, "server", "server-monitor-linux"), REMOTE_DIR + "/serverlinux.new"),
    ]
    for local, remote in uploads:
        sz = os.path.getsize(local)
        print(f"  Upload {os.path.basename(local)} ({sz//1024}KB) -> {remote}")
        sftp.put(local, remote)

    sftp.close()

    # 2. Swap binary and restart
    cmds = [
        f"chmod +x {REMOTE_DIR}/serverlinux.new",
        f"cd {REMOTE_DIR} && (pkill -f './serverlinux' || true)",
        "sleep 1",
        f"cd {REMOTE_DIR} && mv -f serverlinux serverlinux.bak 2>/dev/null; mv serverlinux.new serverlinux",
        f"cd {REMOTE_DIR} && nohup ./serverlinux > /tmp/sl.log 2>&1 &",
        "sleep 3",
    ]
    for cmd in cmds:
        print(f"  $ {cmd}")
        stdin, stdout, stderr = ssh.exec_command(cmd)
        stdout.channel.recv_exit_status()

    # 3. Health check
    print("[*] Health check...")
    stdin, stdout, stderr = ssh.exec_command("curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:5000/api/metrics/overview")
    code = stdout.read().decode().strip()
    print(f"  HTTP status: {code}")

    # 4. Check process
    stdin, stdout, stderr = ssh.exec_command("ps aux | grep '[s]erverlinux'")
    ps = stdout.read().decode().strip()
    print(f"  Process: {'RUNNING' if ps else 'NOT FOUND'}")
    if ps:
        print(f"  {ps[:120]}")

    ssh.close()
    ok = code in ("200", "401", "403")
    print(f"\n{'[+] Deploy SUCCESS' if ok else '[-] Deploy FAILED'}")
    return 0 if ok else 1

if __name__ == "__main__":
    sys.exit(main())
