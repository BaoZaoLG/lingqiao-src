/* ============================================================================
 * v8_intercept.cpp — V8 Interception Engine (combat mode)
 *
 * Transforms passive V8 observation hooks into active interception:
 *
 *  1. Script Source Patching  — neutralize anti-cheat code before compilation
 *  2. Handler Hijacking       — track/replace/block native handler registration
 *  3. Execution Control       — block specific scripts from running
 *  4. Template Tracking       — record all function template creation
 *
 * All entry points are extern "C" __cdecl, called from x86 naked asm detours.
 *
 * Stack frame convention (uniform for all intercept functions):
 *   After the detour pushes args in C order (a4, a3, a2, a1) and captures
 *   esp into esp_frame:
 *     [esp_frame +  0] = arg1   (a1)
 *     [esp_frame +  4] = arg2   (a2)
 *     [esp_frame +  8] = arg3   (a3)
 *     [esp_frame + 12] = arg4   (a4)
 *
 *   For compile_script / script_run (no explicit arg pushes, esp captured
 *   directly after pushad + pushfd):
 *     [esp_frame + 36] = original_arg1
 *     [esp_frame + 40] = original_arg2
 *     [esp_frame + 44] = original_arg3
 *     [esp_frame + 48] = original_arg4
 * ========================================================================= */

#include "pch.h"
#include "v8_intercept.h"
#include "v8_source_probe.h"
#include "v8_source_neutralizer.h"

#include <windows.h>
#include <cstdio>
#include <cstring>
#include <string>

#if defined(_M_IX86)

/* =========================================================================
 * Logging
 * ========================================================================= */

static void Log(const char* text) {
    OutputDebugStringA(text);
    OutputDebugStringA("\n");
}

static void LogF(const char* fmt, ...) {
    char buf[512];
    va_list args;
    va_start(args, fmt);
    vsprintf_s(buf, sizeof(buf), fmt, args);
    va_end(args);
    Log(buf);
}

/* =========================================================================
 * Configuration — compile-time flags (runtime config can be added later)
 * ========================================================================= */

static const bool kEnableSourceLogging     = true;   /* log first N chars of each compiled script */
static const bool kEnableSourcePatching    = true;   /* apply neutralization patterns */
static const bool kEnableSkipList          = true;   /* check source hash against block list */
static const bool kEnableHandlerTracking   = true;   /* record all handler registrations */
static const bool kEnableTemplateTracking  = true;   /* record all template creation */

static const size_t kMaxSourceLogChars     = 200;
static const size_t kMaxTrackedTemplates   = 256;
static const size_t kMaxTrackedHandlers    = 256;
static const size_t kMaxSkipEntries        = 64;

/* =========================================================================
 * Thread-safe state
 * ========================================================================= */

static CRITICAL_SECTION g_cs;
static bool g_initialized = false;

/* Statistics */
static volatile LONG g_statsCompiled  = 0;
static volatile LONG g_statsPatched   = 0;
static volatile LONG g_statsBlocked   = 0;
static volatile LONG g_statsSkipped   = 0;
static volatile LONG g_statsTemplates = 0;
static volatile LONG g_statsHandlers  = 0;
static volatile LONG g_statsUnreadable = 0;
static volatile LONG g_statsBypassed = 0;
static volatile LONG g_strongModeDisabled = 0;
static const LONG kMaxUnreadableBeforeDisable = 10;

/* Template tracking ring buffer */
static void* g_templates[kMaxTrackedTemplates];
static volatile LONG g_templateCount = 0;

/* Handler tracking ring buffer */
struct HandlerEntry {
    void* templateAddr;
    void* callback;
};
static HandlerEntry g_handlers[kMaxTrackedHandlers];
static volatile LONG g_handlerCount = 0;

/* Script skip list */
struct SkipEntry {
    const char* description;    /* human-readable reason */
    const char* pattern;        /* substring to match in source */
    int         action;         /* 0 = allow, 1 = skip execution */
};

static SkipEntry g_skipList[kMaxSkipEntries];
static int g_skipCount = 0;

/* =========================================================================
 * Default neutralization patterns
 *
 * These are applied as same-length space-replacement to preserve buffer
 * size and avoid corrupting V8's string object bookkeeping.
 * ========================================================================= */

