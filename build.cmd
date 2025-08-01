@echo off
REM OpenTrail Build Script for Windows
REM Builds the application for multiple platforms with embedded static assets

setlocal enabledelayedexpansion

REM Configuration
set APP_NAME=opentrail
if "%VERSION%"=="" set VERSION=1.0.0
set BUILD_DIR=build
set CMD_PATH=./cmd/opentrail

REM Build information
for /f "tokens=*" %%i in ('powershell -Command "Get-Date -UFormat '%%Y-%%m-%%d_%%H:%%M:%%S'"') do set BUILD_TIME=%%i
for /f "tokens=*" %%i in ('git rev-parse --short HEAD 2^>nul') do set GIT_COMMIT=%%i
if "%GIT_COMMIT%"=="" set GIT_COMMIT=unknown

REM Go build flags
set LDFLAGS=-s -w -X main.Version=%VERSION% -X main.BuildTime=%BUILD_TIME% -X main.GitCommit=%GIT_COMMIT%

echo Building OpenTrail v%VERSION%
echo Build time: %BUILD_TIME%
echo Git commit: %GIT_COMMIT%
echo.

REM Clean build directory
if exist %BUILD_DIR% rmdir /s /q %BUILD_DIR%
mkdir %BUILD_DIR%

REM Build for Windows (current platform)
echo Building for windows/amd64...
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=1
go build -ldflags="%LDFLAGS%" -o "%BUILD_DIR%/%APP_NAME%-windows-amd64.exe" "%CMD_PATH%"
if errorlevel 1 (
    echo Error building for windows/amd64
    exit /b 1
)
echo ✓ Built %BUILD_DIR%/%APP_NAME%-windows-amd64.exe

REM Build for Linux (if cross-compilation is set up)
echo Building for linux/amd64...
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=1
go build -ldflags="%LDFLAGS%" -o "%BUILD_DIR%/%APP_NAME%-linux-amd64" "%CMD_PATH%"
if errorlevel 1 (
    echo Warning: Could not build for linux/amd64 (cross-compilation may not be set up)
) else (
    echo ✓ Built %BUILD_DIR%/%APP_NAME%-linux-amd64
)

echo.
echo Build completed!
echo Binaries available in %BUILD_DIR%/
dir %BUILD_DIR%

echo.
echo Build summary:
echo - Version: %VERSION%
echo - Build time: %BUILD_TIME%
echo - Git commit: %GIT_COMMIT%
echo - Output directory: %BUILD_DIR%/

endlocal