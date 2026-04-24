//go:build windows

package main

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// ─── Windows API DLL / Proc ─────────────────────────────────────────

var (
	modKernel32Metrics = syscall.NewLazyDLL("kernel32.dll")
	modIphlpapi        = syscall.NewLazyDLL("iphlpapi.dll")
	modPsapi           = syscall.NewLazyDLL("psapi.dll")

	procGetSystemTimes          = modKernel32Metrics.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx    = modKernel32Metrics.NewProc("GlobalMemoryStatusEx")
	procGetLogicalDriveStringsW = modKernel32Metrics.NewProc("GetLogicalDriveStringsW")
	procGetDriveTypeW           = modKernel32Metrics.NewProc("GetDriveTypeW")
	procGetDiskFreeSpaceExW     = modKernel32Metrics.NewProc("GetDiskFreeSpaceExW")
	procGetTickCount64          = modKernel32Metrics.NewProc("GetTickCount64")
	procGetIfTable2             = modIphlpapi.NewProc("GetIfTable2")
	procFreeMibTable            = modIphlpapi.NewProc("FreeMibTable")
	procEnumProcesses           = modPsapi.NewProc("EnumProcesses")
)

// ─── CPU Usage (GetSystemTimes 差值法) ──────────────────────────────

var (
	cpuMu      sync.Mutex
	prevIdle   uint64
	prevKernel uint64
	prevUser   uint64
	cpuInited  bool
)

func fileTimeToUint64(ft syscall.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}

func getCPUUsage() (float64, error) {
	var idleFt, kernelFt, userFt syscall.Filetime
	r, _, e := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleFt)),
		uintptr(unsafe.Pointer(&kernelFt)),
		uintptr(unsafe.Pointer(&userFt)),
	)
	if r == 0 {
		return 0, fmt.Errorf("GetSystemTimes: %v", e)
	}

	idle := fileTimeToUint64(idleFt)
	kernel := fileTimeToUint64(kernelFt)
	user := fileTimeToUint64(userFt)

	cpuMu.Lock()
	defer cpuMu.Unlock()

	if !cpuInited {
		prevIdle, prevKernel, prevUser = idle, kernel, user
		cpuInited = true
		time.Sleep(500 * time.Millisecond)
		// 再采一次
		procGetSystemTimes.Call(
			uintptr(unsafe.Pointer(&idleFt)),
			uintptr(unsafe.Pointer(&kernelFt)),
			uintptr(unsafe.Pointer(&userFt)),
		)
		idle = fileTimeToUint64(idleFt)
		kernel = fileTimeToUint64(kernelFt)
		user = fileTimeToUint64(userFt)
	}

	dIdle := idle - prevIdle
	dKernel := kernel - prevKernel
	dUser := user - prevUser
	prevIdle, prevKernel, prevUser = idle, kernel, user

	totalSys := dKernel + dUser // kernel 已包含 idle
	if totalSys == 0 {
		return 0, nil
	}
	// kernel time 包含 idle time，实际忙碌 = kernel - idle + user
	busy := totalSys - dIdle
	pct := float64(busy) * 100 / float64(totalSys)
	return round2(pct), nil
}

// ─── Memory (GlobalMemoryStatusEx) ──────────────────────────────────

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func getMemory() (total, used int64, usage float64) {
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if r == 0 {
		return
	}
	total = int64(ms.TotalPhys / (1024 * 1024)) // MB
	avail := int64(ms.AvailPhys / (1024 * 1024))
	used = total - avail
	if total > 0 {
		usage = round2(float64(used) * 100 / float64(total))
	}
	return
}

// ─── Disk (GetLogicalDriveStrings + GetDiskFreeSpaceEx) ─────────────

const _DRIVE_FIXED = 3

func getDisk() (total, used int64, usage float64) {
	// 获取盘符列表
	buf := make([]uint16, 256)
	r, _, _ := procGetLogicalDriveStringsW.Call(
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if r == 0 {
		return
	}

	// buf 格式: "C:\\\0D:\\\0\0"
	for i := 0; i < int(r); {
		// 找当前字符串结尾
		j := i
		for j < int(r) && buf[j] != 0 {
			j++
		}
		if j == i {
			break
		}
		drive := syscall.UTF16ToString(buf[i:j])
		i = j + 1

		// 只统计固定磁盘
		drivePtr, _ := syscall.UTF16PtrFromString(drive)
		dt, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(drivePtr)))
		if dt != _DRIVE_FIXED {
			continue
		}

		var freeBytesAvail, totalBytes, totalFreeBytes uint64
		r2, _, _ := procGetDiskFreeSpaceExW.Call(
			uintptr(unsafe.Pointer(drivePtr)),
			uintptr(unsafe.Pointer(&freeBytesAvail)),
			uintptr(unsafe.Pointer(&totalBytes)),
			uintptr(unsafe.Pointer(&totalFreeBytes)),
		)
		if r2 == 0 {
			continue
		}
		gb := int64(totalBytes / (1024 * 1024 * 1024))
		freeGb := int64(totalFreeBytes / (1024 * 1024 * 1024))
		total += gb
		used += gb - freeGb
	}
	if total > 0 {
		usage = round2(float64(used) * 100 / float64(total))
	}
	return
}

