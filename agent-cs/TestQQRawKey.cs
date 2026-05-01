using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;

/// NTQQ 密钥提取: 多种策略
/// 1. x'<96hex>' raw key 模式 (类似 WeChat 4.x)
/// 2. sqlite3_key_v2 的 16 字节 passphrase
/// 3. 头部 hex key 的衍生
class TestQQRawKey
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
        Console.WriteLine("=== NTQQ 多策略密钥提取 ===\n");

        // 准备数据库 (跳过 1024 字节头部)
        string dbPath = @"D:\QQ\Tencent Files\3371574658\nt_qq\nt_db\nt_msg.db";
        byte[] rawDb;
        using (var fs = new FileStream(dbPath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
        {
            fs.Seek(1024, SeekOrigin.Begin);
            rawDb = new byte[fs.Length - 1024];
            fs.Read(rawDb, 0, rawDb.Length);
        }
        Console.WriteLine("清理后DB: " + rawDb.Length + " bytes");
        byte[] salt = new byte[16];
        Array.Copy(rawDb, 0, salt, 0, 16);
        Console.WriteLine("Salt: " + BitConverter.ToString(salt));

        // 也准备一个小 db
        string smallDbPath = null;
        byte[] smallDb = null;
        string[] dbNames = { "recent_contact.db", "group_info.db", "profile_info.db" };
        string dbDir = @"D:\QQ\Tencent Files\3371574658\nt_qq\nt_db";
        foreach (string n in dbNames)
        {
            string p = Path.Combine(dbDir, n);
            if (File.Exists(p) && new FileInfo(p).Length > 2048)
            {
                using (var fs2 = new FileStream(p, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
                {
                    fs2.Seek(1024, SeekOrigin.Begin);
                    smallDb = new byte[fs2.Length - 1024];
                    fs2.Read(smallDb, 0, smallDb.Length);
                }
                smallDbPath = n;
                Console.WriteLine("小DB: " + n + " (" + smallDb.Length + " bytes)");
                Console.WriteLine("小DB Salt: " + BitConverter.ToString(smallDb, 0, 16));
                break;
            }
        }

        // 扫描 QQ 进程
        Console.WriteLine("\n[1] 扫描 QQ 进程...");
        var rawKeys96 = new List<KeyValuePair<byte[], byte[]>>(); // enc_key, salt from x'<96hex>'
        var passphrase16 = new List<string>(); // 16-char passphrase candidates (near sqlite3_key_v2)

        foreach (var proc in Process.GetProcessesByName("QQ"))
        {
            ProcessModule wrapperMod = null;
            try
            {
                foreach (ProcessModule mod in proc.Modules)
                    if (mod.ModuleName.ToLowerInvariant() == "wrapper.node") { wrapperMod = mod; break; }
            }
            catch { continue; }
            if (wrapperMod == null) continue;

            Console.WriteLine("PID=" + proc.Id + " wrapper.node base=0x" + wrapperMod.BaseAddress.ToString("X") + " size=" + (wrapperMod.ModuleMemorySize / 1024 / 1024) + "MB");

            IntPtr hProc = OpenProcess(0x0010 | 0x0400 | 0x0008, false, proc.Id);
            if (hProc == IntPtr.Zero) { Console.WriteLine("OpenProcess 失败"); continue; }

            try
            {
                long addr = 0;
                int regions = 0;
                long scanned = 0;
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
                            scanned += rd;

                            // 策略 A: x'<96hex>' 模式
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
                                foreach (var ex2 in rawKeys96) { bool same = true; for (int k = 0; k < 32; k++) if (ex2.Key[k] != ek[k]) { same = false; break; } if (same) { dup = true; break; } }
                                if (!dup) { rawKeys96.Add(new KeyValuePair<byte[], byte[]>(ek, sa)); Console.WriteLine("  [x'96] key=" + BytesToHex(ek).Substring(0, 16) + "... salt=" + BytesToHex(sa).Substring(0, 16) + "..."); }
                            }

                            // 策略 B: 在 "sqlite3_key_v2" 附近找 16 字节 passphrase
                            byte[] sigBytes = Encoding.ASCII.GetBytes("sqlite3_key_v2");
                            for (int i = 0; i <= rd - sigBytes.Length; i++)
                            {
                                bool match = true;
                                for (int j = 0; j < sigBytes.Length; j++) if (chunk[i + j] != sigBytes[j]) { match = false; break; }
                                if (!match) continue;
                                // 在附近 ±4KB 搜索 16 字节 passphrase
                                int searchStart = Math.Max(0, i - 4096);
                                int searchEnd = Math.Min(rd - 17, i + 4096);
                                for (int k = searchStart; k <= searchEnd; k++)
                                {
                                    if (chunk[k + 16] != 0) continue;
                                    bool valid = true;
                                    bool hasSpec = false, hasAlpha = false, hasDigit = false;
                                    for (int j = 0; j < 16; j++)
                                    {
                                        byte b = chunk[k + j];
                                        if (b < 0x21 || b > 0x7E) { valid = false; break; }
                                        if (b >= '0' && b <= '9') hasDigit = true;
                                        else if ((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')) hasAlpha = true;
                                        else hasSpec = true;
                                    }
                                    if (valid && hasAlpha && (hasSpec || hasDigit))
                                    {
                                        string s = Encoding.ASCII.GetString(chunk, k, 16);
                                        if (!s.Contains("\\") && !s.Contains("/") && !s.Contains("    ") && !passphrase16.Contains(s))
                                        {
                                            passphrase16.Add(s);
                                            if (passphrase16.Count <= 10)
                                                Console.WriteLine("  [near sqlite3_key_v2] \"" + s + "\"");
                                        }
                                    }
                                }
                            }
                        }
                        regions++;
                    }
                    addr += rSize; if (addr < 0) break;
                }
                Console.WriteLine("扫描完: " + regions + " 区域, " + (scanned / 1024 / 1024) + " MB");
            }
            finally { CloseHandle(hProc); }
            break;
        }

        Console.WriteLine("\n[2] 结果汇总:");
        Console.WriteLine("  x'96' raw keys: " + rawKeys96.Count);
        Console.WriteLine("  16-char passphrase near sqlite3_key_v2: " + passphrase16.Count);

        // 3. 验证 raw keys
        if (rawKeys96.Count > 0)
        {
            Console.WriteLine("\n[3] 验证 x'96' raw keys...");
            foreach (var kv in rawKeys96)
            {
                byte[] ek = kv.Key;
                // 尝试直接解密 (跳过 HMAC, 直接 AES)
                foreach (int reserve in new int[] { 48, 80, 32 })
                {
                    foreach (int ps in new int[] { 4096, 1024 })
                    {
                        byte[] testDb = rawDb;
                        if (smallDb != null && smallDb.Length > ps) testDb = smallDb;
                        if (testDb.Length < ps) continue;
                        int encLen = ps - 16 - reserve;
                        if (encLen <= 0 || encLen % 16 != 0) continue;
                        byte[] iv = new byte[16];
                        Array.Copy(testDb, ps - reserve, iv, 0, 16);
                        byte[] enc = new byte[encLen];
                        Array.Copy(testDb, 16, enc, 0, encLen);
                        byte[] dec = AesDecrypt(ek, iv, enc);
                        if (dec != null && dec.Length > 84)
                        {
                            byte ptByte = dec[84];
                            if (ptByte == 0x0D || ptByte == 0x05)
                            {
                                Console.WriteLine("  [成功!] ps=" + ps + " res=" + reserve + " dec[84]=0x" + ptByte.ToString("X2"));
                                Console.WriteLine("  key: " + BytesToHex(ek));
                                ParseDecrypted(testDb, ek, ps, reserve);
                                return;
                            }
                        }
                    }
                }
            }
            Console.WriteLine("  所有 raw key 失败");
        }

        // 4. 验证 passphrase near sqlite3_key_v2
        if (passphrase16.Count > 0)
        {
            Console.WriteLine("\n[4] 验证 passphrase (PBKDF2-SHA1, 4000 iter)...");
            byte[] testDb = (smallDb != null && smallDb.Length > 4096) ? smallDb : rawDb;
            byte[] testSalt = new byte[16];
            Array.Copy(testDb, 0, testSalt, 0, 16);

            foreach (string pass in passphrase16)
            {
                byte[] passBytes = Encoding.UTF8.GetBytes(pass);
                foreach (int ps in new int[] { 4096, 1024 })
                {
                    if (testDb.Length < ps) continue;
                    foreach (int reserve in new int[] { 48, 80 })
                    {
                        int encLen = ps - 16 - reserve;
                        if (encLen <= 0 || encLen % 16 != 0) continue;

                        // PBKDF2-SHA1 derive
                        byte[] encKey;
                        using (var kdf = new Rfc2898DeriveBytes(passBytes, testSalt, 4000))
                            encKey = kdf.GetBytes(32);

                        byte[] iv = new byte[16];
                        Array.Copy(testDb, ps - reserve, iv, 0, 16);
                        byte[] enc = new byte[encLen];
                        Array.Copy(testDb, 16, enc, 0, encLen);
                        byte[] dec = AesDecrypt(encKey, iv, enc);
                        if (dec != null && dec.Length > 84)
                        {
                            byte ptByte = dec[84];
                            if (ptByte == 0x0D || ptByte == 0x05)
                            {
                                Console.WriteLine("  [成功!] pass=\"" + pass + "\" ps=" + ps + " res=" + reserve);
                                return;
                            }
                        }
                    }
                }
            }

            // 也试 SHA512
            Console.WriteLine("  SHA1 全部失败, 尝试 PBKDF2-SHA512 4000 iter...");
            foreach (string pass in passphrase16)
            {
                byte[] passBytes = Encoding.UTF8.GetBytes(pass);
                foreach (int ps in new int[] { 4096 })
                {
                    if (testDb.Length < ps) continue;
                    foreach (int reserve in new int[] { 48, 80 })
                    {
                        int encLen = ps - 16 - reserve;
                        if (encLen <= 0 || encLen % 16 != 0) continue;
                        byte[] encKey = PBKDF2_SHA512(passBytes, testSalt, 4000, 32);
                        byte[] iv = new byte[16];
                        Array.Copy(testDb, ps - reserve, iv, 0, 16);
                        byte[] enc = new byte[encLen];
                        Array.Copy(testDb, 16, enc, 0, encLen);
                        byte[] dec = AesDecrypt(encKey, iv, enc);
                        if (dec != null && dec.Length > 84 && (dec[84] == 0x0D || dec[84] == 0x05))
                        {
                            Console.WriteLine("  [成功! SHA512] pass=\"" + pass + "\" ps=" + ps + " res=" + reserve);
                            return;
                        }
                    }
                }
            }
            Console.WriteLine("  所有 passphrase 失败");
        }

        Console.WriteLine("\n=== 完毕 ===");
    }

    static void ParseDecrypted(byte[] encDb, byte[] key, int pageSize, int reserve)
    {
        int totalPages = encDb.Length / pageSize;
        byte[] output = new byte[Math.Min(totalPages, 10) * pageSize];
        for (int pg = 0; pg < Math.Min(totalPages, 10); pg++)
        {
            int pgOff = pg * pageSize;
            int encStart = pg == 0 ? pgOff + 16 : pgOff;
            int encLen = pg == 0 ? pageSize - 16 - reserve : pageSize - reserve;
            if (encLen <= 0 || encLen % 16 != 0 || encStart + encLen > encDb.Length) break;
            byte[] iv = new byte[16]; Array.Copy(encDb, pgOff + pageSize - reserve, iv, 0, 16);
            byte[] enc = new byte[encLen]; Array.Copy(encDb, encStart, enc, 0, encLen);
            byte[] d = AesDecrypt(key, iv, enc);
            if (d == null) break;
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
        if (output[100] == 0x0D || output[100] == 0x05)
        {
            Console.WriteLine("  sqlite_master 解析:");
            // Simple leaf parse
            int hdr = 100;
            int cc = (output[hdr + 3] << 8) | output[hdr + 4];
            Console.WriteLine("  cell count: " + cc);
            int ps2 = hdr + 8;
            for (int c = 0; c < cc && c < 30; c++)
            {
                int po = ps2 + c * 2;
                if (po + 2 > output.Length) break;
                int co = (output[po] << 8) | output[po + 1];
                if (co >= output.Length || co < 0) continue;
                try
                {
                    int p = co; int n;
                    long pLen; ReadVarint(output, p, out pLen, out n); p += n;
                    long rid; ReadVarint(output, p, out rid, out n); p += n;
                    long rhs; int hb; ReadVarint(output, p, out rhs, out hb);
                    int rhe = p + (int)rhs; int hp = p + hb;
                    var ct = new List<long>();
                    while (hp < rhe && hp < output.Length) { long st; ReadVarint(output, hp, out st, out n); hp += n; ct.Add(st); }
                    if (ct.Count < 5) continue;
                    int dp = rhe; string type = null, name = null;
                    for (int col = 0; col < ct.Count && dp < output.Length; col++)
                    {
                        long st = ct[col]; int cl = ColSize(st); if (dp + cl > output.Length) break;
                        if ((col == 0 || col == 1) && st >= 13 && st % 2 == 1) { int tl = (int)(st - 13) / 2; if (tl > 0 && dp + tl <= output.Length) { string v = Encoding.UTF8.GetString(output, dp, tl); if (col == 0) type = v; else name = v; } }
                        dp += cl;
                    }
                    Console.WriteLine("    " + (type ?? "?") + ": " + (name ?? "?"));
                }
                catch { }
            }
        }
    }

    static byte[] AesDecrypt(byte[] key, byte[] iv, byte[] data) { try { using (var aes = Aes.Create()) { aes.Mode = CipherMode.CBC; aes.Padding = PaddingMode.None; aes.Key = key; aes.IV = iv; using (var dec = aes.CreateDecryptor()) return dec.TransformFinalBlock(data, 0, data.Length); } } catch { return null; } }
    static byte[] PBKDF2_SHA512(byte[] password, byte[] salt, int iterations, int dkLen) { byte[] dk = new byte[dkLen]; byte[] bs = new byte[salt.Length + 4]; Array.Copy(salt, bs, salt.Length); bs[salt.Length + 3] = 1; byte[] u; using (var h = new HMACSHA512(password)) u = h.ComputeHash(bs); byte[] r = (byte[])u.Clone(); for (int i = 1; i < iterations; i++) { using (var h = new HMACSHA512(password)) u = h.ComputeHash(u); for (int j = 0; j < 64; j++) r[j] ^= u[j]; } Array.Copy(r, 0, dk, 0, dkLen); return dk; }
    static byte[] HexToBytes(string hex) { byte[] r = new byte[hex.Length / 2]; for (int i = 0; i < r.Length; i++) r[i] = Convert.ToByte(hex.Substring(i * 2, 2), 16); return r; }
    static string BytesToHex(byte[] b) { var sb = new StringBuilder(b.Length * 2); foreach (byte x in b) sb.Append(x.ToString("x2")); return sb.ToString(); }
    static void ReadVarint(byte[] d, int p, out long v, out int n) { v = 0; n = 0; for (int i = 0; i < 9 && p + i < d.Length; i++) { v = (v << 7) | (long)(d[p + i] & 0x7F); n = i + 1; if ((d[p + i] & 0x80) == 0) return; } }
    static int ColSize(long st) { if (st == 0 || st == 8 || st == 9) return 0; if (st == 1) return 1; if (st == 2) return 2; if (st == 3) return 3; if (st == 4) return 4; if (st == 5) return 6; if (st == 6 || st == 7) return 8; if (st >= 12 && st % 2 == 0) return (int)(st - 12) / 2; if (st >= 13 && st % 2 == 1) return (int)(st - 13) / 2; return 0; }
}
