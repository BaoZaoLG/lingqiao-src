param(
    [Parameter(Mandatory=$true)] [string]$DllPath,
    [Parameter(Mandatory=$true)] [string]$OutHeader
)

$bytes = [System.IO.File]::ReadAllBytes($DllPath)
$sb = [System.Text.StringBuilder]::new()

[void]$sb.AppendLine('#pragma once')
[void]$sb.AppendLine('static unsigned char g_embeddedDll[] = {')

for ($i = 0; $i -lt $bytes.Length; $i++) {
    if ($i % 16 -eq 0) { [void]$sb.Append("`n    ") }
    [void]$sb.Append('0x' + $bytes[$i].ToString('X2') + ', ')
}

[void]$sb.AppendLine()
[void]$sb.AppendLine('};')
[void]$sb.AppendLine('static unsigned int g_embeddedDllSize = ' + $bytes.Length + ';')
[void]$sb.AppendLine('// Generated from: ' + (Split-Path $DllPath -Leaf))

[System.IO.File]::WriteAllText($OutHeader, $sb.ToString())
Write-Host "Generated $OutHeader ($($bytes.Length) bytes)"
