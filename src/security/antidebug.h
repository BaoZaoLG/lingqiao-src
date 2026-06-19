#pragma once
// ============================================================================
// Anti-debug & anti-tamper runtime checks — hardened edition
//
// Multiple layers of detection:
//   1. API-based (IsDebuggerPresent, CheckRemoteDebuggerPresent)
//   2. PEB flags (BeingDebugged, NtGlobalFlag)
//   3. NT API (NtQueryInformationProcess, ProcessDebugPort/DebugObjectHandle)
//   4. Hardware breakpoint detection (DR0-DR3)
//   5. Timing-based (RDTSC, QueryPerformanceCounter)
//   6. Thread hiding (NtSetInformationThread)
//   7. Parent process verification
//   8. Debugger window class enumeration
//   9. INT 2D / ICE breakpoint detection
// ============================================================================
#include <windows.h>

// Forward declarations for NT API
extern "C" {
typedef LONG NTSTATUS;
typedef NTSTATUS(NTAPI* pNtQueryInformationProcess)(
    HANDLE, UINT, PVOID, ULONG, PULONG);
typedef NTSTATUS(NTAPI* pNtSetInformationThread)(
    HANDLE, UINT, PVOID, ULONG);
typedef NTSTATUS(NTAPI* pNtQuerySystemInformation)(
    UINT, PVOID, ULONG, PULONG);
typedef NTSTATUS(NTAPI* pNtClose)(HANDLE);

#ifndef PPEB_DEFINED
#define PPEB_DEFINED
typedef struct _PEB_LITE {
    BYTE Reserved1[2];
    BYTE BeingDebugged;
    BYTE Reserved2[1];
    PVOID Reserved3[2];
    PVOID Ldr;            // PPEB_LDR_DATA
    PVOID ProcessParameters;
    BYTE Reserved4[104];
    PVOID Reserved5[52];
    PVOID PostProcessInitRoutine;
    BYTE Reserved6[128];
    PVOID Reserved7[1];
    ULONG SessionId;
} PEB_LITE;
typedef PEB_LITE* PPEB_LITE;
#endif

// Process Debug Object handle (undocumented)
#define ProcessDebugObjectHandle 0x1E
// Process Debug Flags
#define ProcessDebugFlags 0x1F

// Context flags for hardware breakpoint detection
typedef struct _CONTEXT_DEBUG {
    DWORD64 Dr0;
    DWORD64 Dr1;
    DWORD64 Dr2;
    DWORD64 Dr3;
    DWORD64 Dr6;
    DWORD64 Dr7;
} CONTEXT_DEBUG;
}

// ============================================================================
// Check 1: IsDebuggerPresent (basic, always include)
// ============================================================================
inline bool CheckDebuggerPresent() {
    return IsDebuggerPresent() != 0;
}

// ============================================================================
// Check 2: NtQueryInformationProcess (ProcessDebugPort)
// ============================================================================
inline bool CheckDebugPort() {
    HMODULE ntdll = GetModuleHandleW(L"ntdll.dll");
    if (!ntdll) return false;
    auto NtQIP = (pNtQueryInformationProcess)
        GetProcAddress(ntdll, "NtQueryInformationProcess");
    if (!NtQIP) return false;
    DWORD_PTR debugPort = 0;
    NTSTATUS status = NtQIP((HANDLE)-1, 7, &debugPort, sizeof(debugPort), NULL);
    return (status >= 0 && debugPort != 0);
}

// ============================================================================
// Check 3: CheckRemoteDebuggerPresent
// ============================================================================
inline bool CheckRemoteDebugger() {
    BOOL debugged = FALSE;
    CheckRemoteDebuggerPresent(GetCurrentProcess(), &debugged);
    return debugged != FALSE;
}

// ============================================================================
// Check 4: PEB BeingDebugged flag
// ============================================================================
inline bool CheckPEBDebugged() {
#ifdef _M_IX86
    PPEB_LITE peb = (PPEB_LITE)__readfsdword(0x30);
#elif defined(_M_AMD64)
    PPEB_LITE peb = (PPEB_LITE)__readgsqword(0x60);
#else
    return false;
#endif
    return peb->BeingDebugged != 0;
}

// ============================================================================
// Check 5: PEB NtGlobalFlag
// When a process is debugged, NtGlobalFlag has FLG_HEAP_ENABLE_TAIL_CHECK,
// FLG_HEAP_ENABLE_FREE_CHECK, and FLG_HEAP_VALIDATE_PARAMETERS set (0x70).
// ============================================================================
inline bool CheckNtGlobalFlag() {
#ifdef _M_IX86
    PPEB_LITE peb = (PPEB_LITE)__readfsdword(0x30);
    DWORD* pNtGlobalFlag = (DWORD*)((BYTE*)peb + 0x68);
#elif defined(_M_AMD64)
    PPEB_LITE peb = (PPEB_LITE)__readgsqword(0x60);
    DWORD* pNtGlobalFlag = (DWORD*)((BYTE*)peb + 0xBC);
#else
    return false;
#endif
    return (*pNtGlobalFlag & 0x70) != 0;
}

