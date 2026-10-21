/* ============================================================================
 * bridge_v8.cpp — V8 Native Bridge transport via CEF V8 C API
 *
 * Strategy:
 *   1. At DLL load, resolve V8 C API exports from libcef.dll
 *   2. On first page load (MyOnLoadEnd), enter the frame's V8 context
 *   3. Create a cef_v8handler_t with an execute callback → BridgeDispatch
 *   4. Create a cef_v8value_t function → cef_v8value_create_function()
 *   5. Install as window.__LingQiaoNative.request on the global object
 *
 * If any V8 C API is missing or the context isn't available, the transport
 * gracefully degrades to console. The JS bootstrap auto-detects which
 * transport is active on every request.
 * ========================================================================= */

#include "pch.h"
#include "bridge_v8.h"
#include "bridge_runtime.h"
#include "bridge_dispatcher.h"
#include "bridge_json.h"
#include "bridge_protocol.h"
#include "hook_engine.h"
#include "v8_hooks.h"
#include "include/capi/cef_v8_capi.h"
#include "include/capi/cef_frame_capi.h"

/* ---- State ---- */
static bool g_v8TransportActive = false;

/* =========================================================================
 * V8 C API function pointer types (resolved from libcef.dll at runtime)
 * ========================================================================= */

/* Context */
typedef cef_v8context_t* (CEF_EXPORT* pfn_cef_v8context_get_current_context)();
typedef int              (CEF_EXPORT* pfn_cef_v8context_in_context)();

/* Value creation */
typedef cef_v8value_t* (CEF_EXPORT* pfn_cef_v8value_create_object)(
    cef_v8accessor_t*, cef_v8interceptor_t*);
typedef cef_v8value_t* (CEF_EXPORT* pfn_cef_v8value_create_function)(
    const cef_string_t*, cef_v8handler_t*);
typedef cef_v8value_t* (CEF_EXPORT* pfn_cef_v8value_create_string)(
    const cef_string_t*);
typedef cef_v8value_t* (CEF_EXPORT* pfn_cef_v8value_create_undefined)();

/* String utilities */
typedef void (CEF_EXPORT* pfn_cef_string_userfree_utf16_free)(cef_string_userfree_t);

/* Resolved function pointers */
static pfn_cef_v8context_get_current_context  g_v8ctx_get_current   = NULL;
static pfn_cef_v8context_in_context           g_v8ctx_in_context    = NULL;
static pfn_cef_v8value_create_object          g_v8val_create_obj    = NULL;
static pfn_cef_v8value_create_function        g_v8val_create_func   = NULL;
static pfn_cef_v8value_create_string          g_v8val_create_str    = NULL;
static pfn_cef_v8value_create_undefined       g_v8val_create_undef  = NULL;
static pfn_cef_string_userfree_utf16_free     g_v8str_free          = NULL;

/* CEF string utility functions */
typedef int  (CEF_EXPORT* pfn_cef_string_utf16_set)(const char16*, size_t, cef_string_utf16_t*, int);
typedef void (CEF_EXPORT* pfn_cef_string_utf16_clear)(cef_string_utf16_t*);
static pfn_cef_string_utf16_set   g_v8str_set   = NULL;
static pfn_cef_string_utf16_clear g_v8str_clear  = NULL;

/* =========================================================================
 * Resolve V8 C API exports from libcef.dll
 * ========================================================================= */

