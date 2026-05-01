using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;

class TestBinaryKeyScan
{
    [DllImport("kernel32.dll")] static extern IntPtr OpenProcess(int access, bool inherit, int pid);
    [DllImport("kernel32.dll")] static extern bool ReadProcessMemory(IntPtr hProc, IntPtr baseAddr, byte[] buf, int size, out int read);
    [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr h);
    [DllImport("kernel32.dll")] static extern bool VirtualQueryEx(IntPtr hProc, IntPtr addr, out MEMORY_BASIC_INFORMATION mbi, uint len);

    [StructLayout(LayoutKind.Sequential)]
    struct MEMORY_BASIC_INFORMATION
    {
        public IntPtr BaseAddress, AllocationBase;
        public uint AllocationProtect;
        public IntPtr RegionSize;
        public uint State, Protect, Type;
    }

    static byte[] PBKDF2_SHA512(byte[] pw, byte[] salt, int iter, int outLen)
    {
        byte[] result = new byte[outLen];
        byte[] blockBytes = new byte[] { 0, 0, 0, 1 };
        byte[] saltBlock = new byte[salt.Length + 4];
        Array.Copy(salt, saltBlock, salt.Length);
        Array.Copy(blockBytes, 0, saltBlock, salt.Length, 4);
        byte[] u; byte[] f;
        using (var hmac = new HMACSHA512(pw)) u = hmac.ComputeHash(saltBlock);
        f = (byte[])u.Clone();
        for (int i = 1; i < iter; i++)
        {
            using (var hmac = new HMACSHA512(pw)) u = hmac.ComputeHash(u);
            for (int j = 0; j < f.Length; j++) f[j] ^= u[j];
        }
        Array.Copy(f, 0, result, 0, Math.Min(64, outLen));
        return result;
    }

    // 验证候选 key 能否解密第一页
    static bool VerifyKey(byte[] candidateKey, byte[] page1, int reserveSize, string hashType)
    {
        try
        {
            int pageSize = 4096;
            byte[] salt = new byte[16];
            Array.Copy(page1, 0, salt, 0, 16);

            // 派生 HMAC key
            byte[] hmacSalt = new byte[16];
            for (int i = 0; i < 16; i++) hmacSalt[i] = (byte)(salt[i] ^ 0x3a);

            byte[] hmacKey;
            int hmacLen;
            if (hashType == "sha512")
            {
                hmacKey = PBKDF2_SHA512(candidateKey, hmacSalt, 2, 32);
                hmacLen = 64;
            }
            else
            {
                using (var kdf = new Rfc2898DeriveBytes(candidateKey, hmacSalt, 2))
                    hmacKey = kdf.GetBytes(32);
                hmacLen = 20;
            }

            int dataLen = pageSize - 16 - reserveSize;
            byte[] hmacInput = new byte[dataLen + 16 + 4];
            Array.Copy(page1, 16, hmacInput, 0, dataLen);
            Array.Copy(page1, pageSize - reserveSize, hmacInput, dataLen, 16);
            hmacInput[dataLen + 16] = 0; hmacInput[dataLen + 17] = 0;
            hmacInput[dataLen + 18] = 0; hmacInput[dataLen + 19] = 1;

            byte[] computed;
            if (hashType == "sha512")
                using (var hmac = new HMACSHA512(hmacKey)) computed = hmac.ComputeHash(hmacInput);
            else
                using (var hmac = new HMACSHA1(hmacKey)) computed = hmac.ComputeHash(hmacInput);

            for (int i = 0; i < hmacLen && i < (reserveSize - 16); i++)
                if (page1[pageSize - reserveSize + 16 + i] != computed[i]) return false;
            return true;
        }
        catch { return false; }
    }

    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        Console.WriteLine("=== WeChat 4.x 二进制密钥扫描 ===\n");

