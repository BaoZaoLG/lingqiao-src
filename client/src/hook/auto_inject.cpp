/* ============================================================================
 * auto_inject.cpp — Automatic renderer process injection
 *
 * Hooks CreateProcessW in the browser process to detect child renderer
 * process creation.  When a new renderer is spawned, CefHook.dll is
 * injected via the classic VirtualAllocEx + CreateRemoteThread(LoadLibraryW)
 * technique before the original CreateProcessW returns.
 *
 * Safety:
 *   - Targets Chromium renderer children and same-executable CEF browser
 *     children created by the hooked host process.
 *   - Injection failure is non-fatal (child still launches normally)
 *   - The hook is a standard MinHook inline hook — fully thread-safe
 * ========================================================================= */

#include "pch.h"
#include "auto_inject.h"
#include "hook_engine.h"

/* ---- Original CreateProcessW (trampoline after hook) ---- */
typedef BOOL (WINAPI *CreateProcessW_t)(
    LPCWSTR, LPWSTR, LPSECURITY_ATTRIBUTES, LPSECURITY_ATTRIBUTES,
    BOOL, DWORD, LPVOID, LPCWSTR, LPSTARTUPINFOW, LPPROCESS_INFORMATION);

static CreateProcessW_t g_orig_CreateProcessW = NULL;

/* ---- Cached path to our own DLL ---- */
static wchar_t g_dllPath[MAX_PATH] = {0};
static wchar_t g_hostExePath[MAX_PATH] = {0};

static const wchar_t* BaseName(const wchar_t* path) {
    if (!path) return L"";
    const wchar_t* slash = wcsrchr(path, L'\\');
    const wchar_t* slash2 = wcsrchr(path, L'/');
    if (slash2 && (!slash || slash2 > slash)) slash = slash2;
    return slash ? slash + 1 : path;
}

static bool SameHostExecutable(const wchar_t* applicationName,
                               const wchar_t* commandLine) {
    const wchar_t* hostBase = BaseName(g_hostExePath);
    if (!*hostBase) return false;

    if (applicationName && *applicationName) {
        if (_wcsicmp(applicationName, g_hostExePath) == 0 ||
            _wcsicmp(BaseName(applicationName), hostBase) == 0) {
            return true;
        }
    }

    if (!commandLine || !*commandLine) return false;
    const wchar_t* p = commandLine;
    while (*p == L' ' || *p == L'\t') ++p;
    wchar_t token[MAX_PATH] = {0};
    size_t n = 0;
    if (*p == L'\"') {
        ++p;
        while (*p && *p != L'\"' && n + 1 < _countof(token)) token[n++] = *p++;
    } else {
        while (*p && *p != L' ' && *p != L'\t' && n + 1 < _countof(token)) token[n++] = *p++;
    }
    token[n] = L'\0';
    return n > 0 && (_wcsicmp(token, g_hostExePath) == 0 ||
                     _wcsicmp(BaseName(token), hostBase) == 0);
}

static bool IsNonBrowserCefChild(const wchar_t* commandLine) {
    if (!commandLine) return false;
    static const wchar_t* kNonBrowserTypes[] = {
        L"--type=gpu-process",
        L"--type=utility",
        L"--type=crashpad-handler",
        L"--type=zygote",
        L"--type=sandbox-helper"
    };
    for (size_t i = 0; i < _countof(kNonBrowserTypes); ++i) {
        if (wcsstr(commandLine, kNonBrowserTypes[i])) return true;
    }
    return false;
}

/* ============================================================================
 * CreateProcessW hook
 * ========================================================================= */
