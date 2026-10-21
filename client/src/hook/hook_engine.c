/* ============================================================================
 * hook_engine.c — Inline hook engine (MinHook wrapper)
 *
 * Uses MinHook library for robust x86/x64 inline hooking.
 * MinHook handles all instruction decoding, trampoline generation,
 * and instruction relocation internally — no custom x86/x64 disassembly.
 *
 * Improvements over original:
 *   - Repeat attach with same target+detour returns existing trampoline
 *   - Repeat attach with same target, different detour returns error
 *   - MH_ERROR_ALREADY_CREATED handled gracefully
 *   - MH_ERROR_ENABLED (already active) treated as success
 *   - HookDetach returns ERROR_NOT_FOUND for unknown trampolines
 *     (no dangerous fallback to raw pointer)
 *   - HookDetachAll removes all hooks in reverse installation order
 *   - SRW lock protects hook map writes for thread safety
 *   - MAX_HOOKS increased to 128
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

/* Hook registry — tracks all installed hooks.
 * Protected by g_registryLock for thread-safe writes. */
#define MAX_HOOKS 128
static struct {
    PVOID trampoline;  /* what the caller sees (original function pointer) */
    PVOID target;      /* what MinHook needs for removal */
    PVOID detour;      /* the replacement function */
    int   enabled;     /* 1 = active, 0 = detached */
} g_hookMap[MAX_HOOKS];
static LONG g_hookCount = 0;
static SRWLOCK g_registryLock = SRWLOCK_INIT;

static LONG AddHookMapping(PVOID trampoline, PVOID target, PVOID detour) {
    AcquireSRWLockExclusive(&g_registryLock);

    LONG idx = g_hookCount;
    if (idx >= MAX_HOOKS) {
        ReleaseSRWLockExclusive(&g_registryLock);
        OutputDebugStringA("[HOOK] WARNING: hook registry full (MAX_HOOKS=128)\n");
        return ERROR_OUTOFMEMORY;
    }

    g_hookMap[idx].trampoline = trampoline;
    g_hookMap[idx].target = target;
    g_hookMap[idx].detour = detour;
    g_hookMap[idx].enabled = 1;
    g_hookCount = idx + 1;

    ReleaseSRWLockExclusive(&g_registryLock);
    return ERROR_SUCCESS;
}

static PVOID FindTargetByTrampoline(PVOID trampoline) {
    AcquireSRWLockShared(&g_registryLock);
    LONG count = g_hookCount;
    if (count > MAX_HOOKS) count = MAX_HOOKS;
    for (LONG i = 0; i < count; i++) {
        if (g_hookMap[i].trampoline == trampoline && g_hookMap[i].enabled) {
            ReleaseSRWLockShared(&g_registryLock);
            return g_hookMap[i].target;
        }
    }
    ReleaseSRWLockShared(&g_registryLock);
    return NULL;
}

/* Check if a target is already hooked. Returns index or -1. */
static LONG FindByTarget(PVOID target) {
    LONG count = g_hookCount;
    if (count > MAX_HOOKS) count = MAX_HOOKS;
    for (LONG i = 0; i < count; i++) {
        if (g_hookMap[i].target == target && g_hookMap[i].enabled)
            return i;
    }
    return -1;
}

