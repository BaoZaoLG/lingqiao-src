static const char* BRIDGE_BOOTSTRAP_JS =
/* MSVC rejects any single string literal larger than 16380 bytes (C2026) and
 * this payload is well past that. Adjacent literals concatenate at compile
 * time, so the bootstrap is emitted as three chunks. Splitting at newline
 * boundaries keeps the reconstructed JS byte-identical to the source. */
R"LQBRIDGE(
(function() {
    'use strict';

    /* Guard: don't inject twice */
    if (window.LingQiaoBridge && window.LingQiaoBridge.ready) return;

    var _bridgeVersion = '2.0';
    var _pendingRequests = {};
    var _eventListeners = {};
    /* A native middle-click can arrive during page-script evaluation. Keep
     * that one-shot event until AutoExam registers its listener. */
    var _queuedNativeTriggers = [];
    var _requestCounter = 0;
    var _defaultTimeout = 5000;
    /* Capture console.info immediately — page scripts (e.g. AutoExam) may
     * overwrite console.info later; console transport must keep working. */
    var _nativeConsoleInfo = (console && console.info) ? console.info.bind(console) : null;

    /* ---- Transport detection ---- */
    function detectTransport() {
        if (window.__LingQiaoNative && typeof window.__LingQiaoNative.request === 'function') {
            return 'v8';
        }
        return 'console';
    }

    /* ---- Send via current transport ---- */
    function sendViaTransport(envelope) {
        var transport = detectTransport();
        /* ai.chatComplete does network I/O. V8 transport is synchronous and
         * would freeze the renderer (watchdog/cursor reset never run). Force
         * console transport so the C++ side can dispatch off-thread. */
        if (envelope && envelope.method === 'ai.chatComplete') {
            transport = 'console';
        }
        var jsonStr = JSON.stringify(envelope);

        if (transport === 'v8') {
            try {
                var response = window.__LingQiaoNative.request(jsonStr);
                if (response) {
                    try {
                        var parsed = typeof response === 'string' ? JSON.parse(response) : response;
                        deliverInternal(parsed);
                    } catch(e) {}
                }
                return true;
            } catch(e) {
                /* V8 failed — fall through to console */
            }
        }

        /* Console transport: send via console.info with prefix */
        if (_nativeConsoleInfo) {
            _nativeConsoleInfo('__LQ_BRIDGE_V2__' + jsonStr);
            return true;
        } else if (console && console.info) {
            console.info('__LQ_BRIDGE_V2__' + jsonStr);
            return true;
        }
        return false;
    }

    /* ---- Internal delivery (DLL -> JS) ---- */
    function deliverInternal(message) {
        if (!message || typeof message !== 'object') return;

        if (message.type === 'response') {
            var id = message.id;
            if (id && _pendingRequests[id]) {
                var pending = _pendingRequests[id];
                delete _pendingRequests[id];
                clearTimeout(pending.timer);
                if (message.ok) {
                    pending.resolve(message.result);
                } else {
                    var err = message.error || { code: 'UNKNOWN', message: 'Unknown error' };
                    pending.reject(err);
                }
            }
            return;
        }

        if (message.type === 'event') {
            var eventName = message.method;
            if (eventName && _eventListeners[eventName]) {
                var listeners = _eventListeners[eventName].slice();
                for (var i = 0; i < listeners.length; i++) {
                    try { listeners[i](message.params); } catch(e) {}
                }
            } else if (eventName === 'nativeTrigger') {
                if (_queuedNativeTriggers.length >= 4) _queuedNativeTriggers.shift();
                _queuedNativeTriggers.push(message.params || {});
            }
            return;
        }

        /* Unknown type — log for debugging */
        try { console.debug('[LingQiaoBridge] Unknown message type:', message.type); } catch(e) {}
    }

    /* ---- Public API ---- */
    var bridge = {
        version: _bridgeVersion,
        ready: true,

        /* Transport will be detected dynamically on each request */
        get transport() { return detectTransport(); },

        /**
         * Send an RPC request to the DLL side.
         * @param {string} method - Method name (e.g. 'runtime.ping')
         * @param {object} [params] - Parameters object
         * @param {object} [options] - Options: { timeout: number }
         * @returns {Promise<any>} Resolves with result, rejects with error object
         */
        request: function(method, params, options) {
            var timeout = (options && options.timeout) || _defaultTimeout;
            var id = 'rpc_' + Date.now() + '_' + (++_requestCounter);

            var envelope = {
                v: 2,
                id: id,
                type: 'request',
                method: method,
                params: params || {},
                ts: Date.now()
            };

            return new Promise(function(resolve, reject) {
                var timer = setTimeout(function() {
                    delete _pendingRequests[id];
                    reject({ code: 'TIMEOUT', message: 'Bridge request timed out after ' + timeout + 'ms' });
                }, timeout);

                _pendingRequests[id] = {
                    resolve: resolve,
                    reject: reject,
                    timer: timer
                };

                sendViaTransport(envelope);
            });
        },

        /* Internal raw-envelope sender for long-running requests that use a
         * mailbox/poll response path. Share the captured native console
         * function above instead of the mutable page console.
         */
        __send: function(envelope) {
)LQBRIDGE"
R"LQBRIDGE(            if (!envelope || typeof envelope !== 'object') return false;
            try { return !!sendViaTransport(envelope); } catch (e) { return false; }
        },

        /**
         * Register an event listener.
         * @param {string} event - Event name (e.g. 'script.updated')
         * @param {function} handler - Callback function(params)
         */
        on: function(event, handler) {
            if (typeof handler !== 'function') return;
            if (!_eventListeners[event]) _eventListeners[event] = [];
            _eventListeners[event].push(handler);
            if (event === 'nativeTrigger' && _queuedNativeTriggers.length) {
                var queued = _queuedNativeTriggers.slice();
                _queuedNativeTriggers.length = 0;
                for (var qi = 0; qi < queued.length; qi++) {
                    try { handler(queued[qi]); } catch (e) {}
                }
            }
        },

        /**
         * Remove an event listener.
         * @param {string} event - Event name
         * @param {function} handler - The handler to remove
         */
        off: function(event, handler) {
            if (!_eventListeners[event]) return;
            var listeners = _eventListeners[event];
            for (var i = listeners.length - 1; i >= 0; i--) {
                if (listeners[i] === handler) {
                    listeners.splice(i, 1);
                }
            }
        },

        /**
         * Register a one-time event listener.
         * @param {string} event - Event name
         * @param {function} handler - Callback (called once, then removed)
         */
        once: function(event, handler) {
            if (typeof handler !== 'function') return;
            var wrappedHandler = function(params) {
                bridge.off(event, wrappedHandler);
                handler(params);
            };
            bridge.on(event, wrappedHandler);
        },

        /**
         * Internal delivery endpoint — called by DLL via execute_java_script.
         * DO NOT call directly from user code.
         * @param {object} message - The message envelope from DLL
         */
        __deliver: deliverInternal
    };

    /* Install the bridge object on window */
    Object.defineProperty(window, 'LingQiaoBridge', {
        value: bridge,
        writable: false,
        configurable: false,
        enumerable: true
    });

    try { console.debug('[LingQiaoBridge] v' + _bridgeVersion + ' initialized'); } catch(e) {}

    /* ---- Cross-frame AI mailbox routing ----
     * Only the main frame gets __LingQiaoNative (the DLL refuses subframes), so
     * only the main frame can poll ai.pollResult. A subframe must therefore
     * register its request id with the top frame, and results must be forwarded
     * deeper than one level. Without this a subframe sends the request, the DLL
     * answers and stores it, and the answer is never collected — the page sits
     * in its wait state until the watchdog fires. */
    var LQ_AI_ID_RE = /^ai_\d+_\d+$/;
    var LQ_MAX_HOP = 4;
    var LQ_AI_RESULT_TTL_MS = 120000;

    function lqPostToTop(msg) {
        try {
            if (window.top && window.top !== window) window.top.postMessage(msg, '*');
        } catch (e) {}
    }

    /* Non-relay frames (i.e. every subframe) get a forwarding registrar; the
     * main-frame relay overwrites these with its local table once it starts. */
    if (typeof window.__LQ_REGISTER_AI_POLL !== 'function') {
        window.__LQ_REGISTER_AI_POLL = function(id) {
            if (typeof id === 'string' && LQ_AI_ID_RE.test(id)) {
                lqPostToTop({ type: 'lqAiRegister', id: id });
            }
        };
    }
    if (typeof window.__LQ_CANCEL_AI_POLL !== 'function') {
        window.__LQ_CANCEL_AI_POLL = function(id) {
            if (typeof id === 'string' && LQ_AI_ID_RE.test(id)) {
                lqPostToTop({ type: 'lqAiCancel', id: id });
            }
        };
    }

    window.addEventListener('message', function(ev) {
        var d = ev && ev.data;
        if (!d || typeof d !== 'object') return;

        /* The relay frame is the only one holding the id table. */
        if (d.type === 'lqAiRegister') {
            if (window.__LQ_V8_RELAY && typeof window.__LQ_REGISTER_AI_POLL === 'function') {
                window.__LQ_REGISTER_AI_POLL(d.id);
            }
            return;
        }
        if (d.type === 'lqAiCancel') {
            if (window.__LQ_V8_RELAY && typeof window.__LQ_CANCEL_AI_POLL === 'function') {
                window.__LQ_CANCEL_AI_POLL(d.id);
            }
            return;
        }

        /* Walk the frame tree one hop at a time so nested frames receive
         * results/triggers. Bounded so a misbehaving frame cannot loop. */
        if ((d.type === 'lqAiResult' || d.type === 'lqNativeTrigger') &&
            (d.__lqHop || 0) < LQ_MAX_HOP) {
            var fwd = {
                type: d.type,
                id: d.id,
                result: d.result,
                source: d.source,
                __lqHop: (d.__lqHop || 0) + 1
            };
            for (var i = 0; i < window.frames.length; i++) {
                try { window.frames[i].postMessage(fwd, '*'); } catch (e) {}
            }
        }
    }, false);

    /* Main-frame V8 poll relay.
     * execute_java_script from the DLL often never runs on Chaoxing frames.
     * The main frame has __LingQiaoNative (sync V8). Poll fast mailbox
     * methods there and postMessage the results into every frame.
     * Bootstrap may run BEFORE the native object is installed — retry. */
    (function setupV8PollRelay() {
)LQBRIDGE"
R"LQBRIDGE(        if (window.__LQ_V8_RELAY) return;
        var aiIds = {};
        var seq = 0;
        var started = false;

        function broadcast(msg) {
            /* Stamp the hop counter and post to THIS frame only. The message
             * listener above walks the frame tree one hop at a time, which
             * reaches nested frames exactly once (posting to children here as
             * well would deliver duplicates). */
            if (msg && typeof msg === 'object' && msg.__lqHop === undefined) {
                msg.__lqHop = 0;
            }
            try { window.postMessage(msg, '*'); } catch (e) {}
        }

        function v8req(method, params) {
            var env = {
                v: 2,
                id: 'v8p_' + Date.now() + '_' + (++seq),
                type: 'request',
                method: method,
                params: params || {},
                ts: Date.now()
            };
            try {
                var raw = window.__LingQiaoNative.request(JSON.stringify(env));
                if (!raw) return null;
                return JSON.parse(raw);
            } catch (e) {
                return null;
            }
        }

        function startIfReady() {
            if (started) return true;
            if (!window.__LingQiaoNative || typeof window.__LingQiaoNative.request !== 'function') {
                return false;
            }
            started = true;
            window.__LQ_V8_RELAY = true;
            window.__LQ_REGISTER_AI_POLL = function(id) {
                if (typeof id !== 'string' || !LQ_AI_ID_RE.test(id)) return;
                aiIds[id] = Date.now();
            };
            window.__LQ_CANCEL_AI_POLL = function(id) {
                if (id) delete aiIds[id];
            };
            setInterval(function() {
                try {
                    /* Do not consume the one-shot native trigger until the
                     * page-side AutoExam listener exists.  On Chaoxing the
                     * question DOM can be present before the remote script
                     * finishes evaluating; consuming here would drop the
                     * middle-click event and leave the browser in its native
                     * middle-button auto-scroll state with no AI request. */
                    var autoExamReady = !!(window.AutoExam &&
                                           typeof window.AutoExam.trigger === 'function');
                    var hasQ = autoExamReady && !!(document.querySelector('.questionLi') ||
                                  document.querySelector('.exam-question') ||
                                  document.querySelector('.mark_name'));
                    var t = v8req('native.pollTrigger', { ready: hasQ ? 1 : 0 });
                    if (t && t.ok && t.result && t.result.pending) {
                        broadcast({ type: 'lqNativeTrigger', source: t.result.source || 'poll-MBUTTON' });
                    }
                    var nowTs = Date.now();
                    for (var id in aiIds) {
                        if (!Object.prototype.hasOwnProperty.call(aiIds, id)) continue;
                        /* A registration whose result never arrives must not
                         * leak forever — surface it as an explicit timeout
                         * instead of polling a dead id indefinitely. */
                        if (nowTs - aiIds[id] > LQ_AI_RESULT_TTL_MS) {
                            delete aiIds[id];
                            broadcast({ type: 'lqAiResult', id: id, result: { error: { code: 'TIMEOUT', message: 'AI result never arrived' } } });
                            continue;
                        }
                        var r = v8req('ai.pollResult', { id: id });
                        if (r && r.ok && r.result && !r.result.pending) {
                            delete aiIds[id];
                            broadcast({ type: 'lqAiResult', id: id, result: r.result });
                        } else if (r && r.ok === false) {
                            delete aiIds[id];
                            broadcast({ type: 'lqAiResult', id: id, result: { error: r.error || { code: 'FAIL', message: 'ai failed' } } });
                        }
                    }
                } catch (e) {}
            }, 250);
            try { console.warn('[LingQiaoBridge] V8 poll relay installed'); } catch (e) {}
            return true;
        }

        /* The native object can be installed well after the document loads, so
         * keep retrying — and let the page reopen the window on demand via the
         * lqWantV8Relay event. This bootstrap owns the ONLY relay
         * implementation; page code must not keep a second copy of the id
         * table, which is how the two copies previously drifted apart. */
        var tries = 0;
        var iv = null;

        function pollUntilReady() {
            if (window.__LQ_V8_RELAY) {
                if (iv) { clearInterval(iv); iv = null; }
                return;
            }
            tries++;
            if (startIfReady() || tries > 80) {
                if (iv) { clearInterval(iv); iv = null; }
            }
        }

        if (startIfReady()) return;

        iv = setInterval(pollUntilReady, 250);

        try {
            window.addEventListener('lqWantV8Relay', function() {
                if (window.__LQ_V8_RELAY) return;
                tries = 0; /* reopen the retry window */
                if (!iv) iv = setInterval(pollUntilReady, 250);
                pollUntilReady();
            }, false);
        } catch (e) {}
    })();
})();
)LQBRIDGE";
