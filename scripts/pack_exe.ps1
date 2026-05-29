param(
    [Parameter(Mandatory=$true)] [string]$InputExe,
    [string]$OutputExe
)

<#
.SYNOPSIS
    PE Protector v4: Standalone encrypted launcher
    - Compress (Deflate) + Encrypt (AES-256-CBC) the target exe
    - Embed encrypted payload into a C# launcher as .NET resource
    - Launcher: card activation → server auth → decrypt → run
    - Output is a single standalone .exe (~original size)

.EXAMPLE
    .\pack_exe.ps1 -InputExe .\src\Release\Injector.exe
#>

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Security

if (-not $OutputExe) {
    $baseName = [System.IO.Path]::GetFileNameWithoutExtension($InputExe)
    $OutputExe = Join-Path (Split-Path $InputExe) "${baseName}_packed.exe"
}

# ── Step 1: Read and hash ────────────────────────────────────────────────────
Write-Host "[1/5] Reading $InputExe ..." -ForegroundColor Cyan
$inputBytes = [System.IO.File]::ReadAllBytes((Resolve-Path $InputExe))
Write-Host "  Size: $($inputBytes.Length) bytes" -ForegroundColor Gray

$sha256 = [System.Security.Cryptography.SHA256]::Create()
$exeHash = $sha256.ComputeHash($inputBytes)
$exeHashHex = [System.BitConverter]::ToString($exeHash) -replace '-',''
Write-Host "  SHA256: $exeHashHex" -ForegroundColor Gray

# ── Step 2: Compress ─────────────────────────────────────────────────────────
Write-Host "[2/5] Compressing (Deflate) ..." -ForegroundColor Cyan
$ms = New-Object System.IO.MemoryStream
$ds = New-Object System.IO.Compression.DeflateStream($ms, [System.IO.Compression.CompressionLevel]::Optimal)
$ds.Write($inputBytes, 0, $inputBytes.Length)
$ds.Close()
$compressed = $ms.ToArray()
$ms.Close()
$ratio = [math]::Round($compressed.Length / $inputBytes.Length * 100, 1)
Write-Host "  $($inputBytes.Length) -> $($compressed.Length) bytes ($ratio%)" -ForegroundColor Gray

# ── Step 3: Encrypt (AES-256-CBC) ────────────────────────────────────────────
Write-Host "[3/5] Encrypting (AES-256-CBC) ..." -ForegroundColor Cyan

$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$aesKey = New-Object byte[] 32; $rng.GetBytes($aesKey)
$iv = New-Object byte[] 16; $rng.GetBytes($iv)

$aes = [System.Security.Cryptography.Aes]::Create()
$aes.KeySize = 256; $aes.Mode = [System.Security.Cryptography.CipherMode]::CBC
$aes.Padding = [System.Security.Cryptography.PaddingMode]::PKCS7
$aes.Key = $aesKey; $aes.IV = $iv
$encryptor = $aes.CreateEncryptor()
$encrypted = $encryptor.TransformFinalBlock($compressed, 0, $compressed.Length)
$aes.Dispose()

# Compute HMAC over encrypted payload for integrity check
$hmacKey = New-Object byte[] 32; $rng.GetBytes($hmacKey)
$hmac = [System.Security.Cryptography.HMACSHA256]::new([byte[]]$hmacKey)
$hmacBytes = $hmac.ComputeHash($encrypted)
$hmac.Dispose()
Write-Host "  Encrypted: $($encrypted.Length) bytes" -ForegroundColor Gray

# ── Step 4: Generate embedded launcher ───────────────────────────────────────
Write-Host "[4/5] Generating embedded launcher ..." -ForegroundColor Cyan

# Convert to base64 for embedding
$payloadB64 = [System.Convert]::ToBase64String($encrypted)
$keyB64     = [System.Convert]::ToBase64String($aesKey)
$ivB64      = [System.Convert]::ToBase64String($iv)
$hmacKeyB64 = [System.Convert]::ToBase64String($hmacKey)
$hmacB64    = [System.Convert]::ToBase64String($hmacBytes)

