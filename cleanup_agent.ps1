# ═══ Go Agent 全面清理脚本 ═══
# 以管理员身份运行！
# 清除所有持久化：服务、计划任务、WMI、注册表、进程、文件

Write-Host "========== Go Agent 清理脚本 ==========" -ForegroundColor Yellow
Write-Host ""

# [1] 杀进程 — 杀掉所有从 ProgramData 运行的 exe
Write-Host "[1/7] 杀进程..." -ForegroundColor Cyan
Get-WmiObject Win32_Process | Where-Object {
    $_.ExecutablePath -and $_.ExecutablePath -like 'C:\ProgramData\*\*.exe'
} | ForEach-Object {
    Write-Host "  杀掉: PID=$($_.ProcessId) $($_.ExecutablePath)" -ForegroundColor Red
    Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
}
# 也杀旧版路径
Get-Process | Where-Object { $_.Path -like '*ServerMonitorAgent*' } | ForEach-Object {
    Write-Host "  杀掉旧版: PID=$($_.Id) $($_.Path)" -ForegroundColor Red
    Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
}
Start-Sleep -Seconds 2

# [2] 移除 Windows 服务 — 找所有 binPath 指向 ProgramData 的服务
Write-Host "[2/7] 移除服务..." -ForegroundColor Cyan
Get-WmiObject Win32_Service | Where-Object {
    $_.PathName -and $_.PathName -like '*ProgramData*'
} | ForEach-Object {
    Write-Host "  停止+删除服务: $($_.Name) -> $($_.PathName)" -ForegroundColor Red
    sc.exe stop $_.Name 2>$null
    sc.exe delete $_.Name 2>$null
}

# [3] 移除计划任务 — 扫描所有任务，删除 Action 指向 ProgramData 或含 cradle 的
Write-Host "[3/7] 移除计划任务..." -ForegroundColor Cyan
Get-ScheduledTask -ErrorAction SilentlyContinue | ForEach-Object {
    $task = $_
    $actions = $task.Actions
    foreach ($a in $actions) {
        $exe = $a.Execute
        $arg = $a.Arguments
        $suspicious = $false
        if ($exe -like '*ProgramData*') { $suspicious = $true }
        if ($arg -like '*ProgramData*') { $suspicious = $true }
        if ($arg -like '*EncodedCommand*' -and $arg -like '*agent*') { $suspicious = $true }
        if ($arg -like '*stager*') { $suspicious = $true }
        if ($arg -like '*DownloadString*') { $suspicious = $true }
        # 匹配 guard-a / guard-b 参数
        if ($arg -like '*guard-a*' -or $arg -like '*guard-b*') { $suspicious = $true }
        if ($suspicious) {
            $fullName = if ($task.TaskPath -eq '\') { $task.TaskName } else { $task.TaskPath + $task.TaskName }
            Write-Host "  删除任务: $fullName" -ForegroundColor Red
            Unregister-ScheduledTask -TaskName $task.TaskName -TaskPath $task.TaskPath -Confirm:$false -ErrorAction SilentlyContinue
        }
    }
}

# [4] 移除 WMI 事件订阅 — 删除所有 ActiveScriptEventConsumer
Write-Host "[4/7] 移除 WMI 订阅..." -ForegroundColor Cyan
$ns = 'root\subscription'
Get-WmiObject -Namespace $ns -Class ActiveScriptEventConsumer -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host "  删除 WMI Consumer: $($_.Name)" -ForegroundColor Red
    $_ | Remove-WmiObject -ErrorAction SilentlyContinue
}
Get-WmiObject -Namespace $ns -Class CommandLineEventConsumer -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host "  删除 WMI CmdConsumer: $($_.Name)" -ForegroundColor Red
    $_ | Remove-WmiObject -ErrorAction SilentlyContinue
}
Get-WmiObject -Namespace $ns -Class __FilterToConsumerBinding -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host "  删除 WMI Binding: $($_.Filter)" -ForegroundColor Red
    $_ | Remove-WmiObject -ErrorAction SilentlyContinue
}
Get-WmiObject -Namespace $ns -Class __EventFilter -ErrorAction SilentlyContinue | Where-Object {
    $_.Query -like '*Win32_PerfFormattedData*'
} | ForEach-Object {
    Write-Host "  删除 WMI Filter: $($_.Name)" -ForegroundColor Red
    $_ | Remove-WmiObject -ErrorAction SilentlyContinue
}

