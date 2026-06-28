param (
    [switch]$SkipWeb,
    [switch]$SkipGo,
    [switch]$SkipCpp
)

Write-Host "=========================================" -ForegroundColor Cyan
Write-Host " Lingqiao Quality Gates (Preflight Check)" -ForegroundColor Cyan
Write-Host "=========================================" -ForegroundColor Cyan

# 1. Frontend Checks
if (-not $SkipWeb) {
    Write-Host "`n[1/3] Running Frontend checks (npm run check)..." -ForegroundColor Yellow
    
    $webPath = Join-Path (Get-Location).Path "server/web"
    if (-not (Test-Path "$webPath/node_modules")) {
        Write-Host "  Installing npm dependencies..."
        $installProc = Start-Process -FilePath "npm.cmd" -ArgumentList "install" -WorkingDirectory $webPath -PassThru -Wait -NoNewWindow
        if ($installProc.ExitCode -ne 0) { throw "npm install failed with exit code $($installProc.ExitCode)" }
    }
    
    $checkProc = Start-Process -FilePath "npm.cmd" -ArgumentList "run check" -WorkingDirectory $webPath -PassThru -Wait -NoNewWindow
    if ($checkProc.ExitCode -ne 0) { throw "npm run check failed with exit code $($checkProc.ExitCode)" }

    Write-Host "Frontend checks passed." -ForegroundColor Green
} else {
    Write-Host "`n[1/3] Skipping Frontend checks." -ForegroundColor DarkGray
}

# 2. Backend Checks
if (-not $SkipGo) {
    Write-Host "`n[2/3] Running Backend tests (go test ./...)..." -ForegroundColor Yellow
    Push-Location server
    try {
        go test ./...
        if ($LASTEXITCODE -ne 0) { throw "go test failed with exit code $LASTEXITCODE" }
    } finally {
        Pop-Location
    }
    Write-Host "Backend checks passed." -ForegroundColor Green
} else {
    Write-Host "`n[2/3] Skipping Backend checks." -ForegroundColor DarkGray
}

# 3. Client C++ Checks (CMake config check)
if (-not $SkipCpp) {
    Write-Host "`n[3/3] Checking C++ Client CMake configuration..." -ForegroundColor Yellow
    try {
        cmake -S client -B build -G "Visual Studio 17 2022" -A Win32
        if ($LASTEXITCODE -ne 0) { throw "cmake failed with exit code $LASTEXITCODE" }
    } catch {
        Write-Error "CMake configuration failed: $_"
        exit 1
    }
    Write-Host "C++ checks passed." -ForegroundColor Green
} else {
    Write-Host "`n[3/3] Skipping C++ checks." -ForegroundColor DarkGray
}

Write-Host "`n=========================================" -ForegroundColor Cyan
Write-Host " ALL CHECKS PASSED SUCCESSFULLY!" -ForegroundColor Green
Write-Host "=========================================`n" -ForegroundColor Cyan
