#include "v8_hooks.h"

#include "hook_engine.h"
#include "v8_compile_sampler.h"
#include "v8_fast_neutralize.h"
#include "v8_signatures.h"
#include "v8_intercept.h"

#include <windows.h>

#include <cstdio>

#if defined(_M_IX86)

PVOID g_v8HookTrampolines[V8_SIGNATURE_COUNT] = {};
static LONG g_v8HookHits[V8_SIGNATURE_COUNT] = {};
static LONG g_v8HooksAttached = 0;
static volatile LONG g_v8FastNeutralizePatched = 0;

static void LogLine(const char* text) {
    OutputDebugStringA(text);
    OutputDebugStringA("\n");
}

/*
 * ===========================================================================
 * V8 Detours - Combat Mode
 *
 * Each detour:
 *   1. Saves registers (pushfd + pushad)
 *   2. Calls the interception handler from v8_intercept.cpp
 *   3. Conditionally skips or proceeds based on return value
 *   4. Restores registers and either jumps to trampoline or returns early
 *
 * Stack frame convention for esp_frame:
 *   - compile_script / script_run: esp captured directly -> original args
 *     at [esp_frame+36], [esp_frame+40], [esp_frame+44], [esp_frame+48]
 *   - set_call_handler / new_template: args pushed in C order (a4,a3,a2,a1)
 *     -> args at [esp_frame+0], [esp_frame+4], [esp_frame+8], [esp_frame+12]
 *
 * NOTE: The old diagnostic detours (V8Detour2/3/4) had a bug where all
 * four args were read from [esp+52] -- the same offset -- producing
 * identical a1/a2/a3/a4 in logs. This is now fixed: each arg is read
 * from its correct stack offset.
 * ===========================================================================
 */

/* ---------------------------------------------------------------------------
 * Detour 0: compile_script
 *   Intercept script compilation -- read source, apply patches, optionally block.
 *   Returns 0 (pass-through) or 1 (block: return empty handle to caller).
 * --------------------------------------------------------------------------- */
extern "C" __declspec(naked) void V8Detour0() {
    __asm {
        /* Phase 1: Ultra-lightweight sampling in pure asm.
         * Record a tiny fixed-size sample, preserve state. */
        pushfd
        push eax
        push ecx
        push edx
        push edi

        mov eax, 1
        lock xadd dword ptr [g_v8CompileSampleWriteIndex], eax
        inc eax

        mov edi, eax
        dec edi
        and edi, 63
        imul edi, edi, 28
        mov ecx, offset g_v8CompileSamples
        add edi, ecx

        mov edx, dword ptr [esp + 24]
        mov dword ptr [edi + 4], edx
        mov edx, dword ptr [esp + 28]
        mov dword ptr [edi + 8], edx
        mov edx, dword ptr [esp + 32]
        mov dword ptr [edi + 12], edx
        mov edx, dword ptr [esp + 36]
        mov dword ptr [edi + 16], edx
        mov edx, dword ptr [esp + 40]
        mov dword ptr [edi + 20], edx
        mov edx, dword ptr [esp + 32]
        test edx, edx
        jz no_source_handle
        mov edx, dword ptr [edx]
    no_source_handle:
        mov dword ptr [edi + 24], edx
        mov dword ptr [edi], eax

        pop edi
        pop edx
        pop ecx
        pop eax
        popfd

        /* Phase 2: Fast in-place neutralization BEFORE returning to V8.
         * Stack is now clean: [esp]=retaddr [esp+4]=arg0 [esp+8]=arg1
         * [esp+12]=arg2(sourceHandle) [esp+16]=arg3 [esp+20]=arg4
         *
         * V8FastNeutralizeFromSourceHandle follows tagged pointers from
         * sourceHandle, scans each target object for "debugger", and
         * replaces it with same-length spaces. Uses VirtualProtect + SEH
         * only -- no V8 API, no heap allocation, no locks. */
        push dword ptr [esp + 12]
        call V8FastNeutralizeFromSourceHandle
        add esp, 4
        test eax, eax
        jz no_patch_done
        lock inc dword ptr [g_v8FastNeutralizePatched]
        mov eax, 10
        lock xadd dword ptr [g_v8CompileSampleWriteIndex], eax
    no_patch_done:
        jmp dword ptr [g_v8HookTrampolines + 0]
    }
}
/* ---------------------------------------------------------------------------
 * Detour 1: script_run
 *   Intercept script execution -- trigger bridge install, optionally skip.
 *   Returns 0 (allow) or 1 (skip: return 0 to caller without executing).
 * --------------------------------------------------------------------------- */
