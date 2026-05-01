# Deep header analysis for NTQQ databases
$path = "D:\QQ\Tencent Files\3371574658\nt_qq\nt_db\nt_msg.db"
$bytes = [byte[]]::new(8192)
$fs = [IO.File]::Open($path, 'Open', 'Read', 'ReadWrite,Delete')
$read = $fs.Read($bytes, 0, 8192)
$fs.Close()

Write-Host "=== nt_msg.db deep analysis ==="
Write-Host "File size: $read bytes read"

# Full first 200 bytes as ASCII where printable
$ascii = ""
for ($i=0; $i -lt 200; $i++) {
    if ($bytes[$i] -ge 0x20 -and $bytes[$i] -le 0x7E) { $ascii += [char]$bytes[$i] }
    else { $ascii += "." }
}
Write-Host "ASCII (0-199): $ascii"

# Check specific offsets for page types
Write-Host "`n--- Page boundaries ---"
foreach ($ps in @(1024, 2048, 4096)) {
    if ($ps -lt $read) {
        Write-Host ("Offset $ps : 0x{0:X2} {1:X2} {2:X2} {3:X2} {4:X2} {5:X2} {6:X2} {7:X2}" -f $bytes[$ps], $bytes[$ps+1], $bytes[$ps+2], $bytes[$ps+3], $bytes[$ps+4], $bytes[$ps+5], $bytes[$ps+6], $bytes[$ps+7])
        $pt = $bytes[$ps]
        if ($pt -eq 0x0D) { Write-Host "  = Leaf table page" }
        elseif ($pt -eq 0x05) { Write-Host "  = Interior table page" }
        elseif ($pt -eq 0x0A) { Write-Host "  = Leaf index page" }
        elseif ($pt -eq 0x02) { Write-Host "  = Interior index page" }
        else { Write-Host "  = Unknown page type" }
    }
}

# Check if data after custom header (at some offset) looks like SQLite
# NTQQ might store the key in bytes 32-99 and shift actual data
# Let's see the full byte 32-130 as hex
Write-Host "`n--- Bytes 32-130 ---"
for ($row = 32; $row -lt 131; $row += 16) {
    $end = [Math]::Min($row + 15, 130)
    $hex = ($bytes[$row..$end] | ForEach-Object { '{0:X2}' -f $_ }) -join ' '
    $asc = ""
    for ($i=$row; $i -le $end; $i++) {
        if ($bytes[$i] -ge 0x20 -and $bytes[$i] -le 0x7E) { $asc += [char]$bytes[$i] } else { $asc += "." }
    }
    Write-Host ("{0,4}: {1,-48} {2}" -f $row, $hex, $asc)
}

# Try: what if the first 1024 bytes is a custom header
# and the real SQLite db starts at offset 1024?
Write-Host "`n--- Checking if real SQLite starts at offset 1024 ---"
$magic1024 = [Text.Encoding]::ASCII.GetString($bytes, 1024, 16)
Write-Host "Bytes 1024-1039 ASCII: $magic1024"
$hex1024 = ($bytes[1024..1039] | ForEach-Object { '{0:X2}' -f $_ }) -join ' '
Write-Host "Bytes 1024-1039 HEX: $hex1024"

# Also check bc_09.db (standard) for comparison
Write-Host "`n=== bc_09.db (standard) page 2 ==="
$stdPath = "D:\QQ\Tencent Files\3371574658\nt_qq\nt_db\bc_09.db"
$stdBytes = [byte[]]::new(8192)
$fs2 = [IO.File]::Open($stdPath, 'Open', 'Read', 'ReadWrite,Delete')
$fs2.Read($stdBytes, 0, 8192) | Out-Null
$fs2.Close()
$stdPageSize = ($stdBytes[16] -shl 8) -bor $stdBytes[17]
Write-Host "Page size: $stdPageSize"
Write-Host ("Offset $stdPageSize : 0x{0:X2} {1:X2} {2:X2} {3:X2}" -f $stdBytes[$stdPageSize], $stdBytes[$stdPageSize+1], $stdBytes[$stdPageSize+2], $stdBytes[$stdPageSize+3])

# Hex string extraction from nt_msg header
Write-Host "`n=== Hex string in nt_msg.db header ==="
$hexStr = ""
for ($i=47; $i -lt 200; $i++) {
    $c = [char]$bytes[$i]
    if (($c -ge '0' -and $c -le '9') -or ($c -ge 'a' -and $c -le 'f') -or ($c -ge 'A' -and $c -le 'F')) {
        $hexStr += $c
    } else {
        break
    }
}
Write-Host "Hex string at offset 47: $hexStr"
Write-Host "Hex string length: $($hexStr.Length) chars = $($hexStr.Length / 2) bytes"
