param(
    [string]$ScriptPath = (Join-Path $PSScriptRoot "..\AutoExam_silent.js")
)

$ErrorActionPreference = "Stop"
$text = Get-Content -LiteralPath $ScriptPath -Raw -Encoding UTF8

if ($text -notmatch "function toggleCursorFeedback\(\)\s*\{(?<body>[\s\S]*?)\r?\n\s*\}\r?\n\r?\n\s*/\*") {
    throw "Could not locate toggleCursorFeedback function body."
}

$body = $Matches["body"]

if ($body -notmatch "middleBtnState\s*===\s*'loading'|clipboardState\s*===\s*'loading'") {
    throw "Enabling F3 cursor feedback while an answer request is loading should immediately restore the waiting cursor."
}

if ($body -notmatch "clearCursorLoading\(\)") {
    throw "Disabling F3 cursor feedback should clear the waiting cursor."
}

Write-Output "AutoExam cursor feedback loading-state regression test passed."
