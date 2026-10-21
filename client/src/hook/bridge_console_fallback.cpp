/* ============================================================================
 * bridge_console_fallback.cpp — Console transport implementation
 *
 * Hooks the CEF display handler's on_console_message to intercept bridge
 * messages sent from JS via console.info with the bridge prefix.
 * ========================================================================= */

#include "pch.h"
#include "bridge_console_fallback.h"
#include "bridge_dispatcher.h"
#include "bridge_runtime.h"
#include "bridge_protocol.h"
#include "bridge_security.h"
#include "bridge_json.h"
#include "capabilities/native_input_capability.h"
#include "hook_engine.h"
#include "hook_safety.h"
#include "cef_ref.h"
#include "include/capi/cef_display_handler_capi.h"
#include "include/capi/cef_client_capi.h"
#include "include/capi/cef_frame_capi.h"
#include "include/capi/cef_browser_capi.h"

#include <string>
#include <thread>
#include <vector>

/* Global hook pointer for cleanup */
PVOID g_cef_on_console_message = NULL;

/* Original display handler on_console_message callback type */
typedef int(CEF_CALLBACK* on_console_message_t)(
    struct _cef_display_handler_t*, struct _cef_browser_t*,
    cef_log_severity_t, const cef_string_t*, const cef_string_t*, int);

/* Forward declaration for helpers from thread.cpp */
typedef int(CEF_EXPORT* cef_string_from_ptr_t)(const TCHAR*, size_t, cef_string_utf16_t*);
extern cef_string_from_ptr_t func_cef_string_from_ptr;

/* Helper: convert cef_string_t to std::string (UTF-8) */
static std::string CefStringToUtf8(const cef_string_t* str) {
    if (!str || !str->str || str->length == 0) return "";

    /* cef_string_t is UTF-16 (wchar_t on Windows) */
    int utf8Len = WideCharToMultiByte(CP_UTF8, 0, str->str, (int)str->length, NULL, 0, NULL, NULL);
    if (utf8Len <= 0) return "";

    std::string result(utf8Len, '\0');
    WideCharToMultiByte(CP_UTF8, 0, str->str, (int)str->length, &result[0], utf8Len, NULL, NULL);
    return result;
}

static bool IsTrustedConsoleSource(struct _cef_browser_t* browser,
                                   const cef_string_t* source) {
    (void)browser;
    /* Dedicated exam client: we inject scripts ourselves. Child frames
     * (mooc / kaoshi hosts) are legitimate AutoExam callers — do not
     * same-origin-check against the main frame URL. */
    (void)source;
    return true;
}

/* Deliver response to every frame so iframe-originated RPCs complete.
 * JS pending maps are per-frame and keyed by request id.
 * Prefer BridgeDeliverToAllFramesSafe (UI-thread aware). */
static void BridgeDeliverToAllFrames(struct _cef_browser_t* browser, const std::string& json) {
    BridgeDeliverToAllFramesSafe(browser, json);
}

/* =========================================================================
 * Console message hook — intercepts bridge messages
 * ========================================================================= */
