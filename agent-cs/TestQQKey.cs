using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;

/// 提取 NTQQ passphrase 并解密数据库
class TestQQKey
{
    [DllImport("kernel32.dll")] static extern IntPtr OpenProcess(int access, bool inherit, int pid);
    [DllImport("kernel32.dll")] static extern bool ReadProcessMemory(IntPtr hProc, IntPtr baseAddr, byte[] buf, int size, out int read);
    [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr h);
    [DllImport("kernel32.dll")] static extern bool VirtualQueryEx(IntPtr hProcess, IntPtr lpAddress, out MEMORY_BASIC_INFORMATION lpBuffer, uint dwLength);

    [StructLayout(LayoutKind.Sequential)]
    struct MEMORY_BASIC_INFORMATION
    {
        public IntPtr BaseAddress;
        public IntPtr AllocationBase;
        public uint AllocationProtect;
        public IntPtr RegionSize;
        public uint State;
        public uint Protect;
        public uint Type;
    }

    const uint MEM_COMMIT = 0x1000;
    const uint PAGE_READWRITE = 0x04;
    const uint PAGE_READONLY = 0x02;
    const uint PAGE_EXECUTE_READ = 0x20;
    const uint PAGE_EXECUTE_READWRITE = 0x40;

    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        Console.WriteLine("=== NTQQ Passphrase 提取 + 解密测试 ===\n");

        // 1. 准备数据库（跳过前1024字节头部）
        string dbPath = @"D:\QQ\Tencent Files\3371574658\nt_qq\nt_db\nt_msg.db";
        string tmp = Path.Combine(Path.GetTempPath(), "ntqq_clean.db");
        Console.WriteLine("[1] 准备数据库...");

