#include "pch.h"

#include <string>
#include <thread>
#include <vector>
#include <new>

#include "hook_engine.h"
#include "hook_safety.h"
#include "process_identity.h"
#include "script_loader.h"
#include "v8_hooks.h"
#include "v8_signatures.h"
#include "bridge_protocol.h"
#include "bridge_json.h"
#include "bridge_dispatcher.h"
#include "bridge_runtime.h"
#include "bridge_console_fallback.h"
#include "bridge_v8.h"
#include "auto_inject.h"
#include "cef_ref.h"
#include "page_inject.h"
#include "gpu_cmdline.h"
#include "keyboard_devtools.h"
#include "capabilities/native_input_capability.h"
#include "bridge_bootstrap.js.h"
#include "include/capi/cef_browser_capi.h"
#include "include/capi/cef_client_capi.h"
#include "include/capi/cef_frame_capi.h"
#include "include/capi/cef_life_span_handler_capi.h"
#include "include/capi/cef_load_handler_capi.h"
#include "include/capi/cef_display_handler_capi.h"
#include "include/capi/cef_keyboard_handler_capi.h"
#include "include/internal/cef_types_win.h"
#include "include/internal/cef_win.h"

HMODULE g_hDll = NULL;

/* Runtime stop flag — set by DllMain / CefHookShutdown, checked by ThreadProc */
extern volatile LONG g_runtimeStopRequested;

/* Process role flag — set once during ThreadProc, read by callbacks */
bool g_isRendererProcess = false;

/* Hook installation state tracking */
static LONG g_hooksInstalled = 0;
static LONG g_hookRetryCount = 0;
#define HOOK_MAX_RETRIES    20
#define HOOK_RETRY_DELAY_MS 500

/* =========================================================================
 * CEF function pointers & global state
 * ========================================================================= */
PVOID g_cef_browser_host_create_browser = NULL;
PVOID g_cef_on_load_end = NULL;
PVOID g_cef_on_loading_state_change = NULL;
PVOID g_cef_on_after_created = NULL;
PVOID g_cef_on_before_close = NULL;
PVOID g_cef_on_before_popup = NULL;
PVOID g_cef_on_pre_key_event = NULL;
PVOID g_cef_on_key_event = NULL;
PVOID g_cef_get_keyboard_handler = NULL;

static _cef_frame_t*   g_mainFrame   = NULL;
static _cef_browser_t* g_mainBrowser = NULL;

/* Thread-safe ref-counted accessors for global frame/browser pointers.
 * StoreXxx: add_ref the new pointer, atomically swap, release the old.
 * ReleaseXxx: atomically swap to NULL and release the old reference.
 * CEF callbacks pass borrowed references — we must add_ref when storing. */
static inline void StoreMainFrame(_cef_frame_t* frame) {
    if (frame == g_mainFrame) return; /* same pointer — no-op */
    CefAddRef(&frame->base);
    void* old = InterlockedExchangePointer((volatile PVOID*)&g_mainFrame, frame);
    CefRelease((cef_base_ref_counted_t*)old);
}
static inline void StoreMainBrowser(_cef_browser_t* browser) {
    if (browser == g_mainBrowser) return;
    CefAddRef(&browser->base);
    void* old = InterlockedExchangePointer((volatile PVOID*)&g_mainBrowser, browser);
    CefRelease((cef_base_ref_counted_t*)old);
}
static inline void ReleaseMainFrame() {
    CefReleaseSlot((void* volatile*)&g_mainFrame);
}
static inline void ReleaseMainBrowser() {
    CefReleaseSlot((void* volatile*)&g_mainBrowser);
}

/* Track every frame that finished load — get_frame_identifiers often only
 * reports the main frame for Chaoxing's cross-origin exam iframes. */
#define LQ_MAX_TRACKED_FRAMES 32
static _cef_frame_t* g_trackedFrames[LQ_MAX_TRACKED_FRAMES];
static LONG g_trackedCount = 0;
static CRITICAL_SECTION g_trackedCs;
static LONG g_trackedCsReady = 0;

static void EnsureTrackedCs() {
    if (InterlockedCompareExchange(&g_trackedCsReady, 1, 0) == 0) {
        InitializeCriticalSection(&g_trackedCs);
        InterlockedExchange(&g_trackedCsReady, 2);
    } else {
        while (InterlockedCompareExchange(&g_trackedCsReady, 2, 2) != 2) Sleep(0);
    }
}

static int64 FrameIdOf(_cef_frame_t* f) {
    if (!f || !f->get_identifier) return -1;
    return f->get_identifier(f);
}

static void RememberLoadedFrame(_cef_frame_t* frame) {
    if (!frame) return;
    EnsureTrackedCs();
    EnterCriticalSection(&g_trackedCs);
    int64 id = FrameIdOf(frame);
    LONG n = g_trackedCount;
    for (LONG i = 0; i < n; ++i) {
        if (!g_trackedFrames[i]) continue;
        if (g_trackedFrames[i] == frame || FrameIdOf(g_trackedFrames[i]) == id) {
            LeaveCriticalSection(&g_trackedCs);
            return;
        }
        if (g_trackedFrames[i]->is_valid && !g_trackedFrames[i]->is_valid(g_trackedFrames[i])) {
            CefRelease(&g_trackedFrames[i]->base);
            g_trackedFrames[i] = NULL;
        }
    }
    /* compact nulls */
    LONG w = 0;
    for (LONG i = 0; i < n; ++i) {
        if (g_trackedFrames[i]) g_trackedFrames[w++] = g_trackedFrames[i];
    }
    g_trackedCount = w;
    if (g_trackedCount >= LQ_MAX_TRACKED_FRAMES) {
        CefRelease(&g_trackedFrames[0]->base);
        for (LONG i = 1; i < g_trackedCount; ++i) g_trackedFrames[i - 1] = g_trackedFrames[i];
        g_trackedCount--;
    }
    CefAddRef(&frame->base);
    g_trackedFrames[g_trackedCount++] = frame;
    char dbg[80];
    sprintf_s(dbg, "[HOOK] Remember frame id=%lld total=%ld\n", (long long)id, g_trackedCount);
    OutputDebugStringA(dbg);
    LeaveCriticalSection(&g_trackedCs);
}

