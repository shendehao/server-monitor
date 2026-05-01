# reload_dll.ps1 — 手动加载最新 MiniAgent.dll（绕过自更新）
# 用法：以管理员身份运行 PowerShell，执行此脚本

Write-Host "[1] 杀掉旧 Agent 进程..." -ForegroundColor Yellow
# 释放 mutex：杀掉所有非当前的 powershell 进程
$myPID = $PID
Get-Process powershell -ErrorAction SilentlyContinue | Where-Object { $_.Id -ne $myPID } | ForEach-Object {
    Write-Host "  Kill PID $($_.Id) - $($_.ProcessName)"
    Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
}
Start-Sleep -Seconds 3

Write-Host "[2] 加载新 DLL..." -ForegroundColor Yellow
$dllPath = Join-Path $PSScriptRoot "agent-cs\MiniAgent.dll"
if (-not (Test-Path $dllPath)) {
    Write-Host "  DLL 不存在: $dllPath" -ForegroundColor Red
    exit 1
}
$bytes = [System.IO.File]::ReadAllBytes($dllPath)
$asm = [System.Reflection.Assembly]::Load([byte[]]$bytes)
Write-Host "  DLL 已加载: $($asm.FullName), Size=$($bytes.Length)" -ForegroundColor Green

Write-Host "[3] 启动 Agent (连接服务器)..." -ForegroundColor Yellow
# 参数: serverUrl, token, signKey, deployId
$serverUrl = "http://47.115.222.73:5000"
$signKey   = "f0a521426fd3476e006959842da3b8a181cdac116663c8bb393aa043a28e292c"
Write-Host "  Server: $serverUrl"
Write-Host "  SignKey: $($signKey.Substring(0,8))..."
[MiniAgent.Entry]::Run($serverUrl, "", $signKey, "manual-reload")