static bool ResolveV8CAPIFunctions() {
    g_v8ctx_get_current  = (pfn_cef_v8context_get_current_context)
        HookFindFunction("libcef.dll", "cef_v8context_get_current_context");
    g_v8ctx_in_context   = (pfn_cef_v8context_in_context)
        HookFindFunction("libcef.dll", "cef_v8context_in_context");
    g_v8val_create_obj   = (pfn_cef_v8value_create_object)
        HookFindFunction("libcef.dll", "cef_v8value_create_object");
    g_v8val_create_func  = (pfn_cef_v8value_create_function)
        HookFindFunction("libcef.dll", "cef_v8value_create_function");
    g_v8val_create_str   = (pfn_cef_v8value_create_string)
        HookFindFunction("libcef.dll", "cef_v8value_create_string");
    g_v8val_create_undef = (pfn_cef_v8value_create_undefined)
        HookFindFunction("libcef.dll", "cef_v8value_create_undefined");
    g_v8str_free         = (pfn_cef_string_userfree_utf16_free)
        HookFindFunction("libcef.dll", "cef_string_userfree_utf16_free");
    g_v8str_set          = (pfn_cef_string_utf16_set)
        HookFindFunction("libcef.dll", "cef_string_utf16_set");
    g_v8str_clear        = (pfn_cef_string_utf16_clear)
        HookFindFunction("libcef.dll", "cef_string_utf16_clear");

    /* Check critical functions */
    bool ok = g_v8ctx_get_current && g_v8val_create_obj &&
              g_v8val_create_func && g_v8val_create_str && g_v8str_set;

    char dbg[256];
    sprintf_s(dbg,
        "[BRIDGE:V8] CAPI resolve: ctx_get=%p ctx_in=%p create_obj=%p create_func=%p "
        "create_str=%p create_undef=%p str_free=%p → %s\n",
        g_v8ctx_get_current, g_v8ctx_in_context, g_v8val_create_obj,
        g_v8val_create_func, g_v8val_create_str, g_v8val_create_undef,
        g_v8str_free, ok ? "OK" : "INCOMPLETE");
    OutputDebugStringA(dbg);

    return ok;
}

/* =========================================================================
 * cef_v8handler_t implementation for __LingQiaoNative.request()
 *
 * This is a ref-counted C structure. We implement add_ref/release manually
 * with InterlockedIncrement/Decrement. The handler lives for the lifetime
 * of the V8 context (usually the page lifetime).
 * ========================================================================= */

struct BridgeV8Handler {
    cef_v8handler_t handler;
    volatile LONG refCount;
};

/* Forward declarations for vtable callbacks */
static void CEF_CALLBACK BridgeV8HandlerAddRef(cef_base_ref_counted_t* self);
static int  CEF_CALLBACK BridgeV8HandlerRelease(cef_base_ref_counted_t* self);
static int  CEF_CALLBACK BridgeV8HandlerHasOneRef(cef_base_ref_counted_t* self);
static int  CEF_CALLBACK BridgeV8HandlerHasAtLeastOneRef(cef_base_ref_counted_t* self);
static int  CEF_CALLBACK BridgeV8HandlerExecute(
    struct _cef_v8handler_t* self,
    const cef_string_t* name,
    struct _cef_v8value_t* object,
    size_t argumentsCount,
    struct _cef_v8value_t* const* arguments,
    struct _cef_v8value_t** retval,
    cef_string_t* exception);

/* Convert cef_string_t to std::string */
static std::string V8StringToUtf8(const cef_string_t* str) {
    if (!str || !str->str || str->length == 0) return "";
    int len = WideCharToMultiByte(CP_UTF8, 0, str->str, (int)str->length, NULL, 0, NULL, NULL);
    if (len <= 0) return "";
    std::string result(len, '\0');
    WideCharToMultiByte(CP_UTF8, 0, str->str, (int)str->length, &result[0], len, NULL, NULL);
    return result;
}

/* Convert UTF-8 std::string into a cef_string_t (UTF-16).
 * Uses cef_string_utf16_set with copy=1 so CEF manages the memory.
 * Caller must call g_v8str_clear(&dest) when done. */
static bool V8SetString(cef_string_t* dest, const std::string& src) {
    if (!dest) return false;
    memset(dest, 0, sizeof(cef_string_t));

    if (src.empty()) return true;
    if (!g_v8str_set) return false;

    /* Convert UTF-8 → UTF-16 (wchar_t on Windows is UTF-16 compatible) */
    int wideLen = MultiByteToWideChar(CP_UTF8, 0, src.data(), (int)src.size(), NULL, 0);
    if (wideLen <= 0) return false;

    wchar_t* buf = (wchar_t*)_alloca((wideLen + 1) * sizeof(wchar_t));
    MultiByteToWideChar(CP_UTF8, 0, src.data(), (int)src.size(), buf, wideLen);
    buf[wideLen] = L'\0';

    /* cef_string_utf16_set with copy=1 copies the data internally */
    return g_v8str_set((const char16*)buf, (size_t)wideLen, dest, 1) != 0;
}

/* ---- Ref counting callbacks ---- */