static BOOL WINAPI MyCreateProcessW(
    LPCWSTR lpApplicationName,
    LPWSTR  lpCommandLine,
    LPSECURITY_ATTRIBUTES lpProcessAttributes,
    LPSECURITY_ATTRIBUTES lpThreadAttributes,
    BOOL    bInheritHandles,
    DWORD   dwCreationFlags,
    LPVOID  lpEnvironment,
    LPCWSTR lpCurrentDirectory,
    LPSTARTUPINFOW lpStartupInfo,
    LPPROCESS_INFORMATION lpProcessInformation)
{
    /* ---- Step 1: Detect renderer processes and force CREATE_SUSPENDED ---- */
    bool isRenderer = false;
    DWORD originalFlags = dwCreationFlags;

    if (lpCommandLine && wcsstr(lpCommandLine, L"--type=renderer")) {
        isRenderer = true;
        /* Force suspended creation so we can inject before the process runs */
        dwCreationFlags |= CREATE_SUSPENDED;
    }

    /* Some Chaoxing builds create an additional CEF browser process with no
     * --type= argument.  It is still part of the same CXexam executable and
     * owns the browser/frame where the exam page is rendered. */
    bool isHostChild = SameHostExecutable(lpApplicationName, lpCommandLine);
    bool isBrowserChild = isHostChild && !IsNonBrowserCefChild(lpCommandLine);
    bool shouldInject = isRenderer || isBrowserChild;
    if (shouldInject) {
        dwCreationFlags |= CREATE_SUSPENDED;
    }

    /* ---- Step 2: Call the original CreateProcessW ---- */
    BOOL result = g_orig_CreateProcessW(
        lpApplicationName, lpCommandLine,
        lpProcessAttributes, lpThreadAttributes,
        bInheritHandles, dwCreationFlags, lpEnvironment,
        lpCurrentDirectory, lpStartupInfo, lpProcessInformation);

    if (!result || !lpProcessInformation ||
        !lpProcessInformation->hProcess ||
        !lpProcessInformation->hThread ||
        !lpProcessInformation->dwProcessId)
    {
        return result;
    }

    /* ---- Step 3: Inject if this is a renderer ---- */
    if (shouldInject) {
        HANDLE hProcess = lpProcessInformation->hProcess;
        HANDLE hThread  = lpProcessInformation->hThread;
        DWORD  pid      = lpProcessInformation->dwProcessId;
        bool   injected = false;

        /* Allocate memory in child and write our DLL path */
        size_t pathBytes = (wcslen(g_dllPath) + 1) * sizeof(wchar_t);
        LPVOID remoteBuf = VirtualAllocEx(hProcess, NULL, pathBytes,
                                          MEM_COMMIT | MEM_RESERVE,
                                          PAGE_READWRITE);
        bool canFreeRemoteBuf = true;

        if (remoteBuf) {
            SIZE_T bytesWritten = 0;
            BOOL writeOk = WriteProcessMemory(hProcess, remoteBuf,
                                              g_dllPath, pathBytes,
                                              &bytesWritten);
            if (writeOk && bytesWritten == pathBytes) {
                /* Resolve LoadLibraryW in kernel32 */
                FARPROC loadLib = GetProcAddress(
                    GetModuleHandleA("kernel32.dll"), "LoadLibraryW");

                if (loadLib) {
                    HANDLE hRemoteThread = CreateRemoteThread(
                        hProcess, NULL, 0,
                        (LPTHREAD_START_ROUTINE)loadLib,
                        remoteBuf, 0, NULL);

                    if (hRemoteThread) {
                        /* Wait for LoadLibraryW to complete (4s timeout) */
                        DWORD waitResult = WaitForSingleObject(hRemoteThread, 4000);

                        DWORD exitCode = 0;
                        GetExitCodeThread(hRemoteThread, &exitCode);
                        CloseHandle(hRemoteThread);

                        if (waitResult == WAIT_OBJECT_0 && exitCode != 0) {
                            char dbg[256];
                            sprintf_s(dbg,
                                "[INJECT] CefHook.dll auto-injected into "
                                "%s child pid=%lu\n",
                                isRenderer ? "renderer" : "browser", pid);
                            OutputDebugStringA(dbg);
                            injected = true;
                        } else if (waitResult == WAIT_TIMEOUT) {
                            /* TIMEOUT: remote thread may still be reading
                             * remoteBuf. Do NOT free it — intentional leak
                             * to avoid use-after-free in the child. */
                            canFreeRemoteBuf = false;
                            char dbg[256];
                            sprintf_s(dbg,
                                "[INJECT] LoadLibraryW timed out for "
                                "renderer pid=%lu — remoteBuf leaked "
                                "(%zu bytes) to avoid race\n",
                                pid, pathBytes);
                            OutputDebugStringA(dbg);
                        } else {
                            char dbg[256];
                            sprintf_s(dbg,
                                "[INJECT] Remote LoadLibraryW returned NULL "
                                "for renderer pid=%lu exit=%lu\n",
                                pid, exitCode);
                            OutputDebugStringA(dbg);
                        }
                    } else {
                        char dbg[256];
                        sprintf_s(dbg,
                            "[INJECT] CreateRemoteThread failed for "
                            "renderer pid=%lu err=%lu\n",
                            pid, GetLastError());
                        OutputDebugStringA(dbg);
                    }
                }
            } else {
                char dbg[256];
                sprintf_s(dbg,
                    "[INJECT] WriteProcessMemory failed for "
                    "renderer pid=%lu err=%lu written=%zu\n",
                    pid, GetLastError(), bytesWritten);
                OutputDebugStringA(dbg);
            }

            /* Free remote buffer only if safe (no timeout race) */
            if (canFreeRemoteBuf) {
                VirtualFreeEx(hProcess, remoteBuf, 0, MEM_RELEASE);
            }
        } else {
            char dbg[256];
            sprintf_s(dbg,
                "[INJECT] VirtualAllocEx failed for "
                "renderer pid=%lu err=%lu\n",
                pid, GetLastError());
            OutputDebugStringA(dbg);
        }

        /* ---- Step 4: Resume the renderer main thread ----
         * Only resume if the original caller did NOT request CREATE_SUSPENDED.
         * If they did, leave the thread suspended as they intended. */
        if (!(originalFlags & CREATE_SUSPENDED)) {
            ResumeThread(hThread);
        }

        if (!injected) {
            OutputDebugStringA("[INJECT] Injection failed — child will run "
                               "without CefHook.dll\n");
        }
    }

    return result;
}

