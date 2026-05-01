@echo off
C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe /target:library /optimize+ /nologo /out:MiniAgent.dll /r:System.dll /r:System.Core.dll /r:System.Management.dll /r:System.Net.Http.dll /r:System.Security.dll MiniAgent.cs
echo EXIT_CODE=%ERRORLEVEL%
