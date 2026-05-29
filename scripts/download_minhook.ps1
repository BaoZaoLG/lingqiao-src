# ============================================================================
# download_minhook.ps1 — Download and extract MinHook library
#
# Downloads MinHook from GitHub and places the source files in the correct
# vendor directory for building with the CefHook DLL.
#
# Usage: powershell -NoProfile -ExecutionPolicy Bypass -File scripts/download_minhook.ps1
# ============================================================================

$ErrorActionPreference = "Stop"

$MINHOOK_URL = "https://github.com/TsudaKageyu/MinHook/archive/refs/heads/master.zip"
$VENDOR_DIR = "$PSScriptRoot\..\src\vendor\minhook"
$TEMP_ZIP = "$env:TEMP\minhook_download.zip"
$TEMP_DIR = "$env:TEMP\minhook_extract"

Write-Host "Downloading MinHook..." -ForegroundColor Cyan

# Download
[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12
Invoke-WebRequest -Uri $MINHOOK_URL -OutFile $TEMP_ZIP -UseBasicParsing

Write-Host "Extracting..." -ForegroundColor Cyan

# Clean temp
if (Test-Path $TEMP_DIR) { Remove-Item $TEMP_DIR -Recurse -Force }
Expand-Archive -Path $TEMP_ZIP -DestinationPath $TEMP_DIR -Force

# Find the extracted folder (MinHook-master)
$srcDir = "$TEMP_DIR\MinHook-master"

if (!(Test-Path $srcDir)) {
    Write-Host "ERROR: MinHook source not found after extraction" -ForegroundColor Red
    exit 1
}

# Copy header
Write-Host "Installing MinHook header..." -ForegroundColor Cyan
Copy-Item "$srcDir\include\MinHook.h" "$VENDOR_DIR\include\MinHook.h" -Force

# Copy source files
Write-Host "Installing MinHook source..." -ForegroundColor Cyan
Copy-Item "$srcDir\src\hook.c" "$VENDOR_DIR\src\hook.c" -Force
Copy-Item "$srcDir\src\buffer.c" "$VENDOR_DIR\src\buffer.c" -Force
Copy-Item "$srcDir\src\trampoline.c" "$VENDOR_DIR\src\trampoline.c" -Force
Copy-Item "$srcDir\src\trampoline.h" "$VENDOR_DIR\src\trampoline.h" -Force
Copy-Item "$srcDir\src\buffer.h" "$VENDOR_DIR\src\buffer.h" -Force

# Copy HDE (Hacker Disassembler Engine)
Write-Host "Installing HDE (Hacker Disassembler Engine)..." -ForegroundColor Cyan
$hdeDir = "$VENDOR_DIR\src\hde"
if (!(Test-Path $hdeDir)) { New-Item -ItemType Directory -Path $hdeDir -Force | Out-Null }

# Check for HDE64 source
if (Test-Path "$srcDir\src\hde") {
    Copy-Item "$srcDir\src\hde\*" $hdeDir -Force
} elseif (Test-Path "$srcDir\src\hde64.c") {
    Copy-Item "$srcDir\src\hde64.c" "$hdeDir\hde64.c" -Force
    Copy-Item "$srcDir\src\hde64.h" "$hdeDir\hde64.h" -Force
    Copy-Item "$srcDir\src\hde64_table.h" "$hdeDir\hde64_table.h" -Force
    if (Test-Path "$srcDir\src\hde32.c") {
        Copy-Item "$srcDir\src\hde32.c" "$hdeDir\hde32.c" -Force
        Copy-Item "$srcDir\src\hde32.h" "$hdeDir\hde32.h" -Force
        Copy-Item "$srcDir\src\hde32_table.h" "$hdeDir\hde32_table.h" -Force
    }
} else {
    # MinHook may have a single HDE implementation
    Get-ChildItem "$srcDir\src" -Filter "hde*" | ForEach-Object {
        Copy-Item $_.FullName $hdeDir -Force
    }
}

# Cleanup
Remove-Item $TEMP_ZIP -Force -ErrorAction SilentlyContinue
Remove-Item $TEMP_DIR -Recurse -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "MinHook installed successfully!" -ForegroundColor Green
Write-Host "  Header: $VENDOR_DIR\include\MinHook.h" -ForegroundColor Gray
Write-Host "  Source: $VENDOR_DIR\src\" -ForegroundColor Gray
Write-Host ""
Write-Host "You can now build the project with CMake." -ForegroundColor Cyan
