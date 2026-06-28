param (
    [string]$SourceDir = "build\src\Release",
    [switch]$SkipWeb = $false,
    [switch]$SkipGo = $false,
    [switch]$SkipCpp = $false,
    [switch]$SkipWix = $false
)

$ErrorActionPreference = "Stop"

$Root = (Get-Item .).FullName
if (!(Test-Path "client/CMakeLists.txt")) {
    Write-Error "Must be run from project root"
}

Write-Host "Starting Release Pipeline..." -ForegroundColor Green

# 1. Build Web Admin/Agent
if (-not $SkipWeb) {
    Write-Host "Building Web UI..." -ForegroundColor Cyan
    Push-Location server/web
    npm install
    npm run build
    if ($LASTEXITCODE -ne 0) { throw "npm build failed" }
    Pop-Location
}

# 2. Build Go Server
if (-not $SkipGo) {
    Write-Host "Building Go Server..." -ForegroundColor Cyan
    Push-Location server
    go build -ldflags "-s -w" -o ../dist/server/lingqiao-server.exe .
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }
    Pop-Location
}

# 3. Build C++ Client
if (-not $SkipCpp) {
    Write-Host "Building C++ Client..." -ForegroundColor Cyan
    cmake -S client -B build -G "Visual Studio 17 2022" -A Win32 -DENABLE_HOOK=ON
    if ($LASTEXITCODE -ne 0) { throw "cmake config failed" }
    
    cmake --build build --config Release
    if ($LASTEXITCODE -ne 0) { throw "cmake build failed" }
}

# 4. Build Wix Installer
if (-not $SkipWix) {
    Write-Host "Building Installer..." -ForegroundColor Cyan
    Push-Location installer
    .\build_installer.ps1 -SourceDir "..\$SourceDir"
    if ($LASTEXITCODE -ne 0) { throw "wix installer build failed" }
    Pop-Location
}

Write-Host "Pipeline Completed Successfully!" -ForegroundColor Green