static void CEF_CALLBACK BridgeV8HandlerAddRef(cef_base_ref_counted_t* self) {
    BridgeV8Handler* h = (BridgeV8Handler*)self;
    InterlockedIncrement(&h->refCount);
}

static int CEF_CALLBACK BridgeV8HandlerRelease(cef_base_ref_counted_t* self) {
    BridgeV8Handler* h = (BridgeV8Handler*)self;
    LONG newCount = InterlockedDecrement(&h->refCount);
    if (newCount == 0) {
        free(h);
        return 1;
    }
    return 0;
}

static int CEF_CALLBACK BridgeV8HandlerHasOneRef(cef_base_ref_counted_t* self) {
    BridgeV8Handler* h = (BridgeV8Handler*)self;
    return InterlockedCompareExchange(&h->refCount, 0, 0) == 1 ? 1 : 0;
}

static int CEF_CALLBACK BridgeV8HandlerHasAtLeastOneRef(cef_base_ref_counted_t* self) {
    BridgeV8Handler* h = (BridgeV8Handler*)self;
    return InterlockedCompareExchange(&h->refCount, 0, 0) >= 1 ? 1 : 0;
}

/* ---- Execute callback — the core of V8 transport ---- */

/*
 * Called when JS invokes window.__LingQiaoNative.request(jsonString).
 * arguments[0] = the JSON string
 * Returns: the response JSON string
 */
static int CEF_CALLBACK BridgeV8HandlerExecute(
    struct _cef_v8handler_t* self,
    const cef_string_t* name,
    struct _cef_v8value_t* object,
    size_t argumentsCount,
    struct _cef_v8value_t* const* arguments,
    struct _cef_v8value_t** retval,
    cef_string_t* exception)
{
    (void)self; (void)name; (void)object;

    /* Extract the JSON string argument */
    std::string jsonStr;
    if (argumentsCount >= 1 && arguments[0]) {
        cef_string_userfree_t strVal = arguments[0]->get_string_value(arguments[0]);
        if (strVal) {
            jsonStr = V8StringToUtf8(strVal);
            if (g_v8str_free) {
                g_v8str_free(strVal);
            }
        }
    }

    if (jsonStr.empty()) {
        /* Return error response */
        std::string errResp = BridgeMakeError("", LQ_ERR_INVALID_REQUEST, "Empty request");
        if (retval && g_v8val_create_str) {
            cef_string_t errStr = { 0 };
            V8SetString(&errStr, errResp);
            *retval = g_v8val_create_str(&errStr);
            if (g_v8str_clear) g_v8str_clear(&errStr);
        }
        return 1;
    }

    /* Dispatch through the bridge RPC engine */
    std::string response = BridgeDispatch(jsonStr);

    /* Return the response as a string value */
    if (retval && g_v8val_create_str) {
        cef_string_t respStr = { 0 };
        V8SetString(&respStr, response);
        *retval = g_v8val_create_str(&respStr);
        if (g_v8str_clear) g_v8str_clear(&respStr);
    }

    return 1; /* handled */
}

/* =========================================================================
 * Create the handler object
 * ========================================================================= */

static BridgeV8Handler* CreateBridgeV8Handler() {
    BridgeV8Handler* h = (BridgeV8Handler*)calloc(1, sizeof(BridgeV8Handler));
    if (!h) return NULL;

    h->refCount = 1; /* initial reference */

    /* Set up base ref-counted vtable */
    h->handler.base.size = sizeof(cef_v8handler_t);
    h->handler.base.add_ref = BridgeV8HandlerAddRef;
    h->handler.base.release = BridgeV8HandlerRelease;
    h->handler.base.has_one_ref = BridgeV8HandlerHasOneRef;
    h->handler.base.has_at_least_one_ref = BridgeV8HandlerHasAtLeastOneRef;

    /* Set up handler vtable */
    h->handler.execute = BridgeV8HandlerExecute;

    return h;
}

/* =========================================================================
 * Forward declaration from thread.cpp — CEF string conversion functions
 * ========================================================================= */
typedef int(CEF_EXPORT* cef_string_from_ptr_t)(const TCHAR*, size_t, cef_string_utf16_t*);
typedef void(CEF_EXPORT* cef_string_clear_t)(cef_string_utf16_t*);
extern cef_string_from_ptr_t func_cef_string_from_ptr;
extern cef_string_clear_t    func_cef_string_clear;

