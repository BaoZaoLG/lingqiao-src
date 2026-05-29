# build_and_push.ps1 — 一键编译 + 上传新版本到服务器
# 用法: pwsh scripts/build_and_push.ps1 -Version "2.1.6"
#   或: pwsh scripts/build_and_push.ps1   (自动递增版本号)

param(
    [string]$Version = "",
    [string]$Server = "root@47.110.248.240",
    [string]$RemotePath = "/opt/injector-server/data/updates"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$CMakeLists = Join-Path $ProjectRoot "CMakeLists.txt"

# ── 1. 确定版本号 ──
if (-not $Version) {
    $content = Get-Content $CMakeLists -Raw
    if ($content -match 'set\(APP_VERSION "(\d+)\.(\d+)\.(\d+)"\)') {
        $major = [int]$Matches[1]
        $minor = [int]$Matches[2]
        $patch = [int]$Matches[3] + 1
        $Version = "$major.$minor.$patch"
    } else {
        Write-Error "无法从 CMakeLists.txt 解析版本号"
        exit 1
    }
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  构建版本: v$Version" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# ── 2. 更新 CMakeLists.txt 版本号 ──
$content = Get-Content $CMakeLists -Raw
$content = $content -replace 'set\(APP_VERSION "\d+\.\d+\.\d+"\)', "set(APP_VERSION `"$Version`")"
Set-Content $CMakeLists -Value $content -NoNewline
Write-Host "[1/5] 版本号更新为 v$Version" -ForegroundColor Green

# ── 3. CMake 配置 ──
Write-Host "[2/5] CMake 配置..." -ForegroundColor Yellow
Push-Location $ProjectRoot
try {
    & cmake . -G "Visual Studio 17 2022" -A Win32 -DCMAKE_PREFIX_PATH="C:/Qt/5.15.2/msvc2019" 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "CMake 配置失败" }
    Write-Host "  CMake 配置完成" -ForegroundColor DarkGray
} finally {
    Pop-Location
}

# ── 4. 编译 ──
Write-Host "[3/5] 编译 Release..." -ForegroundColor Yellow
Push-Location $ProjectRoot
try {
    & cmake --build . --config Release 2>&1 | ForEach-Object {
        if ($_ -match "error") { Write-Host "  $_" -ForegroundColor Red }
    }
    if ($LASTEXITCODE -ne 0) { throw "编译失败" }
} finally {
    Pop-Location
}

# ── 4b. 验证服务器当前版本，确保新版本更高 ──
Write-Host "[3.5/5] 检查服务器当前版本..." -ForegroundColor Yellow
$RemoteVersionCheck = & ssh -o StrictHostKeyChecking=no $Server "cat /opt/injector-server/data/updates/info.json 2>/dev/null | python3 -c 'import sys,json; print(json.load(sys.stdin).get(\"version\",\"\"))' 2>/dev/null || echo ''" 2>&1
$RemoteVersion = ($RemoteVersionCheck -join "").Trim()
if ($RemoteVersion) {
    Write-Host "  服务器当前版本: v$RemoteVersion" -ForegroundColor DarkGray
    # Compare versions
    $rv = $RemoteVersion.Split('.') | ForEach-Object { [int]$_ }
    $nv = $Version.Split('.') | ForEach-Object { [int]$_ }
    $isNewer = $false
    for ($i = 0; $i -lt [Math]::Max($rv.Length, $nv.Length); $i++) {
        $a = if ($i -lt $nv.Length) { $nv[$i] } else { 0 }
        $b = if ($i -lt $rv.Length) { $rv[$i] } else { 0 }
        if ($a -gt $b) { $isNewer = $true; break }
        if ($a -lt $b) { break }
    }
    if (-not $isNewer) {
        Write-Error "版本 v$Version 不高于服务器的 v$RemoteVersion！请使用更高的版本号。"
        exit 1
    }
}

$ExePath = Join-Path $ProjectRoot "src\Release\Injector.exe"
if (-not (Test-Path $ExePath)) {
    Write-Error "编译产物不存在: $ExePath"
    exit 1
}
$ExeSize = (Get-Item $ExePath).Length
Write-Host "  编译成功: $ExePath ($([math]::Round($ExeSize/1MB, 1)) MB)" -ForegroundColor DarkGray

# ── 5. 上传到服务器 ──
$RemoteName = "Injector_v$Version.exe"
Write-Host "[4/5] 上传到服务器..." -ForegroundColor Yellow

& scp -o StrictHostKeyChecking=no $ExePath "${Server}:${RemotePath}/${RemoteName}" 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) { throw "上传失败" }
Write-Host "  上传完成: ${RemotePath}/${RemoteName}" -ForegroundColor DarkGray

# ── 6. 通知服务器更新版本信息 ──
Write-Host "[5/5] 更新服务器版本信息..." -ForegroundColor Yellow

$RemoteCmd = @"
cd /opt/injector-server && \
cat > /tmp/update_version.py << 'PYEOF'
import json, os, time

version = "$Version"
filename = "Injector_v${version}.exe"
filepath = os.path.join("data/updates", filename)
filesize = os.path.getsize(filepath) if os.path.exists(filepath) else 0

# Update info.json
info = {"version": version, "filename": filename, "file_size": filesize, "uploaded_at": time.strftime("%Y-%m-%dT%H:%M:%S+08:00")}
with open("data/updates/info.json", "w") as f:
    json.dump(info, f)

# Update announcement latest_version
try:
    with open("data/announcement.json", "r") as f:
        ann = json.load(f)
    ann["latest_version"] = version
    ann["download_url"] = "/admin/api/update/download"
    with open("data/announcement.json", "w") as f:
        json.dump(ann, f)
except:
    ann = {"content": "", "latest_version": version, "min_version": "", "force_update": False, "download_url": "/admin/api/update/download"}
    with open("data/announcement.json", "w") as f:
        json.dump(ann, f)

print(f"OK: version={version} file={filename} size={filesize}")
PYEOF
python3 /tmp/update_version.py && \
systemctl restart injector-server && \
echo "服务已重启"
"@

& ssh -o StrictHostKeyChecking=no $Server $RemoteCmd 2>&1
if ($LASTEXITCODE -ne 0) { throw "服务器更新失败" }

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  v$Version 发布成功!" -ForegroundColor Green
Write-Host "  客户端将在下次心跳时收到更新通知" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
