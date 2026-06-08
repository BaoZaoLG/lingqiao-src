/**
 * AutoExam.js — Embedded lightweight exam auto-answer system
 * Injected by CefHook.dll on every page load.
 *
 * Answering modes:
 *   F2           — Answer current question
 *   F3           — Toggle batch mode (auto-advance)
 *   F4           — Toggle AI model
 *   Middle-click — Fetch answer, middle-click again to fill
 *   Ctrl+C       — Copy question to fetch answer, left-click blank area to fill
 *   Escape       — Cancel pending middle-click / clipboard operation
 *
 * API Key is read from window.__DEEPSEEK_API_KEY (set by CefHook.dll from INJECTOR_API_KEY env var).
 */
(function() {
    'use strict';

    /* ================================================================
     * Constants & In-memory State
     * ================================================================ */

    var API_URL = 'https://api.deepseek.com/chat/completions';
    var MODELS   = ['deepseek-v4-pro', 'deepseek-v4-flash'];

    var config = {
        model:      MODELS[0],
        autoNext:   true,
        batchDelay: 2000
    };

    var batchMode = false;

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

    function getCachedAnswer(questionText) {
        var key = hashQuestion(questionText);
        return answerCache[key] || null;
    }

    function cacheAnswer(questionText, answer) {
        var key = hashQuestion(questionText);
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
                if (stemAnswer.querySelector('.answerBg.multi, .answerBg[class*="multi"], input[type="checkbox"]'))
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

    function parseCurrentQuestion() {
        var qDiv = document.querySelector('.questionLi');
        if (!qDiv) return null;

        var qId = qDiv.getAttribute('data');
        if (!qId) {
            var qIdInput = document.querySelector('input#questionId');
            if (qIdInput) qId = qIdInput.value;
        }
        if (!qId) {
            var tnById = document.querySelector('input[id*="typeName"]');
            if (tnById) {
                var m = tnById.id.match(/typeName(\d+)/);
                if (m) qId = m[1];
            }
        }
        if (!qId) return null;

        var typeNameInput = document.querySelector('input[name="typeName' + qId + '"]')
                         || document.querySelector('input[id*="typeName' + qId + '"]');
        var typeNumInput  = document.querySelector('input[name="type' + qId + '"]')
                         || document.querySelector('input[id*="type' + qId + '"]');
        var typeName = typeNameInput ? typeNameInput.value : '';
        var typeNum  = typeNumInput  ? parseInt(typeNumInput.value) : -1;

        if (typeNum === -1 && typeName) {
            // 关键词模糊匹配：兼容平台题型名称变体（如"选择题"→"单选题"、"分析题"→"论述题"、"选填题"→"填空题"等）
            var TYPE_KEYWORDS = [
                { pattern: /单选|单项选择|单选选择/,                              type: 0 },
                { pattern: /多选|多项选择|复选|多选选择/,                          type: 1 },
                { pattern: /填空|选填|完形填空|补全/,                              type: 2 },
                { pattern: /判断|对错|是非|正误/,                                 type: 3 },
                { pattern: /简答|论述|分析|问答|名词解释|阅读理解|案例分析|综合|输入|主观/, type: 4 }
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

        var questionText = '';
        var markName = qDiv.querySelector('.mark_name')
                    || qDiv.querySelector('[class*="mark_name"]');
        if (markName) {
            var p = markName.querySelector('p');
            if (p) questionText = p.textContent.trim();
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
            var blankInput = document.querySelector('input[name="blankNum' + qId + '"]')
                          || document.querySelector('input[id*="blankNum' + qId + '"]');
            if (blankInput) {
                blankCount = blankInput.value.split(',').filter(function(v) { return v !== ''; }).length;
            }
        }

        return {
            id:         qId,
            type:       typeNum,
            typeName:   typeName,
            text:       questionText,
            options:    options,
            blankCount: blankCount
        };
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

        case 4:
            return '【简答题】' + question.text + '\n\n请简洁、专业地回答上述问题。直接输出答案，不需要解释。';

        default:
            return question.text;
        }
    }

    function parseAIResponse(rawContent, question) {
        var content = rawContent.trim();
        content = content.replace(/^```[\s\S]*?\n/, '').replace(/\n```$/, '').trim();

        switch (question.type) {
        case 0: {
            var m = content.match(/[A-G]/i);
            return { type: 'single', answer: m ? m[0].toUpperCase() : '' };
        }
        case 1: {
            var letters = content.replace(/[^A-Ga-g]/g, '').toUpperCase().split('');
            var seen = {};
            letters = letters.filter(function(ch) {
                if (seen[ch]) return false; seen[ch] = true; return true;
            });
            return { type: 'multi', answer: letters };
        }
        case 3: {
            var isCorrect = content.indexOf('对') !== -1 ||
                           content.toLowerCase().indexOf('true') !== -1 ||
                           content.indexOf('正确') !== -1;
            return { type: 'bool', answer: isCorrect };
        }
        case 2: {
            var parts = content.split('|');
            return { type: 'blank', answer: parts.map(function(p) { return p.trim(); }) };
        }
        case 4:
            return { type: 'essay', answer: content };
        default:
            return { type: 'unknown', answer: content };
        }
    }

    function callDeepSeek(question) {
        var apiKey = getApiKey();
        var prompt = buildPrompt(question);
        var body = JSON.stringify({
            model:    config.model,
            messages: [
                { role: 'system', content: '你是一个专业的答题助手。根据题目给出准确答案。严格按照要求的格式输出，不要输出多余的解释。' },
                { role: 'user',   content: prompt }
            ],
            temperature: 0.1,
            max_tokens:  1024,
            stream:      false
        });

        return fetch(API_URL, {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': 'Bearer ' + apiKey
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
        }).then(function(data) {
            if (!data.choices || !data.choices[0] || !data.choices[0].message)
                throw new Error('API 返回格式异常');
            var rawContent = data.choices[0].message.content || '';
            var result = parseAIResponse(rawContent, question);
            result.raw = rawContent;
            return result;
        });
    }

    /* ================================================================
     * Answer Filling
     *
     * Single/multi/bool: click corresponding .answerBg span
     * Blank/essay: write via UEditor iframe or textarea (three-tier fallback)
     * ================================================================ */

    function findEditorTextareas(qId, isBlank) {
        if (isBlank) {
            var list = document.querySelectorAll('textarea[name^="answerEditor' + qId + '"]');
            if (list.length > 0) return list;
            list = document.querySelectorAll('textarea[name^="answer' + qId + '"][id*="Editor"]');
            if (list.length > 0) return list;
        } else {
            var ta = document.querySelector('textarea[name="answer' + qId + '"]');
            if (ta) return [ta];
        }
        var qDiv = document.querySelector('.questionLi');
        if (qDiv) return qDiv.querySelectorAll('.eidtDiv textarea[id*="' + qId + '"]');
        return [];
    }

    function setEditorContent(textarea, content) {
        if (!textarea) return;

        var container = textarea.closest('.eidtDiv') || textarea.closest('.subEditor') || textarea.parentElement;
        if (container) {
            var iframe = container.querySelector('iframe');
            if (iframe) {
                try {
                    var body = iframe.contentDocument && iframe.contentDocument.body;
                    if (body) {
                        body.innerHTML = '<p>' + content + '</p>';
                        body.dispatchEvent(new Event('input', { bubbles: true }));
                        try {
                            var ed = UE.getEditor(textarea.id);
                            if (ed && ed.sync) { ed.sync(); }
                        } catch(e) {}
                        if (!textarea.value) {
                            textarea.value = '<p>' + htmlEncode(content) + '</p>';
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
                    editor.setContent(content);
                    try { editor.sync(); } catch(e) {}
                    return;
                }
                editor.ready(function() {
                    this.setContent(content);
                    try { this.sync(); } catch(e) {}
                });
                textarea.value = '<p>' + htmlEncode(content) + '</p>';
                return;
            }
        } catch(e) {}

        textarea.value = '<p>' + htmlEncode(content) + '</p>';
    }

    function htmlEncode(str) {
        return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }

    function fillAnswerSync(question, answer) {
        var qId = question.id;

        switch (answer.type) {
        case 'single': {
            var label = answer.answer;
            var span = document.querySelector('.stem_answer span[data="' + label + '"][qid="' + qId + '"]');
            if (span) {
                var bg = span.closest('.answerBg');
                if (bg) { bg.click(); return true; }
            }
            var allSpans = document.querySelectorAll('.stem_answer span[data="' + label + '"]');
            for (var i = 0; i < allSpans.length; i++) {
                if (allSpans[i].getAttribute('qid') === qId) {
                    var b = allSpans[i].closest('.answerBg');
                    if (b) { b.click(); return true; }
                }
            }
            return false;
        }

        case 'multi': {
            var ok = true;
            for (var i = 0; i < answer.answer.length; i++) {
                var letter = answer.answer[i];
                var span = document.querySelector('.stem_answer span[data="' + letter + '"][qid="' + qId + '"]');
                if (span) {
                    var bg = span.closest('.answerBg');
                    if (bg) { bg.click(); continue; }
                }
                ok = false;
            }
            return ok;
        }

        case 'bool': {
            var dataVal = answer.answer ? 'true' : 'false';
            var span = document.querySelector('.stem_answer span[data="' + dataVal + '"][qid="' + qId + '"]');
            if (span) {
                var bg = span.closest('.answerBg');
                if (bg) { bg.click(); return true; }
            }
            return false;
        }

        case 'blank': {
            var textareas = findEditorTextareas(qId, true);
            if (textareas.length === 0) return false;
            for (var i = 0; i < textareas.length; i++) {
                var ans = answer.answer[i] || answer.answer[0] || '';
                setEditorContent(textareas[i], ans);
            }
            return true;
        }

        case 'essay': {
            var textareas = findEditorTextareas(qId, false);
            if (textareas.length === 0) return false;
            setEditorContent(textareas[0], answer.answer);
            return true;
        }

        default:
            return false;
        }
    }

    function fillAnswerAsync(question, answer) {
        return new Promise(function(resolve) {
            if (answer.type !== 'blank' && answer.type !== 'essay') {
                resolve(fillAnswerSync(question, answer));
                return;
            }

            var textareas = findEditorTextareas(question.id, answer.type === 'blank');
            if (textareas.length === 0) {
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

    /* ================================================================
     * Navigation
     * ================================================================ */

    function hasNextQuestion() {
        return !!document.querySelector('a.jb_btn[onclick*="getTheNextQuestion(1)"]');
    }

    function goToNextQuestion() {
        if (!hasNextQuestion()) return false;
        try {
            if (typeof getTheNextQuestion === 'function') {
                getTheNextQuestion(1); return true;
            }
        } catch(e) {}
        var btn = document.querySelector('a.jb_btn[onclick*="getTheNextQuestion(1)"]');
        if (btn) { btn.click(); return true; }
        return false;
    }

    /* ================================================================
     * Core Logic — F2 / F3 / F4
     * ================================================================ */

    function go() {
        var apiKey = getApiKey();
        if (!apiKey) {
            console.error('[AutoExam] API Key not set — the injector must provide INJECTOR_API_KEY');
            return;
        }

        var question = parseCurrentQuestion();
        if (!question) {
            console.warn('[AutoExam] No question detected on this page');
            return;
        }

        console.log('[AutoExam] Question #' + getProgress().current + ' | type=' + question.typeName + ' | model=' + config.model);

        // Check answer cache first
        var cached = getCachedAnswer(question.text);
        if (cached) {
            console.log('[AutoExam] Using cached answer for question #' + getProgress().current);
            fillAnswerAsync(question, cached).then(function(ok) {
                if (!ok) {
                    console.warn('[AutoExam] Fill failed for question #' + getProgress().current);
                    return;
                }
                var info = getProgress();
                var ansDisplay = cached.type === 'multi' ? cached.answer.join(',') :
                    (cached.type === 'blank' ? cached.answer.join(' | ') : cached.answer);
                console.log('[AutoExam] Done #' + info.current + ' (cached) → ' + String(ansDisplay).substring(0, 30));
                if (config.autoNext && hasNextQuestion() && batchMode) {
                    setTimeout(function() { if (batchMode) goToNextQuestion(); }, config.batchDelay);
                }
            });
            return;
        }

        callDeepSeek(question).then(function(answer) {
            // Cache the answer for future use
            cacheAnswer(question.text, answer);
            return fillAnswerAsync(question, answer).then(function(ok) {
                if (!ok) {
                    console.warn('[AutoExam] Fill failed for question #' + getProgress().current);
                    return;
                }

                var info = getProgress();
                var ansDisplay = answer.type === 'multi'  ? answer.answer.join(',') :
                                (answer.type === 'blank' ? answer.answer.join(' | ') : answer.answer);
                console.log('[AutoExam] Done #' + info.current + ' → ' + String(ansDisplay).substring(0, 30));

                if (config.autoNext && hasNextQuestion() && batchMode) {
                    setTimeout(function() {
                        if (batchMode) goToNextQuestion();
                    }, config.batchDelay);
                }
            });
        }).catch(function(err) {
            console.error('[AutoExam] API error:', (err.message || 'request failed'));
        });
    }

    function toggleBatch() {
        batchMode = !batchMode;
        console.log('[AutoExam] Batch mode:', batchMode ? 'ON' : 'OFF');

        if (batchMode) {
            go();
        }
    }

    function toggleModel() {
        var idx = MODELS.indexOf(config.model);
        config.model = MODELS[(idx + 1) % MODELS.length];
        console.log('[AutoExam] Model switched to:', config.model);
    }

    /* ================================================================
     * Middle-click & Clipboard Answering
     *
     * Mode A — Middle-click:
     *   1. Click middle button on exam page → fetch answer
     *   2. Cursor changes to pointer when ready
     *   3. Click middle button again → fill answer
     *
     * Mode B — Clipboard (Ctrl+C):
     *   1. Select + copy question text → fetch answer (with type prefix support)
     *   2. Cursor changes to pointer when ready
     *   3. Left-click blank area below textarea → fill answer
     * ================================================================ */

    var middleBtnState    = 'idle';   // idle | loading | ready
    var clipboardState    = 'idle';   // idle | loading | ready
    var pendingAnswerData = null;     // { question, answer }
    var lastCopyTime      = 0;

    function resetAnswerStates() {
        middleBtnState    = 'idle';
        clipboardState    = 'idle';
        pendingAnswerData = null;
        document.body.style.cursor = '';
    }

    function setCursorLoading() {
        document.body.style.cursor = 'wait';
    }

    function setCursorReady() {
        document.body.style.cursor = 'pointer';
    }

    function parseClipboardQuestion(text) {
        var type     = 4;
        var typeName = '输入题';
        var questionText = text;

        // 扩展正则：覆盖更多题型名称变体（选择题、分析题、选填题、对错题等）
        var typeMatch = text.match(/^(单选题?|单项选择题?|选择题|多选题?|多项选择题?|复选题?|判断题?|对错题?|是非题?|填空题?|选填题?|完形填空|补全题?|简答题?|论述题?|分析题?|问答题?|案例分析题?|名词解释|阅读理解|综合题?|主观题?|输入题?)\s*[\]】)]/);
        if (typeMatch) {
            typeName     = typeMatch[1];
            questionText = text.substring(typeMatch[0].length).replace(/^\s+/, '');
            // 关键词模糊匹配
            if (/单选|单项选择|选择/.test(typeName) && !/多选|多项|复选/.test(typeName)) {
                type = 0;
            } else if (/多选|多项选择|复选/.test(typeName)) {
                type = 1;
            } else if (/填空|选填|完形|补全/.test(typeName)) {
                type = 2;
            } else if (/判断|对错|是非|正误/.test(typeName)) {
                type = 3;
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
            blankCount: 1
        };
    }

    function findNearestTextareaAbove(x, y) {
        var textareas = document.querySelectorAll('textarea');
        var best      = null;
        var bestDist  = Infinity;
        for (var i = 0; i < textareas.length; i++) {
            var ta = textareas[i];
            var rect = ta.getBoundingClientRect();
            if (rect.bottom > y) continue;
            var dist = y - rect.bottom;
            if (dist < bestDist) {
                bestDist = dist;
                best     = ta;
            }
        }
        return best;
    }

    function fillClipboardSync(textarea, answer) {
        if (answer.type === 'blank') {
            var outer = textarea.closest('.eidtDiv') || textarea.closest('.subEditor');
            var group = outer ? outer.parentElement.querySelectorAll('textarea') : null;
            if (!group || group.length === 0) group = [textarea];
            for (var i = 0; i < group.length; i++) {
                var ans = answer.answer[i] || answer.answer[0] || '';
                setEditorContent(group[i], ans);
            }
            return true;
        }
        setEditorContent(textarea, answer.answer);
        return true;
    }

    function fillClipboardAsync(textarea, answer) {
        return new Promise(function(resolve) {
            var outer = textarea.closest('.eidtDiv') || textarea.closest('.subEditor') || textarea.parentElement;
            var group = (answer.type === 'blank' && outer)
                ? outer.parentElement.querySelectorAll('textarea')
                : null;
            if (!group || group.length === 0) group = [textarea];

            var start = Date.now();
            function check() {
                var allReady = true;
                for (var i = 0; i < group.length; i++) {
                    try {
                        var ed = UE.getEditor(group[i].id);
                        if (!ed || !ed.isReady || !ed.isReady()) { allReady = false; break; }
                    } catch(e) { allReady = false; break; }
                }
                if (allReady || Date.now() - start > 5000) {
                    resolve(fillClipboardSync(textarea, answer));
                    return;
                }
                setTimeout(check, 150);
            }
            check();
        });
    }

    // ---- Middle-click listener ----
    document.addEventListener('mousedown', function(e) {
        if (e.button !== 1) return;

        var isExamPage = !!document.querySelector('.questionLi');
        if (!isExamPage) return;

        var tag = (e.target.tagName || '').toLowerCase();
        if (tag === 'input' || tag === 'textarea' || e.target.isContentEditable) return;

        e.preventDefault();

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

        if (middleBtnState === 'loading') return;

        if (!getApiKey()) {
            console.error('[AutoExam] API Key not set');
            return;
        }

        var question = parseCurrentQuestion();
        if (!question) {
            console.warn('[AutoExam] No question detected');
            return;
        }

        middleBtnState = 'loading';
        setCursorLoading();
        console.log('[AutoExam] Middle-click: fetching answer...');

        // Check cache first
        var cached = getCachedAnswer(question.text);
        if (cached) {
            pendingAnswerData = { question: question, answer: cached };
            middleBtnState    = 'ready';
            clipboardState    = 'idle';
            setCursorReady();
            console.log('[AutoExam] Middle-click: cached answer ready');
            return;
        }

        callDeepSeek(question).then(function(answer) {
            cacheAnswer(question.text, answer);
            pendingAnswerData = { question: question, answer: answer };
            middleBtnState    = 'ready';
            clipboardState    = 'idle';
            setCursorReady();
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
        var cached = getCachedAnswer(question.text);
        if (cached) {
            pendingAnswerData = { question: question, answer: cached };
            clipboardState    = 'ready';
            middleBtnState    = 'idle';
            setCursorReady();
            console.log('[AutoExam] Clipboard: cached answer ready');
            return;
        }

        callDeepSeek(question).then(function(answer) {
            cacheAnswer(question.text, answer);
            pendingAnswerData = { question: question, answer: answer };
            clipboardState    = 'ready';
            setCursorReady();
            console.log('[AutoExam] Clipboard: answer ready (left-click blank area to fill)');
        }).catch(function(err) {
            resetAnswerStates();
            console.error('[AutoExam] Clipboard API error:', (err.message || 'request failed'));
        });
    }

    // ---- Clipboard listener (Ctrl+C, main path) ----
    document.addEventListener('copy', function(e) {
        var isExamPage = !!document.querySelector('.questionLi');
        if (!isExamPage) return;
        if (!getApiKey()) return;
        if (clipboardState === 'loading' || middleBtnState === 'loading') return;

        lastCopyTime = Date.now();

        var selection = (window.getSelection() || '').toString().trim();
        var question;

        if (selection && selection.length >= 3) {
            question = parseClipboardQuestion(selection);
            if ((question.type === 0 || question.type === 1 || question.type === 3) && question.options.length === 0) {
                var domQ = parseCurrentQuestion();
                if (domQ && domQ.options.length > 0) {
                    question.options = domQ.options;
                }
            }
        } else {
            question = parseCurrentQuestion();
            if (!question) return;
        }

        startClipboardFetch(question);
    });

    // ---- Ctrl+C keyboard listener (fallback when page blocks copy event) ----
    document.addEventListener('keydown', function(e) {
        if (!e.ctrlKey || (e.key !== 'c' && e.key !== 'C')) return;

        var tag = (e.target.tagName || '').toLowerCase();
        if (tag === 'input' || tag === 'textarea' || e.target.isContentEditable) return;

        var isExamPage = !!document.querySelector('.questionLi');
        if (!isExamPage) return;
        if (!getApiKey()) return;
        if (clipboardState === 'loading' || middleBtnState === 'loading') return;

        var selection = (window.getSelection() || '').toString().trim();
        if (selection && selection.length >= 3) return;

        setTimeout(function() {
            if (Date.now() - lastCopyTime < 500) return;
            if (clipboardState === 'loading' || middleBtnState === 'loading') return;

            var question = parseCurrentQuestion();
            if (!question) return;

            startClipboardFetch(question);
        }, 200);
    });

    // ---- Left-click listener (clipboard mode fill) ----
    document.addEventListener('click', function(e) {
        if (e.button !== 0) return;
        if (clipboardState !== 'ready') return;
        if (!pendingAnswerData) return;

        var tag = (e.target.tagName || '').toLowerCase();
        if (tag === 'input' || tag === 'textarea' || e.target.isContentEditable) return;
        if (tag === 'button' || tag === 'a') return;
        if (e.target.closest('button') || e.target.closest('a')) return;

        var textarea = findNearestTextareaAbove(e.clientX, e.clientY);
        if (!textarea) {
            console.warn('[AutoExam] Clipboard: no textarea found above click point');
            return;
        }

        e.preventDefault();
        e.stopPropagation();

        var question = pendingAnswerData.question;
        var answer   = pendingAnswerData.answer;

        fillClipboardAsync(textarea, answer).then(function(ok) {
            resetAnswerStates();
            if (ok) {
                console.log('[AutoExam] Clipboard fill done');
            } else {
                console.warn('[AutoExam] Clipboard fill failed');
            }
        });
    });

    /* ================================================================
     * Keyboard Shortcuts
     * F2 = answer  |  F3 = batch  |  F4 = toggle model
     * Only active when focus is not on text inputs.
     * ================================================================ */

    document.addEventListener('keydown', function(e) {
        var tag = (e.target.tagName || '').toLowerCase();
        if (tag === 'input' || tag === 'textarea' || e.target.isContentEditable) return;

        var fn = null;
        switch (e.key) {
            case 'F2': fn = 'go';          break;
            case 'F3': fn = 'toggleBatch'; break;
            case 'F4': fn = 'toggleModel'; break;
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
                    ' | hotkeys: F2=answer F3=batch F4=model Middle=click Clipboard=Ctrl+C');
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
        go:           go,
        toggleBatch:  toggleBatch,
        toggleModel:  toggleModel
    };

})();