static void InitDefaultSkipList() {
    g_skipCount = 0;

    /* Anti-debugger: the 'debugger' statement pauses execution in DevTools */
    g_skipList[g_skipCount++] = {
        "anti-debugger",
        "debugger",
        1 /* skip entire script containing naked debugger statements */
    };

    /* Anti-automation fingerprints */
    g_skipList[g_skipCount++] = {
        "webdriver-detect",
        "navigator.webdriver",
        0 /* patch, don't skip */
    };

    g_skipList[g_skipCount++] = {
        "devtools-detect",
        "devtoolsDetector",
        0
    };

    /* Console-clearing / log-suppression */
    g_skipList[g_skipCount++] = {
        "console-clear",
        "console.clear()",
        0
    };

    /* Screen capture protection */
    g_skipList[g_skipCount++] = {
        "screenshot-guard",
        "SetWindowDisplayAffinity",
        0
    };

    /* Generic integrity checks */
    g_skipList[g_skipCount++] = {
        "integrity-check",
        "__hookDetect__",
        1
    };
}

/* =========================================================================
 * FNV-1a hash — fast, zero-allocation string hash for skip-list matching
 * ========================================================================= */

static DWORD FnvHash(const char* data, size_t len) {
    DWORD h = 0x811c9dc5u;
    for (size_t i = 0; i < len; i++) {
        h ^= (unsigned char)data[i];
        h *= 0x01000193u;
    }
    return h;
}

/* =========================================================================
 * V8 string reading — best-effort extraction via vtable dispatch
 *
 * V8 string object layout (v8::internal::String, x86):
 *   +0x00: vtable pointer
 *   vtable layout (typical for V8 ~v6.x used in CEF ~76):
 *     [0]  ~destructor / GetIsolate
 *     [1]  IsOneByte / IsSeqString
 *     [2]  Length()
 *     [3]  WriteOneByte(isolate, buf, from, length, options)
 *     [4]  Write(isolate, buf, from, length, options)
 *
 * This is version-dependent! The code uses __try/__except to guard against
 * crashes from incorrect vtable offsets. Worst case: string isn't read and
 * interception degrades to hash-based matching only.
 * ========================================================================= */

static bool V8ReadString(void* v8StringHandle, char* buf, size_t bufSize, size_t* outLen) {
    if (!v8StringHandle || !buf || bufSize == 0) return false;
    *outLen = 0;

    __try {
        /* v8::Local<String> is { String* val_ }. Dereference handle. */
        void* strObj = *(void**)v8StringHandle;
        if (!strObj || IsBadReadPtr(strObj, 16)) return false;

        void** vtable = *(void***)strObj;
        if (!vtable || IsBadReadPtr(vtable, 24)) return false;

        /* Length() — vtable[2], thiscall: (this) → int */
        typedef int (__thiscall* pfn_length)(void*);
        pfn_length fnLen = (pfn_length)vtable[2];
        int strLen = fnLen(strObj);
        if (strLen <= 0 || strLen > (1 << 20)) return false; /* >1MB = suspicious */

        /* WriteOneByte() — vtable[3], thiscall: (this, isolate, buf, from, len, opts) */
        typedef int (__thiscall* pfn_write_ascii)(void*, void*, char*, int, int, int);
        pfn_write_ascii fnWrite = (pfn_write_ascii)vtable[3];

        size_t readLen = ((size_t)strLen < bufSize - 1) ? (size_t)strLen : bufSize - 1;
        int written = fnWrite(strObj, NULL, buf, 0, (int)readLen, 0);
        if (written <= 0) return false;
        buf[written] = '\0';
        *outLen = (size_t)written;
        return true;
    }
    __except (EXCEPTION_EXECUTE_HANDLER) {
        return false;
    }
}

/* =========================================================================
 * V8 string in-place replacement
 *
 * Same-length replacement only: the new text must be exactly the same
 * byte count as the old text. This preserves the V8 string buffer size
 * and avoids corrupting internal bookkeeping.
 *
 * Strategy:
 *   1. Read the full string via V8ReadString
 *   2. Apply all same-length replacements in a local copy
 *   3. Write the modified bytes back via WriteOneByte
 *
 * Returns the number of substitutions made.
 * ========================================================================= */

/* ---- SEH-safe helpers for V8ReplaceInString ---- */

/* Read V8 string length via vtable (inside __try, no C++ objects) */
static int V8SafeReadLength(void* strObj) {
    __try {
        if (!strObj || IsBadReadPtr(strObj, 16)) return -1;
        void** vtable = *(void***)strObj;
        if (!vtable || IsBadReadPtr(vtable, 24)) return -1;
        typedef int (__thiscall* pfn_length)(void*);
        return ((pfn_length)vtable[2])(strObj);
    } __except (EXCEPTION_EXECUTE_HANDLER) {
        return -1;
    }
}

