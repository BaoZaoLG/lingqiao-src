$ErrorActionPreference = "Stop"

$scriptPath = Join-Path $PSScriptRoot "..\AutoExam_silent.js"
$content = Get-Content -LiteralPath $scriptPath -Raw

if ($content -notmatch "function toggleEnabled\(\)\s*\{(?<body>[\s\S]*?)\r?\n\s*\}\r?\n\r?\n\s*function toggleCursorFeedback\(\)") {
    throw "Could not locate toggleEnabled function body."
}

$toggleEnabledBody = $Matches["body"]

if ($toggleEnabledBody -match "resetAnswerStates\(\);") {
    throw "toggleEnabled must not reset answer/cursor state; Tab should preserve the current F3 cursor-feedback state."
}

if ($content -notmatch "case 'Tab':[\s\S]*window\.AutoExam\.toggle\(\)") {
    throw "Tab shortcut should still call AutoExam.toggle()."
}

if ($content -notmatch "case 'F3':\s*fn = 'toggleCursorFeedback'") {
    throw "F3 shortcut should still toggle cursor feedback."
}

Write-Output "AutoExam Tab/F3 cursor-state regression test passed."