// ============================================================================
// Check 6: NtQueryInformationProcess (ProcessDebugObjectHandle)
// If a debug object exists, a debugger is attached.
// ============================================================================
inline bool CheckDebugObjectHandle() {
    HMODULE ntdll = GetModuleHandleW(L"ntdll.dll");
    if (!ntdll) return false;
    auto NtQIP = (pNtQueryInformationProcess)
        GetProcAddress(ntdll, "NtQueryInformationProcess");
    if (!NtQIP) return false;
    HANDLE debugObject = NULL;
    NTSTATUS status = NtQIP((HANDLE)-1, ProcessDebugObjectHandle,
                            &debugObject, sizeof(debugObject), NULL);
    return (status >= 0 && debugObject != NULL);
}

// ============================================================================
// Check 7: NtQueryInformationProcess (ProcessDebugFlags)
// DebugFlags is 0 when debugger is attached.
// ============================================================================
inline bool CheckDebugFlags() {
    HMODULE ntdll = GetModuleHandleW(L"ntdll.dll");
    if (!ntdll) return false;
    auto NtQIP = (pNtQueryInformationProcess)
        GetProcAddress(ntdll, "NtQueryInformationProcess");
    if (!NtQIP) return false;
    DWORD debugFlags = 1; // default: not debugged
    NtQIP((HANDLE)-1, ProcessDebugFlags, &debugFlags, sizeof(debugFlags), NULL);
    return debugFlags == 0;
}

// ============================================================================
// Check 8: Hardware breakpoint detection (DR0-DR3)
// Debug registers are set when hardware breakpoints are active.
// ============================================================================
inline bool CheckHardwareBreakpoints() {
#ifdef _M_AMD64
    CONTEXT ctx = {0};
    ctx.ContextFlags = CONTEXT_DEBUG_REGISTERS;
    if (GetThreadContext(GetCurrentThread(), &ctx)) {
        if (ctx.Dr0 || ctx.Dr1 || ctx.Dr2 || ctx.Dr3)
            return true;
    }
#elif defined(_M_IX86)
    // On x86, use GetThreadContext
    CONTEXT ctx = {0};
    ctx.ContextFlags = CONTEXT_DEBUG_REGISTERS;
    if (GetThreadContext(GetCurrentThread(), &ctx)) {
        if (ctx.Dr0 || ctx.Dr1 || ctx.Dr2 || ctx.Dr3)
            return true;
    }
#endif
    return false;
}

// ============================================================================
// Check 9: Timing-based detection (RDTSC)
// Single-stepping through instructions causes measurable delays.
// ============================================================================
inline bool CheckTimingRDTSC() {
    DWORD64 start, end;
#ifdef _M_AMD64
    start = __rdtsc();
    volatile int x = 0;
    for (int i = 0; i < 100; i++) x += i;
    end = __rdtsc();
#elif defined(_M_IX86)
    int cpuInfo[4];
    __cpuid(cpuInfo, 0);
    start = ((DWORD64)cpuInfo[3] << 32) | cpuInfo[1];
    volatile int x = 0;
    for (int i = 0; i < 100; i++) x += i;
    __cpuid(cpuInfo, 0);
    end = ((DWORD64)cpuInfo[3] << 32) | cpuInfo[1];
#else
    return false;
#endif
    // If execution took more than ~10000 cycles for 100 additions, something is off
    return (end - start) > 50000;
}

// ============================================================================
// Check 10: Timing-based detection (QueryPerformanceCounter)
// ============================================================================
inline bool CheckTimingQPC() {
    LARGE_INTEGER freq, start, end;
    QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&start);
    volatile int x = 0;
    for (int i = 0; i < 1000; i++) x += i;
    QueryPerformanceCounter(&end);
    double elapsed_ms = (double)(end.QuadPart - start.QuadPart) / freq.QuadPart * 1000.0;
    return elapsed_ms > 50.0;
}

// ============================================================================
// Check 11: NtSetInformationThread — hide thread from debugger
// ThreadHideFromDebugger makes the thread invisible to debuggers,
// causing breakpoints and exceptions to not be caught.
// ============================================================================
inline void HideThreadFromDebugger() {
    HMODULE ntdll = GetModuleHandleW(L"ntdll.dll");
    if (!ntdll) return;
    auto NtSIT = (pNtSetInformationThread)
        GetProcAddress(ntdll, "NtSetInformationThread");
    if (!NtSIT) return;
    // ThreadHideFromDebugger = 0x11
    ULONG hide = 1;
    NtSIT(GetCurrentThread(), 0x11, &hide, sizeof(hide));
}

