#pragma once
/* ============================================================================
 * bridge_runtime.h — Bridge lifecycle, JSON delivery, and event push
 *
 * Manages the bridge state and provides the DLL -> JS JSON delivery mechanism
 * used by all transport layers.
 * ========================================================================= */

#include <string>
#include "include/capi/cef_frame_capi.h"

struct _cef_browser_t;

/* Current transport type in use */
enum BridgeTransport {
    BRIDGE_TRANSPORT_CONSOLE = 0,
    BRIDGE_TRANSPORT_V8,
    BRIDGE_TRANSPORT_PROCESS_MESSAGE
};

/* Initialize the bridge runtime. Call once during DLL startup. */
void BridgeRuntimeInit();

/* Shutdown the bridge runtime. Call during CefHookShutdown() before FreeLibrary. */
void BridgeRuntimeShutdown();

/* Set the active transport type. */
void BridgeSetTransport(BridgeTransport transport);

/* Get the active transport type. */
BridgeTransport BridgeGetTransport();

/* Get transport name string for status reporting. */
const char* BridgeGetTransportName();

/* Deliver a JSON string to the JS side (DLL -> JS).
 * This is the unified delivery mechanism for responses and events.
 * The frame parameter is the CEF frame to inject into.
 * Returns true on success, false on failure. */
bool BridgeDeliverJson(_cef_frame_t* frame, const std::string& json);

/* Deliver a response to a specific request (convenience wrapper). */
bool BridgeDeliverResponse(_cef_frame_t* frame, const std::string& id,
                           const std::string& resultJson);

/* Push an event to the JS side. */
bool BridgeDeliverEvent(_cef_frame_t* frame, const std::string& eventName,
                        const std::string& payloadJson);

/* Store the main frame pointer with add_ref (ref-counted).
 * Replaces any previously stored frame, releasing the old reference. */
void BridgeStoreMainFrame(_cef_frame_t* frame);

/* Release and NULL the stored main frame reference. */
void BridgeReleaseMainFrame();

/* Get the current main frame pointer. */
_cef_frame_t* BridgeGetMainFrame();

/* CEF UI-thread helpers (async AI must deliver from TID_UI). */
bool BridgeIsUiThread();
bool BridgePostToUiThread(void (*fn)(void*), void* ctx);
/* Deliver json to main frame + all frames, posting to UI thread when needed. */
void BridgeDeliverToAllFramesSafe(struct _cef_browser_t* browser, const std::string& json);

/* Track hook states for diagnostics */
struct BridgeHookState {
    bool createBrowser;
    bool loadHandler;
    bool displayHandler;
    bool lifeSpanHandler;
    bool v8;
};

BridgeHookState& BridgeGetHookState();

/* Track script injection state */
void BridgeSetScriptInjected(bool injected);
bool BridgeIsScriptInjected();

/* Track DLL loaded state */
void BridgeSetDllLoaded(bool loaded);
bool BridgeIsDllLoaded();
