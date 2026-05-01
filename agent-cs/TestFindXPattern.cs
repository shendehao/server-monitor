using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;

class TestFindXPattern
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

    static bool IsHex(byte b) { return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F'); }

    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        int pid = 18200; // 主 Weixin 进程
        Console.WriteLine("扫描 PID={0} 查找 x'<hex>' 模式...\n", pid);
        
        IntPtr hProc = OpenProcess(0x0010 | 0x0400 | 0x0008, false, pid);
        if (hProc == IntPtr.Zero) { Console.WriteLine("无法打开进程"); return; }
        
        var sw = Stopwatch.StartNew();
        var found = new List<string>();
        long addr = 0x10000;
        
        try
        {
            while (addr < 0x7FFFFFFFFFFF && sw.ElapsedMilliseconds < 30000)
            {
                MEMORY_BASIC_INFORMATION mbi;
                if (!VirtualQueryEx(hProc, new IntPtr(addr), out mbi, (uint)Marshal.SizeOf(typeof(MEMORY_BASIC_INFORMATION)))) break;
                long rSize = mbi.RegionSize.ToInt64();
                if (rSize <= 0) break;
                
                bool readable = mbi.State == 0x1000 && rSize < 50 * 1024 * 1024
                    && (mbi.Protect & 0x104) == 0 && (mbi.Protect & 0xEE) != 0;
                
                if (readable)
                {
                    byte[] chunk = new byte[(int)Math.Min(rSize, 4 * 1024 * 1024)];
                    int rd;
                    if (ReadProcessMemory(hProc, new IntPtr(addr), chunk, chunk.Length, out rd) && rd > 10)
                    {
                        for (int i = 0; i <= rd - 4; i++)
                        {
                            // 找 x' 开头
                            if (chunk[i] != 0x78 || chunk[i + 1] != 0x27) continue;
                            
                            // 数 hex 字符
                            int hexCount = 0;
                            for (int j = i + 2; j < rd && IsHex(chunk[j]); j++) hexCount++;
                            
                            // 检查闭合引号
                            if (hexCount >= 32 && i + 2 + hexCount < rd && chunk[i + 2 + hexCount] == 0x27)
                            {
                                string hex = Encoding.ASCII.GetString(chunk, i + 2, Math.Min(hexCount, 100));
                                string entry = string.Format("@0x{0:X}: x'{1}{2}' ({3} hex chars = {4} bytes)",
                                    addr + i, hex.Substring(0, Math.Min(32, hex.Length)),
                                    hexCount > 32 ? "..." : "", hexCount, hexCount / 2);
                                
                                // 只记录 >=64 hex chars 的 (>=32 bytes key)
                                if (hexCount >= 64)
                                {
                                    found.Add(entry);
                                    Console.WriteLine(entry);
                                    if (hexCount == 96)
                                        Console.WriteLine("  *** 96 hex = 32B key + 16B salt 完美匹配! ***");
                                }
                            }
                        }
                    }
                }
                addr += rSize; if (addr < 0) break;
            }
        }
        finally { CloseHandle(hProc); }
        
        sw.Stop();
        Console.WriteLine("\n耗时: {0}ms, 找到 {1} 个 >=64hex 的 x'...' 模式", sw.ElapsedMilliseconds, found.Count);
    }
}
