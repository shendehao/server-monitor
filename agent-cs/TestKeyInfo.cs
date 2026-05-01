using System;
using System.IO;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;

class TestKeyInfo
{
    [DllImport("crypt32.dll", SetLastError = true)]
    static extern bool CryptUnprotectData(ref DATA_BLOB pDataIn, IntPtr ppszDesc, IntPtr pOptionalEntropy, IntPtr pvReserved, IntPtr pPromptStruct, int dwFlags, ref DATA_BLOB pDataOut);
    [DllImport("kernel32.dll")] static extern IntPtr LocalFree(IntPtr hMem);
    [StructLayout(LayoutKind.Sequential)]
    struct DATA_BLOB { public int cbData; public IntPtr pbData; }

    static byte[] DPAPIDecrypt(byte[] data)
    {
        var din = new DATA_BLOB();
        din.cbData = data.Length;
        din.pbData = Marshal.AllocHGlobal(data.Length);
        Marshal.Copy(data, 0, din.pbData, data.Length);
        var dout = new DATA_BLOB();
        try
        {
            if (CryptUnprotectData(ref din, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, 0, ref dout))
            {
                byte[] result = new byte[dout.cbData];
                Marshal.Copy(dout.pbData, result, 0, dout.cbData);
                LocalFree(dout.pbData);
                return result;
            }
        }
        finally { Marshal.FreeHGlobal(din.pbData); }
        return null;
    }

    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        string loginDir = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), @"Tencent\xwechat\login");
        if (!Directory.Exists(loginDir)) { Console.WriteLine("未找到 xwechat login 目录"); return; }
        
        foreach (var wxDir in Directory.GetDirectories(loginDir))
        {
            string wxid = Path.GetFileName(wxDir);
            string keyFile = Path.Combine(wxDir, "key_info.dat");
            if (!File.Exists(keyFile)) { Console.WriteLine("{0}: key_info.dat 不存在", wxid); continue; }
            
            byte[] raw = File.ReadAllBytes(keyFile);
            Console.WriteLine("\n=== {0} ===", wxid);
            Console.WriteLine("文件大小: {0}", raw.Length);
            Console.WriteLine("全部Hex: {0}", BitConverter.ToString(raw).Replace("-",""));
            
            // 分析 protobuf 结构
            Console.WriteLine("\n--- Protobuf 分析 ---");
            int pos = 0;
            while (pos < raw.Length)
            {
                if (pos >= raw.Length) break;
                int tag = raw[pos];
                int fieldNum = tag >> 3;
                int wireType = tag & 7;
                pos++;
                Console.Write("  field={0} wireType={1} @{2}: ", fieldNum, wireType, pos - 1);
                
                if (wireType == 0) // varint
                {
                    long val = 0; int shift = 0;
                    while (pos < raw.Length) { byte b = raw[pos++]; val |= (long)(b & 0x7F) << shift; shift += 7; if ((b & 0x80) == 0) break; }
                    Console.WriteLine("varint={0}", val);
                }
                else if (wireType == 2) // length-delimited
                {
                    long len = 0; int shift = 0;
                    while (pos < raw.Length) { byte b = raw[pos++]; len |= (long)(b & 0x7F) << shift; shift += 7; if ((b & 0x80) == 0) break; }
                    Console.WriteLine("bytes len={0}", len);
                    if (len > 0 && pos + len <= raw.Length)
                    {
                        byte[] payload = new byte[len];
                        Array.Copy(raw, pos, payload, 0, (int)len);
                        Console.WriteLine("    Hex: {0}{1}", BitConverter.ToString(payload, 0, Math.Min(48, (int)len)).Replace("-",""), len > 48 ? "..." : "");
                        
                        // 尝试 DPAPI 解密
                        byte[] dec = DPAPIDecrypt(payload);
                        if (dec != null)
                        {
                            Console.WriteLine("    *** DPAPI 解密成功! 长度={0} ***", dec.Length);
                            Console.WriteLine("    解密Hex: {0}", BitConverter.ToString(dec).Replace("-",""));
                        }
                        else
                        {
                            Console.WriteLine("    DPAPI 解密失败");
                        }
                    }
                    pos += (int)len;
                }
                else
                {
                    Console.WriteLine("未知wireType, 停止");
                    break;
                }
            }
            
            // 暴力：尝试从不同偏移开始 DPAPI 解密
            Console.WriteLine("\n--- 暴力 DPAPI 尝试 ---");
            for (int offset = 0; offset < raw.Length - 32; offset++)
            {
                for (int tryLen = raw.Length - offset; tryLen >= 32; tryLen--)
                {
                    byte[] slice = new byte[tryLen];
                    Array.Copy(raw, offset, slice, 0, tryLen);
                    byte[] dec = DPAPIDecrypt(slice);
                    if (dec != null && dec.Length >= 16 && dec.Length <= 64)
                    {
                        Console.WriteLine("  偏移={0} 长度={1} => 解密成功! 结果={2}字节", offset, tryLen, dec.Length);
                        Console.WriteLine("  Key: {0}", BitConverter.ToString(dec).Replace("-",""));
                        break;
                    }
                }
            }
        }
    }
}