/* =========================================================================
 * Install native bridge object into the V8 context
 *
 * Must be called from a context where the V8 context is accessible
 * (i.e., on the renderer process main thread, after page load).
 *
 * Creates: window.__LingQiaoNative = { request: function(jsonStr) { ... } }
 * ========================================================================= */

bool BridgeInstallNativeObject(_cef_frame_t* frame) {
    (void)frame; /* frame unused — V8 context is obtained directly */
    if (!g_v8ctx_get_current || !g_v8val_create_func || !g_v8val_create_obj) {
        OutputDebugStringA("[BRIDGE:V8] V8 CAPI not resolved — cannot install native object\n");
        return false;
    }

    /* Get current V8 context via the raw export address.
     * NOTE: This goes through the hooked entry point if our hook is active,
     * so callers from within the detour should use BridgeInstallNativeObjectInContext
     * instead to avoid reentrancy. */
    cef_v8context_t* ctx = g_v8ctx_get_current();
    if (!ctx) {
        OutputDebugStringA("[BRIDGE:V8] No current V8 context\n");
        return false;
    }

    return BridgeInstallNativeObjectInContext(ctx);
}

/* =========================================================================
 * Install native bridge object using a pre-obtained V8 context.
 *
 * This function does NOT call cef_v8context_get_current_context, making it
 * safe to call from within the detour (no reentrancy through the hook).
 *
 * The ctx parameter must come from the trampoline (g_orig_v8ctx_get), NOT
 * from the hooked export address.
 * ========================================================================= */
bool BridgeInstallNativeObjectInContext(cef_v8context_t* ctx) {
    if (!ctx || !g_v8val_create_func || !g_v8val_create_obj) {
        OutputDebugStringA("[BRIDGE:V8] Invalid context or V8 CAPI not resolved\n");
        return false;
    }

    /* Validate context */
    if (!ctx->is_valid(ctx)) {
        OutputDebugStringA("[BRIDGE:V8] V8 context not valid on this thread\n");
        return false;
    }

    /* The bridge is a privileged native boundary. Never expose it to
     * subframes, which may be cross-origin or controlled by embedded content. */
    cef_frame_t* contextFrame = ctx->get_frame ? ctx->get_frame(ctx) : NULL;
    if (!contextFrame) {
        OutputDebugStringA("[BRIDGE:V8] Context has no frame\n");
        return false;
    }
    const bool isMainFrame = contextFrame->is_main &&
                             contextFrame->is_main(contextFrame) != 0;
    contextFrame->base.release(&contextFrame->base);
    if (!isMainFrame) {
        OutputDebugStringA("[BRIDGE:V8] Refusing bridge installation in subframe\n");
        return false;
    }

    /* Enter the V8 context */
    if (!ctx->enter(ctx)) {
        OutputDebugStringA("[BRIDGE:V8] Failed to enter V8 context\n");
        return false;
    }

    bool success = false;

    /* Get the global object (window) */
    cef_v8value_t* globalObj = ctx->get_global(ctx);
    if (globalObj) {
        /* Create the handler */
        BridgeV8Handler* handler = CreateBridgeV8Handler();
        if (handler) {
            /* Create the "request" function */
            cef_string_t funcName = { 0 };
            V8SetString(&funcName, "request");
            cef_v8value_t* requestFunc = g_v8val_create_func(&funcName, &handler->handler);
            if (g_v8str_clear) g_v8str_clear(&funcName);

            if (requestFunc) {
                /* Create the __LingQiaoNative container object */
                cef_v8value_t* nativeObj = g_v8val_create_obj(NULL, NULL);
                if (nativeObj) {
                    /* Add "request" as a property of the container */
                    cef_string_t keyName = { 0 };
                    V8SetString(&keyName, "request");
                    int setOk = nativeObj->set_value_bykey(nativeObj, &keyName,
                                                           requestFunc, V8_PROPERTY_ATTRIBUTE_NONE);
                    if (g_v8str_clear) g_v8str_clear(&keyName);

                    if (setOk) {
                        /* Install the container as window.__LingQiaoNative */
                        cef_string_t globalKey = { 0 };
                        V8SetString(&globalKey, "__LingQiaoNative");
                        int installOk = globalObj->set_value_bykey(globalObj, &globalKey,
                                                                   nativeObj, V8_PROPERTY_ATTRIBUTE_NONE);
                        if (g_v8str_clear) g_v8str_clear(&globalKey);

                        if (installOk) {
                            OutputDebugStringA("[BRIDGE:V8] Native object installed: "
                                               "window.__LingQiaoNative.request\n");
                            success = true;
                        } else {
                            OutputDebugStringA("[BRIDGE:V8] Failed to set __LingQiaoNative on global\n");
                        }
                    } else {
                        OutputDebugStringA("[BRIDGE:V8] Failed to set request on native object\n");
                    }

                    /* Release native object (V8 holds its own ref) */
                    nativeObj->base.release(&nativeObj->base);
                }

                /* Release function value */
                requestFunc->base.release(&requestFunc->base);
            }

            /* Release handler's initial reference (V8 holds its own ref via create_function) */
            handler->handler.base.release(&handler->handler.base);
        }

        /* Release global object */
        globalObj->base.release(&globalObj->base);
    }

    /* Exit the V8 context */
    ctx->exit(ctx);

    /* Release context reference */
    ctx->base.release(&ctx->base);

    return success;
}