// ─── Load Average (Windows 无此概念，用 CPU% 近似) ──────────────────

func getLoadAvg() (l1, l5, l15 float64) {
	cpu, err := getCPUUsage()
	if err != nil {
		return
	}
	l1 = round2(cpu / 100)
	l5 = l1
	l15 = l1
	return
}

// ─── Process Count (EnumProcesses) ──────────────────────────────────

func getProcessCount() int {
	var pids [4096]uint32
	var needed uint32
	r, _, _ := procEnumProcesses.Call(
		uintptr(unsafe.Pointer(&pids[0])),
		uintptr(uint32(len(pids))*4),
		uintptr(unsafe.Pointer(&needed)),
	)
	if r == 0 {
		return 0
	}
	return int(needed / 4)
}

// ─── Uptime (GetTickCount64) ────────────────────────────────────────

func getUptime() string {
	r, _, _ := procGetTickCount64.Call()
	ms := uint64(r)
	secs := int(ms / 1000)
	days := secs / 86400
	hours := (secs % 86400) / 3600
	return fmt.Sprintf("%d天%d小时", days, hours)
}

// ─── Network Traffic (GetIfTable2) ──────────────────────────────────

// MIB_IF_ROW2 精简版，只取需要的字段偏移
// 完整结构体很大(1352字节)，我们只读 InOctets / OutOctets
const _sizeofMibIfRow2 = 1352

var (
	netMu       sync.Mutex
	prevRxTotal uint64
	prevTxTotal uint64
	prevNetTime time.Time
	netInited   bool
)

func readIfTable() (rx, tx uint64) {
	var pTable uintptr
	r, _, _ := procGetIfTable2.Call(uintptr(unsafe.Pointer(&pTable)))
	if r != 0 || pTable == 0 {
		return
	}
	defer procFreeMibTable.Call(pTable)

	numEntries := *(*uint64)(unsafe.Pointer(pTable))
	rowsBase := pTable + 8 // 跳过 NumEntries (ULONG64 on x64)

	for i := uint64(0); i < numEntries; i++ {
		rowPtr := rowsBase + uintptr(i)*_sizeofMibIfRow2

		// Type 在偏移 168 (IFTYPE, ULONG)
		ifType := *(*uint32)(unsafe.Pointer(rowPtr + 168))
		// 跳过 loopback (type 24) 和 tunnel (type 131)
		if ifType == 24 || ifType == 131 {
			continue
		}

		// InOctets 在偏移 1288 (ULONG64)
		inOctets := *(*uint64)(unsafe.Pointer(rowPtr + 1288))
		// OutOctets 在偏移 1296 (ULONG64)
		outOctets := *(*uint64)(unsafe.Pointer(rowPtr + 1296))

		rx += inOctets
		tx += outOctets
	}
	return
}

func getNetTraffic() (rxPerSec, txPerSec int64) {
	rxNow, txNow := readIfTable()

	netMu.Lock()
	defer netMu.Unlock()

	now := time.Now()
	if !netInited {
		prevRxTotal, prevTxTotal = rxNow, txNow
		prevNetTime = now
		netInited = true
		// 首次需要等一小段再采
		time.Sleep(time.Second)
		rxNow, txNow = readIfTable()
		now = time.Now()
	}

	elapsed := now.Sub(prevNetTime).Seconds()
	if elapsed < 0.5 {
		elapsed = 1
	}

	if rxNow >= prevRxTotal {
		rxPerSec = int64(float64(rxNow-prevRxTotal) / elapsed)
	}
	if txNow >= prevTxTotal {
		txPerSec = int64(float64(txNow-prevTxTotal) / elapsed)
	}

	prevRxTotal, prevTxTotal = rxNow, txNow
	prevNetTime = now
	return
}