extern "C" __declspec(naked) void V8Detour1() {
    __asm {
        pushfd
        pushad
        mov eax, esp
        push eax
        call V8InterceptScriptRun
        add esp, 4
        test eax, eax
        popad
        popfd
        jnz skip_run
        jmp dword ptr [g_v8HookTrampolines + 4]
    skip_run:
        xor eax, eax
        ret
    }
}

/* --------------------------------------------------------------------------- *
 * Detour 2: set_call_handler
 *   Intercept native handler registration -- track, optionally replace/block.
 *   Captures 4 original args in C order (a4,a3,a2,a1) and passes esp_frame
 *   to the intercept function.
 *
 *   FIX: old code read all args from [esp+52] -> all four were identical.
 *   Now reads arg1..arg4 from their correct stack offsets.
 * --------------------------------------------------------------------------- */
extern "C" __declspec(naked) void V8Detour2() {
    __asm {
        pushfd
        pushad
        /* Capture original args: arg1 at esp+36, arg2 at esp+40, etc.
         * (after pushad=32 + pushfd=4 = 36 bytes above original esp) */
        mov eax, dword ptr [esp + 48]  /* arg4 */
        push eax
        mov eax, dword ptr [esp + 48]  /* arg3 (esp shifted by 4 after push) */
        push eax
        mov eax, dword ptr [esp + 48]  /* arg2 */
        push eax
        mov eax, dword ptr [esp + 48]  /* arg1 */
        push eax
        /* Stack: [esp]=a1 [esp+4]=a2 [esp+8]=a3 [esp+12]=a4 */
        mov eax, esp
        push eax
        call V8InterceptSetCallHandler
        add esp, 20  /* clean 4 arg pushes + 1 esp_frame push */
        /* eax = callback to install (original or replacement) */
        mov dword ptr [esp + 28], eax  /* overwrite saved EAX in pushad frame */
        popad
        popfd
        jmp dword ptr [g_v8HookTrampolines + 8]
    }
}

/* ---------------------------------------------------------------------------
 * Detour 3: function_template.new
 *   Track template creation. Same arg capture as Detour 2.
 * --------------------------------------------------------------------------- */
extern "C" __declspec(naked) void V8Detour3() {
    __asm {
        pushfd
        pushad
        mov eax, dword ptr [esp + 48]
        push eax
        mov eax, dword ptr [esp + 48]
        push eax
        mov eax, dword ptr [esp + 48]
        push eax
        mov eax, dword ptr [esp + 48]
        push eax
        mov eax, esp
        push eax
        call V8InterceptNewTemplate
        add esp, 20
        popad
        popfd
        jmp dword ptr [g_v8HookTrampolines + 12]
    }
}

/* ---------------------------------------------------------------------------
 * Detour 4: function_template.new.alt
 *   Same as Detour 3 but with different trampoline target.
 * --------------------------------------------------------------------------- */
extern "C" __declspec(naked) void V8Detour4() {
    __asm {
        pushfd
        pushad
        mov eax, dword ptr [esp + 48]
        push eax
        mov eax, dword ptr [esp + 48]
        push eax
        mov eax, dword ptr [esp + 48]
        push eax
        mov eax, dword ptr [esp + 48]
        push eax
        mov eax, esp
        push eax
        call V8InterceptNewTemplate
        add esp, 20
        popad
        popfd
        jmp dword ptr [g_v8HookTrampolines + 16]
    }
}

static PVOID DetourForIndex(size_t index) {
    switch (index) {
    case 0: return reinterpret_cast<PVOID>(V8Detour0);
    case 1: return reinterpret_cast<PVOID>(V8Detour1);
    case 2: return reinterpret_cast<PVOID>(V8Detour2);
    case 3: return reinterpret_cast<PVOID>(V8Detour3);
    case 4: return reinterpret_cast<PVOID>(V8Detour4);
    default: return nullptr;
    }
}

