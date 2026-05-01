using System;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;

class TestConPTY
{
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern int CreatePseudoConsole(COORD size, IntPtr hInput, IntPtr hOutput, uint dwFlags, out IntPtr phPC);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern void ClosePseudoConsole(IntPtr hPC);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool CreatePipe(out IntPtr hReadPipe, out IntPtr hWritePipe, ref SA sa, uint nSize);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool ReadFile(IntPtr hFile, byte[] buf, int nBytesToRead, out int nBytesRead, IntPtr overlapped);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool WriteFile(IntPtr hFile, byte[] buf, int nBytesToWrite, out int nBytesWritten, IntPtr overlapped);
    [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr h);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool InitializeProcThreadAttributeList(IntPtr lpAttrList, int dwAttrCount, int dwFlags, ref IntPtr lpSize);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool UpdateProcThreadAttribute(IntPtr lpAttrList, uint dwFlags, IntPtr attr, IntPtr lpValue, IntPtr cbSize, IntPtr lpPrev, IntPtr lpRetSz);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern void DeleteProcThreadAttributeList(IntPtr lpAttrList);
    [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
    static extern bool CreateProcessW(string lpApp, string lpCmd, IntPtr lpProcAttr, IntPtr lpThreadAttr,
        bool bInheritHandles, uint dwFlags, IntPtr lpEnv, string lpCwd, ref SIEX lpSI, out PI lpPI);
    [DllImport("kernel32.dll")] static extern uint WaitForSingleObject(IntPtr h, uint ms);

    [StructLayout(LayoutKind.Sequential)] struct COORD { public short X, Y; }
    [StructLayout(LayoutKind.Sequential)] struct SA { public int nLength; public IntPtr lpSD; public bool bInheritHandle; }
    [StructLayout(LayoutKind.Sequential)] struct SI
    {
        public int cb; public IntPtr lpReserved, lpDesktop, lpTitle;
        public int dwX, dwY, dwXSize, dwYSize, dwXCountChars, dwYCountChars, dwFillAttribute, dwFlags;
        public short wShowWindow, cbReserved2; public IntPtr lpReserved2, hStdInput, hStdOutput, hStdError;
    }
    [StructLayout(LayoutKind.Sequential)] struct SIEX { public SI StartupInfo; public IntPtr lpAttributeList; }
    [StructLayout(LayoutKind.Sequential)] struct PI { public IntPtr hProcess, hThread; public int dwProcessId, dwThreadId; }

    const uint EXTENDED_STARTUPINFO_PRESENT = 0x00080000;
    static readonly IntPtr PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE = (IntPtr)0x00020016;

    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        Console.WriteLine("=== ConPTY Test ===");

        IntPtr inputReadHandle, inputWriteHandle, outputReadHandle, outputWriteHandle;
        var sa = new SA { nLength = Marshal.SizeOf(typeof(SA)), bInheritHandle = true };

        if (!CreatePipe(out inputReadHandle, out inputWriteHandle, ref sa, 0))
        { Console.WriteLine("CreatePipe(input) FAIL: " + Marshal.GetLastWin32Error()); return; }
        if (!CreatePipe(out outputReadHandle, out outputWriteHandle, ref sa, 0))
        { Console.WriteLine("CreatePipe(output) FAIL: " + Marshal.GetLastWin32Error()); return; }

        Console.WriteLine("Pipes created OK");

        var size = new COORD { X = 120, Y = 30 };
        IntPtr hPC;
        int hr = CreatePseudoConsole(size, inputReadHandle, outputWriteHandle, 0, out hPC);
        if (hr != 0)
        { Console.WriteLine("CreatePseudoConsole FAIL: 0x" + hr.ToString("X")); return; }
        Console.WriteLine("PseudoConsole created OK, hPC=0x" + hPC.ToString("X"));

        CloseHandle(inputReadHandle);
        CloseHandle(outputWriteHandle);

        // Setup STARTUPINFOEX
        var si = new SIEX();
        si.StartupInfo.cb = Marshal.SizeOf(typeof(SIEX));
        Console.WriteLine("SIEX size = " + si.StartupInfo.cb);

        IntPtr attrSz = IntPtr.Zero;
        InitializeProcThreadAttributeList(IntPtr.Zero, 1, 0, ref attrSz);
        Console.WriteLine("AttrList size = " + attrSz.ToInt64());
        si.lpAttributeList = Marshal.AllocHGlobal(attrSz.ToInt32());
        if (!InitializeProcThreadAttributeList(si.lpAttributeList, 1, 0, ref attrSz))
        { Console.WriteLine("InitializeAttrList FAIL: " + Marshal.GetLastWin32Error()); return; }

        if (!UpdateProcThreadAttribute(si.lpAttributeList, 0, PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
                hPC, (IntPtr)IntPtr.Size, IntPtr.Zero, IntPtr.Zero))
        { Console.WriteLine("UpdateAttr FAIL: " + Marshal.GetLastWin32Error()); return; }
        Console.WriteLine("Attributes set OK");

        PI pi;
        string cmd = "powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass";
        if (!CreateProcessW(null, cmd, IntPtr.Zero, IntPtr.Zero, false,
                EXTENDED_STARTUPINFO_PRESENT, IntPtr.Zero, null, ref si, out pi))
        { Console.WriteLine("CreateProcess FAIL: " + Marshal.GetLastWin32Error()); return; }

        Console.WriteLine("Process created OK, PID=" + pi.dwProcessId);
        CloseHandle(pi.hThread);
        DeleteProcThreadAttributeList(si.lpAttributeList);
        Marshal.FreeHGlobal(si.lpAttributeList);

        // Read output in background
        ThreadPool.QueueUserWorkItem(_ =>
        {
            var buf = new byte[4096];
            while (true)
            {
                int n;
                if (!ReadFile(outputReadHandle, buf, buf.Length, out n, IntPtr.Zero) || n <= 0) break;
                var text = Encoding.UTF8.GetString(buf, 0, n);
                Console.Write("[OUT] " + text.Replace("\x1b", "ESC"));
            }
            Console.WriteLine("\n[ReadLoop ended]");
        });

        Thread.Sleep(2000); // Wait for prompt

        // Test: write "dir\r\n"
        Console.WriteLine("\n--- Sending 'dir' + Enter ---");
        byte[] input = Encoding.UTF8.GetBytes("dir\r");
        int written;
        bool wOk = WriteFile(inputWriteHandle, input, input.Length, out written, IntPtr.Zero);
        Console.WriteLine("WriteFile result=" + wOk + " written=" + written + " err=" + Marshal.GetLastWin32Error());

        Thread.Sleep(3000); // Wait for output

        // Test: write single chars
        Console.WriteLine("\n--- Sending 'a' 'b' 'c' one at a time ---");
        foreach (char c in "abc")
        {
            byte[] ch = Encoding.UTF8.GetBytes(c.ToString());
            wOk = WriteFile(inputWriteHandle, ch, ch.Length, out written, IntPtr.Zero);
            Console.WriteLine("Write '" + c + "' result=" + wOk + " written=" + written);
            Thread.Sleep(500);
        }

        Thread.Sleep(2000);
        Console.WriteLine("\n--- Test done ---");

        // Cleanup
        CloseHandle(inputWriteHandle);
        ClosePseudoConsole(hPC);
        CloseHandle(pi.hProcess);
        CloseHandle(outputReadHandle);
    }
}
