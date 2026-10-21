#pragma once
// Append --disable-gpu flags to GetCommandLineW (INIT_ONCE thread-safe).

#include <windows.h>

typedef LPWSTR(WINAPI* GetCommandLineW_t)(void);
extern GetCommandLineW_t g_orig_GetCommandLineW;

LPWSTR WINAPI MyGetCommandLineW(void);
