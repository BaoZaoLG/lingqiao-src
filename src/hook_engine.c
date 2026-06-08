/* ============================================================================
 * hook_engine.c — Inline hook engine (MinHook wrapper)
 *
 * Uses MinHook library for robust x86/x64 inline hooking.
 * MinHook handles all instruction decoding, trampoline generation,
 * and instruction relocation internally — no custom x86/x64 disassembly.
 *
 * Requires MinHook: run scripts/download_minhook.ps1 before building.
 * ========================================================================= */
#include "hook_engine.h"
#include "MinHook.h"
#include <stdio.h>

/* ============================================================================
 * Internal state
 * ========================================================================= */
static LONG g_mhInitialized = 0;
static SRWLOCK g_mhInitLock = SRWLOCK_INIT;

/* Trampoline -> Target mapping for detach support.
 * MinHook needs the target address to remove a hook, but our API
 * gives the caller the trampoline. We store the mapping here. */
#define MAX_HOOKS 64
static struct {
    PVOID trampoline;  /* what the caller sees (original function pointer) */
    PVOID target;      /* what MinHook needs for removal */
} g_hookMap[MAX_HOOKS];
static LONG g_hookCount = 0;

static void AddHookMapping(PVOID trampoline, PVOID target) {
    LONG idx = InterlockedIncrement(&g_hookCount) - 1;
    if (idx < MAX_HOOKS) {
        g_hookMap[idx].trampoline = trampoline;
        g_hookMap[idx].target = target;
    }
}

static PVOID FindTargetByTrampoline(PVOID trampoline) {
    for (LONG i = 0; i < g_hookCount; i++) {
        if (g_hookMap[i].trampoline == trampoline)
            return g_hookMap[i].target;
    }
    return NULL;
}

/* ============================================================================
 * MinHook initialization (thread-safe, once)
 * ========================================================================= */
static LONG EnsureMinHookInit(void) {
    AcquireSRWLockExclusive(&g_mhInitLock);
    if (g_mhInitialized) {
        ReleaseSRWLockExclusive(&g_mhInitLock);
        return MH_OK;
    }

    {
        MH_STATUS st = MH_Initialize();
        if (st != MH_OK && st != MH_ERROR_ALREADY_INITIALIZED) {
            ReleaseSRWLockExclusive(&g_mhInitLock);
            return st;
        }
        g_mhInitialized = 1;
    }

    ReleaseSRWLockExclusive(&g_mhInitLock);
    return MH_OK;
}

/* ============================================================================
 * Public API
 * ========================================================================= */

PVOID HookFindFunction(PCSTR pszModule, PCSTR pszFunction) {
    HMODULE hMod = GetModuleHandleA(pszModule);
    if (!hMod) return NULL;
    return (PVOID)GetProcAddress(hMod, pszFunction);
}

LONG HookAttach(PVOID* ppPointer, PVOID pDetour) {
    if (!ppPointer || !*ppPointer || !pDetour)
        return ERROR_INVALID_PARAMETER;

    LONG initSt = EnsureMinHookInit();
    if (initSt != MH_OK && initSt != MH_ERROR_ALREADY_INITIALIZED)
        return ERROR_NOT_READY;

    PVOID pTarget = *ppPointer;
    PVOID pOriginal = NULL;

    char dbg[256];
    sprintf_s(dbg, sizeof(dbg),
        "[HOOK] Attach: target=0x%p detour=0x%p\n",
        pTarget, pDetour);
    OutputDebugStringA(dbg);

    /* Create the hook (disabled state) */
    MH_STATUS st = MH_CreateHook(pTarget, pDetour, &pOriginal);
    if (st != MH_OK && st != MH_ERROR_ALREADY_CREATED) {
        sprintf_s(dbg, sizeof(dbg),
            "[HOOK] MH_CreateHook failed: %s (%d)\n",
            MH_StatusToString(st), st);
        OutputDebugStringA(dbg);
        return ERROR_INVALID_DATA;
    }

    /* Enable the hook */
    st = MH_EnableHook(pTarget);
    if (st != MH_OK) {
        sprintf_s(dbg, sizeof(dbg),
            "[HOOK] MH_EnableHook failed: %s (%d)\n",
            MH_StatusToString(st), st);
        OutputDebugStringA(dbg);
        return ERROR_WRITE_FAULT;
    }

    /* Store trampoline -> target mapping for later detach */
    AddHookMapping(pOriginal, pTarget);

    /* Return trampoline address so caller can call original */
    *ppPointer = pOriginal;

    sprintf_s(dbg, sizeof(dbg),
        "[HOOK] Hook installed: original=0x%p trampoline=0x%p\n",
        pTarget, pOriginal);
    OutputDebugStringA(dbg);

    return ERROR_SUCCESS;
}

LONG HookDetach(PVOID* ppPointer) {
    if (!ppPointer || !*ppPointer)
        return ERROR_INVALID_PARAMETER;

    /* Find the original target address from our mapping */
    PVOID pTarget = FindTargetByTrampoline(*ppPointer);
    if (!pTarget) {
        /* Maybe the caller passed the target directly (not trampoline) */
        pTarget = *ppPointer;
    }

    char dbg[256];
    sprintf_s(dbg, sizeof(dbg),
        "[HOOK] Detach: trampoline=0x%p target=0x%p\n",
        *ppPointer, pTarget);
    OutputDebugStringA(dbg);

    /* Disable and remove the hook */
    MH_STATUS st = MH_DisableHook(pTarget);
    if (st != MH_OK) {
        sprintf_s(dbg, sizeof(dbg),
            "[HOOK] MH_DisableHook failed: %s (%d)\n",
            MH_StatusToString(st), st);
        OutputDebugStringA(dbg);
    }

    st = MH_RemoveHook(pTarget);
    if (st != MH_OK) {
        sprintf_s(dbg, sizeof(dbg),
            "[HOOK] MH_RemoveHook failed: %s (%d)\n",
            MH_StatusToString(st), st);
        OutputDebugStringA(dbg);
        return ERROR_NOT_FOUND;
    }

    /* Restore the caller's pointer to the original target */
    *ppPointer = pTarget;

    OutputDebugStringA("[HOOK] Hook detached successfully\n");
    return ERROR_SUCCESS;
}
