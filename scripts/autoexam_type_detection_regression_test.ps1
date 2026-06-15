param(
    [string]$ScriptPath = (Join-Path $PSScriptRoot "..\AutoExam_silent.js")
)

$ErrorActionPreference = "Stop"
$text = Get-Content -LiteralPath $ScriptPath -Raw -Encoding UTF8

function Assert-Matches {
    param(
        [string]$Pattern,
        [string]$Message
    )
    if ($text -notmatch $Pattern) {
        Write-Host "FAIL: $Message"
        Write-Host "  Pattern: $Pattern"
        exit 1
    }
}

Assert-Matches "function\s+detectTypeFromDOM\s*\(" `
    "script should fall back to DOM-based type detection"
Assert-Matches "\u586B\u6846|\u586B\u5145|\u586B\u5165" `
    "blank-like renamed types such as 填框题 should be recognized"
Assert-Matches "\u7A0B\u5E8F|\u4EE3\u7801|\u7F16\u7A0B" `
    "programming-style renamed essay types should be recognized"
Assert-Matches "\u8FDE\u7EBF|\u6392\u5E8F|\u8BA1\u7B97|\u5206\u5F55" `
    "interactive and calculation-style types from real exams should be recognized"
Assert-Matches "\u542C\u529B|\u53E3\u8BED|\u6D4B\u8BC4|\u5199\u4F5C|\u5176\u5B83" `
    "language and miscellaneous real-exam types should be recognized"
Assert-Matches "typeMatch[\s\S]{0,260}\u586B\u6846" `
    "clipboard/manual mode should parse 填框题 prefixes"
Assert-Matches "typeMatch[\s\S]{0,420}\u7A0B\u5E8F" `
    "clipboard/manual mode should parse 程序题 prefixes"
Assert-Matches "typeMatch[\s\S]{0,520}\u8FDE\u7EBF" `
    "clipboard/manual mode should parse 连线题 prefixes"
Assert-Matches "typeMatch[\s\S]{0,650}\u5199\u4F5C" `
    "clipboard/manual mode should parse 写作题 prefixes"
Assert-Matches "question\.typeName\s*\|\|" `
    "essay prompts should preserve the actual source type name instead of always saying 简答题"

Write-Host "PASS: AutoExam type detection regression checks"
