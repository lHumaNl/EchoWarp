@echo off
title EchoWarp Server
"%~dp0EchoWarp.exe" server
if %errorlevel% neq 0 pause
