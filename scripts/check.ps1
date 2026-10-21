param (
    [switch]$SkipWeb,
    [switch]$SkipGo,
    [switch]$SkipE2E,
    [switch]$SkipVuln,
    [switch]$SkipCpp
)

$ErrorActionPreference = "Stop"
$Root = (Get-Location).Path

function Invoke-CheckedProcess {
    param(
        [Parameter(Mandatory=$true)][string]$FilePath,
        [Parameter(Mandatory=$true)][string]$ArgumentList,
        [Parameter(Mandatory=$true)][string]$WorkingDirectory,
        [Parameter(Mandatory=$true)][string]$FailureMessage
    )
    $proc = Start-Process -FilePath $FilePath -ArgumentList $ArgumentList -WorkingDirectory $WorkingDirectory -PassThru -Wait -NoNewWindow
    if ($proc.ExitCode -ne 0) { throw "$FailureMessage with exit code $($proc.ExitCode)" }
}

Write-Host "=========================================" -ForegroundColor Cyan
Write-Host " Lingqiao Quality Gates (Preflight Check)" -ForegroundColor Cyan
Write-Host "=========================================" -ForegroundColor Cyan

# 0. Lightweight package/path consistency (no build required)
Write-Host "`n[0/5] Running package & path consistency checks..." -ForegroundColor Yellow
$pyCmd = Get-Command python -ErrorAction SilentlyContinue
if (-not $pyCmd) { $pyCmd = Get-Command python3 -ErrorAction SilentlyContinue }
if ($pyCmd) {
    $py = $pyCmd.Source
    & $py (Join-Path $Root "scripts/check_admin_api_paths.py")
    if ($LASTEXITCODE -ne 0) { throw "check_admin_api_paths.py failed" }
    & $py (Join-Path $Root "scripts/check_autoexam_package.py")
    if ($LASTEXITCODE -ne 0) { throw "check_autoexam_package.py failed" }
    # Local path parity only; production entrypoint check is optional via env LQ_CHECK_HOST
    if ($env:LQ_CHECK_HOST) {
        & $py (Join-Path $Root "scripts/check_java_production_entrypoint.py") --host $env:LQ_CHECK_HOST
        if ($LASTEXITCODE -ne 0) { throw "check_java_production_entrypoint.py failed" }
    }
    & $py (Join-Path $Root "scripts/acceptance_checklist.py")
    if ($LASTEXITCODE -ne 0) { throw "acceptance_checklist.py failed" }

    # Review report, not a hard gate: a hook entry point that is declared but
    # never called fails silently (no build error, no test failure, no log).
    & $py (Join-Path $Root "scripts/check_unwired_hook_entries.py")

    # Artifact contract: only meaningful once a sidecar JAR has been built.
    # Guards the cutover, where an older JAR would silently break chat/update.
    $sidecarJar = Get-ChildItem -Path (Join-Path $Root "server-java/target") -Filter "*.jar" -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -notlike "*sources*" -and $_.Name -notlike "*javadoc*" } |
        Select-Object -First 1
    if ($sidecarJar) {
        & $py (Join-Path $Root "scripts/check_java_artifact.py") $sidecarJar.FullName
        if ($LASTEXITCODE -ne 0) { throw "check_java_artifact.py failed" }
        Write-Host "Java artifact contract check passed." -ForegroundColor Green
    } else {
        Write-Host "  no sidecar JAR built yet — skipping Java artifact contract check." -ForegroundColor DarkGray
    }
    Write-Host "Package & path checks passed." -ForegroundColor Green
} else {
    Write-Host "  python not found — skipping package/path checks." -ForegroundColor DarkGray
}

# 0b. Bridge JS bootstrap + AI preflight + injection guard (no build required)
$nodeCmd = Get-Command node -ErrorAction SilentlyContinue
if ($nodeCmd) {
    & $nodeCmd.Source (Join-Path $Root "scripts/check_bridge_relay.js")
    if ($LASTEXITCODE -ne 0) { throw "check_bridge_relay.js failed" }
    & $nodeCmd.Source (Join-Path $Root "scripts/check_ai_preflight.js")
    if ($LASTEXITCODE -ne 0) { throw "check_ai_preflight.js failed" }
    & $nodeCmd.Source (Join-Path $Root "scripts/check_injection_guard.js")
    if ($LASTEXITCODE -ne 0) { throw "check_injection_guard.js failed" }
    Write-Host "Bridge relay, AI preflight and injection guard checks passed." -ForegroundColor Green
} else {
    Write-Host "  node not found — skipping bridge/AI/injection checks." -ForegroundColor DarkGray
}

