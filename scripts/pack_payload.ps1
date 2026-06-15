param(
    [Parameter(Mandatory=$true)] [string]$InputDll,
    [string]$OutputDir,
    [string]$PayloadId = "payload-$(Get-Date -Format 'yyyyMMdd-HHmmss')",
    [int]$ChunkSize = 512000
)

<#
.SYNOPSIS
    Package CefHook.dll for injector-server payload upload.
    - Compress (Deflate) + Encrypt (AES-256-CBC) the DLL
    - Split into chunk files
    - Output folder ready for server upload

.EXAMPLE
    .\pack_payload.ps1 -InputDll .\build\src\CefHook.dll
    .\pack_payload.ps1 -InputDll .\build\src\CefHook.dll -PayloadId "payload-v3" -OutputDir .\payload_out
#>

$ErrorActionPreference = 'Stop'

if (-not (Test-Path $InputDll)) {
    throw "Input DLL not found: $InputDll"
}

if (-not $OutputDir) {
    $OutputDir = Join-Path (Split-Path $InputDll) $PayloadId
}

# Create output directory
if (Test-Path $OutputDir) { Remove-Item $OutputDir -Recurse -Force }
New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null

Write-Host "============================================" -ForegroundColor Cyan
Write-Host "  Payload Packager" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""

# ── Step 1: Read and hash ─────────────────────────────────────────────
Write-Host "[1/5] Reading $InputDll ..." -ForegroundColor Cyan
$inputBytes = [System.IO.File]::ReadAllBytes((Resolve-Path $InputDll))
$inputSize = $inputBytes.Length
Write-Host "  Size: $inputSize bytes" -ForegroundColor Gray

$sha256 = [System.Security.Cryptography.SHA256]::Create()
$exeHash = $sha256.ComputeHash($inputBytes)
$exeHashHex = [System.BitConverter]::ToString($exeHash) -replace '-',''
Write-Host "  SHA256: $exeHashHex" -ForegroundColor Gray

# ── Step 2: Compress (Deflate) ────────────────────────────────────────
Write-Host "[2/5] Compressing (Deflate) ..." -ForegroundColor Cyan
$ms = New-Object System.IO.MemoryStream
$ds = New-Object System.IO.Compression.DeflateStream($ms, [System.IO.Compression.CompressionLevel]::Optimal)
$ds.Write($inputBytes, 0, $inputBytes.Length)
$ds.Close()
$compressed = $ms.ToArray()
$ms.Close()
$ratio = [math]::Round($compressed.Length / $inputBytes.Length * 100, 1)
Write-Host "  $inputSize -> $($compressed.Length) bytes ($ratio%)" -ForegroundColor Gray

# ── Step 3: Encrypt (AES-256-CBC) ────────────────────────────────────
Write-Host "[3/5] Encrypting (AES-256-CBC) ..." -ForegroundColor Cyan

$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$aesKey = New-Object byte[] 32; $rng.GetBytes($aesKey)
$iv = New-Object byte[] 16; $rng.GetBytes($iv)
$hmacKey = New-Object byte[] 32; $rng.GetBytes($hmacKey)

$aes = [System.Security.Cryptography.Aes]::Create()
$aes.KeySize = 256; $aes.Mode = [System.Security.Cryptography.CipherMode]::CBC
$aes.Padding = [System.Security.Cryptography.PaddingMode]::PKCS7
$aes.Key = $aesKey; $aes.IV = $iv
$encryptor = $aes.CreateEncryptor()
$encrypted = $encryptor.TransformFinalBlock($compressed, 0, $compressed.Length)
$aes.Dispose()

# HMAC over encrypted payload for integrity
$hmac = [System.Security.Cryptography.HMACSHA256]::new([byte[]]$hmacKey)
$hmacBytes = $hmac.ComputeHash($encrypted)
$hmac.Dispose()
Write-Host "  Encrypted: $($encrypted.Length) bytes" -ForegroundColor Gray

# ── Step 4: Split into chunks ────────────────────────────────────────
Write-Host "[4/5] Splitting into chunks (size=$ChunkSize) ..." -ForegroundColor Cyan

$totalSize = $encrypted.Length
$chunkCount = [math]::Ceiling($totalSize / $ChunkSize)
Write-Host "  Total chunks: $chunkCount" -ForegroundColor Gray

for ($i = 0; $i -lt $chunkCount; $i++) {
    $offset = $i * $ChunkSize
    $len = [math]::Min($ChunkSize, $totalSize - $offset)
    $chunk = New-Object byte[] $len
    [Array]::Copy($encrypted, $offset, $chunk, 0, $len)

    $chunkPath = Join-Path $OutputDir ("chunk_{0:D4}.bin" -f $i)
    [System.IO.File]::WriteAllBytes($chunkPath, $chunk)
    Write-Host ("  chunk_{0:D4}.bin  ({1} bytes)" -f $i, $len) -ForegroundColor Gray
}

# ── Step 5: Write metadata ──────────────────────────────────────────
Write-Host "[5/5] Writing metadata ..." -ForegroundColor Cyan

$metadata = @{
    payload_id  = $PayloadId
    aes_key     = [System.BitConverter]::ToString($aesKey) -replace '-',''
    hmac_key    = [System.BitConverter]::ToString($hmacKey) -replace '-',''
    iv          = [System.BitConverter]::ToString($iv) -replace '-',''
    exe_hash    = $exeHashHex
    chunk_count = $chunkCount
    chunk_size  = $ChunkSize
    total_size  = $totalSize
}

$metaPath = Join-Path $OutputDir "metadata.json"
$metadata | ConvertTo-Json | Out-File -FilePath $metaPath -Encoding UTF8
Write-Host "  metadata.json written" -ForegroundColor Gray

# ── Cleanup sensitive material ───────────────────────────────────────
$aesKey = $null; $hmacKey = $null; $iv = $null

# ── Summary ──────────────────────────────────────────────────────────
Write-Host ""
Write-Host "============================================" -ForegroundColor Green
Write-Host "  Done!" -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Green
Write-Host ""
Write-Host "  Output folder: $OutputDir" -ForegroundColor White
Write-Host "  Payload ID:    $PayloadId" -ForegroundColor White
Write-Host "  Original size: $inputSize bytes" -ForegroundColor Gray
Write-Host "  Chunks:        $chunkCount files" -ForegroundColor Gray
Write-Host ""
Write-Host "  Upload metadata:" -ForegroundColor Yellow
Write-Host "  ──────────────────────────────────────────" -ForegroundColor DarkGray
Write-Host "  payload_id : $($metadata.payload_id)" -ForegroundColor White
Write-Host "  exe_hash   : $($metadata.exe_hash)" -ForegroundColor White
Write-Host "  aes_key    : $($metadata.aes_key.Substring(0,16))..." -ForegroundColor White
Write-Host "  hmac_key   : $($metadata.hmac_key.Substring(0,16))..." -ForegroundColor White
Write-Host "  iv         : $($metadata.iv)" -ForegroundColor White
Write-Host ""
Write-Host "  Next steps:" -ForegroundColor Yellow
Write-Host "  1. Upload via API:  POST /admin/api/payload/upload" -ForegroundColor White
Write-Host "  2. Or manually copy folder to server data/payloads/" -ForegroundColor White
Write-Host "  3. Then activate:   POST /admin/api/payloads/manage" -ForegroundColor White
Write-Host "     {`"action`":`"activate`",`"payload_id`":`"$PayloadId`"}" -ForegroundColor DarkGray
Write-Host ""
