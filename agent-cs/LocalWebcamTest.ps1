# 本地 webcam 测试：加载 DLL，直接调用 DirectShow 采集 + H.264 编码
# 保存帧到本地文件验证

$dllPath = "$PSScriptRoot\MiniAgent2.dll"
$outDir = "$PSScriptRoot\test_frames"
if (Test-Path $outDir) { Remove-Item $outDir -Recurse -Force }
New-Item -ItemType Directory -Path $outDir | Out-Null

$bytes = [IO.File]::ReadAllBytes($dllPath)
$asm = [Reflection.Assembly]::Load([byte[]]$bytes)

# 通过反射拿到 Agent 类型和内部方法
$agentType = $asm.GetType('MiniAgent.Agent')
$bf = [Reflection.BindingFlags]'Instance,NonPublic,Public'
$bfs = [Reflection.BindingFlags]'Static,NonPublic,Public'

# 创建 Agent 实例（不连接服务器）
$ctor = $agentType.GetConstructors($bf)[0]
$agent = $ctor.Invoke(@('http://127.0.0.1:9999', 'dummy', '', 'test'))

Write-Host "=== Agent created, testing webcam capture ==="

# 调用 DsGrabJpeg (拍照方法) 来验证摄像头可用
$grabMethod = $agentType.GetMethod('DsGrabJpeg', $bf)
if ($grabMethod) {
    Write-Host "Found DsGrabJpeg, testing snapshot..."
    $jpegBytes = $grabMethod.Invoke($agent, @(640, 480, 75))
    if ($jpegBytes -and $jpegBytes.Length -gt 0) {
        $snapPath = "$outDir\snapshot.jpg"
        [IO.File]::WriteAllBytes($snapPath, $jpegBytes)
        Write-Host "SNAPSHOT OK: $($jpegBytes.Length) bytes -> $snapPath"
    } else {
        Write-Host "SNAPSHOT FAILED: no data returned"
    }
} else {
    Write-Host "DsGrabJpeg not found, trying alternative..."
}

# 测试 MFH264Encoder
Write-Host "`n=== Testing MFH264Encoder ==="
$encType = $asm.GetType('MiniAgent.MFH264Encoder')
if ($encType) {
    $encCtor = $encType.GetConstructors($bf)[0]
    $params = $encCtor.GetParameters()
    Write-Host "Encoder ctor params: $($params.Length) -> $($params | ForEach-Object { $_.Name })"
    
    try {
        $enc = $encCtor.Invoke(@([int]640, [int]480, [int]15, [int]500000))
        Write-Host "MFH264Encoder created OK"
        
        $encodeMethod = $encType.GetMethod('Encode', $bf)
        Write-Host "Encode method: $($encodeMethod.GetParameters() | ForEach-Object { "$($_.ParameterType.Name) $($_.Name)" })"
        
        # 创建测试 BGRA 帧（渐变色）
        $w = 640; $h = 480; $stride = $w * 4
        for ($f = 0; $f -lt 30; $f++) {
            $bgra = New-Object byte[] ($w * $h * 4)
            for ($y = 0; $y -lt $h; $y++) {
                for ($x = 0; $x -lt $w; $x++) {
                    $off = ($y * $w + $x) * 4
                    $bgra[$off] = [byte](($x + $f * 10) % 256)     # B
                    $bgra[$off+1] = [byte](($y + $f * 5) % 256)    # G
                    $bgra[$off+2] = [byte](($f * 20) % 256)        # R
                    $bgra[$off+3] = 255                              # A
                }
            }
            
            $result = $encodeMethod.Invoke($enc, @($bgra, $stride, ($f -eq 0)))
            if ($result -and $result.Length -gt 0) {
                $framePath = "$outDir\h264_frame_$($f.ToString('D3')).bin"
                [IO.File]::WriteAllBytes($framePath, $result)
                $isKey = if ($f -eq 0) { "KEY" } else { "P" }
                Write-Host "Frame $f : $isKey $($result.Length) bytes -> $framePath"
            } else {
                Write-Host "Frame $f : NULL (no output)"
            }
        }
        
        # Dispose
        $disposeMethod = $encType.GetMethod('Dispose', $bf)
        if ($disposeMethod) { $disposeMethod.Invoke($enc, $null) }
        Write-Host "Encoder disposed"
    } catch {
        Write-Host "Encoder ERROR: $_"
    }
} else {
    Write-Host "MFH264Encoder type not found!"
}

# 汇总
Write-Host "`n=== Test Results ==="
$files = Get-ChildItem $outDir
Write-Host "Output files: $($files.Count)"
foreach ($f in $files) {
    Write-Host "  $($f.Name) - $($f.Length) bytes"
}
