@echo off
cd /d "%~dp0"

REM Load configuration from .env if present
if exist .env (
    for /f "usebackq tokens=1,* delims==" %%A in (".env") do (
        if not "%%A"=="" if not "%%A:~0,1%"=="#" (
            if not defined %%A set "%%A=%%B"
        )
    )
)

REM Ensure JWT_SECRET is set (minimum 32 characters required)
if not defined JWT_SECRET (
    echo [WARN] JWT_SECRET is not set. Generating dynamic runtime session secret...
    set "JWT_SECRET=k8sselfhost_session_%RANDOM%%RANDOM%%RANDOM%%RANDOM%%RANDOM%"
)

REM Ensure ENCRYPTION_KEY is set (32 characters required)
if not defined ENCRYPTION_KEY (
    if not defined MASTER_ENCRYPTION_KEY (
        echo [WARN] ENCRYPTION_KEY is not set. Generating dynamic runtime session key...
        set "ENCRYPTION_KEY=k8sselfhost_key_%RANDOM%%RANDOM%%RANDOM%%RANDOM%%RANDOM%"
    )
)

standalone.exe