        // 读取 DB salt
        string dbPath = @"D:\weixinliaotian\xwechat_files\wxid_3lxrvh9517lg12_7804\db_storage\contact\contact.db";
        byte[] page1 = new byte[4096];
        using (var fs = new FileStream(dbPath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
            fs.Read(page1, 0, 4096);
        byte[] salt = new byte[16];
        Array.Copy(page1, 0, salt, 0, 16);
        Console.WriteLine("DB Salt: " + BitConverter.ToString(salt).Replace("-", ""));

        // 扫描 Weixin 进程
        var procs = Process.GetProcessesByName("Weixin");
        Console.WriteLine("Weixin 进程数: " + procs.Length);

        var sw = Stopwatch.StartNew();
        int totalRegions = 0, scannedRegions = 0;
        long totalScanned = 0;
        var candidates = new List<byte[]>();

        foreach (var proc in procs)
        {
            Console.WriteLine("\n扫描 PID={0}...", proc.Id);
            IntPtr hProc = OpenProcess(0x0010 | 0x0400 | 0x0008, false, proc.Id);
            if (hProc == IntPtr.Zero) { Console.WriteLine("  无法打开进程"); continue; }

            try
            {
                long addr = 0x10000;
                while (addr < 0x7FFFFFFFFFFF && sw.ElapsedMilliseconds < 60000)
                {
                    MEMORY_BASIC_INFORMATION mbi;
                    if (!VirtualQueryEx(hProc, new IntPtr(addr), out mbi, (uint)Marshal.SizeOf(typeof(MEMORY_BASIC_INFORMATION)))) break;
                    long rSize = mbi.RegionSize.ToInt64();
                    if (rSize <= 0) break;
                    totalRegions++;

                    bool readable = mbi.State == 0x1000 && rSize < 50 * 1024 * 1024
                        && (mbi.Protect & 0x104) == 0
                        && (mbi.Protect & 0xEE) != 0;

                    if (readable)
                    {
                        scannedRegions++;
                        int chunkSz = (int)Math.Min(rSize, 4 * 1024 * 1024);
                        byte[] chunk = new byte[chunkSz];
                        for (long off = 0; off < rSize; off += chunkSz)
                        {
                            int rsz = (int)Math.Min(chunkSz, rSize - off);
                            int rd;
                            if (!ReadProcessMemory(hProc, new IntPtr(addr + off), chunk, rsz, out rd) || rd < 48) continue;
                            totalScanned += rd;

                            // 搜索 salt 匹配
                            for (int i = 0; i <= rd - 16; i++)
                            {
                                bool match = true;
                                for (int j = 0; j < 16; j++)
                                    if (chunk[i + j] != salt[j]) { match = false; break; }
                                if (!match) continue;

                                // salt 找到! 检查附近是否有 32 字节 key
                                // key 可能在 salt 之前 (-32, -48, -64) 或之后 (+16, +20, +24)
                                int[] keyOffsets = { -32, -36, -40, -48, -64, 16, 20, 24 };
                                foreach (int ko in keyOffsets)
                                {
                                    int keyPos = i + ko;
                                    if (keyPos < 0 || keyPos + 32 > rd) continue;
                                    byte[] candidateKey = new byte[32];
                                    Array.Copy(chunk, keyPos, candidateKey, 0, 32);

                                    // 检查熵 - 至少16个不同字节
                                    bool[] seen = new bool[256]; int distinct = 0;
                                    foreach (byte b in candidateKey) if (!seen[b]) { seen[b] = true; distinct++; }
                                    if (distinct < 16) continue;

                                    // 快速检查：不全为 ASCII 可打印字符（排除字符串）
                                    int printable = 0;
                                    foreach (byte b in candidateKey) if (b >= 0x20 && b <= 0x7E) printable++;
                                    if (printable > 28) continue; // 太多可打印字符，可能是字符串

                                    // 验证
                                    foreach (int res in new int[] { 80, 48 })
                                    {
                                        if (VerifyKey(candidateKey, page1, res, "sha512"))
                                        {
                                            Console.WriteLine("\n  *** 找到密钥! reserve={0} SHA512 ***", res);
                                            Console.WriteLine("  Key: " + BitConverter.ToString(candidateKey).Replace("-", ""));
                                            Console.WriteLine("  Salt位置: 0x{0:X} + {1}, Key偏移: {2}", addr + off, i, ko);
                                            candidates.Add(candidateKey);
                                        }
                                        if (VerifyKey(candidateKey, page1, res, "sha1"))
                                        {
                                            Console.WriteLine("\n  *** 找到密钥! reserve={0} SHA1 ***", res);
                                            Console.WriteLine("  Key: " + BitConverter.ToString(candidateKey).Replace("-", ""));
                                            Console.WriteLine("  Salt位置: 0x{0:X} + {1}, Key偏移: {2}", addr + off, i, ko);
                                            candidates.Add(candidateKey);
                                        }
                                    }
                                }
                            }
                        }
                    }
                    addr += rSize; if (addr < 0) break;
                }
            }
            finally { CloseHandle(hProc); }
            if (candidates.Count > 0) break; // 找到就停
        }

        sw.Stop();
        Console.WriteLine("\n--- 统计 ---");
        Console.WriteLine("耗时: {0}ms", sw.ElapsedMilliseconds);
        Console.WriteLine("区域: {0}总/{1}扫描", totalRegions, scannedRegions);
        Console.WriteLine("扫描数据量: {0:F1}MB", totalScanned / 1048576.0);
        Console.WriteLine("找到候选密钥: {0}", candidates.Count);
    }
}