/* =========================================================================
 * V8 Context hook — auto-install native object on first V8 activity
 *
 * In the renderer process, MyOnLoadEnd never fires (CEF handler hooks live
 * in the browser process). Instead, we hook cef_v8context_get_current_context
 * and install the native object the first time V8 returns a valid context.
 * ========================================================================= */

typedef cef_v8context_t* (CEF_EXPORT* pfn_cef_v8context_get_current_context_hook)();
static pfn_cef_v8context_get_current_context_hook g_orig_v8ctx_get = NULL;
static volatile LONG g_v8NativeObjectInstalled = 0;

static cef_v8context_t* CEF_EXPORT MyCefV8ContextGetCurrentContext() {
    /* Call via trampoline (NOT the hooked export) to avoid reentrancy */
    cef_v8context_t* ctx = g_orig_v8ctx_get();

    /* First time we see a valid context → install native object */
    if (ctx && !InterlockedCompareExchange(&g_v8NativeObjectInstalled, 1, 0)) {
        /* Use InContext variant — passes ctx directly, no hooked export call */
        if (BridgeInstallNativeObjectInContext(ctx)) {
            g_v8TransportActive = true;
            BridgeGetHookState().v8 = true;
            BridgeSetTransport(BRIDGE_TRANSPORT_V8);
            OutputDebugStringA("[BRIDGE:V8] Native object installed via context hook "
                               "— V8 transport active\n");
        } else {
            OutputDebugStringA("[BRIDGE:V8] Context hook fired but installation failed "
                               "— will retry\n");
            InterlockedExchange(&g_v8NativeObjectInstalled, 0); /* allow retry */
        }
    }

    return ctx;
}

bool InstallV8ContextHook() {
    if (!g_v8ctx_get_current) {
        OutputDebugStringA("[BRIDGE:V8] ctx_get_current not resolved — context hook skipped\n");
        return false;
    }

    g_orig_v8ctx_get = (pfn_cef_v8context_get_current_context_hook)g_v8ctx_get_current;
    LONG r = HookAttach((PVOID*)&g_orig_v8ctx_get, (PVOID)MyCefV8ContextGetCurrentContext);
    if (r != ERROR_SUCCESS) {
        char dbg[128];
        sprintf_s(dbg, "[BRIDGE:V8] Context hook failed: %ld\n", r);
        OutputDebugStringA(dbg);
        return false;
    }

    OutputDebugStringA("[BRIDGE:V8] Context hook installed — native object will be "
                       "installed on first V8 activity\n");
    return true;
}

void UninstallV8ContextHook() {
    if (g_orig_v8ctx_get) {
        HookDetach((PVOID*)&g_orig_v8ctx_get);
        g_orig_v8ctx_get = NULL;
        OutputDebugStringA("[BRIDGE:V8] Context hook removed\n");
    }
    InterlockedExchange(&g_v8NativeObjectInstalled, 0);
}

