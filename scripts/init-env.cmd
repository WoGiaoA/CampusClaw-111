@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0init-env.ps1"
exit /b %errorlevel%
