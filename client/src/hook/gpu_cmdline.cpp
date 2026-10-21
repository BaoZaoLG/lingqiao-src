#include "pch.h"

#include "gpu_cmdline.h"

GetCommandLineW_t g_orig_GetCommandLineW = NULL;

static wchar_t g_cmdLineBuf[32768] = {0};
static INIT_ONCE g_cmdLineInitOnce = INIT_ONCE_STATIC_INIT;

static BOOL CALLBACK InitCommandLineBuf(PINIT_ONCE once, PVOID param, PVOID* context) {
    (void)once; (void)param; (void)context;
    if (!g_orig_GetCommandLineW) return TRUE;

    LPWSTR orig = g_orig_GetCommandLineW();
    if (orig && wcsstr(orig, L"--disable-gpu") == NULL) {
        _snwprintf_s(g_cmdLineBuf, _countof(g_cmdLineBuf), _TRUNCATE,
                     L"%s --disable-gpu --disable-gpu-compositing --use-angle=warp",
                     orig);
    }
    return TRUE;
}

LPWSTR WINAPI MyGetCommandLineW(void) {
    InitOnceExecuteOnce(&g_cmdLineInitOnce, InitCommandLineBuf, NULL, NULL);
    return g_cmdLineBuf[0] ? g_cmdLineBuf : g_orig_GetCommandLineW();
}