/* =========================================================================
 * V8 script_run trigger — install native object during V8 execution
 *
 * Called from the V8 script_run detour (v8_hooks.cpp). At this point we
 * are guaranteed to be on the V8 thread with an active context.
 * First call installs; subsequent calls are a single atomic read.
 * ========================================================================= */

static volatile LONG g_v8ScriptRunInstalled = 0;

extern "C" void BridgeTryInstallFromV8Hook() {
    /* Fast path: already installed or attempted */
    if (InterlockedCompareExchange(&g_v8ScriptRunInstalled, 1, 0) != 0)
        return;

    /* Guard: V8 CAPI must be resolved.
     * Use the trampoline (g_orig_v8ctx_get) to get context — never the hooked
     * export address which would re-enter our detour. */
    if (!g_orig_v8ctx_get || !g_v8val_create_func) {
        static volatile LONG s_warned = 0;
        if (InterlockedCompareExchange(&s_warned, 1, 0) == 0) {
            char dbg[192];
            sprintf_s(dbg, "[BRIDGE:V8] script_run trigger cannot install yet "
                           "(ctx_trampoline=%d create_func=%d) — V8 transport stays off\n",
                      g_orig_v8ctx_get ? 1 : 0, g_v8val_create_func ? 1 : 0);
            OutputDebugStringA(dbg);
        }
        InterlockedExchange(&g_v8ScriptRunInstalled, 0);
        return;
    }

    /* Get context via trampoline — safe, no reentrancy */
    cef_v8context_t* ctx = g_orig_v8ctx_get();
    if (!ctx) {
        static volatile LONG s_noCtx = 0;
        if (InterlockedCompareExchange(&s_noCtx, 1, 0) == 0) {
            OutputDebugStringA("[BRIDGE:V8] script_run trigger: no current context "
                               "— will retry\n");
        }
        InterlockedExchange(&g_v8ScriptRunInstalled, 0);
        return;
    }

    if (BridgeInstallNativeObjectInContext(ctx)) {
        g_v8TransportActive = true;
        BridgeGetHookState().v8 = true;
        BridgeSetTransport(BRIDGE_TRANSPORT_V8);
        OutputDebugStringA("[BRIDGE:V8] Native object installed via script_run trigger "
                           "— V8 transport active\n");
    } else {
        InterlockedExchange(&g_v8ScriptRunInstalled, 0); /* allow retry */
        OutputDebugStringA("[BRIDGE:V8] script_run trigger: installation failed\n");
    }
}

/* =========================================================================
 * Public API
 * ========================================================================= */

bool BridgeInstallV8Transport() {
    OutputDebugStringA("[BRIDGE:V8] Resolving V8 C API functions...\n");

    if (!ResolveV8CAPIFunctions()) {
        OutputDebugStringA("[BRIDGE:V8] V8 CAPI resolution failed — console fallback active\n");
        g_v8TransportActive = false;
        BridgeGetHookState().v8 = false;
        return false;
    }

    /* V8 C API is available. The actual native object installation happens
     * later in BridgeInstallNativeObject() when a V8 context is accessible
     * (called from MyOnLoadEnd on the renderer process thread). */
    OutputDebugStringA("[BRIDGE:V8] V8 CAPI resolved — native object will be installed on first page load\n");
    return true; /* CAPI available, pending context installation */
}

/* Called from MyOnLoadEnd to install the native bridge object.
 * If successful, switches the transport to V8. */
bool BridgeTryInstallNativeObject(_cef_frame_t* frame) {
    if (g_v8TransportActive) return true; /* already installed */

    if (BridgeInstallNativeObject(frame)) {
        g_v8TransportActive = true;
        BridgeGetHookState().v8 = true;
        BridgeSetTransport(BRIDGE_TRANSPORT_V8);
        OutputDebugStringA("[BRIDGE:V8] Transport upgraded to V8 native\n");
        return true;
    }

    OutputDebugStringA("[BRIDGE:V8] Native object installation failed — staying on console\n");
    return false;
}

void BridgeDetachV8Transport() {
    g_v8TransportActive = false;
    BridgeGetHookState().v8 = false;
    OutputDebugStringA("[BRIDGE:V8] V8 transport detached\n");
}

bool BridgeIsV8TransportActive() {
    return g_v8TransportActive;
}
