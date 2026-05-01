# Deep search for xwechat databases and data
$xwRoot = "$env:APPDATA\Tencent\xwechat"

Write-Host "=== xwechat radium structure ==="
if (Test-Path "$xwRoot\radium") {
    Get-ChildItem "$xwRoot\radium" -Directory | ForEach-Object {
        Write-Host ("  [{0}]" -f $_.Name)
        Get-ChildItem $_.FullName -Directory -ErrorAction SilentlyContinue | ForEach-Object {
            Write-Host ("    [{0}]" -f $_.Name)
        }
    }
}

# Search ALL .db files under xwechat
Write-Host "`n=== All .db files under xwechat ==="
Get-ChildItem $xwRoot -Filter "*.db" -Recurse -ErrorAction SilentlyContinue | Where-Object { $_.Length -gt 1024 } | ForEach-Object {
    $rel = $_.FullName.Replace($xwRoot, "")
    $hdr = [byte[]]::new(16)
    try {
        $fs = [IO.File]::Open($_.FullName, 'Open', 'Read', 'ReadWrite,Delete')
        $fs.Read($hdr, 0, 16) | Out-Null
        $fs.Close()
        $hex = ($hdr[0..7] | ForEach-Object { '{0:X2}' -f $_ }) -join ' '
        $magic = [Text.Encoding]::ASCII.GetString($hdr, 0, [Math]::Min(6, $hdr.Length))
    } catch { $hex = "ERROR"; $magic = "?" }
    Write-Host ("{0,-80} {1,10}KB  hdr={2}  magic={3}" -f $rel, [math]::Round($_.Length/1024,1), $hex, $magic)
}

# Search for msg/chat related files
Write-Host "`n=== Search for msg/chat/contact files ==="
Get-ChildItem $xwRoot -Recurse -ErrorAction SilentlyContinue | Where-Object {
    $_.Name -match "msg|chat|contact|session|friend|EnMicroMsg|MediaMsg" -and $_.Length -gt 1024 -and -not $_.PSIsContainer
} | Select-Object -First 30 | ForEach-Object {
    Write-Host ("{0,-80} {1}KB" -f $_.FullName.Replace($xwRoot, ""), [math]::Round($_.Length/1024,1))
}

# Also check Weixin install directory
Write-Host "`n=== Weixin install directory ==="
if (Test-Path "D:\Weixin") {
    Get-ChildItem "D:\Weixin" -Directory | ForEach-Object { Write-Host ("  [{0}]" -f $_.Name) }
    # Check for data directories
    Get-ChildItem "D:\Weixin" -Filter "*.dll" -ErrorAction SilentlyContinue | Select-Object -First 10 | ForEach-Object {
        Write-Host ("  {0} ({1}KB)" -f $_.Name, [math]::Round($_.Length/1024,1))
    }
}

# Check if there's a Weixin data directory on D: drive
Write-Host "`n=== Search D: drive for Weixin data ==="
$dPaths = @("D:\Weixin Files", "D:\xwechat_files", "D:\WeChat Files", "D:\Tencent Files")
foreach ($dp in $dPaths) {
    if (Test-Path $dp) { Write-Host "  [FOUND] $dp" }
}

# Check Weixin config for data path
Write-Host "`n=== Weixin config files ==="
Get-ChildItem "$xwRoot\config" -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host ("  {0} ({1}B)" -f $_.Name, $_.Length)
    if ($_.Length -lt 5000 -and $_.Length -gt 0) {
        try {
            $content = Get-Content $_.FullName -Raw -ErrorAction Stop
            Write-Host ("    Content: {0}" -f ($content.Substring(0, [Math]::Min(500, $content.Length))))
        } catch {
            $raw = [IO.File]::ReadAllBytes($_.FullName)
            $hex = ($raw[0..([Math]::Min(63, $raw.Length-1))] | ForEach-Object { '{0:X2}' -f $_ }) -join ' '
            Write-Host ("    Hex: {0}" -f $hex)
        }
    }
}

# Check login directory
Write-Host "`n=== Weixin login info ==="
if (Test-Path "$xwRoot\login") {
    Get-ChildItem "$xwRoot\login" -Recurse -ErrorAction SilentlyContinue | Where-Object { -not $_.PSIsContainer } | ForEach-Object {
        Write-Host ("  {0} ({1}B)" -f $_.FullName.Replace($xwRoot, ""), $_.Length)
    }
}

# Check Weixin process modules for WeChatWin.dll equivalent
Write-Host "`n=== Weixin process modules ==="
$wxProc = Get-Process -Name "Weixin" -ErrorAction SilentlyContinue | Select-Object -First 1
if ($wxProc) {
    try {
        $wxProc.Modules | Where-Object { $_.ModuleName -match "wechat|weixin|xwechat|radium|sqlite|cipher" } | ForEach-Object {
            Write-Host ("  {0} base=0x{1:X} size={2}KB" -f $_.ModuleName, $_.BaseAddress.ToInt64(), [math]::Round($_.ModuleMemorySize/1024,1))
        }
        # Also check for large modules (potential key containers)
        Write-Host "`n  Large modules (>10MB):"
        $wxProc.Modules | Where-Object { $_.ModuleMemorySize -gt 10*1024*1024 } | ForEach-Object {
            Write-Host ("  {0} base=0x{1:X} size={2}MB" -f $_.ModuleName, $_.BaseAddress.ToInt64(), [math]::Round($_.ModuleMemorySize/1024/1024,1))
        }
    } catch {
        Write-Host "  Cannot access modules: $_"
    }
}
