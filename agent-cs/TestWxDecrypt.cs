using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;

/// WeChat 4.x 直接解密测试 - 跳过 HMAC, 直接 AES 验证
class TestWxDecrypt
{
    [DllImport("kernel32.dll")] static extern IntPtr OpenProcess(int access, bool inherit, int pid);
    [DllImport("kernel32.dll")] static extern bool ReadProcessMemory(IntPtr hProc, IntPtr baseAddr, byte[] buf, int size, out int read);
    [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr h);
    [DllImport("kernel32.dll")] static extern bool VirtualQueryEx(IntPtr hProcess, IntPtr lpAddress, out MEMORY_BASIC_INFORMATION lpBuffer, uint dwLength);
    [StructLayout(LayoutKind.Sequential)]
    struct MEMORY_BASIC_INFORMATION
    {
        public IntPtr BaseAddress; public IntPtr AllocationBase; public uint AllocationProtect;
        public IntPtr RegionSize; public uint State; public uint Protect; public uint Type;
    }

    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        Console.WriteLine("=== WeChat 4.x 直接解密测试 ===\n");

        string dataRoot = @"D:\weixinliaotian\xwechat_files";
        
        // 1. 找一个最小的加密 db
        Console.WriteLine("[1] 找加密数据库...");
        var dbFiles = new List<KeyValuePair<string, byte[]>>(); // path, salt
        try
        {
            foreach (string f in Directory.GetFiles(dataRoot, "*.db", SearchOption.AllDirectories))
            {
                try
                {
                    long sz = new FileInfo(f).Length;
                    if (sz < 4096 || sz > 50 * 1024 * 1024) continue;
                    byte[] hdr = new byte[16];
                    using (var fs = new FileStream(f, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                        fs.Read(hdr, 0, 16);
                    if (Encoding.ASCII.GetString(hdr, 0, 6) == "SQLite") continue;
                    byte[] salt = new byte[16];
                    Array.Copy(hdr, 0, salt, 0, 16);
                    dbFiles.Add(new KeyValuePair<string, byte[]>(f, salt));
                }
                catch { }
            }
        }
        catch { }
        dbFiles.Sort((a, b) => new FileInfo(a.Key).Length.CompareTo(new FileInfo(b.Key).Length));
        Console.WriteLine("  " + dbFiles.Count + " 个加密数据库, 最小: " + (dbFiles.Count > 0 ? new FileInfo(dbFiles[0].Key).Length / 1024 + "KB" : "?"));

        // 2. 扫描 Weixin 进程找 x'<96hex>' 模式
        Console.WriteLine("\n[2] 扫描进程内存...");
        var rawKeys = new List<KeyValuePair<byte[], byte[]>>();
        foreach (var proc in Process.GetProcessesByName("Weixin"))
        {
            IntPtr hProc = OpenProcess(0x0010 | 0x0400 | 0x0008, false, proc.Id);
            if (hProc == IntPtr.Zero) continue;
            try
            {
                long addr = 0;
                while (addr < 0x7FFFFFFFFFFF)
                {
                    MEMORY_BASIC_INFORMATION mbi;
                    if (!VirtualQueryEx(hProc, new IntPtr(addr), out mbi, (uint)Marshal.SizeOf(typeof(MEMORY_BASIC_INFORMATION)))) break;
                    long rSize = mbi.RegionSize.ToInt64();
                    if (rSize <= 0) break;
                    if (mbi.State == 0x1000 && rSize < 200 * 1024 * 1024)
                    {
                        byte[] chunk = new byte[(int)Math.Min(rSize, 4 * 1024 * 1024)];
                        for (long off = 0; off < rSize; off += chunk.Length)
                        {
                            int rsz = (int)Math.Min(chunk.Length, rSize - off);
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
                                foreach (var ex2 in rawKeys) { bool same = true; for (int k = 0; k < 32; k++) if (ex2.Key[k] != ek[k]) { same = false; break; } if (same) { dup = true; break; } }
                                if (!dup) rawKeys.Add(new KeyValuePair<byte[], byte[]>(ek, sa));
                            }
                        }
                    }
                    addr += rSize; if (addr < 0) break;
                }
            }
            finally { CloseHandle(hProc); }
            if (rawKeys.Count > 0) break;
        }
        // Also scan WeChatAppEx
        foreach (var proc in Process.GetProcessesByName("WeChatAppEx"))
        {
            IntPtr hProc = OpenProcess(0x0010 | 0x0400 | 0x0008, false, proc.Id);
            if (hProc == IntPtr.Zero) continue;
            try
            {
                long addr = 0;
                while (addr < 0x7FFFFFFFFFFF)
                {
                    MEMORY_BASIC_INFORMATION mbi;
                    if (!VirtualQueryEx(hProc, new IntPtr(addr), out mbi, (uint)Marshal.SizeOf(typeof(MEMORY_BASIC_INFORMATION)))) break;
                    long rSize = mbi.RegionSize.ToInt64();
                    if (rSize <= 0) break;
                    if (mbi.State == 0x1000 && rSize < 200 * 1024 * 1024)
                    {
                        byte[] chunk = new byte[(int)Math.Min(rSize, 4 * 1024 * 1024)];
                        for (long off = 0; off < rSize; off += chunk.Length)
                        {
                            int rsz = (int)Math.Min(chunk.Length, rSize - off);
                            int rd;
                            if (!ReadProcessMemory(hProc, new IntPtr(addr + off), chunk, rsz, out rd) || rd < 100) continue;
                            for (int i = 0; i <= rd - 99; i++)
                            {
                                if (chunk[i] != 0x78 || chunk[i + 1] != 0x27 || chunk[i + 98] != 0x27) continue;
                                bool ok2 = true;
                                for (int j = 2; j < 98; j++)
                                {
                                    byte b = chunk[i + j];
                                    if (!((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F'))) { ok2 = false; break; }
                                }
                                if (!ok2) continue;
                                string hex = Encoding.ASCII.GetString(chunk, i + 2, 96);
                                byte[] ek = HexToBytes(hex.Substring(0, 64));
                                byte[] sa = HexToBytes(hex.Substring(64, 32));
                                bool dup = false;
                                foreach (var ex2 in rawKeys) { bool same = true; for (int k = 0; k < 32; k++) if (ex2.Key[k] != ek[k]) { same = false; break; } if (same) { dup = true; break; } }
                                if (!dup) rawKeys.Add(new KeyValuePair<byte[], byte[]>(ek, sa));
                            }
                        }
                    }
                    addr += rSize; if (addr < 0) break;
                }
            }
            finally { CloseHandle(hProc); }
        }
        Console.WriteLine("  找到 " + rawKeys.Count + " 个 raw key");

        // 3. 尝试每个 key 解密每个小数据库
        Console.WriteLine("\n[3] 暴力解密测试 (多种 reserve 值)...");
        int[] reserveSizes = { 80, 48, 64, 32 };
        int[] pageSizes = { 4096, 1024 };

        // 取最小的 5 个 db 测试
        int maxDb = Math.Min(5, dbFiles.Count);
        for (int di = 0; di < maxDb; di++)
        {
            string dbPath = dbFiles[di].Key;
            string rel = dbPath.Replace(dataRoot + "\\", "");
            long dbSize = new FileInfo(dbPath).Length;

            byte[] page1 = new byte[4096];
            try
            {
                using (var fs = new FileStream(dbPath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                    fs.Read(page1, 0, 4096);
            }
            catch { continue; }

            Console.WriteLine("\n  DB: " + rel + " (" + dbSize / 1024 + "KB)");
            Console.WriteLine("  Salt: " + BitConverter.ToString(page1, 0, 8) + "...");

            foreach (var kv in rawKeys)
            {
                byte[] encKey = kv.Key;
                byte[] keySalt = kv.Value;

                foreach (int ps in pageSizes)
                {
                    if (ps > page1.Length) continue;

                    foreach (int reserve in reserveSizes)
                    {
                        if (reserve >= ps - 16) continue;
                        int encLen = ps - 16 - reserve;
                        if (encLen <= 0 || encLen % 16 != 0) continue;

                        byte[] iv = new byte[16];
                        Array.Copy(page1, ps - reserve, iv, 0, 16);
                        byte[] enc = new byte[encLen];
                        Array.Copy(page1, 16, enc, 0, encLen);

                        byte[] dec = AesDecrypt(encKey, iv, enc);
                        if (dec == null) continue;

                        // 检查解密结果: 偏移 84 处 (原始偏移 100 - 16 salt) 应该是 B-tree page type
                        // 0x0D = leaf table, 0x05 = interior table
                        int pt = dec[84]; // offset 100 - 16 = 84 in decrypted first page
                        if (pt == 0x0D || pt == 0x05)
                        {
                            Console.WriteLine("  [可能成功!] ps=" + ps + " res=" + reserve + " dec[84]=0x" + pt.ToString("X2"));
                            Console.WriteLine("    key: " + BytesToHex(encKey));
                            Console.WriteLine("    salt match: " + SaltMatch(keySalt, page1));

                            // 进一步验证: 检查更多特征
                            // 解密完整数据库
                            byte[] fullDb = null;
                            try
                            {
                                using (var fs = new FileStream(dbPath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                                {
                                    fullDb = new byte[fs.Length];
                                    fs.Read(fullDb, 0, fullDb.Length);
                                }
                            }
                            catch { }

                            if (fullDb != null)
                            {
                                byte[] decFull = DecryptFull(encKey, fullDb, ps, reserve);
                                if (decFull != null)
                                {
                                    Console.WriteLine("    [完整解密成功!]");
                                    ParseMaster(decFull, ps);
                                    return; // 找到了！
                                }
                            }
                        }
                    }
                }
            }
        }

        // 如果暴力失败，检查 salt 不匹配的情况
        Console.WriteLine("\n[4] 额外检查: 列出所有 key salt 与 db salt 的对应...");
        foreach (var kv in rawKeys)
        {
            string saltHex = BytesToHex(kv.Value);
            // 找匹配的 db
            bool found = false;
            foreach (var db in dbFiles)
            {
                if (SaltMatch(kv.Value, db.Value))
                {
                    string rel = db.Key.Replace(dataRoot + "\\", "");
                    if (!found) Console.WriteLine("  Key salt " + saltHex.Substring(0, 16) + "... matches:");
                    Console.WriteLine("    " + rel);
                    found = true;
                }
            }
            if (!found)
                Console.WriteLine("  Key salt " + saltHex.Substring(0, 16) + "... NO DB MATCH");
        }

        Console.WriteLine("\n=== 完毕 ===");
    }

    static bool SaltMatch(byte[] keySalt, byte[] dbHeader)
    {
        for (int i = 0; i < 16; i++) if (keySalt[i] != dbHeader[i]) return false;
        return true;
    }

    static byte[] DecryptFull(byte[] encKey, byte[] encDb, int pageSize, int reserve)
    {
        int totalPages = encDb.Length / pageSize;
        byte[] output = new byte[totalPages * pageSize];
        for (int pg = 0; pg < Math.Min(totalPages, 3); pg++) // 只解密前3页验证
        {
            int pgOff = pg * pageSize;
            int encStart = pg == 0 ? pgOff + 16 : pgOff;
            int encLen = pg == 0 ? pageSize - 16 - reserve : pageSize - reserve;
            if (encLen <= 0 || encLen % 16 != 0 || encStart + encLen > encDb.Length) return null;
            byte[] iv = new byte[16];
            Array.Copy(encDb, pgOff + pageSize - reserve, iv, 0, 16);
            byte[] enc = new byte[encLen];
            Array.Copy(encDb, encStart, enc, 0, encLen);
            byte[] d = AesDecrypt(encKey, iv, enc);
            if (d == null) return null;
            if (pg == 0)
            {
                byte[] hdr = Encoding.ASCII.GetBytes("SQLite format 3");
                Array.Copy(hdr, 0, output, 0, 15); output[15] = 0;
                Array.Copy(d, 0, output, 16, d.Length);
                output[16] = (byte)((pageSize >> 8) & 0xFF);
                output[17] = (byte)(pageSize & 0xFF);
                output[20] = (byte)reserve;
            }
            else Array.Copy(d, 0, output, pgOff, d.Length);
        }
        if (output[100] != 0x0D && output[100] != 0x05) return null;
        // 解密所有剩余页
        for (int pg = 3; pg < totalPages; pg++)
        {
            int pgOff = pg * pageSize;
            int encLen = pageSize - reserve;
            if (encLen <= 0 || encLen % 16 != 0 || pgOff + pageSize > encDb.Length) break;
            byte[] iv = new byte[16];
            Array.Copy(encDb, pgOff + pageSize - reserve, iv, 0, 16);
            byte[] enc = new byte[encLen];
            Array.Copy(encDb, pgOff, enc, 0, encLen);
            byte[] d = AesDecrypt(encKey, iv, enc);
            if (d == null) break;
            Array.Copy(d, 0, output, pgOff, d.Length);
        }
        return output;
    }

    static void ParseMaster(byte[] data, int pageSize)
    {
        int hdr = 100;
        byte pt = data[hdr];
        if (pt == 0x05)
        {
            int cc = (data[hdr + 3] << 8) | data[hdr + 4];
            long rc = ((long)data[hdr + 8] << 24) | ((long)data[hdr + 9] << 16) | ((long)data[hdr + 10] << 8) | data[hdr + 11];
            var children = new List<int>();
            for (int c = 0; c < cc; c++)
            {
                int po = hdr + 12 + c * 2;
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
    }

    static void ParseMasterLeaf(byte[] data, int pageOff, int pageSize)
    {
        int hdr = pageOff + (pageOff == 0 ? 100 : 0);
        int cc = (data[hdr + 3] << 8) | data[hdr + 4];
        for (int c = 0; c < cc && c < 50; c++)
        {
            int po = hdr + 8 + c * 2;
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
                    if ((col == 0 || col == 1 || col == 4) && st >= 13 && st % 2 == 1) { int tl = (int)(st - 13) / 2; if (tl > 0 && dp + tl <= data.Length) { string v = Encoding.UTF8.GetString(data, dp, tl); if (col == 0) type = v; else if (col == 1) name = v; else sql = v; } }
                    dp += cl;
                }
                Console.WriteLine("      " + (type ?? "?") + ": " + (name ?? "?"));
            }
            catch { }
        }
    }

    static byte[] AesDecrypt(byte[] key, byte[] iv, byte[] data)
    {
        try
        {
            using (var aes = Aes.Create()) { aes.Mode = CipherMode.CBC; aes.Padding = PaddingMode.None; aes.Key = key; aes.IV = iv; using (var dec = aes.CreateDecryptor()) return dec.TransformFinalBlock(data, 0, data.Length); }
        }
        catch { return null; }
    }

    static byte[] HexToBytes(string hex) { byte[] r = new byte[hex.Length / 2]; for (int i = 0; i < r.Length; i++) r[i] = Convert.ToByte(hex.Substring(i * 2, 2), 16); return r; }
    static string BytesToHex(byte[] b) { var sb = new StringBuilder(b.Length * 2); foreach (byte x in b) sb.Append(x.ToString("x2")); return sb.ToString(); }
    static void ReadVarint(byte[] d, int p, out long v, out int n) { v = 0; n = 0; for (int i = 0; i < 9 && p + i < d.Length; i++) { v = (v << 7) | (long)(d[p + i] & 0x7F); n = i + 1; if ((d[p + i] & 0x80) == 0) return; } }
    static int ColSize(long st) { if (st == 0 || st == 8 || st == 9) return 0; if (st == 1) return 1; if (st == 2) return 2; if (st == 3) return 3; if (st == 4) return 4; if (st == 5) return 6; if (st == 6 || st == 7) return 8; if (st >= 12 && st % 2 == 0) return (int)(st - 12) / 2; if (st >= 13 && st % 2 == 1) return (int)(st - 13) / 2; return 0; }
}
