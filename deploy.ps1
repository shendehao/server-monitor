$ROOT = $PSScriptRoot
$SERVER = "47.115.222.73"
$U = "root"
$TMP = "C:\deploy_tmp"

if (-not (Test-Path $TMP)) { mkdir $TMP | Out-Null }
Copy-Item (Join-Path $ROOT "server\serverlinux") (Join-Path $TMP "serverlinux") -Force
Copy-Item (Join-Path $ROOT "agent-cs\MiniAgent.dll") (Join-Path $TMP "MiniAgent.dll") -Force

Write-Host "[1/3] Upload serverlinux"
scp "$TMP\serverlinux" "${U}@${SERVER}:/www/wwwroot/goo/serverlinux"
if ($LASTEXITCODE -ne 0) { Write-Host "FAILED" -Fore Red; exit 1 }
Write-Host "OK" -Fore Green

Write-Host "[2/3] Upload DLL"
scp "$TMP\MiniAgent.dll" "${U}@${SERVER}:/www/wwwroot/goo/data/MiniAgent.dll"
if ($LASTEXITCODE -ne 0) { Write-Host "FAILED" -Fore Red; exit 1 }
Write-Host "OK" -Fore Green

Write-Host "[3/3] Restart"
ssh "${U}@${SERVER}" "pkill -f serverlinux; sleep 2; chmod +x /www/wwwroot/goo/serverlinux; cd /www/wwwroot/goo && nohup ./serverlinux >/dev/null 2>&1 & sleep 1 && pgrep -a serverlinux"

Remove-Item $TMP -Recurse -Force -EA SilentlyContinue
Write-Host "Done"
