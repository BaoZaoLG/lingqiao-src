param(
    [string]$ScriptPath = (Join-Path $PSScriptRoot "..\AutoExam_silent.js")
)

$ErrorActionPreference = "Stop"
$text = Get-Content -LiteralPath $ScriptPath -Raw -Encoding UTF8

if ($text -notmatch "CURSOR_FEEDBACK_STORAGE_KEY") {
    throw "Cursor feedback preference should have a dedicated storage key."
}

if ($text -notmatch "function loadCursorFeedbackPreference\(\)") {
    throw "Cursor feedback preference should be loaded when the script starts."
}

if ($text -notmatch "function saveCursorFeedbackPreference\(") {
    throw "Cursor feedback preference should be saved when F3 changes it."
}

if ($text -notmatch "var cursorFeedback\s*=\s*loadCursorFeedbackPreference\(\);") {
    throw "cursorFeedback should initialize from persisted preference, not always default to true."
}

$toggleIndex = $text.IndexOf("function toggleCursorFeedback()")
$saveIndex = $text.IndexOf("saveCursorFeedbackPreference(cursorFeedback)", $toggleIndex)
$nextSectionIndex = $text.IndexOf("Middle-click & Clipboard Answering", $toggleIndex)

if ($toggleIndex -lt 0 -or $saveIndex -lt $toggleIndex -or $nextSectionIndex -lt $saveIndex) {
    throw "F3 toggle should persist the new cursor feedback preference."
}

Write-Output "AutoExam cursor feedback persistence regression test passed."
