using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Reflection;
using System.Text;

class TestWxKeyLocal
{
    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        Console.WriteLine("=== 微信密钥提取本地测试 ===\n");

        // 检查微信进程
        string[] names = { "Weixin", "WeChatAppEx", "WeChat" };
        foreach (var n in names)
        {
            var procs = Process.GetProcessesByName(n);
            Console.WriteLine("进程 {0}: {1}个", n, procs.Length);
            foreach (var p in procs)
            {
                try { Console.WriteLine("  PID={0} MainModule={1}", p.Id, p.MainModule.FileName); } catch (Exception ex) { Console.WriteLine("  PID={0} (无法读取模块: {1})", p.Id, ex.GetType().Name); }
            }
        }

        // 用反射调用 MiniAgent 的方法
        Console.WriteLine("\n--- 加载 MiniAgent.dll ---");
        var asm = Assembly.LoadFrom("MiniAgent.dll");
        var agentType = asm.GetType("MiniAgent.Agent");
        var agent = System.Runtime.Serialization.FormatterServices.GetUninitializedObject(agentType);

        // 调用 ScanProcessForRawKeys
        Console.WriteLine("\n--- ScanProcessForRawKeys ---");
        var scanMethod = agentType.GetMethod("ScanProcessForRawKeys", BindingFlags.NonPublic | BindingFlags.Instance);
        var sw = Stopwatch.StartNew();
        var rawKeys = scanMethod.Invoke(agent, new object[] { names }) as System.Collections.IList;
        sw.Stop();
        Console.WriteLine("耗时: {0}ms, 找到 {1} 个候选密钥", sw.ElapsedMilliseconds, rawKeys.Count);

        if (rawKeys.Count > 0)
        {
            foreach (var kv in rawKeys)
            {
                var keyProp = kv.GetType().GetProperty("Key");
                var valProp = kv.GetType().GetProperty("Value");
                byte[] key = keyProp.GetValue(kv, null) as byte[];
                byte[] salt = valProp.GetValue(kv, null) as byte[];
                Console.WriteLine("  Key: {0}...", BitConverter.ToString(key, 0, 8).Replace("-",""));
                Console.WriteLine("  Salt: {0}", BitConverter.ToString(salt).Replace("-",""));
            }
        }

        // 调用 DumpWeChatInfo
        Console.WriteLine("\n--- DumpWeChatInfo ---");
        var dumpMethod = agentType.GetMethod("DumpWeChatInfo", BindingFlags.NonPublic | BindingFlags.Instance);
        var sb = new StringBuilder();
        sw.Restart();
        try
        {
            dumpMethod.Invoke(agent, new object[] { sb, true });
            sw.Stop();
            Console.WriteLine("耗时: {0}ms, 输出: {1}字符", sw.ElapsedMilliseconds, sb.Length);
            // 打印前2000字符
            string output = sb.ToString();
            if (output.Length > 2000) output = output.Substring(0, 2000) + "...";
            Console.WriteLine(output);
        }
        catch (Exception ex)
        {
            sw.Stop();
            var inner = ex.InnerException ?? ex;
            Console.WriteLine("异常! {0}ms: {1}: {2}", sw.ElapsedMilliseconds, inner.GetType().Name, inner.Message);
            Console.WriteLine(inner.StackTrace);
        }

        Console.WriteLine("\n=== 完毕 ===");
    }
}
