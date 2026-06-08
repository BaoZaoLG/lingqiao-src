# build_and_push.ps1 — 构建客户端并生成企业级安装包
# 用法: pwsh scripts/build_and_push.ps1 -Version "3.0.0"
# 旧版裸 exe 推送仅用于兼容旧客户端：追加 -LegacyPush

param(
    [string]$Version = "",
    [string]$Server = $env:LINGQIAO_DEPLOY_SERVER,
    [string]$RemoteRoot = "/opt/injector-server",
    [string]$RemoteDataDir = "data",
    [string]$BuildDir = "build-release",
    [string]$QtPrefixPath = $(if ($env:QT_PREFIX_PATH) { $env:QT_PREFIX_PATH } else { "C:/Qt/5.15.2/msvc2019" }),
    [switch]$BuildInstaller,
    [switch]$InstallerOnly,
    [switch]$LegacyPush
)

$ErrorActionPreference = "Stop"
if (-not $LegacyPush) {
    $BuildInstaller = $true
    $InstallerOnly = $true
}
if ($LegacyPush -and [string]::IsNullOrWhiteSpace($Server)) {
    throw "Server is required. Pass -Server or set LINGQIAO_DEPLOY_SERVER."
}
$ProjectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$CMakeLists = Join-Path $ProjectRoot "CMakeLists.txt"
$BuildPath = Join-Path $ProjectRoot $BuildDir
$RemotePath = "$RemoteRoot/$RemoteDataDir/updates"

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

# ── 2. CMake 配置（版本号通过缓存变量传入，不修改源码） ──
Write-Host "[1/5] CMake 配置 (v$Version)..." -ForegroundColor Yellow
Push-Location $ProjectRoot
try {
    & cmake -S . -B $BuildPath -G "Visual Studio 17 2022" -A Win32 -DCMAKE_PREFIX_PATH="$QtPrefixPath" -DAPP_VERSION="$Version" 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "CMake 配置失败" }
    Write-Host "  CMake 配置完成" -ForegroundColor DarkGray
} finally {
    Pop-Location
}

# ── 4. 编译 ──
Write-Host "[2/5] 编译 Release..." -ForegroundColor Yellow
Push-Location $ProjectRoot
try {
    & cmake --build $BuildPath --config Release --target Injector 2>&1 | ForEach-Object {
        if ($_ -match "error") { Write-Host "  $_" -ForegroundColor Red }
    }
    if ($LASTEXITCODE -ne 0) { throw "编译失败" }
} finally {
    Pop-Location
}

# ── 4b. 验证服务器当前版本，确保新版本更高 ──
Write-Host "[3.5/5] 检查服务器当前版本..." -ForegroundColor Yellow
$RemoteInfoPath = "$RemoteRoot/$RemoteDataDir/updates/info.json"
if ($LegacyPush) {
    $RemoteVersionCheck = & ssh -o StrictHostKeyChecking=no $Server "cat '$RemoteInfoPath' 2>/dev/null | python3 -c 'import sys,json; print(json.load(sys.stdin).get(\"version\",\"\"))' 2>/dev/null || echo ''" 2>&1
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
} else {
    Write-Host "  跳过旧服务器版本检查；新流程通过管理后台 Release 灰度发布。" -ForegroundColor DarkGray
}

$ExePath = Join-Path $BuildPath "src\Release\CX_LingQiao.exe"
if (-not (Test-Path $ExePath)) {
    Write-Error "编译产物不存在: $ExePath"
    exit 1
}
$ExeSize = (Get-Item $ExePath).Length
Write-Host "  编译成功: $ExePath ($([math]::Round($ExeSize/1MB, 1)) MB)" -ForegroundColor DarkGray

if ($BuildInstaller -or $InstallerOnly) {
    Write-Host "[installer] 构建 WiX/Burn 安装包..." -ForegroundColor Yellow
    $InstallerScript = Join-Path $ProjectRoot "installer\build_installer.ps1"
    & powershell -NoProfile -ExecutionPolicy Bypass -File $InstallerScript -Version $Version -SourceDir (Split-Path -Parent $ExePath)
    if ($LASTEXITCODE -ne 0) { throw "安装包构建失败" }
    if ($InstallerOnly) {
        Write-Host ""
        Write-Host "安装包已生成。请在管理后台创建 Release 并上传 dist\installer\LingqiaoSetup-$Version.exe。" -ForegroundColor Green
        exit 0
    }
    Write-Host "安装包已生成；下面继续执行旧版裸 exe 推送，仅用于兼容旧客户端。" -ForegroundColor DarkYellow
}

# ── 5. 上传到服务器 ──
$RemoteName = "Injector_v$Version.exe"
Write-Host "[4/5] 上传到服务器..." -ForegroundColor Yellow

& scp -o StrictHostKeyChecking=no $ExePath "${Server}:${RemotePath}/${RemoteName}" 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) { throw "上传失败" }
Write-Host "  上传完成: ${RemotePath}/${RemoteName}" -ForegroundColor DarkGray

# ── 6. 通知服务器更新版本信息 ──
Write-Host "[5/5] 更新服务器版本信息..." -ForegroundColor Yellow

$RemoteCmd = @"
cd "$RemoteRoot" && \
cat > /tmp/update_version.py << 'PYEOF'
import hashlib, json, os, time

version = "$Version"
filename = "Injector_v${version}.exe"
data_dir = "$RemoteDataDir"
update_dir = os.path.join(data_dir, "updates")
filepath = os.path.join(update_dir, filename)
filesize = os.path.getsize(filepath) if os.path.exists(filepath) else 0
with open(filepath, "rb") as f:
    sha256 = hashlib.sha256(f.read()).hexdigest()

# Update info.json
info = {"version": version, "filename": filename, "file_size": filesize, "sha256": sha256, "uploaded_at": time.strftime("%Y-%m-%dT%H:%M:%S+08:00")}
with open(os.path.join(update_dir, "info.json"), "w") as f:
    json.dump(info, f)

# Update package index
index_path = os.path.join(update_dir, "index.json")
try:
    with open(index_path, "r") as f:
        index = json.load(f)
except:
    index = {"active_version": "", "packages": []}
packages = [p for p in index.get("packages", []) if p.get("version") != version]
packages.insert(0, {
    "version": version,
    "filename": filename,
    "file_size": filesize,
    "sha256": sha256,
    "uploaded_at": info["uploaded_at"],
    "active": True,
})
index["active_version"] = version
for p in packages:
    p["active"] = p.get("version") == version
index["packages"] = packages
with open(index_path, "w") as f:
    json.dump(index, f)

# Update announcement latest_version
try:
    with open(os.path.join(data_dir, "announcement.json"), "r") as f:
        ann = json.load(f)
    ann["latest_version"] = version
    ann["download_url"] = "/admin/api/update/download"
    ann["sha256"] = sha256
    with open(os.path.join(data_dir, "announcement.json"), "w") as f:
        json.dump(ann, f)
except:
    ann = {"content": "", "latest_version": version, "min_version": "", "force_update": False, "download_url": "/admin/api/update/download", "sha256": sha256}
    with open(os.path.join(data_dir, "announcement.json"), "w") as f:
        json.dump(ann, f)

print(f"OK: version={version} file={filename} size={filesize} sha256={sha256}")
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
