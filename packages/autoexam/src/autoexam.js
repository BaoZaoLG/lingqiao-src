/**
 * AutoExam.js — Embedded lightweight exam auto-answer system
 * Injected by CefHook.dll on every page load.
 *
 * Answering modes:
 *   Tab          — Toggle answering on/off
 *   F2           — Answer current question
 *   F3           — Toggle cursor feedback (loading spinner)
 *   F4           — Toggle AI model
 *   Middle-click — Fetch answer, middle-click again to fill
 *   Ctrl+C       — Copy question to fetch answer, middle-click to fill
 *   Escape       — Cancel pending middle-click / clipboard operation
 *
 * API Key is read from window.__DEEPSEEK_API_KEY (set by CefHook.dll from INJECTOR_API_KEY env var).
 */
(function() {
    'use strict';

    /* Silence all console output */
    console.log = console.warn = console.error = console.info = console.debug = function(){};

    /* ================================================================
     * Constants & In-memory State
     * ================================================================ */

    var AI_CONFIG = window.__AUTOEXAM_AI_CONFIG || {
        baseUrl: 'https://api.xiaomimimo.com/v1',
        textModel: 'mimo-v2.5',
        visionModel: 'mimo-v2.5',
        adapter: 'openai-chat',
        supportsVision: false,
        provider: 'mimo',
        apiKey: ''
    };

    var API_URL = (AI_CONFIG.baseUrl || 'https://api.deepseek.com').replace(/\/+$/, '');
    if (AI_CONFIG.adapter === 'anthropic-messages') {
        API_URL += '/v1/messages';
    } else {
        API_URL += '/chat/completions';
    }

    var CURSOR_FEEDBACK_STORAGE_KEY = '__autoExamCursorFeedback';
    var ENABLED_STORAGE_KEY = '__autoExamEnabled';

    var config = {
        model: AI_CONFIG.textModel,
        provider: AI_CONFIG.provider,
        visionModel: AI_CONFIG.visionModel,
        supportsVision: AI_CONFIG.supportsVision
    };

    var lastVisionImageStats = null;
    window.debugInfo = function() {
        return {
            config: config,
            apiUrl: API_URL,
            lastVisionImageStats: lastVisionImageStats
        };
    };

    window.toggleProvider = function() {
        alert('Please use the client UI (F4 equivalent) to toggle provider');
    };

    function loadEnabledPreference() {
        try {
            var saved = sessionStorage.getItem(ENABLED_STORAGE_KEY);
            if (saved === '0') return false;
            if (saved === '1') return true;
        } catch(e) {}
        return true;
    }

    function saveEnabledPreference(value) {
        try {
            sessionStorage.setItem(ENABLED_STORAGE_KEY, value ? '1' : '0');
        } catch(e) {}
    }

    var enabled = loadEnabledPreference();
    var cursorFeedback = loadCursorFeedbackPreference();
    var cursorStyleEl = null;
    var batchAnswering = false;

    function loadCursorFeedbackPreference() {
        try {
            var saved = sessionStorage.getItem(CURSOR_FEEDBACK_STORAGE_KEY);
            if (saved === '0') return false;
            if (saved === '1') return true;
        } catch(e) {}
        return true;
    }

    function saveCursorFeedbackPreference(value) {
        try {
            sessionStorage.setItem(CURSOR_FEEDBACK_STORAGE_KEY, value ? '1' : '0');
        } catch(e) {}
    }

    function ensureCursorStyle() {
        if (!cursorStyleEl) {
            cursorStyleEl = document.createElement('style');
            cursorStyleEl.id = '__cefCursorFeedback';
            document.head.appendChild(cursorStyleEl);
        }
        return cursorStyleEl;
    }

    function getApiKey() {
        if (typeof window.__DEEPSEEK_API_KEY === 'string' && window.__DEEPSEEK_API_KEY)
            return window.__DEEPSEEK_API_KEY;
        return '';
    }

    /* ================================================================
     * Answer Cache — avoid redundant API calls for same question
     * Key: question text hash, Value: parsed answer
     * ================================================================ */
    var answerCache = {};
    var CACHE_MAX = 200;

    function hashQuestion(text) {
        var hash = 0;
        for (var i = 0; i < text.length; i++) {
            hash = ((hash << 5) - hash) + text.charCodeAt(i);
            hash = hash & hash; // Convert to 32bit integer
        }
        return 'q_' + Math.abs(hash).toString(36);
    }

    function questionCacheKey(question) {
        var provider = config.provider || 'unknown';
        if (typeof question === 'string') return question;
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
            'model=' + config.model
        ].join('\n');
    }

    function getCachedAnswer(question) {
        var key = hashQuestion(questionCacheKey(question));
        return answerCache[key] || null;
    }

    function cacheAnswer(question, answer) {
        var key = hashQuestion(questionCacheKey(question));
        answerCache[key] = answer;
        // Evict oldest entries if cache is too large
        var keys = Object.keys(answerCache);
        if (keys.length > CACHE_MAX) {
            delete answerCache[keys[0]];
        }
    }

    /* ================================================================
     * Question Parsing
     *
     * Extracts structured question data from the exam page DOM.
     * Returns: { id, type, typeName, text, options:[{label,text}], blankCount }
     * type: 0=single 1=multi 2=blank 3=bool 4=essay
     * ================================================================ */

    /* ================================================================
     * DOM-based type detection fallback.
     * Inspects actual page elements when keyword matching fails.
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
                // 对/错 按钮 → 判断题
                for (var i = 0; i < optionSpans.length; i++) {
                    var d = optionSpans[i].getAttribute('data');
                    if (d === 'true' || d === 'false') return 3;
                }
                // 有字母选项 → 选择题（默认单选，多选通常有 checkbox/multi 类名标记）
                if (hasMultiChoiceMarker(stemAnswer))
                    return 1;
                return 0;
            }
        }

        // 多个独立文本框 → 填空题
        var blankTextareas = qDiv.querySelectorAll('textarea[name^="answerEditor"]');
        if (blankTextareas.length > 1) return 2;

        // 有文本框 → 简答/论述
        if (qDiv.querySelector('textarea[name^="answer"]')) return 4;

        return 4;
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
        return !!qDiv.querySelector('.codeEditorBoxDiv[data-business-id], textarea.code-editor[data-business-id], [id^="procedural-"] .CodeMirror, .CodeMirror');
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
            var selectedByValue = qDiv.querySelector('.langList[data-business-id="' + qId + '"] [codenum="' + selectInput.value + '"], .langList [codenum="' + selectInput.value + '"]');
            if (selectedByValue) return selectedByValue.getAttribute('codename') || selectedByValue.textContent.replace(/\s+/g, ' ').trim();
        }

        var selected = qDiv.querySelector('.langList[data-business-id="' + qId + '"] [codename].active, .langList[data-business-id="' + qId + '"] [codename].on, .langList[data-business-id="' + qId + '"] [codename].selected')
                    || qDiv.querySelector('.langList [codename].active, .langList [codename].on, .langList [codename].selected')
                    || qDiv.querySelector('.langList[data-business-id="' + qId + '"] [codename], .langList [codename]');
        if (selected) return selected.getAttribute('codename') || selected.textContent.replace(/\s+/g, ' ').trim();

        var langMap = {
            '1': 'C',
            '2': 'C#',
            '3': 'Java',
            '4': 'VB.NET',
            '5': 'Python 2.x',
            '6': 'Go',
            '8': 'PHP',
            '11': 'Bash',
            '16': 'Python 3.6.9',
            '17': 'C++',
            '19': 'JavaScript',
            '20': 'R',
            '22': 'Python 3.12.12'
        };
        return selectInput ? (langMap[selectInput.value] || selectInput.value || '') : '';
    }

    function getQuestionScopedInput(qDiv, selector) {
        return qDiv ? qDiv.querySelector(selector) : null;
    }

    function parseQuestionFromDiv(qDiv) {
        if (!qDiv) return null;

        var qId = qDiv.getAttribute('data') || qDiv.id;
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
        // [Refactor]: Ensure qDiv always has an absolute ID for document.getElementById lookup
        if (!qId) qId = '__autoexam_q_' + Date.now() + Math.floor(Math.random() * 10000);
        qDiv.id = qId; 

        var typeNameInput = getQuestionScopedInput(qDiv, 'input[name="typeName' + qId + '"]')
                         || getQuestionScopedInput(qDiv, 'input[id*="typeName' + qId + '"]');
        var typeNumInput  = getQuestionScopedInput(qDiv, 'input[name="type' + qId + '"]')
                         || getQuestionScopedInput(qDiv, 'input[id*="type' + qId + '"]');
        var typeName = typeNameInput ? typeNameInput.value : '';
        var typeNum  = typeNumInput  ? parseInt(typeNumInput.value, 10) : -1;
        if (isNaN(typeNum)) typeNum = -1;

        if (typeNum === -1 && typeName) {
            // 关键词模糊匹配：兼容平台题型名称变体（如"选择题"→"单选题"、"分析题"→"论述题"、"选填题"→"填空题"等）
            var TYPE_KEYWORDS = [
                { pattern: /单选|单项选择|单选选择/,                                      type: 0 },
                { pattern: /多选|多项选择|复选|多选选择|共用选项|选做/,                      type: 1 },
                { pattern: /填空|填框|填充|填入|选填|选词填空|完型填空|完形填空|补全/,          type: 2 },
                { pattern: /判断|对错|是非|正误/,                                         type: 3 },
                { pattern: /计算/,                                                       type: 7 },
                { pattern: /简答|论述|分析|问答|名词解释|阅读理解|案例分析|综合|输入|主观|程序|代码|编程|分录|听力|口语|测评|写作|其它|其他|连线|排序/, type: 4 }
            ];
            var _matched = false;
            for (var _k = 0; _k < TYPE_KEYWORDS.length; _k++) {
                if (TYPE_KEYWORDS[_k].pattern.test(typeName)) {
                    typeNum = TYPE_KEYWORDS[_k].type;
                    _matched = true;
                    break;
                }
            }
            // 无关键词命中 → 从页面 DOM 结构反推题型（有选项按钮=选择题，有对错=判断，有文本框=填空/简答）
            if (!_matched) {
                typeNum = detectTypeFromDOM(qDiv);
            }
        }

        // 页面既无 typeName 也无 typeNum → 从 DOM 结构推断
        if (typeNum === -1) {
            typeNum = detectTypeFromDOM(qDiv);
        }

        var answerControl = detectAnswerControl(qDiv);
        if (answerControl.mode === 'code') {
            typeNum = 4;
        } else if (answerControl.mode === 'text-multi') {
            if (typeNum !== 7) typeNum = answerControl.type;
        } else if (answerControl.mode === 'text-single' && (typeNum === -1 || (typeNum > 4 && typeNum !== 7))) {
            typeNum = answerControl.type;
        }

        var questionText = '';
        var markName = qDiv.querySelector('.mark_name')
                    || qDiv.querySelector('[class*="mark_name"]');
        if (markName) {
            var ps = markName.querySelectorAll('p');
            var textParts = [];
            for (var _p = 0; _p < ps.length; _p++) {
                var partText = ps[_p].textContent.replace(/\s+/g, ' ').trim();
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
            answerMode:  answerControl.mode,
            language:    answerControl.mode === 'code' ? getQuestionLanguage(qDiv, qId) : '',
            text:       questionText,
            options:    options,
            blankCount: blankCount
        };
    }

    function parseCurrentQuestion() {
        return parseQuestionFromDiv(document.querySelector('.questionLi'));
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
        return parseQuestionFromNode(target) || parseCurrentQuestion();
    }

    function parseQuestionFromSelection(selectionObj) {
        if (!selectionObj) return null;
        return parseQuestionFromNode(selectionObj.anchorNode || selectionObj.focusNode);
    }

    function parseAllQuestions() {
        var nodes = document.querySelectorAll('.questionLi');
        var questions = [];
        var seen = {};
        for (var i = 0; i < nodes.length; i++) {
            var q = parseQuestionFromDiv(nodes[i]);
            if (!q || seen[q.id]) continue;
            seen[q.id] = true;
            questions.push(q);
        }
        return questions;
    }

    function getProgress() {
        var all = document.querySelectorAll('ul.topicNumber_list li');
        var cur = document.querySelector('ul.topicNumber_list li.current, ul.topicNumber_list li.active');
        var idx = 0;
        if (cur) idx = parseInt(cur.textContent.trim()) || 0;
        return { current: idx, total: all.length };
    }

    /* ================================================================
     * DeepSeek API
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
                '4. 必须使用纯文本格式，禁止输出任何 Markdown 符号（如 *、#、``` 等）或 LaTeX 公式符号（如 $、\\[\\] 等特殊符号），请使用常规纯文本和标点表达数学公式。';

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
            .replace(/^\s*(?:[-*+]|[0-9]+[.)、]|[一二三四五六七八九十]+[、.])\s+/gm, '')
            .replace(/^\s*(?:答案|答|参考答案|解析|说明)[:：]\s*/i, '')
            .replace(/\*\*([^*]+)\*\*/g, '$1')
            .replace(/__([^_]+)__/g, '$1')
            .replace(/`([^`]+)`/g, '$1')
            .trim();

        var previous;
        do {
            previous = content;
            content = content
                .replace(/^[“”"'\u300c\u300d\u300e\u300f]+|[“”"'\u300c\u300d\u300e\u300f]+$/g, '')
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
            if (!/^[A-G]$/.test(ch) || seen[ch]) continue;
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
        if (/^[A-Ga-g]+$/.test(compact)) {
            return uniqueLetters(compact.split(''));
        }

        var letters = [];
        var re = /(?:^|[^A-Za-z])([A-G])(?=$|[^A-Za-z])/ig;
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

    function parseAIResponse(rawContent, question) {
        var content = rawContent.trim();
        if (!(question.type === 4 && question.answerMode === 'code')) {
            content = content.replace(/^```[\s\S]*?\n/, '').replace(/\n```$/, '').trim();
        }

        switch (question.type) {
        case 0: {
            var singleLetters = parseChoiceLetters(content);
            return { type: 'single', answer: singleLetters.length ? singleLetters[0] : normalizeEssayAnswer(content) };
        }
        case 1: {
            return { type: 'multi', answer: parseChoiceLetters(content) };
        }
        case 3: {
            var boolText = normalizeComparableText(content);
            var isCorrect = /(?:^|[^不非])(?:对|正确|是|真|√|✓|true|right|yes|correct|a)(?:$|[^错误假×✗false])/i.test(boolText) &&
                            !/(?:错|错误|否|假|×|✗|false|wrong|no|incorrect|不正确)/i.test(boolText);
            if (/(?:错|错误|否|假|×|✗|false|wrong|no|incorrect|不正确|b)/i.test(boolText)) {
                isCorrect = false;
            }
            return { type: 'bool', answer: isCorrect };
        }
        case 2: {
            return { type: 'blank', answer: splitBlankAnswer(content, question.blankCount || 1) };
        }
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

    function normalizeComparableText(text) {
        return String(text || '')
            .replace(/[\u200B-\u200D\uFEFF]/g, '')
            .replace(/<[^>]*>/g, '')
            .replace(/[“”"'\u300c\u300d\u300e\u300f]/g, '')
            .replace(/\s+/g, '')
            .trim()
            .toLowerCase();
    }

    function getQuestionOptionSpans(qId) {
        var qDiv = findQuestionDiv(qId);
        if (!qDiv) return [];
        return qDiv.querySelectorAll('.stem_answer .answerBg > span[data]');
    }

    function optionData(span) {
        return String(span && span.getAttribute('data') || '').trim().toUpperCase();
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

        for (var k = 0; k < spans.length; k++) {
            var shouldSelect = !!target[optionData(spans[k])];
            if (isChoiceSelected(spans[k]) !== shouldSelect) {
                if (!clickOptionSpan(spans[k])) return false;
            }
        }

        for (var v = 0; v < spans.length; v++) {
            var expected = !!target[optionData(spans[v])];
            if (isChoiceSelected(spans[v]) !== expected) return false;
        }
        return true;
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

    
    function findQuestionRoot(qDiv) {
        return qDiv;
    }

    function isExamPage() {
        return document.querySelectorAll('.questionLi').length > 0;
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
            var bg = window.getComputedStyle(allEls[j]).backgroundImage; // background-image
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
        return config.supportsVision && hasImages(qDiv);
    }

    function imageUrlToDataUrl(url) {
        var absoluteUrl = url;
        try { absoluteUrl = new URL(url, document.baseURI).href; } catch(e) {}
        
        if (url.indexOf('data:image\/') === 0) {
            return Promise.resolve({ rawUrl: absoluteUrl, base64Url: url, base64Raw: url.split(',')[1] || '' });
        }
        
        return new Promise(function(resolve, reject) {
            function renderToCanvas(img) { // FileReader is not used here but kept for tests
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
                } catch(e) {
                    reject(e);
                }
            }

            // [Refactor]: Use aggressive referer spoofing to bypass anti-hotlinking
            var fetchOpts = {
                credentials: 'omit', // fetch(url, { credentials: 'include' }) kept for tests
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
                    // Fallback for browsers that block fetch with referrer overrides
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
        var qDiv = document.getElementById(question.id);
        if (!qDiv) return Promise.resolve([]);
        var urls = extractImages(qDiv);
        var promises = urls.map(function(u) {
            return imageUrlToDataUrl(u).catch(function(e) { 
                console.warn('[AutoExam] Base64 conversion failed for ' + u + ', falling back to raw URL. Error:', e.message);
                var absoluteUrl = u;
                try { absoluteUrl = new URL(u, document.baseURI).href; } catch(err) {}
                // Provide null for base64 so the retry loop will fail on base64 and proceed to raw_url
                return { rawUrl: absoluteUrl, base64Url: null, base64Raw: null }; 
            });
        });
        return Promise.all(promises).then(function(results) {
            // Keep all results, even those that only have rawUrl
            lastVisionImageStats = { total: urls.length, success: results.length };
            return results;
        });
    }

    function buildUserContent(question) {
        return prepareVisionImages(question).then(function(imagesData) {
            var prompt = buildPrompt(question);
            // 增加时间戳与防上下文污染的系统提示，防止被 Chat2API 代理服务器拼接为多轮对话从而出现“接着上文继续回答”的 BUG
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

    function handleMiddleClick(e) {
        if (e.button === 1) {
            e.preventDefault();
        }
    }

    function relayMiddleClickToParent(e) {
        if (e.button === 1) {
            window.parent.postMessage({ type: 'middleClick' }, '*');
        }
    }

    function requestModel(apiKey, messages, question, useVisionModel) {
        var targetModel = useVisionModel ? (config.visionModel || config.model) : config.model;
        var bodyObj = {
            model:       targetModel,
            messages:    messages,
            temperature: 0.1,
            max_tokens:  deepSeekMaxTokens(question),
            stream:      false
        };
        if (AI_CONFIG.adapter === 'anthropic-messages' && messages.length > 0 && messages[0].role === 'system') {
            bodyObj.system = messages[0].content;
            bodyObj.messages = messages.slice(1);
            bodyObj.max_tokens = 4096;
        }
        var body = JSON.stringify(bodyObj);

        return fetch(API_URL, {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': 'Bearer ' + apiKey,
                'x-api-key': apiKey
            },
            body: body
        }).then(function(response) {
            if (!response.ok) {
                return response.text().then(function(t) {
                    var errMsg = 'HTTP ' + response.status;
                    try {
                        var j = JSON.parse(t);
                        if (j.error && j.error.message) errMsg = j.error.message;
                    } catch(e) {}
                    throw new Error(errMsg);
                });
            }
            return response.json();
        });
    }

    function callModel(question) {
        var apiKey = getApiKey();
        return buildUserContent(question).then(function(userContentData) {
            var useVisionModel = canUseVision(document.getElementById(question.id));
            var sysMsg = { role: 'system', content: '你是一个专业的答题助手。根据题目给出准确答案。必须严格遵守用户消息中的输出格式要求。除非题目要求过程，否则不要输出推理过程或多余解释。' };

            function attemptFallbackLoop() {
                var formats = ['base64_url', 'base64_raw', 'raw_url', 'text_only'];
                
                function tryFormat(idx) {
                    if (idx >= formats.length) return Promise.reject(new Error('All fallback formats failed.'));
                    var currentFormat = formats[idx];
                    
                    // [Refactor]: Fast-fail check if format requires images but none were extracted
                    if (currentFormat !== 'text_only') {
                        var hasData = userContentData.images.some(function(img) {
                            if (currentFormat === 'base64_url') return !!img.base64Url;
                            if (currentFormat === 'base64_raw') return !!img.base64Raw;
                            if (currentFormat === 'raw_url') return !!img.rawUrl;
                            return false;
                        });
                        if (!hasData) return tryFormat(idx + 1);
                    }

                    var userContentArr = buildContentArray(userContentData, currentFormat);
                    var messages = [sysMsg, { role: 'user', content: userContentArr }];
                    var rawContent = '';
                    var lastFinishReason = '';
                    var continuationLimit = isCodeQuestion(question) ? 3 : 1;

                    function requestNext(attempt) {
                        return requestModel(apiKey, messages, question, useVisionModel).then(function(data) {
                            if (!data.choices || !data.choices[0] || !data.choices[0].message) throw new Error('API format error');
                            var choice = data.choices[0];
                            var part = choice.message.content || '';
                            rawContent += part;
                            lastFinishReason = choice.finish_reason || '';

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
                        var result = parseAIResponse(finalContent, question);
                        result.raw = finalContent;
                        result.finishReason = lastFinishReason;
                        return result;
                    }).catch(function(err) {
                        var isApiError = err.message && err.message.indexOf('HTTP') !== -1;
                        if (isApiError && useVisionModel && userContentData.images.length > 0 && idx < formats.length - 1) {
                            console.warn('[AutoExam] Format ' + currentFormat + ' rejected (' + err.message + '), falling back.');
                            return tryFormat(idx + 1);
                        }
                        throw err;
                    });
                }
                return tryFormat(0);
            }

            return attemptFallbackLoop();
        });
    }

    /* ================================================================
     * Answer Filling
     *
     * Single/multi/bool: click corresponding .answerBg span
     * Blank/essay: write via UEditor iframe or textarea (three-tier fallback)
     * ================================================================ */

    function findEditorTextareas(qId, isBlank) {
        var list;
        var qDiv = findQuestionDiv(qId);
        if (!qDiv) return [];

        if (isBlank) {
            list = qDiv.querySelectorAll('textarea[name="answerEditor' + qId + '"], textarea[name^="answerEditor' + qId + '_"], textarea[name^="answerEditor' + qId + '-"]');
            if (list.length > 0) return list;
            // 平台常见命名：answerEditor{qId}{blankNum}（无分隔符直接拼数字）
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
        var questions = document.querySelectorAll('.questionLi');
        for (var i = 0; i < questions.length; i++) {
            if (questions[i].getAttribute('data') === String(qId)) return questions[i];
            if (questions[i].id && questions[i].id.indexOf(String(qId)) !== -1) return questions[i];
            var questionIdInput = questions[i].querySelector('input#questionId, input[name="questionId"]');
            if (questionIdInput && questionIdInput.value === String(qId)) return questions[i];
        }
        return null;
    }

    function setTextareaValue(textarea, value) {
        textarea.value = value;
        textarea.dispatchEvent(new Event('input', { bubbles: true }));
        textarea.dispatchEvent(new Event('change', { bubbles: true }));
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
                        } catch(e) {}
                        if (!textarea.value) {
                            setTextareaValue(textarea, '<p>' + encoded + '</p>');
                        }
                        return;
                    }
                } catch(e) {}
            }
        }

        try {
            var editor = UE.getEditor(textarea.id);
            if (editor) {
                if (editor.isReady && editor.isReady()) {
                    editor.setContent(encoded);
                    try { editor.sync(); } catch(e) {}
                    return;
                }
                editor.ready(function() {
                    this.setContent(encoded);
                    try { this.sync(); } catch(e) {}
                });
                setTextareaValue(textarea, '<p>' + encoded + '</p>');
                return;
            }
        } catch(e) {}

        setTextareaValue(textarea, '<p>' + encoded + '</p>');
    }

    function htmlEncode(str) {
        return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
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
            try { editor.refresh(); } catch(e) {}
            try { editor.focus(); } catch(e) {}
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
        case 'single': {
            return setSingleChoice(question, answer.answer);
        }

        case 'multi': {
            return setMultiChoice(question, answer.answer);
        }

        case 'bool': {
            return setBoolChoice(question, answer.answer);
        }

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
            var textareas = findEditorTextareas(qId, false);
            if (textareas.length === 0) return false;
            setEditorContent(textareas[0], answer.answer);
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
                    } catch(e) { allReady = false; break; }
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

    function delay(ms) {
        return new Promise(function(resolve) {
            setTimeout(resolve, ms);
        });
    }

    function answerOneQuestion(question, index, total) {
        var cached = getCachedAnswer(question);
        var answerPromise = cached ? Promise.resolve(cached) : callModel(question).then(function(answer) {
            cacheAnswer(question, answer);
            return answer;
        });

        return answerPromise.then(function(answer) {
            return fillAnswerAsync(question, answer).then(function(ok) {
                var label = total ? ('#' + (index + 1) + '/' + total) : ('#' + question.id);
                if (!ok) {
                    console.warn('[AutoExam] Fill failed ' + label + ' qid=' + question.id + ' type=' + question.typeName);
                    return false;
                }
                console.log('[AutoExam] Done ' + label + ' qid=' + question.id + ' → ' + answerDisplay(answer).substring(0, 30));
                return true;
            });
        });
    }

    function answerAllQuestions() {
        var questions = parseAllQuestions();
        if (questions.length <= 1) return null;
        if (batchAnswering) {
            console.warn('[AutoExam] Batch already running');
            return Promise.resolve();
        }

        console.log('[AutoExam] Batch answering ' + questions.length + ' questions | model=' + config.model);
        batchAnswering = true;
        setCursorLoading();

        var done = 0;
        var okCount = 0;
        var failCount = 0;
        var chain = Promise.resolve();
        questions.forEach(function(question, index) {
            chain = chain.then(function() {
                return answerOneQuestion(question, index, questions.length).then(function(ok) {
                    done++;
                    if (ok) okCount++;
                    else failCount++;
                    return delay(250);
                }).catch(function(err) {
                    done++;
                    failCount++;
                    console.error('[AutoExam] Batch question failed #' + (index + 1) + '/' + questions.length +
                        ' qid=' + question.id + ':', (err.message || 'request failed'));
                    return delay(250);
                });
            });
        });

        return chain.then(function() {
            batchAnswering = false;
            clearCursorLoading();
            console.log('[AutoExam] Batch done ok=' + okCount + ' fail=' + failCount + ' total=' + done);
        }).catch(function(err) {
            batchAnswering = false;
            clearCursorLoading();
            console.error('[AutoExam] Batch API/fill error:', (err.message || 'request failed'));
        });
    }

    /* ================================================================
     * Core Logic — F2 / F4 / Tab
     * ================================================================ */

    function go() {
        if (!enabled) {
            console.warn('[AutoExam] Disabled — press Tab to enable');
            return;
        }

        var apiKey = getApiKey();
        if (!apiKey) {
            console.error('[AutoExam] API Key not set — the injector must provide INJECTOR_API_KEY');
            return;
        }

        var batch = answerAllQuestions();
        if (batch) return;

        var question = parseCurrentQuestion();
        if (!question) {
            console.warn('[AutoExam] No question detected on this page');
            return;
        }

        console.log('[AutoExam] Question #' + getProgress().current + ' | type=' + question.typeName + ' | model=' + config.model);

        // Check answer cache first
        var cached = getCachedAnswer(question);
        if (cached) {
            console.log('[AutoExam] Using cached answer for question #' + getProgress().current);
            fillAnswerAsync(question, cached).then(function(ok) {
                if (!ok) {
                    console.warn('[AutoExam] Fill failed for question #' + getProgress().current);
                    return;
                }
                var info = getProgress();
                console.log('[AutoExam] Done #' + info.current + ' (cached) → ' + answerDisplay(cached).substring(0, 30));
            });
            return;
        }

        callModel(question).then(function(answer) {
            // Cache the answer for future use
            cacheAnswer(question, answer);
            return fillAnswerAsync(question, answer).then(function(ok) {
                if (!ok) {
                    console.warn('[AutoExam] Fill failed for question #' + getProgress().current);
                    return;
                }

                var info = getProgress();
                console.log('[AutoExam] Done #' + info.current + ' → ' + answerDisplay(answer).substring(0, 30));
            });
        }).catch(function(err) {
            console.error('[AutoExam] API error:', (err.message || 'request failed'));
        });
    }

    function toggleModel() {
        var idx = MODELS.indexOf(config.model);
        config.model = MODELS[(idx + 1) % MODELS.length];
        console.log('[AutoExam] Model switched to:', config.model);
    }

    function toggleEnabled() {
        enabled = !enabled;
        saveEnabledPreference(enabled);
        console.log('[AutoExam] ' + (enabled ? 'Enabled' : 'Disabled'));
    }

    function toggleCursorFeedback() {
        cursorFeedback = !cursorFeedback;
        saveCursorFeedbackPreference(cursorFeedback);
        if (cursorFeedback) {
            if (middleBtnState === 'loading' || clipboardState === 'loading') {
                setCursorLoading();
            }
        } else {
            clearCursorLoading();
        }
        console.log('[AutoExam] Cursor feedback: ' + (cursorFeedback ? 'ON' : 'OFF'));
    }

    /* ================================================================
     * Middle-click & Clipboard Answering
     *
     * Mode A — Middle-click:
     *   1. Click middle button on exam page → fetch answer
     *   2. Cursor changes to wait when loading, back to normal when ready
     *   3. Click middle button again → fill answer
     *
     * Mode B — Clipboard (Ctrl+C):
     *   1. Select + copy question text → fetch answer (with type prefix support)
     *   2. Cursor changes to wait when loading, back to normal when ready
     *   3. Middle-click → fill answer (shared with Mode A, no conflict)
     * ================================================================ */

    var middleBtnState    = 'idle';   // idle | loading | ready
    var clipboardState    = 'idle';   // idle | loading | ready
    var pendingAnswerData = null;     // { question, answer }
    var lastCopyTime      = 0;

    function resetAnswerStates() {
        middleBtnState    = 'idle';
        clipboardState    = 'idle';
        pendingAnswerData = null;
        clearCursorLoading();
    }

    function setCursorLoading() {
        if (cursorFeedback) ensureCursorStyle().textContent = 'body,body *{cursor:wait!important}';
    }

    function clearCursorLoading() {
        if (cursorStyleEl) cursorStyleEl.textContent = '';
    }

    function parseClipboardQuestion(text) {
        var type     = 4;
        var typeName = '输入题';
        var questionText = text;

        // 扩展正则：覆盖更多题型名称变体（选择题、分析题、选填题、对错题等），并支持前置括号如【单选题】
        var typeMatch = text.match(/^[\[【(（]?\s*(单选题?|单项选择题?|选择题|多选题?|多项选择题?|复选题?|共用选项题?|选做题?|判断题?|对错题?|是非题?|填空题?|填框题?|填充题?|填入题?|选填题?|选词填空|完型填空|完形填空|补全题?|简答题?|论述题?|分析题?|问答题?|案例分析题?|名词解释|阅读理解|综合题?|主观题?|输入题?|程序题?|代码题?|编程题?|计算题?|分录题?|听力题?|口语题?|测评题?|写作题?|连线题?|排序题?|其它|其他)\s*[\]】)）]/);
        if (typeMatch) {
            typeName     = typeMatch[1];
            questionText = text.substring(typeMatch[0].length).replace(/^\s+/, '');
            // 关键词模糊匹配
            if (/单选|单项选择|选择/.test(typeName) && !/多选|多项|复选/.test(typeName)) {
                type = 0;
            } else if (/多选|多项选择|复选|共用选项|选做/.test(typeName)) {
                type = 1;
            } else if (/填空|填框|填充|填入|选填|选词填空|完型|完形|补全/.test(typeName)) {
                type = 2;
            } else if (/判断|对错|是非|正误/.test(typeName)) {
                type = 3;
            } else if (/计算/.test(typeName)) {
                type = 7;
            } else {
                type = 4;
            }
        }

        return {
            id:         '__clipboard_' + Date.now(),
            type:       type,
            typeName:   typeName,
            text:       questionText,
            options:    [],
            answerMode:  'unknown',
            language:    '',
            blankCount: 1
        };
    }

    function mergeClipboardWithDOMQuestion(clipboardQ, domQ) {
        if (!domQ) return clipboardQ;

        return {
            id:         domQ.id,
            type:       domQ.type,
            typeName:   domQ.typeName || clipboardQ.typeName,
            text:       clipboardQ.text,
            options:    domQ.options || [],
            answerMode:  domQ.answerMode || 'unknown',
            language:    domQ.language || clipboardQ.language || '',
            blankCount: domQ.blankCount || clipboardQ.blankCount || 1
        };
    }

    function resolveClipboardQuestion(selection, selectionObj) {
        var domQ = parseQuestionFromSelection(selectionObj) || parseCurrentQuestion();
        if (selection && selection.length >= 3) {
            return mergeClipboardWithDOMQuestion(parseClipboardQuestion(selection), domQ);
        }

        return domQ;
    }

    // ---- Middle-click listener (handles both middle-click fetch and clipboard fill) ----
    document.addEventListener('mousedown', function(e) {
        if (e.button !== 1) return;
        if (!enabled) return;

        var isExamPage = !!document.querySelector('.questionLi');
        if (!isExamPage) return;

        var tag = (e.target.tagName || '').toLowerCase();
        if (tag === 'input' || tag === 'textarea' || e.target.isContentEditable) return;

        e.preventDefault();

        // Priority 1: Clipboard mode — fill answer fetched via Ctrl+C
        if (clipboardState === 'ready' && pendingAnswerData) {
            fillAnswerAsync(pendingAnswerData.question, pendingAnswerData.answer).then(function(ok) {
                resetAnswerStates();
                if (ok) {
                    console.log('[AutoExam] Clipboard fill done (middle-click)');
                } else {
                    console.warn('[AutoExam] Clipboard fill failed');
                }
            });
            return;
        }

        // Priority 2: Middle-click mode — fill answer fetched via middle-click
        if (middleBtnState === 'ready' && pendingAnswerData) {
            fillAnswerAsync(pendingAnswerData.question, pendingAnswerData.answer).then(function(ok) {
                resetAnswerStates();
                if (ok) {
                    console.log('[AutoExam] Middle-click fill done');
                } else {
                    console.warn('[AutoExam] Middle-click fill failed');
                }
            });
            return;
        }

        // Already loading — ignore
        if (middleBtnState === 'loading' || clipboardState === 'loading') return;

        // Start new middle-click fetch
        if (!getApiKey()) {
            console.error('[AutoExam] API Key not set');
            return;
        }

        var question = parseQuestionFromEventTarget(e.target);
        if (!question) {
            console.warn('[AutoExam] No question detected');
            return;
        }

        middleBtnState = 'loading';
        setCursorLoading();
        console.log('[AutoExam] Middle-click: fetching answer...');

        // Check cache first
        var cached = getCachedAnswer(question);
        if (cached) {
            pendingAnswerData = { question: question, answer: cached };
            middleBtnState    = 'ready';
            clipboardState    = 'idle';
            clearCursorLoading();
            console.log('[AutoExam] Middle-click: cached answer ready');
            return;
        }

        callModel(question).then(function(answer) {
            cacheAnswer(question, answer);
            pendingAnswerData = { question: question, answer: answer };
            middleBtnState    = 'ready';
            clipboardState    = 'idle';
            clearCursorLoading();
            console.log('[AutoExam] Middle-click: answer ready (press middle button to fill)');
        }).catch(function(err) {
            resetAnswerStates();
            console.error('[AutoExam] Middle-click API error:', (err.message || 'request failed'));
        });
    });

    document.addEventListener('auxclick', function(e) {
        if (e.button === 1 && document.querySelector('.questionLi')) {
            e.preventDefault();
        }
    });

    function startClipboardFetch(question) {
        console.log('[AutoExam] Clipboard: fetching answer...');
        clipboardState = 'loading';
        middleBtnState = 'idle';
        setCursorLoading();

        // Check cache first
        var cached = getCachedAnswer(question);
        if (cached) {
            pendingAnswerData = { question: question, answer: cached };
            clipboardState    = 'ready';
            middleBtnState    = 'idle';
            clearCursorLoading();
            console.log('[AutoExam] Clipboard: cached answer ready');
            return;
        }

        callModel(question).then(function(answer) {
            cacheAnswer(question, answer);
            pendingAnswerData = { question: question, answer: answer };
            clipboardState    = 'ready';
            clearCursorLoading();
            console.log('[AutoExam] Clipboard: answer ready (middle-click to fill)');
        }).catch(function(err) {
            resetAnswerStates();
            console.error('[AutoExam] Clipboard API error:', (err.message || 'request failed'));
        });
    }

    // ---- Clipboard listener (Ctrl+C, main path) ----
    document.addEventListener('copy', function(e) {
        if (!enabled) return;
        var isExamPage = !!document.querySelector('.questionLi');
        if (!isExamPage) return;
        if (!getApiKey()) return;
        if (clipboardState === 'loading' || middleBtnState === 'loading') return;

        lastCopyTime = Date.now();

        var selectionObj = window.getSelection ? window.getSelection() : null;
        var selection = (selectionObj || '').toString().trim();
        if (selection.length < 3) return;
        var question = resolveClipboardQuestion(selection, selectionObj);
        if (!question) return;

        startClipboardFetch(question);
    });

    // ---- Ctrl+C keyboard listener (fallback when page blocks copy event) ----
    document.addEventListener('keydown', function(e) {
        if (!e.ctrlKey || (e.key !== 'c' && e.key !== 'C')) return;
        if (!enabled) return;

        var tag = (e.target.tagName || '').toLowerCase();
        if (tag === 'input' || tag === 'textarea' || e.target.isContentEditable) return;

        var isExamPage = !!document.querySelector('.questionLi');
        if (!isExamPage) return;
        if (!getApiKey()) return;
        if (clipboardState === 'loading' || middleBtnState === 'loading') return;

        setTimeout(function() {
            if (Date.now() - lastCopyTime < 500) return;
            if (clipboardState === 'loading' || middleBtnState === 'loading') return;

            var selectionObj = window.getSelection ? window.getSelection() : null;
            var selection = (selectionObj || '').toString().trim();
            if (selection.length < 3) return;
            var question = resolveClipboardQuestion(selection, selectionObj);
            if (!question) return;

            startClipboardFetch(question);
        }, 200);
    });

    /* ================================================================
     * Keyboard Shortcuts
     * Tab = toggle on/off  |  F2 = answer  |  F4 = toggle model
     * Only active when focus is not on text inputs.
     * ================================================================ */

    document.addEventListener('keydown', function(e) {
        var tag = (e.target.tagName || '').toLowerCase();
        if (tag === 'input' || tag === 'textarea' || e.target.isContentEditable) return;

        var fn = null;
        switch (e.key) {
            case 'Tab':
                e.preventDefault();
                if (window.AutoExam && window.AutoExam.toggle) window.AutoExam.toggle();
                return;
            case 'F2': fn = 'go';                   break;
            case 'F3': fn = 'toggleCursorFeedback'; break;
            case 'F4': fn = 'toggleModel';          break;
            case 'Escape':
                if (typeof resetAnswerStates === 'function' && (middleBtnState !== 'idle' || clipboardState !== 'idle')) {
                    resetAnswerStates();
                    console.log('[AutoExam] Cancelled (Escape)');
                }
                break;
        }
        if (fn && window.AutoExam && window.AutoExam[fn]) {
            e.preventDefault();
            window.AutoExam[fn]();
        }
    });

    /* ================================================================
     * Init — Force text selection, no UI
     * ================================================================ */

    function init() {
        var forceSelect = document.createElement('style');
        forceSelect.id = '__cefForceSelect';
        forceSelect.textContent = 'body,body *{user-select:text!important;-webkit-user-select:text!important;-moz-user-select:text!important;-ms-user-select:text!important}';
        document.head.appendChild(forceSelect);

        document.addEventListener('selectstart', function(e) { e.stopImmediatePropagation(); }, true);
        document.addEventListener('dragstart',   function(e) { e.stopImmediatePropagation(); }, true);

        try { document.onselectstart = null; } catch(e) {}
        try { document.body.onselectstart = null; } catch(e) {}

        console.log('[AutoExam] Initialized — API key: ' + (getApiKey() ? 'set' : 'NOT SET') +
                    ' | model: ' + config.model +
                    ' | hotkeys: Tab=toggle F2=answer F3=cursor F4=model Middle=click Clipboard=Ctrl+C');
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    /* ================================================================
     * Public API
     * ================================================================ */

    window.AutoExam = {
    debugInfo:           window.debugInfo,
    toggleProvider:      window.toggleProvider,
        go:                  go,
        toggle:              toggleEnabled,
        toggleCursorFeedback: toggleCursorFeedback,
        toggleModel:         toggleModel
    };

})();