static void ClearTrackedFrames() {
    EnsureTrackedCs();
    EnterCriticalSection(&g_trackedCs);
    for (LONG i = 0; i < g_trackedCount; ++i) {
        if (g_trackedFrames[i]) CefRelease(&g_trackedFrames[i]->base);
        g_trackedFrames[i] = NULL;
    }
    g_trackedCount = 0;
    LeaveCriticalSection(&g_trackedCs);
}

/* =========================================================================
 * CEF type definitions
 * ========================================================================= */
typedef int(CEF_EXPORT* cef_string_from_ptr_t)(const TCHAR*, size_t, cef_string_utf16_t*);
typedef void(CEF_EXPORT* cef_string_clear_t)(cef_string_utf16_t*);
cef_string_from_ptr_t func_cef_string_from_ptr = NULL;
cef_string_clear_t    func_cef_string_clear    = NULL;

typedef int(__cdecl* create_browser_t)(
    const cef_window_info_t*, struct _cef_client_t*,
    const cef_string_t*, const struct _cef_browser_settings_t*,
    struct _cef_dictionary_value_t*, struct _cef_request_context_t*);

typedef void(CEF_CALLBACK* on_loading_state_change_t)(
    struct _cef_load_handler_t*, struct _cef_browser_t*, int, int, int);

typedef void(CEF_CALLBACK* on_after_created_t)(
    struct _cef_life_span_handler_t*, struct _cef_browser_t*);

typedef void(CEF_CALLBACK* on_before_close_t)(
    struct _cef_life_span_handler_t*, struct _cef_browser_t*);

/* CEF cancels a popup unless the client allows it, so "click 开始考试 and nothing
 * happens" is indistinguishable from "the button is broken" without this log.
 * Returning 1 means the host blocked the popup. */
typedef int(CEF_CALLBACK* on_before_popup_t)(
    struct _cef_life_span_handler_t*, struct _cef_browser_t*, struct _cef_frame_t*,
    const cef_string_t*, const cef_string_t*, cef_window_open_disposition_t, int,
    const struct _cef_popup_features_t*, struct _cef_window_info_t*,
    struct _cef_client_t**, struct _cef_browser_settings_t*,
    struct _cef_dictionary_value_t**, int*);

typedef void(CEF_CALLBACK* on_load_end_t)(
    struct _cef_load_handler_t*, struct _cef_browser_t*, struct _cef_frame_t*, int);

/* =========================================================================
 * Helpers
 * ========================================================================= */
static PVOID SafeReadVtableSlot(PVOID* vtable, int idx) {
    __try {
        PVOID val = vtable[idx];
        if (IsInvalidCodePointer(val))
            return NULL;
        return val;
    }
    __except (EXCEPTION_EXECUTE_HANDLER) { return NULL; }
}

static _cef_frame_t* GetMainFrame(_cef_browser_t* browser) {
    if (!browser) return NULL;
    return browser->get_main_frame(browser);
}

/* =========================================================================
 * CEF Handler callbacks
 * ========================================================================= */

/* ---- Frame URL logging ----------------------------------------------------
 * A page the client never actually reaches cannot be diagnosed from the page
 * side, because nothing runs there. Logging every navigation from the browser
 * process shows exactly how far the client gets and whether a step bounces back
 * to a login page — which is otherwise invisible.
 * The SEH read is isolated in its own tiny function to keep C++ objects out of
 * the __try block (C2712). */
typedef void(CEF_EXPORT* cef_string_userfree_free_t)(cef_string_utf16_t*);
static cef_string_userfree_free_t g_cef_userfree_free = NULL;

static cef_string_userfree_t FrameUrlRaw(_cef_frame_t* frame) {
    cef_string_userfree_t url = NULL;
    __try {
        if (frame && frame->get_url) url = frame->get_url(frame);
    }
    __except (EXCEPTION_EXECUTE_HANDLER) { url = NULL; }
    return url;
}

/* Same isolation rule as FrameUrlRaw: no C++ objects may live in a function
 * that contains __try, or the compiler raises C2712. */
static int FrameIsMainSafe(_cef_frame_t* frame) {
    int isMain = 0;
    __try {
        if (frame && frame->is_main) isMain = frame->is_main(frame) ? 1 : 0;
    }
    __except (EXCEPTION_EXECUTE_HANDLER) { isMain = 0; }
    return isMain;
}

/* cef_string_t (UTF-16) to UTF-8, for logging the URL passed to create_browser. */
static std::string CefStringUtf8(const cef_string_t* str) {
    if (!str || !str->str || str->length == 0) return std::string();
    int len = WideCharToMultiByte(CP_UTF8, 0, str->str, (int)str->length, NULL, 0, NULL, NULL);
    if (len <= 0) return std::string();
    std::string out((size_t)len, '\0');
    WideCharToMultiByte(CP_UTF8, 0, str->str, (int)str->length, &out[0], len, NULL, NULL);
    return out;
}

static std::string FrameUrlUtf8(_cef_frame_t* frame) {
    cef_string_userfree_t u = FrameUrlRaw(frame);
    if (!u) return std::string();
    std::string out;
    if (u->str && u->length > 0) {
        int len = WideCharToMultiByte(CP_UTF8, 0, u->str, (int)u->length, NULL, 0, NULL, NULL);
        if (len > 0) {
            out.resize((size_t)len);
            WideCharToMultiByte(CP_UTF8, 0, u->str, (int)u->length, &out[0], len, NULL, NULL);
        }
    }
    if (g_cef_userfree_free) g_cef_userfree_free(u);
    return out;
}

