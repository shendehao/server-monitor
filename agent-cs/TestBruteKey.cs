using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;
using System.Threading;
using System.Threading.Tasks;

class TestBruteKey
{
    [DllImport("kernel32.dll")] static extern IntPtr OpenProcess(int access, bool inherit, int pid);
    [DllImport("kernel32.dll")] static extern bool ReadProcessMemory(IntPtr hProc, IntPtr baseAddr, byte[] buf, int size, out int read);
    [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr h);
    [DllImport("kernel32.dll")] static extern bool VirtualQueryEx(IntPtr hProc, IntPtr addr, out MEMORY_BASIC_INFORMATION mbi, uint len);
    [StructLayout(LayoutKind.Sequential)]
    struct MEMORY_BASIC_INFORMATION { public IntPtr BaseAddress, AllocationBase; public uint AllocationProtect; public IntPtr RegionSize; public uint State, Protect, Type; }

    static byte[] page1;
    static byte[] dbSalt = new byte[16];
    static byte[] hmacSalt = new byte[16];
    static byte[] storedHmac = new byte[64];
    static byte[] hmacContent; // pre-built content for HMAC verification
    const int PAGE_SIZE = 4096;
    const int RESERVE = 80;

    static bool VerifyCandidate(byte[] key)
    {
        try
        {
            // PBKDF2-SHA512(key, hmacSalt, 2, 32) — only 2 iterations, fast
            byte[] saltBlock = new byte[hmacSalt.Length + 4];
            Array.Copy(hmacSalt, saltBlock, 16);
            saltBlock[16] = 0; saltBlock[17] = 0; saltBlock[18] = 0; saltBlock[19] = 1;
            byte[] u, f;
            using (var h = new HMACSHA512(key)) u = h.ComputeHash(saltBlock);
            f = (byte[])u.Clone();
            using (var h = new HMACSHA512(key)) u = h.ComputeHash(u);
            for (int j = 0; j < 64; j++) f[j] ^= u[j];
            byte[] hmacKey = new byte[32];
            Array.Copy(f, hmacKey, 32);

            // HMAC-SHA512 of page content
            byte[] computed;
            using (var h = new HMACSHA512(hmacKey)) computed = h.ComputeHash(hmacContent);

            // Compare first 64 bytes of HMAC
            for (int i = 0; i < 64; i++)
                if (storedHmac[i] != computed[i]) return false;
            return true;
        }
        catch { return false; }
    }

    static volatile int foundCount = 0;
    static byte[] foundKey = null;

    static void ScanChunk(byte[] data, int length, long baseAddr)
    {
        // Try every 8-byte aligned position as a 32-byte key candidate
        for (int i = 0; i <= length - 32 && foundCount == 0; i += 8)
        {
            // Quick pre-filter: at least 15 distinct byte values in 32 bytes
            byte flags0 = 0, flags1 = 0, flags2 = 0, flags3 = 0;
            int allZero = 0;
            for (int j = 0; j < 32; j++)
            {
                byte b = data[i + j];
                if (b == 0) allZero++;
                int idx = b >> 6;
                byte bit = (byte)(1 << (b & 0x3F));
                // simplified distinct counting
            }
            // Use a simpler check: not all zeros, not all same byte
            if (allZero > 24) continue;
            if (data[i] == data[i+1] && data[i] == data[i+2] && data[i] == data[i+8] && data[i] == data[i+16] && data[i] == data[i+24]) continue;
            
            // Full distinct byte count
            bool[] seen = new bool[256];
            int distinct = 0;
            for (int j = 0; j < 32; j++)
            {
                byte b = data[i + j];
                if (!seen[b]) { seen[b] = true; distinct++; }
            }
            if (distinct < 15) continue;

            // Extract candidate key
            byte[] ck = new byte[32];
            Array.Copy(data, i, ck, 0, 32);

            if (VerifyCandidate(ck))
            {
                foundKey = ck;
                Interlocked.Increment(ref foundCount);
                Console.WriteLine("\n*** 找到密钥! 内存地址=0x{0:X} ***", baseAddr + i);
                Console.WriteLine("Key: " + BitConverter.ToString(ck).Replace("-", ""));
                return;
            }
        }
    }

    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        Console.WriteLine("=== WeChat 4.x 暴力密钥搜索 ===\n");