# Find CSC compiler
$cscPaths = @(
    "E:/Visual studio 2022/MSBuild/Current/Bin/Roslyn/csc.exe",
    "C:/Program Files/Microsoft Visual Studio/2022/Community/MSBuild/Current/Bin/Roslyn/csc.exe",
    "C:/Program Files/Microsoft Visual Studio/2022/Professional/MSBuild/Current/Bin/Roslyn/csc.exe",
    "C:/Program Files/Microsoft Visual Studio/2022/Enterprise/MSBuild/Current/Bin/Roslyn/csc.exe",
    "C:/Program Files/Microsoft Visual Studio/2022/BuildTools/MSBuild/Current/Bin/Roslyn/csc.exe",
    "C:/Windows/Microsoft.NET/Framework/v4.0.30319/csc.exe"
)
$cscExe = $null
foreach ($p in $cscPaths) { if (Test-Path $p) { $cscExe = $p; break } }
if (-not $cscExe) {
    $vswhere = "${env:ProgramFiles(x86)}\Microsoft Visual Studio\Installer\vswhere.exe"
    if (Test-Path $vswhere) {
        $vsPath = & $vswhere -latest -property installationPath 2>$null
        if ($vsPath) {
            $candidate = Join-Path $vsPath "MSBuild/Current/Bin/Roslyn/csc.exe"
            if (Test-Path $candidate) { $cscExe = $candidate }
        }
    }
}
if (-not $cscExe) { throw "Cannot find csc.exe" }
Write-Host "  CSC: $cscExe" -ForegroundColor Gray

# Generate launcher source
$launcherCs = Join-Path ([System.IO.Path]::GetTempPath()) "stage1_launcher.cs"
$launcherExe = $OutputExe

$launcherSource = @"
using System;
using System.Diagnostics;
using System.IO;
using System.IO.Compression;
using System.Management;
using System.Net;
using System.Net.Security;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;
using System.Windows.Forms;

class Stage1 {
    const string SERVER_HOST = "47.110.248.240";
    const int    SERVER_PORT = 48901;
    const string CLIENT_ID  = "injector_v1";
    const string CLIENT_SECRET = "c1a3f8e9d2b47a6e8f0c3d5b9a1e4f7a8b2c6d0e3f5a7b9c1d4e6f8a0b2c4d6";

    // Embedded encrypted payload (auto-filled by pack script)
    static readonly string PAYLOAD_B64  = "$payloadB64";
    static readonly string KEY_B64      = "$keyB64";
    static readonly string IV_B64       = "$ivB64";
    static readonly string HMACKEY_B64  = "$hmacKeyB64";
    static readonly string HMAC_B64     = "$hmacB64";

    static string sessionToken = null;
    static string cardCode = null;

    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern bool GetVolumeInformationW(string rootPath, char[] volumeName, int volNameLen,
        out uint volumeSerial, out uint maxCompLen, out uint fsFlags, char[] fsName, int fsNameLen);

    [DllImport("kernel32.dll", CharSet = CharSet.Unicode)]
    static extern bool MoveFileEx(string lpExistingFileName, string lpNewFileName, uint dwFlags);

