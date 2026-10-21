param(
    [string]$ScriptPath = (Join-Path $PSScriptRoot "..\packages\autoexam\src\autoexam.js"),
    [string]$ThreadPath = (Join-Path $PSScriptRoot "..\client\src\hook\thread.cpp"),
    [string]$UiHeaderPath = (Join-Path $PSScriptRoot "..\client\src\ui\main_window.h"),
    [string]$UiActionsPath = (Join-Path $PSScriptRoot "..\client\src\ui\impl_actions.h"),
    [string]$EmbeddedHeaderPath = (Join-Path $PSScriptRoot "..\packages\autoexam\dist\autoexam.embedded.h")
)

$autoExam = Get-Content -Path $ScriptPath -Raw -Encoding UTF8
$thread = Get-Content -Path $ThreadPath -Raw -Encoding UTF8
$uiHeader = Get-Content -Path $UiHeaderPath -Raw -Encoding UTF8
$uiActions = Get-Content -Path $UiActionsPath -Raw -Encoding UTF8

function Assert-Contains {
    param(
        [string]$Content,
        [string]$Pattern,
        [string]$Message
    )
    if ($Content -notmatch $Pattern) {
        throw $Message
    }
}

function Assert-NotContains {
    param(
        [string]$Content,
        [string]$Pattern,
        [string]$Message
    )
    if ($Content -match $Pattern) {
        throw $Message
    }
}

Assert-NotContains $uiHeader 'AiProviderConfig|aiProviders|m_providerCombo' 'Qt UI must not contain provider presets.'
Assert-Contains $uiHeader 'm_baseUrlInput' 'Qt UI should include a BaseUrl input.'
Assert-Contains $uiHeader 'm_fetchModelsBtn' 'Qt UI should include a fetch-models button.'
Assert-Contains $uiActions 'm_modelNameInput->currentText' 'Injection should export the client model-name input.'
Assert-Contains $uiHeader 'apiKeyEnc' 'Qt UI should save the API key encrypted.'
Assert-Contains $uiHeader 'aiBaseUrl' 'Qt UI should persist BaseURL.'
Assert-Contains $uiHeader 'aiModel' 'Qt UI should persist model name.'
Assert-Contains $uiActions 'INJECTOR_AI_MODEL' 'Injection should export the client model-name input.'
Assert-Contains $uiActions 'm_baseUrlInput->text' 'Injection should export the client BaseUrl input.'
Assert-Contains $uiActions 'm_apiKeyInput->text' 'Injection should export the client API key input.'
Assert-Contains $uiActions 'INJECTOR_AI_API_KEY' 'Injection should export the unified AI API key.'
Assert-Contains $uiActions 'm_modelNameInput->currentText' 'Injection should use the selected model name.'

Assert-Contains $thread '__AUTOEXAM_AI_CONFIG' 'Hook should inject window.__AUTOEXAM_AI_CONFIG.'
Assert-Contains $thread "model: '" 'Hook should inject the model field at JS granularity.'
Assert-Contains $thread 'INJECTOR_AI_BASE_URL' 'Hook should read INJECTOR_AI_BASE_URL.'
Assert-Contains $thread 'INJECTOR_AI_API_KEY' 'Hook should read the unified AI API key.'
Assert-NotContains $thread 'INJECTOR_API_KEY' 'Hook must not use the legacy API key variable.'
Assert-NotContains $thread 'INJECTOR_AI_PROVIDER|INJECTOR_AI_TEXT_MODEL|INJECTOR_AI_VISION_MODEL|INJECTOR_AI_ADAPTER|INJECTOR_AI_SUPPORTS_VISION' 'Hook must only receive the unified AI config fields.'
Assert-Contains $thread '__AUTOEXAM_INJECTED__' 'Hook should guard each frame against duplicate AutoExam injection.'
Assert-Contains $thread 'FetchVersionedScript' 'Hook should fetch AutoExam script from the server.'
Assert-NotContains $thread 'autoexam\.embedded\.h' 'CefHook must not include an embedded AutoExam header.'
Assert-NotContains $thread 'g_enc_js_' 'CefHook must not keep encrypted JS fallback blobs.'
if (Test-Path $EmbeddedHeaderPath) {
    throw "AutoExam build output must not retain autoexam.embedded.h."
}

Assert-Contains $autoExam 'function\s+callModel\s*\(' 'AutoExam should use provider-aware callModel().'
Assert-Contains $autoExam 'function\s+findQuestionRoot\s*\(' 'AutoExam should find question roots beyond .questionLi.'
Assert-Contains $autoExam 'function\s+isExamPage\s*\(' 'AutoExam should centralize exam-page detection.'
Assert-Contains $autoExam 'function\s+extractImages\s*\(' 'AutoExam should extract images from question DOM.'
Assert-Contains $autoExam 'function\s+hasImages\s*\(' 'AutoExam should detect image questions.'
Assert-Contains $autoExam 'data-src' 'AutoExam should detect lazy-loaded data-src images.'
Assert-Contains $autoExam 'data-original' 'AutoExam should detect lazy-loaded data-original images.'
Assert-Contains $autoExam 'ans-ued-img' 'AutoExam should explicitly account for Chaoxing UEditor images.'
Assert-Contains $autoExam 'srcset' 'AutoExam should detect srcset images.'
Assert-Contains $autoExam 'background-image' 'AutoExam should detect CSS background images.'
Assert-Contains $autoExam 'debugInfo' 'AutoExam should expose debugInfo for field diagnosis.'
Assert-Contains $autoExam 'handleMiddleClick' 'AutoExam should expose reusable middle-click handling.'
Assert-Contains $autoExam 'relayMiddleClickToParent' 'AutoExam should relay middle-clicks from child frames to the parent exam frame.'
Assert-Contains $autoExam 'data:image\\/' 'AutoExam should allow data:image/* sources.'
Assert-Contains $autoExam 'blob:' 'AutoExam should explicitly skip blob: images.'
Assert-Contains $autoExam 'AI_CONFIG\.model' 'AutoExam should accept the client model field directly.'
Assert-Contains $autoExam 'apiKey: AI_CONFIG\.apiKey' 'AutoExam should accept the client API key field directly.'
Assert-NotContains $autoExam 'provider:\s*[''\"]|textModel\s*:|visionModel\s*:|supportsVision\s*:|anthropic-messages' 'AutoExam must not contain provider-specific configuration.'
Assert-Contains $autoExam 'function\s+imageUrlToDataUrl\s*\(' 'AutoExam should convert remote image URLs into data URLs before sending vision requests.'
Assert-Contains $autoExam 'function\s+prepareVisionImages\s*\(' 'AutoExam should prepare vision images asynchronously before building the API request.'
Assert-Contains $autoExam 'FileReader' 'AutoExam should encode fetched image blobs as base64 data URLs.'
Assert-Contains $autoExam 'credentials:\s*''include''' 'AutoExam should fetch question images with page credentials when preparing vision requests.'
Assert-Contains $autoExam 'buildUserContent\(question\)\.then' 'AutoExam should wait for async vision image preparation before calling the model.'
Assert-Contains $autoExam 'lastVisionImageStats' 'AutoExam should expose image conversion diagnostics through debugInfo().'
Assert-Contains $autoExam 'baseUrl' 'AutoExam cache/config should use the configured BaseURL.'

Write-Output "AutoExam provider/multimodal regression checks passed."