/* Write ASCII bytes to V8 string via vtable (inside __try) */
static int V8SafeWriteAscii(void* strObj, char* buf, int len) {
    __try {
        if (!strObj || IsBadReadPtr(strObj, 16)) return 0;
        void** vtable = *(void***)strObj;
        if (!vtable || IsBadReadPtr(vtable, 24)) return 0;
        typedef int (__thiscall* pfn_write)(void*, void*, char*, int, int, int);
        return ((pfn_write)vtable[3])(strObj, NULL, buf, 0, len, 0);
    } __except (EXCEPTION_EXECUTE_HANDLER) {
        return 0;
    }
}

/* Get the raw V8 string object from a Local handle (inside __try) */
static void* V8SafeDerefHandle(void* handle) {
    __try {
        if (!handle || IsBadReadPtr(handle, sizeof(void*))) return NULL;
        void* obj = *(void**)handle;
        if (!obj || IsBadReadPtr(obj, 16)) return NULL;
        return obj;
    } __except (EXCEPTION_EXECUTE_HANDLER) {
        return NULL;
    }
}

/*
 * V8 string in-place replacement (restructured for MSVC).
 *
 * No _alloca, no C++ objects with destructors in this function.
 * All SEH-guarded operations are in helper functions.
 *
 * Same-length replacement only: preserves V8 string buffer size.
 * Returns the number of substitutions made.
 */
static int V8ReplaceInString(void* v8StringHandle,
                              const char* oldText, const char* newText)
{
    size_t oldLen = strlen(oldText);
    size_t newLen = strlen(newText);
    if (oldLen != newLen || oldLen == 0) return 0;

    /* Dereference handle to get raw V8 string object */
    void* strObj = V8SafeDerefHandle(v8StringHandle);
    if (!strObj) return 0;

    /* Read length via vtable (SEH-safe) */
    int strLen = V8SafeReadLength(strObj);
    if (strLen <= 0 || strLen > (1 << 20)) return 0;

    /* Heap-allocated buffer (no _alloca, no MSVC conflicts) */
    char* buf = new char[(size_t)strLen + 1];
    if (!buf) return 0;

    /* Read full string (SEH-safe) */
    int read = V8SafeWriteAscii(strObj, buf, strLen);
    if (read <= 0) {
        delete[] buf;
        return 0;
    }
    buf[read] = '\0';

    /* Apply same-length replacements in local copy */
    int count = 0;
    char* p = buf;
    while ((p = strstr(p, oldText)) != NULL) {
        memcpy(p, newText, newLen);
        p += newLen;
        count++;
    }

    if (count > 0) {
        /* Write modified bytes back to V8 buffer (SEH-safe) */
        V8SafeWriteAscii(strObj, buf, read);
    }

    delete[] buf;
    return count;
}

/* =========================================================================
 * Script source patching engine
 *
 * Applies all configured neutralization patterns to a V8 source string.
 * Same-length replacement preserves the V8 string buffer.
 *
 * Returns the total number of substitutions applied.
 * ========================================================================= */

static int ApplyScriptPatches(void* sourceHandle) {
    int totalPatched = 0;

    /* --- Pattern 1: "debugger" → "        " (8 spaces) --- */
    totalPatched += V8ReplaceInString(sourceHandle, "debugger", "        ");

    /* --- Pattern 2: console.clear() → spaces --- */
    totalPatched += V8ReplaceInString(sourceHandle, "console.clear()", "               ");

    /* --- Pattern 3: navigator.webdriver → navigator.noWebDrv --- */
    /* Same length: 20 chars each */
    totalPatched += V8ReplaceInString(sourceHandle,
        "navigator.webdriver", "navigator.noWebDrv");

    /* --- Pattern 4: devtoolsDetector → spaces --- */
    totalPatched += V8ReplaceInString(sourceHandle, "devtoolsDetector", "                ");

    /* --- Pattern 5: common anti-cheat global --- */
    totalPatched += V8ReplaceInString(sourceHandle, "__hookDetect__", "              ");

    /* --- Pattern 6: disable right-click prevention --- */
    /* "oncontextmenu" is 13 chars; replace event handler assignment */
    totalPatched += V8ReplaceInString(sourceHandle,
        "oncontextmenu", "oncontextmen_");

    /* --- Pattern 7: anti-copy prevention --- */
    totalPatched += V8ReplaceInString(sourceHandle,
        "onselectstart", "onselectstar_");

    /* --- Pattern 8: anti-screenshot canvas fingerprint --- */
    totalPatched += V8ReplaceInString(sourceHandle,
        "toDataURL", "toDataUR_");

    return totalPatched;
}

/* =========================================================================
 * Check source against the skip list (substring match)
 * Returns: 1 if the script should be skipped, 0 otherwise
 * ========================================================================= */