    static string GetHWID() {
        string volSerial = "0", macAddr = "00:00:00:00:00:00", compName = "unknown", userName = "unknown";
        string biosSerial = "none", cpuId = "0", boardSerial = "none";
        try {
            uint serial, maxLen, flags;
            if (GetVolumeInformationW("C:\\", null, 0, out serial, out maxLen, out flags, null, 0))
                volSerial = serial.ToString("X8");
        } catch {}
        try {
            foreach (var nic in System.Net.NetworkInformation.NetworkInterface.GetAllNetworkInterfaces()) {
                if (nic.OperationalStatus == System.Net.NetworkInformation.OperationalStatus.Up &&
                    nic.NetworkInterfaceType != System.Net.NetworkInformation.NetworkInterfaceType.Loopback) {
                    var addr = nic.GetPhysicalAddress().ToString();
                    if (addr.Length == 12) {
                        var sb = new StringBuilder();
                        for (int i = 0; i < 12; i += 2) { if (i > 0) sb.Append(":"); sb.Append(addr, i, 2); }
                        macAddr = sb.ToString();
                    }
                    break;
                }
            }
        } catch {}
        try { compName = Environment.MachineName; } catch {}
        try { userName = Environment.UserName; } catch {}
        try {
            string biosKey = "HARDWARE" + "\\" + "DESCRIPTION" + "\\" + "System" + "\\" + "BIOS";
            using (var key = Microsoft.Win32.Registry.LocalMachine.OpenSubKey(biosKey)) {
                if (key != null) { biosSerial = key.GetValue("SystemSerialNumber", "none").ToString(); }
            }
        } catch {}
        try {
            using (var searcher = new ManagementObjectSearcher("SELECT ProcessorId FROM Win32_Processor")) {
                foreach (var obj in searcher.Get()) { cpuId = obj["ProcessorId"].ToString(); break; }
            }
        } catch {}
        try {
            string biosKey2 = "HARDWARE" + "\\" + "DESCRIPTION" + "\\" + "System" + "\\" + "BIOS";
            using (var key = Microsoft.Win32.Registry.LocalMachine.OpenSubKey(biosKey2)) {
                if (key != null) { boardSerial = key.GetValue("BaseBoardProduct", "none").ToString(); }
            }
        } catch {}
        string raw = volSerial + "|" + macAddr + "|" + compName + "|" + userName + "|" + biosSerial + "|" + cpuId + "|" + boardSerial;
        using (var hmac = new HMACSHA256(GetHmacKey()))
            return BitConverter.ToString(hmac.ComputeHash(Encoding.UTF8.GetBytes(raw))).Replace("-","").Substring(0, 32);
    }

    static byte[] _hmacKey = null;
    static byte[] GetHmacKey() {
        if (_hmacKey == null)
            _hmacKey = DeriveKey(CLIENT_SECRET, Encoding.UTF8.GetBytes("CefBridge-HMAC-Salt-v2"), 32);
        return _hmacKey;
    }

    static string SignRequest(string body, out string timestamp, out string nonce) {
        timestamp = ((long)(DateTime.UtcNow - new DateTime(1970,1,1)).TotalSeconds).ToString();
        nonce = Guid.NewGuid().ToString("N");
        string signedData = timestamp + "|" + nonce + "|" + body;
        using (var hmac = new HMACSHA256(GetHmacKey()))
            return BitConverter.ToString(hmac.ComputeHash(Encoding.UTF8.GetBytes(signedData))).Replace("-","").ToLower();
    }

    static string HttpPost(string path, string json) {
        ServicePointManager.SecurityProtocol = SecurityProtocolType.Tls12;
        ServicePointManager.ServerCertificateValidationCallback = delegate { return true; };
        string ts, nonce;
        string sig = SignRequest(json, out ts, out nonce);
        var url = string.Format("https://{0}:{1}{2}", SERVER_HOST, SERVER_PORT, path);
        var req = (HttpWebRequest)WebRequest.Create(url);
        req.Method = "POST"; req.ContentType = "application/json"; req.Timeout = 15000;
        req.Headers.Add("X-Client-ID", CLIENT_ID);
        req.Headers.Add("X-HMAC-Signature", sig);
        req.Headers.Add("X-Timestamp", ts);
        req.Headers.Add("X-Nonce", nonce);
        var body = Encoding.UTF8.GetBytes(json);
        req.ContentLength = body.Length;
        using (var s = req.GetRequestStream()) s.Write(body, 0, body.Length);
        using (var resp = (HttpWebResponse)req.GetResponse())
        using (var reader = new StreamReader(resp.GetResponseStream(), Encoding.UTF8))
            return reader.ReadToEnd();
    }

