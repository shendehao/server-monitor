using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;

class TestScanAll
{
    [DllImport("kernel32.dll")] static extern IntPtr OpenProcess(int access, bool inherit, int pid);
    [DllImport("kernel32.dll")] static extern bool ReadProcessMemory(IntPtr hProc, IntPtr baseAddr, byte[] buf, int size, out int read);
    [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr h);
    [DllImport("kernel32.dll")] static extern bool VirtualQueryEx(IntPtr hProc, IntPtr addr, out MEMORY_BASIC_INFORMATION mbi, uint len);
    [StructLayout(LayoutKind.Sequential)]
    struct MEMORY_BASIC_INFORMATION { public IntPtr BaseAddress, AllocationBase; public uint AllocationProtect; public IntPtr RegionSize; public uint State, Protect, Type; }

    static bool IsHex(byte b) { return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F'); }

    static byte[] PBKDF2_SHA512(byte[] pw, byte[] salt, int iter, int outLen)
    {
        byte[] result = new byte[outLen];
        byte[] blockBytes = new byte[] { 0, 0, 0, 1 };
        byte[] saltBlock = new byte[salt.Length + 4];
        Array.Copy(salt, saltBlock, salt.Length);
        Array.Copy(blockBytes, 0, saltBlock, salt.Length, 4);
        byte[] u, f;
        using (var hmac = new HMACSHA512(pw)) u = hmac.ComputeHash(saltBlock);
        f = (byte[])u.Clone();
        for (int i = 1; i < iter; i++) { using (var hmac = new HMACSHA512(pw)) u = hmac.ComputeHash(u); for (int j = 0; j < f.Length; j++) f[j] ^= u[j]; }
        Array.Copy(f, 0, result, 0, Math.Min(64, outLen));
        return result;
    }

    static bool VerifyKey(byte[] key, byte[] page1, int reserveSize)
    {
        try
        {
            int ps = 4096;
            byte[] salt = new byte[16]; Array.Copy(page1, 0, salt, 0, 16);
            byte[] hmacSalt = new byte[16]; for (int i = 0; i < 16; i++) hmacSalt[i] = (byte)(salt[i] ^ 0x3a);
            byte[] hmacKey = PBKDF2_SHA512(key, hmacSalt, 2, 32);
            int dataLen = ps - 16 - reserveSize;
            byte[] hmacInput = new byte[dataLen + 16 + 4];
            Array.Copy(page1, 16, hmacInput, 0, dataLen);
            Array.Copy(page1, ps - reserveSize, hmacInput, dataLen, 16);
            hmacInput[dataLen + 16] = 0; hmacInput[dataLen + 17] = 0; hmacInput[dataLen + 18] = 0; hmacInput[dataLen + 19] = 1;
            byte[] computed; using (var hmac = new HMACSHA512(hmacKey)) computed = hmac.ComputeHash(hmacInput);
            for (int i = 0; i < 64 && i < (reserveSize - 16); i++)
                if (page1[ps - reserveSize + 16 + i] != computed[i]) return false;
            return true;
        }
        catch { return false; }
    }

    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;

        // 读 DB 第一页
        string dbPath = @"D:\weixinliaotian\xwechat_files\wxid_3lxrvh9517lg12_7804\db_storage\contact\contact.db";
        byte[] page1 = new byte[4096];
        using (var fs = new FileStream(dbPath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
            fs.Read(page1, 0, 4096);
        byte[] salt = new byte[16]; Array.Copy(page1, 0, salt, 0, 16);
        Console.WriteLine("Salt: " + BitConverter.ToString(salt).Replace("-", ""));

        string[] procNames = { "Weixin", "WeChatAppEx" };
        var sw = Stopwatch.StartNew();
        int totalFound = 0;

        foreach (string pn in procNames)
        {
            var procs = Process.GetProcessesByName(pn);
            Console.WriteLine("\n=== {0}: {1}个进程 ===", pn, procs.Length);
            foreach (var proc in procs)
            {
                IntPtr hProc = OpenProcess(0x0010 | 0x0400 | 0x0008, false, proc.Id);
                if (hProc == IntPtr.Zero) continue;
                int saltHits = 0, xpatternHits = 0;
                long scanned = 0;
                try
                {
                    long addr = 0x10000;
                    while (addr < 0x7FFFFFFFFFFF && sw.ElapsedMilliseconds < 120000)
                    {
                        MEMORY_BASIC_INFORMATION mbi;
                        if (!VirtualQueryEx(hProc, new IntPtr(addr), out mbi, (uint)Marshal.SizeOf(typeof(MEMORY_BASIC_INFORMATION)))) break;
                        long rSize = mbi.RegionSize.ToInt64();
                        if (rSize <= 0) break;
                        bool readable = mbi.State == 0x1000 && rSize < 50 * 1024 * 1024 && (mbi.Protect & 0x104) == 0 && (mbi.Protect & 0xEE) != 0;
                        if (readable)
                        {
                            int chunkSz = (int)Math.Min(rSize, 4 * 1024 * 1024);
                            byte[] chunk = new byte[chunkSz];
                            for (long off = 0; off < rSize; off += chunkSz)
                            {
                                int rsz = (int)Math.Min(chunkSz, rSize - off);
                                int rd;
                                if (!ReadProcessMemory(hProc, new IntPtr(addr + off), chunk, rsz, out rd) || rd < 48) continue;
                                scanned += rd;

                                for (int i = 0; i <= rd - 48; i++)
                                {
                                    // 查找 salt
                                    if (chunk[i] == salt[0] && chunk[i+1] == salt[1])
                                    {
                                        bool m = true;
                                        for (int j = 2; j < 16; j++) if (chunk[i+j] != salt[j]) { m = false; break; }
                                        if (m)
                                        {
                                            saltHits++;
                                            if (saltHits <= 5)
                                            {
                                                Console.Write("  salt@0x{0:X}+{1}", addr+off, i);
                                                // 尝试附近的 32 字节作为 key
                                                int[] offsets = { -32, -48, -64, -80, -96, -128, 16, 20, 24, 32 };
                                                foreach (int ko in offsets)
                                                {
                                                    int kp = i + ko;
                                                    if (kp < 0 || kp + 32 > rd) continue;
                                                    byte[] ck = new byte[32]; Array.Copy(chunk, kp, ck, 0, 32);
                                                    bool[] seen = new bool[256]; int d = 0;
                                                    foreach (byte b in ck) if (!seen[b]) { seen[b] = true; d++; }
                                                    if (d < 12) continue;
                                                    if (VerifyKey(ck, page1, 80))
                                                    {
                                                        Console.WriteLine("\n  *** KEY FOUND (r80)! offset={0} ***", ko);
                                                        Console.WriteLine("  " + BitConverter.ToString(ck).Replace("-","")); totalFound++;
                                                    }
                                                    if (VerifyKey(ck, page1, 48))
                                                    {
                                                        Console.WriteLine("\n  *** KEY FOUND (r48)! offset={0} ***", ko);
                                                        Console.WriteLine("  " + BitConverter.ToString(ck).Replace("-","")); totalFound++;
                                                    }
                                                }
                                                Console.WriteLine();
                                            }
                                        }
                                    }

                                    // 查找 x' 模式 (>=64 hex)
                                    if (chunk[i] == 0x78 && chunk[i+1] == 0x27)
                                    {
                                        int hc = 0;
                                        for (int j = i+2; j < rd && IsHex(chunk[j]); j++) hc++;
                                        if (hc >= 64 && i+2+hc < rd && chunk[i+2+hc] == 0x27)
                                        {
                                            xpatternHits++;
                                            string hex = Encoding.ASCII.GetString(chunk, i+2, Math.Min(hc, 40));
                                            Console.WriteLine("  x'@0x{0:X}+{1}: {2}hex={3}B ({4}...)", addr+off, i, hc, hc/2, hex);
                                        }
                                    }
                                }
                            }
                        }
                        addr += rSize; if (addr < 0) break;
                    }
                }
                finally { CloseHandle(hProc); }
                Console.WriteLine("  PID={0}: scanned={1:F1}MB salt_hits={2} x_hits={3}", proc.Id, scanned/1048576.0, saltHits, xpatternHits);
            }
        }
        Console.WriteLine("\n总耗时: {0}s, 找到密钥: {1}", sw.ElapsedMilliseconds/1000, totalFound);
    }
}