# [5] 清理注册表
Write-Host "[5/7] 清理注册表..." -ForegroundColor Cyan
$runPath = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run'
(Get-ItemProperty $runPath -ErrorAction SilentlyContinue).PSObject.Properties | Where-Object {
    $_.Value -like '*ProgramData*' -or $_.Value -like '*powershell*stager*' -or $_.Value -like '*DownloadString*'
} | ForEach-Object {
    Write-Host "  删除 Run 键: $($_.Name)" -ForegroundColor Red
    Remove-ItemProperty -Path $runPath -Name $_.Name -ErrorAction SilentlyContinue
}
# HKCU Run
$runPathCU = 'HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run'
(Get-ItemProperty $runPathCU -ErrorAction SilentlyContinue).PSObject.Properties | Where-Object {
    $_.Value -like '*ProgramData*' -or $_.Value -like '*powershell*stager*' -or $_.Value -like '*DownloadString*'
} | ForEach-Object {
    Write-Host "  删除 HKCU Run 键: $($_.Name)" -ForegroundColor Red
    Remove-ItemProperty -Path $runPathCU -Name $_.Name -ErrorAction SilentlyContinue
}
# UserInitMprLogonScript（最危险，会阻止登录）
$winlogonPath = 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon'
$logonScript = (Get-ItemProperty $winlogonPath -ErrorAction SilentlyContinue).UserInitMprLogonScript
if ($logonScript) {
    Write-Host "  删除 UserInitMprLogonScript: $logonScript" -ForegroundColor Red
    Remove-ItemProperty -Path $winlogonPath -Name 'UserInitMprLogonScript' -ErrorAction SilentlyContinue
}

# [6] 删除文件
Write-Host "[6/7] 删除文件..." -ForegroundColor Cyan
# 旧版固定路径
if (Test-Path 'C:\ProgramData\ServerMonitorAgent') {
    icacls 'C:\ProgramData\ServerMonitorAgent' /reset /T /Q 2>$null
    Remove-Item 'C:\ProgramData\ServerMonitorAgent' -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host "  删除: C:\ProgramData\ServerMonitorAgent" -ForegroundColor Red
}
# 动态路径 — 扫描 ProgramData 下含 agent.conf 的目录
Get-ChildItem 'C:\ProgramData' -Directory -ErrorAction SilentlyContinue | ForEach-Object {
    $conf = Join-Path $_.FullName 'agent.conf'
    if (Test-Path $conf) {
        Write-Host "  发现 agent 目录: $($_.FullName)" -ForegroundColor Red
        icacls $_.FullName /reset /T /Q 2>$null
        Remove-Item $_.FullName -Recurse -Force -ErrorAction SilentlyContinue
    }
}
# 备份位置
$backupDirs = @(
    "$env:LOCALAPPDATA\Microsoft\CLR_Security",
    "$env:APPDATA\Microsoft\Crypto\Keys",
    "$env:ProgramData\Microsoft\Diagnosis\ETLLogs\ShutdownLogger",
    "$env:windir\Temp\MpCmdRun",
    "$env:ProgramData\Microsoft\Windows\WER\Temp"
)
foreach ($d in $backupDirs) {
    if (Test-Path $d) {
        $hasExe = Get-ChildItem $d -Filter '*.exe' -ErrorAction SilentlyContinue
        if ($hasExe) {
            Write-Host "  清理备份: $d" -ForegroundColor Red
            Remove-Item $d -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

# [7] 清理 Defender/火绒/360 白名单
Write-Host "[7/7] 清理杀软白名单..." -ForegroundColor Cyan
Get-MpPreference -ErrorAction SilentlyContinue | Select-Object -ExpandProperty ExclusionPath -ErrorAction SilentlyContinue | Where-Object {
    $_ -like '*ProgramData*' -and $_ -notlike '*Microsoft*Windows*'
} | ForEach-Object {
    Write-Host "  移除 Defender 排除: $_" -ForegroundColor Red
    Remove-MpPreference -ExclusionPath $_ -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Host "========== 清理完成 ==========" -ForegroundColor Green
Write-Host "现在可以安全重启电脑了。" -ForegroundColor Green