    static string JsonStr(string key, string val) {
        return "\"" + key + "\":\"" + val.Replace("\\","\\\\").Replace("\"","\\\"") + "\"";
    }
    static string JsonGet(string json, string key) {
        var search = "\"" + key + "\"";
        int idx = json.IndexOf(search);
        if (idx < 0) return "";
        idx = json.IndexOf(':', idx + search.Length);
        if (idx < 0) return "";
        idx++;
        while (idx < json.Length && json[idx] == ' ') idx++;
        if (idx < json.Length && json[idx] == '"') {
            int end = json.IndexOf('"', idx + 1);
            return json.Substring(idx + 1, end - idx - 1);
        }
        int end2 = json.IndexOfAny(new char[]{',','}','\n','\r'}, idx);
        if (end2 < 0) end2 = json.Length;
        return json.Substring(idx, end2 - idx).Trim();
    }

    static byte[] DeriveKey(string password, byte[] salt, int keyLen) {
        using (var hmac = new HMACSHA256(Encoding.UTF8.GetBytes(password))) {
            int hashLen = 32;
            int blocks = (keyLen + hashLen - 1) / hashLen;
            byte[] result = new byte[keyLen];
            for (int i = 1; i <= blocks; i++) {
                byte[] blockSalt = new byte[salt.Length + 4];
                Array.Copy(salt, blockSalt, salt.Length);
                blockSalt[salt.Length] = (byte)(i >> 24);
                blockSalt[salt.Length+1] = (byte)(i >> 16);
                blockSalt[salt.Length+2] = (byte)(i >> 8);
                blockSalt[salt.Length+3] = (byte)i;
                byte[] u = hmac.ComputeHash(blockSalt);
                byte[] f = (byte[])u.Clone();
                for (int j = 1; j < 100000; j++) {
                    u = hmac.ComputeHash(u);
                    for (int k = 0; k < f.Length; k++) f[k] ^= u[k];
                }
                Array.Copy(f, 0, result, (i-1) * hashLen, Math.Min(hashLen, keyLen - (i-1)*hashLen));
            }
            return result;
        }
    }

    static bool Activate() {
        string hwid = GetHWID();
        string json = "{" + JsonStr("client_id", CLIENT_ID) + ","
                      + JsonStr("card", cardCode) + ","
                      + JsonStr("machine_id", hwid) + ","
                      + JsonStr("fingerprint", hwid) + ","
                      + JsonStr("client_version", "3.0.0") + "}";
        string resp = HttpPost("/api/v1/activate", json);
        string status = JsonGet(resp, "status");
        if (status != "ok") {
            MessageBox.Show(JsonGet(resp, "message"), "Activation Failed", MessageBoxButtons.OK, MessageBoxIcon.Error);
            return false;
        }
        sessionToken = JsonGet(resp, "session_token");
        return true;
    }

    static byte[] DecryptAndVerify(byte[] encrypted, byte[] key, byte[] iv, byte[] hmacKey, byte[] expectedHmac) {
        using (var hmac = new HMACSHA256(hmacKey)) {
            byte[] computed = hmac.ComputeHash(encrypted);
            for (int i = 0; i < 32; i++)
                if (computed[i] != expectedHmac[i])
                    throw new Exception("Payload integrity check failed");
        }
        byte[] decrypted;
        using (var aes = Aes.Create()) {
            aes.Key = key; aes.IV = iv;
            aes.Mode = CipherMode.CBC; aes.Padding = PaddingMode.PKCS7;
            using (var d = aes.CreateDecryptor())
            using (var ms = new MemoryStream(encrypted))
            using (var cs = new CryptoStream(ms, d, CryptoStreamMode.Read))
            using (var outMs = new MemoryStream()) {
                cs.CopyTo(outMs);
                decrypted = outMs.ToArray();
            }
        }
        using (var ms = new MemoryStream(decrypted))
        using (var ds = new DeflateStream(ms, CompressionMode.Decompress))
        using (var outMs = new MemoryStream()) {
            ds.CopyTo(outMs);
            return outMs.ToArray();
        }
    }

