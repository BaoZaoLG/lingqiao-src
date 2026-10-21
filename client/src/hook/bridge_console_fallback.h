#pragma once
/* ============================================================================
 * bridge_console_fallback.h — Console transport for Bridge v2
 *
 * Intercepts console.info messages with the bridge prefix from the CEF
 * display handler. This is the most reliable transport channel since it
 * doesn't require V8 or process message hooks.
 *
 * Flow: JS -> console.info("__LQ_BRIDGE_V2__" + JSON) -> display handler
 *       -> strip prefix -> BridgeDispatch -> BridgeDeliverJson -> JS
 * ========================================================================= */

#include "include/capi/cef_display_handler_capi.h"

/* Install the console bridge by hooking the display handler's
 * on_console_message callback on the given CEF client.
 * Returns true on success. */
bool TryHookDisplayHandler(struct _cef_client_t* client);

/* Detach the console bridge hook. Call during DLL cleanup. */
void DetachDisplayHandlerHook();

/* Get the original on_console_message function pointer (for cleanup). */
extern PVOID g_cef_on_console_message;
