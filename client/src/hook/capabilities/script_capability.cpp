/* ============================================================================
 * script_capability.cpp — script.* bridge methods
 *
 * script.getVersion — get the injected AutoExam script version
 * ========================================================================= */

#include "pch.h"
#include "bridge_dispatcher.h"
#include "bridge_protocol.h"

/* Script version is tracked globally by the script loader */
static std::string g_scriptVersion = "";

void BridgeSetScriptVersion(const std::string& version) {
    g_scriptVersion = version;
}

/* ---- script.getVersion ---- */
static std::string HandleScriptGetVersion(const BridgeRequest& req) {
    (void)req;
    return "{\"version\":\"" + BridgeJsonEscape(g_scriptVersion) + "\"}";
}

/* ---- Registration ---- */
void RegisterScriptCapabilities() {
    BridgeRegisterMethod(LQ_METHOD_SCRIPT_GET_VERSION, HandleScriptGetVersion);
}