static void LogNavigation(const char* phase, _cef_frame_t* frame, int httpStatus) {    std::string url = FrameUrlUtf8(frame);
    if (url.empty()) return;
    const int isMain = FrameIsMainSafe(frame);
    char dbg[384];
    if (httpStatus >= 0) {
        sprintf_s(dbg, "[HOOK] nav %s main=%d http=%d url=%.300s\n",
                  phase, isMain, httpStatus, url.c_str());
    } else {
        sprintf_s(dbg, "[HOOK] nav %s main=%d url=%.300s\n", phase, isMain, url.c_str());
    }
    OutputDebugStringA(dbg);
}

void CEF_CALLBACK MyOnLoadingStateChange(
    struct _cef_load_handler_t* self, struct _cef_browser_t* browser,
    int isLoading, int canGoBack, int canGoForward)
{
    (void)canGoBack; (void)canGoForward;
    if (isLoading) {
        PageScriptInjectReset();
        /* Log at load START too: a navigation that never completes (redirect
         * loop, hung request) would otherwise leave no trace at all. */
        if (browser && browser->get_main_frame) {
            _cef_frame_t* mf = browser->get_main_frame(browser);
            if (mf) {
                LogNavigation("start", mf, -1);
                CefRelease(&mf->base);
            }
        }
    } else {
        _cef_frame_t* frame = GetMainFrame(browser);
        if (frame) {
            StoreMainFrame(frame);
            StoreMainBrowser(browser);
            /* get_main_frame returned a new ref; StoreMainFrame added its own.
             * Release the temporary ref to avoid leaking it. */
            CefRelease(&frame->base);
        }
    }
    if (g_cef_on_loading_state_change)
        ((on_loading_state_change_t)g_cef_on_loading_state_change)(
            self, browser, isLoading, canGoBack, canGoForward);
}

void CEF_CALLBACK MyOnLoadEnd(
    struct _cef_load_handler_t* self, struct _cef_browser_t* browser,
    struct _cef_frame_t* frame, int httpStatusCode)
{
    if (frame) {
        LogNavigation("done", frame, httpStatusCode);
        if (frame->is_main(frame)) {
            StoreMainFrame(frame);
            StoreMainBrowser(browser);
        }
        RememberLoadedFrame(frame);
        /* Chaoxing exam UI often lives in child frames — inject every frame.
         * JS guard __AUTOEXAM_INJECTED__ prevents double init. */
        InjectPageScripts(frame);
        SchedulePageScriptRetries(frame);
    }
    if (g_cef_on_load_end)
        ((on_load_end_t)g_cef_on_load_end)(self, browser, frame, httpStatusCode);
}

int CEF_CALLBACK MyOnBeforePopup(
    struct _cef_life_span_handler_t* self, struct _cef_browser_t* browser,
    struct _cef_frame_t* frame, const cef_string_t* target_url,
    const cef_string_t* target_frame_name,
    cef_window_open_disposition_t target_disposition, int user_gesture,
    const struct _cef_popup_features_t* popupFeatures,
    struct _cef_window_info_t* windowInfo, struct _cef_client_t** client,
    struct _cef_browser_settings_t* settings,
    struct _cef_dictionary_value_t** extra_info, int* no_javascript_access)
{
    int blocked = -1;
    if (g_cef_on_before_popup) {
        blocked = ((on_before_popup_t)g_cef_on_before_popup)(
            self, browser, frame, target_url, target_frame_name, target_disposition,
            user_gesture, popupFeatures, windowInfo, client, settings, extra_info,
            no_javascript_access);
    }

    std::string url = CefStringUtf8(target_url);
    std::string name = CefStringUtf8(target_frame_name);
    char dbg[512];
    sprintf_s(dbg,
              "[HOOK] popup attempt url=%.300s frame=%.60s disposition=%d gesture=%d blocked=%d\n",
              url.empty() ? "(none)" : url.c_str(),
              name.empty() ? "(none)" : name.c_str(),
              (int)target_disposition, user_gesture, blocked);
    OutputDebugStringA(dbg);

    /* -1 means the host has no handler of its own; 0 preserves CEF's default
     * (allow). Never widen a block the host asked for. */
    return blocked < 0 ? 0 : blocked;
}

void CEF_CALLBACK MyOnAfterCreated(
    struct _cef_life_span_handler_t* self, struct _cef_browser_t* browser)
{
    StoreMainBrowser(browser);
    _cef_frame_t* frame = GetMainFrame(browser);
    if (frame) {
        StoreMainFrame(frame);
        CefRelease(&frame->base); /* release get_main_frame's temporary ref */
    }
    if (g_cef_on_after_created)
        ((on_after_created_t)g_cef_on_after_created)(self, browser);
}

void CEF_CALLBACK MyOnBeforeClose(
    struct _cef_life_span_handler_t* self, struct _cef_browser_t* browser)
{    /* If the closing browser is our tracked main browser, release all
     * stored CEF references to prevent dangling pointers. */
    if (g_mainBrowser == browser) {
        ReleaseMainFrame();
        ReleaseMainBrowser();
        BridgeReleaseMainFrame();
    }

    if (g_cef_on_before_close)
        ((on_before_close_t)g_cef_on_before_close)(self, browser);
}

/* =========================================================================
 * Dynamic vtable offset detection
 *
 * CEF client vtable layout varies between versions. We probe multiple
 * candidate offsets to find the get_load_handler and get_life_span_handler
 * slots rather than relying on hardcoded offsets.
 * ========================================================================= */

/* Candidate vtable slot indices for get_load_handler */
static const int LOAD_HANDLER_SLOTS[] = { 11, 12, 13, 10, 14, 9 };
#define LOAD_HANDLER_SLOT_COUNT (sizeof(LOAD_HANDLER_SLOTS) / sizeof(LOAD_HANDLER_SLOTS[0]))

