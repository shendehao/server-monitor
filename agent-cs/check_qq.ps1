$dbDir = "D:\QQ\Tencent Files\3371574658\nt_qq\nt_db"
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

# Search for passphrase/key files
Write-Host "`n--- Searching for passphrase/key files ---"
$searchPaths = @(
    "D:\QQ\Tencent Files\3371574658",
    "D:\QQ\Tencent Files\nt_qq",
    "D:\QQ\Tencent Files"
)
foreach ($sp in $searchPaths) {
    if (Test-Path $sp) {
        Get-ChildItem -Path $sp -Recurse -ErrorAction SilentlyContinue | Where-Object {
            $_.Name -match "passphrase|\.key$|^key$|^key\.dat$" -and $_.Length -gt 0 -and $_.Length -lt 10240
        } | ForEach-Object {
            Write-Host ("  {0} ({1} bytes)" -f $_.FullName, $_.Length)
        }
    }
}

# List directory structure of nt_qq
Write-Host "`n--- D:\QQ\Tencent Files\3371574658\nt_qq structure ---"
Get-ChildItem -Path "D:\QQ\Tencent Files\3371574658\nt_qq" -Directory -ErrorAction SilentlyContinue | ForEach-Object {
    $count = (Get-ChildItem -Path $_.FullName -Recurse -File -ErrorAction SilentlyContinue).Count
    Write-Host ("  [{0}] ({1} files)" -f $_.Name, $count)
}

# Also check global nt_qq
Write-Host "`n--- D:\QQ\Tencent Files\nt_qq\global structure ---"
if (Test-Path "D:\QQ\Tencent Files\nt_qq\global") {
    Get-ChildItem -Path "D:\QQ\Tencent Files\nt_qq\global" -Recurse -File -ErrorAction SilentlyContinue | ForEach-Object {
        Write-Host ("  {0} ({1}KB)" -f $_.FullName.Replace("D:\QQ\Tencent Files\nt_qq\global\",""), [math]::Round($_.Length/1024,1))
    }
}