/* ============================================================================
 * Public API
 * ========================================================================= */

bool InstallAutoInjectHook()
{
    /* ---- Resolve our own DLL path ---- */
    HMODULE hSelf = NULL;
    GetModuleHandleExW(
        GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
        GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
        (LPCWSTR)InstallAutoInjectHook,   /* address inside our DLL */
        &hSelf);

    if (!hSelf) {
        OutputDebugStringA("[INJECT] FATAL: cannot resolve own DLL handle\n");
        return false;
    }
    GetModuleFileNameW(hSelf, g_dllPath, MAX_PATH);
    GetModuleFileNameW(NULL, g_hostExePath, MAX_PATH);

    /* ---- Hook CreateProcessW ---- */
    g_orig_CreateProcessW = (CreateProcessW_t)
        HookFindFunction("kernel32.dll", "CreateProcessW");
    if (!g_orig_CreateProcessW) {
        OutputDebugStringA("[INJECT] CreateProcessW not found\n");
        return false;
    }

    LONG r = HookAttach((PVOID*)&g_orig_CreateProcessW,
                        (PVOID)MyCreateProcessW);
    if (r != ERROR_SUCCESS) {
        char dbg[128];
        sprintf_s(dbg, "[INJECT] CreateProcessW hook failed: %ld\n", r);
        OutputDebugStringA(dbg);
        return false;
    }

    OutputDebugStringA("[INJECT] Auto-inject hook installed — will inject "
                       "CefHook.dll into renderer and same-exe browser "
                       "children automatically\n");

    char pathDbg[MAX_PATH + 64];
    sprintf_s(pathDbg, "[INJECT] DLL path: %ls\n", g_dllPath);
    OutputDebugStringA(pathDbg);

    return true;
}

void UninstallAutoInjectHook()
{
    if (g_orig_CreateProcessW) {
        HookDetach((PVOID*)&g_orig_CreateProcessW);
        g_orig_CreateProcessW = NULL;
        OutputDebugStringA("[INJECT] Auto-inject hook removed\n");
    }
}
