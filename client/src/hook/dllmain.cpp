#include "pch.h"
#include "hook_engine.h"
#include "v8_hooks.h"
#include "bridge_runtime.h"
#include "bridge_console_fallback.h"
#include "bridge_v8.h"
#include "auto_inject.h"
#include "v8_intercept.h"
#include "cef_ref.h"

typedef LPWSTR(WINAPI* GetCommandLineW_t)(void);

extern DWORD WINAPI ThreadProc(LPVOID lpThreadParameter);

/* ---- Global runtime state ---- */
volatile LONG g_runtimeStopRequested = 0;
static HANDLE  g_hWorkerThread  = NULL;
static DWORD   g_workerThreadId = 0;

/* Shutdown state machine (three-state):
 *   0 = idle        — no shutdown attempted yet
 *   1 = in progress — shutdown is running (or was aborted, reset to 0)
 *   2 = completed   — full cleanup done, further calls are no-ops */
static volatile LONG g_shutdownState = 0;

/* External hook pointers for cleanup */
extern PVOID g_cef_browser_host_create_browser;
extern PVOID g_cef_on_load_end;
extern PVOID g_cef_on_loading_state_change;
extern PVOID g_cef_on_after_created;
extern PVOID g_cef_on_before_close;
extern PVOID g_cef_on_pre_key_event;
extern PVOID g_cef_on_key_event;
extern PVOID g_cef_get_keyboard_handler;
extern GetCommandLineW_t g_orig_GetCommandLineW;
extern PVOID g_set_window_display_affinity;

/* =========================================================================
 * CefHookShutdown — explicit full cleanup for clean unload scenarios
 *
 * CONTRACT:
 *   - Caller MUST call CefHookShutdown() BEFORE FreeLibrary() if a clean
 *     unload is desired.  This function performs all hook detachment,
 *     transport shutdown, and CEF reference release.
 *   - Safe to call from any thread EXCEPT the worker thread itself.
 *   - Three-state idempotency:
 *       completed (state=2) → returns immediately
 *       in-progress (state=1) → returns immediately (another thread running)
 *       idle (state=0) → claims ownership, does the work
 *   - If the worker thread does not exit within 3 s, or if called from
 *     the worker thread itself, the shutdown is ABORTED and state is
 *     reset to 0 so the caller may retry later.
 *   - If the process is terminating (no clean unload), DllMain only sets
 *     the stop flag; OS reclaims everything else.
 * ========================================================================= */
extern void ExamCapabilityShutdown();

extern "C" __declspec(dllexport) void CefHookShutdown() {
    /* ---- Three-state entry guard ---- */
    LONG prev = InterlockedCompareExchange(&g_shutdownState, 1, 0);
    if (prev == 2) {
        OutputDebugStringA("[DLL] CefHookShutdown already completed\n");
        return;
    }
    if (prev != 0) {
        OutputDebugStringA("[DLL] CefHookShutdown already in progress\n");
        return;
    }

    /* 1. Signal all background threads to stop */
    InterlockedExchange(&g_runtimeStopRequested, 1);

    /* 2. Wait for the worker thread — but only if we're not the worker */
    HANDLE worker = g_hWorkerThread;
    if (worker) {
        if (GetCurrentThreadId() == g_workerThreadId) {
            OutputDebugStringA("[DLL] CefHookShutdown aborted: called from worker thread\n");
            /* NOTE: g_runtimeStopRequested stays 1.  Abort allows the
             * caller to retry shutdown later — it does NOT resume normal
             * operation.  Monitoring loops will have already begun exiting. */
            InterlockedExchange(&g_shutdownState, 0);
            return;
        }

        DWORD waitResult = WaitForSingleObject(worker, 3000);
        if (waitResult != WAIT_OBJECT_0) {
            char dbg[160];
            sprintf_s(dbg, sizeof(dbg),
                "[DLL] CefHookShutdown aborted: worker wait failed "
                "wait=%lu err=%lu\n",
                waitResult, GetLastError());
            OutputDebugStringA(dbg);
            /* Reset state to idle so caller can retry after the worker
             * eventually exits.  g_runtimeStopRequested stays 1 — abort
             * allows retry, not resume normal operation. */
            InterlockedExchange(&g_shutdownState, 0);
            return;
        }

        CloseHandle(worker);
        g_hWorkerThread  = NULL;
        g_workerThreadId = 0;
    }

    /* 3. Detach transports and hooks (order matters: high-level first) */
    ExamCapabilityShutdown();
    UninstallV8ContextHook();
    BridgeDetachV8Transport();
    DetachDisplayHandlerHook();
    UninstallAutoInjectHook();
    V8InterceptShutdown();
    DetachV8SignatureHooks();
    HookDetachAll();

    /* 4. Clear global trampoline / hook pointers so stale calls crash loud */
    g_cef_browser_host_create_browser = NULL;
    g_cef_on_load_end = NULL;
    g_cef_on_loading_state_change = NULL;
    g_cef_on_after_created = NULL;
    g_cef_on_before_close = NULL;
    g_cef_on_pre_key_event = NULL;
    g_cef_on_key_event = NULL;
    g_cef_get_keyboard_handler = NULL;
    g_set_window_display_affinity = NULL;
    g_orig_GetCommandLineW = NULL;

    /* 5. Release CEF references and shut down bridge runtime */
    BridgeRuntimeShutdown();

    /* Mark completed (state 2) — subsequent calls return immediately */
    InterlockedExchange(&g_shutdownState, 2);
    OutputDebugStringA("[DLL] CefHookShutdown complete\n");
}

/* =========================================================================
 * DllMain — minimal entry point
 *
 * DLL_PROCESS_ATTACH: create the worker thread; keep the handle and ID so
 *     that CefHookShutdown() can wait on it and detect self-calls.
 *
 * DLL_PROCESS_DETACH: set the stop flag and nothing else.
 *     We do NOT wait for threads, close handles, detach hooks, or call
 *     into other DLLs here — all of that is unsafe under the loader lock
 *     (or when other DLLs may already be unmapped during process exit).
 *     Clean unload is the caller's responsibility: call CefHookShutdown()
 *     BEFORE FreeLibrary().  On process exit the OS reclaims everything.
 * ========================================================================= */
BOOL APIENTRY DllMain(HMODULE hModule, DWORD dwReason, PVOID pvReserved) {
    (void)pvReserved;
    switch (dwReason) {
    case DLL_PROCESS_ATTACH:
        DisableThreadLibraryCalls(hModule);
        InterlockedExchange(&g_runtimeStopRequested, 0);
        InterlockedExchange(&g_shutdownState, 0);
        g_hWorkerThread = CreateThread(NULL, 0, ThreadProc, NULL, 0,
                                       &g_workerThreadId);
        if (!g_hWorkerThread) {
            OutputDebugStringA("[DLL] FATAL: CreateThread failed\n");
            return FALSE;
        }
        break;

    case DLL_PROCESS_DETACH:
        /* Only set the stop flag.  No waits, no handle closes, no external
         * calls.  CefHookShutdown() (if called before FreeLibrary) already
         * did the full cleanup; if not, the OS reclaims on process exit. */
        InterlockedExchange(&g_runtimeStopRequested, 1);
        break;
    }
    return TRUE;
}
