/* ============================================================================
 * bridge_runtime.cpp — Bridge lifecycle, JSON delivery, and state management
 * ========================================================================= */

#include "pch.h"
#include "bridge_runtime.h"
#include "bridge_json.h"
#include "bridge_protocol.h"
#include "cef_ref.h"
#include "hook_engine.h"
#include "include/capi/cef_browser_capi.h"
#include "include/capi/cef_task_capi.h"
#include <vector>
#include <new>

/* Forward declaration from thread.cpp */
typedef int(CEF_EXPORT* cef_string_from_ptr_t)(const TCHAR*, size_t, cef_string_utf16_t*);
typedef void(CEF_EXPORT* cef_string_clear_t)(cef_string_utf16_t*);
extern cef_string_from_ptr_t func_cef_string_from_ptr;
extern cef_string_clear_t    func_cef_string_clear;

/* ---- State ---- */

static BridgeTransport g_transport = BRIDGE_TRANSPORT_CONSOLE;
static _cef_frame_t*   g_bridgeMainFrame = NULL;
static BridgeHookState g_hookState = { false, false, false, false, false };
static LONG            g_scriptInjected = 0;
static LONG            g_dllLoaded = 0;

/* Thread-safe main frame accessors */
static inline void SetMainFrameSafe(_cef_frame_t* frame) {
    InterlockedExchangePointer((volatile PVOID*)&g_bridgeMainFrame, frame);
}
static inline _cef_frame_t* GetMainFrameSafe() {
    return (_cef_frame_t*)InterlockedCompareExchangePointer(
        (volatile PVOID*)&g_bridgeMainFrame, NULL, NULL);
}

/* ---- Init / Shutdown ---- */

void BridgeRuntimeInit() {
    OutputDebugStringA("[BRIDGE] Runtime initialized\n");
    CefReleaseSlot((void* volatile*)&g_bridgeMainFrame);
    g_transport = BRIDGE_TRANSPORT_CONSOLE;
    g_hookState = { false, false, false, false, false };
    g_scriptInjected = 0;
    g_dllLoaded = 1;
}

void BridgeRuntimeShutdown() {
    CefReleaseSlot((void* volatile*)&g_bridgeMainFrame);
    g_dllLoaded = 0;
    OutputDebugStringA("[BRIDGE] Runtime shutdown\n");
}

/* ---- Transport ---- */

void BridgeSetTransport(BridgeTransport transport) {
    g_transport = transport;
    char dbg[64];
    sprintf_s(dbg, "[BRIDGE] Transport set to: %s\n", BridgeGetTransportName());
    OutputDebugStringA(dbg);
}

BridgeTransport BridgeGetTransport() {
    return g_transport;
}

const char* BridgeGetTransportName() {
    switch (g_transport) {
    case BRIDGE_TRANSPORT_V8:              return "v8";
    case BRIDGE_TRANSPORT_PROCESS_MESSAGE:  return "process-message";
    case BRIDGE_TRANSPORT_CONSOLE:
    default:                               return "console";
    }
}

/* ---- Frame management (ref-counted) ---- */

void BridgeStoreMainFrame(_cef_frame_t* frame) {
    if (!frame) return;
    if (frame == g_bridgeMainFrame) return; /* same pointer — no-op */
    CefAddRef(&frame->base);
    void* old = InterlockedExchangePointer((volatile PVOID*)&g_bridgeMainFrame, frame);
    CefRelease((cef_base_ref_counted_t*)old);
}

void BridgeReleaseMainFrame() {
    CefReleaseSlot((void* volatile*)&g_bridgeMainFrame);
}

_cef_frame_t* BridgeGetMainFrame() {
    return GetMainFrameSafe();
}

/* ---- Hook state ---- */

BridgeHookState& BridgeGetHookState() {
    return g_hookState;
}

/* ---- Script injection tracking ---- */

void BridgeSetScriptInjected(bool injected) {
    InterlockedExchange(&g_scriptInjected, injected ? 1 : 0);
}

bool BridgeIsScriptInjected() {
    return InterlockedCompareExchange(&g_scriptInjected, 0, 0) != 0;
}

/* ---- DLL loaded state ---- */

void BridgeSetDllLoaded(bool loaded) {
    InterlockedExchange(&g_dllLoaded, loaded ? 1 : 0);
}

bool BridgeIsDllLoaded() {
    return InterlockedCompareExchange(&g_dllLoaded, 0, 0) != 0;
}

/* SEH-isolated execute_java_script call — no C++ objects allowed in this function */
static int ExecuteJsSafe(_cef_frame_t* frame, cef_string_t* eval, cef_string_t* url) {
    __try {
        frame->execute_java_script(frame, eval, url, 0);
        return 1;
    }
    __except (EXCEPTION_EXECUTE_HANDLER) {
        return 0;
    }
}

