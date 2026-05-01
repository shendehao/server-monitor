$dbDir = "D:\QQ\Tencent Files\3371574658\nt_qq\nt_db"
Write-Host "=== NTQQ Database Headers ==="
$dbs = Get-ChildItem -Path $dbDir -Filter "*.db" -ErrorAction SilentlyContinue
foreach ($f in $dbs) {
    $bytes = [byte[]]::new(16)
    try {
        $fs = [IO.File]::Open($f.FullName, 'Open', 'Read', 'ReadWrite,Delete')
        $fs.Read($bytes, 0, 16) | Out-Null
        $fs.Close()
        $hex = ($bytes[0..7] | ForEach-Object { '{0:X2}' -f $_ }) -join ' '
        $isSqlite = [Text.Encoding]::ASCII.GetString($bytes, 0, 6) -eq 'SQLite'
        Write-Host ("{0,-30} {1,10}KB  enc={2,-6} hdr={3}" -f $f.Name, [math]::Round($f.Length/1024,1), (-not $isSqlite), $hex)
    } catch {
        Write-Host ("{0,-30} {1,10}KB  ERROR: {2}" -f $f.Name, [math]::Round($f.Length/1024,1), $_.Exception.Message)
    }
}

# Global DBs
Write-Host "`n=== Global DBs ==="
$gDir = "D:\QQ\Tencent Files\nt_qq\global\nt_db"
if (Test-Path $gDir) {
    $gdbs = Get-ChildItem -Path $gDir -Filter "*.db" -ErrorAction SilentlyContinue
    foreach ($f in $gdbs) {
        $bytes = [byte[]]::new(16)
        try {
            $fs = [IO.File]::Open($f.FullName, 'Open', 'Read', 'ReadWrite,Delete')
            $fs.Read($bytes, 0, 16) | Out-Null
            $fs.Close()
            $hex = ($bytes[0..7] | ForEach-Object { '{0:X2}' -f $_ }) -join ' '
            $isSqlite = [Text.Encoding]::ASCII.GetString($bytes, 0, 6) -eq 'SQLite'
            Write-Host ("{0,-30} {1,10}KB  enc={2,-6} hdr={3}" -f $f.Name, [math]::Round($f.Length/1024,1), (-not $isSqlite), $hex)
        } catch {
            Write-Host ("{0,-30} {1,10}KB  ERROR" -f $f.Name, [math]::Round($f.Length/1024,1))
        }
    }
}

# passphrase search
Write-Host "`n=== Passphrase/Key Search ==="
$searchPaths = @(
    "D:\QQ\Tencent Files\3371574658",
    "D:\QQ\Tencent Files\nt_qq"
)
foreach ($sp in $searchPaths) {
    if (Test-Path $sp) {
        Get-ChildItem -Path $sp -Recurse -File -ErrorAction SilentlyContinue | Where-Object {
            ($_.Name -match "passphrase|^key$|^key\.dat$|\.key$") -and $_.Length -gt 0 -and $_.Length -lt 10240
        } | ForEach-Object {
            Write-Host ("  {0} ({1} bytes)" -f $_.FullName, $_.Length)
            $raw = [IO.File]::ReadAllBytes($_.FullName)
            $hex = ($raw[0..([Math]::Min(31,$raw.Length-1))] | ForEach-Object { '{0:X2}' -f $_ }) -join ' '
            Write-Host ("    HEX: {0}" -f $hex)
        }
    }
}

# nt_qq dir structure (top 2 levels only)
Write-Host "`n=== nt_qq Directory Structure (top 2 levels) ==="
Get-ChildItem -Path "D:\QQ\Tencent Files\3371574658\nt_qq" -Directory -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host ("  [{0}]" -f $_.Name)
    Get-ChildItem -Path $_.FullName -Directory -ErrorAction SilentlyContinue | ForEach-Object {
        Write-Host ("    [{0}]" -f $_.Name)
    }
}
