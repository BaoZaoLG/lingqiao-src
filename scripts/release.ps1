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

# 3. Build AutoExam Script
Write-Host "Building AutoExam Script..." -ForegroundColor Cyan
Push-Location packages/autoexam
.\build.ps1
if ($LASTEXITCODE -ne 0) { throw "autoexam build failed" }
Pop-Location

# 4. Build C++ Client
if (-not $SkipCpp) {
    Write-Host "Building C++ Client..." -ForegroundColor Cyan
    if (Test-Path "build/CMakeCache.txt") {
        Remove-Item -Force "build/CMakeCache.txt"
    }
    cmake -S client -B build -G "Visual Studio 17 2022" -A Win32 -DENABLE_HOOK=ON
    if ($LASTEXITCODE -ne 0) { throw "cmake config failed" }
    
    cmake --build build --config Release
    if ($LASTEXITCODE -ne 0) { throw "cmake build failed" }
}

# 5. Build Wix Installer
if (-not $SkipWix) {
    Write-Host "Building Installer..." -ForegroundColor Cyan
    Push-Location installer
    .\build_installer.ps1 -SourceDir "$Root\$SourceDir"
    if ($LASTEXITCODE -ne 0) { throw "wix installer build failed" }
    Pop-Location
}

# 6. Generate Release Artifacts
Write-Host "Generating Release Artifacts..." -ForegroundColor Cyan
$ReleaseDir = "dist/release"
if (-not (Test-Path $ReleaseDir)) { New-Item -ItemType Directory -Path $ReleaseDir | Out-Null }

$MsiPath = "installer/Output/LingqiaoSetup.msi"
if (Test-Path $MsiPath) {
    Copy-Item $MsiPath -Destination "$ReleaseDir\LingqiaoSetup.msi" -Force
    
    # Optional code signing
    if ($env:SIGNING_CERT_THUMBPRINT) {
        Write-Host "Signing MSI..."
        Set-AuthenticodeSignature -FilePath "$ReleaseDir\LingqiaoSetup.msi" -Thumbprint $env:SIGNING_CERT_THUMBPRINT -TimestampServer "http://timestamp.digicert.com"
    }

    $MsiHash = (Get-FileHash "$ReleaseDir\LingqiaoSetup.msi" -Algorithm SHA256).Hash.ToLower()
    $ScriptManifest = Get-Content "packages/autoexam/dist/autoexam.manifest.json" | ConvertFrom-Json
    
    $CommitHash = ""
    try { $CommitHash = (git rev-parse HEAD).Trim() } catch { }

    $ReleaseInfo = @{
        version = (Get-Content client/version.txt -ErrorAction SilentlyContinue) # Or a standard version fetch
        msi_sha256 = $MsiHash
        script_version = $ScriptManifest.version
        script_sha256 = $ScriptManifest.sha256
        built_at = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
        source_commit = $CommitHash
    }
    $ReleaseInfo | ConvertTo-Json | Out-File -FilePath "$ReleaseDir\release-info.json" -Encoding utf8 -Force
    
    $Checksums = @"
$MsiHash *LingqiaoSetup.msi
$($ScriptManifest.sha256) *autoexam.bundle.js
"@
    Set-Content -Path "$ReleaseDir\checksums.txt" -Value $Checksums -Encoding utf8
}

Write-Host "Pipeline Completed Successfully!" -ForegroundColor Green
