param(
    [string]$Version = "",
    [string]$SourceDir = "",
    [string]$OutputDir = ""
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
    $SourceDir = Join-Path $Root "build\src\Release"
}
if (-not $OutputDir) {
    $OutputDir = Join-Path $Root "dist\installer"
}

$SourceDir = [System.IO.Path]::GetFullPath($SourceDir)
$OutputDir = [System.IO.Path]::GetFullPath($OutputDir)
$AppExe = Join-Path $SourceDir "CX_LingQiao.exe"
$QtPlatform = Join-Path $SourceDir "platforms\qwindows.dll"
if (-not (Test-Path -LiteralPath $AppExe)) {
    throw "CX_LingQiao.exe not found at $AppExe. Build the Release client first."
}
if (-not (Test-Path -LiteralPath $QtPlatform)) {
    throw "Qt platform plugin not found at $QtPlatform. Run the Release deployment step first."
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$WorkDir = Join-Path ([System.IO.Path]::GetTempPath()) ("lingqiao_setup_" + [Guid]::NewGuid().ToString("N"))
$StageDir = Join-Path $WorkDir "app"
$ZipPath = Join-Path $WorkDir "app.zip"
$SetupCs = Join-Path $WorkDir "LingqiaoSetup.cs"
$SetupExe = Join-Path $OutputDir "LingqiaoSetup-$Version.exe"
New-Item -ItemType Directory -Force -Path $StageDir | Out-Null

try {
    Copy-Item -Path (Join-Path $SourceDir "*") -Destination $StageDir -Recurse -Force
    Compress-Archive -Path (Join-Path $StageDir "*") -DestinationPath $ZipPath -Force
    if (-not (Test-Path -LiteralPath $ZipPath)) {
        throw "Failed to create embedded app.zip"
    }

    $source = @'
using System;
using System.Diagnostics;
using System.IO;
using System.IO.Compression;
using System.Reflection;
using System.Windows.Forms;

internal static class LingqiaoSetup {
    [STAThread]
    private static int Main() {
        try {
            string installDir = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "Lingqiao");
            Directory.CreateDirectory(installDir);
            using (Stream zip = Assembly.GetExecutingAssembly().GetManifestResourceStream("app.zip")) {
                if (zip == null) throw new InvalidOperationException("embedded package missing");
                string tempZip = Path.Combine(Path.GetTempPath(), "LingqiaoSetup_" + Guid.NewGuid().ToString("N") + ".zip");
                using (FileStream fs = File.Create(tempZip)) zip.CopyTo(fs);
                try {
                    using (ZipArchive archive = ZipFile.OpenRead(tempZip)) {
                        foreach (ZipArchiveEntry entry in archive.Entries) {
                            string target = Path.GetFullPath(Path.Combine(installDir, entry.FullName));
                            string root = Path.GetFullPath(installDir) + Path.DirectorySeparatorChar;
                            if (!target.StartsWith(root, StringComparison.OrdinalIgnoreCase)) {
                                throw new InvalidOperationException("invalid package path");
                            }
                            if (String.IsNullOrEmpty(entry.Name)) {
                                Directory.CreateDirectory(target);
                                continue;
                            }
                            Directory.CreateDirectory(Path.GetDirectoryName(target));
                            entry.ExtractToFile(target, true);
                        }
                    }
                } finally {
                    try { File.Delete(tempZip); } catch {}
                }
            }
            string exe = Path.Combine(installDir, "CX_LingQiao.exe");
            if (!File.Exists(exe)) throw new FileNotFoundException("CX_LingQiao.exe missing after install", exe);
            Process.Start(new ProcessStartInfo(exe) { WorkingDirectory = installDir, UseShellExecute = true });
            return 0;
        } catch (Exception ex) {
            MessageBox.Show("Install failed:\r\n" + ex.Message, "Lingqiao Setup", MessageBoxButtons.OK, MessageBoxIcon.Error);
            return 1;
        }
    }
}
'@
    Set-Content -LiteralPath $SetupCs -Value $source -Encoding UTF8

    $cscPaths = @(
        "E:/Visual studio 2022/MSBuild/Current/Bin/Roslyn/csc.exe",
        "C:/Program Files/Microsoft Visual Studio/2022/Community/MSBuild/Current/Bin/Roslyn/csc.exe",
        "C:/Program Files/Microsoft Visual Studio/2022/Professional/MSBuild/Current/Bin/Roslyn/csc.exe",
        "C:/Program Files/Microsoft Visual Studio/2022/Enterprise/MSBuild/Current/Bin/Roslyn/csc.exe",
        "C:/Program Files/Microsoft Visual Studio/2022/BuildTools/MSBuild/Current/Bin/Roslyn/csc.exe",
        "C:/Windows/Microsoft.NET/Framework/v4.0.30319/csc.exe"
    )
    $cscExe = $null
    foreach ($p in $cscPaths) {
        if (Test-Path -LiteralPath $p) { $cscExe = $p; break }
    }
    if (-not $cscExe) { throw "Cannot find csc.exe" }

    & $cscExe /nologo /target:winexe /optimize+ /platform:x86 `
        /r:System.IO.Compression.dll /r:System.IO.Compression.FileSystem.dll /r:System.Windows.Forms.dll `
        "/resource:$ZipPath,app.zip" "/out:$SetupExe" $SetupCs
    if ($LASTEXITCODE -ne 0) { throw "Portable setup build failed" }

    $bundleBytes = [System.IO.File]::ReadAllBytes($SetupExe)
    $sha = [System.Security.Cryptography.SHA256]::Create()
    $hash = ($sha.ComputeHash($bundleBytes) | ForEach-Object { $_.ToString("x2") }) -join ""
    $info = [ordered]@{
        version = $Version
        bundle = (Split-Path -Leaf $SetupExe)
        sha256 = $hash
        size = $bundleBytes.Length
        built_at = (Get-Date).ToString("o")
        includes = @("CX_LingQiao.exe", "platforms/qwindows.dll", "Qt5Core.dll", "Qt5Gui.dll", "Qt5Widgets.dll")
    }
    $infoPath = Join-Path $OutputDir "release-info.json"
    $info | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $infoPath -Encoding UTF8

    Write-Host "Portable setup ready:" -ForegroundColor Green
    Write-Host "  Bundle: $SetupExe"
    Write-Host "  SHA256: $hash"
    Write-Host "  Info:   $infoPath"
} finally {
    if (Test-Path -LiteralPath $WorkDir) {
        Remove-Item -LiteralPath $WorkDir -Recurse -Force
    }
}