/* Candidate vtable slot indices for get_life_span_handler */
static const int LIFE_SPAN_HANDLER_SLOTS[] = { 10, 11, 12, 9, 13, 8 };
#define LIFE_SPAN_HANDLER_SLOT_COUNT (sizeof(LIFE_SPAN_HANDLER_SLOTS) / sizeof(LIFE_SPAN_HANDLER_SLOTS[0]))

/* Candidate table slot indices relative to cef_client_t.get_audio_handler. */
static const int KEYBOARD_HANDLER_SLOTS[] = { 9, 8, 10, 7, 11 };
#define KEYBOARD_HANDLER_SLOT_COUNT (sizeof(KEYBOARD_HANDLER_SLOTS) / sizeof(KEYBOARD_HANDLER_SLOTS[0]))

/* Resolve a handler through its documented struct field.
 *
 * cef_client_t exposes every getter directly, so guessing vtable slots is
 * unnecessary — and risky: a wrong slot returns a handler of a different type,
 * so the callback fields read afterwards land out of bounds and can look like
 * valid code pointers. These are deliberately tiny separate functions so the
 * SEH block stays free of C++ objects that would require unwinding (C2712). */
static struct _cef_load_handler_t* ResolveLoadHandlerField(struct _cef_client_t* client) {
    struct _cef_load_handler_t* lh = NULL;
    __try {
        if (client && client->get_load_handler) lh = client->get_load_handler(client);
    }
    __except (EXCEPTION_EXECUTE_HANDLER) { lh = NULL; }
    return lh;
}

static struct _cef_life_span_handler_t* ResolveLifeSpanHandlerField(struct _cef_client_t* client) {
    struct _cef_life_span_handler_t* lsh = NULL;
    __try {
        if (client && client->get_life_span_handler) lsh = client->get_life_span_handler(client);
    }
    __except (EXCEPTION_EXECUTE_HANDLER) { lsh = NULL; }
    return lsh;
}

/* Try to hook load handler, preferring the documented struct field */
static void TryHookLoadHandler(struct _cef_client_t* client) {
    if (g_cef_on_load_end && g_cef_on_loading_state_change) return;
    if (!client) return;

    /* Once the authoritative handler is in hand we never fall through to the
     * slot probe, which also stops callbacks from being mixed across two
     * different slots. */
    struct _cef_load_handler_t* direct = ResolveLoadHandlerField(client);
    if (direct) {
        if (direct->on_loading_state_change && !g_cef_on_loading_state_change) {
            g_cef_on_loading_state_change = (PVOID)direct->on_loading_state_change;
            HookAttach(&g_cef_on_loading_state_change, MyOnLoadingStateChange);
        }
        if (direct->on_load_end && !g_cef_on_load_end) {
            g_cef_on_load_end = (PVOID)direct->on_load_end;
            HookAttach(&g_cef_on_load_end, MyOnLoadEnd);
        }
        if (g_cef_on_load_end || g_cef_on_loading_state_change) {
            OutputDebugStringA("[HOOK] LoadHandler hooked via struct field\n");
            BridgeGetHookState().loadHandler = true;
            return;
        }
    }

    PVOID* vtable = (PVOID*)((BYTE*)client + CefClientHandlerTableOffset());

    for (int i = 0; i < (int)LOAD_HANDLER_SLOT_COUNT; i++) {
        int slot = LOAD_HANDLER_SLOTS[i];
        PVOID ldSlot = SafeReadVtableSlot(vtable, slot);
        if (!ldSlot) continue;

        typedef struct _cef_load_handler_t*(CEF_CALLBACK* get_ld_t)(struct _cef_client_t*);
        struct _cef_load_handler_t* lh = ((get_ld_t)ldSlot)(client);
        if (!lh) continue;

        if (lh->on_loading_state_change && !g_cef_on_loading_state_change) {
            g_cef_on_loading_state_change = (PVOID)lh->on_loading_state_change;
            HookAttach(&g_cef_on_loading_state_change, MyOnLoadingStateChange);
        }
        if (lh->on_load_end && !g_cef_on_load_end) {
            g_cef_on_load_end = (PVOID)lh->on_load_end;
            HookAttach(&g_cef_on_load_end, MyOnLoadEnd);
        }

        if (g_cef_on_load_end && g_cef_on_loading_state_change) {
            char dbg[128];
            sprintf_s(dbg, "[HOOK] LoadHandler hooked at slot %d\n", slot);
            OutputDebugStringA(dbg);
            BridgeGetHookState().loadHandler = true;
            return;
        }
    }
    OutputDebugStringA("[HOOK] WARNING: LoadHandler not found at any candidate slot\n");
}