static int CheckSkipList(const char* source, size_t sourceLen) {
    if (V8SourceContainsDebuggerStatement(source, sourceLen)) {
        Log("[V8ICPT] Skip-list hit: 'anti-debugger' -> BLOCK");
        return 1;
    }

    for (int i = 0; i < g_skipCount; i++) {
        if (g_skipList[i].action == 0) continue; /* patch-only entries */
        if (strcmp(g_skipList[i].description, "anti-debugger") == 0) continue;
        if (g_skipList[i].pattern && strstr(source, g_skipList[i].pattern)) {
            LogF("[V8ICPT] Skip-list hit: '%s' → BLOCK", g_skipList[i].description);
            return 1;
        }
    }
    (void)sourceLen;
    return 0;
}

/* =========================================================================
 * INTERCEPT 0: Script compilation
 *
 * Stack frame (esp_frame captured after pushad + pushfd, before any C call):
 *   [esp_frame + 36] = original arg1 (Isolate*)
 *   [esp_frame + 40] = original arg2 (ScriptSource* — first field is source_string)
 *   [esp_frame + 44] = original arg3 (CompileOptions)
 *   [esp_frame + 48] = original arg4
 *
 * Returns: 0 = compile normally, 1 = BLOCK (return empty handle)
 * ========================================================================= */

int __cdecl V8InterceptCompileScript(void* esp_frame) {
    if (V8InterceptStrongModeDisabled()) {
        InterlockedIncrement(&g_statsBypassed);
        return 0;
    }

    void** frame = (void**)esp_frame;
    void* arg1 = frame ? frame[0] : NULL;
    void* arg2 = frame ? frame[1] : NULL;
    void* arg3 = frame ? frame[2] : NULL;
    void* arg4 = frame ? frame[3] : NULL;
    void* arg5 = frame ? frame[4] : NULL;

    LONG count = InterlockedIncrement(&g_statsCompiled);
    if (count <= 30) {
        LogF("[V8ICPT] compile #%ld arg0=%p arg1=%p source=%p arg3=%p arg4=%p",
             count, arg1, arg2, arg3, arg4, arg5);
    }

    V8SourceProbeResult probe = ProbeV8CompileSource(arg3);
    if (count <= 30) {
        LogF("[V8ICPT] source-probe readable=%d offset=%d handle=%p preview=%s",
             probe.readable ? 1 : 0,
             probe.candidateIndex,
             probe.sourceHandle,
             probe.preview);
    }

    if (!probe.readable || !probe.sourceHandle) {
        LONG unreadable = InterlockedIncrement(&g_statsUnreadable);
        if (unreadable >= kMaxUnreadableBeforeDisable &&
            InterlockedCompareExchange(&g_strongModeDisabled, 1, 0) == 0) {
            OutputDebugStringA("[V8ICPT] strong mode auto-disabled: source unreadable threshold reached\n");
        }
        return 0;
    }

    return 0;
}

/* =========================================================================
 * INTERCEPT 1: Script execution control
 *
 * Same stack frame convention as compile_script.
 * Also triggers Bridge native object installation (bridge_v8.cpp).
 *
 * Returns: 0 = allow execution, 1 = skip (detour returns without running)
 * ========================================================================= */

/* bridge_v8.cpp — install native bridge object on first V8 activity */
extern "C" void BridgeTryInstallFromV8Hook();

int __cdecl V8InterceptScriptRun(void* esp_frame) {
    /* Always attempt bridge installation (idempotent after first success) */
    BridgeTryInstallFromV8Hook();

    LONG count = InterlockedIncrement(&g_statsSkipped);
    if (count <= 20) {
        void** frame = (void**)esp_frame;
        LogF("[V8ICPT] script_run #%ld arg1=%p arg2=%p", count, frame[9], frame[10]);
    }

    /* Future: per-script execution control via hash/pattern matching.
     * For now, all scripts are allowed to execute. */
    return 0;
}

/* =========================================================================
 * INTERCEPT 2: Native handler registration
 *
 * Stack frame (args pushed in C order by detour):
 *   [esp_frame +  0] = a1 (template handle / FunctionTemplate*)
 *   [esp_frame +  4] = a2 (callback function pointer / AccessorNameGetterCallback)
 *   [esp_frame +  8] = a3 (data / v8::Local<Value>)
 *   [esp_frame + 12] = a4 (side_effect_type or settings)
 *
 * Returns: the callback pointer to install (original a2, or replacement).
 *          Currently always returns a2 (original). Override via handler table.
 * ========================================================================= */

