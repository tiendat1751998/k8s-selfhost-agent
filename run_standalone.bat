@echo off
set JWT_SECRET=supersecretjwtkey_k8sselfhost_min32chars_test
set ENCRYPTION_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
cd /d "%~dp0"
standalone.exe