/* Try to hook life span handler, preferring the documented struct field */
static void TryHookLifeSpanHandler(struct _cef_client_t* client) {
    if (g_cef_on_after_created && g_cef_on_before_close) return;
    if (!client) return;

    struct _cef_life_span_handler_t* direct = ResolveLifeSpanHandlerField(client);
    if (direct) {
        if (direct->on_after_created && !g_cef_on_after_created) {
            g_cef_on_after_created = (PVOID)direct->on_after_created;
            HookAttach(&g_cef_on_after_created, MyOnAfterCreated);
        }
        if (direct->on_before_close && !g_cef_on_before_close) {
            g_cef_on_before_close = (PVOID)direct->on_before_close;
            HookAttach(&g_cef_on_before_close, MyOnBeforeClose);
        }
        /* Log popup attempts so a blocked "start exam" popup is visible. Only
         * hookable when the host implements the callback; if it does not, the
         * absence of a popup log plus the create_browser log still tells us
         * whether a new window was ever requested. */
        if (!g_cef_on_before_popup) {
            if (direct->on_before_popup) {
                g_cef_on_before_popup = (PVOID)direct->on_before_popup;
                LONG r = HookAttach(&g_cef_on_before_popup, (PVOID)MyOnBeforePopup);
                if (r == ERROR_SUCCESS) {
                    OutputDebugStringA("[HOOK] on_before_popup hooked — popup attempts will be logged\n");
                } else {
                    char dbg[128];
                    sprintf_s(dbg, "[HOOK] on_before_popup hook failed: %ld\n", r);
                    OutputDebugStringA(dbg);
                    g_cef_on_before_popup = NULL;
                }
            } else {
                OutputDebugStringA("[HOOK] host has no on_before_popup — popup attempts "
                                   "cannot be logged (CEF default applies)\n");
            }
        }
        if (g_cef_on_after_created || g_cef_on_before_close) {
            OutputDebugStringA("[HOOK] LifeSpanHandler hooked via struct field\n");
            BridgeGetHookState().lifeSpanHandler = true;
            return;
        }
    }

    PVOID* vtable = (PVOID*)((BYTE*)client + CefClientHandlerTableOffset());

    for (int i = 0; i < (int)LIFE_SPAN_HANDLER_SLOT_COUNT; i++) {
        int slot = LIFE_SPAN_HANDLER_SLOTS[i];
        PVOID lsSlot = SafeReadVtableSlot(vtable, slot);
        if (!lsSlot) continue;

        typedef struct _cef_life_span_handler_t*(CEF_CALLBACK* get_ls_t)(struct _cef_client_t*);
        struct _cef_life_span_handler_t* lsh = ((get_ls_t)lsSlot)(client);
        if (!lsh) continue;

        if (lsh->on_after_created && !g_cef_on_after_created) {
            g_cef_on_after_created = (PVOID)lsh->on_after_created;
            HookAttach(&g_cef_on_after_created, MyOnAfterCreated);
        }
        if (lsh->on_before_close && !g_cef_on_before_close) {
            g_cef_on_before_close = (PVOID)lsh->on_before_close;
            HookAttach(&g_cef_on_before_close, MyOnBeforeClose);
        }

        if (g_cef_on_after_created && g_cef_on_before_close) {
            char dbg[128];
            sprintf_s(dbg, "[HOOK] LifeSpanHandler hooked at slot %d\n", slot);
            OutputDebugStringA(dbg);
            BridgeGetHookState().lifeSpanHandler = true;
            return;
        }
    }
    OutputDebugStringA("[HOOK] WARNING: LifeSpanHandler not found at any candidate slot\n");
}

static void TryHookKeyboardHandler(struct _cef_client_t* client) {
    if (!client || (g_cef_on_pre_key_event && g_cef_on_key_event)) return;

    struct _cef_keyboard_handler_t* kh = NULL;
    __try {
        if (client->get_keyboard_handler) {
            if (!g_cef_get_keyboard_handler) {
                g_cef_get_keyboard_handler = (PVOID)client->get_keyboard_handler;
                LONG r = HookAttach(&g_cef_get_keyboard_handler,
                                    (PVOID)MyGetKeyboardHandler);
                if (r == ERROR_SUCCESS) {
                    OutputDebugStringA("[HOOK] get_keyboard_handler hooked for DevTools shortcut\n");
                } else {
                    char dbg[128];
                    sprintf_s(dbg, sizeof(dbg),
                              "[HOOK] get_keyboard_handler hook failed: %ld\n", r);
                    OutputDebugStringA(dbg);
                    g_cef_get_keyboard_handler = NULL;
                }
            }

            if (g_cef_get_keyboard_handler) {
                kh = ((get_keyboard_handler_t)g_cef_get_keyboard_handler)(client);
            } else {
                kh = client->get_keyboard_handler(client);
            }
        }
    }
    __except (EXCEPTION_EXECUTE_HANDLER) {
        kh = NULL;
    }

    if (!kh && !g_cef_get_keyboard_handler) {
        PVOID* table = (PVOID*)((BYTE*)client + CefClientHandlerTableOffset());
        for (int i = 0; i < (int)KEYBOARD_HANDLER_SLOT_COUNT && !kh; ++i) {
            int slot = KEYBOARD_HANDLER_SLOTS[i];
            PVOID kbSlot = SafeReadVtableSlot(table, slot);
            if (!kbSlot) continue;

            typedef struct _cef_keyboard_handler_t*(CEF_CALLBACK* get_kb_t)(struct _cef_client_t*);
            __try {
                kh = ((get_kb_t)kbSlot)(client);
            }
            __except (EXCEPTION_EXECUTE_HANDLER) {
                kh = NULL;
            }
        }
    }

    if (!kh) {
        if (g_cef_get_keyboard_handler) {
            OutputDebugStringA("[HOOK] KeyboardHandler fallback ready for DevTools shortcut\n");
        } else {
            OutputDebugStringA("[HOOK] WARNING: KeyboardHandler not found\n");
        }
        return;
    }

    bool hooked = HookKeyboardCallbacks(kh);

    if (hooked) {
        OutputDebugStringA("[HOOK] KeyboardHandler hooked for DevTools shortcut\n");
    } else {
        OutputDebugStringA("[HOOK] WARNING: KeyboardHandler has no hookable callbacks\n");
    }
}

/* =========================================================================
 * create_browser Hook
 * ========================================================================= */
