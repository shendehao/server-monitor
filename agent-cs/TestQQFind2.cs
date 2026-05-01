using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
using Microsoft.Win32;

/// 快速定位 NTQQ 数据 — 不做密钥验证，只找文件
class TestQQFind2
{
    [DllImport("kernel32.dll")] static extern IntPtr OpenProcess(int access, bool inherit, int pid);
    [DllImport("kernel32.dll")] static extern bool ReadProcessMemory(IntPtr hProc, IntPtr baseAddr, byte[] buf, int size, out int read);
    [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr h);
    [DllImport("crypt32.dll", SetLastError = true)]
    static extern bool CryptUnprotectData(ref DATA_BLOB pDataIn, IntPtr ppszDesc, IntPtr pOptionalEntropy, IntPtr pvReserved, IntPtr pPromptStruct, int dwFlags, ref DATA_BLOB pDataOut);
    [DllImport("kernel32.dll")] static extern IntPtr LocalFree(IntPtr hMem);
    [StructLayout(LayoutKind.Sequential)]
    struct DATA_BLOB { public int cbData; public IntPtr pbData; }

    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        Console.WriteLine("=== NTQQ 数据快速定位 ===\n");

        string userProfile = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
        string appData = Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);
        string localAppData = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);

        // 1. QQ 进程信息
        Console.WriteLine("[1] QQ 进程:");
        string qqExePath = null;
        try
        {
            var procs = Process.GetProcessesByName("QQ");
            Console.WriteLine("  进程数: " + procs.Length);
            foreach (var p in procs)
            {
                try
                {
                    if (qqExePath == null) qqExePath = p.MainModule.FileName;
                    Console.WriteLine("  PID=" + p.Id + " " + p.MainModule.FileName + " v" + p.MainModule.FileVersionInfo.FileVersion);
                }
                catch { Console.WriteLine("  PID=" + p.Id + " (无法读取)"); }
            }
        }
        catch (Exception ex) { Console.WriteLine("  异常: " + ex.Message); }

        // 2. 注册表
        Console.WriteLine("\n[2] 注册表:");
        string[] regKeys = {
            @"HKCU\Software\Tencent\QQ", @"HKCU\Software\Tencent\QQNT",
            @"HKLM\Software\Tencent\QQ", @"HKLM\Software\Tencent\QQNT",
            @"HKLM\Software\WOW6432Node\Tencent\QQ",
        };
        foreach (string rk in regKeys)
        {
            try
            {
                bool isHKLM = rk.StartsWith("HKLM");
                string subKey = rk.Substring(5);
                RegistryKey root = isHKLM ? Registry.LocalMachine : Registry.CurrentUser;
                using (var key = root.OpenSubKey(subKey))
                {
                    if (key == null) continue;
                    Console.WriteLine("  " + rk + ":");
                    foreach (string vn in key.GetValueNames())
                        Console.WriteLine("    " + vn + " = " + key.GetValue(vn));
                    foreach (string sk in key.GetSubKeyNames())
                        Console.WriteLine("    [子键] " + sk);
                }
            }
            catch { }
        }

        // 3. 广泛搜索所有可能的 QQ 数据路径
        Console.WriteLine("\n[3] 数据目录搜索:");
        var allPaths = new List<string>();
        Action<string, string> addIfExists = (path, label) => {
            if (Directory.Exists(path)) { allPaths.Add(path); Console.WriteLine("  [存在] " + label + ": " + path); }
        };

        addIfExists(Path.Combine(userProfile, "Documents", "Tencent Files"), "Documents/Tencent Files");
        addIfExists(Path.Combine(appData, "Tencent"), "AppData/Roaming/Tencent");
        addIfExists(Path.Combine(appData, "QQ"), "AppData/Roaming/QQ");
        addIfExists(Path.Combine(localAppData, "Tencent"), "AppData/Local/Tencent");
        addIfExists(Path.Combine(localAppData, "QQ"), "AppData/Local/QQ");
        addIfExists(Path.Combine(localAppData, "QQNT"), "AppData/Local/QQNT");

        // QQ 安装目录附近
        if (qqExePath != null)
        {
            string installDir = Path.GetDirectoryName(qqExePath);
            addIfExists(installDir, "QQ安装目录");
            addIfExists(Path.Combine(installDir, "UserData"), "QQ/UserData");
            addIfExists(Path.Combine(installDir, "resources"), "QQ/resources");
            addIfExists(Path.Combine(installDir, "resources", "app"), "QQ/resources/app");
            // 同盘其他位置
            string drive = Path.GetPathRoot(installDir);
            addIfExists(Path.Combine(drive, "QQData"), drive + "QQData");
            addIfExists(Path.Combine(drive, "Tencent Files"), drive + "Tencent Files");
        }

        // 4. 递归搜索所有相关目录内容
        Console.WriteLine("\n[4] 递归搜索目录结构:");
        foreach (string p in allPaths)
        {
            Console.WriteLine("\n  === " + p + " ===");
            PrintTree(p, 0, 4);
        }

        // 5. 全面搜索 .db 文件
        Console.WriteLine("\n[5] 搜索加密 .db 文件:");
        var encryptedDbs = new List<string>();
        foreach (string p in allPaths)
        {
            try
            {
                foreach (string f in Directory.GetFiles(p, "*.db", SearchOption.AllDirectories))
                {
                    try
                    {
                        long sz = new FileInfo(f).Length;
                        if (sz < 4096) continue;
                        byte[] hdr = new byte[16];
                        using (var fs = new FileStream(f, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                            fs.Read(hdr, 0, 16);
                        bool enc = Encoding.ASCII.GetString(hdr, 0, 6) != "SQLite";
                        if (enc)
                        {
                            encryptedDbs.Add(f);
                            Console.WriteLine("  [加密] " + f + " (" + FormatSize(sz) + ") hdr=" + BitConverter.ToString(hdr, 0, 8));
                        }
                        else
                            Console.WriteLine("  [明文] " + f + " (" + FormatSize(sz) + ")");
                    }
                    catch { }
                }
            }
            catch { }
        }

        // 6. 搜索 passphrase / config 文件
        Console.WriteLine("\n[6] 搜索密钥/配置文件:");
        foreach (string p in allPaths)
        {
            try
            {
                foreach (string f in Directory.GetFiles(p, "*", SearchOption.AllDirectories))
                {
                    string fn = Path.GetFileName(f).ToLowerInvariant();
                    bool interesting = fn.Contains("passphrase") || fn == "config.json" ||
                                       fn == "key" || fn == "key.dat" || fn == "session.json" ||
                                       fn.EndsWith(".key") || fn == "protocoldevice.json";
                    if (!interesting) continue;
                    long sz = 0;
                    try { sz = new FileInfo(f).Length; } catch { }
                    if (sz == 0 || sz > 10240) continue;
                    Console.WriteLine("  " + f + " (" + sz + " bytes)");
                    try
                    {
                        byte[] raw = File.ReadAllBytes(f);
                        bool printable = true;
                        for (int i = 0; i < Math.Min(raw.Length, 100); i++)
                            if (raw[i] < 0x09 || (raw[i] > 0x0D && raw[i] < 0x20 && raw[i] != 0x1B)) { printable = false; break; }

                        if (printable)
                        {
                            string txt = Encoding.UTF8.GetString(raw);
                            Console.WriteLine("    TEXT: " + (txt.Length > 300 ? txt.Substring(0, 300) + "..." : txt));
                        }
                        else
                        {
                            Console.WriteLine("    HEX: " + BitConverter.ToString(raw, 0, Math.Min(64, raw.Length)));
                            // DPAPI
                            byte[] dec = DPAPIDecrypt(raw, false);
                            if (dec != null)
                            {
                                Console.WriteLine("    [DPAPI成功] len=" + dec.Length + " hex=" + BytesToHex(dec));
                                string ds = Encoding.UTF8.GetString(dec).Trim();
                                Console.WriteLine("    [DPAPI成功] utf8=" + (ds.Length > 100 ? ds.Substring(0, 100) : ds));
                            }
                        }
                    }
                    catch { }
                }
            }
            catch { }
        }

        // 7. wrapper.node .data 段信息（不做密钥验证）
        Console.WriteLine("\n[7] wrapper.node 模块信息:");
        try
        {
            foreach (var proc in Process.GetProcessesByName("QQ"))
            {
                try
                {
                    foreach (ProcessModule mod in proc.Modules)
                    {
                        if (mod.ModuleName.ToLowerInvariant() != "wrapper.node") continue;

                        Console.WriteLine("  PID=" + proc.Id + " wrapper.node");
                        Console.WriteLine("  base=0x" + mod.BaseAddress.ToString("X") + " size=" + FormatSize(mod.ModuleMemorySize));

                        IntPtr hProc = OpenProcess(0x0010 | 0x0400, false, proc.Id);
                        if (hProc == IntPtr.Zero) { Console.WriteLine("  OpenProcess 失败"); break; }

                        try
                        {
                            byte[] peHdr = new byte[4096];
                            int read;
                            ReadProcessMemory(hProc, mod.BaseAddress, peHdr, 4096, out read);
                            int peOff = BitConverter.ToInt32(peHdr, 0x3C);
                            int secCount = BitConverter.ToInt16(peHdr, peOff + 6);
                            int optSize = BitConverter.ToInt16(peHdr, peOff + 20);
                            int secTable = peOff + 24 + optSize;

                            for (int s = 0; s < secCount && secTable + s * 40 + 40 <= read; s++)
                            {
                                int off = secTable + s * 40;
                                string secName = Encoding.ASCII.GetString(peHdr, off, 8).TrimEnd('\0');
                                int vSize = BitConverter.ToInt32(peHdr, off + 8);
                                int vAddr = BitConverter.ToInt32(peHdr, off + 12);
                                Console.WriteLine("  段: " + secName.PadRight(10) + " RVA=0x" + vAddr.ToString("X8") + " Size=" + FormatSize(vSize));
                            }

                            // 快速统计 .data 段高熵 32 字节块
                            int dataRva = 0, dataSize = 0;
                            for (int s = 0; s < secCount && secTable + s * 40 + 40 <= read; s++)
                            {
                                int off = secTable + s * 40;
                                string secName = Encoding.ASCII.GetString(peHdr, off, 8).TrimEnd('\0');
                                if (secName == ".data")
                                {
                                    dataRva = BitConverter.ToInt32(peHdr, off + 12);
                                    dataSize = BitConverter.ToInt32(peHdr, off + 8);
                                }
                            }

                            if (dataRva > 0)
                            {
                                Console.WriteLine("\n  快速扫描 .data 段高熵候选 (distinct>=16):");
                                int count = 0;
                                int chunkSize = 1024 * 1024;
                                for (int ofs = dataRva; ofs < dataRva + dataSize; ofs += chunkSize)
                                {
                                    int rsz = Math.Min(chunkSize, dataRva + dataSize - ofs);
                                    byte[] chunk = new byte[rsz];
                                    int br;
                                    if (!ReadProcessMemory(hProc, new IntPtr(mod.BaseAddress.ToInt64() + ofs), chunk, rsz, out br) || br < 32)
                                        continue;

                                    for (int i = 0; i <= br - 32; i += 8)
                                    {
                                        if (chunk[i] == 0 && chunk[i + 1] == 0 && chunk[i + 2] == 0 && chunk[i + 3] == 0) continue;
                                        byte[] cand = new byte[32];
                                        Array.Copy(chunk, i, cand, 0, 32);
                                        int d = CountDistinct(cand);
                                        if (d < 16) continue;
                                        count++;
                                        if (count <= 10)
                                            Console.WriteLine("    #" + count + " off=0x" + (ofs + i).ToString("X") + " d=" + d + " " + BytesToHex(cand));
                                    }
                                }
                                Console.WriteLine("  共 " + count + " 个高熵候选 (distinct>=16)");
                            }
                        }
                        finally { CloseHandle(hProc); }
                        return; // 只看第一个
                    }
                }
                catch { }
            }
        }
        catch (Exception ex) { Console.WriteLine("  异常: " + ex.Message); }

        Console.WriteLine("\n=== 完毕 ===");
    }

    static void PrintTree(string dir, int depth, int maxDepth)
    {
        if (depth > maxDepth) return;
        string indent = new string(' ', depth * 2 + 2);
        try
        {
            foreach (string f in Directory.GetFiles(dir))
            {
                string fn = Path.GetFileName(f);
                long sz = 0;
                try { sz = new FileInfo(f).Length; } catch { }
                // 只显示重要文件
                string fnl = fn.ToLowerInvariant();
                if (fnl.EndsWith(".db") || fnl.EndsWith(".key") || fnl.EndsWith(".json") ||
                    fnl.Contains("passphrase") || fnl.Contains("config") || fnl.Contains("session") ||
                    fnl.Contains("key") || fnl.EndsWith(".dat") || fnl.EndsWith(".sqlite"))
                    Console.WriteLine(indent + fn + " (" + FormatSize(sz) + ")");
            }
        }
        catch { }
        try
        {
            foreach (string d in Directory.GetDirectories(dir))
            {
                string dn = Path.GetFileName(d).ToLowerInvariant();
                // 跳过缓存
                if (dn == "cache" || dn == "crashpad" || dn == "gpucache" ||
                    dn == "code cache" || dn == "shader cache" || dn == "service worker" ||
                    dn == "blob_storage" || dn == "media-stack") continue;

                int fileCount = 0;
                try { fileCount = Directory.GetFiles(d, "*", SearchOption.AllDirectories).Length; } catch { }
                Console.WriteLine(indent + "[" + Path.GetFileName(d) + "] (" + fileCount + " files)");
                PrintTree(d, depth + 1, maxDepth);
            }
        }
        catch { }
    }

    static int CountDistinct(byte[] d) { var s = new HashSet<byte>(); foreach (byte b in d) s.Add(b); return s.Count; }
    static string BytesToHex(byte[] b) { var sb = new StringBuilder(b.Length * 2); foreach (byte x in b) sb.Append(x.ToString("x2")); return sb.ToString(); }
    static string FormatSize(long b) { if (b < 1024) return b + "B"; if (b < 1024 * 1024) return (b / 1024.0).ToString("F1") + "KB"; return (b / (1024.0 * 1024)).ToString("F1") + "MB"; }

    static byte[] DPAPIDecrypt(byte[] data, bool machine)
    {
        var din = new DATA_BLOB(); var dout = new DATA_BLOB();
        din.cbData = data.Length; din.pbData = Marshal.AllocHGlobal(data.Length);
        Marshal.Copy(data, 0, din.pbData, data.Length);
        try
        {
            if (!CryptUnprotectData(ref din, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, machine ? 4 : 0, ref dout)) return null;
            byte[] r = new byte[dout.cbData]; Marshal.Copy(dout.pbData, r, 0, dout.cbData); LocalFree(dout.pbData); return r;
        }
        finally { Marshal.FreeHGlobal(din.pbData); }
    }
}
