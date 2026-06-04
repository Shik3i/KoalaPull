@echo off
setlocal
node "%~dp0verify.mjs"
exit /b %errorlevel%
