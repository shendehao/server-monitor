using System;
using System.Collections.Generic;
using System.IO;
using System.Security.Cryptography;
using System.Text;

/// 测试 NTQQ 数据库解密 — 从头部提取密钥，尝试多种解密方式
class TestQQDecrypt
{
    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        string path = @"D:\QQ\Tencent Files\3371574658\nt_qq\nt_db\nt_msg.db";
        string tmp = Path.Combine(Path.GetTempPath(), "ntqq_test.db");
        File.Copy(path, tmp, true);
        byte[] data = File.ReadAllBytes(tmp);
        File.Delete(tmp);

        Console.WriteLine("=== NTQQ 解密测试 ===");
        Console.WriteLine("文件大小: " + data.Length + " bytes");

        // 1. 提取头部信息
        Console.WriteLine("\n[1] 头部分析:");
        string headerMagic = Encoding.ASCII.GetString(data, 0, 15);
        Console.WriteLine("Magic: \"" + headerMagic + "\"");

        int pageSizeRaw = (data[16] << 8) | data[17];
        Console.WriteLine("Page size (raw): " + pageSizeRaw);

        string qqMarker = Encoding.ASCII.GetString(data, 32, 8);
        Console.WriteLine("QQ Marker: \"" + qqMarker + "\"");

        // 提取 hex key 字符串
        int hexStart = 47;
        StringBuilder hexSb = new StringBuilder();
        for (int i = hexStart; i < data.Length && i < 300; i++)
        {
            char c = (char)data[i];
            if ((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F'))
                hexSb.Append(c);
            else break;
        }
        string hexKeyStr = hexSb.ToString();
        Console.WriteLine("Hex key string: " + hexKeyStr);
        Console.WriteLine("Hex key length: " + hexKeyStr.Length + " chars = " + hexKeyStr.Length / 2 + " bytes");

        byte[] fullKey = HexToBytes(hexKeyStr);
        byte[] key32 = new byte[32];
        Array.Copy(fullKey, 0, key32, 0, Math.Min(32, fullKey.Length));
        byte[] key32_second = new byte[32];
        if (fullKey.Length >= 64)
            Array.Copy(fullKey, 32, key32_second, 0, 32);

        Console.WriteLine("First 32 bytes: " + BytesToHex(key32));
        Console.WriteLine("Second 32 bytes: " + BytesToHex(key32_second));

        // 查找更多头部信息
        Console.WriteLine("\n[2] 扫描头部额外信息 (offset 175-1024):");
        for (int i = 175; i < Math.Min(1024, data.Length); i++)
        {
            // 找可打印字符串
            if (data[i] >= 0x20 && data[i] <= 0x7E)
            {
                int end = i;
                while (end < Math.Min(i + 200, data.Length) && data[end] >= 0x20 && data[end] <= 0x7E) end++;
                if (end - i >= 4)
                {
                    string s = Encoding.ASCII.GetString(data, i, end - i);
                    Console.WriteLine("  Offset " + i + ": \"" + s + "\"");
                    i = end;
                }
            }
        }

        // 3. 尝试多种解密方式
        int pageSize = pageSizeRaw > 0 ? pageSizeRaw : 4096;
        Console.WriteLine("\n[3] 尝试解密 (page size=" + pageSize + "):");

        // 方法 A: 直接用 first 32 bytes 做 AES-CBC key, 无 KDF
        // IV 取自页面末尾的 reserved bytes
        int[] reserveSizes = { 48, 32, 16, 0 };
        foreach (int reserve in reserveSizes)
        {
            Console.WriteLine("\n  --- 方法 A: Raw AES-CBC, reserve=" + reserve + " ---");
            TryDecryptPage(data, pageSize, key32, reserve, 1, false); // page 2
        }

        // 方法 B: 直接用 second 32 bytes 做 AES-CBC key
        Console.WriteLine("\n  --- 方法 B: Second 32 bytes as AES key, reserve=48 ---");
        TryDecryptPage(data, pageSize, key32_second, 48, 1, false);

        // 方法 C: SQLCipher 3 style with PBKDF2-SHA1 but using embedded key as passphrase
        Console.WriteLine("\n  --- 方法 C: SQLCipher 3 (PBKDF2-SHA1, 64000 iter) ---");
        // 对第一页: salt = first 16 bytes of page
        TrySQLCipher3(data, pageSize, fullKey, 1);

        // 方法 D: SQLCipher 4 raw key mode (PRAGMA key = "x'...'")
        // In raw key mode, first 32 bytes = enc key, no KDF
        Console.WriteLine("\n  --- 方法 D: SQLCipher raw key mode ---");
        // 对于 raw key, salt is first 16 bytes of DB, no PBKDF2
        // enc key = first 32 bytes, hmac key = next 32 bytes
        TryRawKey(data, pageSize, key32, key32_second, 1);

        // 方法 E: 尝试 page 1 不是标准 SQLCipher (整个第一页都是自定义头部)
        // 真实数据可能从 page 2 开始，使用不同的 page size
        int[] altPageSizes = { 4096, 2048, 1024 };
        Console.WriteLine("\n  --- 方法 E: 自定义头部(1024字节) + 页面数据从 offset 1024 开始 ---");
        foreach (int aps in altPageSizes)
        {
            Console.WriteLine("    尝试 page size " + aps + ":");
            // 数据从 offset 1024 开始 (跳过自定义头部)
            // 第一个 "真实" 页的 salt = 前 16 字节
            if (1024 + aps <= data.Length)
            {
                byte[] salt = new byte[16];
                Array.Copy(data, 1024, salt, 0, 16);
                Console.WriteLine("    Salt: " + BitConverter.ToString(salt));

                foreach (int res in new int[] { 48, 32 })
                {
                    // Try raw key with this salt
                    byte[] iv = new byte[16];
                    if (1024 + aps - res >= 0)
                        Array.Copy(data, 1024 + aps - res, iv, 0, 16);
                    int encLen = aps - 16 - res;
                    if (encLen <= 0 || encLen % 16 != 0) continue;
                    byte[] enc = new byte[encLen];
                    Array.Copy(data, 1024 + 16, enc, 0, encLen);
                    byte[] dec = AesDecrypt(key32, iv, enc);
                    if (dec != null)
                    {
                        bool valid = CheckDecrypted(dec);
                        Console.WriteLine("    res=" + res + " dec[0]=0x" + dec[0].ToString("X2") + " valid=" + valid);
                        if (valid) Console.WriteLine("    [可能成功!]");
                    }
                }
            }
        }

        // 方法 F: XOR-based encryption (整个文件XOR)
        Console.WriteLine("\n  --- 方法 F: 尝试单字节 XOR ---");
        for (int xk = 1; xk <= 255; xk++)
        {
            byte b = (byte)(data[1024] ^ xk);
            if (b == 0x0D || b == 0x05) // valid page types
            {
                // 检查更多字节
                byte b1 = (byte)(data[1025] ^ xk);
                if (b1 == 0x00) // cell count high byte usually 0
                {
                    Console.WriteLine("  XOR 0x" + xk.ToString("X2") + ": byte[1024]=0x" + b.ToString("X2") + " byte[1025]=0x" + b1.ToString("X2"));
                    // Check more
                    byte[] sample = new byte[16];
                    for (int i = 0; i < 16; i++) sample[i] = (byte)(data[1024 + i] ^ xk);
                    Console.WriteLine("    First 16 XORed: " + BitConverter.ToString(sample));
                }
            }
        }

        // 方法 G: 试试把 hex key 当作 AES raw key, 无 salt/PBKDF2
        // 直接对 page 1 (offset 0+16) 或 page 2 (offset pageSize) 做 AES-CBC
        Console.WriteLine("\n  --- 方法 G: 从 wrapper.node 的角度考虑 ---");
        Console.WriteLine("  全部 64 字节密钥可能是: enc_key(32) + hmac_key(32)");
        Console.WriteLine("  尝试直接解密 page 2 (offset " + pageSize + "):");
        foreach (int res in new int[] { 48, 32, 16 })
        {
            if (pageSize <= res) continue;
            byte[] iv = new byte[16];
            Array.Copy(data, pageSize + pageSize - res, iv, 0, 16);
            int encLen = pageSize - res;
            if (encLen <= 0 || encLen % 16 != 0) { encLen = (encLen / 16) * 16; }
            if (encLen <= 0) continue;
            byte[] enc = new byte[encLen];
            Array.Copy(data, pageSize, enc, 0, encLen);
            byte[] dec = AesDecrypt(key32, iv, enc);
            if (dec != null)
            {
                bool valid = CheckDecrypted(dec);
                Console.WriteLine("  res=" + res + " IV=" + BitConverter.ToString(iv, 0, 4) + "... dec[0]=0x" + dec[0].ToString("X2") + " valid=" + valid);
            }

            // 也试试 fullKey 的前16字节做 IV
            byte[] fixedIv = new byte[16];
            Array.Copy(fullKey, 0, fixedIv, 0, 16);
            dec = AesDecrypt(key32_second, fixedIv, enc);
            if (dec != null)
            {
                bool valid = CheckDecrypted(dec);
                Console.WriteLine("  [alt] key=2nd32, IV=1st16 res=" + res + " dec[0]=0x" + dec[0].ToString("X2") + " valid=" + valid);
            }
        }

        Console.WriteLine("\n=== 测试完毕 ===");
    }

    static void TryDecryptPage(byte[] db, int pageSize, byte[] key, int reserve, int pageNum, bool isFirst)
    {
        int pgOff = pageNum * pageSize;
        if (pgOff + pageSize > db.Length) { Console.WriteLine("  页面超出文件范围"); return; }

        int dataStart = isFirst ? 16 : 0;
        int encLen = pageSize - dataStart - reserve;
        if (encLen <= 0 || encLen % 16 != 0) { Console.WriteLine("  encLen=" + encLen + " 不对齐"); return; }

        byte[] iv = new byte[16];
        if (reserve >= 16)
            Array.Copy(db, pgOff + pageSize - reserve, iv, 0, 16);

        byte[] enc = new byte[encLen];
        Array.Copy(db, pgOff + dataStart, enc, 0, encLen);
        byte[] dec = AesDecrypt(key, iv, enc);
        if (dec == null) { Console.WriteLine("  AES 解密失败"); return; }

        bool valid = CheckDecrypted(dec);
        Console.WriteLine("  dec[0]=0x" + dec[0].ToString("X2") + " dec[1]=0x" + dec[1].ToString("X2") + " valid=" + valid);
    }

    static void TrySQLCipher3(byte[] db, int pageSize, byte[] rawKey, int pageNum)
    {
        int pgOff = pageNum * pageSize;
        if (pgOff + pageSize > db.Length) return;

        byte[] salt = new byte[16];
        Array.Copy(db, 0, salt, 0, 16); // salt from first page

        try
        {
            byte[] encKey;
            using (var kdf = new Rfc2898DeriveBytes(rawKey, salt, 64000))
                encKey = kdf.GetBytes(32);
            byte[] hs = new byte[16];
            for (int i = 0; i < 16; i++) hs[i] = (byte)(salt[i] ^ 0x3a);
            byte[] hmacKey;
            using (var kdf = new Rfc2898DeriveBytes(encKey, hs, 2))
                hmacKey = kdf.GetBytes(32);

            int reserve = 48;
            byte[] iv = new byte[16];
            Array.Copy(db, pgOff + pageSize - reserve, iv, 0, 16);
            int encLen = pageSize - reserve;
            byte[] enc = new byte[encLen];
            Array.Copy(db, pgOff, enc, 0, encLen);
            byte[] dec = AesDecrypt(encKey, iv, enc);
            if (dec != null)
            {
                Console.WriteLine("  dec[0]=0x" + dec[0].ToString("X2") + " valid=" + CheckDecrypted(dec));
            }
            else Console.WriteLine("  AES 失败");
        }
        catch (Exception ex) { Console.WriteLine("  异常: " + ex.Message); }
    }

    static void TryRawKey(byte[] db, int pageSize, byte[] encKey, byte[] hmacKey, int pageNum)
    {
        int pgOff = pageNum * pageSize;
        if (pgOff + pageSize > db.Length) return;

        int reserve = 48;
        byte[] iv = new byte[16];
        Array.Copy(db, pgOff + pageSize - reserve, iv, 0, 16);

        int encLen = pageSize - reserve;
        if (encLen % 16 != 0) encLen = (encLen / 16) * 16;
        byte[] enc = new byte[encLen];
        Array.Copy(db, pgOff, enc, 0, encLen);

        byte[] dec = AesDecrypt(encKey, iv, enc);
        if (dec != null)
        {
            Console.WriteLine("  dec[0]=0x" + dec[0].ToString("X2") + " valid=" + CheckDecrypted(dec));

            // HMAC verify
            byte[] hmacInput = new byte[encLen + 16 + 4];
            Array.Copy(db, pgOff, hmacInput, 0, encLen);
            Array.Copy(db, pgOff + pageSize - reserve, hmacInput, encLen, 16);
            hmacInput[encLen + 16] = 0; hmacInput[encLen + 17] = 0;
            hmacInput[encLen + 18] = 0; hmacInput[encLen + 19] = (byte)(pageNum + 1);
            using (var hmac = new HMACSHA1(hmacKey))
            {
                byte[] computed = hmac.ComputeHash(hmacInput);
                byte[] stored = new byte[20];
                Array.Copy(db, pgOff + pageSize - reserve + 16, stored, 0, 20);
                bool hmacMatch = true;
                for (int i = 0; i < 20; i++) if (computed[i] != stored[i]) { hmacMatch = false; break; }
                Console.WriteLine("  HMAC match: " + hmacMatch);
                if (!hmacMatch)
                {
                    Console.WriteLine("  stored HMAC: " + BitConverter.ToString(stored, 0, 8) + "...");
                    Console.WriteLine("  computed:    " + BitConverter.ToString(computed, 0, 8) + "...");
                }
            }
        }
        else Console.WriteLine("  AES 失败");
    }

    static bool CheckDecrypted(byte[] dec)
    {
        if (dec == null || dec.Length == 0) return false;
        byte b = dec[0];
        // Valid B-tree page types
        return b == 0x0D || b == 0x05 || b == 0x0A || b == 0x02;
    }

    static byte[] AesDecrypt(byte[] key, byte[] iv, byte[] data)
    {
        try
        {
            using (var aes = Aes.Create())
            {
                aes.Mode = CipherMode.CBC;
                aes.Padding = PaddingMode.None;
                aes.Key = key;
                aes.IV = iv;
                using (var dec = aes.CreateDecryptor())
                    return dec.TransformFinalBlock(data, 0, data.Length);
            }
        }
        catch { return null; }
    }

    static byte[] HexToBytes(string hex)
    {
        byte[] result = new byte[hex.Length / 2];
        for (int i = 0; i < result.Length; i++)
            result[i] = Convert.ToByte(hex.Substring(i * 2, 2), 16);
        return result;
    }

    static string BytesToHex(byte[] b)
    {
        var sb = new StringBuilder(b.Length * 2);
        foreach (byte x in b) sb.Append(x.ToString("x2"));
        return sb.ToString();
    }
}
