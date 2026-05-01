using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;

/// 独立测试程序：逐步诊断微信+QQ数据提取
class TestExtract
{
    // P/Invoke
    [DllImport("kernel32.dll")] static extern IntPtr OpenProcess(int access, bool inherit, int pid);
    [DllImport("kernel32.dll")] static extern bool ReadProcessMemory(IntPtr hProc, IntPtr baseAddr, byte[] buf, int size, out int read);
    [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr h);
    [DllImport("crypt32.dll", SetLastError = true)]
    static extern bool CryptUnprotectData(ref DATA_BLOB pDataIn, IntPtr ppszDesc, IntPtr pOptionalEntropy, IntPtr pvReserved, IntPtr pPromptStruct, int dwFlags, ref DATA_BLOB pDataOut);
    [DllImport("kernel32.dll")] static extern IntPtr LocalFree(IntPtr hMem);

    [StructLayout(LayoutKind.Sequential)]
    struct DATA_BLOB { public int cbData; public IntPtr pbData; }

    static void Main(string[] args)
    {
        Console.OutputEncoding = Encoding.UTF8;
        Console.WriteLine("=== 微信/QQ 数据提取诊断测试 ===\n");

        Console.WriteLine("══════ 第1部分: 微信 ══════");
        TestWeChat();

        Console.WriteLine("\n══════ 第2部分: QQ/NTQQ ══════");
        TestQQ();

        Console.WriteLine("\n=== 测试完毕 ===");
        Console.WriteLine("请将以上输出全部复制给我分析");
    }

    static void TestWeChat()
    {
        // Step 1: 查找数据目录
        Console.WriteLine("\n[步骤1] 查找微信数据目录...");
        var dataDirs = new List<string[]>(); // [dir, wxid]

        string docsPath = Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments);
        string wechatFilesPath = Path.Combine(docsPath, "WeChat Files");
        Console.WriteLine("  文档目录: " + docsPath);
        Console.WriteLine("  WeChat Files 路径: " + wechatFilesPath);
        Console.WriteLine("  存在: " + Directory.Exists(wechatFilesPath));

        if (Directory.Exists(wechatFilesPath))
        {
            foreach (string dir in Directory.GetDirectories(wechatFilesPath))
            {
                string name = Path.GetFileName(dir);
                if (name.StartsWith("wxid_") || name.StartsWith("Applet") || name == "All Users" || name == "Backup") continue;
                string msgDir = Path.Combine(dir, "Msg");
                if (Directory.Exists(msgDir) || name.StartsWith("wxid"))
                {
                    dataDirs.Add(new string[] { dir, name });
                    Console.WriteLine("  找到账号目录: " + name + " → " + dir);
                }
            }
        }

        // 也检查注册表
        try
        {
            using (var key = Microsoft.Win32.Registry.CurrentUser.OpenSubKey(@"Software\Tencent\WeChat"))
            {
                if (key != null)
                {
                    string installPath = key.GetValue("InstallPath") as string;
                    Console.WriteLine("  注册表安装路径: " + (installPath ?? "null"));
                }
                else Console.WriteLine("  注册表 HKCU\\Software\\Tencent\\WeChat: 不存在");
            }
        }
        catch (Exception ex) { Console.WriteLine("  注册表读取异常: " + ex.Message); }

        if (dataDirs.Count == 0)
        {
            // 尝试所有用户
            string usersRoot = Path.Combine(Environment.GetEnvironmentVariable("SystemDrive") ?? "C:", "Users");
            foreach (string ud in Directory.GetDirectories(usersRoot))
            {
                string dn = Path.GetFileName(ud).ToLowerInvariant();
                if (dn == "public" || dn == "default" || dn == "default user" || dn == "all users") continue;
                string wf = Path.Combine(ud, "Documents", "WeChat Files");
                if (Directory.Exists(wf))
                {
                    Console.WriteLine("  发现其他用户微信目录: " + wf);
                    foreach (string dir in Directory.GetDirectories(wf))
                    {
                        string name = Path.GetFileName(dir);
                        if (name.StartsWith("Applet") || name == "All Users" || name == "Backup") continue;
                        if (Directory.Exists(Path.Combine(dir, "Msg")))
                        {
                            dataDirs.Add(new string[] { dir, name });
                            Console.WriteLine("  找到账号目录: " + name);
                        }
                    }
                }
            }
        }

        Console.WriteLine("  共找到 " + dataDirs.Count + " 个微信账号目录");
        if (dataDirs.Count == 0) { Console.WriteLine("  [失败] 没有找到微信数据目录"); return; }