/* ---- JSON delivery (DLL -> JS) ---- */

bool BridgeDeliverJson(_cef_frame_t* frame, const std::string& json) {
    if (!frame || json.empty()) return false;
    if (!func_cef_string_from_ptr) return false;

    /* Escape JSON as a single-quoted JS string and JSON.parse it.
     * Embedding the raw object literal can break on U+2028/2029 or
     * content that looks like script terminators. */
    std::string escaped;
    escaped.reserve(json.size() + 32);
    for (unsigned char c : json) {
        switch (c) {
        case '\\': escaped += "\\\\"; break;
        case '\'': escaped += "\\'"; break;
        case '\n': escaped += "\\n"; break;
        case '\r': escaped += "\\r"; break;
        case '\t': escaped += "\\t"; break;
        default:
            if (c < 0x20) {
                char b[8];
                sprintf_s(b, "\\u%04x", c);
                escaped += b;
            } else {
                escaped += (char)c;
            }
        }
    }
    /* Neutralize U+2028/U+2029 (valid JSON, illegal in JS string literals). */
    {
        size_t p = 0;
        const char* LS = "\xE2\x80\xA8";
        const char* PS = "\xE2\x80\xA9";
        while ((p = escaped.find(LS, p)) != std::string::npos) {
            escaped.replace(p, 3, "\\u2028");
            p += 6;
        }
        p = 0;
        while ((p = escaped.find(PS, p)) != std::string::npos) {
            escaped.replace(p, 3, "\\u2029");
            p += 6;
        }
    }

    std::string jsCode;
    jsCode.reserve(escaped.size() + 160);
    jsCode += "(function(){try{";
    jsCode += "if(window.LingQiaoBridge&&window.LingQiaoBridge.__deliver){";
    jsCode += "window.LingQiaoBridge.__deliver(JSON.parse('";
    jsCode += escaped;
    jsCode += "'));}}catch(e){}})();";

    int wideLen = MultiByteToWideChar(CP_UTF8, 0, jsCode.data(), (int)jsCode.size(), NULL, 0);
    if (wideLen <= 0) return false;

    WCHAR* wideBuf = new WCHAR[wideLen + 1];
    MultiByteToWideChar(CP_UTF8, 0, jsCode.data(), (int)jsCode.size(), wideBuf, wideLen);
    wideBuf[wideLen] = L'\0';

    cef_string_t eval = { 0 }, url = { 0 };
    func_cef_string_from_ptr(wideBuf, wideLen, &eval);
    func_cef_string_from_ptr(L"", 0, &url);

    bool ok = ExecuteJsSafe(frame, &eval, &url) != 0;

    if (func_cef_string_clear) {
        func_cef_string_clear(&eval);
        func_cef_string_clear(&url);
    }
    delete[] wideBuf;

    return ok;
}

bool BridgeDeliverResponse(_cef_frame_t* frame, const std::string& id,
                           const std::string& resultJson) {
    return BridgeDeliverJson(frame, BridgeMakeSuccess(id, resultJson));
}

bool BridgeDeliverEvent(_cef_frame_t* frame, const std::string& eventName,
                        const std::string& payloadJson) {
    return BridgeDeliverJson(frame, BridgeMakeEvent(eventName, payloadJson));
}

/* ---- CEF UI-thread helpers ---- */

typedef int (CEF_EXPORT* pfn_cef_post_task)(cef_thread_id_t threadId, cef_task_t* task);
typedef int (CEF_EXPORT* pfn_cef_currently_on)(cef_thread_id_t threadId);

static pfn_cef_post_task     g_cef_post_task = NULL;
static pfn_cef_currently_on  g_cef_currently_on = NULL;

static void ResolveTaskApis() {
    static LONG once = 0;
    if (InterlockedCompareExchange(&once, 1, 0) != 0) return;
    g_cef_post_task = (pfn_cef_post_task)HookFindFunction("libcef.dll", "cef_post_task");
    g_cef_currently_on = (pfn_cef_currently_on)HookFindFunction("libcef.dll", "cef_currently_on");
    char dbg[128];
    sprintf_s(dbg, "[BRIDGE] task APIs post=%p on=%p\n",
              (void*)g_cef_post_task, (void*)g_cef_currently_on);
    OutputDebugStringA(dbg);
}

bool BridgeIsUiThread() {
    ResolveTaskApis();
    if (g_cef_currently_on) {
        return g_cef_currently_on(TID_UI) != 0;
    }
    return false;
}

struct BridgeUiTask {
    cef_task_t task;
    volatile LONG refCount;
    void (*fn)(void*);
    void* ctx;
};

static void CEF_CALLBACK BridgeUiTaskAddRef(cef_base_ref_counted_t* self) {
    BridgeUiTask* t = (BridgeUiTask*)self;
    InterlockedIncrement(&t->refCount);
}

