#pragma once
/* ============================================================================
 * hook_engine.h — Inline hook engine API (MinHook-backed)
 *
 * Provides a clean wrapper around the MinHook library for inline function
 * hooking. Supports both x86 and x64 targets with proper instruction
 * relocation and thread safety.
 *
 * Requires MinHook to be downloaded via: scripts/download_minhook.ps1
 * ========================================================================= */
#include <windows.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Find a function by module name and export name.
 * Returns NULL if not found. */
PVOID HookFindFunction(PCSTR pszModule, PCSTR pszFunction);

/* Install an inline hook.
 *
 * ppPointer [in/out]:
 *   IN:  pointer to the original function address
 *   OUT: pointer to the trampoline (call via *ppPointer to call original)
 *
 * pDetour:
 *   Address of the replacement function.
 *
 * Returns ERROR_SUCCESS on success, Win32 error code on failure.
 *
 * Thread-safe. Can be called from any thread. */
LONG HookAttach(PVOID* ppPointer, PVOID pDetour);

/* Remove an inline hook previously installed by HookAttach.
 *
 * ppPointer [in/out]:
 *   IN:  pointer to the trampoline address (as returned by HookAttach)
 *   OUT: pointer restored to original function address
 *
 * Returns ERROR_SUCCESS on success, Win32 error code on failure.
 *
 * Thread-safe. Can be called from any thread. */
LONG HookDetach(PVOID* ppPointer);

#ifdef __cplusplus
}
#endif
