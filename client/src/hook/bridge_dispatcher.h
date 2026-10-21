#pragma once
/* ============================================================================
 * bridge_dispatcher.h — RPC method whitelist and dispatch engine
 *
 * All bridge methods must be registered here before they can be called from JS.
 * Unknown methods are rejected with METHOD_NOT_FOUND.
 * ========================================================================= */

#include <string>
#include "bridge_json.h"

/* Method handler function signature.
 * Receives the parsed BridgeRequest, returns a JSON result value (not the full envelope).
 * On error, returns a JSON string starting with "__ERROR__:" followed by code|message. */
typedef std::string (*BridgeMethodHandler)(const BridgeRequest& req);

/* Register a method handler. Must be called during bridge initialization.
 * name: dot-notation method name (e.g. "runtime.ping")
 * handler: function pointer that handles the method. */
void BridgeRegisterMethod(const char* name, BridgeMethodHandler handler);

/* Dispatch a raw JSON request string.
 * Parses the request, looks up the method in the whitelist, calls the handler,
 * and returns the full response envelope JSON string.
 * Never crashes — all errors are caught and returned as error envelopes. */
std::string BridgeDispatch(const std::string& rawJson);

/* Get the list of registered method names (for runtime.getCapabilities). */
std::string BridgeGetCapabilitiesJson();

/* Number of registered methods. A DLL built before a feature landed reports a
 * smaller count, which is how a stale binary is spotted without forensics. */
int BridgeGetMethodCount();

/* Compile stamp of this binary ("<date> <time>" of bridge_dispatcher.cpp).
 * Two DLLs can both claim bridge_version 2.0, so the stamp is what actually
 * distinguishes one build from another. */
const char* BridgeGetBuildStamp();

/* Target architecture of this binary ("x86" or "x64"). V8 interception is only
 * implemented on x86, so this explains a missing __LingQiaoNative. */
const char* BridgeGetBuildArch();

/* Check if a method name is registered. */
bool BridgeIsMethodRegistered(const char* name);

/* Initialize the dispatcher with built-in capabilities. */
void BridgeInitDispatcher();