// ============================================================================
// Check 12: Parent process verification
// Explorer.exe is the normal parent. If parent is a debugger, exit.
// ============================================================================
inline bool CheckParentProcess() {
    HMODULE ntdll = GetModuleHandleW(L"ntdll.dll");
    if (!ntdll) return false;

    // Get parent process ID from PEB
#ifdef _M_IX86
    PPEB_LITE peb = (PPEB_LITE)__readfsdword(0x30);
#elif defined(_M_AMD64)
    PPEB_LITE peb = (PPEB_LITE)__readgsqword(0x60);
#else
    return false;
#endif

    // PEB -> ProcessParameters -> offset for InheritedFromUniqueProcessId
    // This is at different offsets for x86/x64
#ifdef _M_AMD64
    BYTE* procParams = (BYTE*)peb->ProcessParameters;
    ULONG_PTR* pParentPid = (ULONG_PTR*)(procParams + 0x440); // x64 offset
#else
    BYTE* procParams = (BYTE*)peb->ProcessParameters;
    ULONG_PTR* pParentPid = (ULONG_PTR*)(procParams + 0x290); // x86 offset
#endif

    if (!pParentPid || *pParentPid == 0) return false;

    // Known debugger process names
    HANDLE hParent = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, (DWORD)*pParentPid);
    if (!hParent) return false;

    WCHAR path[MAX_PATH] = {0};
    DWORD size = MAX_PATH;
    bool isDebugger = false;

    if (QueryFullProcessImageNameW(hParent, 0, path, &size)) {
        // Extract filename
        WCHAR* fname = wcsrchr(path, L'\\');
        if (fname) fname++; else fname = path;
        _wcslwr_s(fname, MAX_PATH);

        // Check against known debugger names
        const WCHAR* debuggers[] = {
            L"ollydbg.exe", L"x64dbg.exe", L"x32dbg.exe",
            L"windbg.exe", L"ida.exe", L"ida64.exe",
            L"idaq.exe", L"idaq64.exe", L"immunitydebugger.exe",
            L"cheatengine.exe", L"processhacker.exe",
            L"de4dot.exe", L"dumper.exe", L"ilspy.exe",
            NULL
        };
        for (int i = 0; debuggers[i]; i++) {
            if (wcsstr(fname, debuggers[i])) {
                isDebugger = true;
                break;
            }
        }
    }
    CloseHandle(hParent);
    return isDebugger;
}

// ============================================================================
// Check 13: Debugger window class enumeration
// ============================================================================
inline BOOL CALLBACK EnumWindowsProc(HWND hwnd, LPARAM lParam) {
    WCHAR className[256] = {0};
    GetClassNameW(hwnd, className, 256);
    _wcslwr_s(className, 256);

    // Use exact class name matches to avoid false positives
    const WCHAR* debugClasses[] = {
        L"ollydbg", L"windbg_main_window",
        L"x64dbg", L"x32dbg",
        L"immunitydebugger", L"cheatengine",
        NULL
    };
    for (int i = 0; debugClasses[i]; i++) {
        if (wcscmp(className, debugClasses[i]) == 0) {
            *(bool*)lParam = true;
            return FALSE; // stop enumeration
        }
    }
    return TRUE;
}

inline bool CheckDebuggerWindows() {
    bool found = false;
    EnumWindows(EnumWindowsProc, (LPARAM)&found);
    return found;
}

// ============================================================================
// Check 14: INT 2D detection
// INT 2D is a kernel debug interrupt. When no kernel debugger is present,
// it causes an exception that can be caught. Debuggers may handle it differently.
// ============================================================================
inline bool CheckInt2D() {
#ifdef _M_IX86
    __try {
        __asm {
            int 0x2D
            nop
        }
    }
    __except (EXCEPTION_EXECUTE_HANDLER) {
        return false; // normal: exception was caught
    }
    // If we get here without exception, a kernel debugger may be present
    return true;
#else
    return false;
#endif
}

// ============================================================================
// Master check: run all anti-debug checks
// ============================================================================
inline bool IsBeingDebugged() {
    // Layer 1: Quick API checks
    if (CheckDebuggerPresent()) return true;
    if (CheckDebugPort()) return true;
    if (CheckRemoteDebugger()) return true;

    // Layer 2: PEB-based checks
    if (CheckPEBDebugged()) return true;
    if (CheckNtGlobalFlag()) return true;

    // Layer 3: NT API deep checks
    if (CheckDebugObjectHandle()) return true;
    if (CheckDebugFlags()) return true;

    // Layer 4: Hardware breakpoints
    if (CheckHardwareBreakpoints()) return true;

    // Layer 5: Timing checks — DISABLED: false positives on slow/virtualized systems
    // if (CheckTimingRDTSC()) return true;
    // if (CheckTimingQPC()) return true;

    // Layer 6: Parent process — DISABLED: offset-based, fragile across Windows versions
    // if (CheckParentProcess()) return true;

    // Layer 7: Window enumeration (exact match only)
    if (CheckDebuggerWindows()) return true;

    return false;
}
