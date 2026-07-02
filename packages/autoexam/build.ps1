param (
    [string]$SourceFile = (Join-Path $PSScriptRoot "..\..\AutoExam.js"),
    [string]$OutDir = (Join-Path $PSScriptRoot "dist")
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $SourceFile)) {
    Write-Error "Source file not found: $SourceFile"
}

# Create directories
$SrcDir = Join-Path $PSScriptRoot "src"
if (-not (Test-Path $SrcDir)) { New-Item -ItemType Directory -Path $SrcDir | Out-Null }
if (-not (Test-Path $OutDir)) { New-Item -ItemType Directory -Path $OutDir | Out-Null }

$ResolvedSourceFile = (Resolve-Path $SourceFile).Path
$ScriptBytes = [System.IO.File]::ReadAllBytes($ResolvedSourceFile)

# 1. Copy source to src/ and dist/
Copy-Item $ResolvedSourceFile -Destination "$SrcDir\autoexam.js" -Force
Copy-Item $ResolvedSourceFile -Destination "$OutDir\autoexam.bundle.js" -Force

# 2. Calculate SHA256 and version metadata
$HashObj = [System.Security.Cryptography.SHA256]::Create()
$HashBytes = $HashObj.ComputeHash($ScriptBytes)
$Sha256Hex = [System.BitConverter]::ToString($HashBytes).Replace("-", "").ToLower()

$Version = Get-Date -Format "yyyyMMdd.HHmmss"

$Manifest = @{
    version = $Version
    sha256 = $Sha256Hex
    size = $ScriptBytes.Length
    created_at = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
}
$Manifest | ConvertTo-Json | Out-File -FilePath "$OutDir\autoexam.manifest.json" -Encoding utf8 -Force

$EmbeddedHeaderPath = Join-Path $OutDir "autoexam.embedded.h"
if (Test-Path $EmbeddedHeaderPath) {
    Remove-Item -LiteralPath $EmbeddedHeaderPath -Force
}

Write-Host "Done building autoexam package."
