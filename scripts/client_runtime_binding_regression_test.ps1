$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$actions = Get-Content -Raw (Join-Path $root 'client/src/ui/impl_actions.h')
$loader = Get-Content -Raw (Join-Path $root 'client/src/hook/script_loader.h')
$apiClient = Get-Content -Raw (Join-Path $root 'client/src/services/api_client.cpp')
$httpClient = Get-Content -Raw (Join-Path $root 'client/src/network/http_client.h')

function Assert-Contains {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Message
    )
    if ($Text -notmatch $Pattern) {
        throw $Message
    }
}

Assert-Contains $actions 'INJECTOR_MACHINE_ID' 'Injector must pass the activated machine id to the hook runtime.'
Assert-Contains $actions 'INJECTOR_CARD_CODE' 'Injector must pass the activated card code to the hook runtime.'
Assert-Contains $loader 'GetEnvironmentVariableW\(L"INJECTOR_MACHINE_ID"' 'Hook script fetch must read the machine id from the environment.'
Assert-Contains $loader 'GetEnvironmentVariableW\(L"INJECTOR_CARD_CODE"' 'Hook script fetch must read the card code from the environment.'
Assert-Contains $loader 'X-Machine-ID: %s' 'Hook script fetch must send X-Machine-ID.'
Assert-Contains $loader 'X-Card-Code: %s' 'Hook script fetch must send X-Card-Code.'
Assert-Contains $apiClient 'postJson\(g_pathAct, req\)' 'Activation must use the signed POST helper with the configured activate path.'
Assert-Contains $httpClient 'WinHttpOpenRequest\(hConnect\.get\(\), L"POST", path' 'Signed JSON requests must use POST.'
Assert-Contains $apiClient '请求方法不匹配' 'HTTP 405 must produce an actionable activation diagnostic.'

Write-Host 'client runtime binding regression checks passed'
