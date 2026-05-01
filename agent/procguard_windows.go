//go:build windows

package main

import (
	"log"
	"syscall"
	"unsafe"
)

// ═══ 进程保护（ACL 模式，不会导致 BSOD）═══
// 通过修改进程安全描述符，拒绝所有人的 PROCESS_TERMINATE 权限
// 效果：taskkill / 任务管理器无法杀掉进程
// 优势：进程自己崩溃或正常退出不会蓝屏（区别于 RtlSetProcessIsCritical）

var (
	modAdvapi32              = syscall.NewLazyDLL("advapi32.dll")
	procSetKernelObjectSec   = modAdvapi32.NewProc("SetKernelObjectSecurity")
	procConvertStrSecDescToA = modAdvapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")

	modNtdll               = syscall.NewLazyDLL("ntdll.dll")
	procRtlAdjustPrivilege = modNtdll.NewProc("RtlAdjustPrivilege")
)

const (
	seDebugPrivilege = 20
	// DACL_SECURITY_INFORMATION
	daclSecInfo = 0x00000004
)

// enableProcessProtection 通过 ACL 保护进程不被外部终止（不会 BSOD）
func enableProcessProtection() {
	enableDebugPrivilege()

	// SDDL: D = DACL
	// (D;;0x0001;;WD) = Deny PROCESS_TERMINATE(0x0001) to Everyone(WD)
	// (A;;GA;;;SY)    = Allow GenericAll to SYSTEM
	// (A;;GA;;;BA)    = Allow GenericAll to Administrators (除了被 Deny 的 TERMINATE)
	// Deny ACE 优先于 Allow ACE，所以即使管理员也无法 TERMINATE
	sddl := "D:(D;;0x0001;;;WD)(A;;0x001FFFFF;;;SY)(A;;0x001FFFFE;;;BA)"
	sddlPtr, _ := syscall.UTF16PtrFromString(sddl)

	var sd uintptr
	var sdLen uint32
	r1, _, err := procConvertStrSecDescToA.Call(
		uintptr(unsafe.Pointer(sddlPtr)),
		1, // SDDL_REVISION_1
		uintptr(unsafe.Pointer(&sd)),
		uintptr(unsafe.Pointer(&sdLen)),
	)
	if r1 == 0 {
		log.Printf("[进程保护] 解析 SDDL 失败: %v", err)
		return
	}
	defer syscall.LocalFree(syscall.Handle(sd))

	// 获取当前进程句柄
	hProc := uintptr(^uintptr(0)) // pseudo handle = current process

	r2, _, err := procSetKernelObjectSec.Call(
		hProc,
		daclSecInfo,
		sd,
	)
	if r2 != 0 {
		log.Printf("[进程保护] ACL 保护已启用（taskkill/任务管理器无法终止，崩溃不蓝屏）")
	} else {
		log.Printf("[进程保护] 设置 ACL 失败: %v", err)
	}
}

// enableDebugPrivilege 通过 RtlAdjustPrivilege 提升 SeDebugPrivilege
func enableDebugPrivilege() bool {
	var wasEnabled uint32
	r, _, _ := procRtlAdjustPrivilege.Call(
		uintptr(seDebugPrivilege),
		1, // enable
		0, // for process (not thread)
		uintptr(unsafe.Pointer(&wasEnabled)),
	)
	return r == 0
}

// disableProcessProtection ACL 模式下无需特殊清理
func disableProcessProtection() {
	// ACL 保护在进程退出时自动失效，无需手动恢复
}