        byte[] rawDb = null;
        try
        {
            using (var src = new FileStream(dbPath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
            {
                // 跳过 1024 字节自定义头部
                src.Seek(1024, SeekOrigin.Begin);
                rawDb = new byte[src.Length - 1024];
                src.Read(rawDb, 0, rawDb.Length);
            }
        }
        catch (Exception ex)
        {
            // 尝试复制
            Console.WriteLine("  直接读取失败, 尝试复制: " + ex.Message);
            File.Copy(dbPath, tmp, true);
            byte[] full = File.ReadAllBytes(tmp);
            rawDb = new byte[full.Length - 1024];
            Array.Copy(full, 1024, rawDb, 0, rawDb.Length);
            File.Delete(tmp);
        }

        Console.WriteLine("  清理后大小: " + rawDb.Length + " bytes (" + (rawDb.Length / 1024.0 / 1024).ToString("F1") + " MB)");

        // 检查清理后的头部
        Console.WriteLine("  前16字节: " + BitConverter.ToString(rawDb, 0, 16));
        int pageSize = (rawDb[16] << 8) | rawDb[17];
        if (pageSize == 1) pageSize = 65536;
        Console.WriteLine("  页面大小: " + pageSize);
        Console.WriteLine("  总页数: " + rawDb.Length / pageSize);
        Console.WriteLine("  Byte[100]=0x" + rawDb[100].ToString("X2") + " (expect 0x0D/0x05 after decryption)");

        // 提取 salt（清理后数据的前16字节）
        byte[] salt = new byte[16];
        Array.Copy(rawDb, 0, salt, 0, 16);
        Console.WriteLine("  Salt: " + BitConverter.ToString(salt));

        // 2. 扫描 QQ 进程内存找 passphrase
        Console.WriteLine("\n[2] 扫描 QQ 进程内存...");
        var candidates = new List<string>();

        try
        {
            var procs = Process.GetProcessesByName("QQ");
            Console.WriteLine("  QQ 进程数: " + procs.Length);

            foreach (var proc in procs)
            {
                // 找有 wrapper.node 的进程
                ProcessModule wrapperMod = null;
                try
                {
                    foreach (ProcessModule mod in proc.Modules)
                    {
                        if (mod.ModuleName.ToLowerInvariant() == "wrapper.node")
                        { wrapperMod = mod; break; }
                    }
                }
                catch { continue; }
                if (wrapperMod == null) continue;

                Console.WriteLine("  PID=" + proc.Id + " 有 wrapper.node");

                IntPtr hProc = OpenProcess(0x0010 | 0x0400 | 0x0008, false, proc.Id);
                if (hProc == IntPtr.Zero) { Console.WriteLine("  OpenProcess 失败"); continue; }

                try
                {
                    // 扫描所有可读内存区域
                    long addr = 0;
                    long maxAddr = 0x7FFFFFFFFFFF;
                    int regionsScanned = 0;
                    long bytesScanned = 0;

                    while (addr < maxAddr)
                    {
                        MEMORY_BASIC_INFORMATION mbi;
                        if (!VirtualQueryEx(hProc, new IntPtr(addr), out mbi, (uint)Marshal.SizeOf(typeof(MEMORY_BASIC_INFORMATION))))
                            break;

                        long regionSize = mbi.RegionSize.ToInt64();
                        if (regionSize <= 0) break;

                        // 只扫描已提交的可读内存
                        if (mbi.State == MEM_COMMIT &&
                            (mbi.Protect == PAGE_READWRITE || mbi.Protect == PAGE_READONLY ||
                             mbi.Protect == PAGE_EXECUTE_READ || mbi.Protect == PAGE_EXECUTE_READWRITE) &&
                            regionSize < 100 * 1024 * 1024) // 跳过超大区域
                        {
                            int chunkSize = (int)Math.Min(regionSize, 4 * 1024 * 1024);
                            byte[] chunk = new byte[chunkSize];
                            int read;

                            for (long offset = 0; offset < regionSize; offset += chunkSize)
                            {
                                int readSize = (int)Math.Min(chunkSize, regionSize - offset);
                                if (ReadProcessMemory(hProc, new IntPtr(addr + offset), chunk, readSize, out read) && read > 16)
                                {
                                    bytesScanned += read;
                                    // 搜索 16 字节 printable ASCII 字符串（后面跟 null）
                                    for (int i = 0; i <= read - 17; i++)
                                    {
                                        // 快速过滤：检查是否可能是 passphrase
                                        if (chunk[i + 16] != 0) continue; // 必须以 null 结尾
                                        if (chunk[i] < 0x21 || chunk[i] > 0x7E) continue; // 必须可打印

                                        bool valid = true;
                                        bool hasSpecial = false;
                                        bool hasAlpha = false;
                                        bool hasDigit = false;

                                        for (int j = 0; j < 16; j++)
                                        {
                                            byte b = chunk[i + j];
                                            if (b < 0x21 || b > 0x7E) { valid = false; break; }
                                            if (b >= '0' && b <= '9') hasDigit = true;
                                            else if ((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')) hasAlpha = true;
                                            else hasSpecial = true;
                                        }

                                        if (!valid || !hasAlpha) continue;
                                        // passphrase 通常包含混合字符
                                        if (!(hasSpecial || (hasDigit && hasAlpha))) continue;

                                        string candidate = Encoding.ASCII.GetString(chunk, i, 16);

                                        // 跳过明显不是 passphrase 的（如路径、URL、常见字符串）
                                        if (candidate.Contains("\\") || candidate.Contains("/") ||
                                            candidate.Contains("://") || candidate.Contains("    ") ||
                                            candidate.Contains("....") || candidate.Contains("===="))
                                            continue;

                                        if (!candidates.Contains(candidate))
                                            candidates.Add(candidate);
                                    }
                                }
                            }
                            regionsScanned++;
                        }

                        addr += regionSize;
                        if (addr < 0) break;
                    }

                    Console.WriteLine("  扫描完成: " + regionsScanned + " 个区域, " + (bytesScanned / 1024.0 / 1024).ToString("F0") + " MB");
                    Console.WriteLine("  候选 passphrase 数: " + candidates.Count);
                }
                finally { CloseHandle(hProc); }
                break; // 只扫描一个有 wrapper.node 的进程
            }
        }
        catch (Exception ex) { Console.WriteLine("  扫描异常: " + ex.Message); }

        if (candidates.Count == 0)
        {
            Console.WriteLine("  [失败] 没有找到候选 passphrase");
            return;
        }

        // 显示前20个候选
        Console.WriteLine("\n  前20个候选:");
        for (int i = 0; i < Math.Min(20, candidates.Count); i++)
            Console.WriteLine("    " + (i + 1) + ". \"" + candidates[i] + "\"");

        // 3. 逐个验证候选 passphrase
        Console.WriteLine("\n[3] 验证候选 passphrase (PBKDF2-SHA1, 4000 iter)...");
        string validPassphrase = null;

        for (int i = 0; i < candidates.Count; i++)
        {
            string pass = candidates[i];
            byte[] passBytes = Encoding.UTF8.GetBytes(pass);

            // PBKDF2-SHA1, 4000 iterations
            byte[] encKey;
            using (var kdf = new Rfc2898DeriveBytes(passBytes, salt, 4000))
                encKey = kdf.GetBytes(32);

            // Derive HMAC key
            byte[] hmacSalt = new byte[16];
            for (int j = 0; j < 16; j++) hmacSalt[j] = (byte)(salt[j] ^ 0x3a);
            byte[] hmacKey;
            using (var kdf = new Rfc2898DeriveBytes(encKey, hmacSalt, 2))
                hmacKey = kdf.GetBytes(32);

            // Verify HMAC of first page (page size = 4096, reserve = 48)
            int ps = 4096;
            int reserve = 48;
            int dataStart = 16; // first page starts after salt
            int dataLen = ps - dataStart - reserve;

            byte[] hmacInput = new byte[dataLen + 16 + 4];
            Array.Copy(rawDb, dataStart, hmacInput, 0, dataLen);
            Array.Copy(rawDb, ps - reserve, hmacInput, dataLen, 16); // IV
            hmacInput[dataLen + 16] = 0;
            hmacInput[dataLen + 17] = 0;
            hmacInput[dataLen + 18] = 0;
            hmacInput[dataLen + 19] = 1; // page number

            byte[] computed;
            using (var hmac = new HMACSHA1(hmacKey))
                computed = hmac.ComputeHash(hmacInput);

            // Compare with stored HMAC
            bool match = true;
            for (int k = 0; k < 20; k++)
            {
                if (rawDb[ps - reserve + 16 + k] != computed[k])
                { match = false; break; }
            }

            if (match)
            {
                validPassphrase = pass;
                Console.WriteLine("  [成功!] 候选 #" + (i + 1) + ": \"" + pass + "\"");
                break;
            }

            if (i < 5 || i % 100 == 0)
                Console.WriteLine("  候选 #" + (i + 1) + " \"" + pass + "\" - 不匹配");
        }

        if (validPassphrase == null)
        {
            Console.WriteLine("  [失败] 没有找到有效 passphrase (已测试 " + candidates.Count + " 个)");

            // 也试试 SHA512 (新版本可能用)
            Console.WriteLine("\n[3b] 尝试 HMAC_SHA512 验证...");
            for (int i = 0; i < candidates.Count; i++)
            {
                string pass = candidates[i];
                byte[] passBytes = Encoding.UTF8.GetBytes(pass);
                byte[] encKey = PBKDF2_SHA512(passBytes, salt, 4000, 32);
                byte[] hmacSalt = new byte[16];
                for (int j = 0; j < 16; j++) hmacSalt[j] = (byte)(salt[j] ^ 0x3a);
                byte[] hmacKey = PBKDF2_SHA512(encKey, hmacSalt, 2, 32);

                int ps = 4096, reserve = 48;
                int dataLen = ps - 16 - reserve;
                byte[] hmacInput = new byte[dataLen + 16 + 4];
                Array.Copy(rawDb, 16, hmacInput, 0, dataLen);
                Array.Copy(rawDb, ps - reserve, hmacInput, dataLen, 16);
                hmacInput[dataLen + 16] = 0; hmacInput[dataLen + 17] = 0;
                hmacInput[dataLen + 18] = 0; hmacInput[dataLen + 19] = 1;

                byte[] computed;
                using (var hmac = new HMACSHA512(hmacKey))
                    computed = hmac.ComputeHash(hmacInput);

                bool match = true;
                int cmpLen = Math.Min(reserve - 16, computed.Length);
                for (int k = 0; k < cmpLen; k++)
                    if (rawDb[ps - reserve + 16 + k] != computed[k]) { match = false; break; }

                if (match)
                {
                    validPassphrase = pass;
                    Console.WriteLine("  [成功! SHA512] 候选 #" + (i + 1) + ": \"" + pass + "\"");
                    break;
                }
            }
        }

        if (validPassphrase == null) { Console.WriteLine("  所有验证失败"); return; }

        // 4. 解密数据库
        Console.WriteLine("\n[4] 解密数据库...");
        byte[] passKey = Encoding.UTF8.GetBytes(validPassphrase);
        byte[] decDb = DecryptNTQQ(passKey, rawDb, 4096, 48);
        if (decDb == null) { Console.WriteLine("  解密失败"); return; }

        Console.WriteLine("  [成功] 解密完成! 大小=" + decDb.Length);

        // 5. 解析 sqlite_master
        Console.WriteLine("\n[5] 解析 sqlite_master:");
        ParseSqliteMaster(decDb);

        Console.WriteLine("\n=== 测试完毕 ===");
    }

    static byte[] DecryptNTQQ(byte[] passphrase, byte[] encDb, int pageSize, int reserve)
    {
        byte[] salt = new byte[16];
        Array.Copy(encDb, 0, salt, 0, 16);

        byte[] encKey;
        using (var kdf = new Rfc2898DeriveBytes(passphrase, salt, 4000))
            encKey = kdf.GetBytes(32);

        byte[] hmacSalt = new byte[16];
        for (int i = 0; i < 16; i++) hmacSalt[i] = (byte)(salt[i] ^ 0x3a);
        byte[] hmacKey;
        using (var kdf = new Rfc2898DeriveBytes(encKey, hmacSalt, 2))
            hmacKey = kdf.GetBytes(32);

        int totalPages = encDb.Length / pageSize;
        byte[] output = new byte[totalPages * pageSize];

        for (int pg = 0; pg < totalPages; pg++)
        {
            int pgOff = pg * pageSize;
            int encStart = pg == 0 ? pgOff + 16 : pgOff;
            int encLen = pg == 0 ? pageSize - 16 - reserve : pageSize - reserve;
            if (encStart + encLen > encDb.Length) break;

            byte[] iv = new byte[16];
            Array.Copy(encDb, pgOff + pageSize - reserve, iv, 0, 16);
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
                    byte[] d = dec.TransformFinalBlock(encrypted, 0, encrypted.Length);
                    if (pg == 0)
                    {
                        byte[] hdr = Encoding.ASCII.GetBytes("SQLite format 3");
                        Array.Copy(hdr, 0, output, 0, 15);
                        output[15] = 0;
                        Array.Copy(d, 0, output, 16, d.Length);
                        output[16] = (byte)((pageSize >> 8) & 0xFF);
                        output[17] = (byte)(pageSize & 0xFF);
                        output[20] = (byte)reserve;
                    }
                    else Array.Copy(d, 0, output, pgOff, d.Length);
                }
            }
        }

        if (Encoding.ASCII.GetString(output, 0, 15) != "SQLite format 3") return null;
        // Check page type
        if (output[100] != 0x0D && output[100] != 0x05) return null;
        return output;
    }

    static void ParseSqliteMaster(byte[] data)
    {
        int pageSize = (data[16] << 8) | data[17];
        if (pageSize == 1) pageSize = 65536;
        int hdr = 100;
        if (data[hdr] == 0x05)
        {
            Console.WriteLine("  sqlite_master 是 interior page, 遍历子页...");
            int cellCount = (data[hdr + 3] << 8) | data[hdr + 4];
            long rightChild = ((long)data[hdr + 8] << 24) | ((long)data[hdr + 9] << 16) | ((long)data[hdr + 10] << 8) | data[hdr + 11];
            var childPages = new List<int>();
            int ptrStart = hdr + 12;
            for (int c = 0; c < cellCount; c++)
            {
                int ptrOff = ptrStart + c * 2;
                if (ptrOff + 2 > data.Length) break;
                int cellOff = (data[ptrOff] << 8) | data[ptrOff + 1];
                if (cellOff + 4 > data.Length) continue;
                long cp = ((long)data[cellOff] << 24) | ((long)data[cellOff + 1] << 16) | ((long)data[cellOff + 2] << 8) | data[cellOff + 3];
                childPages.Add((int)cp);
            }
            childPages.Add((int)rightChild);
            foreach (int cp in childPages)
            {
                if (cp < 1 || (cp - 1) * pageSize >= data.Length) continue;
                int pgOff = (cp - 1) * pageSize;
                if (data[pgOff] == 0x0D)
                    ParseMasterLeaf(data, pgOff, pageSize);
            }
        }
        else if (data[hdr] == 0x0D)
            ParseMasterLeaf(data, 0, pageSize);
        else
            Console.WriteLine("  sqlite_master 页面类型: 0x" + data[hdr].ToString("X2"));
    }

    static void ParseMasterLeaf(byte[] data, int pageOff, int pageSize)
    {
        int hdr = pageOff + (pageOff == 0 ? 100 : 0);
        int cellCount = (data[hdr + 3] << 8) | data[hdr + 4];
        int ptrStart = hdr + 8;
        for (int c = 0; c < cellCount && c < 100; c++)
        {
            int ptrOff = ptrStart + c * 2;
            if (ptrOff + 2 > data.Length) break;
            int cellOff = pageOff + ((data[ptrOff] << 8) | data[ptrOff + 1]);
            if (cellOff >= data.Length || cellOff < pageOff) continue;
            try
            {
                int p = cellOff; int n;
                long pLen; ReadVarint(data, p, out pLen, out n); p += n;
                long rid; ReadVarint(data, p, out rid, out n); p += n;
                long rhs; int hb; ReadVarint(data, p, out rhs, out hb);
                int rhe = p + (int)rhs; int hp = p + hb;
                var ct = new List<long>();
                while (hp < rhe && hp < data.Length) { long st; ReadVarint(data, hp, out st, out n); hp += n; ct.Add(st); }
                if (ct.Count < 5) continue;
                int dp = rhe; string type = null, name = null, sql = null; long rp = 0;
                for (int col = 0; col < ct.Count && dp < data.Length; col++)
                {
                    long st = ct[col]; int cl = ColSize(st); if (dp + cl > data.Length) break;
                    if ((col == 0 || col == 1 || col == 4) && st >= 13 && st % 2 == 1)
                    {
                        int tl = (int)(st - 13) / 2;
                        if (tl > 0 && dp + tl <= data.Length) { string v = Encoding.UTF8.GetString(data, dp, tl); if (col == 0) type = v; else if (col == 1) name = v; else sql = v; }
                    }
                    else if (col == 3) rp = ReadInt(data, dp, cl);
                    dp += cl;
                }
                string sqlP = sql != null && sql.Length > 200 ? sql.Substring(0, 200) + "..." : sql;
                Console.WriteLine("  " + (type ?? "?") + ": " + (name ?? "?") + " rp=" + rp + " sql=" + sqlP);
            }
            catch { }
        }
    }

    static byte[] PBKDF2_SHA512(byte[] password, byte[] salt, int iterations, int dkLen)
    {
        byte[] dk = new byte[dkLen];
        byte[] blockSalt = new byte[salt.Length + 4];
        Array.Copy(salt, blockSalt, salt.Length);
        blockSalt[salt.Length + 3] = 1;
        byte[] u;
        using (var hmac = new HMACSHA512(password)) u = hmac.ComputeHash(blockSalt);
        byte[] result = (byte[])u.Clone();
        for (int iter = 1; iter < iterations; iter++)
        {
            using (var hmac = new HMACSHA512(password)) u = hmac.ComputeHash(u);
            for (int j = 0; j < 64; j++) result[j] ^= u[j];
        }
        Array.Copy(result, 0, dk, 0, dkLen);
        return dk;
    }

    static void ReadVarint(byte[] d, int p, out long v, out int n) { v = 0; n = 0; for (int i = 0; i < 9 && p + i < d.Length; i++) { v = (v << 7) | (long)(d[p + i] & 0x7F); n = i + 1; if ((d[p + i] & 0x80) == 0) return; } }
    static int ColSize(long st) { if (st == 0 || st == 8 || st == 9) return 0; if (st == 1) return 1; if (st == 2) return 2; if (st == 3) return 3; if (st == 4) return 4; if (st == 5) return 6; if (st == 6 || st == 7) return 8; if (st >= 12 && st % 2 == 0) return (int)(st - 12) / 2; if (st >= 13 && st % 2 == 1) return (int)(st - 13) / 2; return 0; }
    static long ReadInt(byte[] d, int o, int l) { long v = 0; for (int i = 0; i < l && o + i < d.Length; i++) v = (v << 8) | d[o + i]; return v; }
}