int __cdecl MyCreateBrowser(
    const cef_window_info_t* windowInfo, struct _cef_client_t* client,
    const cef_string_t* url, const struct _cef_browser_settings_t* settings,
    struct _cef_dictionary_value_t* extra_info,
    struct _cef_request_context_t* request_context)
{
    /* Log every browser creation. A new window (popup) is a plausible way for
     * the exam paper to open, and without this it is indistinguishable from
     * "the client never left the previous page". */
    {
        std::string u = CefStringUtf8(url);
        char dbg[384];
        sprintf_s(dbg, "[HOOK] create_browser url=%.300s\n",
                  u.empty() ? "(none)" : u.c_str());
        OutputDebugStringA(dbg);
    }
    if (client) {
        if (!g_cef_on_after_created || !g_cef_on_before_close) TryHookLifeSpanHandler(client);
        if (!g_cef_on_load_end || !g_cef_on_loading_state_change) TryHookLoadHandler(client);
        if (!g_cef_on_pre_key_event || !g_cef_on_key_event) TryHookKeyboardHandler(client);
        /* Bridge: hook display handler for console transport */
        if (!g_cef_on_console_message) TryHookDisplayHandler(client);
    }
    create_browser_t orig = (create_browser_t)g_cef_browser_host_create_browser;
    return orig(windowInfo, client, url, settings, extra_info, request_context);
}

/* =========================================================================
 * Screen capture prevention
 * ========================================================================= */
typedef BOOL(WINAPI* set_affinity_t)(HWND, DWORD);
PVOID g_set_window_display_affinity = NULL;

BOOL WINAPI MySetWindowDisplayAffinity(HWND hWnd, DWORD dwAffinity) {
    if (dwAffinity == WDA_MONITOR) return TRUE;
    if (g_set_window_display_affinity)
        return ((set_affinity_t)g_set_window_display_affinity)(hWnd, dwAffinity);
    return TRUE;
}

/* =========================================================================
 * InstallHook — install all hooks with retry logic
 * ========================================================================= */
BOOL APIENTRY InstallHook() {
    OutputDebugString(TEXT("[HOOK] Installing all hooks...\n"));

    /* 1. GPU software rendering */
    g_orig_GetCommandLineW = (GetCommandLineW_t)
        HookFindFunction("kernel32.dll", "GetCommandLineW");
    if (g_orig_GetCommandLineW) {
        LONG r = HookAttach((PVOID*)&g_orig_GetCommandLineW, (PVOID)MyGetCommandLineW);
        if (r != ERROR_SUCCESS) {
            char dbg[128];
            sprintf_s(dbg, "[HOOK] GetCommandLineW hook failed: %ld\n", r);
            OutputDebugStringA(dbg);
        }
    }

    /* 2. create_browser */
    g_cef_browser_host_create_browser =
        HookFindFunction("libcef.dll", "cef_browser_host_create_browser");
    if (g_cef_browser_host_create_browser) {
        LONG r = HookAttach(&g_cef_browser_host_create_browser, (PVOID)MyCreateBrowser);
        if (r != ERROR_SUCCESS) {
            char dbg[128];
            sprintf_s(dbg, "[HOOK] create_browser hook failed: %ld\n", r);
            OutputDebugStringA(dbg);
            return FALSE;
        }
    } else {
        OutputDebugString(TEXT("[HOOK] FATAL: create_browser not found\n"));
        return FALSE;
    }

    /* 3. CEF string utilities */
#ifdef UNICODE
    func_cef_string_from_ptr = (cef_string_from_ptr_t)
        HookFindFunction("libcef.dll", "cef_string_wide_to_utf16");
    func_cef_string_clear = (cef_string_clear_t)
        HookFindFunction("libcef.dll", "cef_string_utf16_clear");
#else
    func_cef_string_from_ptr = (cef_string_from_ptr_t)
        HookFindFunction("libcef.dll", "cef_string_ascii_to_utf16");
    func_cef_string_clear = (cef_string_clear_t)
        HookFindFunction("libcef.dll", "cef_string_utf8_clear");
#endif

    /* Frame URL logging frees the string returned by cef_frame_t::get_url. */
    g_cef_userfree_free = (cef_string_userfree_free_t)
        HookFindFunction("libcef.dll", "cef_string_userfree_utf16_free");

    /* 4. Screen capture prevention */
    g_set_window_display_affinity =
        HookFindFunction("user32.dll", "SetWindowDisplayAffinity");
    if (g_set_window_display_affinity)
        HookAttach(&g_set_window_display_affinity, (PVOID)MySetWindowDisplayAffinity);

    /* Update bridge hook state tracking */
    BridgeGetHookState().createBrowser = (g_cef_browser_host_create_browser != NULL);

    return TRUE;
}

/* =========================================================================
 * Native input poller (browser process)
 *
 * Chaoxing native shell swallows middle-button / F2 before CEF JS.
 * Inject AutoExam.trigger into EVERY frame — the exam UI often lives
 * in a child iframe, not the top-level document.
 * ========================================================================= */
static void ExecJsOnFrame(_cef_frame_t* frame, const wchar_t* js, size_t jsLen) {
    if (!frame || !js || !jsLen || !func_cef_string_from_ptr) return;
    cef_string_t eval = {0}, url = {0};
    func_cef_string_from_ptr(js, jsLen, &eval);
    func_cef_string_from_ptr(L"", 0, &url);
    frame->execute_java_script(frame, &eval, &url, 0);
    if (func_cef_string_clear) {
        func_cef_string_clear(&eval);
        func_cef_string_clear(&url);
    }
}

struct TriggerCtx {
    char source[32];
};

/* Must run on CEF UI thread. Prefer the bridge deliver path (already proven
 * with "deliver frames ok=2") — execute_java_script from the poller often
 * never reaches Chaoxing frames even when posted to the UI thread. */
