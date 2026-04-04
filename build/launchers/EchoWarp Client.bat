@echo off
title EchoWarp Client
"%~dp0EchoWarp.exe" client
if %errorlevel% neq 0 pause
