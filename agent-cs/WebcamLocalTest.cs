using System;
using System.IO;
using System.Drawing;
using System.Drawing.Imaging;
using System.Runtime.InteropServices;
using System.Threading;
using System.Reflection;

class WebcamLocalTest
{
    static void Main()
    {
        string dllPath = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "MiniAgent2.dll");
        string outDir = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "test_frames");
        if (Directory.Exists(outDir)) Directory.Delete(outDir, true);
        Directory.CreateDirectory(outDir);

        byte[] raw = File.ReadAllBytes(dllPath);
        var asm = Assembly.Load(raw);

        // 1) 测试 MFH264Encoder
        Console.WriteLine("=== MFH264Encoder Test ===");
        var encType = asm.GetType("MiniAgent.MFH264Encoder");
        var bf = BindingFlags.Instance | BindingFlags.NonPublic | BindingFlags.Public;
        var enc = encType.GetConstructors(bf)[0].Invoke(new object[] { 640, 480, 15, 500000 });
        Console.WriteLine("Encoder created OK");

        var encodeMethod = encType.GetMethod("Encode", bf);
        int w = 640, h = 480;
        int okFrames = 0, nullFrames = 0;

        for (int f = 0; f < 30; f++)
        {
            byte[] bgra = new byte[w * h * 4];
            for (int y = 0; y < h; y++)
                for (int x = 0; x < w; x++)
                {
                    int off = (y * w + x) * 4;
                    bgra[off] = (byte)((x + f * 10) & 0xFF);
                    bgra[off + 1] = (byte)((y + f * 5) & 0xFF);
                    bgra[off + 2] = (byte)((f * 30) & 0xFF);
                    bgra[off + 3] = 255;
                }

            object[] args = { bgra, w * 4, f == 0 };
            byte[] result = (byte[])encodeMethod.Invoke(enc, args);
            bool wasKey = (bool)args[2];

            if (result != null && result.Length > 0)
            {
                okFrames++;
                string tag = wasKey ? "KEY" : "P";
                File.WriteAllBytes(Path.Combine(outDir, string.Format("h264_{0:D3}.bin", f)), result);
                if (f < 5 || f % 10 == 0)
                    Console.WriteLine("  Frame {0}: {1} {2} bytes", f, tag, result.Length);
            }
            else
            {
                nullFrames++;
                if (f < 5) Console.WriteLine("  Frame {0}: NULL", f);
            }
        }
        Console.WriteLine("H264 Result: {0} output, {1} null, 30 total", okFrames, nullFrames);
        encType.GetMethod("Dispose", bf).Invoke(enc, null);

        // 2) 测试真实摄像头拍照 — 用 HandleWebcamSnap 的逻辑
        Console.WriteLine("\n=== Real Webcam Snapshot Test ===");
        var agentType = asm.GetType("MiniAgent.Agent");
        var agent = agentType.GetConstructors(bf)[0].Invoke(new object[] { "http://127.0.0.1:9999", "x", "", "t" });

        // 调用 HandleWebcamSnap 需要参数；直接用反射调用内部的 DirectShow 流程
        // 找到 WebcamStreamLoop 的 DirectShow 初始化部分
        // 更简单：用 webcam_snap 的 payload 触发拍照
        var snapMethod = agentType.GetMethod("HandleWebcamSnap", bf);
        if (snapMethod != null)
        {
            Console.WriteLine("Found HandleWebcamSnap, params: " + snapMethod.GetParameters().Length);
            // HandleWebcamSnap(string id, string payload) — 它内部会拍照并通过 WS 发送
            // 我们不能直接调用因为没有 WS 连接
        }

        // 用 webcam snap API 的核心逻辑：直接构建 DirectShow graph
        // DsGrabJpeg(IntPtr pGrabber, int w, int h, int quality, ImageCodecInfo jpgEnc, EncoderParameters ep)
        // pGrabber=IntPtr.Zero 表示用新的 grabber
        var grabMethod = agentType.GetMethod("DsGrabJpeg", bf);
        if (grabMethod != null)
        {
            Console.WriteLine("Calling DsGrabJpeg (6-param)...");
            // 获取 JPEG encoder
            ImageCodecInfo jpgEnc = null;
            foreach (var ci in ImageCodecInfo.GetImageEncoders())
                if (ci.MimeType == "image/jpeg") { jpgEnc = ci; break; }
            var ep = new EncoderParameters(1);
            ep.Param[0] = new EncoderParameter(System.Drawing.Imaging.Encoder.Quality, 75L);

            try
            {
                byte[] jpg = (byte[])grabMethod.Invoke(agent, new object[] { IntPtr.Zero, 640, 480, 75, jpgEnc, ep });
                if (jpg != null && jpg.Length > 0)
                {
                    string p = Path.Combine(outDir, "webcam_real.jpg");
                    File.WriteAllBytes(p, jpg);
                    Console.WriteLine("WEBCAM OK: {0} bytes -> {1}", jpg.Length, p);
                }
                else
                    Console.WriteLine("WEBCAM: returned null");
            }
            catch (Exception ex)
            {
                Console.WriteLine("WEBCAM ERROR: " + (ex.InnerException != null ? ex.InnerException.Message : ex.Message));
            }
        }
        else
        {
            Console.WriteLine("DsGrabJpeg not found!");
        }

        // 3) 连续拍 5 张验证画面变化
        Console.WriteLine("\n=== Multi-frame capture (5 frames, 1s apart) ===");
        for (int i = 0; i < 5; i++)
        {
            try
            {
                ImageCodecInfo jpgEnc2 = null;
                foreach (var ci in ImageCodecInfo.GetImageEncoders())
                    if (ci.MimeType == "image/jpeg") { jpgEnc2 = ci; break; }
                var ep2 = new EncoderParameters(1);
                ep2.Param[0] = new EncoderParameter(System.Drawing.Imaging.Encoder.Quality, 75L);

                byte[] jpg = (byte[])grabMethod.Invoke(agent, new object[] { IntPtr.Zero, 640, 480, 75, jpgEnc2, ep2 });
                if (jpg != null && jpg.Length > 0)
                {
                    string p = Path.Combine(outDir, string.Format("cam_{0}.jpg", i));
                    File.WriteAllBytes(p, jpg);
                    Console.WriteLine("  cam_{0}.jpg: {1} bytes", i, jpg.Length);
                }
                else
                    Console.WriteLine("  cam_{0}: null", i);
            }
            catch (Exception ex)
            {
                Console.WriteLine("  cam_{0} ERROR: {1}", i, ex.InnerException != null ? ex.InnerException.Message : ex.Message);
            }
            if (i < 4) Thread.Sleep(1000);
        }

        Console.WriteLine("\n=== Output files ===");
        foreach (var f in Directory.GetFiles(outDir))
            Console.WriteLine("  {0} - {1} bytes", Path.GetFileName(f), new FileInfo(f).Length);

        Console.WriteLine("\nDone! Check: " + outDir);
    }
}
