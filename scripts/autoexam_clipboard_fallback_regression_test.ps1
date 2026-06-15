param(
    [string]$ScriptPath = (Join-Path $PSScriptRoot "..\AutoExam_silent.js")
)

$ErrorActionPreference = "Stop"
$text = Get-Content -LiteralPath $ScriptPath -Raw -Encoding UTF8

if ($text -notmatch "// ---- Ctrl\+C keyboard listener[\s\S]*?document\.addEventListener\('keydown', function\(e\) \{(?<body>[\s\S]*?)\n\s*\}\);") {
    throw "Could not locate Ctrl+C keyboard fallback listener."
}

$body = $Matches["body"]

if ($body -match "if \(selection && selection\.length >= 3\) return;") {
    throw "Ctrl+C fallback must not return early when text is selected; selected text is needed when the page blocks copy events."
}

if ($body -notmatch "resolveClipboardQuestion\(selection\)") {
    throw "Ctrl+C fallback should resolve selected text when the copy event does not fire."
}

if ($text -notmatch "function resolveClipboardQuestion\(selection\)\s*\{[\s\S]*?parseClipboardQuestion\(selection\)") {
    throw "resolveClipboardQuestion should parse selected text."
}

if ($body -notmatch "Date\.now\(\) - lastCopyTime < 500") {
    throw "Ctrl+C fallback should still avoid duplicate fetches when the normal copy event already ran."
}

Write-Output "AutoExam clipboard fallback regression test passed."
