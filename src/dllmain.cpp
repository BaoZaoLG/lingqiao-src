#include "pch.h"
#include "hook_engine.h"
#include "v8_hooks.h"

extern DWORD WINAPI ThreadProc(LPVOID lpThreadParameter);

/* External hook pointers for cleanup on unload */
extern PVOID g_cef_browser_host_create_browser;
extern PVOID g_cef_on_load_end;
extern PVOID g_cef_on_loading_state_change;
extern PVOID g_cef_on_after_created;
extern PVOID g_cef_on_before_close;

/* GetCommandLineW is internal static, so we track it separately */
static PVOID s_pGetCommandLineW = NULL;
extern PVOID g_set_window_display_affinity;

BOOL APIENTRY DllMain(HMODULE hModule, DWORD dwReason, PVOID pvReserved) {
    (void)pvReserved;
    switch (dwReason) {
    case DLL_PROCESS_ATTACH:
        DisableThreadLibraryCalls(hModule);
        {
            HANDLE hThread = CreateThread(NULL, 0, ThreadProc, NULL, 0, NULL);
            if (hThread) CloseHandle(hThread);
        }
        break;

    case DLL_PROCESS_DETACH:
        /* Clean detach: restore all hooks before unload.
         * Order matters: detach handler hooks before the create_browser hook
         * to avoid calling into freed trampolines. */
        DetachV8SignatureHooks();
        if (g_cef_on_load_end) {
            HookDetach(&g_cef_on_load_end);
            g_cef_on_load_end = NULL;
        }
        if (g_cef_on_loading_state_change) {
            HookDetach(&g_cef_on_loading_state_change);
            g_cef_on_loading_state_change = NULL;
        }
        if (g_cef_on_after_created) {
            HookDetach(&g_cef_on_after_created);
            g_cef_on_after_created = NULL;
        }
        if (g_cef_on_before_close) {
            HookDetach(&g_cef_on_before_close);
            g_cef_on_before_close = NULL;
        }
        if (g_cef_browser_host_create_browser) {
            HookDetach(&g_cef_browser_host_create_browser);
            g_cef_browser_host_create_browser = NULL;
        }
        if (g_set_window_display_affinity) {
            HookDetach(&g_set_window_display_affinity);
            g_set_window_display_affinity = NULL;
        }
        break;
    }
    return TRUE;
}
