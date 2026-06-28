#include "v8_hooks.h"

#include "hook_engine.h"
#include "v8_signatures.h"

#include <windows.h>

#include <cstdio>

#if defined(_M_IX86)

PVOID g_v8HookTrampolines[V8_SIGNATURE_COUNT] = {};
static LONG g_v8HookHits[V8_SIGNATURE_COUNT] = {};
static LONG g_v8HooksAttached = 0;
static const LONG kMaxLoggedHitsPerHook = 20;

static void LogLine(const char* text) {
    OutputDebugStringA(text);
    OutputDebugStringA("\n");
}

extern "C" void __cdecl LogV8HookHit(int index) {
    if (index < 0 || index >= static_cast<int>(V8_SIGNATURE_COUNT)) return;
    LONG count = InterlockedIncrement(&g_v8HookHits[index]);
    if (count > kMaxLoggedHitsPerHook) return;

    char msg[256];
    sprintf_s(msg, sizeof(msg), "[HOOK] V8 hit name=%s count=%ld",
              GetV8SignatureName(static_cast<size_t>(index)), count);
    LogLine(msg);
}

extern "C" void __cdecl LogV8BridgeArgs(int index, void* a1, void* a2, void* a3, void* a4) {
    if (index < 0 || index >= static_cast<int>(V8_SIGNATURE_COUNT)) return;
    LONG count = InterlockedIncrement(&g_v8HookHits[index]);
    if (count > kMaxLoggedHitsPerHook) return;

    char msg[512];
    sprintf_s(msg, sizeof(msg), "[HOOK] V8Bridge hit name=%s count=%ld a1=0x%p a2=0x%p a3=0x%p a4=0x%p",
              GetV8SignatureName(static_cast<size_t>(index)), count, a1, a2, a3, a4);
    LogLine(msg);
}

extern "C" __declspec(naked) void V8Detour0() {
    __asm {
        pushfd
        pushad
        push 0
        call LogV8HookHit
        add esp, 4
        popad
        popfd
        jmp dword ptr [g_v8HookTrampolines + 0]
    }
}

extern "C" __declspec(naked) void V8Detour1() {
    __asm {
        pushfd
        pushad
        push 1
        call LogV8HookHit
        add esp, 4
        popad
        popfd
        jmp dword ptr [g_v8HookTrampolines + 4]
    }
}

extern "C" __declspec(naked) void V8Detour2() {
    __asm {
        pushfd
        pushad
        mov eax, dword ptr [esp + 52]
        push eax
        mov eax, dword ptr [esp + 52]
        push eax
        mov eax, dword ptr [esp + 52]
        push eax
        mov eax, dword ptr [esp + 52]
        push eax
        push 2
        call LogV8BridgeArgs
        add esp, 20
        popad
        popfd
        jmp dword ptr [g_v8HookTrampolines + 8]
    }
}

extern "C" __declspec(naked) void V8Detour3() {
    __asm {
        pushfd
        pushad
        mov eax, dword ptr [esp + 52]
        push eax
        mov eax, dword ptr [esp + 52]
        push eax
        mov eax, dword ptr [esp + 52]
        push eax
        mov eax, dword ptr [esp + 52]
        push eax
        push 3
        call LogV8BridgeArgs
        add esp, 20
        popad
        popfd
        jmp dword ptr [g_v8HookTrampolines + 12]
    }
}

extern "C" __declspec(naked) void V8Detour4() {
    __asm {
        pushfd
        pushad
        mov eax, dword ptr [esp + 52]
        push eax
        mov eax, dword ptr [esp + 52]
        push eax
        mov eax, dword ptr [esp + 52]
        push eax
        mov eax, dword ptr [esp + 52]
        push eax
        push 4
        call LogV8BridgeArgs
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

void DetachV8SignatureHooks(void) {
    for (size_t i = 0; i < V8_SIGNATURE_COUNT; ++i) {
        if (g_v8HookTrampolines[i]) {
            HookDetach(&g_v8HookTrampolines[i]);
            g_v8HookTrampolines[i] = nullptr;
        }
        InterlockedExchange(&g_v8HookHits[i], 0);
    }
    InterlockedExchange(&g_v8HooksAttached, 0);
}

#else

bool AttachV8SignatureHooks(void) {
    OutputDebugStringA("[HOOK] V8 skipped: attach-only naked hooks are implemented for Win32/x86 only\n");
    return false;
}

void DetachV8SignatureHooks(void) {}

#endif
