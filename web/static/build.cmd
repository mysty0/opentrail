@echo off
REM Build script for OpenTrail frontend on Windows
setlocal enabledelayedexpansion

echo Building OpenTrail frontend...

REM Check if node_modules exists
if not exist "node_modules" (
    echo Installing dependencies...
    npm install
    if !errorlevel! neq 0 (
        echo Failed to install dependencies
        exit /b 1
    )
)

REM Build the project
echo Building React application...
npm run build
if !errorlevel! neq 0 (
    echo Build failed
    exit /b 1
)

REM Copy built files to replace the old static files
echo Copying built files...
copy "dist\index.html" ".\index.html" >nul
copy "dist\style.css" ".\style.css" >nul
copy "dist\app.js" ".\app.js" >nul

REM Clean up dist directory
rmdir /s /q dist

echo Build completed successfully!
echo Files updated:
echo   - index.html
echo   - style.css
echo   - app.js