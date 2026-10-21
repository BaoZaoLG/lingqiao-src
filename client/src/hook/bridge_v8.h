#pragma once
/* ============================================================================
 * bridge_v8.h — V8 Native Bridge transport
 *
 * Uses the CEF V8 C API to register a native bridge object in the V8
 * context, providing synchronous JS → C++ calls without console overhead.
 *
 * Registers window.__LingQiaoNative.request(jsonString) in the V8 context.
 * Falls back to console transport if V8 C API is not available.
 * ========================================================================= */

#include "include/capi/cef_frame_capi.h"
#include "include/capi/cef_v8_capi.h"

/* Resolve V8 C API exports from libcef.dll. Call once during DLL init.
 * Returns true if the required V8 C API functions were found. */
bool BridgeInstallV8Transport();

/* Attempt to install the native __LingQiaoNative object into the frame's
 * V8 context. Call from MyOnLoadEnd when a V8 context is available.
 * Returns true if the native object was installed and V8 transport is active. */
bool BridgeTryInstallNativeObject(_cef_frame_t* frame);

/* Install native object using a pre-obtained V8 context.
 * Does NOT call cef_v8context_get_current_context — safe from detours.
 * The ctx must come from the trampoline, not the hooked export. */
bool BridgeInstallNativeObjectInContext(cef_v8context_t* ctx);

/* Detach V8 bridge. Call during DLL cleanup. */
void BridgeDetachV8Transport();

/* Check if V8 transport is currently active. */
bool BridgeIsV8TransportActive();

/* Hook cef_v8context_get_current_context to auto-install native object
 * on first V8 activity. Call from renderer process after V8 CAPI resolution. */
bool InstallV8ContextHook();

/* Remove the V8 context hook. Call during DLL cleanup. */
void UninstallV8ContextHook();

/* Called from V8 script_run detour (v8_hooks.cpp). Attempts to install
 * the native object while the V8 context is active. Thread-safe, fast
 * on subsequent calls (returns immediately via atomic flag). */
extern "C" void BridgeTryInstallFromV8Hook();