static void TriggerAutoExamOnUi(void* raw) {
    TriggerCtx* ctx = (TriggerCtx*)raw;
    if (!ctx) return;
    const char* source = ctx->source;

    char payload[80];
    sprintf_s(payload, "{\"source\":\"%s\"}", source);
    std::string eventJson = BridgeMakeEvent("nativeTrigger", payload);

    _cef_browser_t* browser = g_mainBrowser;
    if (browser) {
        BridgeDeliverToAllFramesSafe(browser, eventJson);
        OutputDebugStringA("[HOOK] Native trigger pushed via bridge event\n");
    } else {
        OutputDebugStringA("[HOOK] Native trigger: no browser for bridge event\n");
    }

    /* Fallback: also try direct execute_java_script on known frames. */
    if (func_cef_string_from_ptr) {
        char jsUtf8[768];
        sprintf_s(jsUtf8,
            "(function(){try{"
            "var br=window.LingQiaoBridge;"
            "var ae=window.AutoExam;"
            "var has=!!(ae&&ae.trigger);"
            "if(br&&br.ready){br.request('diagnostics.log',{level:'info',scope:'hook',message:'trigger-js src=%s autoexam='+has}).catch(function(){});}"
            "if(has)ae.trigger('%s');"
            "}catch(e){try{if(window.LingQiaoBridge)window.LingQiaoBridge.request('diagnostics.log',{level:'error',scope:'hook',message:'trigger-js err '+e}).catch(function(){});}catch(e2){}}})();",
            source, source);
        int wideLen = MultiByteToWideChar(CP_UTF8, 0, jsUtf8, -1, NULL, 0);
        if (wideLen > 1) {
            std::wstring js((size_t)wideLen, L'\0');
            MultiByteToWideChar(CP_UTF8, 0, jsUtf8, -1, &js[0], wideLen);
            size_t jsLen = wcslen(js.c_str());
            int delivered = 0;
            EnsureTrackedCs();
            EnterCriticalSection(&g_trackedCs);
            LONG n = g_trackedCount;
            std::vector<_cef_frame_t*> snapshot;
            snapshot.reserve(n);
            for (LONG i = 0; i < n; ++i) {
                if (!g_trackedFrames[i]) continue;
                if (g_trackedFrames[i]->is_valid && !g_trackedFrames[i]->is_valid(g_trackedFrames[i])) continue;
                CefAddRef(&g_trackedFrames[i]->base);
                snapshot.push_back(g_trackedFrames[i]);
            }
            LeaveCriticalSection(&g_trackedCs);
            for (size_t i = 0; i < snapshot.size(); ++i) {
                ExecJsOnFrame(snapshot[i], js.c_str(), jsLen);
                CefRelease(&snapshot[i]->base);
                ++delivered;
            }
            if (g_mainFrame) {
                ExecJsOnFrame(g_mainFrame, js.c_str(), jsLen);
                ++delivered;
            }
            char dbg[96];
            sprintf_s(dbg, "[HOOK] Native trigger exec-js fallback frames=%d src=%s\n", delivered, source);
            OutputDebugStringA(dbg);
        }
    }
    delete ctx;
}

static void TriggerAutoExamOnAllFrames(const char* source) {
    TriggerCtx* ctx = new (std::nothrow) TriggerCtx();
    if (!ctx) return;
    ctx->source[0] = 0;
    if (source) {
        strncpy_s(ctx->source, source, _TRUNCATE);
    }

    if (BridgeIsUiThread()) {
        TriggerAutoExamOnUi(ctx);
        return;
    }
    if (BridgePostToUiThread(TriggerAutoExamOnUi, ctx)) {
        OutputDebugStringA("[HOOK] Native trigger posted to CEF UI thread\n");
        return;
    }
    OutputDebugStringA("[HOOK] Native trigger UI post failed — running inline\n");
    TriggerAutoExamOnUi(ctx);
}

static void StartNativeInputPoller() {
    std::thread([]() {
        bool f2Prev = false, mbtnPrev = false;
        DWORD lastFire = 0;
        bool warnedStale = false;
        while (InterlockedCompareExchange(&g_runtimeStopRequested, 0, 0) == 0) {
            Sleep(40);

            /* A trigger only leaves the mailbox for a frame that reports
             * ready=1. If it stays armed, no frame saw the exam DOM — say so
             * loudly, because otherwise a middle-click on a non-exam page looks
             * exactly like a broken AI request. */
            unsigned pendingMs = BridgeNativeTriggerPendingMs();
            if (pendingMs == 0) {
                warnedStale = false;
            } else if (pendingMs >= 3000 && !warnedStale) {
                warnedStale = true;
                char dbg[256];
                sprintf_s(dbg,
                    "[HOOK] trigger armed %u ms ago and still unconsumed — no frame reported "
                    "ready=1, i.e. no exam DOM on the current page (check the AutoExam "
                    "'examDom='/'hits=' lines). No AI request will be sent.\n",
                    pendingMs);
                OutputDebugStringA(dbg);
            }

            bool f2 = (GetAsyncKeyState(VK_F2) & 0x8000) != 0;
            bool mbtn = (GetAsyncKeyState(VK_MBUTTON) & 0x8000) != 0;
            bool f2Edge = f2 && !f2Prev;
            bool mEdge = mbtn && !mbtnPrev;
            f2Prev = f2;
            mbtnPrev = mbtn;
            if (!f2Edge && !mEdge) continue;

            DWORD now = GetTickCount();
            if (now - lastFire < 800) continue;
            lastFire = now;
            const char* src = f2Edge ? "poll-F2" : "poll-MBUTTON";
            /* Page polls native.pollTrigger — execute_java_script is unreliable. */
            BridgeArmNativeTrigger(src);
            TriggerAutoExamOnAllFrames(src);
        }
    }).detach();
}

/* =========================================================================
 * Worker thread entry with retry logic
 * ========================================================================= */
