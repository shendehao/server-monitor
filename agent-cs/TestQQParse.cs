using System;
using System.Collections.Generic;
using System.IO;
using System.Text;

/// 直接解析 NTQQ 明文 SQLite 数据库的表结构和消息
class TestQQParse
{
    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        string baseDir = @"D:\QQ\Tencent Files\3371574658\nt_qq\nt_db";

        // 1. 解析所有数据库的 sqlite_master
        string[] importantDbs = { "nt_msg.db", "recent_contact.db", "profile_info.db", "group_info.db" };
        foreach (string dbName in importantDbs)
        {
            string path = Path.Combine(baseDir, dbName);
            if (!File.Exists(path)) { Console.WriteLine("不存在: " + dbName); continue; }

            Console.WriteLine("\n══════ " + dbName + " (" + FormatSize(new FileInfo(path).Length) + ") ══════");
            byte[] data;
            try
            {
                // 复制避免锁定
                string tmp = Path.Combine(Path.GetTempPath(), "qqtest_" + dbName);
                File.Copy(path, tmp, true);
                data = File.ReadAllBytes(tmp);
                File.Delete(tmp);
            }
            catch (Exception ex) { Console.WriteLine("读取异常: " + ex.Message); continue; }

            if (data.Length < 100 || Encoding.ASCII.GetString(data, 0, 6) != "SQLite")
            { Console.WriteLine("不是 SQLite 文件"); continue; }

            int pageSize = (data[16] << 8) | data[17];
            if (pageSize == 1) pageSize = 65536;
            Console.WriteLine("Page size: " + pageSize + ", Total pages: " + data.Length / pageSize);

            // 解析 sqlite_master
            ParseSqliteMaster(data, pageSize, dbName);
        }
    }

    static void ParseSqliteMaster(byte[] data, int pageSize, string dbName)
    {
        int hdr = 100;
        if (hdr >= data.Length || data[hdr] != 0x0D)
        {
            // 可能是 interior page
            if (data[hdr] == 0x05)
            {
                Console.WriteLine("sqlite_master 是 interior page, 遍历子页...");
                // 获取子页列表
                int cellCount = (data[hdr + 3] << 8) | data[hdr + 4];
                long rightChild = ((long)data[hdr + 8] << 24) | ((long)data[hdr + 9] << 16) | ((long)data[hdr + 10] << 8) | data[hdr + 11];
                Console.WriteLine("  cells=" + cellCount + " rightChild=" + rightChild);
                var childPages = new List<int>();
                int ptrStart = hdr + 12;
                for (int c = 0; c < cellCount; c++)
                {
                    int ptrOff = ptrStart + c * 2;
                    if (ptrOff + 2 > data.Length) break;
                    int cellOff = (data[ptrOff] << 8) | data[ptrOff + 1];
                    if (cellOff + 4 > data.Length) continue;
                    long cp = ((long)data[cellOff] << 24) | ((long)data[cellOff + 1] << 16) | ((long)data[cellOff + 2] << 8) | data[cellOff + 3];
                    childPages.Add((int)cp);
                }
                childPages.Add((int)rightChild);
                foreach (int cp in childPages)
                {
                    if (cp < 1 || cp > data.Length / pageSize) continue;
                    int pgOff = (cp - 1) * pageSize;
                    if (pgOff >= data.Length) continue;
                    if (data[pgOff] == 0x0D)
                        ParseLeafMasterPage(data, pgOff, pageSize, dbName);
                }
            }
            else
            {
                Console.WriteLine("sqlite_master 页面类型异常: 0x" + data[hdr].ToString("X2"));
            }
            return;
        }
        ParseLeafMasterPage(data, 0, pageSize, dbName);
    }

    static void ParseLeafMasterPage(byte[] data, int pageOff, int pageSize, string dbName)
    {
        int hdr = pageOff + (pageOff == 0 ? 100 : 0);
        if (hdr >= data.Length || data[hdr] != 0x0D) return;

        int cellCount = (data[hdr + 3] << 8) | data[hdr + 4];
        int ptrStart = hdr + 8;

        for (int c = 0; c < cellCount && c < 200; c++)
        {
            int ptrOff = ptrStart + c * 2;
            if (ptrOff + 2 > data.Length) break;
            int cellOff = pageOff + ((data[ptrOff] << 8) | data[ptrOff + 1]);
            if (cellOff >= data.Length || cellOff < pageOff) continue;

            try
            {
                int p = cellOff; int n;
                long pLen; ReadVarint(data, p, out pLen, out n); p += n;
                long rid; ReadVarint(data, p, out rid, out n); p += n;
                long rhs; int hb; ReadVarint(data, p, out rhs, out hb);
                int rhe = p + (int)rhs; int hp = p + hb;
                var ct = new List<long>();
                while (hp < rhe && hp < data.Length) { long st; ReadVarint(data, hp, out st, out n); hp += n; ct.Add(st); }
                if (ct.Count < 5) continue;

                int dp = rhe; string type = null, name = null, sql = null; long rp = 0;
                for (int col = 0; col < ct.Count && dp < data.Length; col++)
                {
                    long st = ct[col]; int cl = ColSize(st); if (dp + cl > data.Length) break;
                    if ((col == 0 || col == 1 || col == 4) && st >= 13 && st % 2 == 1)
                    {
                        int tl = (int)(st - 13) / 2;
                        if (tl > 0 && dp + tl <= data.Length)
                        {
                            string v = Encoding.UTF8.GetString(data, dp, tl);
                            if (col == 0) type = v;
                            else if (col == 1) name = v;
                            else sql = v;
                        }
                    }
                    else if (col == 3) rp = ReadInt(data, dp, cl);
                    dp += cl;
                }

                Console.WriteLine("\n  " + (type ?? "?") + ": " + (name ?? "?") + " (rootpage=" + rp + ")");
                if (sql != null) Console.WriteLine("  SQL: " + sql);

                // 如果是 table 且是消息相关的，尝试读取前几条记录
                if (type == "table" && rp > 0 && dbName == "nt_msg.db" &&
                    (name == "c2c_msg_table" || name == "group_msg_table" ||
                     name == "c2c_msg" || name == "group_msg" ||
                     (name != null && name.Contains("msg"))))
                {
                    Console.WriteLine("  >>> 尝试读取前5条记录:");
                    ReadTableRows(data, (int)rp, pageSize, sql, 5);
                }
                else if (type == "table" && rp > 0 && dbName == "recent_contact.db" &&
                         name != null && name.Contains("recent"))
                {
                    Console.WriteLine("  >>> 尝试读取前5条记录:");
                    ReadTableRows(data, (int)rp, pageSize, sql, 5);
                }
                else if (type == "table" && rp > 0 && dbName == "profile_info.db" &&
                         name != null && (name.Contains("profile") || name.Contains("buddy")))
                {
                    Console.WriteLine("  >>> 尝试读取前5条记录:");
                    ReadTableRows(data, (int)rp, pageSize, sql, 5);
                }
                else if (type == "table" && rp > 0 && dbName == "group_info.db" &&
                         name != null && name.Contains("group"))
                {
                    Console.WriteLine("  >>> 尝试读取前3条记录:");
                    ReadTableRows(data, (int)rp, pageSize, sql, 3);
                }
            }
            catch { }
        }
    }

    static void ReadTableRows(byte[] data, int rootPage, int pageSize, string createSql, int maxRows)
    {
        // 从 CREATE SQL 提取列名
        var colNames = new List<string>();
        if (createSql != null)
        {
            int paren = createSql.IndexOf('(');
            if (paren > 0)
            {
                string inner = createSql.Substring(paren + 1);
                // 处理嵌套括号
                int depth = 0;
                var parts = new List<string>();
                var current = new StringBuilder();
                foreach (char ch in inner)
                {
                    if (ch == '(') depth++;
                    else if (ch == ')') { if (depth == 0) break; depth--; }
                    else if (ch == ',' && depth == 0) { parts.Add(current.ToString()); current.Clear(); continue; }
                    current.Append(ch);
                }
                if (current.Length > 0) parts.Add(current.ToString());

                foreach (string part in parts)
                {
                    string trimmed = part.Trim();
                    if (trimmed.StartsWith("PRIMARY") || trimmed.StartsWith("UNIQUE") ||
                        trimmed.StartsWith("CHECK") || trimmed.StartsWith("FOREIGN") ||
                        trimmed.StartsWith("CONSTRAINT")) continue;
                    string cn = trimmed.Split(new char[] { ' ', '\t', '\r', '\n' }, StringSplitOptions.RemoveEmptyEntries)[0];
                    cn = cn.Trim('"', '`', '[', ']');
                    colNames.Add(cn);
                }
            }
        }

        if (colNames.Count > 0)
            Console.WriteLine("    列: " + string.Join(", ", colNames));

        // 找叶子页
        var leaves = new List<int>();
        CollectLeaves(data, rootPage - 1, pageSize, data.Length / pageSize, leaves, 0);
        Console.WriteLine("    叶子页数: " + leaves.Count);

        int rowCount = 0;
        foreach (int pgIdx in leaves)
        {
            if (rowCount >= maxRows) break;
            int pgOff = pgIdx * pageSize;
            int hdr = pgOff;
            if (hdr >= data.Length || data[hdr] != 0x0D) continue;

            int cellCount = (data[hdr + 3] << 8) | data[hdr + 4];
            int ptrStart = hdr + 8;

            for (int c = 0; c < cellCount && rowCount < maxRows; c++)
            {
                int ptrOff = ptrStart + c * 2;
                if (ptrOff + 2 > data.Length) break;
                int cellOff = pgOff + ((data[ptrOff] << 8) | data[ptrOff + 1]);
                if (cellOff >= data.Length || cellOff < pgOff) continue;

                try
                {
                    int p = cellOff; int n;
                    long pLen; ReadVarint(data, p, out pLen, out n); p += n;
                    long rid; ReadVarint(data, p, out rid, out n); p += n;
                    if (pLen <= 0 || pLen > pageSize * 2) continue;

                    long rhs; int hb; ReadVarint(data, p, out rhs, out hb);
                    int rhe = p + (int)rhs; int hp = p + hb;
                    var ct = new List<long>();
                    while (hp < rhe && hp < data.Length) { long st; ReadVarint(data, hp, out st, out n); hp += n; ct.Add(st); }

                    int dp = rhe;
                    Console.Write("    Row#" + rowCount + " (rid=" + rid + ", " + ct.Count + " cols): ");
                    for (int col = 0; col < ct.Count && dp < data.Length; col++)
                    {
                        long st = ct[col]; int cl = ColSize(st);
                        if (dp + cl > data.Length) break;

                        string colName = col < colNames.Count ? colNames[col] : "c" + col;
                        string val;
                        if (st == 0) val = "NULL";
                        else if (st == 8) val = "0";
                        else if (st == 9) val = "1";
                        else if (st >= 1 && st <= 6) val = ReadInt(data, dp, cl).ToString();
                        else if (st == 7) val = "[float]";
                        else if (st >= 13 && st % 2 == 1) // text
                        {
                            int tl = (int)(st - 13) / 2;
                            if (tl > 200) val = "[text " + tl + " bytes]";
                            else if (tl > 0 && dp + tl <= data.Length) val = Encoding.UTF8.GetString(data, dp, tl);
                            else val = "";
                        }
                        else if (st >= 12 && st % 2 == 0) // blob
                        {
                            int bl = (int)(st - 12) / 2;
                            val = "[blob " + bl + " bytes]";
                        }
                        else val = "[?st=" + st + "]";

                        if (val.Length > 100) val = val.Substring(0, 100) + "...";
                        Console.Write(colName + "=" + val + " | ");
                        dp += cl;
                    }
                    Console.WriteLine();
                    rowCount++;
                }
                catch { }
            }
        }
    }

    static void CollectLeaves(byte[] data, int pgIdx, int pageSize, int totalPages, List<int> leaves, int depth)
    {
        if (depth > 20 || pgIdx < 0 || pgIdx >= totalPages || leaves.Count > 100) return;
        int off = pgIdx * pageSize;
        if (off >= data.Length) return;
        byte pt = data[off];
        if (pt == 0x0D) { leaves.Add(pgIdx); return; }
        if (pt != 0x05) return;
        int cellCount = (data[off + 3] << 8) | data[off + 4];
        long rc = ((long)data[off + 8] << 24) | ((long)data[off + 9] << 16) | ((long)data[off + 10] << 8) | data[off + 11];
        int ps = off + 12;
        for (int c = 0; c < cellCount && c < 500; c++)
        {
            int po = ps + c * 2;
            if (po + 2 > data.Length) break;
            int co = off + ((data[po] << 8) | data[po + 1]);
            if (co + 4 > data.Length || co < off) continue;
            long cp = ((long)data[co] << 24) | ((long)data[co + 1] << 16) | ((long)data[co + 2] << 8) | data[co + 3];
            if (cp > 0 && cp <= totalPages) CollectLeaves(data, (int)cp - 1, pageSize, totalPages, leaves, depth + 1);
        }
        if (rc > 0 && rc <= totalPages) CollectLeaves(data, (int)rc - 1, pageSize, totalPages, leaves, depth + 1);
    }

    static void ReadVarint(byte[] d, int p, out long v, out int n) { v = 0; n = 0; for (int i = 0; i < 9 && p + i < d.Length; i++) { v = (v << 7) | (long)(d[p + i] & 0x7F); n = i + 1; if ((d[p + i] & 0x80) == 0) return; } }
    static int ColSize(long st) { if (st == 0 || st == 8 || st == 9) return 0; if (st == 1) return 1; if (st == 2) return 2; if (st == 3) return 3; if (st == 4) return 4; if (st == 5) return 6; if (st == 6 || st == 7) return 8; if (st >= 12 && st % 2 == 0) return (int)(st - 12) / 2; if (st >= 13 && st % 2 == 1) return (int)(st - 13) / 2; return 0; }
    static long ReadInt(byte[] d, int o, int l) { long v = 0; for (int i = 0; i < l && o + i < d.Length; i++) v = (v << 8) | d[o + i]; return v; }
    static string FormatSize(long b) { if (b < 1024) return b + "B"; if (b < 1024 * 1024) return (b / 1024.0).ToString("F1") + "KB"; return (b / (1024.0 * 1024)).ToString("F1") + "MB"; }
}
