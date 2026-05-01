using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;

/// WeChat 4.x (xwechat/Weixin) 密钥提取 + 解密测试
/// 原理: WCDB 在进程内存中缓存 raw key, 格式为 x'<64hex_enc_key><32hex_salt>'
/// SQLCipher 4: AES-256-CBC + HMAC-SHA512, page_size=4096, reserve=80
class TestWxKey
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

    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        Console.WriteLine("=== WeChat 4.x 密钥提取 + 解密 ===\n");

        // 1. 准备: 找到一个加密的数据库文件
        string dataRoot = @"D:\weixinliaotian\xwechat_files";
        Console.WriteLine("[1] 搜索加密数据库...");
        
        var dbFiles = new List<string>();
        var dbSalts = new Dictionary<string, byte[]>();

        try
        {
            foreach (string f in Directory.GetFiles(dataRoot, "*.db", SearchOption.AllDirectories))
            {
                try
                {
                    long sz = new FileInfo(f).Length;
                    if (sz < 4096) continue;
                    byte[] hdr = new byte[4096];
                    using (var fs = new FileStream(f, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                        fs.Read(hdr, 0, 4096);
                    if (Encoding.ASCII.GetString(hdr, 0, 6) == "SQLite") continue; // 明文，跳过
                    
                    dbFiles.Add(f);
                    byte[] salt = new byte[16];
                    Array.Copy(hdr, 0, salt, 0, 16);
                    dbSalts[f] = salt;
                    
                    if (dbFiles.Count <= 5)
                    {
                        string rel = f.Replace(dataRoot + "\\", "");
                        Console.WriteLine("  " + rel + " (" + (sz / 1024) + "KB) salt=" + BitConverter.ToString(salt, 0, 8) + "...");
                    }
                }
                catch { }
            }
        }
        catch (Exception ex) { Console.WriteLine("  搜索异常: " + ex.Message); }
        Console.WriteLine("  共 " + dbFiles.Count + " 个加密数据库");

        if (dbFiles.Count == 0)
        {
            Console.WriteLine("  [失败] 没有找到加密数据库");
            return;
        }

        // 2. 扫描 Weixin 进程内存寻找 x'<64hex><32hex>' 模式
        Console.WriteLine("\n[2] 扫描 Weixin 进程内存...");
        // 模式: x' 后跟 96 个 hex 字符 再跟 '
        // hex chars: 0-9, a-f, A-F
        
        var rawKeys = new List<KeyValuePair<byte[], byte[]>>(); // enc_key(32bytes), salt(16bytes)
        
        try
        {
            // Weixin 进程
            var procs = new List<Process>();
            procs.AddRange(Process.GetProcessesByName("Weixin"));
            procs.AddRange(Process.GetProcessesByName("WeChatAppEx"));
            
            Console.WriteLine("  Weixin 进程: " + Process.GetProcessesByName("Weixin").Length);
            Console.WriteLine("  WeChatAppEx 进程: " + Process.GetProcessesByName("WeChatAppEx").Length);

            // 先扫描主 Weixin 进程
            foreach (var proc in Process.GetProcessesByName("Weixin"))
            {
                Console.WriteLine("  扫描 PID=" + proc.Id + "...");
                IntPtr hProc = OpenProcess(0x0010 | 0x0400 | 0x0008, false, proc.Id);
                if (hProc == IntPtr.Zero) { Console.WriteLine("  OpenProcess 失败(需管理员)"); continue; }

                try
                {
                    ScanForKeys(hProc, rawKeys);
                }
                finally { CloseHandle(hProc); }
                
                if (rawKeys.Count > 0) break;
            }

            // 如果主进程没找到，试 WeChatAppEx
            if (rawKeys.Count == 0)
            {
                foreach (var proc in Process.GetProcessesByName("WeChatAppEx"))
                {
                    Console.WriteLine("  扫描 WeChatAppEx PID=" + proc.Id + "...");
                    IntPtr hProc = OpenProcess(0x0010 | 0x0400 | 0x0008, false, proc.Id);
                    if (hProc == IntPtr.Zero) continue;

                    try { ScanForKeys(hProc, rawKeys); }
                    finally { CloseHandle(hProc); }
                    
                    if (rawKeys.Count > 0) break;
                }
            }
        }
        catch (Exception ex) { Console.WriteLine("  扫描异常: " + ex.Message); }

        Console.WriteLine("  找到 " + rawKeys.Count + " 个 raw key 候选");

        if (rawKeys.Count == 0)
        {
            Console.WriteLine("  [失败] 没有找到密钥");
            return;
        }

        // 3. 验证每个 raw key 对每个数据库
        Console.WriteLine("\n[3] 验证密钥...");
        var validKeys = new Dictionary<string, byte[]>(); // db_path -> enc_key
        
        foreach (var kv in rawKeys)
        {
            byte[] encKey = kv.Key;
            byte[] keySalt = kv.Value;
            
            // 找到 salt 匹配的数据库
            foreach (var dbKv in dbSalts)
            {
                string dbPath = dbKv.Key;
                byte[] dbSalt = dbKv.Value;
                
                bool saltMatch = true;
                for (int i = 0; i < 16; i++)
                {
                    if (dbSalt[i] != keySalt[i]) { saltMatch = false; break; }
                }
                
                if (!saltMatch) continue;
                
                // Salt 匹配! 验证 HMAC
                string rel = dbPath.Replace(dataRoot + "\\", "");
                Console.WriteLine("  Salt 匹配: " + rel);
                
                // 读取第一页
                byte[] page1 = new byte[4096];
                try
                {
                    using (var fs = new FileStream(dbPath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                        fs.Read(page1, 0, 4096);
                }
                catch { continue; }
                
                // 验证 HMAC-SHA512: reserve=80, IV=16, HMAC=64
                // HMAC input: encrypted data + IV + page number (4 bytes, 1-indexed)
                int reserve = 80;
                int dataStart = 16; // 跳过 salt
                int dataLen = 4096 - dataStart - reserve;
                
                // Derive HMAC key: PBKDF2-SHA512(enc_key, salt ^ 0x3a, 2)
                byte[] hmacSalt = new byte[16];
                for (int i = 0; i < 16; i++) hmacSalt[i] = (byte)(dbSalt[i] ^ 0x3a);
                byte[] hmacKey = PBKDF2_SHA512(encKey, hmacSalt, 2, 32);
                
                byte[] hmacInput = new byte[dataLen + 16 + 4];
                Array.Copy(page1, dataStart, hmacInput, 0, dataLen);
                Array.Copy(page1, 4096 - reserve, hmacInput, dataLen, 16); // IV
                hmacInput[dataLen + 16] = 0;
                hmacInput[dataLen + 17] = 0;
                hmacInput[dataLen + 18] = 0;
                hmacInput[dataLen + 19] = 1; // page 1
                
                byte[] computed;
                using (var hmac = new HMACSHA512(hmacKey))
                    computed = hmac.ComputeHash(hmacInput);
                
                // Compare stored HMAC (after IV, 64 bytes)
                bool hmacMatch = true;
                for (int i = 0; i < 64; i++)
                {
                    if (page1[4096 - reserve + 16 + i] != computed[i])
                    { hmacMatch = false; break; }
                }
                
                Console.WriteLine("  HMAC 验证: " + (hmacMatch ? "[成功!]" : "失败"));
                
                if (hmacMatch)
                {
                    validKeys[dbPath] = encKey;
                    Console.WriteLine("  密钥(hex): " + BytesToHex(encKey));
                }
            }
        }

        // 也尝试不匹配 salt 的验证（密钥可能对多个数据库通用）
        if (validKeys.Count == 0)
        {
            Console.WriteLine("\n  Salt 精确匹配失败, 尝试暴力验证...");
            foreach (var kv in rawKeys)
            {
                byte[] encKey = kv.Key;
                foreach (var dbKv in dbSalts)
                {
                    string dbPath = dbKv.Key;
                    byte[] dbSalt = dbKv.Value;
                    
                    byte[] page1 = new byte[4096];
                    try
                    {
                        using (var fs = new FileStream(dbPath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                            fs.Read(page1, 0, 4096);
                    }
                    catch { continue; }
                    
                    int reserve = 80;
                    byte[] hmacSalt = new byte[16];
                    for (int i = 0; i < 16; i++) hmacSalt[i] = (byte)(dbSalt[i] ^ 0x3a);
                    byte[] hmacKey = PBKDF2_SHA512(encKey, hmacSalt, 2, 32);
                    
                    int dataLen = 4096 - 16 - reserve;
                    byte[] hmacInput = new byte[dataLen + 16 + 4];
                    Array.Copy(page1, 16, hmacInput, 0, dataLen);
                    Array.Copy(page1, 4096 - reserve, hmacInput, dataLen, 16);
                    hmacInput[dataLen + 16] = 0; hmacInput[dataLen + 17] = 0;
                    hmacInput[dataLen + 18] = 0; hmacInput[dataLen + 19] = 1;
                    
                    byte[] computed;
                    using (var hmac = new HMACSHA512(hmacKey))
                        computed = hmac.ComputeHash(hmacInput);
                    
                    bool ok = true;
                    for (int i = 0; i < 64; i++)
                        if (page1[4096 - reserve + 16 + i] != computed[i]) { ok = false; break; }
                    
                    if (ok)
                    {
                        string rel = dbPath.Replace(dataRoot + "\\", "");
                        Console.WriteLine("  [暴力验证成功!] " + rel);
                        Console.WriteLine("  密钥: " + BytesToHex(encKey));
                        validKeys[dbPath] = encKey;
                        break;
                    }
                }
                if (validKeys.Count > 0) break;
            }
        }
        
        Console.WriteLine("\n  有效密钥数: " + validKeys.Count);

        // 4. 解密并解析
        if (validKeys.Count > 0)
        {
            Console.WriteLine("\n[4] 解密数据库...");
            foreach (var kv in validKeys)
            {
                string dbPath = kv.Key;
                byte[] encKey = kv.Value;
                string rel = dbPath.Replace(dataRoot + "\\", "");
                
                Console.WriteLine("\n  解密: " + rel);
                byte[] encDb = null;
                try
                {
                    using (var fs = new FileStream(dbPath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                    {
                        encDb = new byte[fs.Length];
                        fs.Read(encDb, 0, encDb.Length);
                    }
                }
                catch (Exception ex)
                {
                    string tmpCopy = Path.Combine(Path.GetTempPath(), "wx_" + Path.GetFileName(dbPath));
                    try { File.Copy(dbPath, tmpCopy, true); encDb = File.ReadAllBytes(tmpCopy); File.Delete(tmpCopy); }
                    catch { Console.WriteLine("  读取失败: " + ex.Message); continue; }
                }
                
                byte[] decDb = DecryptSQLCipher4(encKey, encDb, 4096, 80);
                if (decDb == null) { Console.WriteLine("  解密失败!"); continue; }
                
                Console.WriteLine("  [解密成功!] 大小=" + (decDb.Length / 1024) + "KB");
                ParseSqliteMaster(decDb);
                
                // 只解密一个做验证
                break;
            }
        }
        
        Console.WriteLine("\n=== 完毕 ===");
    }

    static void ScanForKeys(IntPtr hProc, List<KeyValuePair<byte[], byte[]>> results)
    {
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

            if (mbi.State == MEM_COMMIT && regionSize < 200 * 1024 * 1024 &&
                (mbi.Protect & 0xEE) != 0) // any readable
            {
                int chunkSize = (int)Math.Min(regionSize, 4 * 1024 * 1024);
                byte[] chunk = new byte[chunkSize];

                for (long offset = 0; offset < regionSize; offset += chunkSize)
                {
                    int readSize = (int)Math.Min(chunkSize, regionSize - offset);
                    int read;
                    if (!ReadProcessMemory(hProc, new IntPtr(addr + offset), chunk, readSize, out read) || read < 100)
                        continue;

                    bytesScanned += read;

                    // 搜索 x' 模式: 0x78 0x27 后跟 96 个 hex 字符 再跟 0x27
                    for (int i = 0; i <= read - 99; i++)
                    {
                        if (chunk[i] != 0x78 || chunk[i + 1] != 0x27) continue; // x'
                        if (i + 98 >= read) continue;
                        if (chunk[i + 98] != 0x27) continue; // ending '

                        // 检查中间 96 字符是否全是 hex
                        bool allHex = true;
                        for (int j = 2; j < 98; j++)
                        {
                            byte b = chunk[i + j];
                            if (!((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')))
                            { allHex = false; break; }
                        }
                        if (!allHex) continue;

                        // 提取 key 和 salt
                        string hexStr = Encoding.ASCII.GetString(chunk, i + 2, 96);
                        byte[] encKey = HexToBytes(hexStr.Substring(0, 64));
                        byte[] salt = HexToBytes(hexStr.Substring(64, 32));

                        // 检查是否重复
                        bool dup = false;
                        foreach (var existing in results)
                        {
                            bool same = true;
                            for (int k = 0; k < 32; k++)
                                if (existing.Key[k] != encKey[k]) { same = false; break; }
                            if (same) { dup = true; break; }
                        }
                        if (dup) continue;

                        results.Add(new KeyValuePair<byte[], byte[]>(encKey, salt));
                        Console.WriteLine("    找到 raw key #" + results.Count + " at offset 0x" + (addr + offset + i).ToString("X"));
                        Console.WriteLine("    enc_key: " + BytesToHex(encKey));
                        Console.WriteLine("    salt:    " + BytesToHex(salt));
                    }
                }
                regionsScanned++;
            }

            addr += regionSize;
            if (addr < 0) break;
        }

        Console.WriteLine("  扫描了 " + regionsScanned + " 区域, " + (bytesScanned / 1024 / 1024) + " MB");
    }

    static byte[] DecryptSQLCipher4(byte[] encKey, byte[] encDb, int pageSize, int reserve)
    {
        if (encDb == null || encDb.Length < pageSize) return null;

        byte[] salt = new byte[16];
        Array.Copy(encDb, 0, salt, 0, 16);

        int totalPages = encDb.Length / pageSize;
        byte[] output = new byte[totalPages * pageSize];

        for (int pg = 0; pg < totalPages; pg++)
        {
            int pgOff = pg * pageSize;
            int encStart = pg == 0 ? pgOff + 16 : pgOff;
            int encLen = pg == 0 ? pageSize - 16 - reserve : pageSize - reserve;
            if (encStart + encLen > encDb.Length || encLen <= 0) break;
            if (encLen % 16 != 0) encLen = (encLen / 16) * 16;

            byte[] iv = new byte[16];
            Array.Copy(encDb, pgOff + pageSize - reserve, iv, 0, 16);
            byte[] encrypted = new byte[encLen];
            Array.Copy(encDb, encStart, encrypted, 0, encLen);

            try
            {
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
            catch { return null; }
        }

        if (Encoding.ASCII.GetString(output, 0, 15) != "SQLite format 3") return null;
        return output;
    }

    static void ParseSqliteMaster(byte[] data)
    {
        int pageSize = (data[16] << 8) | data[17];
        if (pageSize == 1) pageSize = 65536;
        Console.WriteLine("  pageSize=" + pageSize);
        int hdr = 100;
        byte pt = data[hdr];
        if (pt == 0x05) // interior
        {
            int cellCount = (data[hdr + 3] << 8) | data[hdr + 4];
            long rc = ((long)data[hdr + 8] << 24) | ((long)data[hdr + 9] << 16) | ((long)data[hdr + 10] << 8) | data[hdr + 11];
            var children = new List<int>();
            int ps = hdr + 12;
            for (int c = 0; c < cellCount; c++)
            {
                int po = ps + c * 2;
                if (po + 2 > data.Length) break;
                int co = (data[po] << 8) | data[po + 1];
                if (co + 4 > data.Length) continue;
                long cp = ((long)data[co] << 24) | ((long)data[co + 1] << 16) | ((long)data[co + 2] << 8) | data[co + 3];
                children.Add((int)cp);
            }
            children.Add((int)rc);
            foreach (int cp in children)
            {
                if (cp < 1 || (cp - 1) * pageSize >= data.Length) continue;
                int pgOff = (cp - 1) * pageSize;
                if (data[pgOff] == 0x0D) ParseMasterLeaf(data, pgOff, pageSize);
            }
        }
        else if (pt == 0x0D) ParseMasterLeaf(data, 0, pageSize);
        else Console.WriteLine("  页面类型: 0x" + pt.ToString("X2"));
    }

    static void ParseMasterLeaf(byte[] data, int pageOff, int pageSize)
    {
        int hdr = pageOff + (pageOff == 0 ? 100 : 0);
        int cellCount = (data[hdr + 3] << 8) | data[hdr + 4];
        int ps = hdr + 8;
        for (int c = 0; c < cellCount && c < 100; c++)
        {
            int po = ps + c * 2;
            if (po + 2 > data.Length) break;
            int co = pageOff + ((data[po] << 8) | data[po + 1]);
            if (co >= data.Length || co < pageOff) continue;
            try
            {
                int p = co; int n;
                long pLen; ReadVarint(data, p, out pLen, out n); p += n;
                long rid; ReadVarint(data, p, out rid, out n); p += n;
                long rhs; int hb; ReadVarint(data, p, out rhs, out hb);
                int rhe = p + (int)rhs; int hp = p + hb;
                var ct = new List<long>();
                while (hp < rhe && hp < data.Length) { long st; ReadVarint(data, hp, out st, out n); hp += n; ct.Add(st); }
                if (ct.Count < 5) continue;
                int dp = rhe; string type = null, name = null, sql = null;
                for (int col = 0; col < ct.Count && dp < data.Length; col++)
                {
                    long st = ct[col]; int cl = ColSize(st); if (dp + cl > data.Length) break;
                    if ((col == 0 || col == 1 || col == 4) && st >= 13 && st % 2 == 1)
                    {
                        int tl = (int)(st - 13) / 2;
                        if (tl > 0 && dp + tl <= data.Length) { string v = Encoding.UTF8.GetString(data, dp, tl); if (col == 0) type = v; else if (col == 1) name = v; else sql = v; }
                    }
                    dp += cl;
                }
                string sqlP = sql != null && sql.Length > 200 ? sql.Substring(0, 200) + "..." : sql;
                Console.WriteLine("  " + (type ?? "?") + ": " + (name ?? "?") + " sql=" + sqlP);
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

    static byte[] HexToBytes(string hex) { byte[] r = new byte[hex.Length / 2]; for (int i = 0; i < r.Length; i++) r[i] = Convert.ToByte(hex.Substring(i * 2, 2), 16); return r; }
    static string BytesToHex(byte[] b) { var sb = new StringBuilder(b.Length * 2); foreach (byte x in b) sb.Append(x.ToString("x2")); return sb.ToString(); }
    static void ReadVarint(byte[] d, int p, out long v, out int n) { v = 0; n = 0; for (int i = 0; i < 9 && p + i < d.Length; i++) { v = (v << 7) | (long)(d[p + i] & 0x7F); n = i + 1; if ((d[p + i] & 0x80) == 0) return; } }
    static int ColSize(long st) { if (st == 0 || st == 8 || st == 9) return 0; if (st == 1) return 1; if (st == 2) return 2; if (st == 3) return 3; if (st == 4) return 4; if (st == 5) return 6; if (st == 6 || st == 7) return 8; if (st >= 12 && st % 2 == 0) return (int)(st - 12) / 2; if (st >= 13 && st % 2 == 1) return (int)(st - 13) / 2; return 0; }
}