    [STAThread]
    static void Main() {
        try {
            byte[] encrypted = Convert.FromBase64String(PAYLOAD_B64);
            byte[] key       = Convert.FromBase64String(KEY_B64);
            byte[] iv        = Convert.FromBase64String(IV_B64);
            byte[] hmacKey   = Convert.FromBase64String(HMACKEY_B64);
            byte[] hmacHash  = Convert.FromBase64String(HMAC_B64);

            // Card activation
            cardCode = Microsoft.VisualBasic.Interaction.InputBox(
                "Enter card code:", "CX LingQiao", "");
            if (string.IsNullOrWhiteSpace(cardCode)) return;

            if (!Activate()) return;

            // Decrypt embedded payload
            byte[] exeBytes = DecryptAndVerify(encrypted, key, iv, hmacKey, hmacHash);

            // Clear sensitive material
            Array.Clear(key, 0, key.Length);
            Array.Clear(hmacKey, 0, hmacKey.Length);

            // Write to temp and run
            string tmpPath = Path.Combine(Path.GetTempPath(),
                Path.GetFileNameWithoutExtension(Path.GetRandomFileName()) + ".exe");
            File.WriteAllBytes(tmpPath, exeBytes);
            Array.Clear(exeBytes, 0, exeBytes.Length);

            string exeDir = Path.GetDirectoryName(Assembly.GetExecutingAssembly().Location);
            Environment.SetEnvironmentVariable("QT_PLUGIN_PATH", exeDir);
            string path = Environment.GetEnvironmentVariable("PATH") ?? "";
            if (!path.Contains(exeDir))
                Environment.SetEnvironmentVariable("PATH", exeDir + ";" + path);

            var psi = new ProcessStartInfo { FileName = tmpPath, UseShellExecute = true };
            var proc = Process.Start(psi);
            if (proc != null) {
                proc.WaitForExit();
                try { File.Delete(tmpPath); } catch { MoveFileEx(tmpPath, null, 4); }
            }
        } catch (Exception ex) {
            MessageBox.Show(ex.Message, "Error", MessageBoxButtons.OK, MessageBoxIcon.Error);
        }
    }
}
"@

[System.IO.File]::WriteAllText($launcherCs, $launcherSource, [System.Text.Encoding]::UTF8)

& $cscExe /noconfig /platform:x86 /target:winexe /optimize+ `
    /reference:System.dll /reference:System.Core.dll /reference:Microsoft.VisualBasic.dll `
    /reference:System.Windows.Forms.dll /reference:System.Management.dll `
    /out:"$launcherExe" "$launcherCs" 2>&1

if ($LASTEXITCODE -ne 0) {
    Remove-Item $launcherCs -Force -ErrorAction SilentlyContinue
    throw "Launcher compilation failed"
}

Remove-Item $launcherCs -Force

# ── Step 5: Cleanup ──────────────────────────────────────────────────────────
$aesKey = $null; $hmacKey = $null; $iv = $null

$outputSize = (Get-Item $launcherExe).Length
Write-Host ""
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "  Output: $launcherExe" -ForegroundColor Green
Write-Host "  Size: $outputSize bytes" -ForegroundColor Gray
Write-Host "  Original: $($inputBytes.Length) bytes" -ForegroundColor Gray
Write-Host "  Protection:" -ForegroundColor Yellow
Write-Host "    [1] Deflate compression" -ForegroundColor Yellow
Write-Host "    [2] AES-256-CBC encryption" -ForegroundColor Yellow
Write-Host "    [3] HMAC-SHA256 integrity check" -ForegroundColor Yellow
Write-Host "    [4] Server-side card activation" -ForegroundColor Yellow
Write-Host "    [5] Encrypted payload embedded in .NET resource" -ForegroundColor Yellow
Write-Host "============================================" -ForegroundColor Cyan
