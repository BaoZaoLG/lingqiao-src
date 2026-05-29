param(
    [Parameter(Mandatory=$true)] [string]$InputExe,
    [string]$OutputExe
)

$ErrorActionPreference = 'Stop'

if (-not $OutputExe) {
    $dir = Split-Path $InputExe
    $name = [System.IO.Path]::GetFileNameWithoutExtension($InputExe)
    $OutputExe = Join-Path $dir "${name}_protected.exe"
}

Write-Host "[1/3] Reading PE ..." -ForegroundColor Cyan
$bytes = [System.IO.File]::ReadAllBytes((Resolve-Path $InputExe))
$origSize = $bytes.Length
Write-Host "  Size: $origSize bytes" -ForegroundColor Gray

# Check PE magic
if ($bytes[0] -ne 0x4D -or $bytes[1] -ne 0x5A) { throw "Not a valid PE file" }
$peOff = [BitConverter]::ToInt32($bytes, 0x3C)
$is64 = [BitConverter]::ToInt16($bytes, $peOff + 4) -eq 0x8664
$numSec = [BitConverter]::ToInt16($bytes, $peOff + 6)
$optSize = [BitConverter]::ToInt16($bytes, $peOff + 20)
$secOff = $peOff + 24 + $optSize
Write-Host "  PE${if($is64){'64'}else{'32'}}, $numSec sections" -ForegroundColor Gray

# Parse sections
$sections = @()
for ($i = 0; $i -lt $numSec; $i++) {
    $o = $secOff + $i * 40
    $sName = [System.Text.Encoding]::ASCII.GetString($bytes, $o, 8).TrimEnd([char]0)
    $sVirtSize  = [BitConverter]::ToInt32($bytes, $o + 8)
    $sVirtAddr  = [BitConverter]::ToInt32($bytes, $o + 12)
    $sRawSize   = [BitConverter]::ToInt32($bytes, $o + 16)
    $sRawOff    = [BitConverter]::ToInt32($bytes, $o + 20)
    $sFlags     = [BitConverter]::ToInt32($bytes, $o + 36)
    $sections += [PSCustomObject]@{
        Name=$sName; VirtSize=$sVirtSize; VirtAddr=$sVirtAddr;
        RawSize=$sRawSize; RawOff=$sRawOff; Flags=$sFlags; Index=$i
    }
    Write-Host "  Section[$i]: $sName raw=$sRawSize virt=$sVirtSize flags=0x$($sFlags.ToString('X8'))" -ForegroundColor Gray
}

# Find code section (.text or CODE)
$codeSec = $sections | Where-Object { $_.Name -eq '.text' -or $_.Name -eq 'CODE' } | Select-Object -First 1
if (-not $codeSec) { $codeSec = $sections[1] } # fallback to second section

Write-Host "  Target section: $($codeSec.Name)" -ForegroundColor Gray

# Extract code section
$codeData = New-Object byte[] $codeSec.RawSize
[Array]::Copy($bytes, $codeSec.RawOff, $codeData, 0, $codeSec.RawSize)

# Hash original code for integrity
$sha = [System.Security.Cryptography.SHA256]::Create()
$codeHash = $sha.ComputeHash($codeData)
Write-Host "  Code SHA256: $([BitConverter]::ToString($codeHash).Replace('-','').Substring(0,16))..." -ForegroundColor Gray

Write-Host "[2/3] Compressing + Encrypting ..." -ForegroundColor Cyan

# Compress
$ms = New-Object System.IO.MemoryStream
$ds = New-Object System.IO.Compression.DeflateStream($ms, [System.IO.Compression.CompressionLevel]::Optimal)
$ds.Write($codeData, 0, $codeData.Length)
$ds.Close()
$compressed = $ms.ToArray()
$ms.Close()
Write-Host "  $($codeData.Length) -> $($compressed.Length) bytes ($([math]::Round($compressed.Length/$codeData.Length*100,1))%)" -ForegroundColor Gray

# XOR encrypt with random key
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$xorKey = New-Object byte[] 32; $rng.GetBytes($xorKey)
for ($i = 0; $i -lt $compressed.Length; $i++) {
    $compressed[$i] = $compressed[$i] -bxor $xorKey[$i % 32]
}

# Build packed payload: [4B original_size][32B xor_key][32B sha256][compressed_data]
$payload = New-Object System.IO.MemoryStream
$payload.Write([BitConverter]::GetBytes([int]$codeData.Length), 0, 4)
$payload.Write($xorKey, 0, 32)
$payload.Write($codeHash, 0, 32)
$payload.Write($compressed, 0, $compressed.Length)
$packedPayload = $payload.ToArray()
$payload.Close()

Write-Host "[3/3] Building protected PE ..." -ForegroundColor Cyan

# Create output: copy original, replace code section with packed data
$outBytes = New-Object byte[] $bytes.Length
[Array]::Copy($bytes, $outBytes, $bytes.Length)

if ($packedPayload.Length -le $codeSec.RawSize) {
    # Fit in original section
    [Array]::Copy($packedPayload, 0, $outBytes, $codeSec.RawOff, $packedPayload.Length)
    # Zero remaining space
    for ($i = $packedPayload.Length; $i -lt $codeSec.RawSize; $i++) {
        $outBytes[$codeSec.RawOff + $i] = 0
    }
    Write-Host "  Packed payload fits in section ($($packedPayload.Length) <= $($codeSec.RawSize))" -ForegroundColor Gray
} else {
    # Need to extend the section
    Write-Host "  WARNING: payload ($($packedPayload.Length)) > section ($($codeSec.RawSize)), using overlay" -ForegroundColor Yellow
    # Append to end of file and update section pointer
    $newFile = New-Object byte[] ($bytes.Length + $packedPayload.Length)
    [Array]::Copy($bytes, $newFile, $bytes.Length)
    [Array]::Copy($packedPayload, 0, $newFile, $bytes.Length, $packedPayload.Length)

    # Update section raw size and pointer
    $newRawOff = $bytes.Length
    $newRawSize = $packedPayload.Length
    [BitConverter]::GetBytes($newRawSize).CopyTo($newFile, $secOff + $codeSec.Index * 40 + 16)
    [BitConverter]::GetBytes($newRawOff).CopyTo($newFile, $secOff + $codeSec.Index * 40 + 20)

    $outBytes = $newFile
}

# Mark section as executable + readable + writable (for unpacking)
$flags = 0xE0000060 ; # IMAGE_SCN_MEM_READ | IMAGE_SCN_MEM_WRITE | IMAGE_SCN_MEM_EXECUTE | IMAGE_SCN_CNT_CODE
[BitConverter]::GetBytes($flags).CopyTo($outBytes, $secOff + $codeSec.Index * 40 + 36)

[System.IO.File]::WriteAllBytes($OutputExe, $outBytes)

$outSize = (Get-Item $OutputExe).Length
Write-Host ""
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "  Output: $OutputExe" -ForegroundColor Green
Write-Host "  Original: $origSize bytes" -ForegroundColor Gray
Write-Host "  Protected: $outSize bytes" -ForegroundColor Gray
Write-Host "  Section: $($codeSec.Name) $($codeData.Length) -> $($compressed.Length) bytes" -ForegroundColor Gray
Write-Host "  XOR key: 32 bytes random" -ForegroundColor Gray
Write-Host "  Integrity: SHA-256 hash embedded" -ForegroundColor Gray
Write-Host "============================================" -ForegroundColor Cyan