        // Step 2: 列出数据库文件
        Console.WriteLine("\n[步骤2] 列出数据库文件...");
        string firstDbPath = null;
        foreach (var info in dataDirs)
        {
            string msgDir = Path.Combine(info[0], "Msg");
            if (!Directory.Exists(msgDir)) { Console.WriteLine("  " + info[1] + ": Msg 目录不存在"); continue; }

            string[] importantDbs = { "MicroMsg.db", "ChatMsg.db", "MediaMsg.db", "Emotion.db" };
            foreach (string dbName in importantDbs)
            {
                try
                {
                    foreach (string found in Directory.GetFiles(msgDir, dbName, SearchOption.AllDirectories))
                    {
                        long sz = new FileInfo(found).Length;
                        string rel = found.Substring(info[0].Length + 1);
                        Console.WriteLine("  " + rel + " (" + FormatSize(sz) + ")");

                        // 检查是否加密
                        byte[] header = new byte[16];
                        using (var fs = new FileStream(found, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                            fs.Read(header, 0, 16);
                        bool isEncrypted = Encoding.ASCII.GetString(header, 0, 6) != "SQLite";
                        Console.WriteLine("    加密: " + isEncrypted + " 前6字节: " + BitConverter.ToString(header, 0, 6));

                        if (isEncrypted && firstDbPath == null && sz > 4096)
                            firstDbPath = found;
                    }
                }
                catch (Exception ex) { Console.WriteLine("  枚举 " + dbName + " 异常: " + ex.Message); }
            }
        }

        if (firstDbPath == null) { Console.WriteLine("  [失败] 没有找到加密数据库文件"); return; }
        Console.WriteLine("  用于验证的数据库: " + firstDbPath);

        // Step 3: 提取密钥
        Console.WriteLine("\n[步骤3] 提取微信密钥（从进程内存）...");
        byte[] extractedKey = null;
        try
        {
            var procs = Process.GetProcessesByName("WeChat");
            Console.WriteLine("  WeChat 进程数: " + procs.Length);
            if (procs.Length == 0) { Console.WriteLine("  [失败] 微信未运行，无法提取密钥"); return; }

            foreach (var proc in procs)
            {
                Console.WriteLine("  PID=" + proc.Id);
                ProcessModule wechatWinDll = null;
                try
                {
                    Console.WriteLine("  模块数: " + proc.Modules.Count);
                    foreach (ProcessModule mod in proc.Modules)
                    {
                        if (mod.ModuleName.Equals("WeChatWin.dll", StringComparison.OrdinalIgnoreCase))
                        {
                            wechatWinDll = mod;
                            Console.WriteLine("  找到 WeChatWin.dll: base=0x" + mod.BaseAddress.ToString("X") + " size=" + FormatSize(mod.ModuleMemorySize));
                            break;
                        }
                    }
                }
                catch (Exception ex) { Console.WriteLine("  枚举模块异常: " + ex.Message); continue; }

                if (wechatWinDll == null) { Console.WriteLine("  [失败] 找不到 WeChatWin.dll 模块"); continue; }

                IntPtr hProc = OpenProcess(0x0010 | 0x0400, false, proc.Id);
                Console.WriteLine("  OpenProcess 句柄: " + (hProc != IntPtr.Zero ? "成功" : "失败(需要管理员)"));
                if (hProc == IntPtr.Zero) continue;

                try
                {
                    IntPtr baseAddr = wechatWinDll.BaseAddress;
                    // 读 PE 头定位 .data 段
                    byte[] peHeader = new byte[4096];
                    int read;
                    ReadProcessMemory(hProc, baseAddr, peHeader, 4096, out read);
                    Console.WriteLine("  PE 头读取: " + read + " 字节");

                    int peOff = BitConverter.ToInt32(peHeader, 0x3C);
                    Console.WriteLine("  PE 偏移: 0x" + peOff.ToString("X"));

                    int secCount = BitConverter.ToInt16(peHeader, peOff + 6);
                    int optSize = BitConverter.ToInt16(peHeader, peOff + 20);
                    int secTable = peOff + 24 + optSize;
                    Console.WriteLine("  段数: " + secCount + ", 可选头大小: " + optSize);

                    int dataRva = 0, dataSize = 0;
                    for (int s = 0; s < secCount && secTable + s * 40 + 40 <= read; s++)
                    {
                        int off = secTable + s * 40;
                        string secName = Encoding.ASCII.GetString(peHeader, off, 8).TrimEnd('\0');
                        int vSize = BitConverter.ToInt32(peHeader, off + 8);
                        int vAddr = BitConverter.ToInt32(peHeader, off + 12);
                        Console.WriteLine("    段 " + s + ": " + secName + " RVA=0x" + vAddr.ToString("X") + " Size=0x" + vSize.ToString("X") + " (" + FormatSize(vSize) + ")");
                        if (secName == ".data")
                        {
                            dataRva = vAddr;
                            dataSize = vSize;
                        }
                    }

                    if (dataRva == 0) { Console.WriteLine("  [失败] 找不到 .data 段"); continue; }
                    Console.WriteLine("  .data 段: RVA=0x" + dataRva.ToString("X") + " Size=" + FormatSize(dataSize));

                    // 读取第一个加密数据库的 header 用于验证
                    byte[] dbHeader = new byte[4096];
                    using (var fs = new FileStream(firstDbPath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                        fs.Read(dbHeader, 0, 4096);

                    Console.WriteLine("  开始扫描 .data 段...");
                    int candidateCount = 0;
                    int testedCount = 0;

                    int chunkSize = 512 * 1024;
                    for (int offset = dataRva; offset < dataRva + dataSize && extractedKey == null; offset += chunkSize)
                    {
                        int readSize = Math.Min(chunkSize, dataRva + dataSize - offset);
                        byte[] chunk = new byte[readSize];
                        int bytesRead;
                        if (!ReadProcessMemory(hProc, new IntPtr(baseAddr.ToInt64() + offset), chunk, readSize, out bytesRead) || bytesRead < 32)
                            continue;

                        int step = IntPtr.Size;
                        for (int i = 0; i <= bytesRead - 32; i += step)
                        {
                            byte b0 = chunk[i], b1 = chunk[i + 1], b2 = chunk[i + 2], b3 = chunk[i + 3];
                            if (b0 == 0 && b1 == 0 && b2 == 0 && b3 == 0) continue;
                            if (b0 == b1 && b1 == b2 && b2 == b3) continue;

                            byte[] cand = new byte[32];
                            Array.Copy(chunk, i, cand, 0, 32);
                            int distinct = CountDistinct(cand);
                            if (distinct < 12) continue;

                            candidateCount++;

                            // 用 SQLCipher HMAC 验证
                            bool valid = ValidateKey(cand, dbHeader);
                            if (valid)
                            {
                                extractedKey = cand;
                                Console.WriteLine("  [成功] 找到有效密钥! offset=0x" + (offset + i).ToString("X") + " distinct=" + distinct);
                                Console.WriteLine("  密钥(hex): " + BytesToHex(cand));
                                break;
                            }
                            testedCount++;
                            if (testedCount <= 5)
                                Console.WriteLine("    候选 #" + testedCount + " offset=0x" + (offset + i).ToString("X") + " distinct=" + distinct + " valid=false");
                        }
                    }
                    Console.WriteLine("  扫描完成: 高熵候选 " + candidateCount + " 个, 测试 " + testedCount + " 个");
                }
                finally { CloseHandle(hProc); }
            }
        }
        catch (Exception ex) { Console.WriteLine("  密钥提取异常: " + ex.Message); }

        if (extractedKey == null) { Console.WriteLine("  [失败] 未能提取到有效密钥"); return; }

        // Step 4: 解密数据库
        Console.WriteLine("\n[步骤4] 尝试解密数据库...");
        try
        {
            string tmpFile = Path.Combine(Path.GetTempPath(), "wx_test_" + Guid.NewGuid().ToString("N").Substring(0, 6));
            File.Copy(firstDbPath, tmpFile, true);
            byte[] encDb = File.ReadAllBytes(tmpFile);
            Console.WriteLine("  数据库大小: " + FormatSize(encDb.Length));

            byte[] salt = new byte[16];
            Array.Copy(encDb, 0, salt, 0, 16);
            Console.WriteLine("  Salt: " + BitConverter.ToString(salt));

            // 尝试 SQLCipher 3
            Console.WriteLine("  尝试 SQLCipher 3 (PBKDF2-SHA1 64000次)...");
            byte[] encKey3, hmacKey3;
            using (var kdf = new Rfc2898DeriveBytes(extractedKey, salt, 64000))
                encKey3 = kdf.GetBytes(32);
            byte[] hs3 = new byte[16];
            for (int i = 0; i < 16; i++) hs3[i] = (byte)(salt[i] ^ 0x3a);
            using (var kdf = new Rfc2898DeriveBytes(encKey3, hs3, 2))
                hmacKey3 = kdf.GetBytes(32);

            bool v3ok = VerifyPageHMAC(encDb, 0, encKey3, hmacKey3, 4096, 48, false);
            Console.WriteLine("  SQLCipher 3 HMAC 验证: " + v3ok);

            // 尝试 SQLCipher 4
            Console.WriteLine("  尝试 SQLCipher 4 (PBKDF2-SHA512 256000次)...");
            byte[] encKey4 = PBKDF2_SHA512(extractedKey, salt, 256000, 32);
            byte[] hs4 = new byte[16];
            for (int i = 0; i < 16; i++) hs4[i] = (byte)(salt[i] ^ 0x3a);
            byte[] hmacKey4 = PBKDF2_SHA512(encKey4, hs4, 2, 32);

            bool v4ok = VerifyPageHMAC(encDb, 0, encKey4, hmacKey4, 4096, 48, true);
            Console.WriteLine("  SQLCipher 4 HMAC 验证: " + v4ok);

            if (!v3ok && !v4ok) { Console.WriteLine("  [失败] 密钥验证都不通过"); File.Delete(tmpFile); return; }

            byte[] encKey = v3ok ? encKey3 : encKey4;
            byte[] hmacKey = v3ok ? hmacKey3 : hmacKey4;
            bool useSHA512 = v4ok && !v3ok;
            string cipherVer = v3ok ? "3" : "4";
            Console.WriteLine("  使用 SQLCipher 版本: " + cipherVer);

            // 解密第一页
            int pageSize = 4096, reserveSize = 48;
            byte[] iv = new byte[16];
            Array.Copy(encDb, pageSize - reserveSize, iv, 0, 16);
            byte[] encrypted = new byte[pageSize - 16 - reserveSize];
            Array.Copy(encDb, 16, encrypted, 0, encrypted.Length);

            byte[] decrypted;
            using (var aes = Aes.Create())
            {
                aes.Mode = CipherMode.CBC;
                aes.Padding = PaddingMode.None;
                aes.Key = encKey;
                aes.IV = iv;
                using (var dec = aes.CreateDecryptor())
                    decrypted = dec.TransformFinalBlock(encrypted, 0, encrypted.Length);
            }

            // 构造第一页
            byte[] page1 = new byte[pageSize];
            byte[] sqlHdr = Encoding.ASCII.GetBytes("SQLite format 3");
            Array.Copy(sqlHdr, 0, page1, 0, 15);
            page1[15] = 0;
            Array.Copy(decrypted, 0, page1, 16, decrypted.Length);
            page1[16] = (byte)((pageSize >> 8) & 0xFF);
            page1[17] = (byte)(pageSize & 0xFF);

            string hdrCheck = Encoding.ASCII.GetString(page1, 0, 15);
            Console.WriteLine("  解密后头部: \"" + hdrCheck + "\"");
            Console.WriteLine("  [成功] 第一页解密成功!");

            // Step 5: 解析 sqlite_master
            Console.WriteLine("\n[步骤5] 解析 sqlite_master (数据库名: " + Path.GetFileName(firstDbPath) + ")...");
            // 整体解密
            int totalPages = encDb.Length / pageSize;
            byte[] output = new byte[totalPages * pageSize];
            Array.Copy(page1, 0, output, 0, pageSize);

            for (int pg = 1; pg < totalPages; pg++)
            {
                int pgOff = pg * pageSize;
                byte[] pIv = new byte[16];
                Array.Copy(encDb, pgOff + pageSize - reserveSize, pIv, 0, 16);
                byte[] pEnc = new byte[pageSize - reserveSize];
                Array.Copy(encDb, pgOff, pEnc, 0, pEnc.Length);

                using (var aes = Aes.Create())
                {
                    aes.Mode = CipherMode.CBC;
                    aes.Padding = PaddingMode.None;
                    aes.Key = encKey;
                    aes.IV = pIv;
                    using (var d = aes.CreateDecryptor())
                    {
                        byte[] pd = d.TransformFinalBlock(pEnc, 0, pEnc.Length);
                        Array.Copy(pd, 0, output, pgOff, pd.Length);
                    }
                }
            }
            output[20] = (byte)reserveSize;
            Console.WriteLine("  整体解密完成: " + totalPages + " 页");

            // 解析 sqlite_master
            ParseSqliteMaster(output, pageSize);

            // 如果是 ChatMsg.db，尝试解析消息
            if (Path.GetFileName(firstDbPath).Equals("ChatMsg.db", StringComparison.OrdinalIgnoreCase))
            {
                Console.WriteLine("\n[步骤6] 解析聊天消息...");
                ParseMessages(output, pageSize, reserveSize);
            }
            else
            {
                // 也试试解密 ChatMsg.db
                Console.WriteLine("\n[步骤6] 尝试定位并解密 ChatMsg.db...");
                foreach (var info in dataDirs)
                {
                    string msgDir = Path.Combine(info[0], "Msg");
                    if (!Directory.Exists(msgDir)) continue;
                    foreach (string chatDb in Directory.GetFiles(msgDir, "ChatMsg.db", SearchOption.AllDirectories))
                    {
                        Console.WriteLine("  找到 ChatMsg.db: " + chatDb);
                        try
                        {
                            string tmp2 = Path.Combine(Path.GetTempPath(), "wx_chat_" + Guid.NewGuid().ToString("N").Substring(0, 6));
                            File.Copy(chatDb, tmp2, true);
                            byte[] chatEnc = File.ReadAllBytes(tmp2);
                            byte[] chatDec = DecryptFullDb(extractedKey, chatEnc, cipherVer == "3");
                            if (chatDec != null)
                            {
                                Console.WriteLine("  [成功] ChatMsg.db 解密成功! 大小=" + FormatSize(chatDec.Length));
                                ParseSqliteMaster(chatDec, pageSize);
                                ParseMessages(chatDec, pageSize, reserveSize);
                            }
                            else Console.WriteLine("  [失败] ChatMsg.db 解密失败");
                            File.Delete(tmp2);
                        }
                        catch (Exception ex) { Console.WriteLine("  ChatMsg.db 异常: " + ex.Message); }
                        break;
                    }
                    break;
                }
            }

            File.Delete(tmpFile);
        }
        catch (Exception ex) { Console.WriteLine("  解密异常: " + ex.ToString()); }
    }

    static void TestQQ()
    {
        // Step 1: 查找 QQ 数据目录
        Console.WriteLine("\n[步骤1] 查找 QQ/NTQQ 数据目录...");
        var qqDirs = new List<string[]>(); // [dir, id, type]

        string docsPath = Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments);
        string tencentFiles = Path.Combine(docsPath, "Tencent Files");
        Console.WriteLine("  Tencent Files: " + tencentFiles + " 存在=" + Directory.Exists(tencentFiles));

        if (Directory.Exists(tencentFiles))
        {
            foreach (string dir in Directory.GetDirectories(tencentFiles))
            {
                string name = Path.GetFileName(dir);
                Console.WriteLine("  目录: " + name);
                if (name.Length >= 5 && name.Length <= 12 && IsNumeric(name))
                    qqDirs.Add(new string[] { dir, name, "classic" });
                string ntSub = Path.Combine(dir, "nt_qq");
                if (Directory.Exists(ntSub))
                {
                    qqDirs.Add(new string[] { ntSub, name, "ntqq-tf" });
                    Console.WriteLine("    存在 nt_qq 子目录");
                }
            }
        }

        string[] extraPaths = {
            Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Tencent", "QQ"),
            Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Tencent", "QQNT"),
            Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "QQ"),
        };
        foreach (string ep in extraPaths)
        {
            Console.WriteLine("  检查: " + ep + " 存在=" + Directory.Exists(ep));
            if (!Directory.Exists(ep)) continue;
            foreach (string dir in Directory.GetDirectories(ep))
            {
                string name = Path.GetFileName(dir);
                qqDirs.Add(new string[] { dir, name, "ntqq" });
                Console.WriteLine("    子目录: " + name);
            }
        }

        Console.WriteLine("  共找到 " + qqDirs.Count + " 个 QQ 目录");
        if (qqDirs.Count == 0) { Console.WriteLine("  [失败] 没有找到QQ数据目录"); return; }

        // Step 2: 查找 passphrase 文件
        Console.WriteLine("\n[步骤2] 查找 passphrase 文件...");
        foreach (var info in qqDirs)
        {
            string dataDir = info[0];
            Console.WriteLine("  目录: " + dataDir + " (type=" + info[2] + ")");

            // 列出所有文件和子目录
            try
            {
                foreach (string subDir in Directory.GetDirectories(dataDir))
                    Console.WriteLine("    子目录: " + Path.GetFileName(subDir));
            }
            catch (Exception ex) { Console.WriteLine("    列目录异常: " + ex.Message); }

            string[] keyPaths = {
                Path.Combine(dataDir, "nt_db", "passphrase"),
                Path.Combine(dataDir, "databases", "passphrase"),
                Path.Combine(dataDir, "passphrase"),
            };
            foreach (string kp in keyPaths)
            {
                bool exists = File.Exists(kp);
                Console.WriteLine("    passphrase: " + kp + " 存在=" + exists);
                if (exists)
                {
                    byte[] raw = File.ReadAllBytes(kp);
                    Console.WriteLine("    大小: " + raw.Length + " 字节, 前16字节: " + BitConverter.ToString(raw, 0, Math.Min(16, raw.Length)));

                    // 尝试 DPAPI 解密
                    Console.WriteLine("    尝试 DPAPI 用户级解密...");
                    byte[] dec = DPAPIDecrypt(raw, false);
                    if (dec != null)
                    {
                        Console.WriteLine("    [成功] DPAPI 解密成功! 长度=" + dec.Length);
                        Console.WriteLine("    解密结果(hex): " + BytesToHex(dec));
                        string decStr = Encoding.UTF8.GetString(dec).Trim();
                        Console.WriteLine("    解密结果(utf8): " + (decStr.Length > 100 ? decStr.Substring(0, 100) + "..." : decStr));
                        if (decStr.Length == 64 && IsHexStr(decStr))
                            Console.WriteLine("    看起来是 64 字符 hex 字符串 → 32 字节密钥");
                    }
                    else
                    {
                        Console.WriteLine("    DPAPI 用户级失败");
                        Console.WriteLine("    尝试 DPAPI 机器级解密...");
                        dec = DPAPIDecrypt(raw, true);
                        if (dec != null)
                        {
                            Console.WriteLine("    [成功] DPAPI 机器级解密成功! 长度=" + dec.Length);
                            Console.WriteLine("    解密结果(hex): " + BytesToHex(dec));
                        }
                        else Console.WriteLine("    DPAPI 机器级也失败");
                    }
                }
            }

            // 搜索所有可能的密钥文件
            Console.WriteLine("    搜索其他可能的密钥文件...");
            try
            {
                foreach (string f in Directory.GetFiles(dataDir, "*passphrase*", SearchOption.AllDirectories))
                    Console.WriteLine("      找到: " + f + " (" + new FileInfo(f).Length + " bytes)");
                foreach (string f in Directory.GetFiles(dataDir, "*key*", SearchOption.AllDirectories))
                {
                    long sz = new FileInfo(f).Length;
                    if (sz > 0 && sz < 1024)
                        Console.WriteLine("      找到: " + f + " (" + sz + " bytes)");
                }
            }
            catch (Exception ex) { Console.WriteLine("      搜索异常: " + ex.Message); }

            // Step 3: 列出数据库文件
            Console.WriteLine("\n[步骤3] 列出数据库文件...");
            try
            {
                int dbCount = 0;
                foreach (string f in Directory.GetFiles(dataDir, "*.db", SearchOption.AllDirectories))
                {
                    long sz = new FileInfo(f).Length;
                    if (sz <= 0) continue;
                    string rel = f.Substring(dataDir.Length + 1);
                    if (rel.Split(Path.DirectorySeparatorChar).Length > 5) continue;

                    byte[] header = new byte[16];
                    try
                    {
                        using (var fs = new FileStream(f, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                            fs.Read(header, 0, Math.Min(16, (int)sz));
                    }
                    catch { }
                    bool isEncrypted = Encoding.ASCII.GetString(header, 0, Math.Min(6, header.Length)) != "SQLite";
                    Console.WriteLine("    " + rel + " (" + FormatSize(sz) + ") 加密=" + isEncrypted);
                    dbCount++;
                    if (dbCount > 20) { Console.WriteLine("    ... 更多文件已省略"); break; }
                }
            }
            catch (Exception ex) { Console.WriteLine("    列出DB异常: " + ex.Message); }
        }

        // Step 4: 检查 QQ 进程
        Console.WriteLine("\n[步骤4] 检查 QQ 进程...");
        try
        {
            var qqProcs = Process.GetProcessesByName("QQ");
            Console.WriteLine("  QQ.exe 进程数: " + qqProcs.Length);
            foreach (var proc in qqProcs)
            {
                Console.WriteLine("  PID=" + proc.Id);
                try
                {
                    string ver = proc.MainModule.FileVersionInfo.FileVersion;
                    string path = proc.MainModule.FileName;
                    Console.WriteLine("  版本: " + ver);
                    Console.WriteLine("  路径: " + path);
                }
                catch (Exception ex) { Console.WriteLine("  读取版本异常: " + ex.Message); }

                // 列出关键模块
                try
                {
                    foreach (ProcessModule mod in proc.Modules)
                    {
                        string mn = mod.ModuleName.ToLowerInvariant();
                        if (mn == "wrapper.node" || mn == "qqnt.node" || mn == "sqlite3.dll" || mn.Contains("ntqq"))
                            Console.WriteLine("  关键模块: " + mod.ModuleName + " base=0x" + mod.BaseAddress.ToString("X") + " size=" + FormatSize(mod.ModuleMemorySize));
                    }
                }
                catch (Exception ex) { Console.WriteLine("  列模块异常: " + ex.Message); }
            }
        }
        catch (Exception ex) { Console.WriteLine("  进程检查异常: " + ex.Message); }
    }

    // ═══ 辅助方法 ═══

    static void ParseSqliteMaster(byte[] dbData, int pageSize)
    {
        Console.WriteLine("  sqlite_master 内容:");
        int pg0Hdr = 100;
        if (pg0Hdr >= dbData.Length || dbData[pg0Hdr] != 0x0D)
        {
            Console.WriteLine("    页面类型: 0x" + dbData[pg0Hdr].ToString("X2") + " (期望 0x0D)");
            return;
        }

        int cellCount = (dbData[pg0Hdr + 3] << 8) | dbData[pg0Hdr + 4];
        Console.WriteLine("    条目数: " + cellCount);
        int ptrStart = pg0Hdr + 8;

        for (int c = 0; c < cellCount && c < 50; c++)
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
                string objType = null, objName = null, sql = null;
                long rootPage = 0;

                for (int col = 0; col < colTypes.Count && dp < dbData.Length; col++)
                {
                    long st = colTypes[col];
                    int colLen = SqliteColSize(st);
                    if (dp + colLen > dbData.Length) break;

                    if ((col == 0 || col == 1 || col == 4) && st >= 13 && st % 2 == 1)
                    {
                        int tl = (int)(st - 13) / 2;
                        if (tl > 0 && dp + tl <= dbData.Length)
                        {
                            string val = Encoding.UTF8.GetString(dbData, dp, tl);
                            if (col == 0) objType = val;
                            else if (col == 1) objName = val;
                            else if (col == 4) sql = val;
                        }
                    }
                    else if (col == 3) rootPage = ReadSqliteInt(dbData, dp, colLen);

                    dp += colLen;
                }

                string sqlPreview = sql != null && sql.Length > 120 ? sql.Substring(0, 120) + "..." : sql;
                Console.WriteLine("    " + (objType ?? "?") + " " + (objName ?? "?") + " rootpage=" + rootPage + " SQL=" + sqlPreview);
            }
            catch { }
        }
    }

    static void ParseMessages(byte[] dbData, int pageSize, int reserve)
    {
        // 从 sqlite_master 找 MSG 表
        int msgRootPage = -1;
        int talkerIdx = -1, contentIdx = -1, typeIdx = -1, senderIdx = -1, timeIdx = -1;

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
                    long pLen; ReadVarint(dbData, p, out pLen, out n); p += n;
                    long rid; ReadVarint(dbData, p, out rid, out n); p += n;
                    long rhs; int hb; ReadVarint(dbData, p, out rhs, out hb);
                    int rhe = p + (int)rhs;
                    int hp = p + hb;
                    var ct = new List<long>();
                    while (hp < rhe && hp < dbData.Length) { long st; ReadVarint(dbData, hp, out st, out n); hp += n; ct.Add(st); }
                    if (ct.Count < 5) continue;

                    int dp = rhe;
                    string name = null, sql = null; long rp = 0;
                    for (int col = 0; col < ct.Count && dp < dbData.Length; col++)
                    {
                        long st = ct[col]; int cl = SqliteColSize(st);
                        if (dp + cl > dbData.Length) break;
                        if (col == 1 && st >= 13 && st % 2 == 1) { int tl = (int)(st - 13) / 2; if (tl > 0 && dp + tl <= dbData.Length) name = Encoding.UTF8.GetString(dbData, dp, tl); }
                        else if (col == 3) rp = ReadSqliteInt(dbData, dp, cl);
                        else if (col == 4 && st >= 13 && st % 2 == 1) { int tl = (int)(st - 13) / 2; if (tl > 0 && dp + tl <= dbData.Length) sql = Encoding.UTF8.GetString(dbData, dp, tl); }
                        dp += cl;
                    }

                    if (name != null && rp > 0 && (name.Equals("MSG", StringComparison.OrdinalIgnoreCase) || name.Equals("message", StringComparison.OrdinalIgnoreCase)))
                    {
                        msgRootPage = (int)rp;
                        Console.WriteLine("  找到 MSG 表: rootpage=" + rp);
                        if (sql != null)
                        {
                            Console.WriteLine("  SQL: " + sql);
                            int paren = sql.IndexOf('(');
                            if (paren > 0)
                            {
                                string colDefs = sql.Substring(paren + 1).TrimEnd(')', ' ');
                                string[] cols = colDefs.Split(',');
                                Console.WriteLine("  列数: " + cols.Length);
                                for (int ci = 0; ci < cols.Length; ci++)
                                {
                                    string cn = cols[ci].Trim().Split(' ')[0].Trim('"', '`', '[', ']');
                                    Console.WriteLine("    col[" + ci + "] = " + cn);
                                    string cu = cn.ToUpperInvariant();
                                    if (cu == "STRTALKER") talkerIdx = ci;
                                    else if (cu == "STRCONTENT") contentIdx = ci;
                                    else if (cu == "TYPE") typeIdx = ci;
                                    else if (cu == "ISSENDER") senderIdx = ci;
                                    else if (cu == "CREATETIME") timeIdx = ci;
                                }
                            }
                        }
                        break;
                    }
                }
                catch { }
            }
        }

        if (talkerIdx < 0) talkerIdx = 13;
        if (contentIdx < 0) contentIdx = 14;
        if (typeIdx < 0) typeIdx = 3;
        if (senderIdx < 0) senderIdx = 5;
        if (timeIdx < 0) timeIdx = 6;

        Console.WriteLine("  列索引: Type=" + typeIdx + " IsSender=" + senderIdx + " CreateTime=" + timeIdx + " StrTalker=" + talkerIdx + " StrContent=" + contentIdx);

        int totalPages = dbData.Length / pageSize;
        int msgCount = 0;
        int minCols = Math.Max(Math.Max(Math.Max(talkerIdx, contentIdx), Math.Max(typeIdx, senderIdx)), timeIdx) + 1;

        // 遍历所有叶子页找消息
        for (int pg = 0; pg < totalPages && msgCount < 20; pg++)
        {
            int off = pg * pageSize;
            int hdr = off + (pg == 0 ? 100 : 0);
            if (hdr >= dbData.Length) continue;
            if (dbData[hdr] != 0x0D) continue;

            int cellCount = (dbData[hdr + 3] << 8) | dbData[hdr + 4];
            int ptrStart = hdr + 8;

            for (int c = 0; c < cellCount && c < 500 && msgCount < 20; c++)
            {
                int ptrOff = ptrStart + c * 2;
                if (ptrOff + 2 > dbData.Length) break;
                int cellOff = off + ((dbData[ptrOff] << 8) | dbData[ptrOff + 1]);
                if (cellOff >= dbData.Length || cellOff < off) continue;

                try
                {
                    int p = cellOff;
                    int n;
                    long payloadLen; ReadVarint(dbData, p, out payloadLen, out n); p += n;
                    long rowid; ReadVarint(dbData, p, out rowid, out n); p += n;
                    if (payloadLen <= 0 || payloadLen > pageSize - reserve) continue;

                    long recHdrSize; int hb;
                    ReadVarint(dbData, p, out recHdrSize, out hb);
                    int recHdrEnd = p + (int)recHdrSize;
                    int hp = p + hb;

                    var colTypes = new List<long>();
                    while (hp < recHdrEnd && hp < dbData.Length) { long st; ReadVarint(dbData, hp, out st, out n); hp += n; colTypes.Add(st); }
                    if (colTypes.Count < minCols) continue;
                    if (talkerIdx < colTypes.Count && (colTypes[talkerIdx] < 13 || colTypes[talkerIdx] % 2 != 1)) continue;

                    int dp = recHdrEnd;
                    long msgType = 0, isSender = 0, createTime = 0;
                    string strTalker = "", strContent = "";

                    for (int col = 0; col < colTypes.Count && dp < dbData.Length; col++)
                    {
                        long st = colTypes[col]; int colLen = SqliteColSize(st);
                        if (dp + colLen > dbData.Length) break;
                        if (col == typeIdx) msgType = ReadSqliteInt(dbData, dp, colLen);
                        else if (col == senderIdx) isSender = ReadSqliteInt(dbData, dp, colLen);
                        else if (col == timeIdx) createTime = ReadSqliteInt(dbData, dp, colLen);
                        else if (col == talkerIdx && st >= 13 && st % 2 == 1) { int tl = (int)(st - 13) / 2; if (tl > 0 && dp + tl <= dbData.Length) strTalker = Encoding.UTF8.GetString(dbData, dp, tl); }
                        else if (col == contentIdx && st >= 13 && st % 2 == 1) { int tl = (int)(st - 13) / 2; if (tl > 0 && dp + tl <= dbData.Length) strContent = Encoding.UTF8.GetString(dbData, dp, tl); }
                        dp += colLen;
                    }

                    if (!string.IsNullOrEmpty(strTalker) && createTime > 1000000000 && (msgType == 1 || msgType == 49 || msgType == 3))
                    {
                        string dir = isSender == 1 ? "发→" : "←收";
                        string preview = strContent.Length > 80 ? strContent.Substring(0, 80) + "..." : strContent;
                        Console.WriteLine("  [消息] " + dir + " " + strTalker + " type=" + msgType + " time=" + createTime + ": " + preview);
                        msgCount++;
                    }
                }
                catch { }
            }
        }
        Console.WriteLine("  显示了 " + msgCount + " 条消息 (最多20条)");
    }

    static byte[] DecryptFullDb(byte[] rawKey, byte[] encDb, bool isV3)
    {
        int pageSize = 4096, reserveSize = 48;
        if (encDb == null || encDb.Length < pageSize) return null;

        byte[] salt = new byte[16];
        Array.Copy(encDb, 0, salt, 0, 16);

        byte[] encKey, hmacKey;
        if (isV3)
        {
            using (var kdf = new Rfc2898DeriveBytes(rawKey, salt, 64000))
                encKey = kdf.GetBytes(32);
            byte[] hs = new byte[16];
            for (int i = 0; i < 16; i++) hs[i] = (byte)(salt[i] ^ 0x3a);
            using (var kdf = new Rfc2898DeriveBytes(encKey, hs, 2))
                hmacKey = kdf.GetBytes(32);
        }
        else
        {
            encKey = PBKDF2_SHA512(rawKey, salt, 256000, 32);
            byte[] hs = new byte[16];
            for (int i = 0; i < 16; i++) hs[i] = (byte)(salt[i] ^ 0x3a);
            hmacKey = PBKDF2_SHA512(encKey, hs, 2, 32);
        }

        int totalPages = encDb.Length / pageSize;
        byte[] output = new byte[totalPages * pageSize];

        for (int pg = 0; pg < totalPages; pg++)
        {
            int pgOff = pg * pageSize;
            int encStart = pg == 0 ? 16 : pgOff;
            int encLen = pg == 0 ? pageSize - 16 - reserveSize : pageSize - reserveSize;
            if (pg > 0) encStart = pgOff;
            if (encStart + encLen > encDb.Length) break;

            byte[] iv = new byte[16];
            Array.Copy(encDb, pgOff + pageSize - reserveSize, iv, 0, 16);
            byte[] encrypted = new byte[encLen];
            Array.Copy(encDb, encStart, encrypted, 0, encLen);

            using (var aes = Aes.Create())
            {
                aes.Mode = CipherMode.CBC;
                aes.Padding = PaddingMode.None;
                aes.Key = encKey;
                aes.IV = iv;
                using (var dec = aes.CreateDecryptor())
                {
                    byte[] decrypted = dec.TransformFinalBlock(encrypted, 0, encrypted.Length);
                    if (pg == 0)
                    {
                        byte[] hdr = Encoding.ASCII.GetBytes("SQLite format 3");
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
            }
        }

        if (Encoding.ASCII.GetString(output, 0, 15) != "SQLite format 3") return null;
        return output;
    }

    static bool ValidateKey(byte[] rawKey, byte[] dbHeader)
    {
        byte[] salt = new byte[16];
        Array.Copy(dbHeader, 0, salt, 0, 16);

        // SQLCipher 3
        byte[] encKey, hmacKey;
        using (var kdf = new Rfc2898DeriveBytes(rawKey, salt, 64000))
            encKey = kdf.GetBytes(32);
        byte[] hs = new byte[16];
        for (int i = 0; i < 16; i++) hs[i] = (byte)(salt[i] ^ 0x3a);
        using (var kdf = new Rfc2898DeriveBytes(encKey, hs, 2))
            hmacKey = kdf.GetBytes(32);

        if (VerifyPageHMAC(dbHeader, 0, encKey, hmacKey, 4096, 48, false)) return true;

        // SQLCipher 4
        encKey = PBKDF2_SHA512(rawKey, salt, 256000, 32);
        for (int i = 0; i < 16; i++) hs[i] = (byte)(salt[i] ^ 0x3a);
        hmacKey = PBKDF2_SHA512(encKey, hs, 2, 32);
        return VerifyPageHMAC(dbHeader, 0, encKey, hmacKey, 4096, 48, true);
    }

    static bool VerifyPageHMAC(byte[] db, int pgOff, byte[] encKey, byte[] hmacKey, int pageSize, int reserveSize, bool useSHA512)
    {
        int dataStart = (pgOff == 0) ? 16 : 0;
        int dataLen = pageSize - dataStart - reserveSize;
        if (pgOff + pageSize > db.Length) return false;

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

    static byte[] PBKDF2_SHA512(byte[] password, byte[] salt, int iterations, int dkLen)
    {
        int hLen = 64;
        int blocks = (dkLen + hLen - 1) / hLen;
        byte[] dk = new byte[dkLen];
        for (int block = 1; block <= blocks; block++)
        {
            byte[] blockSalt = new byte[salt.Length + 4];
            Array.Copy(salt, blockSalt, salt.Length);
            blockSalt[salt.Length] = (byte)((block >> 24) & 0xFF);
            blockSalt[salt.Length + 1] = (byte)((block >> 16) & 0xFF);
            blockSalt[salt.Length + 2] = (byte)((block >> 8) & 0xFF);
            blockSalt[salt.Length + 3] = (byte)(block & 0xFF);

            byte[] u;
            using (var hmac = new HMACSHA512(password)) u = hmac.ComputeHash(blockSalt);
            byte[] result = (byte[])u.Clone();
            for (int iter = 1; iter < iterations; iter++)
            {
                using (var hmac = new HMACSHA512(password)) u = hmac.ComputeHash(u);
                for (int j = 0; j < hLen; j++) result[j] ^= u[j];
            }
            int copyLen = Math.Min(hLen, dkLen - (block - 1) * hLen);
            Array.Copy(result, 0, dk, (block - 1) * hLen, copyLen);
        }
        return dk;
    }

    static byte[] DPAPIDecrypt(byte[] data, bool machineScope)
    {
        var dataIn = new DATA_BLOB();
        var dataOut = new DATA_BLOB();
        dataIn.cbData = data.Length;
        dataIn.pbData = Marshal.AllocHGlobal(data.Length);
        Marshal.Copy(data, 0, dataIn.pbData, data.Length);
        try
        {
            int flags = machineScope ? 0x04 : 0;
            if (!CryptUnprotectData(ref dataIn, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, flags, ref dataOut))
                return null;
            byte[] result = new byte[dataOut.cbData];
            Marshal.Copy(dataOut.pbData, result, 0, dataOut.cbData);
            LocalFree(dataOut.pbData);
            return result;
        }
        finally { Marshal.FreeHGlobal(dataIn.pbData); }
    }

    static int CountDistinct(byte[] data)
    {
        var seen = new HashSet<byte>();
        foreach (byte b in data) seen.Add(b);
        return seen.Count;
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
        if (st == 1) return 1; if (st == 2) return 2; if (st == 3) return 3;
        if (st == 4) return 4; if (st == 5) return 6;
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

    static bool IsNumeric(string s) { foreach (char c in s) if (c < '0' || c > '9') return false; return true; }
    static bool IsHexStr(string s) { foreach (char c in s) if (!((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F'))) return false; return true; }
    static string BytesToHex(byte[] b) { var sb = new StringBuilder(b.Length * 2); foreach (byte x in b) sb.Append(x.ToString("x2")); return sb.ToString(); }
    static string FormatSize(long b)
    {
        if (b < 1024) return b + " B";
        if (b < 1024 * 1024) return (b / 1024.0).ToString("F1") + " KB";
        return (b / (1024.0 * 1024.0)).ToString("F1") + " MB";
    }
}
