@echo off
title EchoWarp
"%~dp0EchoWarp.exe" %*
if %errorlevel% neq 0 pause
