using System;
using System.IO;
using System.Reflection;
using System.Security.Cryptography;
using System.Text;

class TestDecryptWx4
{
    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        
        // 从 key_info.dat 提取密钥 (file offset 35-66, 32 bytes)
        string keyFile = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
            @"Tencent\xwechat\login\wxid_3lxrvh9517lg12\key_info.dat");
        byte[] keyData = File.ReadAllBytes(keyFile);
        byte[] rawKey = new byte[32];
        Array.Copy(keyData, 35, rawKey, 0, 32);
        Console.WriteLine("Raw Key: " + BitConverter.ToString(rawKey).Replace("-",""));
        
        // 读取加密数据库第一页
        string dbPath = @"D:\weixinliaotian\xwechat_files\wxid_3lxrvh9517lg12_7804\db_storage\contact\contact.db";
        byte[] dbData;
        using (var fs = new FileStream(dbPath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete))
        {
            dbData = new byte[fs.Length];
            fs.Read(dbData, 0, dbData.Length);
        }
        Console.WriteLine("DB 大小: {0} bytes", dbData.Length);
        Console.WriteLine("DB前16(salt): " + BitConverter.ToString(dbData, 0, 16).Replace("-",""));
        
        // 尝试不同 reserve size
        foreach (int reserveSize in new int[] { 80, 48 })
        {
            Console.WriteLine("\n=== reserve={0} ===", reserveSize);
            
            // 方法1: 直接用 rawKey 做 AES-CBC 解密 (raw key mode)
            Console.WriteLine("--- 方法1: 直接 AES-CBC (raw key) ---");
            TryRawKeyDecrypt(rawKey, dbData, reserveSize);
            
            // 方法2: PBKDF2-SHA512 派生 (SQLCipher 4)
            Console.WriteLine("--- 方法2: PBKDF2-SHA512 (SQLCipher 4) ---");
            TryPBKDF2Decrypt(rawKey, dbData, reserveSize, true);
            
            // 方法3: PBKDF2-SHA1 派生 (SQLCipher 3)  
            Console.WriteLine("--- 方法3: PBKDF2-SHA1 (SQLCipher 3) ---");
            TryPBKDF2Decrypt(rawKey, dbData, reserveSize, false);
        }
        
        // 方法4: rawKey 可能是密文, 需要用作 HMAC key 验证
        Console.WriteLine("\n=== 尝试 rawKey 作为 enc_key 直接解密第一页 ===");
        foreach (int res in new int[] { 80, 48 })
        {
            byte[] salt = new byte[16];
            Array.Copy(dbData, 0, salt, 0, 16);
            
            // 派生 HMAC key: PBKDF2(rawKey, salt^0x3a, 2, 32)
            byte[] hmacSalt = new byte[16];
            for (int i = 0; i < 16; i++) hmacSalt[i] = (byte)(salt[i] ^ 0x3a);
            
            // SHA512 (WeChat 4.x)
            try
            {
                byte[] hmacKey = PBKDF2_SHA512(rawKey, hmacSalt, 2, 32);
                
                // 验证 HMAC-SHA512
                int pageSize = 4096;
                int dataLen = pageSize - 16 - res;
                byte[] hmacInput = new byte[dataLen + 16 + 4];
                Array.Copy(dbData, 16, hmacInput, 0, dataLen);
                Array.Copy(dbData, pageSize - res, hmacInput, dataLen, 16);
                hmacInput[dataLen + 16] = 0; hmacInput[dataLen + 17] = 0;
                hmacInput[dataLen + 18] = 0; hmacInput[dataLen + 19] = 1;
                
                byte[] computed;
                using (var hmac = new HMACSHA512(hmacKey))
                    computed = hmac.ComputeHash(hmacInput);
                
                byte[] stored = new byte[64];
                Array.Copy(dbData, pageSize - res + 16, stored, 0, Math.Min(64, res - 16));
                
                bool match = true;
                int cmpLen = Math.Min(64, res - 16);
                for (int i = 0; i < cmpLen; i++)
                    if (stored[i] != computed[i]) { match = false; break; }
                
                Console.WriteLine("  reserve={0} SHA512 HMAC: {1}", res, match ? "MATCH!" : "不匹配");
                if (match)
                {
                    // 解密第一页
                    byte[] iv = new byte[16];
                    Array.Copy(dbData, pageSize - res, iv, 0, 16);
                    byte[] encData = new byte[dataLen];
                    Array.Copy(dbData, 16, encData, 0, dataLen);
                    
                    using (var aes = Aes.Create())
                    {
                        aes.KeySize = 256; aes.Mode = CipherMode.CBC; aes.Padding = PaddingMode.None;
                        aes.Key = rawKey; aes.IV = iv;
                        byte[] dec = aes.CreateDecryptor().TransformFinalBlock(encData, 0, encData.Length);
                        Console.WriteLine("  解密后前32: " + BitConverter.ToString(dec, 0, Math.Min(32, dec.Length)).Replace("-",""));
                        // 检查 byte 84 (=100-16) 是否为 0x0D 或 0x05
                        if (dec.Length > 84)
                            Console.WriteLine("  byte[84]=0x{0:X2} (期望0x0D或0x05): {1}", dec[84], (dec[84] == 0x0D || dec[84] == 0x05) ? "VALID!" : "invalid");
                    }
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine("  reserve={0} SHA512: 异常 {1}", res, ex.Message);
            }
            
            // SHA1
            try
            {
                byte[] hmacKeySha1;
                using (var kdf = new Rfc2898DeriveBytes(rawKey, hmacSalt, 2))
                    hmacKeySha1 = kdf.GetBytes(32);
                
                int pageSize = 4096;
                int dataLen = pageSize - 16 - res;
                byte[] hmacInput = new byte[dataLen + 16 + 4];
                Array.Copy(dbData, 16, hmacInput, 0, dataLen);
                Array.Copy(dbData, pageSize - res, hmacInput, dataLen, 16);
                hmacInput[dataLen + 16] = 0; hmacInput[dataLen + 17] = 0;
                hmacInput[dataLen + 18] = 0; hmacInput[dataLen + 19] = 1;
                
                byte[] computed;
                using (var hmac = new HMACSHA1(hmacKeySha1))
                    computed = hmac.ComputeHash(hmacInput);
                
                byte[] stored = new byte[20];
                Array.Copy(dbData, pageSize - res + 16, stored, 0, Math.Min(20, res - 16));
                
                bool match = true;
                for (int i = 0; i < 20; i++)
                    if (stored[i] != computed[i]) { match = false; break; }
                
                Console.WriteLine("  reserve={0} SHA1 HMAC: {1}", res, match ? "MATCH!" : "不匹配");
            }
            catch (Exception ex)
            {
                Console.WriteLine("  reserve={0} SHA1: 异常 {1}", res, ex.Message);
            }
        }
    }
    
    static void TryRawKeyDecrypt(byte[] key, byte[] dbData, int reserveSize)
    {
        try
        {
            int pageSize = 4096;
            byte[] salt = new byte[16];
            Array.Copy(dbData, 0, salt, 0, 16);
            byte[] iv = new byte[16];
            Array.Copy(dbData, pageSize - reserveSize, iv, 0, 16);
            int dataLen = pageSize - 16 - reserveSize;
            byte[] encData = new byte[dataLen];
            Array.Copy(dbData, 16, encData, 0, dataLen);
            
            using (var aes = Aes.Create())
            {
                aes.KeySize = 256; aes.Mode = CipherMode.CBC; aes.Padding = PaddingMode.None;
                aes.Key = key; aes.IV = iv;
                byte[] dec = aes.CreateDecryptor().TransformFinalBlock(encData, 0, encData.Length);
                Console.WriteLine("  解密后前16: " + BitConverter.ToString(dec, 0, 16).Replace("-",""));
                if (dec.Length > 84)
                    Console.WriteLine("  byte[84]=0x{0:X2} (期望0x0D/0x05): {1}", dec[84], (dec[84]==0x0D||dec[84]==0x05) ? "VALID!" : "invalid");
            }
        }
        catch (Exception ex) { Console.WriteLine("  异常: " + ex.Message); }
    }
    
    static void TryPBKDF2Decrypt(byte[] passphrase, byte[] dbData, int reserveSize, bool useSha512)
    {
        try
        {
            int pageSize = 4096;
            byte[] salt = new byte[16];
            Array.Copy(dbData, 0, salt, 0, 16);
            
            byte[] encKey;
            if (useSha512)
                encKey = PBKDF2_SHA512(passphrase, salt, 256000, 32);
            else
                using (var kdf = new Rfc2898DeriveBytes(passphrase, salt, 64000))
                    encKey = kdf.GetBytes(32);
            
            byte[] iv = new byte[16];
            Array.Copy(dbData, pageSize - reserveSize, iv, 0, 16);
            int dataLen = pageSize - 16 - reserveSize;
            byte[] encData = new byte[dataLen];
            Array.Copy(dbData, 16, encData, 0, dataLen);
            
            using (var aes = Aes.Create())
            {
                aes.KeySize = 256; aes.Mode = CipherMode.CBC; aes.Padding = PaddingMode.None;
                aes.Key = encKey; aes.IV = iv;
                byte[] dec = aes.CreateDecryptor().TransformFinalBlock(encData, 0, encData.Length);
                Console.WriteLine("  解密后前16: " + BitConverter.ToString(dec, 0, 16).Replace("-",""));
                if (dec.Length > 84)
                    Console.WriteLine("  byte[84]=0x{0:X2}: {1}", dec[84], (dec[84]==0x0D||dec[84]==0x05) ? "VALID!" : "invalid");
            }
        }
        catch (Exception ex) { Console.WriteLine("  异常: " + ex.Message); }
    }
    
    static byte[] PBKDF2_SHA512(byte[] password, byte[] salt, int iterations, int outputLen)
    {
        // 手动实现 PBKDF2-SHA512 (Rfc2898DeriveBytes 只支持 SHA1)
        byte[] result = new byte[outputLen];
        int blocks = (outputLen + 63) / 64;
        int offset = 0;
        for (int block = 1; block <= blocks; block++)
        {
            byte[] blockBytes = BitConverter.GetBytes(block);
            if (BitConverter.IsLittleEndian) Array.Reverse(blockBytes);
            byte[] saltBlock = new byte[salt.Length + 4];
            Array.Copy(salt, saltBlock, salt.Length);
            Array.Copy(blockBytes, 0, saltBlock, salt.Length, 4);
            
            byte[] u;
            using (var hmac = new HMACSHA512(password))
                u = hmac.ComputeHash(saltBlock);
            byte[] f = (byte[])u.Clone();
            for (int i = 1; i < iterations; i++)
            {
                using (var hmac = new HMACSHA512(password))
                    u = hmac.ComputeHash(u);
                for (int j = 0; j < f.Length; j++) f[j] ^= u[j];
            }
            int toCopy = Math.Min(64, outputLen - offset);
            Array.Copy(f, 0, result, offset, toCopy);
            offset += toCopy;
        }
        return result;
    }
}
