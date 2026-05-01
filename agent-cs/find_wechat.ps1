# Find WeChat processes
Write-Host "=== WeChat Processes ==="
$procs = Get-Process | Where-Object { $_.ProcessName -match "wechat|weixin|xwechat" }
foreach ($p in $procs) {
    try {
        Write-Host ("  PID={0} Name={1} Path={2}" -f $p.Id, $p.ProcessName, $p.MainModule.FileName)
    } catch {
        Write-Host ("  PID={0} Name={1} (no path)" -f $p.Id, $p.ProcessName)
    }
}
if (-not $procs) { Write-Host "  No WeChat processes found" }

# Search registry for WeChat
Write-Host "`n=== Registry ==="
$regPaths = @(
    "HKCU:\Software\Tencent\WeChat",
    "HKCU:\Software\Tencent\xwechat",
    "HKCU:\Software\Tencent\WXWork",
    "HKLM:\Software\Tencent\WeChat",
    "HKLM:\Software\WOW6432Node\Tencent\WeChat",
    "HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\WeChat",
    "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\WeChat"
)
foreach ($rp in $regPaths) {
    if (Test-Path $rp) {
        Write-Host "  $rp :"
        Get-ItemProperty $rp | Format-List | Out-String | Write-Host
    }
}

# Search for WeChat data directories
Write-Host "`n=== WeChat Data Directories ==="
$searchPaths = @(
    "$env:USERPROFILE\Documents\WeChat Files",
    "$env:USERPROFILE\Documents\xwechat_files",
    "$env:APPDATA\Tencent\WeChat",
    "$env:APPDATA\Tencent\xwechat",
    "$env:LOCALAPPDATA\Tencent\WeChat",
    "$env:APPDATA\WeChat",
    "$env:LOCALAPPDATA\WeChat",
    "$env:APPDATA\Tencent\WXWork"
)
# Also check all drives
$drives = Get-PSDrive -PSProvider FileSystem | Where-Object { $_.Used -gt 0 }
foreach ($drv in $drives) {
    $searchPaths += "$($drv.Root)WeChat Files"
    $searchPaths += "$($drv.Root)Tencent Files"
    $searchPaths += "$($drv.Root)xwechat_files"
}

foreach ($sp in $searchPaths) {
    if (Test-Path $sp) {
        Write-Host "  [FOUND] $sp"
        Get-ChildItem $sp -Directory -ErrorAction SilentlyContinue | ForEach-Object {
            Write-Host ("    [{0}]" -f $_.Name)
        }
    }
}

# Search for WeChatWin.dll or wechat.exe on all drives
Write-Host "`n=== Search for WeChat executables ==="
foreach ($drv in $drives) {
    $root = $drv.Root
    # Quick search in common locations
    $commonPaths = @(
        "${root}Program Files\Tencent\WeChat",
        "${root}Program Files (x86)\Tencent\WeChat",
        "${root}Tencent\WeChat",
        "${root}WeChat",
        "${root}Program Files\Tencent\xwechat",
        "${root}xwechat"
    )
    foreach ($cp in $commonPaths) {
        if (Test-Path $cp) {
            Write-Host "  [FOUND] $cp"
            Get-ChildItem $cp -Filter "*.exe" -ErrorAction SilentlyContinue | ForEach-Object {
                Write-Host ("    {0} ({1}KB)" -f $_.Name, [math]::Round($_.Length/1024,1))
            }
        }
    }
}

# Also check xwechat data (found in previous scan)
Write-Host "`n=== xwechat data ==="
$xwPath = "$env:APPDATA\Tencent\xwechat"
if (Test-Path $xwPath) {
    Write-Host "  Found: $xwPath"
    Get-ChildItem "$xwPath\radium\users" -Directory -ErrorAction SilentlyContinue | ForEach-Object {
        Write-Host ("    User: {0}" -f $_.Name)
        # Check for database files
        Get-ChildItem $_.FullName -Filter "*.db" -Recurse -ErrorAction SilentlyContinue | Where-Object { $_.Length -gt 4096 } | Select-Object -First 10 | ForEach-Object {
            $hdr = [byte[]]::new(6)
            try {
                $fs = [IO.File]::Open($_.FullName, 'Open', 'Read', 'ReadWrite,Delete')
                $fs.Read($hdr, 0, 6) | Out-Null
                $fs.Close()
                $isSqlite = [Text.Encoding]::ASCII.GetString($hdr) -eq 'SQLite'
            } catch { $isSqlite = "?" }
            Write-Host ("      {0} ({1}KB) sqlite={2}" -f $_.FullName.Replace($xwPath,""), [math]::Round($_.Length/1024,1), $isSqlite)
        }
    }
}