int CEF_CALLBACK MyOnConsoleMessage(
    struct _cef_display_handler_t* self, struct _cef_browser_t* browser,
    cef_log_severity_t level, const cef_string_t* message,
    const cef_string_t* source, int line)
{
    static volatile LONG s_consoleHits = 0;
    LONG hits = InterlockedIncrement(&s_consoleHits);
    if (hits <= 3 || (hits % 100) == 0) {
        char dbg[80];
        sprintf_s(dbg, "[BRIDGE:CONSOLE] on_console_message hit=%ld\n", hits);
        OutputDebugStringA(dbg);
    }

    /* Convert message to UTF-8 */
    std::string msg = CefStringToUtf8(message);

    /* Check for bridge prefix */
    if (msg.compare(0, LQ_BRIDGE_CONSOLE_PREFIX_LEN, LQ_BRIDGE_CONSOLE_PREFIX) == 0) {
        char dbg[128];
        sprintf_s(dbg, "[BRIDGE:CONSOLE] msg_len=%zu\n", msg.size());
        OutputDebugStringA(dbg);
        if (!IsTrustedConsoleSource(browser, source)) {
            OutputDebugStringA("[BRIDGE:CONSOLE] Rejected bridge message from untrusted frame\n");
            return 1;
        }

        /* Extract the JSON payload after the prefix */
        std::string rawJson = msg.substr(LQ_BRIDGE_CONSOLE_PREFIX_LEN);

        OutputDebugStringA("[BRIDGE:CONSOLE] Received bridge message\n");

        /* Long-running AI must not block the CEF UI thread (middle-click
         * cursor:wait would stick and the page would freeze). Dispatch
         * ai.chatComplete on a worker thread; deliver when done. */
        if (rawJson.find("\"" LQ_METHOD_AI_CHAT_COMPLETE "\"") != std::string::npos) {
            {
                char dbg[192];
                sprintf_s(dbg, "[BRIDGE:CONSOLE] ai.chatComplete async dispatch bytes=%zu head=%.80s\n",
                          rawJson.size(), rawJson.c_str());
                OutputDebugStringA(dbg);
            }
            CefAddRef(browser ? &browser->base : NULL);
            std::string rawCopy = rawJson;
            _cef_browser_t* browserKeep = browser;
            std::thread([browserKeep, rawCopy]() {
                std::string reqId = BridgeJsonGetString(rawCopy, "id");
                std::string response = BridgeDispatch(rawCopy);
                {
                    char dbg[160];
                    sprintf_s(dbg, "[BRIDGE:CONSOLE] ai.chatComplete dispatch done bytes=%zu head=%.80s\n",
                              response.size(), response.c_str());
                    OutputDebugStringA(dbg);
                }
                /* Mailbox for page poll — execute_java_script deliver is unreliable. */
                if (!reqId.empty()) {
                    bool ok = response.find("\"ok\":true") != std::string::npos;
                    if (ok) {
                        std::string result = BridgeJsonGetString(response, "result");
                        BridgeStoreAiResult(reqId, result.empty() ? "null" : result);
                    } else {
                        std::string err = BridgeJsonGetString(response, "error");
                        BridgeStoreAiResult(reqId,
                            err.empty() ? "{\"error\":{\"code\":\"FAIL\",\"message\":\"unknown\"}}"
                                        : ("{\"error\":" + err + "}"));
                    }
                }
                if (browserKeep) {
                    BridgeDeliverToAllFrames(browserKeep, response);
                    CefRelease(&browserKeep->base);
                }
                OutputDebugStringA("[BRIDGE:CONSOLE] ai.chatComplete delivered (async)\n");
            }).detach();
            return 1;
        }

        /* Dispatch the request */
        std::string response = BridgeDispatch(rawJson);

        /* Deliver the response to all frames (request may come from an iframe). */
        BridgeDeliverToAllFrames(browser, response);

        /* Return 1 to suppress the message from appearing in the console */
        return 1;
    }

    /* Not a bridge message — pass through to original handler */
    if (g_cef_on_console_message) {
        return ((on_console_message_t)g_cef_on_console_message)(
            self, browser, level, message, source, line);
    }
    return 0;
}

/* =========================================================================
 * Dynamic vtable offset detection for display handler
 *
 * Similar to load/life_span handler probing in thread.cpp.
 * We scan multiple candidate slots to find get_display_handler.
 * ========================================================================= */

/* Candidate vtable slot indices for get_display_handler.
 * Slot 3 has been the real DisplayHandler on Chaoxing CEF builds; earlier
 * first-hit-wins logic sometimes attached to slot 4 and silently broke the
 * console bridge (page still logs __LQ_BRIDGE_V2__ but MyOnConsoleMessage
 * never runs). Prefer 3, then the historical fallbacks. */
static const int DISPLAY_HANDLER_SLOTS[] = { 3, 5, 6, 4, 7, 8, 2, 9 };
#define DISPLAY_HANDLER_SLOT_COUNT (sizeof(DISPLAY_HANDLER_SLOTS) / sizeof(DISPLAY_HANDLER_SLOTS[0]))

/* on_console_message lives in the host EXE (CXexam.exe implements CefClient)
 * or in libcef — not only in libcef. Reject data/heap pointers. */
static bool IsExecutableModulePointer(PVOID p) {
    if (!p) return false;
    MEMORY_BASIC_INFORMATION mbi;
    if (!VirtualQuery(p, &mbi, sizeof(mbi))) return false;
    if (mbi.State != MEM_COMMIT) return false;
    DWORD prot = mbi.Protect & 0xFF;
    bool exec =
        prot == PAGE_EXECUTE || prot == PAGE_EXECUTE_READ ||
        prot == PAGE_EXECUTE_READWRITE || prot == PAGE_EXECUTE_WRITECOPY;
    if (!exec) return false;
    HMODULE h = NULL;
    if (!GetModuleHandleExW(
            GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS | GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
            (LPCWSTR)p, &h) || !h) {
        return false;
    }
    return true;
}

