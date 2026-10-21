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
    try {
        if (Test-Path "package-lock.json") {
            npm ci
            if ($LASTEXITCODE -ne 0) { throw "npm ci failed" }
        } else {
            npm install
            if ($LASTEXITCODE -ne 0) { throw "npm install failed" }
        }
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "npm build failed" }
    } finally {
        Pop-Location
    }
}

# 2. Build Go Server
if (-not $SkipGo) {
    Write-Host "Building Go Server..." -ForegroundColor Cyan
    if (-not (Test-Path "dist/server")) { New-Item -ItemType Directory -Path "dist/server" | Out-Null }
    Push-Location server
    try {
        go build -ldflags "-s -w" -o ../dist/server/lingqiao-server.exe .
        if ($LASTEXITCODE -ne 0) { throw "go build failed" }
    } finally {
        Pop-Location
    }
}

# 3. Build AutoExam Script
Write-Host "Building AutoExam Script..." -ForegroundColor Cyan
Push-Location packages/autoexam
try {
    .\build.ps1
    if ($LASTEXITCODE -ne 0) { throw "autoexam build failed" }
} finally {
    Pop-Location
}

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
    $ResolvedSourceDir = if ([System.IO.Path]::IsPathRooted($SourceDir)) { $SourceDir } else { Join-Path $Root $SourceDir }
    $InstallerOutputDir = Join-Path $Root "dist\installer"
    & (Join-Path $Root "installer\build_installer.ps1") -SourceDir $ResolvedSourceDir -OutputDir $InstallerOutputDir
    if ($LASTEXITCODE -ne 0) { throw "wix installer build failed" }
}

# 6. Generate Release Artifacts
Write-Host "Generating Release Artifacts..." -ForegroundColor Cyan
$ReleaseDir = "dist/release"
if (-not (Test-Path $ReleaseDir)) { New-Item -ItemType Directory -Path $ReleaseDir | Out-Null }

$ClientCMake = Get-Content "client/CMakeLists.txt" -Raw
if ($ClientCMake -notmatch 'set\(APP_VERSION\s+"([^"]+)"\)') {
    throw "APP_VERSION was not found in client/CMakeLists.txt"
}
$ClientVersion = $Matches[1]
$ScriptBundlePath = "packages/autoexam/dist/autoexam.bundle.js"
$ScriptManifestPath = "packages/autoexam/dist/autoexam.manifest.json"
if (-not (Test-Path $ScriptBundlePath)) { throw "AutoExam bundle not found: $ScriptBundlePath" }
if (-not (Test-Path $ScriptManifestPath)) { throw "AutoExam manifest not found: $ScriptManifestPath" }

$MsiPath = "dist/installer/Lingqiao-$ClientVersion.msi"
$BundlePath = "dist/installer/LingqiaoSetup-$ClientVersion.exe"
if (Test-Path $MsiPath -and Test-Path $BundlePath) {
    $ReleaseMsi = "$ReleaseDir\Lingqiao-$ClientVersion.msi"
    $ReleaseBundle = "$ReleaseDir\LingqiaoSetup-$ClientVersion.exe"
    $ReleaseScript = "$ReleaseDir\autoexam.bundle.js"
    Copy-Item $MsiPath -Destination $ReleaseMsi -Force
    Copy-Item $BundlePath -Destination $ReleaseBundle -Force
    Copy-Item $ScriptBundlePath -Destination $ReleaseScript -Force

    $MsiHash = (Get-FileHash $ReleaseMsi -Algorithm SHA256).Hash.ToLower()
    $BundleHash = (Get-FileHash $ReleaseBundle -Algorithm SHA256).Hash.ToLower()
    $ScriptHash = (Get-FileHash $ReleaseScript -Algorithm SHA256).Hash.ToLower()
    $ScriptManifest = Get-Content $ScriptManifestPath | ConvertFrom-Json
    if ($ScriptManifest.sha256 -and $ScriptManifest.sha256.ToLower() -ne $ScriptHash) {
        throw "AutoExam bundle hash does not match manifest"
    }
    
    $CommitHash = ""
    try { $CommitHash = (git rev-parse HEAD).Trim() } catch { }

    $ReleaseInfo = @{
        version = $ClientVersion
        bundle = (Split-Path -Leaf $ReleaseBundle)
        bundle_sha256 = $BundleHash
        msi = (Split-Path -Leaf $ReleaseMsi)
        msi_sha256 = $MsiHash
        script_version = $ScriptManifest.version
        script_sha256 = $ScriptHash
        built_at = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
        source_commit = $CommitHash
    }
    $ReleaseInfo | ConvertTo-Json | Out-File -FilePath "$ReleaseDir\release-info.json" -Encoding utf8 -Force
    
$Checksums = @"
$BundleHash *$(Split-Path -Leaf $ReleaseBundle)
$MsiHash *$(Split-Path -Leaf $ReleaseMsi)
$ScriptHash *autoexam.bundle.js
"@
    Set-Content -Path "$ReleaseDir\checksums.txt" -Value $Checksums -Encoding utf8
} elseif (-not $SkipWix) {
    throw "Installer artifacts not found: $MsiPath and $BundlePath"
}

Write-Host "Pipeline Completed Successfully!" -ForegroundColor Green
