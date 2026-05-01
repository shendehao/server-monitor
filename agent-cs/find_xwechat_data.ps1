# Check D:\weixinliaotian for chat databases
Write-Host "=== D:\weixinliaotian ==="
if (Test-Path "D:\weixinliaotian") {
    Get-ChildItem "D:\weixinliaotian" -Directory | ForEach-Object {
        Write-Host ("  [{0}]" -f $_.Name)
        Get-ChildItem $_.FullName -Directory -ErrorAction SilentlyContinue | ForEach-Object {
            Write-Host ("    [{0}]" -f $_.Name)
        }
    }
    
    # Find .db files
    Write-Host "`n=== Database files ==="
    Get-ChildItem "D:\weixinliaotian" -Filter "*.db" -Recurse -ErrorAction SilentlyContinue | Where-Object { $_.Length -gt 4096 } | ForEach-Object {
        $rel = $_.FullName.Replace("D:\weixinliaotian\", "")
        $hdr = [byte[]]::new(16)
        try {
            $fs = [IO.File]::Open($_.FullName, 'Open', 'Read', 'ReadWrite,Delete')
            $fs.Read($hdr, 0, 16) | Out-Null
            $fs.Close()
            $hex = ($hdr[0..7] | ForEach-Object { '{0:X2}' -f $_ }) -join ' '
            $isSqlite = [Text.Encoding]::ASCII.GetString($hdr, 0, 6) -eq 'SQLite'
        } catch { $hex = "LOCKED"; $isSqlite = "?" }
        Write-Host ("{0,-70} {1,10}KB  sqlite={2}  hdr={3}" -f $rel, [math]::Round($_.Length/1024,1), $isSqlite, $hex)
    }
    
    # Find EnMicroMsg.db or MicroMsg.db
    Write-Host "`n=== Key database search ==="
    Get-ChildItem "D:\weixinliaotian" -Recurse -ErrorAction SilentlyContinue | Where-Object {
        $_.Name -match "EnMicroMsg|MicroMsg|ChatMsg|MSG\d|contact|FTS.*db|MediaMsg|sns\.db|Favorite\.db" -and $_.Length -gt 1024
    } | Select-Object -First 20 | ForEach-Object {
        Write-Host ("  {0} ({1}KB)" -f $_.FullName.Replace("D:\weixinliaotian\",""), [math]::Round($_.Length/1024,1))
    }
} else {
    Write-Host "  NOT FOUND!"
}

# Read key_info.dat files
Write-Host "`n=== key_info.dat files ==="
$loginDir = "$env:APPDATA\Tencent\xwechat\login"
Get-ChildItem $loginDir -Directory -ErrorAction SilentlyContinue | ForEach-Object {
    $wxid = $_.Name
    $keyFile = Join-Path $_.FullName "key_info.dat"
    if (Test-Path $keyFile) {
        $raw = [IO.File]::ReadAllBytes($keyFile)
        Write-Host ("  wxid: {0}" -f $wxid)
        Write-Host ("  key_info.dat size: {0} bytes" -f $raw.Length)
        $hex = ($raw[0..([Math]::Min(63, $raw.Length-1))] | ForEach-Object { '{0:X2}' -f $_ }) -join ' '
        Write-Host ("  Hex (0-63): {0}" -f $hex)
        if ($raw.Length -gt 64) {
            $hex2 = ($raw[64..([Math]::Min(127, $raw.Length-1))] | ForEach-Object { '{0:X2}' -f $_ }) -join ' '
            Write-Host ("  Hex (64-127): {0}" -f $hex2)
        }
        if ($raw.Length -gt 128) {
            $hex3 = ($raw[128..([Math]::Min($raw.Length-1, 179))] | ForEach-Object { '{0:X2}' -f $_ }) -join ' '
            Write-Host ("  Hex (128-179): {0}" -f $hex3)
        }
        # Check printable
        $printable = ""
        for ($i = 0; $i -lt $raw.Length; $i++) {
            if ($raw[$i] -ge 0x20 -and $raw[$i] -le 0x7E) { $printable += [char]$raw[$i] }
            else { $printable += "." }
        }
        Write-Host ("  Printable: {0}" -f $printable)
    }
}