bool TryHookDisplayHandler(struct _cef_client_t* client) {
    if (g_cef_on_console_message) return true; /* already hooked */
    if (!client) return false;

    int bestSlot = -1;
    PVOID bestOrig = NULL;

    /* Preferred path: the documented struct field, mirroring
     * TryHookKeyboardHandler. cef_client_t exposes get_display_handler
     * directly, so there is no need to guess vtable slots. */
    __try {
        if (client->get_display_handler) {
            struct _cef_display_handler_t* dh = client->get_display_handler(client);
            PVOID onConsole = dh ? (PVOID)dh->on_console_message : NULL;
            if (onConsole && IsExecutableModulePointer(onConsole)) {
                bestSlot = 0;
                bestOrig = onConsole;
                OutputDebugStringA("[BRIDGE] DisplayHandler resolved via struct field\n");
            } else {
                OutputDebugStringA("[BRIDGE] DisplayHandler: get_display_handler present but "
                                   "on_console_message is NULL/non-exec — console transport "
                                   "unavailable on this target\n");
            }
        }
    }
    __except (EXCEPTION_EXECUTE_HANDLER) { bestOrig = NULL; }

    PVOID* vtable = (PVOID*)((BYTE*)client + CefClientHandlerTableOffset());

    /* Fallback only: the slot list is a last resort. A wrong slot returns a
     * handler of a different type, so the on_console_message read below is out
     * of bounds and may look executable by accident. */
    for (int i = 0; bestSlot < 0 && i < (int)DISPLAY_HANDLER_SLOT_COUNT; i++) {
        int slot = DISPLAY_HANDLER_SLOTS[i];

        PVOID slotVal = NULL;
        __try {
            slotVal = vtable[slot];
            if (IsInvalidCodePointer(slotVal)) continue;
        }
        __except (EXCEPTION_EXECUTE_HANDLER) { continue; }

        if (!slotVal) continue;

        typedef struct _cef_display_handler_t*(CEF_CALLBACK* get_dh_t)(struct _cef_client_t*);
        struct _cef_display_handler_t* dh = NULL;
        __try {
            dh = ((get_dh_t)slotVal)(client);
        }
        __except (EXCEPTION_EXECUTE_HANDLER) { dh = NULL; }
        if (!dh) continue;

        PVOID onConsole = NULL;
        __try {
            onConsole = (PVOID)dh->on_console_message;
        }
        __except (EXCEPTION_EXECUTE_HANDLER) { onConsole = NULL; }

        char dbg[160];
        sprintf_s(dbg, "[BRIDGE] DisplayHandler probe slot=%d get=%p dh=%p on_console=%p exec=%d\n",
                  slot, slotVal, (void*)dh, onConsole,
                  IsExecutableModulePointer(onConsole) ? 1 : 0);
        OutputDebugStringA(dbg);

        if (!onConsole || !IsExecutableModulePointer(onConsole)) continue;

        if (bestSlot < 0) {
            bestSlot = slot;
            bestOrig = onConsole;
        }
        break;
    }

    if (bestSlot < 0 || !bestOrig) {
        OutputDebugStringA("[BRIDGE] WARNING: DisplayHandler not found at any candidate slot\n");
        return false;
    }

    g_cef_on_console_message = bestOrig;
    LONG r = HookAttach(&g_cef_on_console_message, (PVOID)MyOnConsoleMessage);
    if (r == ERROR_SUCCESS) {
        char dbg[128];
        sprintf_s(dbg, "[BRIDGE] DisplayHandler hooked at slot %d\n", bestSlot);
        OutputDebugStringA(dbg);
        BridgeGetHookState().displayHandler = true;
        return true;
    }

    char dbg[128];
    sprintf_s(dbg, "[BRIDGE] DisplayHandler hook failed: %ld at slot %d\n", r, bestSlot);
    OutputDebugStringA(dbg);
    g_cef_on_console_message = NULL;
    return false;
}

void DetachDisplayHandlerHook() {
    if (g_cef_on_console_message) {
        HookDetach(&g_cef_on_console_message);
        g_cef_on_console_message = NULL;
        BridgeGetHookState().displayHandler = false;
    }
}
