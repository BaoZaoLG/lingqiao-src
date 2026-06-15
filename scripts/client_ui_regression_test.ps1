param(
    [string]$SourceRoot = (Join-Path $PSScriptRoot "..\src")
)

$ErrorActionPreference = "Stop"

function Assert-NotContains {
    param(
        [string]$Path,
        [string]$Pattern,
        [string]$Message
    )

    $text = Get-Content -LiteralPath $Path -Raw
    if ($text -match $Pattern) {
        Write-Host "FAIL: $Message"
        Write-Host "  File: $Path"
        Write-Host "  Pattern: $Pattern"
        exit 1
    }
}

function Assert-Contains {
    param(
        [string]$Path,
        [string]$Pattern,
        [string]$Message
    )

    $text = Get-Content -LiteralPath $Path -Raw
    if ($text -notmatch $Pattern) {
        Write-Host "FAIL: $Message"
        Write-Host "  File: $Path"
        Write-Host "  Pattern: $Pattern"
        exit 1
    }
}

$mainWindow = Join-Path $SourceRoot "ui\main_window.h"
$builder = Join-Path $SourceRoot "ui\impl_builder.h"
$button = Join-Path $SourceRoot "ui\animated_button.h"
$entry = Join-Path $SourceRoot "injector_qt.cpp"

Assert-Contains $mainWindow "WA_TranslucentBackground,\s*true" `
    "main window should keep the frosted-glass translucent surface"
Assert-Contains $builder "ACCENT_POLICY\s+policy\s*=\s*\{\s*4\s*,\s*0\s*,\s*(static_cast<int>\()?0x[3-7][0-9A-Fa-f]FFFFFF" `
    "acrylic blur should use a light transparent tint, not an opaque white panel"
Assert-NotContains $builder "0x[89A-Fa-f][0-9A-Fa-f]FFFFFF" `
    "acrylic tint alpha must stay transparent enough to show glass"
Assert-NotContains $builder "ACCENT_POLICY\s+policy\s*=\s*\{\s*4\s*,\s*0\s*,\s*0x00000000" `
    "acrylic blur must not use a fully transparent tint"
Assert-NotContains $mainWindow "QColor\(255,\s*255,\s*255,\s*2[0-9]{2}\)" `
    "main window paint must not add a near-opaque white backing"
Assert-NotContains $mainWindow "QColor\(255,\s*255,\s*255,\s*1[5-9][0-9]\)" `
    "main window paint must stay visibly transparent"
Assert-NotContains $entry "AA_EnableHighDpiScaling" `
    "window dimensions must not be scaled differently by Qt high-DPI mode"
Assert-Contains $entry "AA_DisableHighDpiScaling" `
    "window dimensions should remain fixed native pixels across DPI settings"
Assert-NotContains $mainWindow "setFixedSize\s*\(\s*WINDOW_WIDTH\s*,\s*WINDOW_HEIGHT\s*\)" `
    "main window must be resizable instead of fixed-size"
Assert-Contains $mainWindow "setMinimumSize\s*\(" `
    "resizable main window should keep a safe minimum size"
Assert-Contains $mainWindow "HTTOPLEFT|HTBOTTOMRIGHT|HTLEFT|HTRIGHT|HTTOP|HTBOTTOM" `
    "frameless window must expose native resize hit-test zones"
Assert-Contains $mainWindow "resizeEvent\s*\(" `
    "resizable main window should react to size changes"
Assert-Contains $mainWindow "applyUiScale\s*\(" `
    "resizable main window should scale UI text when the window grows"
Assert-Contains $mainWindow "m_announceLabel->setStyleSheet[\s\S]{0,180}spx\(11\)" `
    "announcement text should scale with the resizable UI"
Assert-Contains $mainWindow "m_updateLabel->setStyleSheet[\s\S]{0,180}spx\(11\)" `
    "update banner text should scale with the resizable UI"
Assert-Contains $mainWindow "WINDOW_WIDTH\).*WINDOW_HEIGHT\)|WINDOW_WIDTH[\s\S]{0,160}WINDOW_HEIGHT" `
    "UI scale should be derived from the designed base window size"
Assert-NotContains $button "rgba\(40,60,100" `
    "primary buttons must not retain the old dark palette"
Assert-NotContains $mainWindow "primaryButtonStyle[\s\S]{0,700}rgba\(74,158,255" `
    "activate and inject shared button style should match the target browse card style, not blue primary fill"
Assert-NotContains $mainWindow "primaryButtonStyle[\s\S]{0,220}background:\s*rgba\(255,255,255" `
    "activate and inject buttons should be transparent by default like the browse button"
Assert-NotContains $mainWindow "m_activateBtn->setStyleSheet\([\s\S]{0,700}rgba\(74,158,255" `
    "activate button reset style should match the target browse card style"
Assert-NotContains $entry "if\s*\(\s*IsBeingDebugged\(\)\s*\)\s*ExitProcess\(0\)" `
    "startup anti-debug checks must not silently terminate before logging"
Assert-NotContains $mainWindow "if\s*\(\s*IsBeingDebugged\(\)\s*\)\s*ExitProcess\(0\)" `
    "runtime anti-debug checks must not silently terminate the UI"
Assert-NotContains $entry "if\s*\(\s*IsBeingDebugged\(\)\s*\)\s*ExitProcess\(0\)" `
    "startup anti-debug checks must not silently terminate before logging"
Assert-NotContains $entry "IsBeingDebugged\(\)[\s\S]{0,220}return\s+0\s*;" `
    "startup anti-debug checks must not prevent the client UI from opening"

Write-Host "PASS: client UI regression checks"
