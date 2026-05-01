# 本地 webcam 测试：直接测试 MFH264Encoder + DirectShow 拍照
$dllPath = "$PSScriptRoot\MiniAgent2.dll"
$outDir = "$PSScriptRoot\test_frames"
if (Test-Path $outDir) { Remove-Item $outDir -Recurse -Force }
New-Item -ItemType Directory -Path $outDir | Out-Null

$bytes = [IO.File]::ReadAllBytes($dllPath)
$asm = [Reflection.Assembly]::Load([byte[]]$bytes)

# 列出所有类型
$types = $asm.GetTypes()
Write-Host "Types in DLL:"
foreach ($t in $types) { Write-Host "  $($t.FullName) (public=$($t.IsPublic))" }

$bf = [Reflection.BindingFlags]'Instance,NonPublic,Public'

# 找 MFH264Encoder
$encType = $types | Where-Object { $_.Name -eq 'MFH264Encoder' }
if (!$encType) { Write-Host "ERROR: MFH264Encoder not found!"; exit 1 }

Write-Host "`n=== Testing MFH264Encoder (640x480, 15fps, 500kbps) ==="
$encCtor = $encType.GetConstructors($bf)[0]
$enc = $encCtor.Invoke(@([int]640, [int]480, [int]15, [int]500000))
Write-Host "Encoder created OK"

$encodeMethod = $encType.GetMethod('Encode', $bf)
$encParams = $encodeMethod.GetParameters()
Write-Host "Encode params: $($encParams | ForEach-Object { "$($_.ParameterType) $($_.Name)" })"

# 生成渐变测试帧并编码
$w = 640; $h = 480
$totalFrames = 0; $nullFrames = 0; $keyFrames = 0
for ($f = 0; $f -lt 30; $f++) {
    $bgra = [byte[]]::new($w * $h * 4)
    for ($y = 0; $y -lt $h; $y++) {
        for ($x = 0; $x -lt $w; $x++) {
            $off = ($y * $w + $x) * 4
            $bgra[$off]   = [byte](($x + $f * 10) -band 0xFF)
            $bgra[$off+1] = [byte](($y + $f * 5) -band 0xFF)
            $bgra[$off+2] = [byte](($f * 20) -band 0xFF)
            $bgra[$off+3] = [byte]255
        }
    }
    
    # Encode(byte[] bgra, int stride, ref bool keyFrame)
    $isKey = ($f -eq 0)
    $invokeArgs = @($bgra, [int]($w * 4), $isKey)
    $result = $encodeMethod.Invoke($enc, $invokeArgs)
    $wasKey = $invokeArgs[2]
    
    if ($result -and $result.Length -gt 0) {
        $totalFrames++
        $tag = if ($wasKey) { $keyFrames++; "KEY" } else { "P" }
        $framePath = "$outDir\frame_$($f.ToString('D3')).h264"
        [IO.File]::WriteAllBytes($framePath, $result)
        Write-Host "  Frame $f : $tag $($result.Length) bytes"
    } else {
        $nullFrames++
        Write-Host "  Frame $f : NULL"
    }
}

$dispMethod = $encType.GetMethod('Dispose', $bf)
if ($dispMethod) { $dispMethod.Invoke($enc, $null) }

Write-Host "`n=== H.264 Encoder Results ==="
Write-Host "  Total output frames: $totalFrames / 30"
Write-Host "  Key frames: $keyFrames"
Write-Host "  Null frames: $nullFrames"

# 测试 DirectShow 拍照 (DsGrabJpeg 是 Agent 的私有方法)
Write-Host "`n=== Testing DirectShow Webcam Snapshot ==="
$agentType = $types | Where-Object { $_.Name -eq 'Agent' }
if ($agentType) {
    $grabMethod = $agentType.GetMethod('DsGrabJpeg', [Reflection.BindingFlags]'Static,NonPublic,Public')
    if (!$grabMethod) { $grabMethod = $agentType.GetMethod('DsGrabJpeg', $bf) }
    if ($grabMethod) {
        $agentCtor = $agentType.GetConstructors($bf)[0]
        $agent = $agentCtor.Invoke(@('http://127.0.0.1:9999', 'x', '', 'test'))
        Write-Host "Agent created, calling DsGrabJpeg..."
        try {
            $jpg = $grabMethod.Invoke($agent, @([int]640, [int]480, [int]75))
            if ($jpg -and $jpg.Length -gt 0) {
                $p = "$outDir\webcam_snap.jpg"
                [IO.File]::WriteAllBytes($p, $jpg)
                Write-Host "  WEBCAM OK: $($jpg.Length) bytes -> $p"
            } else {
                Write-Host "  WEBCAM: returned null/empty"
            }
        } catch {
            Write-Host "  WEBCAM ERROR: $_"
        }
    } else {
        Write-Host "  DsGrabJpeg method not found"
        $methods = $agentType.GetMethods($bf) | Where-Object { $_.Name -match 'Grab|Webcam|Snap|Ds' }
        foreach ($m in $methods) { Write-Host "    Found: $($m.Name)($($m.GetParameters() | ForEach-Object { $_.Name }))" }
    }
} else {
    Write-Host "  Agent type not found"
}

Write-Host "`n=== All Output Files ==="
Get-ChildItem $outDir | ForEach-Object { Write-Host "  $($_.Name) - $($_.Length) bytes" }
