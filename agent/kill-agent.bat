@echo off
echo ============================
echo   停止 Agent 并清理旧版本
echo ============================

:: 杀掉所有 agent-windows 进程
taskkill /F /IM agent-windows.exe >nul 2>&1
if %errorlevel%==0 (
    echo [OK] 已停止 agent-windows.exe
) else (
    echo [INFO] 未找到运行中的 agent-windows.exe
)

:: 等待进程完全退出
timeout /t 2 /nobreak >nul

:: 删除备份文件防止回滚
del /F /Q "%ProgramData%\ServerMonitorAgent\*.bak" >nul 2>&1
del /F /Q "%ProgramData%\ServerMonitorAgent\.agent-update-tmp.exe" >nul 2>&1
echo [OK] 已清理备份文件

echo.
echo 进程已停止，现在可以执行安装脚本更新到最新版本：
echo   irm http://你的服务器:5000/api/agent/install-win ^| iex
echo.
pause
