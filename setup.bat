@echo off
echo [Imprint] Building and installing plugin...
echo.

:: Check Go
where go >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Go is not installed. Please install Go 1.25+ from https://go.dev
    exit /b 1
)

:: Check Node
where node >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Node.js is not installed. Please install Node.js 18+ from https://nodejs.org
    exit /b 1
)

:: Run installer
go run ./cmd/install