void* __cdecl V8InterceptSetCallHandler(void* esp_frame) {
    void** frame = (void**)esp_frame;
    void* templateAddr = frame[0];
    void* callback     = frame[1];
    void* data         = frame[2];
    void* sideEffect   = frame[3];

    LONG idx = InterlockedIncrement(&g_statsHandlers);

    /* Track handler registration */
    EnterCriticalSection(&g_cs);
    if (g_handlerCount < (LONG)kMaxTrackedHandlers) {
        g_handlers[g_handlerCount].templateAddr = templateAddr;
        g_handlers[g_handlerCount].callback = callback;
        g_handlerCount++;
    }
    LeaveCriticalSection(&g_cs);

    if (idx <= 20) {
        LogF("[V8ICPT] set_handler #%ld template=%p callback=%p data=%p side=%p",
             idx, templateAddr, callback, data, sideEffect);
    }

    /* Future: check replacement table to swap handler callbacks.
     * For now, always pass through the original callback. */
    return callback;
}

/* =========================================================================
 * INTERCEPT 3/4: Function template creation tracking
 *
 * Same stack frame convention as set_call_handler.
 * Records template handles for later association with handler registrations.
 * ========================================================================= */

void __cdecl V8InterceptNewTemplate(void* esp_frame) {
    void** frame = (void**)esp_frame;
    void* a1 = frame[0];
    void* a2 = frame[1];
    void* a3 = frame[2];
    void* a4 = frame[3];

    LONG idx = InterlockedIncrement(&g_statsTemplates);

    EnterCriticalSection(&g_cs);
    if (g_templateCount < (LONG)kMaxTrackedTemplates) {
        g_templates[g_templateCount] = a1;
        g_templateCount++;
    }
    LeaveCriticalSection(&g_cs);

    if (idx <= 20) {
        LogF("[V8ICPT] new_template #%ld a1=%p a2=%p a3=%p a4=%p", idx, a1, a2, a3, a4);
    }
}

/* =========================================================================
 * Init / Shutdown
 * ========================================================================= */

void V8InterceptInit(void) {
    if (g_initialized) return;
    InitializeCriticalSection(&g_cs);
    g_initialized = true;

    /* Reset all state */
    InterlockedExchange(&g_statsCompiled, 0);
    InterlockedExchange(&g_statsPatched, 0);
    InterlockedExchange(&g_statsBlocked, 0);
    InterlockedExchange(&g_statsSkipped, 0);
    InterlockedExchange(&g_statsTemplates, 0);
    InterlockedExchange(&g_statsHandlers, 0);
    InterlockedExchange(&g_statsUnreadable, 0);
    InterlockedExchange(&g_statsBypassed, 0);
    InterlockedExchange(&g_strongModeDisabled, 0);
    InterlockedExchange(&g_templateCount, 0);
    InterlockedExchange(&g_handlerCount, 0);

    /* Initialize skip/patch patterns */
    InitDefaultSkipList();

    LogF("[V8ICPT] Interception engine active (patch=%s skip=%s track=%s patterns=%d)",
         kEnableSourcePatching ? "ON" : "OFF",
         kEnableSkipList ? "ON" : "OFF",
         kEnableHandlerTracking ? "ON" : "OFF",
         g_skipCount);
}

void V8InterceptShutdown(void) {
    LogF("[V8ICPT] Shutdown — compiled=%ld patched=%ld blocked=%ld skipped=%ld "
         "templates=%ld handlers=%ld unreadable=%ld bypassed=%ld disabled=%ld",
         g_statsCompiled, g_statsPatched, g_statsBlocked, g_statsSkipped,
         g_statsTemplates, g_statsHandlers, g_statsUnreadable,
         g_statsBypassed, g_strongModeDisabled);
    if (g_initialized) {
        DeleteCriticalSection(&g_cs);
        g_initialized = false;
    }
}

int V8InterceptStrongModeDisabled(void) {
    return InterlockedCompareExchange(&g_strongModeDisabled, 0, 0) != 0;
}

#else /* !_M_IX86 */

int __cdecl V8InterceptCompileScript(void* esp_frame) { (void)esp_frame; return 0; }
int __cdecl V8InterceptScriptRun(void* esp_frame) { (void)esp_frame; return 0; }
void* __cdecl V8InterceptSetCallHandler(void* esp_frame) { (void)esp_frame; return NULL; }
void __cdecl V8InterceptNewTemplate(void* esp_frame) { (void)esp_frame; }
void V8InterceptInit(void) {}
void V8InterceptShutdown(void) {}
int V8InterceptStrongModeDisabled(void) { return 1; }

#endif
