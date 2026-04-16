@echo off
chcp 65001 >nul 2>&1
setlocal

cd /d "%~dp0\.."
echo Working dir: %cd%

echo [1/3] Building...
go build -a -o bin\server.exe .\cmd\server\main.go
if errorlevel 1 (
    echo Build failed!
    pause
    exit /b 1
)

echo [2/3] Stopping old server...
for /f "tokens=2" %%i in ('tasklist /fi "imagename eq server.exe" /nh 2^>nul ^| findstr /i "server.exe"') do (
    taskkill /f /pid %%i >nul 2>&1 && echo   Stopped PID %%i
)
timeout /t 1 /nobreak >nul


echo [3/3] Starting new server...
start /b "" cmd /c "bin\server.exe > server.log 2>&1"

timeout /t 2 /nobreak >nul
curl -s http://localhost:18089/api/health >nul 2>&1
if not errorlevel 1 (
    echo Server is running at http://localhost:18089
) else (
    echo WARNING: health check failed, check server.log for errors
)

endlocal
