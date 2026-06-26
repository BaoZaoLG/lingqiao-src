param(
    [string]$ScriptPath = (Join-Path $PSScriptRoot "..\AutoExam.js"),
    [string]$ThreadPath = (Join-Path $PSScriptRoot "..\src\hook\thread.cpp"),
    [string]$UiHeaderPath = (Join-Path $PSScriptRoot "..\src\ui\main_window.h"),
    [string]$UiActionsPath = (Join-Path $PSScriptRoot "..\src\ui\impl_actions.h")
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

Assert-Contains $uiHeader 'struct\s+AiProviderConfig' 'Qt UI should define an AI provider config table.'
Assert-Contains $uiHeader 'm_providerCombo' 'Qt UI should include a provider combo box.'
Assert-Contains $uiHeader 'apiKeyEnc/' 'Qt UI should save encrypted API keys per provider.'
Assert-Contains $uiHeader 'supportsVision' 'Qt UI should track whether a provider supports image inputs.'
Assert-Contains $uiHeader 'gpt-5\.5' 'OpenAI provider should use a current GPT-5.5 default model.'
Assert-Contains $uiHeader 'qwen3\.7-plus' 'Qwen provider should use a current multimodal qwen3.7 model.'
Assert-Contains $uiHeader 'deepseek-v4-flash' 'DeepSeek provider should use a current DeepSeek model id.'
Assert-Contains $uiHeader 'mimo-v2\.5' 'MiMo provider should use MiMo V2.5 for unified text/image requests.'
Assert-Contains $uiActions 'INJECTOR_AI_PROVIDER' 'Injection should export INJECTOR_AI_PROVIDER.'
Assert-Contains $uiActions 'INJECTOR_AI_VISION_MODEL' 'Injection should export INJECTOR_AI_VISION_MODEL.'
Assert-Contains $uiActions 'INJECTOR_AI_ADAPTER' 'Injection should export INJECTOR_AI_ADAPTER.'
Assert-Contains $uiActions 'INJECTOR_AI_SUPPORTS_VISION' 'Injection should export INJECTOR_AI_SUPPORTS_VISION.'

Assert-Contains $thread '__AUTOEXAM_AI_CONFIG' 'Hook should inject window.__AUTOEXAM_AI_CONFIG.'
Assert-Contains $thread 'INJECTOR_AI_BASE_URL' 'Hook should read INJECTOR_AI_BASE_URL.'
Assert-Contains $thread 'INJECTOR_AI_VISION_MODEL' 'Hook should read INJECTOR_AI_VISION_MODEL.'
Assert-Contains $thread 'INJECTOR_AI_SUPPORTS_VISION' 'Hook should read INJECTOR_AI_SUPPORTS_VISION.'
Assert-Contains $thread '__AUTOEXAM_INJECTED__' 'Hook should guard each frame against duplicate AutoExam injection.'

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
Assert-Contains $autoExam 'visionModel' 'AutoExam should use configured visionModel for image questions.'
Assert-Contains $autoExam 'textModel' 'AutoExam should use configured textModel for text-only questions.'
Assert-Contains $autoExam 'supportsVision' 'AutoExam should know if the provider supports image inputs.'
Assert-Contains $autoExam 'canUseVision' 'AutoExam should gate multimodal requests behind provider vision support.'
Assert-Contains $autoExam 'function\s+imageUrlToDataUrl\s*\(' 'AutoExam should convert remote image URLs into data URLs before sending vision requests.'
Assert-Contains $autoExam 'function\s+prepareVisionImages\s*\(' 'AutoExam should prepare vision images asynchronously before building the API request.'
Assert-Contains $autoExam 'FileReader' 'AutoExam should encode fetched image blobs as base64 data URLs.'
Assert-Contains $autoExam 'credentials:\s*''include''' 'AutoExam should fetch question images with page credentials when preparing vision requests.'
Assert-Contains $autoExam 'buildUserContent\(question\)\.then' 'AutoExam should wait for async vision image preparation before calling the model.'
Assert-Contains $autoExam 'lastVisionImageStats' 'AutoExam should expose image conversion diagnostics through debugInfo().'
Assert-Contains $autoExam 'provider' 'AutoExam cache key should include provider.'
Assert-Contains $autoExam 'toggleProvider' 'F4 should switch provider instead of only cycling text models.'

Write-Output "AutoExam provider/multimodal regression checks passed."
