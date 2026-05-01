# Dump first 200 bytes of nt_msg.db and bc_09.db (standard header) for comparison
$dbDir = "D:\QQ\Tencent Files\3371574658\nt_qq\nt_db"

foreach ($dbName in @("bc_09.db", "nt_msg.db", "profile_info.db", "recent_contact.db")) {
    $path = Join-Path $dbDir $dbName
    if (-not (Test-Path $path)) { continue }
    Write-Host "`n=== $dbName ==="
    $bytes = [byte[]]::new(200)
    $fs = [IO.File]::Open($path, 'Open', 'Read', 'ReadWrite,Delete')
    $read = $fs.Read($bytes, 0, 200)
    $fs.Close()
    
    # Header (0-99)
    Write-Host "Header (0-15): $( ($bytes[0..15] | ForEach-Object { '{0:X2}' -f $_ }) -join ' ' )"
    Write-Host "ASCII (0-15):  $( [Text.Encoding]::ASCII.GetString($bytes, 0, 16).Replace("`0",'_') )"
    Write-Host "Header (16-31): $( ($bytes[16..31] | ForEach-Object { '{0:X2}' -f $_ }) -join ' ' )"
    Write-Host "Header (32-47): $( ($bytes[32..47] | ForEach-Object { '{0:X2}' -f $_ }) -join ' ' )"
    Write-Host "Header (48-63): $( ($bytes[48..63] | ForEach-Object { '{0:X2}' -f $_ }) -join ' ' )"
    Write-Host "Header (64-79): $( ($bytes[64..79] | ForEach-Object { '{0:X2}' -f $_ }) -join ' ' )"
    Write-Host "Header (80-99): $( ($bytes[80..99] | ForEach-Object { '{0:X2}' -f $_ }) -join ' ' )"
    
    # Page 1 content (100+)
    Write-Host "Page1 (100-115): $( ($bytes[100..115] | ForEach-Object { '{0:X2}' -f $_ }) -join ' ' )"
    Write-Host "Page1 (116-131): $( ($bytes[116..131] | ForEach-Object { '{0:X2}' -f $_ }) -join ' ' )"
    
    # Try XOR with each byte (0x00-0xFF) on byte 100 to see if it becomes 0x0D
    $b100 = $bytes[100]
    Write-Host "Byte[100] = 0x$( '{0:X2}' -f $b100 ) (expect 0x0D for leaf table)"
    if ($b100 -ne 0x0D) {
        $xorKey = $b100 -bxor 0x0D
        Write-Host "XOR key to get 0x0D: 0x$( '{0:X2}' -f $xorKey )"
        # Check if XORing header byte 7 with this key gives 0x66
        $b7 = $bytes[7]
        $b7xor = $b7 -bxor $xorKey
        Write-Host "Byte[7] XOR 0x$( '{0:X2}' -f $xorKey ) = 0x$( '{0:X2}' -f $b7xor ) (expect 0x66='f')"
    }
}

# Also check if the standard SQLite header for bc_09 has normal page content
Write-Host "`n=== Standard SQLite header reference ==="
Write-Host "Standard: 53 51 4C 69 74 65 20 66 6F 72 6D 61 74 20 33 00"
Write-Host "         (S  Q  L  i  t  e  SP f  o  r  m  a  t  SP 3  NUL)"