# 1. Frontend Checks
if (-not $SkipWeb) {
    Write-Host "`n[1/5] Running Frontend checks (npm run check)..." -ForegroundColor Yellow
    
    $webPath = Join-Path $Root "server/web"
    if (-not (Test-Path "$webPath/node_modules")) {
        Write-Host "  Installing npm dependencies..."
        $installArgs = if (Test-Path (Join-Path $webPath "package-lock.json")) { "ci" } else { "install" }
        Invoke-CheckedProcess -FilePath "npm.cmd" -ArgumentList $installArgs -WorkingDirectory $webPath -FailureMessage "npm $installArgs failed"
    }
    
    Invoke-CheckedProcess -FilePath "npm.cmd" -ArgumentList "run check" -WorkingDirectory $webPath -FailureMessage "npm run check failed"

    Write-Host "Frontend checks passed." -ForegroundColor Green
} else {
    Write-Host "`n[1/5] Skipping Frontend checks." -ForegroundColor DarkGray
}

# 2. Backend Checks
if (-not $SkipGo) {
    Write-Host "`n[2/5] Running Backend tests (go test ./...)..." -ForegroundColor Yellow
    Push-Location server
    try {
        go test ./...
        if ($LASTEXITCODE -ne 0) { throw "go test failed with exit code $LASTEXITCODE" }
    } finally {
        Pop-Location
    }
    Write-Host "Backend checks passed." -ForegroundColor Green
} else {
    Write-Host "`n[2/5] Skipping Backend checks." -ForegroundColor DarkGray
}

# 3. E2E Checks
if (-not $SkipE2E) {
    Write-Host "`n[3/5] Running E2E tests (go test ./...)..." -ForegroundColor Yellow
    Push-Location e2e_test
    try {
        go test ./...
        if ($LASTEXITCODE -ne 0) { throw "e2e go test failed with exit code $LASTEXITCODE" }
    } finally {
        Pop-Location
    }
    Write-Host "E2E checks passed." -ForegroundColor Green
} else {
    Write-Host "`n[3/5] Skipping E2E checks." -ForegroundColor DarkGray
}

# 4. Vulnerability Checks
if (-not $SkipVuln) {
    Write-Host "`n[4/5] Running Go vulnerability scan (govulncheck)..." -ForegroundColor Yellow
    Push-Location server
    try {
        go run golang.org/x/vuln/cmd/govulncheck@latest ./...
        if ($LASTEXITCODE -ne 0) { throw "govulncheck failed with exit code $LASTEXITCODE" }
    } finally {
        Pop-Location
    }
    Write-Host "Vulnerability checks passed." -ForegroundColor Green
} else {
    Write-Host "`n[4/5] Skipping vulnerability checks." -ForegroundColor DarkGray
}

# 5. Client C++ Checks (isolated configure, build, and tests)
if (-not $SkipCpp) {
    Write-Host "`n[5/5] Building and testing C++ Client..." -ForegroundColor Yellow
    $cppBuildPath = Join-Path $Root "build-check"
    try {
        cmake -S client -B $cppBuildPath -G "Visual Studio 17 2022" -A Win32
        if ($LASTEXITCODE -ne 0) { throw "cmake configure failed with exit code $LASTEXITCODE" }
        cmake --build $cppBuildPath --config Release --target ClientTests BridgeSecurityTests V8CompileSamplerTests V8FastNeutralizeTests V8RawSourceProbeTests V8SourceNeutralizerTests CefHook Injector
        if ($LASTEXITCODE -ne 0) { throw "cmake build failed with exit code $LASTEXITCODE" }
        ctest --test-dir $cppBuildPath -C Release --output-on-failure
        if ($LASTEXITCODE -ne 0) { throw "ctest failed with exit code $LASTEXITCODE" }
        & (Join-Path $cppBuildPath "src/Release/ClientTests.exe")
        if ($LASTEXITCODE -ne 0) { throw "ClientTests failed with exit code $LASTEXITCODE" }
    } catch {
        Write-Error "C++ checks failed: $_"
        exit 1
    }
    Write-Host "C++ checks passed." -ForegroundColor Green
} else {
    Write-Host "`n[5/5] Skipping C++ checks." -ForegroundColor DarkGray
}

Write-Host "`n=========================================" -ForegroundColor Cyan
Write-Host " ALL CHECKS PASSED SUCCESSFULLY!" -ForegroundColor Green
Write-Host "=========================================`n" -ForegroundColor Cyan
