using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
using Microsoft.Win32;

/// 专门定位 NTQQ 9.9.x 数据目录和密钥
class TestQQFind
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
        Console.WriteLine("=== NTQQ 9.9.x 数据目录深度搜索 ===\n");

        // 1. 检查 QQ 安装目录
        Console.WriteLine("[1] QQ 安装信息:");
        string qqInstallDir = null;
        try
        {
            var procs = Process.GetProcessesByName("QQ");
            if (procs.Length > 0)
            {
                qqInstallDir = Path.GetDirectoryName(procs[0].MainModule.FileName);
                Console.WriteLine("  安装目录: " + qqInstallDir);
                Console.WriteLine("  版本: " + procs[0].MainModule.FileVersionInfo.FileVersion);
            }
        }
        catch (Exception ex) { Console.WriteLine("  异常: " + ex.Message); }

        // 2. 检查注册表所有 QQ/Tencent 相关键
        Console.WriteLine("\n[2] 注册表搜索:");
        string[] regPaths = {
            @"Software\Tencent\QQ",
            @"Software\Tencent\QQNT",
            @"Software\Tencent\QQBrowser",
            @"Software\Tencent",
            @"Software\WOW6432Node\Tencent",
        };
        foreach (string rp in regPaths)
        {
            try
            {
                using (var key = Registry.CurrentUser.OpenSubKey(rp))
                {
                    if (key != null)
                    {
                        Console.WriteLine("  HKCU\\" + rp + ":");
                        foreach (string vn in key.GetValueNames())
                        {
                            object val = key.GetValue(vn);
                            Console.WriteLine("    " + vn + " = " + (val ?? "null"));
                        }
                        foreach (string sk in key.GetSubKeyNames())
                            Console.WriteLine("    [子键] " + sk);
                    }
                }
            }
            catch { }
            try
            {
                using (var key = Registry.LocalMachine.OpenSubKey(rp))
                {
                    if (key != null)
                    {
                        Console.WriteLine("  HKLM\\" + rp + ":");
                        foreach (string vn in key.GetValueNames())
                        {
                            object val = key.GetValue(vn);
                            Console.WriteLine("    " + vn + " = " + (val ?? "null"));
                        }
                    }
                }
            }
            catch { }
        }

        // Uninstall keys
        try
        {
            using (var key = Registry.LocalMachine.OpenSubKey(@"SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall"))
            {
                if (key != null)
                {
                    foreach (string sk in key.GetSubKeyNames())
                    {
                        if (sk.ToLowerInvariant().Contains("qq") || sk.ToLowerInvariant().Contains("tencent"))
                        {
                            using (var sub = key.OpenSubKey(sk))
                            {
                                string loc = sub.GetValue("InstallLocation") as string;
                                string name = sub.GetValue("DisplayName") as string;
                                if (loc != null || name != null)
                                    Console.WriteLine("  Uninstall: " + (name ?? sk) + " → " + (loc ?? "?"));
                            }
                        }
                    }
                }
            }
        }
        catch { }

        // 3. 全面搜索 NTQQ 数据目录
        Console.WriteLine("\n[3] 文件系统搜索 NTQQ 数据:");
        string userProfile = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
        string appData = Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);
        string localAppData = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);

        string[] searchRoots = {
            appData,
            localAppData,
            Path.Combine(userProfile, "Documents"),
            userProfile,
        };

        // 搜索包含 QQ/Tencent/nt_db 的目录
        var foundDirs = new List<string>();
        foreach (string root in searchRoots)
        {
            Console.WriteLine("\n  搜索: " + root);
            try { SearchForQQData(root, 0, 4, foundDirs); }
            catch (Exception ex) { Console.WriteLine("    异常: " + ex.Message); }
        }

        // 也检查 QQ 安装目录附近
        if (qqInstallDir != null)
        {
            // NTQQ 可能在安装盘根目录的 QQ 用户数据
            string installDrive = Path.GetPathRoot(qqInstallDir);
            string[] nearPaths = {
                Path.Combine(installDrive, "QQData"),
                Path.Combine(installDrive, "Tencent Files"),
                Path.Combine(qqInstallDir, "UserData"),
                Path.Combine(qqInstallDir, "resources", "app"),
            };
            foreach (string np in nearPaths)
            {
                if (Directory.Exists(np))
                {
                    Console.WriteLine("  找到安装目录附近: " + np);
                    try { SearchForQQData(np, 0, 3, foundDirs); }
                    catch { }
                }
            }
        }

        // 4. 搜索 *.db 文件（全盘关键路径）
        Console.WriteLine("\n[4] 搜索 NTQQ 数据库文件 (*.db):");
        var dbFiles = new List<string>();
        string[] dbSearchPaths = {
            Path.Combine(appData, "Tencent"),
            Path.Combine(localAppData, "Tencent"),
            Path.Combine(appData, "QQ"),
            Path.Combine(localAppData, "QQ"),
            Path.Combine(userProfile, "Documents", "Tencent Files"),
        };
        if (qqInstallDir != null)
            dbSearchPaths = new List<string>(dbSearchPaths) { qqInstallDir }.ToArray();

        foreach (string sp in dbSearchPaths)
        {
            if (!Directory.Exists(sp)) continue;
            Console.WriteLine("  目录: " + sp);
            try
            {
                foreach (string f in Directory.GetFiles(sp, "*.db", SearchOption.AllDirectories))
                {
                    long sz = 0;
                    try { sz = new FileInfo(f).Length; } catch { }
                    if (sz < 1024) continue;
                    dbFiles.Add(f);

                    byte[] hdr = new byte[16];
                    try { using (var fs = new FileStream(f, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete)) fs.Read(hdr, 0, 16); }
                    catch { }
                    bool enc = Encoding.ASCII.GetString(hdr, 0, Math.Min(6, hdr.Length)) != "SQLite";
                    Console.WriteLine("    " + f.Substring(sp.Length) + " (" + FormatSize(sz) + ") enc=" + enc);
                }
            }
            catch (Exception ex) { Console.WriteLine("    异常: " + ex.Message); }
        }

        // 5. 搜索 passphrase / key 文件
        Console.WriteLine("\n[5] 搜索 passphrase/key 文件:");
        foreach (string sp in dbSearchPaths)
        {
            if (!Directory.Exists(sp)) continue;
            try
            {
                foreach (string f in Directory.GetFiles(sp, "*", SearchOption.AllDirectories))
                {
                    string fn = Path.GetFileName(f).ToLowerInvariant();
                    if (fn.Contains("passphrase") || fn.Contains("key") || fn == "config.json" || fn == "session.json")
                    {
                        long sz = 0;
                        try { sz = new FileInfo(f).Length; } catch { }
                        if (sz == 0 || sz > 10240) continue;
                        Console.WriteLine("  " + f + " (" + sz + " bytes)");

                        // 如果是小文件，显示内容摘要
                        try
                        {
                            byte[] raw = File.ReadAllBytes(f);
                            if (raw.Length <= 256)
                            {
                                // 尝试 UTF-8
                                string txt = Encoding.UTF8.GetString(raw);
                                bool isPrintable = true;
                                foreach (char c in txt)
                                    if (c < 0x20 && c != '\r' && c != '\n' && c != '\t') { isPrintable = false; break; }

                                if (isPrintable && txt.Length > 0)
                                    Console.WriteLine("    内容(text): " + (txt.Length > 200 ? txt.Substring(0, 200) + "..." : txt));
                                else
                                    Console.WriteLine("    内容(hex): " + BitConverter.ToString(raw, 0, Math.Min(64, raw.Length)));

                                // 尝试 DPAPI
                                if (raw.Length >= 16 && !isPrintable)
                                {
                                    Console.WriteLine("    尝试 DPAPI...");
                                    byte[] dec = DPAPIDecrypt(raw, false);
                                    if (dec != null)
                                    {
                                        Console.WriteLine("    [DPAPI成功] 长度=" + dec.Length + " hex=" + BytesToHex(dec));
                                        string ds = Encoding.UTF8.GetString(dec).Trim();
                                        Console.WriteLine("    [DPAPI成功] utf8=" + (ds.Length > 100 ? ds.Substring(0, 100) : ds));
                                    }
                                    else
                                    {
                                        dec = DPAPIDecrypt(raw, true);
                                        if (dec != null)
                                            Console.WriteLine("    [DPAPI机器级成功] 长度=" + dec.Length + " hex=" + BytesToHex(dec));
                                        else
                                            Console.WriteLine("    DPAPI 失败");
                                    }
                                }
                            }
                        }
                        catch { }
                    }
                }
            }
            catch (Exception ex) { Console.WriteLine("    异常: " + ex.Message); }
        }

        // 6. QQ 进程内存 wrapper.node 密钥扫描（如果有数据库可验证）
        Console.WriteLine("\n[6] wrapper.node 内存密钥扫描:");
        byte[] testDbHeader = null;
        string testDbPath = null;
        foreach (string dbf in dbFiles)
        {
            try
            {
                byte[] hdr = new byte[4096];
                using (var fs = new FileStream(dbf, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                {
                    if (fs.Length < 4096) continue;
                    fs.Read(hdr, 0, 4096);
                }
                if (Encoding.ASCII.GetString(hdr, 0, 6) != "SQLite") // 加密的
                {
                    testDbHeader = hdr;
                    testDbPath = dbf;
                    Console.WriteLine("  用于验证的加密数据库: " + dbf);
                    break;
                }
            }
            catch { }
        }

        if (testDbHeader == null)
        {
            Console.WriteLine("  没有找到加密数据库，无法验证密钥");
        }

        try
        {
            var procs = Process.GetProcessesByName("QQ");
            foreach (var proc in procs)
            {
                ProcessModule wrapperMod = null;
                try
                {
                    foreach (ProcessModule mod in proc.Modules)
                    {
                        string mn = mod.ModuleName.ToLowerInvariant();
                        if (mn == "wrapper.node")
                        { wrapperMod = mod; break; }
                    }
                }
                catch { continue; }
                if (wrapperMod == null) continue;

                Console.WriteLine("  PID=" + proc.Id + " wrapper.node base=0x" + wrapperMod.BaseAddress.ToString("X") + " size=" + FormatSize(wrapperMod.ModuleMemorySize));

                IntPtr hProc = OpenProcess(0x0010 | 0x0400, false, proc.Id);
                if (hProc == IntPtr.Zero) { Console.WriteLine("  OpenProcess 失败(需要管理员)"); continue; }

                try
                {
                    IntPtr baseAddr = wrapperMod.BaseAddress;
                    byte[] peHeader = new byte[4096];
                    int read;
                    ReadProcessMemory(hProc, baseAddr, peHeader, 4096, out read);

                    int peOff = BitConverter.ToInt32(peHeader, 0x3C);
                    int secCount = BitConverter.ToInt16(peHeader, peOff + 6);
                    int optSize = BitConverter.ToInt16(peHeader, peOff + 20);
                    int secTable = peOff + 24 + optSize;

                    int dataRva = 0, dataSize = 0;
                    for (int s = 0; s < secCount && secTable + s * 40 + 40 <= read; s++)
                    {
                        int off = secTable + s * 40;
                        string secName = Encoding.ASCII.GetString(peHeader, off, 8).TrimEnd('\0');
                        int vSize = BitConverter.ToInt32(peHeader, off + 8);
                        int vAddr = BitConverter.ToInt32(peHeader, off + 12);
                        if (secName == ".data" || secName == ".rdata")
                            Console.WriteLine("    段: " + secName + " RVA=0x" + vAddr.ToString("X") + " Size=" + FormatSize(vSize));
                        if (secName == ".data") { dataRva = vAddr; dataSize = vSize; }
                    }

                    if (dataRva == 0) { Console.WriteLine("  .data 段未找到"); continue; }

                    Console.WriteLine("  扫描 .data 段 (" + FormatSize(dataSize) + ")...");
                    int candidateCount = 0, validCount = 0;

                    int chunkSize = 1024 * 1024;
                    for (int offset = dataRva; offset < dataRva + dataSize; offset += chunkSize)
                    {
                        int readSize = Math.Min(chunkSize, dataRva + dataSize - offset);
                        byte[] chunk = new byte[readSize];
                        int bytesRead;
                        if (!ReadProcessMemory(hProc, new IntPtr(baseAddr.ToInt64() + offset), chunk, readSize, out bytesRead) || bytesRead < 32)
                            continue;

                        int step = 8; // 64bit aligned
                        for (int i = 0; i <= bytesRead - 32; i += step)
                        {
                            byte b0 = chunk[i], b1 = chunk[i + 1], b2 = chunk[i + 2], b3 = chunk[i + 3];
                            if (b0 == 0 && b1 == 0 && b2 == 0 && b3 == 0) continue;
                            if (b0 == b1 && b1 == b2 && b2 == b3) continue;

                            byte[] cand = new byte[32];
                            Array.Copy(chunk, i, cand, 0, 32);
                            int distinct = CountDistinct(cand);
                            if (distinct < 16) continue;

                            candidateCount++;

                            if (testDbHeader != null)
                            {
                                bool valid = ValidateKey(cand, testDbHeader);
                                if (valid)
                                {
                                    validCount++;
                                    Console.WriteLine("  [有效密钥!] offset=0x" + (offset + i).ToString("X") + " distinct=" + distinct);
                                    Console.WriteLine("  密钥(hex): " + BytesToHex(cand));
                                    // 尝试解密
                                    Console.WriteLine("  尝试解密数据库...");
                                    try
                                    {
                                        string tmp = Path.Combine(Path.GetTempPath(), "qqtest_" + Guid.NewGuid().ToString("N").Substring(0, 6));
                                        File.Copy(testDbPath, tmp, true);
                                        byte[] encDb = File.ReadAllBytes(tmp);
                                        byte[] decDb = DecryptFullDb(cand, encDb);
                                        if (decDb != null)
                                        {
                                            Console.WriteLine("  [解密成功!] 大小=" + FormatSize(decDb.Length));
                                            ParseSqliteMaster(decDb);
                                        }
                                        else Console.WriteLine("  解密失败");
                                        File.Delete(tmp);
                                    }
                                    catch (Exception ex) { Console.WriteLine("  解密异常: " + ex.Message); }
                                    break;
                                }
                            }

                            if (candidateCount <= 3)
                                Console.WriteLine("    候选 #" + candidateCount + " off=0x" + (offset + i).ToString("X") + " d=" + distinct + " hex=" + BytesToHex(cand).Substring(0, 16) + "...");
                        }
                        if (validCount > 0) break;
                    }
                    Console.WriteLine("  扫描完成: 高熵候选=" + candidateCount + " 有效=" + validCount);
                }
                finally { CloseHandle(hProc); }
                break; // 只扫描第一个有 wrapper.node 的进程
            }
        }
        catch (Exception ex) { Console.WriteLine("  异常: " + ex.Message); }

        Console.WriteLine("\n=== 完毕 ===");
    }

    static void SearchForQQData(string dir, int depth, int maxDepth, List<string> found)
    {
        if (depth > maxDepth) return;
        try
        {
            foreach (string sub in Directory.GetDirectories(dir))
            {
                string name = Path.GetFileName(sub).ToLowerInvariant();
                // 跳过无关目录
                if (name.StartsWith(".") || name == "cache" || name == "temp" || name == "tmp" ||
                    name == "logs" || name == "log" || name == "crash" || name == "update" ||
                    name == "uninstall" || name == "stemp") continue;

                if (name == "nt_db" || name == "nt_qq" || name == "databases" ||
                    name.Contains("msg") || name.Contains("chat"))
                {
                    found.Add(sub);
                    Console.WriteLine("    [关键目录] " + sub);
                    // 列出内容
                    try
                    {
                        foreach (string f in Directory.GetFiles(sub))
                        {
                            long sz = 0;
                            try { sz = new FileInfo(f).Length; } catch { }
                            Console.WriteLine("      " + Path.GetFileName(f) + " (" + FormatSize(sz) + ")");
                        }
                        foreach (string d in Directory.GetDirectories(sub))
                            Console.WriteLine("      [目录] " + Path.GetFileName(d));
                    }
                    catch { }
                }

                if (name.Contains("tencent") || name.Contains("qq") || name.Contains("nt_"))
                {
                    Console.WriteLine("    [目录] " + sub + " (depth=" + depth + ")");
                    SearchForQQData(sub, depth + 1, maxDepth, found);
                }
            }
        }
        catch { }
    }

    // ═══ Crypto helpers ═══

    static byte[] DecryptFullDb(byte[] rawKey, byte[] encDb)
    {
        int pageSize = 4096, reserveSize = 48;
        if (encDb == null || encDb.Length < pageSize) return null;
        byte[] salt = new byte[16];
        Array.Copy(encDb, 0, salt, 0, 16);

        // Try SQLCipher 4 first (NTQQ uses v4)
        byte[] encKey = PBKDF2_SHA512(rawKey, salt, 256000, 32);
        byte[] hs = new byte[16];
        for (int i = 0; i < 16; i++) hs[i] = (byte)(salt[i] ^ 0x3a);
        byte[] hmacKey = PBKDF2_SHA512(encKey, hs, 2, 32);

        bool v4ok = VerifyPageHMAC(encDb, 0, encKey, hmacKey, pageSize, reserveSize, true);
        if (!v4ok)
        {
            // Try v3
            using (var kdf = new System.Security.Cryptography.Rfc2898DeriveBytes(rawKey, salt, 64000))
                encKey = kdf.GetBytes(32);
            for (int i = 0; i < 16; i++) hs[i] = (byte)(salt[i] ^ 0x3a);
            using (var kdf = new System.Security.Cryptography.Rfc2898DeriveBytes(encKey, hs, 2))
                hmacKey = kdf.GetBytes(32);
            if (!VerifyPageHMAC(encDb, 0, encKey, hmacKey, pageSize, reserveSize, false))
                return null;
        }

        int totalPages = encDb.Length / pageSize;
        byte[] output = new byte[totalPages * pageSize];
        for (int pg = 0; pg < totalPages; pg++)
        {
            int pgOff = pg * pageSize;
            int encStart = pg == 0 ? pgOff + 16 : pgOff;
            int encLen = pg == 0 ? pageSize - 16 - reserveSize : pageSize - reserveSize;
            if (encStart + encLen > encDb.Length) break;
            byte[] iv = new byte[16];
            Array.Copy(encDb, pgOff + pageSize - reserveSize, iv, 0, 16);
            byte[] encrypted = new byte[encLen];
            Array.Copy(encDb, encStart, encrypted, 0, encLen);
            using (var aes = System.Security.Cryptography.Aes.Create())
            {
                aes.Mode = System.Security.Cryptography.CipherMode.CBC;
                aes.Padding = System.Security.Cryptography.PaddingMode.None;
                aes.Key = encKey; aes.IV = iv;
                using (var dec = aes.CreateDecryptor())
                {
                    byte[] d = dec.TransformFinalBlock(encrypted, 0, encrypted.Length);
                    if (pg == 0)
                    {
                        byte[] hdr = Encoding.ASCII.GetBytes("SQLite format 3");
                        Array.Copy(hdr, 0, output, 0, 15); output[15] = 0;
                        Array.Copy(d, 0, output, 16, d.Length);
                        output[16] = (byte)((pageSize >> 8) & 0xFF);
                        output[17] = (byte)(pageSize & 0xFF);
                        output[20] = (byte)reserveSize;
                    }
                    else Array.Copy(d, 0, output, pgOff, d.Length);
                }
            }
        }
        if (Encoding.ASCII.GetString(output, 0, 15) != "SQLite format 3") return null;
        return output;
    }

    static bool ValidateKey(byte[] rawKey, byte[] dbHeader)
    {
        byte[] salt = new byte[16];
        Array.Copy(dbHeader, 0, salt, 0, 16);
        // v4
        byte[] encKey = PBKDF2_SHA512(rawKey, salt, 256000, 32);
        byte[] hs = new byte[16];
        for (int i = 0; i < 16; i++) hs[i] = (byte)(salt[i] ^ 0x3a);
        byte[] hmacKey = PBKDF2_SHA512(encKey, hs, 2, 32);
        if (VerifyPageHMAC(dbHeader, 0, encKey, hmacKey, 4096, 48, true)) return true;
        // v3
        using (var kdf = new System.Security.Cryptography.Rfc2898DeriveBytes(rawKey, salt, 64000))
            encKey = kdf.GetBytes(32);
        for (int i = 0; i < 16; i++) hs[i] = (byte)(salt[i] ^ 0x3a);
        using (var kdf = new System.Security.Cryptography.Rfc2898DeriveBytes(encKey, hs, 2))
            hmacKey = kdf.GetBytes(32);
        return VerifyPageHMAC(dbHeader, 0, encKey, hmacKey, 4096, 48, false);
    }

    static bool VerifyPageHMAC(byte[] db, int pgOff, byte[] encKey, byte[] hmacKey, int pageSize, int reserveSize, bool useSHA512)
    {
        int dataStart = (pgOff == 0) ? 16 : 0;
        int dataLen = pageSize - dataStart - reserveSize;
        if (pgOff + pageSize > db.Length) return false;
        byte[] hmacInput = new byte[dataLen + 16 + 4];
        Array.Copy(db, pgOff + dataStart, hmacInput, 0, dataLen);
        Array.Copy(db, pgOff + pageSize - reserveSize, hmacInput, dataLen, 16);
        hmacInput[dataLen + 16] = 0; hmacInput[dataLen + 17] = 0; hmacInput[dataLen + 18] = 0; hmacInput[dataLen + 19] = 1;
        if (useSHA512)
        {
            using (var hmac = new System.Security.Cryptography.HMACSHA512(hmacKey))
            {
                byte[] computed = hmac.ComputeHash(hmacInput);
                for (int i = 0; i < Math.Min(reserveSize - 16, computed.Length); i++)
                    if (db[pgOff + pageSize - reserveSize + 16 + i] != computed[i]) return false;
            }
        }
        else
        {
            using (var hmac = new System.Security.Cryptography.HMACSHA1(hmacKey))
            {
                byte[] computed = hmac.ComputeHash(hmacInput);
                for (int i = 0; i < 20; i++)
                    if (db[pgOff + pageSize - reserveSize + 16 + i] != computed[i]) return false;
            }
        }
        return true;
    }

    static byte[] PBKDF2_SHA512(byte[] password, byte[] salt, int iterations, int dkLen)
    {
        int hLen = 64;
        byte[] dk = new byte[dkLen];
        byte[] blockSalt = new byte[salt.Length + 4];
        Array.Copy(salt, blockSalt, salt.Length);
        blockSalt[salt.Length + 3] = 1;
        byte[] u;
        using (var hmac = new System.Security.Cryptography.HMACSHA512(password)) u = hmac.ComputeHash(blockSalt);
        byte[] result = (byte[])u.Clone();
        for (int iter = 1; iter < iterations; iter++)
        {
            using (var hmac = new System.Security.Cryptography.HMACSHA512(password)) u = hmac.ComputeHash(u);
            for (int j = 0; j < hLen; j++) result[j] ^= u[j];
        }
        Array.Copy(result, 0, dk, 0, dkLen);
        return dk;
    }

    static byte[] DPAPIDecrypt(byte[] data, bool machineScope)
    {
        var dataIn = new DATA_BLOB(); var dataOut = new DATA_BLOB();
        dataIn.cbData = data.Length;
        dataIn.pbData = Marshal.AllocHGlobal(data.Length);
        Marshal.Copy(data, 0, dataIn.pbData, data.Length);
        try
        {
            if (!CryptUnprotectData(ref dataIn, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, machineScope ? 0x04 : 0, ref dataOut))
                return null;
            byte[] result = new byte[dataOut.cbData];
            Marshal.Copy(dataOut.pbData, result, 0, dataOut.cbData);
            LocalFree(dataOut.pbData);
            return result;
        }
        finally { Marshal.FreeHGlobal(dataIn.pbData); }
    }

    static void ParseSqliteMaster(byte[] dbData)
    {
        int pageSize = (dbData[16] << 8) | dbData[17];
        if (pageSize == 1) pageSize = 65536;
        Console.WriteLine("  sqlite_master (pageSize=" + pageSize + "):");
        int hdr = 100;
        if (dbData[hdr] != 0x0D) { Console.WriteLine("    页面类型异常: 0x" + dbData[hdr].ToString("X2")); return; }
        int cellCount = (dbData[hdr + 3] << 8) | dbData[hdr + 4];
        Console.WriteLine("    条目数: " + cellCount);
        int ptrStart = hdr + 8;
        for (int c = 0; c < cellCount && c < 50; c++)
        {
            int ptrOff = ptrStart + c * 2;
            if (ptrOff + 2 > dbData.Length) break;
            int cellOff = (dbData[ptrOff] << 8) | dbData[ptrOff + 1];
            if (cellOff >= dbData.Length) continue;
            try
            {
                int p = cellOff; int n;
                long pLen; ReadVarint(dbData, p, out pLen, out n); p += n;
                long rid; ReadVarint(dbData, p, out rid, out n); p += n;
                long rhs; int hb; ReadVarint(dbData, p, out rhs, out hb);
                int rhe = p + (int)rhs; int hp = p + hb;
                var ct = new List<long>();
                while (hp < rhe && hp < dbData.Length) { long st; ReadVarint(dbData, hp, out st, out n); hp += n; ct.Add(st); }
                if (ct.Count < 5) continue;
                int dp = rhe; string type = null, name = null, sql = null; long rp = 0;
                for (int col = 0; col < ct.Count && dp < dbData.Length; col++)
                {
                    long st = ct[col]; int cl = ColSize(st); if (dp + cl > dbData.Length) break;
                    if ((col == 0 || col == 1 || col == 4) && st >= 13 && st % 2 == 1) { int tl = (int)(st - 13) / 2; if (tl > 0 && dp + tl <= dbData.Length) { string v = Encoding.UTF8.GetString(dbData, dp, tl); if (col == 0) type = v; else if (col == 1) name = v; else sql = v; } }
                    else if (col == 3) rp = ReadInt(dbData, dp, cl);
                    dp += cl;
                }
                string sqlPrev = sql != null && sql.Length > 150 ? sql.Substring(0, 150) + "..." : sql;
                Console.WriteLine("    " + (type ?? "?") + " " + (name ?? "?") + " rp=" + rp + " sql=" + sqlPrev);
            }
            catch { }
        }
    }

    static int CountDistinct(byte[] d) { var s = new HashSet<byte>(); foreach (byte b in d) s.Add(b); return s.Count; }
    static void ReadVarint(byte[] d, int p, out long v, out int n) { v = 0; n = 0; for (int i = 0; i < 9 && p + i < d.Length; i++) { v = (v << 7) | (long)(d[p + i] & 0x7F); n = i + 1; if ((d[p + i] & 0x80) == 0) return; } }
    static int ColSize(long st) { if (st == 0 || st == 8 || st == 9) return 0; if (st == 1) return 1; if (st == 2) return 2; if (st == 3) return 3; if (st == 4) return 4; if (st == 5) return 6; if (st == 6 || st == 7) return 8; if (st >= 12 && st % 2 == 0) return (int)(st - 12) / 2; if (st >= 13 && st % 2 == 1) return (int)(st - 13) / 2; return 0; }
    static long ReadInt(byte[] d, int o, int l) { long v = 0; for (int i = 0; i < l && o + i < d.Length; i++) v = (v << 8) | d[o + i]; return v; }
    static string BytesToHex(byte[] b) { var sb = new StringBuilder(b.Length * 2); foreach (byte x in b) sb.Append(x.ToString("x2")); return sb.ToString(); }
    static string FormatSize(long b) { if (b < 1024) return b + "B"; if (b < 1024 * 1024) return (b / 1024.0).ToString("F1") + "KB"; return (b / (1024.0 * 1024)).ToString("F1") + "MB"; }
}