DWORD WINAPI ThreadProc(LPVOID lpThreadParameter) {
    (void)lpThreadParameter;
    Sleep(100);
    ProcessIdentity identity = GetCurrentProcessIdentity();
    const std::wstring line = FormatProcessIdentityLog(identity);
    OutputDebugStringW(line.c_str());
    OutputDebugStringW(L"\n");

    g_isRendererProcess = (identity.cefRole == L"renderer");

    /* ---- Initialize Bridge v2 ---- */
    BridgeRuntimeInit();
    BridgeInitDispatcher();

    DWORD t0 = GetTickCount();

    if (identity.cefRole == L"renderer") {
        /* === Renderer process: V8-level hooks (gated by env var) ===
         * CEF handler hooks (create_browser, load/life_span/display) are
         * installed in the browser process because the renderer never calls
         * cef_browser_host_create_browser — browser objects live in the
         * browser process and are communicated to the renderer via IPC. */

        char v8Enable[8] = {0};
        GetEnvironmentVariableA("LQ_ENABLE_V8_INTERCEPT", v8Enable, sizeof(v8Enable));
#if defined(_M_IX86)
        /* V8 strong mode is enabled by default after IDA-confirmed ABI
         * correction. Set LQ_ENABLE_V8_INTERCEPT=0 to force a passive
         * renderer if this target regresses. */
        if (v8Enable[0] != '0') {
            RunV8SignatureDiagnostics();
            if (AttachV8CompileHookOnly()) {
                OutputDebugStringA("[HOOK] V8 strong anti-debugger compile hook active\n");
            } else {
                OutputDebugStringA("[HOOK] V8 strong anti-debugger compile hook unavailable\n");
            }
        } else {
            OutputDebugStringA("[HOOK] V8 intercept disabled by LQ_ENABLE_V8_INTERCEPT=0\n");
        }
#else
        /* x64 V8 naked detours are not implemented. Keep the existing safe default. */
        if (v8Enable[0] == '1') {
            OutputDebugStringA("[HOOK] V8 intercept requested but unavailable on x64\n");
        } else {
            OutputDebugStringA("[HOOK] V8 intercept disabled on x64\n");
        }
#endif

        /* === Bridge V8 transport — the synchronous JS -> C++ channel ===
         *
         * This was never wired up. Without it the V8 C API pointers stay NULL,
         * so InstallV8ContextHook() bails out on its guard, g_orig_v8ctx_get
         * stays NULL, and BridgeTryInstallFromV8Hook() — which IS called from
         * the script_run detour — returns immediately on every single call.
         *
         * Net effect: window.__LingQiaoNative was never created in any frame.
         * detectTransport() therefore always reported 'console', the V8 poll
         * relay never started, and every bridge call (including the 250ms
         * native.pollTrigger / ai.pollResult polls) went over the console
         * transport — whose response path is execute_java_script, which the
         * rest of this codebase documents as unreliable on Chaoxing frames.
         * That is why so much of the bridge is built around polling mailboxes.
         *
         * Set LQ_ENABLE_V8_BRIDGE=0 to keep the old console-only behaviour. */
        {
            char v8Bridge[8] = {0};
            GetEnvironmentVariableA("LQ_ENABLE_V8_BRIDGE", v8Bridge, sizeof(v8Bridge));
            if (v8Bridge[0] != '0') {
                if (!BridgeInstallV8Transport()) {
                    OutputDebugStringA("[HOOK] V8 bridge transport unavailable — "
                                       "console fallback remains\n");
                } else if (!InstallV8ContextHook()) {
                    OutputDebugStringA("[HOOK] V8 bridge transport: context hook failed — "
                                       "console fallback remains\n");
                } else {
                    OutputDebugStringA("[HOOK] V8 bridge transport armed — "
                                       "__LingQiaoNative installs on first V8 activity\n");
                }
            } else {
                OutputDebugStringA("[HOOK] V8 bridge transport disabled by "
                                   "LQ_ENABLE_V8_BRIDGE=0\n");
            }
        }

        /* Process-message transport was a scaffold and is no longer built. */
    } else {
        /* === Browser process: CEF hooks + auto-inject ===
         * The browser process is where cef_browser_host_create_browser is
         * called, so all CEF handler hooks (load, life_span, display) must
         * be installed here.  execute_java_script on the browser-side frame
         * sends scripts to the renderer via CEF IPC. */
        if (InstallHook()) {
            InterlockedExchange(&g_hooksInstalled, 1);
            OutputDebugString(TEXT("[HOOK] All hooks installed successfully\n"));
        } else {
            OutputDebugString(TEXT("[HOOK] Some hooks FAILED - will retry on next browser creation\n"));
        }

        /* Auto-inject: hook CreateProcessW to inject into child renderers */
        InstallAutoInjectHook();

        /* Native shell swallows F2 / middle-click — poll at Win32 level. */
        StartNativeInputPoller();
        OutputDebugStringA("[HOOK] Native input poller started (F2 / middle-button)\n");

        /* Periodic self-healing: check if hooks are still intact.
         * Some CEF versions reload DLLs or re-initialize vtables.
         * This loop exits promptly when the runtime stop flag is set. */
        for (int retry = 0; retry < HOOK_MAX_RETRIES; retry++) {
            /* Check stop flag FIRST — exit promptly on shutdown request */
            if (InterlockedCompareExchange(&g_runtimeStopRequested, 0, 0) != 0) {
                OutputDebugStringA("[HOOK] Stop requested — exiting monitoring loop\n");
                break;
            }

            Sleep(HOOK_RETRY_DELAY_MS);

            /* Check if main CEF hooks are still active */
            if (g_cef_browser_host_create_browser &&
                g_cef_on_load_end && g_cef_on_after_created) {
                /* All hooks active — continue monitoring */
                continue;
            }

            /* Some hooks missing — try to re-install */
            if (!g_cef_browser_host_create_browser) {
                PVOID addr = HookFindFunction("libcef.dll", "cef_browser_host_create_browser");
                if (addr) {
                    g_cef_browser_host_create_browser = addr;
                    HookAttach(&g_cef_browser_host_create_browser, (PVOID)MyCreateBrowser);
                    OutputDebugStringA("[HOOK] Re-hooked create_browser\n");
                }
            }
        }
    }

    DWORD t1 = GetTickCount();
    char dbg[128];
    sprintf_s(dbg, "[HOOK] Hook thread completed after %lu ms\n", t1 - t0);
    OutputDebugStringA(dbg);

    return 0;
}
