param(
    [Parameter(Mandatory=$true)] [string]$BinaryPath,
    [ValidateSet('exe','dll','both')] [string]$Target = 'both'
)

<#
.SYNOPSIS
    Post-build binary obfuscation for Injector/CefHook.

.DESCRIPTION
    Step 1: Strip debug info (PDB)
    Step 2: Encrypt sensitive string sections with XOR
    Step 3: UPX pack with custom key (optional)

.EXAMPLE
    .\obfuscate_build.ps1 -BinaryPath C:\build\Release\Injector.exe
#>

$ErrorActionPreference = 'Stop'

# ============================================================
# Configuration
# ============================================================
$UPX_PATH = "upx.exe"  # Put UPX on PATH or specify full path

# Sensitive strings to find and warn about (for verification)
$SENSITIVE_PATTERNS = @(
    'injector_v1',
    'api\.deepseek\.com',
    'deepseek-v4-pro',
    'sk-[a-zA-Z0-9]{20,}'
)
if ($env:LINGQIAO_SERVER_HOST) {
    $SENSITIVE_PATTERNS += [regex]::Escape($env:LINGQIAO_SERVER_HOST)
}
if ($env:HMAC_SECRET) {
    $SENSITIVE_PATTERNS += [regex]::Escape($env:HMAC_SECRET)
}

# ============================================================
# Step 1: Audit exposed strings
# ============================================================
Write-Host "`n[1/4] Auditing exposed strings in $BinaryPath ..." -ForegroundColor Cyan

$binaryBytes = [System.IO.File]::ReadAllBytes($BinaryPath)
$binaryText = [System.Text.Encoding]::ASCII.GetString($binaryBytes)

foreach ($pattern in $SENSITIVE_PATTERNS) {
    $matches = [regex]::Matches($binaryText, $pattern)
    if ($matches.Count -gt 0) {
        Write-Host "  ⚠ EXPOSED: '$pattern' found $($matches.Count) time(s)" -ForegroundColor Red
    } else {
        Write-Host "  ✓ Not found: $pattern" -ForegroundColor Green
    }
}

# Count total printable strings > 8 chars (typical sensitive strings)
$strings = [regex]::Matches($binaryText, '[\x20-\x7E]{16,}') | ForEach-Object { $_.Value }
Write-Host "  ℹ Total printable ASCII strings (>=16 chars): $($strings.Count)" -ForegroundColor Gray

# ============================================================
# Step 2: Strip debug symbols
# ============================================================
Write-Host "`n[2/4] Stripping debug info..." -ForegroundColor Cyan

$pdbPath = [System.IO.Path]::ChangeExtension($BinaryPath, '.pdb')
if (Test-Path $pdbPath) {
    Remove-Item $pdbPath -Force
    Write-Host "  ✓ Removed $pdbPath" -ForegroundColor Green
} else {
    Write-Host "  - No PDB found" -ForegroundColor Gray
}

# ============================================================
# Step 3: XOR-encrypt .rdata section
# ============================================================
Write-Host "`n[3/4] XOR-encrypting readable string sections..." -ForegroundColor Cyan
Write-Host "  NOTE: This requires a PE section editor (e.g., pefile for Python)." -ForegroundColor Yellow
Write-Host "  Skipping automatic encryption. Use VMProtect/Themida for strong protection." -ForegroundColor Yellow

<#
Manual .rdata encryption approach (requires pyelftools/pefile):

from pefile import PE
import os

pe = PE(binary_path)
key = os.urandom(32)  # random 32-byte XOR key

for section in pe.sections:
    if b'.rdata' in section.Name:
        data = section.get_data()
        encrypted = bytes(d ^ key[i % len(key)] for i, d in enumerate(data))
        pe.set_bytes_at_offset(section.PointerToRawData, encrypted)
        break

# The key must be embedded in a stub that decrypts at runtime
# This requires a small assembly trampoline inserted into the PE
# For details: https://github.com/erocarrera/pefile
#>

# ============================================================
# Step 4: UPX packing (optional)
# ============================================================
Write-Host "`n[4/4] UPX packing..." -ForegroundColor Cyan

$upxFound = Get-Command $UPX_PATH -ErrorAction SilentlyContinue
if ($upxFound) {
    $backupPath = "$BinaryPath.bak"
    Copy-Item $BinaryPath $backupPath -Force

    # UPX with best compression, strip relocs, encrypt
    & $UPX_PATH --best --strip-relocs=0 --compress-icons=0 --force $BinaryPath

    if ($LASTEXITCODE -eq 0) {
        $origSize = (Get-Item $backupPath).Length
        $newSize  = (Get-Item $BinaryPath).Length
        $ratio    = [math]::Round((1 - $newSize / $origSize) * 100, 1)
        Write-Host "  ✓ UPX packed: ${origSize}KB → ${newSize}KB ($ratio% smaller)" -ForegroundColor Green
        Write-Host "  Backup: $backupPath" -ForegroundColor Gray
    } else {
        Write-Host "  ✗ UPX failed with exit code $LASTEXITCODE" -ForegroundColor Red
        Copy-Item $backupPath $BinaryPath -Force
        Remove-Item $backupPath -Force
    }
} else {
    Write-Host "  - UPX not found. Download from: https://upx.github.io/" -ForegroundColor Yellow
    Write-Host "  - Or use commercial: VMProtect / Themida / Enigma Protector" -ForegroundColor Yellow
}

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "  Post-build obfuscation complete" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "`nRecommendation: For production, use one of:" -ForegroundColor White
Write-Host "  1. VMProtect (vmprotect.ru) — strongest, ~$200" -ForegroundColor Yellow
Write-Host "  2. Themida (oreans.com) — strong, ~$200" -ForegroundColor Yellow
Write-Host "  3. UPX + custom stub — free but reversible" -ForegroundColor Yellow
Write-Host "  4. String encryption at build time — modify CMakeLists.txt" -ForegroundColor Yellow
