param(
    [Parameter(Mandatory=$true)] [string]$String,
    [switch]$Wide
)

<#
.SYNOPSIS
    Encrypt a string and output it as an EncryptedBlob C/C++ initializer.

.EXAMPLE
    # Encrypt the HMAC secret as ANSI
    .\encrypt_strings.ps1 -String "c1a3f8e9d2b47a6e8f0c3d5b9a1e4f7a8b2c6d0e3f5a7b9c1d4e6f8a0b2c4d6"

    # Encrypt a path as wide string
    .\encrypt_strings.ps1 -String "/api/v1/activate" -Wide
#>

$ErrorActionPreference = 'Stop'

# Generate random 64-bit key
$keyBytes = New-Object byte[] 8
[System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($keyBytes)
$key = [System.BitConverter]::ToUInt64($keyBytes, 0)

# Encode to bytes (ANSI or UTF-16LE for wide)
if ($Wide) {
    $bytes = [System.Text.Encoding]::Unicode.GetBytes($String)
} else {
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($String)
}

# XOR encrypt
$kb = [System.BitConverter]::GetBytes($key)
for ($i = 0; $i -lt $bytes.Length; $i++) {
    $bytes[$i] = $bytes[$i] -bxor $kb[$i -band 7] -bxor ([byte](($i * 0x9D) -band 0xFF))
}

# Format as C array initializer
$hexLines = @()
for ($i = 0; $i -lt $bytes.Length; $i += 16) {
    $line = "    "
    for ($j = $i; $j -lt [Math]::Min($i + 16, $bytes.Length); $j++) {
        $line += "0x{0:X2}, " -f $bytes[$j]
    }
    $hexLines += $line.TrimEnd()
}

$hexStr = $hexLines -join "`n"

$count = $bytes.Length
$keyHex = "0x{0:X16}" -f $key

@"
static const BYTE __enc_data_[] = {
${hexStr}
};

static const EncryptedBlob g_enc = {
    __enc_data_,
    ${count},   // byte count
    ${keyHex}ULL
};
"@

Write-Host "Generated encrypted blob: ${count} bytes, key=${keyHex}" -ForegroundColor Green
