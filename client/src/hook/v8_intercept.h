#pragma once
/* ============================================================================
 * v8_intercept.h — V8 Interception Engine (combat mode)
 *
 * Transforms passive V8 observation hooks into active interception:
 *
 *  1. Script Interception  — read/patch JS source before V8 compiles it
 *  2. Handler Hijacking    — replace native function handlers at registration
 *  3. Execution Control    — per-script breakpoints, skip, replay
 *
 * All entry points are extern "C" __cdecl — called from x86 naked asm detours.
 * They receive a captured stack pointer (esp_frame) from which original
 * arguments are read at well-known offsets.
 * ========================================================================= */

#include <windows.h>

#ifdef __cplusplus
extern "C" {
#endif

/* ---- 1. Script interception (compile_script hook, index 0) ----
 * Called before V8 compiles a script.  Receives the raw stack pointer
 * so it can inspect function arguments.
 * Returns:  0 = pass-through to original (possibly with patched source)
 *           1 = BLOCK script (detour should return without compiling) */
int __cdecl V8InterceptCompileScript(void* esp_frame);

/* ---- 2. Handler hijacking (set_call_handler hook, index 2) ----
 * Called when the app registers a native function handler.
 * Receives esp_frame with args at [0]=a1 [4]=a2 [8]=a3 [12]=a4.
 * Returns the callback pointer to install (original or replacement). */
void* __cdecl V8InterceptSetCallHandler(void* esp_frame);

/* ---- 3. Execution control (script_run hook, index 1) ----
 * Called before a compiled script executes.  Also triggers bridge install.
 * Returns:  0 = allow execution
 *           1 = skip execution (detour should return early) */
int __cdecl V8InterceptScriptRun(void* esp_frame);

/* ---- Template tracking (function_template.new hooks, index 3/4) ----
 * Called when a new function template is created. Records the handle. */
void __cdecl V8InterceptNewTemplate(void* esp_frame);

/* ---- Initialize / shutdown ---- */
void V8InterceptInit(void);
void V8InterceptShutdown(void);
int V8InterceptStrongModeDisabled(void);

#ifdef __cplusplus
}
#endif
