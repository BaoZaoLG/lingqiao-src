param(
    [string]$Version = "",
    [string]$SourceDir = "",
    [string]$OutputDir = "",
    [string]$WixExe = "wix"
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
if (-not $Version) {
    $cmake = Get-Content -LiteralPath (Join-Path $Root "CMakeLists.txt") -Raw
    if ($cmake -match 'set\(APP_VERSION "([^"]+)"\)') {
        $Version = $Matches[1]
    } else {
        throw "Version is required and APP_VERSION was not found"
    }
}
if (-not $SourceDir) {
    $SourceDir = Join-Path $Root "build-hook-log\src\Release"
}
if (-not $OutputDir) {
    $OutputDir = Join-Path $Root "dist\installer"
}

$SourceDir = [System.IO.Path]::GetFullPath($SourceDir)
$OutputDir = [System.IO.Path]::GetFullPath($OutputDir)
$InjectorExe = Join-Path $SourceDir "Injector.exe"
if (-not (Test-Path -LiteralPath $InjectorExe)) {
    throw "Injector.exe not found at $InjectorExe. Build the Release client first."
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

$MsiPath = Join-Path $OutputDir "Lingqiao-$Version.msi"
$BundlePath = Join-Path $OutputDir "LingqiaoSetup-$Version.exe"
$ProductWxs = Join-Path $PSScriptRoot "Product.wxs"
$BundleWxs = Join-Path $PSScriptRoot "Bundle.wxs"

Write-Host "[1/3] Building MSI $MsiPath" -ForegroundColor Cyan
& $WixExe build $ProductWxs `
    -d "ProductVersion=$Version" `
    -d "SourceDir=$SourceDir" `
    -o $MsiPath
if ($LASTEXITCODE -ne 0) { throw "WiX MSI build failed" }

Write-Host "[2/3] Building Burn bundle $BundlePath" -ForegroundColor Cyan
& $WixExe build $BundleWxs `
    -ext WixToolset.Bal.wixext `
    -d "ProductVersion=$Version" `
    -d "MsiPath=$MsiPath" `
    -o $BundlePath
if ($LASTEXITCODE -ne 0) { throw "WiX bundle build failed" }

function Invoke-CodeSignIfConfigured([string]$Path) {
    $signtool = $env:SIGNTOOL_PATH
    $thumbprint = $env:SIGN_CERT_SHA1
    if (-not $signtool -or -not $thumbprint) {
        Write-Host "  Signing skipped for $(Split-Path -Leaf $Path): SIGNTOOL_PATH or SIGN_CERT_SHA1 not set" -ForegroundColor DarkYellow
        return
    }
    $timestamp = if ($env:SIGN_TIMESTAMP_URL) { $env:SIGN_TIMESTAMP_URL } else { "http://timestamp.digicert.com" }
    & $signtool sign /sha1 $thumbprint /fd SHA256 /tr $timestamp /td SHA256 $Path
    if ($LASTEXITCODE -ne 0) { throw "Code signing failed for $Path" }
}

Invoke-CodeSignIfConfigured $MsiPath
Invoke-CodeSignIfConfigured $BundlePath

$sha = [System.Security.Cryptography.SHA256]::Create()
$bundleBytes = [System.IO.File]::ReadAllBytes($BundlePath)
$hash = ($sha.ComputeHash($bundleBytes) | ForEach-Object { $_.ToString("x2") }) -join ""
$info = [ordered]@{
    version = $Version
    bundle = (Split-Path -Leaf $BundlePath)
    msi = (Split-Path -Leaf $MsiPath)
    sha256 = $hash
    size = $bundleBytes.Length
    built_at = (Get-Date).ToString("o")
}
$infoPath = Join-Path $OutputDir "release-info.json"
$info | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $infoPath -Encoding UTF8

Write-Host "[3/3] Installer ready" -ForegroundColor Green
Write-Host "  Bundle: $BundlePath"
Write-Host "  MSI:    $MsiPath"
Write-Host "  SHA256: $hash"
Write-Host "  Info:   $infoPath"