static void MarkDisabled(PVOID trampoline) {
    AcquireSRWLockExclusive(&g_registryLock);
    LONG count = g_hookCount;
    if (count > MAX_HOOKS) count = MAX_HOOKS;
    for (LONG i = 0; i < count; i++) {
        if (g_hookMap[i].trampoline == trampoline) {
            g_hookMap[i].enabled = 0;
            break;
        }
    }
    ReleaseSRWLockExclusive(&g_registryLock);
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

    /* ---- Hold exclusive lock for the entire check+create+enable+store ----
     * This prevents two threads from racing on the same target. */
    AcquireSRWLockExclusive(&g_registryLock);

    /* Check for repeat attach (same target already hooked) */
    LONG existingIdx = FindByTarget(pTarget);
    if (existingIdx >= 0) {
        PVOID existingDetour = g_hookMap[existingIdx].detour;
        PVOID existingTrampoline = g_hookMap[existingIdx].trampoline;
        ReleaseSRWLockExclusive(&g_registryLock);

        if (existingDetour == pDetour) {
            /* Same target + same detour → return existing trampoline (idempotent) */
            *ppPointer = existingTrampoline;
            sprintf_s(dbg, sizeof(dbg),
                "[HOOK] Attach: target=0x%p already hooked with same detour "
                "(trampoline=0x%p)\n", pTarget, existingTrampoline);
            OutputDebugStringA(dbg);
            return ERROR_SUCCESS;
        } else {
            /* Same target, different detour → conflict error */
            sprintf_s(dbg, sizeof(dbg),
                "[HOOK] Attach CONFLICT: target=0x%p already hooked with "
                "detour=0x%p, new detour=0x%p rejected\n",
                pTarget, existingDetour, pDetour);
            OutputDebugStringA(dbg);
            return ERROR_ALREADY_EXISTS;
        }
    }

    /* ---- Create the hook (disabled state) ---- */
    MH_STATUS st = MH_CreateHook(pTarget, pDetour, &pOriginal);
    if (st == MH_ERROR_ALREADY_CREATED) {
        /* Hook exists externally (not in our registry) — this is a conflict.
         * We cannot reliably obtain the trampoline from MinHook in this case,
         * so treat it as an error rather than attempting an unreliable retry. */
        ReleaseSRWLockExclusive(&g_registryLock);
        sprintf_s(dbg, sizeof(dbg),
            "[HOOK] Attach REJECTED: target=0x%p already has an external hook "
            "(not in our registry)\n", pTarget);
        OutputDebugStringA(dbg);
        return ERROR_ALREADY_EXISTS;
    }
    if (st != MH_OK) {
        ReleaseSRWLockExclusive(&g_registryLock);
        sprintf_s(dbg, sizeof(dbg),
            "[HOOK] MH_CreateHook failed: %s (%d) target=0x%p\n",
            MH_StatusToString(st), st, pTarget);
        OutputDebugStringA(dbg);
        return ERROR_INVALID_DATA;
    }

    /* ---- Enable the hook ---- */
    st = MH_EnableHook(pTarget);
    if (st == MH_ERROR_ENABLED) {
        /* Already enabled — that's fine, treat as success */
    } else if (st != MH_OK) {
        /* Enable failed — clean up the created hook before returning */
        MH_RemoveHook(pTarget);
        ReleaseSRWLockExclusive(&g_registryLock);
        sprintf_s(dbg, sizeof(dbg),
            "[HOOK] MH_EnableHook failed: %s (%d) target=0x%p\n",
            MH_StatusToString(st), st, pTarget);
        OutputDebugStringA(dbg);
        return ERROR_WRITE_FAULT;
    }

    /* ---- Store in registry (under lock) ---- */
    LONG idx = g_hookCount;
    if (idx >= MAX_HOOKS) {
        MH_DisableHook(pTarget);
        MH_RemoveHook(pTarget);
        ReleaseSRWLockExclusive(&g_registryLock);
        OutputDebugStringA("[HOOK] WARNING: hook registry full (MAX_HOOKS=128)\n");
        return ERROR_OUTOFMEMORY;
    }
    g_hookMap[idx].trampoline = pOriginal;
    g_hookMap[idx].target = pTarget;
    g_hookMap[idx].detour = pDetour;
    g_hookMap[idx].enabled = 1;
    g_hookCount = idx + 1;

    ReleaseSRWLockExclusive(&g_registryLock);

    /* Return trampoline address so caller can call original */
    *ppPointer = pOriginal;

    sprintf_s(dbg, sizeof(dbg),
        "[HOOK] Hook installed: target=0x%p trampoline=0x%p detour=0x%p\n",
        pTarget, pOriginal, pDetour);
    OutputDebugStringA(dbg);

    return ERROR_SUCCESS;
}

LONG HookDetach(PVOID* ppPointer) {
    if (!ppPointer || !*ppPointer)
        return ERROR_INVALID_PARAMETER;

    /* Find the original target address from our registry */
    PVOID pTarget = FindTargetByTrampoline(*ppPointer);

    char dbg[256];
    sprintf_s(dbg, sizeof(dbg),
        "[HOOK] Detach: trampoline=0x%p target=0x%p\n",
        *ppPointer, pTarget);
    OutputDebugStringA(dbg);

    if (!pTarget) {
        /* Trampoline not found in registry — do NOT fall back to raw pointer.
         * This prevents calling MH_DisableHook on a non-hook address. */
        sprintf_s(dbg, sizeof(dbg),
            "[HOOK] Detach SKIPPED: trampoline=0x%p not in registry\n",
            *ppPointer);
        OutputDebugStringA(dbg);
        return ERROR_NOT_FOUND;
    }

    /* Disable and remove the hook */
    MH_STATUS st = MH_DisableHook(pTarget);
    if (st != MH_OK && st != MH_ERROR_NOT_CREATED) {
        sprintf_s(dbg, sizeof(dbg),
            "[HOOK] MH_DisableHook failed: %s (%d)\n",
            MH_StatusToString(st), st);
        OutputDebugStringA(dbg);
    }

    st = MH_RemoveHook(pTarget);
    if (st != MH_OK && st != MH_ERROR_NOT_CREATED) {
        sprintf_s(dbg, sizeof(dbg),
            "[HOOK] MH_RemoveHook failed: %s (%d)\n",
            MH_StatusToString(st), st);
        OutputDebugStringA(dbg);
        return ERROR_NOT_FOUND;
    }

    /* Mark as disabled in registry */
    MarkDisabled(*ppPointer);

    /* Restore the caller's pointer to the original target */
    *ppPointer = pTarget;

    OutputDebugStringA("[HOOK] Hook detached successfully\n");
    return ERROR_SUCCESS;
}

void HookDetachAll(void) {
    AcquireSRWLockExclusive(&g_registryLock);
    LONG count = g_hookCount;
    if (count > MAX_HOOKS) count = MAX_HOOKS;

    char dbg[128];
    sprintf_s(dbg, sizeof(dbg), "[HOOK] DetachAll: %ld hooks to remove\n", count);
    OutputDebugStringA(dbg);

    /* Remove in reverse installation order */
    for (LONG i = count - 1; i >= 0; i--) {
        if (!g_hookMap[i].enabled) continue;

        PVOID target = g_hookMap[i].target;
        g_hookMap[i].enabled = 0;

        MH_DisableHook(target);
        MH_RemoveHook(target);
    }

    g_hookCount = 0;
    ReleaseSRWLockExclusive(&g_registryLock);

    OutputDebugStringA("[HOOK] DetachAll complete\n");
}