        // 读取加密数据库第一页
        string dbPath = @"D:\weixinliaotian\xwechat_files\wxid_3lxrvh9517lg12_7804\db_storage\contact\contact.db";
        page1 = new byte[PAGE_SIZE];
        using (var fs = new FileStream(dbPath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
            fs.Read(page1, 0, PAGE_SIZE);

        // 预计算
        Array.Copy(page1, 0, dbSalt, 0, 16);
        for (int i = 0; i < 16; i++) hmacSalt[i] = (byte)(dbSalt[i] ^ 0x3a);
        Array.Copy(page1, PAGE_SIZE - RESERVE + 16, storedHmac, 0, 64);
        
        // hmacContent = page[16..PAGE_SIZE-RESERVE] + IV(16) + pageNo(4)
        int dataLen = PAGE_SIZE - 16 - RESERVE;
        hmacContent = new byte[dataLen + 16 + 4];
        Array.Copy(page1, 16, hmacContent, 0, dataLen);
        Array.Copy(page1, PAGE_SIZE - RESERVE, hmacContent, dataLen, 16); // IV
        hmacContent[dataLen + 16] = 0;
        hmacContent[dataLen + 17] = 0;
        hmacContent[dataLen + 18] = 0;
        hmacContent[dataLen + 19] = 1; // page number = 1

        Console.WriteLine("DB Salt: " + BitConverter.ToString(dbSalt).Replace("-", ""));
        Console.WriteLine("Stored HMAC前16: " + BitConverter.ToString(storedHmac, 0, 16).Replace("-", ""));

        // 扫描所有 WeChat 进程
        string[] procNames = { "Weixin", "WeChatAppEx" };
        var sw = Stopwatch.StartNew();
        long totalCandidates = 0;
        long totalScanned = 0;

        foreach (string pn in procNames)
        {
            if (foundCount > 0) break;
            var procs = Process.GetProcessesByName(pn);
            Console.WriteLine("\n--- {0}: {1}个进程 ---", pn, procs.Length);
            
            foreach (var proc in procs)
            {
                if (foundCount > 0) break;
                IntPtr hProc = OpenProcess(0x0010 | 0x0400 | 0x0008, false, proc.Id);
                if (hProc == IntPtr.Zero) continue;
                Console.Write("PID={0} 扫描中...", proc.Id);
                long procCandidates = 0;
                long procScanned = 0;

                try
                {
                    long addr = 0x10000;
                    while (addr < 0x7FFFFFFFFFFF && foundCount == 0)
                    {
                        MEMORY_BASIC_INFORMATION mbi;
                        if (!VirtualQueryEx(hProc, new IntPtr(addr), out mbi, (uint)Marshal.SizeOf(typeof(MEMORY_BASIC_INFORMATION)))) break;
                        long rSize = mbi.RegionSize.ToInt64();
                        if (rSize <= 0) break;

                        bool readable = mbi.State == 0x1000 && rSize < 100 * 1024 * 1024
                            && (mbi.Protect & 0x104) == 0 && (mbi.Protect & 0xEE) != 0;

                        if (readable && rSize >= 32)
                        {
                            int chunkSz = (int)Math.Min(rSize, 8 * 1024 * 1024);
                            byte[] chunk = new byte[chunkSz];
                            for (long off = 0; off < rSize && foundCount == 0; off += chunkSz)
                            {
                                int rsz = (int)Math.Min(chunkSz, rSize - off);
                                int rd;
                                if (!ReadProcessMemory(hProc, new IntPtr(addr + off), chunk, rsz, out rd) || rd < 32) continue;
                                procScanned += rd;
                                ScanChunk(chunk, rd, addr + off);
                                procCandidates += rd / 8;
                            }
                        }
                        addr += rSize; if (addr < 0) break;
                    }
                }
                finally { CloseHandle(hProc); }
                totalCandidates += procCandidates;
                totalScanned += procScanned;
                Console.WriteLine(" {0:F1}MB, {1:F1}M候选", procScanned / 1048576.0, procCandidates / 1000000.0);
            }
        }

        sw.Stop();
        Console.WriteLine("\n=== 结果 ===");
        Console.WriteLine("总耗时: {0:F1}秒", sw.ElapsedMilliseconds / 1000.0);
        Console.WriteLine("扫描: {0:F1}MB, {1:F1}M候选", totalScanned / 1048576.0, totalCandidates / 1000000.0);
        if (foundKey != null)
        {
            Console.WriteLine("密钥: " + BitConverter.ToString(foundKey).Replace("-", ""));
            
            // 解密验证
            int dLen = PAGE_SIZE - 16 - RESERVE;
            byte[] iv = new byte[16];
            Array.Copy(page1, PAGE_SIZE - RESERVE, iv, 0, 16);
            byte[] encData = new byte[dLen];
            Array.Copy(page1, 16, encData, 0, dLen);
            using (var aes = Aes.Create())
            {
                aes.KeySize = 256; aes.Mode = CipherMode.CBC; aes.Padding = PaddingMode.None;
                aes.Key = foundKey; aes.IV = iv;
                byte[] dec = aes.CreateDecryptor().TransformFinalBlock(encData, 0, encData.Length);
                Console.WriteLine("解密后前32字节: " + BitConverter.ToString(dec, 0, 32).Replace("-",""));
                Console.WriteLine("byte[84]=0x{0:X2} (期望0x0D/0x05): {1}", dec[84], (dec[84]==0x0D||dec[84]==0x05) ? "有效!" : "无效");
            }
        }
        else
        {
            Console.WriteLine("未找到密钥");
        }
    }
}
