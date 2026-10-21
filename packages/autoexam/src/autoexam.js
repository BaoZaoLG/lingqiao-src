/**
 * AutoExam.js — Lightweight exam auto-answer (middle-click only)
 * Injected by CefHook.dll on every page load.
 *
 * Controls:
 *   Tab          — Toggle answering on/off
 *   F3           — Toggle cursor feedback (loading spinner)
 *   Middle-click — First click fetches answer; second click fills it
 *   Escape       — Cancel pending middle-click fetch
 *
 * AI config comes from window.__AUTOEXAM_AI_CONFIG
 *   { baseUrl, model, apiKey } (set by the injector).
 */
(function() {
    'use strict';

    /* Do NOT silence console.info — LingQiaoBridge console transport uses it. */
    console.log = console.debug = function(){};

    /* ================================================================
     * Bridge
     * ================================================================ */
    var _bridgeReady = Boolean(window.LingQiaoBridge && window.LingQiaoBridge.ready);
    var _bridgeInfo = {
        ready: _bridgeReady,
        version: (window.LingQiaoBridge && window.LingQiaoBridge.version) || '',
        transport: (window.LingQiaoBridge && window.LingQiaoBridge.transport) || 'none'
    };

    function bridgeLog(level, message) {
        var br = window.LingQiaoBridge;
        if (!br || !br.ready) return;
        try {
            br.request('diagnostics.log', {
                level: level,
                scope: 'autoexam',
                message: message
            }).catch(function(){});
        } catch (e) {}
    }

    function bridgeReportQuestionState(qId, type, state) {
        var br = window.LingQiaoBridge;
        if (!br || !br.ready) return;
        try {
            br.request('page.reportQuestionState', {
                questionId: qId,
                type: type,
                state: state
            }).catch(function(){});
        } catch (e) {}
    }

    /* ================================================================
     * Config
     * ================================================================ */
    var AI_CONFIG = window.__AUTOEXAM_AI_CONFIG || {
        baseUrl: '',
        model: '',
        apiKey: ''
    };

    var config = {
        baseUrl: (AI_CONFIG.baseUrl || '').replace(/\/+$/, ''),
        model: AI_CONFIG.model || '',
        apiKey: AI_CONFIG.apiKey || ''
    };

    function getApiKey() {
        return config.apiKey || '';
    }

    function apiChatUrl() {
        var base = (config.baseUrl || '').replace(/\/+$/, '');
        if (!base) return '';
        if (!/\/chat\/completions$/i.test(base)) base += '/chat/completions';
        return base;
    }

    var lastVisionImageStats = null;
    window.debugInfo = function() {
        var qCount = 0;
        var stemCount = 0;
        var examDom = false;
        try {
            qCount = document.querySelectorAll('.questionLi').length;
            stemCount = document.querySelectorAll('.stem_answer').length;
            examDom = hasExamDom();
        } catch (e) {}
        return {
            config: config,
            lastVisionImageStats: lastVisionImageStats,
            bridge: _bridgeInfo,
            href: (window.location && window.location.href) || '',
            examDom: examDom,
            questionCount: qCount,
            stemAnswerCount: stemCount,
            autoExamReady: !!(window.AutoExam && typeof window.AutoExam.trigger === 'function'),
            middleButtonState: typeof middleBtnState === 'string' ? middleBtnState : 'uninitialized'
        };
    };

    var ENABLED_STORAGE_KEY = '__autoExamEnabled';
    var CURSOR_FEEDBACK_STORAGE_KEY = '__autoExamCursorFeedback';

    function loadEnabledPreference() {
        try {
            var saved = sessionStorage.getItem(ENABLED_STORAGE_KEY);
            if (saved === '0') return false;
            if (saved === '1') return true;
        } catch (e) {}
        return true;
    }

    function saveEnabledPreference(value) {
        try {
            sessionStorage.setItem(ENABLED_STORAGE_KEY, value ? '1' : '0');
        } catch (e) {}
    }

    function loadCursorFeedbackPreference() {
        try {
            var saved = sessionStorage.getItem(CURSOR_FEEDBACK_STORAGE_KEY);
            if (saved === '0') return false;
            if (saved === '1') return true;
        } catch (e) {}
        return true;
    }

    function saveCursorFeedbackPreference(value) {
        try {
            sessionStorage.setItem(CURSOR_FEEDBACK_STORAGE_KEY, value ? '1' : '0');
        } catch (e) {}
    }

    var enabled = loadEnabledPreference();
    var cursorFeedback = loadCursorFeedbackPreference();
    var cursorStyleEl = null;

    function ensureCursorStyle() {
        if (cursorStyleEl && cursorStyleEl.isConnected) return cursorStyleEl;
        cursorStyleEl = document.createElement('style');
        cursorStyleEl.id = '__cefCursorFeedback';
        (document.head || document.documentElement).appendChild(cursorStyleEl);
        return cursorStyleEl;
    }

    function setCursorLoading() {
        if (!cursorFeedback) return;
        try {
            ensureCursorStyle().textContent = 'body,body *{cursor:wait!important}';
        } catch (e) {}
    }

    function clearCursorLoading() {
        try {
            if (cursorStyleEl) cursorStyleEl.textContent = '';
        } catch (e) {}
    }

    function toggleCursorFeedback() {
        cursorFeedback = !cursorFeedback;
        saveCursorFeedbackPreference(cursorFeedback);
        if (!cursorFeedback) {
            clearCursorLoading();
        } else if (middleBtnState === 'loading') {
            setCursorLoading();
        }
        console.log('[AutoExam] Cursor feedback: ' + (cursorFeedback ? 'ON' : 'OFF'));
    }

    function canCallAi() {
        return Boolean(window.LingQiaoBridge && window.LingQiaoBridge.ready);
    }

    /* DLL capability handshake.
     * A CefHook.dll that predates the AI mailbox registers ai.chatComplete but
     * NOT ai.pollResult. Without this check the page sends the request, then
     * polls a method that does not exist, and sits in the wait state until the
     * watchdog silently gives up. Probe once and cache the verdict. */
    var _aiCapState = 'unknown'; /* unknown | ok | missing */
    function probeAiCapability() {
        var br = window.LingQiaoBridge;
        if (!br || !br.ready) return;
        try {
            br.request('runtime.getCapabilities', {}, { timeout: 4000 }).then(function(res) {
                var methods = (res && res.methods) || [];
                var hasChat = methods.indexOf('ai.chatComplete') >= 0;
                var hasPoll = methods.indexOf('ai.pollResult') >= 0;
                _aiCapState = (hasChat && hasPoll) ? 'ok' : 'missing';
                if (_aiCapState === 'missing') {
                    console.error('[AutoExam] 当前 CefHook.dll 缺少 AI 能力 ' +
                        '(ai.chatComplete=' + hasChat + ', ai.pollResult=' + hasPoll +
                        ')，AI 答题不可用，请更新 DLL。methods=' + JSON.stringify(methods));
                    bridgeLog('error', 'DLL AI capability missing chat=' + hasChat +
                        ' poll=' + hasPoll + ' methods=' + methods.join(','));
                    /* Capture the build identity so the report says which DLL it was. */
                    br.request('runtime.getStatus', {}, { timeout: 4000 }).then(function(st) {
                        if (!st) return;
                        var line = 'DLL build_stamp=' + st.build_stamp +
                                   ' arch=' + st.build_arch +
                                   ' methods=' + st.method_count +
                                   ' ai_ready=' + st.ai_ready;
                        console.error('[AutoExam] ' + line);
                        bridgeLog('error', line);
                    }).catch(function() {});
                } else {
                    bridgeLog('info', 'DLL AI capability ok, methodCount=' + methods.length);
                }
            }).catch(function(e) {
                bridgeLog('warn', 'capability probe failed ' + ((e && e.code) || 'unknown'));
            });
        } catch (e) {}
    }

    /* ================================================================
     * Answer cache
     * ================================================================ */
    var answerCache = {};
    var CACHE_MAX = 200;

    function hashQuestion(text) {
        var hash = 0;
        for (var i = 0; i < text.length; i++) {
            hash = ((hash << 5) - hash) + text.charCodeAt(i);
            hash = hash & hash;
        }
        return 'q_' + Math.abs(hash).toString(36);
    }

    function questionCacheKey(question) {
        var optionText = (question.options || []).map(function(o) {
            return String(o.label || '') + ':' + String(o.text || '');
        }).join('|');
        return [
            question.text || '',
            'type=' + question.type,
            'mode=' + (question.answerMode || ''),
            'lang=' + (question.language || ''),
            'blank=' + (question.blankCount || 0),
            'options=' + optionText,
            'base=' + config.baseUrl,
            'model=' + config.model
        ].join('\n');
    }

    function getCachedAnswer(question) {
        return answerCache[hashQuestion(questionCacheKey(question))] || null;
    }

    function cacheAnswer(question, answer) {
        var key = hashQuestion(questionCacheKey(question));
        answerCache[key] = answer;
        var keys = Object.keys(answerCache);
        if (keys.length > CACHE_MAX) {
            delete answerCache[keys[0]];
        }
    }

    /* ================================================================
     * Question parsing
     * ================================================================ */
    function hasMultiChoiceMarker(stemAnswer) {
        if (!stemAnswer) return false;
        return !!stemAnswer.querySelector(
            '.answerBg.multi, .answerBg.multipleoption, .answerBg[class*="multi"], ' +
            '.answerBg[role="checkbox"], .answerBg[onclick*="Multi"], .saveMultiSelect, input[type="checkbox"]'
        );
    }

    function detectTypeFromDOM(qDiv) {
        if (!qDiv) return 4;

        var stemAnswer = qDiv.querySelector('.stem_answer');
        if (stemAnswer) {
            var optionSpans = stemAnswer.querySelectorAll('.answerBg > span[data]');
            if (optionSpans.length > 0) {
                for (var i = 0; i < optionSpans.length; i++) {
                    var d = optionSpans[i].getAttribute('data');
                    if (d === 'true' || d === 'false') return 3;
                }
                if (hasMultiChoiceMarker(stemAnswer)) return 1;
                return 0;
            }
        }

        var blankTextareas = qDiv.querySelectorAll('textarea[name^="answerEditor"]');
        if (blankTextareas.length > 1) return 2;
        if (qDiv.querySelector('textarea[name^="answer"]')) return 4;
        return 4;
    }

    function getCodeEditorsMap() {
        if (typeof window !== 'undefined' && window.codeEditors) return window.codeEditors;
        if (typeof codeEditors !== 'undefined') return codeEditors;
        return null;
    }

    function hasCodeAnswerControl(qDiv) {
        if (!qDiv) return false;
        var qId = qDiv.getAttribute('data') || '';
        if (qId) {
            var editors = getCodeEditorsMap();
            if (editors && editors[qId]) return true;
        }
        return !!qDiv.querySelector(
            '.codeEditorBoxDiv[data-business-id], textarea.code-editor[data-business-id], ' +
            '[id^="procedural-"] .CodeMirror, .CodeMirror'
        );
    }

    function findCodeEditor(qId) {
        var editors = getCodeEditorsMap();
        if (editors && editors[qId]) return editors[qId];

        var qDiv = findQuestionDiv(qId);
        if (!qDiv) return null;

        var businessEl = qDiv.querySelector('.codeEditorBoxDiv[data-business-id], textarea.code-editor[data-business-id]');
        var businessId = businessEl ? businessEl.getAttribute('data-business-id') : '';
        if (businessId && editors && editors[businessId]) return editors[businessId];

        var cmEl = qDiv.querySelector('.CodeMirror');
        if (cmEl && cmEl.CodeMirror) return cmEl.CodeMirror;
        return null;
    }

    function getQuestionLanguage(qDiv, qId) {
        if (!qDiv) return '';

        var langBox = qDiv.querySelector('.langList[data-business-id="' + qId + '"][codename], .langList[codename]');
        if (langBox && langBox.getAttribute('codename')) {
            return langBox.getAttribute('codename');
        }

        var selectInput = qDiv.querySelector('input[name="languageSelect' + qId + '"], input#languageSelect' + qId);
        if (selectInput && selectInput.value) {
            var selectedByValue = qDiv.querySelector(
                '.langList[data-business-id="' + qId + '"] [codenum="' + selectInput.value + '"], ' +
                '.langList [codenum="' + selectInput.value + '"]'
            );
            if (selectedByValue) {
                return selectedByValue.getAttribute('codename') ||
                       selectedByValue.textContent.replace(/\s+/g, ' ').trim();
            }
        }

        var selected = qDiv.querySelector('.langList[data-business-id="' + qId + '"] [codename].active, .langList[data-business-id="' + qId + '"] [codename].on, .langList[data-business-id="' + qId + '"] [codename].selected')
                    || qDiv.querySelector('.langList [codename].active, .langList [codename].on, .langList [codename].selected')
                    || qDiv.querySelector('.langList[data-business-id="' + qId + '"] [codename], .langList [codename]');
        if (selected) {
            return selected.getAttribute('codename') || selected.textContent.replace(/\s+/g, ' ').trim();
        }

        var langMap = {
            '1': 'C', '2': 'C#', '3': 'Java', '4': 'VB.NET', '5': 'Python 2.x',
            '6': 'Go', '8': 'PHP', '11': 'Bash', '16': 'Python 3.6.9', '17': 'C++',
            '19': 'JavaScript', '20': 'R', '22': 'Python 3.12.12'
        };
        return selectInput ? (langMap[selectInput.value] || selectInput.value || '') : '';
    }

    function getQuestionScopedInput(qDiv, selector) {
        return qDiv ? qDiv.querySelector(selector) : null;
    }

    function parseQuestionFromDiv(qDiv) {
        if (!qDiv) return null;

        var qId = qDiv.getAttribute('data') || '';
        if (!qId) {
            var qIdInput = getQuestionScopedInput(qDiv, 'input#questionId, input[name="questionId"]');
            if (qIdInput) qId = qIdInput.value;
        }
        if (!qId) {
            var tnById = qDiv.querySelector('input[id*="typeName"]');
            if (tnById) {
                var m = tnById.id.match(/typeName(\d+)/);
                if (m) qId = m[1];
            }
        }
        if (!qId && qDiv.id) qId = qDiv.id;
        if (!qId) {
            qId = '__autoexam_q_' + Date.now() + Math.floor(Math.random() * 10000);
            if (!qDiv.id) qDiv.id = qId;
        } else if (!qDiv.id) {
            qDiv.id = qId;
        }

        var typeNameInput = getQuestionScopedInput(qDiv, 'input[name="typeName' + qId + '"]')
                         || getQuestionScopedInput(qDiv, 'input[id*="typeName' + qId + '"]');
        var typeNumInput  = getQuestionScopedInput(qDiv, 'input[name="type' + qId + '"]')
                         || getQuestionScopedInput(qDiv, 'input[id*="type' + qId + '"]');
        var typeName = typeNameInput ? typeNameInput.value : '';
        var typeNum  = typeNumInput  ? parseInt(typeNumInput.value, 10) : -1;
        if (isNaN(typeNum)) typeNum = -1;

        if (typeNum === -1 && typeName) {
            var TYPE_KEYWORDS = [
                { pattern: /单选|单项选择|单选选择/, type: 0 },
                { pattern: /多选|多项选择|复选|多选选择|共用选项|选做/, type: 1 },
                { pattern: /填空|填框|填充|填入|选填|选词填空|完型填空|完形填空|补全/, type: 2 },
                { pattern: /判断|对错|是非|正误/, type: 3 },
                { pattern: /计算/, type: 7 },
                { pattern: /简答|论述|分析|问答|名词解释|阅读理解|案例分析|综合|输入|主观|程序|代码|编程|分录|听力|口语|测评|写作|其它|其他|连线|排序/, type: 4 }
            ];
            var matched = false;
            for (var k = 0; k < TYPE_KEYWORDS.length; k++) {
                if (TYPE_KEYWORDS[k].pattern.test(typeName)) {
                    typeNum = TYPE_KEYWORDS[k].type;
                    matched = true;
                    break;
                }
            }
            if (!matched) typeNum = detectTypeFromDOM(qDiv);
        }
        if (typeNum === -1) typeNum = detectTypeFromDOM(qDiv);

        var answerControl = detectAnswerControl(qDiv);
        if (answerControl.mode === 'code') {
            typeNum = 4;
        } else if (answerControl.mode === 'text-multi') {
            if (typeNum !== 7) typeNum = answerControl.type;
        } else if (answerControl.mode === 'text-single' && (typeNum === -1 || (typeNum > 4 && typeNum !== 7))) {
            typeNum = answerControl.type;
        }

        var questionText = '';
        var markName = qDiv.querySelector('.mark_name') || qDiv.querySelector('[class*="mark_name"]');
        if (markName) {
            var ps = markName.querySelectorAll('p');
            var textParts = [];
            for (var p = 0; p < ps.length; p++) {
                var partText = ps[p].textContent.replace(/\s+/g, ' ').trim();
                if (partText) textParts.push(partText);
            }
            if (textParts.length > 0) questionText = textParts.join('\n');
            if (!questionText) questionText = markName.textContent.replace(/\s+/g, ' ').trim();
        }
        if (!questionText) {
            var splitLeft = qDiv.querySelector('.splitS-left');
            if (splitLeft) questionText = splitLeft.textContent.replace(/\s+/g, ' ').trim();
        }

        var options = [];
        var stemAnswer = qDiv.querySelector('.stem_answer');
        if (stemAnswer) {
            var labelEls = stemAnswer.querySelectorAll('.answerBg > span[data]');
            var textEls  = stemAnswer.querySelectorAll('.answerBg > .answer_p');
            for (var j = 0; j < labelEls.length; j++) {
                options.push({
                    label: labelEls[j].getAttribute('data'),
                    text:  textEls[j] ? textEls[j].textContent.replace(/\s+/g, ' ').trim() : ''
                });
            }
        }

        var blankCount = 0;
        if (typeNum === 2) {
            var blankInput = getQuestionScopedInput(qDiv, 'input[name="blankNum' + qId + '"]')
                          || getQuestionScopedInput(qDiv, 'input[id*="blankNum' + qId + '"]');
            if (blankInput) {
                blankCount = blankInput.value.split(',').filter(function(v) { return v !== ''; }).length;
            }
            if (!blankCount && answerControl.mode === 'text-multi') {
                blankCount = answerControl.textareaCount;
            }
        }

        return {
            id:         qId,
            type:       typeNum,
            typeName:   typeName,
            answerMode: answerControl.mode,
            language:   answerControl.mode === 'code' ? getQuestionLanguage(qDiv, qId) : '',
            text:       questionText,
            options:    options,
            blankCount: blankCount
        };
    }

    function detectAnswerControl(qDiv) {
        if (!qDiv) return { mode: 'essay', type: 4, textareaCount: 0 };

        var stemAnswer = qDiv.querySelector('.stem_answer');
        if (stemAnswer) {
            var optionSpans = stemAnswer.querySelectorAll('.answerBg > span[data]');
            if (optionSpans.length > 0) {
                for (var i = 0; i < optionSpans.length; i++) {
                    var d = optionSpans[i].getAttribute('data');
                    if (d === 'true' || d === 'false') {
                        return { mode: 'choice', type: 3, textareaCount: 0 };
                    }
                }
                if (hasMultiChoiceMarker(stemAnswer)) {
                    return { mode: 'choice', type: 1, textareaCount: 0 };
                }
                return { mode: 'choice', type: 0, textareaCount: 0 };
            }
        }

        if (hasCodeAnswerControl(qDiv)) {
            return { mode: 'code', type: 4, textareaCount: 0 };
        }

        var answerTextareas = qDiv.querySelectorAll('textarea[name^="answer"], textarea[name^="answerEditor"]');
        if (answerTextareas.length > 1) return { mode: 'text-multi', type: 2, textareaCount: answerTextareas.length };
        if (answerTextareas.length === 1) return { mode: 'text-single', type: 4, textareaCount: 1 };

        return { mode: 'essay', type: 4, textareaCount: 0 };
    }

    function findQuestionDivFromNode(node) {
        if (!node) return null;
        var el = node.nodeType === 1 ? node : node.parentElement;
        while (el) {
            if (el.classList && el.classList.contains('questionLi')) return el;
            if (el.closest) {
                var closest = el.closest('.questionLi');
                if (closest) return closest;
            }
            el = el.parentElement;
        }
        return null;
    }

    function parseQuestionFromNode(node) {
        return parseQuestionFromDiv(findQuestionDivFromNode(node));
    }

    function parseQuestionFromEventTarget(target) {
        return parseQuestionFromNode(target) || parseQuestionFromDiv(document.querySelector('.questionLi'));
    }

    function parseCurrentQuestion() {
        var q = parseQuestionFromDiv(document.querySelector('.questionLi'));
        if (q) return q;
        var roots = findQuestionRootAll();
        for (var i = 0; i < roots.length; i++) {
            q = parseQuestionFromDiv(roots[i]);
            if (q && q.text) return q;
        }
        return null;
    }

    function getProgress() {
        var all = document.querySelectorAll('ul.topicNumber_list li');
        var cur = document.querySelector('ul.topicNumber_list li.current, ul.topicNumber_list li.active');
        var idx = 0;
        if (cur) idx = parseInt(cur.textContent.trim(), 10) || 0;
        return { current: idx, total: all.length };
    }

    /* ================================================================
     * AI API
     * ================================================================ */
    function plainAnswerInstruction() {
        return '输出格式要求：\n1. 只输出最终答案正文。禁止输出"答案："、"答："、"参考答案："、Markdown符号、代码块、标题、项目符号等前缀。\n2. 返回的答案中不要出现小括号内的内容（除非是必要的公式）。\n3. 不要一长段的内容紧挨着，适当换行，解答过程必须条理清晰。';
    }

    function buildPrompt(question) {
        switch (question.type) {
        case 0:
            return '【单选题】' + question.text + '\n\n选项：\n' +
                question.options.map(function(o) { return o.label + '. ' + o.text; }).join('\n') +
                '\n\n请只输出正确答案的字母（如：A），不要输出其他任何内容。';
        case 1:
            return '【多选题】' + question.text + '\n\n选项：\n' +
                question.options.map(function(o) { return o.label + '. ' + o.text; }).join('\n') +
                '\n\n请输出所有正确答案的字母（如：ABC），不要输出其他任何内容。';
        case 3:
            return '【判断题】' + question.text +
                '\n\n请只输出一个字："对" 或 "错"，不要输出其他任何内容。';
        case 2:
            var p = '【填空题】' + question.text;
            if (question.blankCount > 1)
                p += '\n\n该题有 ' + question.blankCount + ' 个空，请按顺序用 "|" 分隔每个空的答案。只输出答案内容本身。';
            else
                p += '\n\n请输出填空的答案内容，只输出答案本身，不要多余内容。';
            return p;
        case 7:
            return '【' + (question.typeName || '计算题') + '】' + question.text +
                '\n\n输出要求：\n' +
                '1. 请详细写出计算过程和最终答案，解答过程必须条理清晰。\n' +
                '2. 不要一长段的内容紧挨着，必须适当换行分段，保持排版整洁干净。\n' +
                '3. 返回的答案中不要出现小括号内的内容（除非是数学公式必需）。\n' +
                '4. 必须使用纯文本格式，禁止输出任何 Markdown 符号或 LaTeX 公式符号，请使用常规纯文本和标点表达数学公式。';
        case 4:
            if (question.answerMode === 'code') {
                return '【' + (question.typeName || '程序题') + '】' + question.text +
                    (question.language ? '\n\n当前编程语言：' + question.language : '') +
                    '\n\n请只输出完整可运行代码。禁止输出 Markdown 代码块、```、答案前缀、解释说明、运行结果或多余文字。';
            }
            return '【' + (question.typeName || '简答题') + '】' + question.text +
                '\n\n请简洁、专业地回答上述问题。\n' + plainAnswerInstruction();
        default:
            return '【' + (question.typeName || '未知题型') + '】' + question.text +
                '\n\n请根据题目给出最合适的最终答案。\n' + plainAnswerInstruction();
        }
    }

    function normalizeEssayAnswer(content) {
        content = String(content || '').trim();
        content = content
            .replace(/[\u200B-\u200D\uFEFF]/g, '')
            .replace(/^\s*#{1,6}\s*/gm, '')
            .replace(/^\s*(?:答案|答|参考答案|解析|说明)[:：]\s*/i, '')
            .replace(/\*\*([^*]+)\*\*/g, '$1')
            .replace(/__([^_]+)__/g, '$1')
            .replace(/`([^`]+)`/g, '$1')
            .trim();

        var previous;
        do {
            previous = content;
            content = content
                .replace(/^[“”"'「」『』]+|[“”"'「」『』]+$/g, '')
                .replace(/^[【\[\(（]+|[】\]\)）]+$/g, '')
                .trim();
        } while (content !== previous);

        return content;
    }

    function normalizeCodeAnswer(content) {
        content = String(content || '')
            .replace(/[\u200B-\u200D\uFEFF]/g, '')
            .trim();

        var fence = content.match(/^```[a-zA-Z0-9_+#.-]*\s*\n([\s\S]*?)\n?```\s*$/);
        if (fence) content = fence[1].trim();

        content = content
            .replace(/^\s*(?:答案|答|参考答案|代码|程序)[:：]\s*/i, '')
            .trim();

        return content;
    }

    function uniqueLetters(letters) {
        var seen = {};
        var out = [];
        for (var i = 0; i < letters.length; i++) {
            var ch = String(letters[i] || '').toUpperCase();
            if (!/^[A-Z]$/.test(ch) || seen[ch]) continue;
            seen[ch] = true;
            out.push(ch);
        }
        return out;
    }

    function parseChoiceLetters(content) {
        var text = normalizeEssayAnswer(content);
        var firstLine = text.split(/\r?\n/).map(function(line) {
            return line.trim();
        }).filter(Boolean)[0] || text;

        var compact = firstLine
            .replace(/(?:答案|正确答案|正确|选项|选择|选|为|是|和|与|及|以及)/g, '')
            .replace(/[,\uFF0C\u3001;；:：|/\\\s.。()[\]【】]/g, '');
        if (/^[A-Za-z]+$/.test(compact)) {
            return uniqueLetters(compact.split(''));
        }

        var letters = [];
        var re = /(?:^|[^A-Za-z])([A-Z])(?=$|[^A-Za-z])/ig;
        var m;
        while ((m = re.exec(firstLine)) !== null) {
            letters.push(m[1]);
        }
        return uniqueLetters(letters);
    }

    function splitBlankAnswer(content, blankCount) {
        var text = normalizeEssayAnswer(content);
        if (blankCount <= 1) return [text.trim()];

        function cleanParts(parts) {
            return parts.map(function(p) { return p.trim(); }).filter(function(p) { return p !== ''; });
        }

        var parts = cleanParts(text.split('|'));
        if (parts.length >= blankCount) return parts;

        var delimiters = [/\r?\n+/, /[;；]+/, /[,\uFF0C\u3001]+/];
        for (var i = 0; i < delimiters.length; i++) {
            parts = cleanParts(text.split(delimiters[i]));
            if (parts.length >= blankCount) return parts;
        }

        return cleanParts([text]);
    }

    function normalizeComparableText(text) {
        return String(text || '')
            .replace(/[\u200B-\u200D\uFEFF]/g, '')
            .replace(/<[^>]*>/g, '')
            .replace(/[“”"'「」『』]/g, '')
            .replace(/\s+/g, '')
            .trim()
            .toLowerCase();
    }

    function parseBoolAnswer(content) {
        var t = normalizeComparableText(content);
        if (!t) return false;

        if (t === '对' || t === '正确' || t === '是' || t === '真' ||
            t === '√' || t === 'true' || t === 'right' || t === 'yes' || t === 'correct') {
            return true;
        }
        if (t === '错' || t === '错误' || t === '否' || t === '假' ||
            t === '×' || t === 'false' || t === 'wrong' || t === 'no' || t === 'incorrect' ||
            t === '不正确' || t === '不对') {
            return false;
        }

        if (/(?:错|错误|否|假|×|✗|false|wrong|no|incorrect|不正确|不对)/.test(t)) return false;
        if (/(?:对|正确|是|真|√|✓|true|right|yes|correct)/.test(t)) return true;
        return false;
    }

    function parseAIResponse(rawContent, question) {
        var content = String(rawContent || '').trim();
        if (!(question.type === 4 && question.answerMode === 'code')) {
            content = content.replace(/^```[\s\S]*?\n/, '').replace(/\n```$/, '').trim();
        }

        switch (question.type) {
        case 0: {
            var singleLetters = parseChoiceLetters(content);
            return { type: 'single', answer: singleLetters.length ? singleLetters[0] : normalizeEssayAnswer(content) };
        }
        case 1:
            return { type: 'multi', answer: parseChoiceLetters(content) };
        case 3:
            return { type: 'bool', answer: parseBoolAnswer(content) };
        case 2:
            return { type: 'blank', answer: splitBlankAnswer(content, question.blankCount || 1) };
        case 7:
            return { type: 'essay', answer: normalizeEssayAnswer(content) };
        case 4:
            if (question.answerMode === 'code') {
                return { type: 'code', answer: normalizeCodeAnswer(content) };
            }
            return { type: 'essay', answer: normalizeEssayAnswer(content) };
        default:
            return { type: 'unknown', answer: normalizeEssayAnswer(content) };
        }
    }

    function getQuestionOptionSpans(qId) {
        var qDiv = findQuestionDiv(qId);
        if (!qDiv) return [];
        return qDiv.querySelectorAll('.stem_answer .answerBg > span[data]');
    }

    function optionData(span) {
        return String((span && span.getAttribute('data')) || '').trim().toUpperCase();
    }

    function findOptionSpanByData(qId, values) {
        var spans = getQuestionOptionSpans(qId);
        for (var i = 0; i < spans.length; i++) {
            var data = normalizeComparableText(spans[i].getAttribute('data'));
            var text = normalizeComparableText(spans[i].textContent);
            for (var j = 0; j < values.length; j++) {
                var v = normalizeComparableText(values[j]);
                if (data === v || text === v) return spans[i];
            }
        }
        return null;
    }

    function findOptionSpanByAnswerText(question, answerText) {
        var qDiv = findQuestionDiv(question.id);
        if (!qDiv) return null;
        var target = normalizeComparableText(answerText);
        if (!target) return null;

        var rows = qDiv.querySelectorAll('.stem_answer .answerBg');
        for (var i = 0; i < rows.length; i++) {
            var span = rows[i].querySelector('span[data]');
            var optionTextEl = rows[i].querySelector('.answer_p');
            var optionText = normalizeComparableText(optionTextEl ? optionTextEl.textContent : rows[i].textContent);
            if (span && optionText && (optionText === target || target.indexOf(optionText) !== -1 || optionText.indexOf(target) !== -1)) {
                return span;
            }
        }
        return null;
    }

    function clickOptionSpan(span) {
        if (!span) return false;
        var bg = span.closest('.answerBg');
        if (bg) {
            bg.click();
            return true;
        }
        span.click();
        return true;
    }

    function isChoiceSelected(span) {
        if (!span) return false;
        var bg = span.closest('.answerBg');
        return (span.classList && (span.classList.contains('check_answer') || span.classList.contains('check_answer_dx'))) ||
               (bg && bg.classList && (bg.classList.contains('check_answer') || bg.classList.contains('check_answer_dx'))) ||
               span.getAttribute('aria-checked') === 'true' ||
               (bg && bg.getAttribute && bg.getAttribute('aria-checked') === 'true');
    }

    function setSingleChoice(question, answerText) {
        var span = findOptionSpanByData(question.id, [answerText]);
        if (!span) span = findOptionSpanByAnswerText(question, answerText);
        if (!span) return fillTextFallback(question, { type: 'single', answer: answerText });
        if (isChoiceSelected(span)) return true;
        return clickOptionSpan(span);
    }

    function setBoolChoice(question, value) {
        var dataVal = value ? 'true' : 'false';
        var span = findOptionSpanByData(question.id, [dataVal, value ? '对' : '错']);
        if (!span) return fillTextFallback(question, { type: 'bool', answer: value });
        if (isChoiceSelected(span)) return true;
        return clickOptionSpan(span);
    }

    function setMultiChoice(question, letters) {
        letters = uniqueLetters(letters || []);
        if (!letters.length) return false;

        var qId = question.id;
        var spans = getQuestionOptionSpans(qId);
        var byData = {};
        for (var i = 0; i < spans.length; i++) {
            var data = optionData(spans[i]);
            if (data) byData[data] = spans[i];
        }

        var target = {};
        for (var j = 0; j < letters.length; j++) {
            target[letters[j]] = true;
            if (!byData[letters[j]]) return false;
        }

        // Click only the options that need to change. Do not verify selection
        // immediately — many platforms update class asynchronously.
        var clicked = 0;
        for (var k = 0; k < spans.length; k++) {
            var shouldSelect = !!target[optionData(spans[k])];
            if (isChoiceSelected(spans[k]) !== shouldSelect) {
                if (clickOptionSpan(spans[k])) clicked++;
            }
        }
        return clicked > 0 || letters.every(function(ch) {
            return byData[ch] && isChoiceSelected(byData[ch]);
        });
    }

    function isCodeQuestion(question) {
        return question && question.type === 4 && question.answerMode === 'code';
    }

    function deepSeekMaxTokens(question) {
        return isCodeQuestion(question) ? 8192 : 1024;
    }

    function buildContinuationPrompt(question) {
        if (isCodeQuestion(question)) {
            return '上一次输出因为长度限制被截断。请从上一条代码中断的位置继续输出剩余代码。不要重复已经输出的代码，不要解释，不要 Markdown，不要代码块。';
        }
        return '上一次输出因为长度限制被截断。请继续输出剩余答案内容，不要重复已经输出的内容，不要解释。';
    }

    /* Chaoxing / mooc exam DOM varies by version — accept several roots. */
    var QUESTION_ROOT_SELECTORS = [
        '.questionLi',
        '.exam-question',
        '.examPaper_question',
        '.question-item',
        '[class*="questionLi"]',
        '[id^="question_"]',
        '[data-question-id]',
        '.subject-item',
        '.tm-item',
        /* Live-exam variants seen on chaoxing */
        '.examQuestions',
        '.examQuestions > li',
        '.singleQues',
        '.multipleQues',
        '.judgeQues',
        '.fillQues',
        '.shortQues',
        '.TiMu',
        '.timu',
        '.tm',
        '#divQuestion',
        '.divQuestion',
        '[class*="Timu"]',
        '[class*="timu"]',
        '[id*="question"]',
        '[class*="Question"]',
        '.question-content',
        '.questionContent',
        '.topic-body',
        '.topicBody',
        '.exam-topic',
        'li[class*="question"]',
        'div[class*="question"]'
    ];

    function findQuestionRootAll() {
        var out = [];
        for (var i = 0; i < QUESTION_ROOT_SELECTORS.length; i++) {
            var nodes = document.querySelectorAll(QUESTION_ROOT_SELECTORS[i]);
            for (var j = 0; j < nodes.length; j++) {
                if (out.indexOf(nodes[j]) === -1) out.push(nodes[j]);
            }
            if (out.length) break; /* first matching selector family wins */
        }
        return out;
    }

    function hasExamDom() {
        try {
            if (document.querySelector('.questionLi')) return true;
            if (findQuestionRootAll().length > 0) return true;
            if (document.querySelector(
                '.stem_answer, .mark_name, textarea[name^="answer"], input[name^="type"], .answerBg')) {
                return true;
            }
        } catch (e) {}
        return false;
    }

    function isExamPage() {
        return hasExamDom();
    }

    function dumpExamDomHints(reason) {
        try {
            if (window.__LQ_DOM_DUMPED) return;
            window.__LQ_DOM_DUMPED = true;
            var sels = [
                '.questionLi', '.mark_name', '.stem_answer', '.answerBg',
                '[class*="question"]', '[class*="Timu"]', '[class*="timu"]',
                '[id*="question"]', 'iframe', 'textarea[name^="answer"]'
            ];
            var parts = [];
            for (var i = 0; i < sels.length; i++) {
                var n = 0;
                try { n = document.querySelectorAll(sels[i]).length; } catch (e) {}
                if (n > 0) parts.push(sels[i] + '=' + n);
            }
            var href = (window.location && window.location.href) || '';
            bridgeLog('warn', 'dom-hints ' + reason + ' href=' + href.substring(0, 90) +
                ' title=' + ((document.title || '').substring(0, 40)) +
                ' hits=' + (parts.join(',') || 'none') +
                ' iframes=' + window.frames.length);
        } catch (e) {}
    }

    function extractImages(qDiv) {
        var imgs = qDiv.querySelectorAll('img[src], img[data-src], img[data-original], img.ans-ued-img');
        var res = [];
        for (var i = 0; i < imgs.length; i++) {
            var url = imgs[i].getAttribute('data-original') || imgs[i].getAttribute('data-src') || imgs[i].getAttribute('src');
            if (url && url.indexOf('blob:') !== 0) {
                res.push(url);
            }
            if (imgs[i].hasAttribute('srcset')) {
                var srcset = imgs[i].getAttribute('srcset').split(',')[0].trim().split(' ')[0];
                if (srcset && srcset.indexOf('blob:') !== 0) res.push(srcset);
            }
        }
        var allEls = qDiv.querySelectorAll('*');
        for (var j = 0; j < allEls.length; j++) {
            var bg = window.getComputedStyle(allEls[j]).backgroundImage;
            if (bg && bg !== 'none' && bg.indexOf('url(') === 0) {
                var match = bg.match(/^url\(['"]?(.*?)['"]?\)/);
                if (match && match[1] && match[1].indexOf('blob:') !== 0) {
                    res.push(match[1]);
                }
            }
        }
        return res;
    }

    function hasImages(qDiv) {
        return extractImages(qDiv).length > 0;
    }

    function canUseVision(qDiv) {
        return hasImages(qDiv);
    }

    function imageUrlToDataUrl(url) {
        var absoluteUrl = url;
        try { absoluteUrl = new URL(url, document.baseURI).href; } catch (e) {}

        if (url.indexOf('data:image/') === 0) {
            return Promise.resolve({ rawUrl: absoluteUrl, base64Url: url, base64Raw: url.split(',')[1] || '' });
        }

        return new Promise(function(resolve, reject) {
            function renderToCanvas(img) {
                try {
                    var canvas = document.createElement('canvas');
                    var maxDim = 1024;
                    var width = img.naturalWidth || img.width || 800;
                    var height = img.naturalHeight || img.height || 600;

                    if (width > maxDim || height > maxDim) {
                        var ratio = Math.min(maxDim / width, maxDim / height);
                        width = Math.round(width * ratio);
                        height = Math.round(height * ratio);
                    }
                    canvas.width = width;
                    canvas.height = height;

                    var ctx = canvas.getContext('2d');
                    ctx.fillStyle = '#FFFFFF';
                    ctx.fillRect(0, 0, width, height);
                    ctx.drawImage(img, 0, 0, width, height);

                    var b64url = canvas.toDataURL('image/jpeg', 0.8);
                    resolve({ rawUrl: absoluteUrl, base64Url: b64url, base64Raw: b64url.split(',')[1] || '' });
                } catch (e) {
                    reject(e);
                }
            }

            var fetchOpts = {
                credentials: 'omit',
                referrer: window.location.href,
                referrerPolicy: 'unsafe-url'
            };

            fetch(url, fetchOpts)
                .then(function(res) {
                    if (!res.ok) throw new Error('HTTP ' + res.status);
                    return res.blob();
                })
                .then(function(blob) {
                    var objectUrl = URL.createObjectURL(blob);
                    var img = new Image();
                    img.onload = function() {
                        URL.revokeObjectURL(objectUrl);
                        renderToCanvas(img);
                    };
                    img.onerror = function() {
                        URL.revokeObjectURL(objectUrl);
                        reject(new Error('Blob decode failed'));
                    };
                    img.src = objectUrl;
                })
                .catch(function(err) {
                    console.warn('[AutoExam] Fetch image failed, falling back to direct DOM render:', err.message);
                    var img = new Image();
                    img.crossOrigin = 'anonymous';
                    img.onload = function() { renderToCanvas(img); };
                    img.onerror = function() { reject(err); };
                    img.src = url;
                });
        });
    }

    function prepareVisionImages(question) {
        var qDiv = findQuestionDiv(question.id) || document.getElementById(question.id);
        if (!qDiv) return Promise.resolve([]);
        var urls = extractImages(qDiv);
        if (urls.length === 0) return Promise.resolve([]);
        /* Cap images and time so middle-click cannot hang on CORS/network. */
        if (urls.length > 4) urls = urls.slice(0, 4);
        var promises = urls.map(function(u) {
            return Promise.race([
                imageUrlToDataUrl(u),
                new Promise(function(resolve) {
                    setTimeout(function() {
                        var absoluteUrl = u;
                        try { absoluteUrl = new URL(u, document.baseURI).href; } catch (err) {}
                        resolve({ rawUrl: absoluteUrl, base64Url: null, base64Raw: null });
                    }, 6000);
                })
            ]).catch(function(e) {
                console.warn('[AutoExam] Base64 conversion failed for ' + u + ', falling back to raw URL. Error:', e.message);
                var absoluteUrl = u;
                try { absoluteUrl = new URL(u, document.baseURI).href; } catch (err) {}
                return { rawUrl: absoluteUrl, base64Url: null, base64Raw: null };
            });
        });
        return Promise.all(promises).then(function(results) {
            lastVisionImageStats = { total: urls.length, success: results.length };
            return results;
        });
    }

    function buildUserContent(question) {
        return prepareVisionImages(question).then(function(imagesData) {
            var prompt = buildPrompt(question);
            prompt += '\n\n[System Note: 这是一个全新的独立请求，请完全忽略任何之前的历史对话上下文，从头开始完整解答本题。RequestID=' + Date.now() + ']';
            return {
                textPrompt: prompt,
                images: imagesData || []
            };
        });
    }

    function buildContentArray(userContentData, imageFormat) {
        if (imageFormat === 'text_only' || !userContentData.images || userContentData.images.length === 0) {
            return userContentData.textPrompt;
        }
        var content = [];
        for (var i = 0; i < userContentData.images.length; i++) {
            var imgData = userContentData.images[i];
            var urlValue = '';
            if (imageFormat === 'base64_url') urlValue = imgData.base64Url;
            else if (imageFormat === 'raw_url') urlValue = imgData.rawUrl;
            else if (imageFormat === 'base64_raw') urlValue = imgData.base64Raw;

            if (urlValue) {
                content.push({ type: 'image_url', image_url: { url: urlValue } });
            }
        }
        content.push({ type: 'text', text: userContentData.textPrompt });
        return content;
    }

    /* Bridge AI — send via console (page→DLL works), receive via poll
     * (DLL→JS execute_java_script is unreliable on Chaoxing frames). */
    var _aiPollSeq = 0;

    function sendAiConsoleRequest(params, id) {
        var envelope = {
            v: 2,
            id: id,
            type: 'request',
            method: 'ai.chatComplete',
            params: params || {},
            ts: Date.now()
        };
        var json = JSON.stringify(envelope);
        console.warn('[AutoExam] Bridge AI send id=' + id + ' bytes=' + json.length);
        try {
            var br = window.LingQiaoBridge;
            if (br && typeof br.__send === 'function') {
                if (br.__send(envelope)) return true;
                throw new Error('bridge transport unavailable');
            }
            /* Compatibility fallback for an older bootstrap. */
            if (console && typeof console.info === 'function') {
                console.info('__LQ_BRIDGE_V2__' + json);
                return true;
            }
            throw new Error('console transport unavailable');
        } catch (e) {
            console.warn('[AutoExam] Bridge AI send failed: ' + ((e && e.message) || e));
            return false;
        }
    }

    function pollAiResult(id, timeoutMs) {
        var start = Date.now();
        return new Promise(function(resolve, reject) {
            var settled = false;
            function finish(err, res) {
                if (settled) return;
                settled = true;
                window.removeEventListener('message', onMsg);
                /* Release the id so the main-frame relay stops polling it. */
                try {
                    if (window.__LQ_CANCEL_AI_POLL) window.__LQ_CANCEL_AI_POLL(id);
                } catch (eCancel) {}
                if (err) reject(err);
                else resolve(res);
            }
            function onMsg(ev) {
                var d = ev && ev.data;
                if (!d || d.type !== 'lqAiResult' || d.id !== id) return;
                if (d.result && d.result.error) finish(d.result.error);
                else finish(null, d.result);
            }
            window.addEventListener('message', onMsg);

            /* Prefer main-frame V8 relay (sync, reliable). */
            try {
                if (window.__LQ_REGISTER_AI_POLL) window.__LQ_REGISTER_AI_POLL(id);
            } catch (e) {}

            function tickConsole() {
                if (settled) return;
                if (Date.now() - start > timeoutMs) {
                    finish({ code: 'TIMEOUT', message: 'AI poll timed out after ' + timeoutMs + 'ms' });
                    return;
                }
                /* Fallback: direct V8 if this frame has native (main frame). */
                try {
                    if (window.__LingQiaoNative) {
                        var env = {
                            v: 2, id: 'aip_' + Date.now(), type: 'request',
                            method: 'ai.pollResult', params: { id: id }, ts: Date.now()
                        };
                        var raw = window.__LingQiaoNative.request(JSON.stringify(env));
                        if (raw) {
                            var parsed = JSON.parse(raw);
                            /* ok:false here means the DLL rejected the call — most
                             * often METHOD_NOT_FOUND because the loaded CefHook.dll
                             * predates the AI mailbox. Fail fast, do not spin. */
                            if (parsed && parsed.ok === false) {
                                finish(parsed.error || {
                                    code: 'METHOD_NOT_FOUND',
                                    message: 'ai.pollResult not available in this DLL build'
                                });
                                return;
                            }
                            if (parsed && parsed.ok && parsed.result && !parsed.result.pending) {
                                if (parsed.result.error) finish(parsed.result.error);
                                else finish(null, parsed.result);
                                return;
                            }
                        }
                    }
                } catch (eV) {}

                window.LingQiaoBridge.request('ai.pollResult', { id: id }, { timeout: 3000 })
                    .then(function(res) {
                        if (settled) return;
                        if (res && res.pending) {
                            setTimeout(tickConsole, 400);
                            return;
                        }
                        if (res && res.error) finish(res.error);
                        else finish(null, res);
                    })
                    .catch(function(err) {
                        if (settled) return;
                        /* Only a transport TIMEOUT is retryable. Any other bridge
                         * error (METHOD_NOT_FOUND / INVALID_REQUEST / ...) is
                         * terminal: retrying it forever is what left the page in
                         * the wait state without ever reporting a reason. */
                        if (err && err.code && err.code !== 'TIMEOUT') {
                            finish(err);
                            return;
                        }
                        setTimeout(tickConsole, 450);
                    });
            }
            setTimeout(tickConsole, 200);
        });
    }

    function requestModel(messages, question) {
        if (!canCallAi()) {
            return Promise.reject(new Error('LingQiaoBridge not ready'));
        }
        var id = 'ai_' + Date.now() + '_' + (++_aiPollSeq);
        var params = {
            messages: messages,
            max_tokens: deepSeekMaxTokens(question),
            temperature: 0.1
        };
        if (!sendAiConsoleRequest(params, id)) {
            return Promise.reject(new Error('console.info send failed'));
        }
        return pollAiResult(id, 90000).then(function(res) {
            if (!res || typeof res !== 'object') throw new Error('AI response format error');
            return {
                choices: [{
                    message: { content: res.content || '' },
                    finish_reason: res.finish_reason || 'stop'
                }]
            };
        });
    }

    function callModel(question) {
        /* Text-only path first: vision fetch/CORS can stall middle-click.
         * Keep prompt text; do not wait for image conversion. */
        var prompt = buildPrompt(question);
        prompt += '\n\n[System Note: 这是一个全新的独立请求，请完全忽略任何之前的历史对话上下文，从头开始完整解答本题。RequestID=' + Date.now() + ']';
        var userContentData = { textPrompt: prompt, images: [] };

        var sysMsg = {
            role: 'system',
            content: '你是一个专业的答题助手。根据题目给出准确答案。必须严格遵守用户消息中的输出格式要求。除非题目要求过程，否则不要输出推理过程或多余解释。'
        };

        var userContentArr = userContentData.textPrompt;
        var messages = [sysMsg, { role: 'user', content: userContentArr }];
        var rawContent = '';
        var continuationLimit = isCodeQuestion(question) ? 3 : 1;

        bridgeLog('info', 'callModel start text_only qid=' + (question.id || ''));

        function requestNext(attempt) {
            return requestModel(messages, question).then(function(data) {
                if (!data.choices || !data.choices[0] || !data.choices[0].message) {
                    throw new Error('API format error');
                }
                var choice = data.choices[0];
                var part = choice.message.content || '';
                rawContent += part;
                var lastFinishReason = choice.finish_reason || '';

                if (lastFinishReason === 'length' && attempt < continuationLimit) {
                    messages.push({ role: 'assistant', content: part });
                    messages.push({ role: 'user', content: buildContinuationPrompt(question) });
                    return requestNext(attempt + 1);
                }
                if (lastFinishReason === 'length') throw new Error('API output truncated');
                return rawContent;
            });
        }

        return requestNext(0).then(function(finalContent) {
            bridgeLog('info', 'callModel done len=' + (finalContent ? finalContent.length : 0));
            return parseAIResponse(finalContent, question);
        });
    }

    /* ================================================================
     * Answer filling
     * ================================================================ */
    function findEditorTextareas(qId, isBlank) {
        var list;
        var qDiv = findQuestionDiv(qId);
        if (!qDiv) return [];

        if (isBlank) {
            list = qDiv.querySelectorAll('textarea[name="answerEditor' + qId + '"], textarea[name^="answerEditor' + qId + '_"], textarea[name^="answerEditor' + qId + '-"]');
            if (list.length > 0) return list;
            list = qDiv.querySelectorAll('textarea[name^="answerEditor' + qId + '"]');
            if (list.length > 0) return list;
            list = qDiv.querySelectorAll('textarea[name="answer' + qId + '"][id*="Editor"], textarea[name^="answer' + qId + '_"][id*="Editor"], textarea[name^="answer' + qId + '-"][id*="Editor"]');
            if (list.length > 0) return list;
        } else {
            var ta = qDiv.querySelector('textarea[name="answer' + qId + '"]');
            if (ta) return [ta];
            list = qDiv.querySelectorAll('textarea[name="answerEditor' + qId + '"], textarea[name^="answerEditor' + qId + '_"], textarea[name^="answerEditor' + qId + '-"]');
            if (list.length > 0) return list;
            list = qDiv.querySelectorAll('textarea[name="answer' + qId + '"][id*="Editor"], textarea[name^="answer' + qId + '_"][id*="Editor"], textarea[name^="answer' + qId + '-"][id*="Editor"]');
            if (list.length > 0) return list;
        }
        list = qDiv.querySelectorAll('.eidtDiv textarea[id*="' + qId + '"]');
        if (list.length > 0) return list;
        list = qDiv.querySelectorAll('textarea[name^="answer"]');
        if (list.length > 0) return list;
        return [];
    }

    function findQuestionDiv(qId) {
        var sid = String(qId);
        var questions = findQuestionRootAll();
        for (var i = 0; i < questions.length; i++) {
            if (questions[i].getAttribute('data') === sid) return questions[i];
            if (questions[i].id === sid) return questions[i];
            var questionIdInput = questions[i].querySelector(
                'input#questionId, input[name="questionId"], ' +
                'input[name="questionId' + sid + '"], input[id*="questionId' + sid + '"]'
            );
            if (questionIdInput && questionIdInput.value === sid) return questions[i];
            if (questions[i].getAttribute('data-question-id') === sid ||
                questions[i].getAttribute('data-business-id') === sid) return questions[i];
        }
        return null;
    }

    function setTextareaValue(textarea, value) {
        textarea.value = value;
        textarea.dispatchEvent(new Event('input', { bubbles: true }));
        textarea.dispatchEvent(new Event('change', { bubbles: true }));
    }

    function htmlEncode(str) {
        return String(str || '')
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');
    }

    function setEditorContent(textarea, content) {
        if (!textarea) return;
        var encoded = htmlEncode(content);

        var container = textarea.closest('.eidtDiv') || textarea.closest('.subEditor') || textarea.parentElement;
        if (container) {
            var iframe = container.querySelector('iframe');
            if (iframe) {
                try {
                    var body = iframe.contentDocument && iframe.contentDocument.body;
                    if (body) {
                        body.innerHTML = '<p>' + encoded + '</p>';
                        body.dispatchEvent(new Event('input', { bubbles: true }));
                        try {
                            var ed = UE.getEditor(textarea.id);
                            if (ed && ed.sync) { ed.sync(); }
                        } catch (e) {}
                        if (!textarea.value) {
                            setTextareaValue(textarea, '<p>' + encoded + '</p>');
                        }
                        return;
                    }
                } catch (e) {}
            }
        }

        try {
            var editor = UE.getEditor(textarea.id);
            if (editor) {
                if (editor.isReady && editor.isReady()) {
                    editor.setContent(encoded);
                    try { editor.sync(); } catch (e) {}
                    return;
                }
                editor.ready(function() {
                    this.setContent(encoded);
                    try { this.sync(); } catch (e) {}
                });
                setTextareaValue(textarea, '<p>' + encoded + '</p>');
                return;
            }
        } catch (e) {}

        setTextareaValue(textarea, '<p>' + encoded + '</p>');
    }

    function getCodeLanguageValue(qDiv, qId) {
        if (!qDiv) return 0;
        var selectInput = qDiv.querySelector('input[name="languageSelect' + qId + '"], input#languageSelect' + qId + ', .languageSelect');
        var value = selectInput ? (selectInput.getAttribute('value') || selectInput.value) : '';
        if (!value) {
            var langBox = qDiv.querySelector('.langList[data-business-id="' + qId + '"], .langList');
            value = langBox ? langBox.getAttribute('codenum') : '';
        }
        return parseInt(value, 10) || 0;
    }

    function wrapCodeForSubmit(code) {
        var pre = document.createElement('pre');
        pre.className = 'line-numbers hover newProcedure';
        var codeEl = document.createElement('code');
        codeEl.className = 'language-plain';
        codeEl.setAttribute('lang', 'Plain Text');
        codeEl.textContent = code;
        pre.appendChild(codeEl);
        return pre.outerHTML;
    }

    function buildCodeSubmitValue(qDiv, qId, code) {
        return JSON.stringify([{
            language: getCodeLanguageValue(qDiv, qId),
            answer: wrapCodeForSubmit(code),
            answerTxt: htmlEncode(code)
        }]);
    }

    function fillCodeEditor(question, code) {
        var qId = question.id;
        var editor = findCodeEditor(qId);
        var wroteEditor = false;
        var wroteFallback = false;

        if (editor && typeof editor.setValue === 'function') {
            editor.setValue(code);
            try { editor.refresh(); } catch (e) {}
            try { editor.focus(); } catch (e) {}
            wroteEditor = true;
        }

        var qDiv = findQuestionDiv(qId);
        if (qDiv) {
            var hiddenAnswer = qDiv.querySelector('input[name="answer' + qId + '"], textarea[name="answer' + qId + '"]');
            if (hiddenAnswer) {
                setTextareaValue(hiddenAnswer, buildCodeSubmitValue(qDiv, qId, code));
                wroteFallback = true;
            }

            var codeTextarea = qDiv.querySelector('textarea.code-editor[data-business-id="' + qId + '"], textarea.code-editor');
            if (codeTextarea) {
                setTextareaValue(codeTextarea, code);
                wroteFallback = true;
            }

            var runBtn = qDiv.querySelector('#runBtn, .runBtn');
            if (runBtn) {
                runBtn.classList.remove('jb_btn_92_disable');
                runBtn.classList.add('jb_btn_92');
            }
        }

        return wroteEditor || wroteFallback;
    }

    function answerToText(answer) {
        if (!answer) return '';
        if (answer.type === 'code') return String(answer.answer || '');
        if (answer.type === 'multi') return (answer.answer || []).join('');
        if (answer.type === 'blank') return (answer.answer || []).join(' | ');
        if (answer.type === 'bool') return answer.answer ? '对' : '错';
        return String(answer.answer || '');
    }

    function fillTextFallback(question, answer) {
        var qDiv = findQuestionDiv(question.id);
        var control = detectAnswerControl(qDiv);
        if (control.mode !== 'text-single' && control.mode !== 'text-multi') return false;

        var textareas = findEditorTextareas(question.id, false);
        if (textareas.length === 0) return false;
        setEditorContent(textareas[0], answerToText(answer));
        return true;
    }

    function fillUnknownTextFallback(question, answer) {
        var qDiv = findQuestionDiv(question.id);
        if (!qDiv) return false;

        var control = detectAnswerControl(qDiv);
        if (control.mode === 'choice' || control.mode === 'code') return false;

        var direct = qDiv.querySelector('textarea[name="answer' + question.id + '"], textarea#answer' + question.id);
        if (direct && !direct.classList.contains('code-editor')) {
            setEditorContent(direct, answerToText(answer));
            return true;
        }

        var textareas = qDiv.querySelectorAll('textarea[name^="answer"], textarea[name^="answerEditor"]');
        var writable = [];
        for (var i = 0; i < textareas.length; i++) {
            if (textareas[i].classList.contains('code-editor')) continue;
            if (textareas[i].readOnly || textareas[i].disabled) continue;
            writable.push(textareas[i]);
        }
        if (writable.length === 1) {
            setEditorContent(writable[0], answerToText(answer));
            return true;
        }
        return false;
    }

    function fillAnswerSync(question, answer) {
        var qId = question.id;

        switch (answer.type) {
        case 'single':
            return setSingleChoice(question, answer.answer);
        case 'multi':
            return setMultiChoice(question, answer.answer);
        case 'bool':
            return setBoolChoice(question, answer.answer);
        case 'blank': {
            var textareas = findEditorTextareas(qId, true);
            if (textareas.length === 0) return false;
            var complete = true;
            for (var i = 0; i < textareas.length; i++) {
                var ans = answer.answer[i] || '';
                if (!ans) complete = false;
                setEditorContent(textareas[i], ans);
            }
            if (answer.answer.length < textareas.length) complete = false;
            return complete;
        }
        case 'essay': {
            var essayAreas = findEditorTextareas(qId, false);
            if (essayAreas.length === 0) return false;
            setEditorContent(essayAreas[0], answer.answer);
            return true;
        }
        case 'code':
            return fillCodeEditor(question, answer.answer);
        case 'unknown':
            return fillUnknownTextFallback(question, answer);
        default:
            return false;
        }
    }

    function fillAnswerAsync(question, answer) {
        return new Promise(function(resolve) {
            if (answer.type === 'code') {
                var codeStart = Date.now();
                function waitCodeEditor() {
                    if (findCodeEditor(question.id) || Date.now() - codeStart > 3000) {
                        resolve(fillAnswerSync(question, answer));
                        return;
                    }
                    setTimeout(waitCodeEditor, 150);
                }
                waitCodeEditor();
                return;
            }

            if (answer.type !== 'blank' && answer.type !== 'essay') {
                resolve(fillAnswerSync(question, answer));
                return;
            }

            var textareas = findEditorTextareas(question.id, answer.type === 'blank');
            if (textareas.length === 0) {
                resolve(fillAnswerSync(question, answer));
                return;
            }
            if (typeof UE === 'undefined' || !UE || typeof UE.getEditor !== 'function') {
                resolve(fillAnswerSync(question, answer));
                return;
            }

            var start = Date.now();
            function check() {
                var allReady = true;
                for (var i = 0; i < textareas.length; i++) {
                    try {
                        var ed = UE.getEditor(textareas[i].id);
                        if (!ed || !ed.isReady || !ed.isReady()) { allReady = false; break; }
                    } catch (e) { allReady = false; break; }
                }
                if (allReady || Date.now() - start > 5000) {
                    resolve(fillAnswerSync(question, answer));
                    return;
                }
                setTimeout(check, 150);
            }
            check();
        });
    }

    function answerDisplay(answer) {
        if (!answer) return '';
        if (answer.type === 'multi') return (answer.answer || []).join(',');
        if (answer.type === 'blank') return (answer.answer || []).join(' | ');
        if (answer.type === 'bool') return answer.answer ? '对' : '错';
        return String(answer.answer || '');
    }

    /* ---- Server-side exam record reporting ---- */
    var _reportBuffer = [];
    var _reportTimer = null;
    var _reportMaxBuffer = 200;

    function requeueReportBatch(batch) {
        _reportBuffer = batch.concat(_reportBuffer);
        if (_reportBuffer.length > _reportMaxBuffer) {
            _reportBuffer.splice(0, _reportBuffer.length - _reportMaxBuffer);
        }
        if (!_reportTimer) {
            _reportTimer = setTimeout(flushReportBuffer, 5000);
        }
    }

    function reportToServer(question, answer, ok, elapsedMs) {
        _reportBuffer.push({
            question_id: String(question.id || ''),
            question_text: (question.text || '').substring(0, 500),
            question_type: question.typeName || '',
            options_json: JSON.stringify(question.options || []),
            provided_answer: answerDisplay(answer),
            correct_answer: '',
            model_name: config.model || '',
            confidence: (answer && typeof answer.confidence === 'number') ? answer.confidence : 0,
            answer_time_ms: elapsedMs || 0,
            fill_ok: !!ok
        });
        if (_reportBuffer.length >= 5) {
            flushReportBuffer();
        } else if (!_reportTimer) {
            _reportTimer = setTimeout(flushReportBuffer, 3000);
        }
    }

    function flushReportBuffer() {
        if (_reportBuffer.length === 0) return;
        if (_reportTimer) { clearTimeout(_reportTimer); _reportTimer = null; }
        var batch = _reportBuffer.splice(0);

        var bridgeReady = !!(window.LingQiaoBridge && window.LingQiaoBridge.ready);
        if (bridgeReady) _bridgeReady = true;

        if (bridgeReady) {
            try {
                var bridgeSettled = false;
                window.LingQiaoBridge.request('page.reportExamRecord', {
                    records: batch
                }, { timeout: 5000 }).then(function() {
                    if (bridgeSettled) return;
                    bridgeSettled = true;
                }).catch(function() {
                    if (bridgeSettled) return;
                    bridgeSettled = true;
                    requeueReportBatch(batch);
                });
                setTimeout(function() {
                    if (!bridgeSettled) {
                        bridgeSettled = true;
                        requeueReportBatch(batch);
                    }
                }, 8000);
                return;
            } catch (e) {
                /* fall through */
            }
        }

        requeueReportBatch(batch);
    }

    /* ================================================================
     * Middle-click answering
     * ================================================================ */
    var middleBtnState = 'idle'; // idle | loading | ready
    var pendingAnswerData = null;
    var _loadingWatchdog = null;
    var FETCH_LOCK_KEY = '__autoExamFetchLock';
    var FETCH_LOCK_TTL_MS = 70000;
    var LOADING_WATCHDOG_MS = 45000;

    /* Cross-frame single-flight: poller may inject trigger into every frame. */
    function tryAcquireFetchLock() {
        try {
            var now = Date.now();
            var raw = sessionStorage.getItem(FETCH_LOCK_KEY);
            if (raw) {
                var ts = parseInt(raw, 10);
                if (!isNaN(ts) && (now - ts) < FETCH_LOCK_TTL_MS) return false;
            }
            sessionStorage.setItem(FETCH_LOCK_KEY, String(now));
            return true;
        } catch (e) {
            return true;
        }
    }

    function releaseFetchLock() {
        try { sessionStorage.removeItem(FETCH_LOCK_KEY); } catch (e) {}
    }

    function armLoadingWatchdog(source) {
        if (_loadingWatchdog) clearTimeout(_loadingWatchdog);
        _loadingWatchdog = setTimeout(function() {
            _loadingWatchdog = null;
            if (middleBtnState === 'loading') {
                resetAnswerStates();
                bridgeLog('error', source + ' watchdog timeout');
                console.error('[AutoExam] ' + source + ' watchdog timeout');
            }
        }, LOADING_WATCHDOG_MS);
    }

    function resetAnswerStates() {
        middleBtnState = 'idle';
        pendingAnswerData = null;
        if (_loadingWatchdog) {
            clearTimeout(_loadingWatchdog);
            _loadingWatchdog = null;
        }
        releaseFetchLock();
        clearCursorLoading();
    }

    function beginLoading(source) {
        middleBtnState = 'loading';
        /* Arm watchdog first so any later throw still recovers the cursor. */
        armLoadingWatchdog(source);
        setCursorLoading();
    }

    function toggleEnabled() {
        enabled = !enabled;
        saveEnabledPreference(enabled);
        if (!enabled) resetAnswerStates();
        console.log('[AutoExam] ' + (enabled ? 'Enabled' : 'Disabled'));
    }

    /* Shared fetch/fill used by middle-click and F2 (F2 helps when the
     * native exam shell swallows middle-button events). */
    function triggerAnswerAction(source, target) {
        if (!enabled) {
            bridgeLog('warn', source + ': disabled');
            return;
        }
        if (!isExamPage()) {
            bridgeLog('warn', source + ': no exam DOM on this page url=' +
                (window.location && window.location.href || '').substring(0, 100));
            return;
        }
        var tag = (target && target.tagName || '').toLowerCase();
        if (tag === 'input' || tag === 'textarea' || (target && target.isContentEditable)) return;

        if (middleBtnState === 'ready' && pendingAnswerData) {
            var fillStart = pendingAnswerData.startTime || Date.now();
            var question = pendingAnswerData.question;
            var answer = pendingAnswerData.answer;
            fillAnswerAsync(question, answer).then(function(ok) {
                resetAnswerStates();
                reportToServer(question, answer, ok, Date.now() - fillStart);
                bridgeLog('info', source + ' fill ok=' + (ok ? '1' : '0'));
                console.warn('[AutoExam] ' + source + ' fill ' + (ok ? 'done' : 'failed'));
            }).catch(function(err) {
                resetAnswerStates();
                bridgeLog('error', source + ' fill error ' + ((err && (err.message || err.code)) || 'fail'));
                console.error('[AutoExam] ' + source + ' fill error:', (err && err.message) || 'fill failed');
            });
            return;
        }

        if (middleBtnState === 'loading') return;

        if (!canCallAi()) {
            bridgeLog('error', source + ': bridge not ready');
            console.error('[AutoExam] LingQiaoBridge not ready for AI');
            return;
        }

        /* Abort before entering the wait state when the loaded DLL has no AI
         * mailbox — otherwise the cursor spins until the watchdog fires. */
        if (_aiCapState === 'missing') {
            bridgeLog('error', source + ': aborted, DLL lacks ai.pollResult');
            console.error('[AutoExam] ' + source + ' 中止：当前 CefHook.dll 不含 ai.pollResult，' +
                '请更新 DLL 后再试。');
            return;
        }

        var q = parseQuestionFromEventTarget(target) || parseCurrentQuestion();
        if (!q) {
            bridgeLog('warn', source + ': no question parsed roots=' + findQuestionRootAll().length);
            console.warn('[AutoExam] No question detected');
            return;
        }

        if (!tryAcquireFetchLock()) {
            bridgeLog('info', source + ' skip: another frame is fetching');
            return;
        }

        beginLoading(source);
        var startTime = Date.now();
        console.warn('[AutoExam] ' + source + ': fetching answer...');
        try {
            bridgeReportQuestionState(q.id, q.typeName, 'fetching');
            bridgeLog('info', source + ' start qid=' + (q.id || '') + ' type=' + (q.type || -1) +
                ' page=' + (window.location && window.location.href || '').substring(0, 80));
        } catch (eLog) {}

        var cached = null;
        try { cached = getCachedAnswer(q); } catch (eCache) { cached = null; }
        if (cached) {
            pendingAnswerData = { question: q, answer: cached, startTime: startTime };
            middleBtnState = 'ready';
            if (_loadingWatchdog) {
                clearTimeout(_loadingWatchdog);
                _loadingWatchdog = null;
            }
            releaseFetchLock();
            clearCursorLoading();
            console.warn('[AutoExam] ' + source + ': cached answer ready');
            return;
        }

        try {
            callModel(q).then(function(answer) {
                if (middleBtnState !== 'loading') return;
                cacheAnswer(q, answer);
                pendingAnswerData = { question: q, answer: answer, startTime: startTime };
                middleBtnState = 'ready';
                if (_loadingWatchdog) {
                    clearTimeout(_loadingWatchdog);
                    _loadingWatchdog = null;
                }
                releaseFetchLock();
                clearCursorLoading();
                bridgeLog('info', source + ' ready');
                console.warn('[AutoExam] ' + source + ': answer ready (trigger again to fill)');
            }).catch(function(err) {
                resetAnswerStates();
                bridgeReportQuestionState(q.id, q.typeName, 'fetch_failed');
                bridgeLog('error', source + ' api error ' + ((err && (err.message || err.code)) || 'fail'));
                console.error('[AutoExam] ' + source + ' API error:', (err && err.message) || 'request failed');
            });
        } catch (err) {
            resetAnswerStates();
            bridgeReportQuestionState(q.id, q.typeName, 'fetch_failed');
            bridgeLog('error', source + ' sync throw ' + ((err && err.message) || 'fail'));
            console.error('[AutoExam] ' + source + ' sync error:', (err && err.message) || 'throw');
        }
    }

    /* Capture-phase: log if middle button even reaches the page. */
    document.addEventListener('mousedown', function(e) {
        if (e.button === 1) {
            bridgeLog('info', 'raw mousedown button=1 tag=' + ((e.target && e.target.tagName) || '?'));
        }
    }, true);

    document.addEventListener('mousedown', function(e) {
        if (e.button !== 1) return;
        e.preventDefault();
        triggerAnswerAction('middle-click', e.target);
    });

    document.addEventListener('auxclick', function(e) {
        if (e.button === 1 && isExamPage()) {
            e.preventDefault();
        }
    });

    /* ================================================================
     * Keyboard: Tab toggle, F3 cursor feedback, Escape cancel
     * ================================================================ */
    document.addEventListener('keydown', function(e) {
        var tag = (e.target.tagName || '').toLowerCase();
        if (tag === 'input' || tag === 'textarea' || e.target.isContentEditable) return;

        if (e.key === 'Tab') {
            e.preventDefault();
            if (window.AutoExam && window.AutoExam.toggle) window.AutoExam.toggle();
            return;
        }
        if (e.key === 'F3') {
            e.preventDefault();
            if (window.AutoExam && window.AutoExam.toggleCursorFeedback) {
                window.AutoExam.toggleCursorFeedback();
            }
            return;
        }
        if (e.key === 'Escape') {
            if (middleBtnState !== 'idle') {
                resetAnswerStates();
                console.log('[AutoExam] Cancelled (Escape)');
            }
        }
    });

    /* ================================================================
     * Init
     * ================================================================ */
    function init() {
        var forceSelect = document.createElement('style');
        forceSelect.id = '__cefForceSelect';
        forceSelect.textContent = 'body,body *{user-select:text!important;-webkit-user-select:text!important;-moz-user-select:text!important;-ms-user-select:text!important}';
        document.head.appendChild(forceSelect);

        try { document.onselectstart = null; } catch (e) {}
        try { document.body.onselectstart = null; } catch (e) {}

        console.log('[AutoExam] Initialized — model: ' + config.model +
                    ' | bridge: ' + (_bridgeReady ? ('v' + _bridgeInfo.version + '/' + _bridgeInfo.transport) : 'none') +
                    ' | hotkeys: Tab=toggle F3=cursor Middle=fetch/fill Escape=cancel');

        bridgeLog('info', 'AutoExam initialized, model=' + config.model +
                  ', aiViaBridge=' + (canCallAi() ? '1' : '0') +
                  ', examDom=' + (hasExamDom() ? '1' : '0') +
                  ', href=' + ((window.location && window.location.href) || '').substring(0, 80));
        if (!hasExamDom()) dumpExamDomHints('init');
        /* Verify the loaded DLL actually exposes the AI methods. */
        probeAiCapability();
        /* Ask the bootstrap to (re)start its V8 relay. The bootstrap owns the
         * single relay implementation; a second copy here previously meant two
         * aiIds tables that had to be kept in sync by hand. */
        try {
            if (!window.__LQ_V8_RELAY) {
                var evRelay = document.createEvent('Event');
                evRelay.initEvent('lqWantV8Relay', true, true);
                window.dispatchEvent(evRelay);
            }
        } catch (eRelay) {}
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    window.AutoExam = {
        debugInfo: window.debugInfo,
        toggle: toggleEnabled,
        toggleCursorFeedback: toggleCursorFeedback,
        /* Called from C++ when the native shell swallows keyboard/mouse. */
        trigger: function(source) {
            var src = source || 'native';
            var hasRoot = hasExamDom();
            var roots = 0;
            try { roots = findQuestionRootAll().length; } catch (e) {}
            bridgeLog('info', 'trigger src=' + src + ' hasRoot=' + hasRoot +
                ' roots=' + roots + ' state=' + middleBtnState +
                ' href=' + ((window.location && window.location.href) || '').substring(0, 80));
            console.warn('[AutoExam] trigger src=' + src + ' hasRoot=' + hasRoot + ' roots=' + roots);
            if (!hasRoot) {
                dumpExamDomHints('trigger-noroot');
                return;
            }
            triggerAnswerAction(src, document.activeElement || document.body);
        }
    };

    /* Native poller pushes {type:event, method:nativeTrigger} via the proven
     * bridge deliver path — execute_java_script from the shell often misses
     * Chaoxing frames. */
    try {
        if (window.LingQiaoBridge && window.LingQiaoBridge.on) {
            window.LingQiaoBridge.on('nativeTrigger', function(params) {
                var src = (params && params.source) || 'native';
                bridgeLog('info', 'bridge event nativeTrigger src=' + src);
                if (window.AutoExam && window.AutoExam.trigger) {
                    window.AutoExam.trigger(src);
                }
            });
            bridgeLog('info', 'nativeTrigger listener registered');
        }
    } catch (eEv) {
        bridgeLog('warn', 'nativeTrigger listener failed: ' + ((eEv && eEv.message) || 'err'));
    }

    /* Receive trigger from main-frame V8 poll relay via postMessage. */
    window.addEventListener('message', function(ev) {
        var d = ev && ev.data;
        if (!d || d.type !== 'lqNativeTrigger') return;
        var src = d.source || 'poll-MBUTTON';
        bridgeLog('info', 'postMessage nativeTrigger src=' + src);
        if (window.AutoExam && window.AutoExam.trigger) {
            window.AutoExam.trigger(src);
        }
    });

    /* Own poll fallback when this frame has no V8 relay parent.
     * Also kick the main-frame V8 relay if native appears late. */
    /* Per-frame DOM state report.
     *
     * The native trigger is only released to a frame that polls ready=1, so
     * "I am definitely on the exam page but nothing happens" can only be
     * explained by looking at what each frame actually sees. This logs one line
     * per frame whenever its URL changes or its readiness flips — low volume,
     * and it names the frame (top or child), the selectors that matched, and the
     * iframe count, which is what distinguishes "wrong frame" from "markup
     * changed". Cross-origin children are invisible to the parent frame, so the
     * child must report for itself. */
    var _domStateHref = null;
    var _domStateReady = null;
    var _domStateIframes = -1;
    var _domStateAt = 0;

    /* When no exam DOM is recognised, the useful question is "what markup does
     * this page actually use?". Sampling the most frequent class names answers it
     * from the log alone, without anyone having to copy HTML by hand.
     * Note: diagnostics.log truncates the message at 512 bytes, so keep this
     * sample short. */
    function sampleClassNames(limit) {
        try {
            var nodes = document.querySelectorAll('body *');
            var n = Math.min(nodes.length, 1200);
            var counts = {}, order = [];
            for (var i = 0; i < n; i++) {
                var cls = nodes[i].className;
                if (typeof cls !== 'string' || !cls) continue;
                var parts = cls.split(/\s+/);
                for (var j = 0; j < parts.length; j++) {
                    var c = parts[j];
                    if (!c || c.length > 32 || c.charAt(0) === '_') continue;
                    if (counts[c] === undefined) { counts[c] = 0; order.push(c); }
                    counts[c]++;
                }
            }
            order.sort(function(a, b) { return counts[b] - counts[a]; });
            var out = [];
            for (var k = 0; k < order.length && k < limit; k++) {
                out.push(order[k] + ':' + counts[order[k]]);
            }
            return out.join(' ');
        } catch (e) { return ''; }
    }

    function reportDomState(ready) {
        try {
            var now = Date.now();
            var href = (window.location && window.location.href) || '';
            var iframes = 0;
            try { iframes = document.querySelectorAll('iframe').length; } catch (e) {}
            /* Re-log when an iframe appears or disappears too: an exam paper that
             * loads into a child frame changes neither this frame's URL nor its
             * readiness, so that transition would otherwise leave no trace. */
            if (href === _domStateHref && ready === _domStateReady &&
                iframes === _domStateIframes) return;
            if (now - _domStateAt < 2000) return;
            _domStateHref = href;
            _domStateReady = ready;
            _domStateIframes = iframes;
            _domStateAt = now;
            var sels = ['.questionLi', '.mark_name', '.stem_answer', '.answerBg',
                        'textarea[name^="answer"]', 'input[name^="type"]', 'iframe'];
            var counts = [];
            for (var i = 0; i < sels.length; i++) {
                var n = 0;
                try { n = document.querySelectorAll(sels[i]).length; } catch (e) {}
                counts.push(sels[i] + '=' + n);
            }
            var line = 'dom-state top=' + (window.top === window ? 1 : 0) +
                ' ready=' + (ready ? 1 : 0) + ' ' + counts.join(' ');
            if (!ready) {
                var sample = sampleClassNames(14);
                if (sample) line += ' classes=' + sample.substring(0, 170);
            }
            line += ' href=' + href.substring(0, 110);
            bridgeLog('warn', line);
        } catch (e) {}
    }

    (function startNativeTriggerPoll() {
        var busy = false;
        var busySince = 0;
        var kickTries = 0;
        setInterval(function() {
            /* A stuck busy flag would silently stop the trigger poll for the
             * rest of the page's life, which looks identical to a dead AI path.
             * Clear it if a request somehow never settles. */
            if (busy && busySince && (Date.now() - busySince) > 8000) busy = false;
            /* One DOM scan per tick, shared by the diagnostic and the poll. */
            var hasQ = hasExamDom();
            reportDomState(hasQ);
            if (window.__LQ_V8_RELAY) return;
            /* Main frame: if native is ready but relay not started, start it. */
            if (window.__LingQiaoNative && !window.__LQ_V8_RELAY && kickTries < 40) {
                kickTries++;
                try {
                    /* Re-run relay installer if bootstrap missed the timing. */
                    if (!window.__LQ_RELAY_RETRY) {
                        window.__LQ_RELAY_RETRY = true;
                        var ev = document.createEvent('Event');
                        ev.initEvent('lqWantV8Relay', true, true);
                        window.dispatchEvent(ev);
                    }
                } catch (e) {}
            }
            if (busy || !enabled) return;
            if (!canCallAi()) return;
            if (middleBtnState === 'loading') return;
            if (window.__LQ_V8_RELAY) return;
            busy = true;
            busySince = Date.now();
            window.LingQiaoBridge.request('native.pollTrigger', { ready: hasQ ? 1 : 0 }, { timeout: 2500 })
                .then(function(res) {
                    busy = false;
                    if (res && res.pending) {
                        var src = res.source || 'poll-MBUTTON';
                        bridgeLog('info', 'polled nativeTrigger src=' + src);
                        if (window.AutoExam && window.AutoExam.trigger) {
                            window.AutoExam.trigger(src);
                        }
                    }
                })
                .catch(function() { busy = false; });
        }, 300);
    })();
})();