static int CEF_CALLBACK BridgeUiTaskRelease(cef_base_ref_counted_t* self) {
    BridgeUiTask* t = (BridgeUiTask*)self;
    if (InterlockedDecrement(&t->refCount) == 0) {
        delete t;
        return 1;
    }
    return 0;
}

static int CEF_CALLBACK BridgeUiTaskHasOneRef(cef_base_ref_counted_t* self) {
    BridgeUiTask* t = (BridgeUiTask*)self;
    return InterlockedCompareExchange(&t->refCount, 0, 0) == 1 ? 1 : 0;
}

static int CEF_CALLBACK BridgeUiTaskHasAtLeastOneRef(cef_base_ref_counted_t* self) {
    BridgeUiTask* t = (BridgeUiTask*)self;
    return InterlockedCompareExchange(&t->refCount, 0, 0) >= 1 ? 1 : 0;
}

static void CEF_CALLBACK BridgeUiTaskExecute(cef_task_t* self) {
    BridgeUiTask* t = (BridgeUiTask*)self;
    if (t->fn) t->fn(t->ctx);
}

bool BridgePostToUiThread(void (*fn)(void*), void* ctx) {
    if (!fn) return false;
    ResolveTaskApis();
    if (!g_cef_post_task) return false;

    BridgeUiTask* t = new (std::nothrow) BridgeUiTask();
    if (!t) return false;
    t->refCount = 1;
    t->fn = fn;
    t->ctx = ctx;
    t->task.base.size = sizeof(cef_task_t);
    t->task.base.add_ref = BridgeUiTaskAddRef;
    t->task.base.release = BridgeUiTaskRelease;
    t->task.base.has_one_ref = BridgeUiTaskHasOneRef;
    t->task.base.has_at_least_one_ref = BridgeUiTaskHasAtLeastOneRef;
    t->task.execute = BridgeUiTaskExecute;

    if (!g_cef_post_task(TID_UI, &t->task)) {
        t->task.base.release(&t->task.base);
        OutputDebugStringA("[BRIDGE] cef_post_task failed\n");
        return false;
    }
    return true;
}

static void DeliverAllFramesOnCurrentThread(struct _cef_browser_t* browser, const std::string& json) {
    if (!browser || json.empty()) return;
    int delivered = 0;
    if (browser->get_frame_identifiers && browser->get_frame_byident) {
        size_t count = browser->get_frame_count ? browser->get_frame_count(browser) : 0;
        if (count == 0) count = 32;
        if (count > 256) count = 256;
        std::vector<int64> ids(count);
        size_t n = count;
        browser->get_frame_identifiers(browser, &n, ids.data());
        for (size_t i = 0; i < n; ++i) {
            _cef_frame_t* f = browser->get_frame_byident(browser, ids[i]);
            if (!f) continue;
            if (BridgeDeliverJson(f, json)) ++delivered;
            CefRelease(&f->base);
        }
    }
    /* Always hit main frame — exam UI often lives in a child frame that
     * get_frame_identifiers misses; main still helps shell chrome. */
    if (browser->get_main_frame) {
        _cef_frame_t* main = browser->get_main_frame(browser);
        if (main) {
            BridgeStoreMainFrame(main);
            if (BridgeDeliverJson(main, json)) ++delivered;
            CefRelease(&main->base);
        }
    }
    char dbg[96];
    sprintf_s(dbg, "[BRIDGE] deliver frames ok=%d bytes=%zu\n", delivered, json.size());
    OutputDebugStringA(dbg);
}

struct DeliverCtx {
    struct _cef_browser_t* browser;
    std::string json;
};

static void DeliverCtxRun(void* raw) {
    DeliverCtx* ctx = (DeliverCtx*)raw;
    if (ctx) {
        DeliverAllFramesOnCurrentThread(ctx->browser, ctx->json);
        if (ctx->browser && ctx->browser->base.release)
            ctx->browser->base.release(&ctx->browser->base);
        delete ctx;
    }
}

void BridgeDeliverToAllFramesSafe(struct _cef_browser_t* browser, const std::string& json) {
    if (!browser || json.empty()) return;
    if (BridgeIsUiThread()) {
        DeliverAllFramesOnCurrentThread(browser, json);
        return;
    }
    DeliverCtx* ctx = new (std::nothrow) DeliverCtx();
    if (!ctx) {
        DeliverAllFramesOnCurrentThread(browser, json);
        return;
    }
    ctx->browser = browser;
    ctx->json = json;
    CefAddRef(&browser->base);
    if (!BridgePostToUiThread(DeliverCtxRun, ctx)) {
        OutputDebugStringA("[BRIDGE] UI post failed — delivering inline (may be unsafe)\n");
        DeliverCtxRun(ctx);
    }
}