bool AttachV8SignatureHooks(void) {
    if (InterlockedCompareExchange(&g_v8HooksAttached, 1, 0) != 0) {
        LogLine("[HOOK] V8 already attached");
        return true;
    }

    /* Initialize the interception engine (patterns, skip list, tracking) */
    V8InterceptInit();

    V8ResolvedSignature resolved[V8_SIGNATURE_COUNT];
    if (!ResolveV8Signatures(resolved, V8_SIGNATURE_COUNT)) {
        LogLine("[HOOK] V8 skipped: signatures are not all unique");
        InterlockedExchange(&g_v8HooksAttached, 0);
        return false;
    }

    for (size_t i = 0; i < V8_SIGNATURE_COUNT; ++i) {
        g_v8HookTrampolines[i] = resolved[i].address;
        PVOID detour = DetourForIndex(i);
        LONG r = HookAttach(&g_v8HookTrampolines[i], detour);
        if (r != ERROR_SUCCESS) {
            char msg[256];
            sprintf_s(msg, sizeof(msg), "[HOOK] V8 attach failed name=%s error=%ld",
                      resolved[i].name, r);
            LogLine(msg);
            DetachV8SignatureHooks();
            return false;
        }

        char msg[256];
        sprintf_s(msg, sizeof(msg), "[HOOK] V8 attached name=%s target=0x%p trampoline=0x%p",
                  resolved[i].name, resolved[i].address, g_v8HookTrampolines[i]);
        LogLine(msg);
    }

    return true;
}

bool AttachV8CompileHookOnly(void) {
    if (InterlockedCompareExchange(&g_v8HooksAttached, 1, 0) != 0) {
        LogLine("[HOOK] V8 already attached");
        return true;
    }

    V8InterceptInit();

    V8ResolvedSignature resolved[V8_SIGNATURE_COUNT];
    if (!ResolveV8Signatures(resolved, V8_SIGNATURE_COUNT)) {
        LogLine("[HOOK] V8 compile-only skipped: signatures are not all unique");
        InterlockedExchange(&g_v8HooksAttached, 0);
        return false;
    }

    /* Index 0 (compile_script) drives the anti-debugger sampler.
     *
     * Index 1 (script_run) is what reaches BridgeTryInstallFromV8Hook(), and
     * that is the ONLY path that installs window.__LingQiaoNative: nothing in
     * the renderer ever calls the cef_v8context_get_current_context C export
     * that the context hook waits on, so that route never fires. Attaching only
     * index 0 left the V8 transport permanently unarmed — every bridge call fell
     * back to the console transport, whose reply path is execute_java_script,
     * and the V8 poll relay never started.
     *
     * V8Detour1 saves flags/registers and always continues to the trampoline
     * (V8InterceptScriptRun returns 0), so attaching it is behaviour-neutral
     * apart from the bridge install attempt. Signatures 2-4 stay unattached. */
    const size_t indexes[] = { 0, 1 };
    for (size_t n = 0; n < sizeof(indexes) / sizeof(indexes[0]); ++n) {
        const size_t i = indexes[n];
        g_v8HookTrampolines[i] = resolved[i].address;
        LONG r = HookAttach(&g_v8HookTrampolines[i], DetourForIndex(i));
        if (r != ERROR_SUCCESS) {
            char msg[256];
            sprintf_s(msg, sizeof(msg), "[HOOK] V8 attach failed name=%s index=%d error=%ld",
                      resolved[i].name, (int)i, r);
            LogLine(msg);
            DetachV8SignatureHooks();
            return false;
        }

        char msg[256];
        sprintf_s(msg, sizeof(msg), "[HOOK] V8 attached name=%s target=0x%p trampoline=0x%p",
                  resolved[i].name, resolved[i].address, g_v8HookTrampolines[i]);
        LogLine(msg);
    }

    V8CompileSamplerStartLogger();
    return true;
}

void DetachV8SignatureHooks(void) {
    V8CompileSamplerStopLogger();
    for (size_t i = 0; i < V8_SIGNATURE_COUNT; ++i) {
        if (g_v8HookTrampolines[i]) {
            HookDetach(&g_v8HookTrampolines[i]);
            g_v8HookTrampolines[i] = nullptr;
        }
        InterlockedExchange(&g_v8HookHits[i], 0);
    }
    InterlockedExchange(&g_v8HooksAttached, 0);
}

bool AreV8HooksResolved(void) {
    return InterlockedCompareExchange(&g_v8HooksAttached, 0, 0) != 0;
}

#else

bool AttachV8SignatureHooks(void) {
    OutputDebugStringA("[HOOK] V8 skipped: attach-only naked hooks are implemented for Win32/x86 only\n");
    return false;
}

bool AttachV8CompileHookOnly(void) {
    OutputDebugStringA("[HOOK] V8 compile-only skipped: x86 only\n");
    return false;
}

void DetachV8SignatureHooks(void) {}

bool AreV8HooksResolved(void) { return false; }

#endif
