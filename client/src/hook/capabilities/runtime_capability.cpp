/* ============================================================================
 * runtime_capability.cpp — runtime.* bridge methods
 *
 * runtime.ping          — health check
 * runtime.getStatus     — DLL/bridge status
 * runtime.getCapabilities — list registered methods
 * runtime.getConfig     — get non-sensitive model metadata
 * ========================================================================= */

#include "pch.h"
#include "bridge_dispatcher.h"
#include "bridge_runtime.h"
#include "bridge_protocol.h"
#include "secret_channel.h"

/* ---- runtime.ping ---- */
static std::string HandleRuntimePing(const BridgeRequest& req) {
    (void)req;
    return "\"pong\"";
}

/* ---- runtime.getStatus ---- */
static std::string HandleRuntimeGetStatus(const BridgeRequest& req) {
    (void)req;
    BridgeHookState& hs = BridgeGetHookState();

    std::string out = "{";
    out += "\"dll_loaded\":" + std::string(BridgeIsDllLoaded() ? "true" : "false") + ",";
    out += "\"bridge_version\":\"" LQ_BRIDGE_VERSION_TEXT "\",";
    /* Two DLL builds can both report bridge_version 2.0, so expose something
     * that actually distinguishes them: the compile stamp, the target arch
     * (V8 interception is x86-only) and the registered method count. */
    out += "\"build_stamp\":\"" + BridgeJsonEscape(BridgeGetBuildStamp()) + "\",";
    out += "\"build_arch\":\"" + std::string(BridgeGetBuildArch()) + "\",";
    out += "\"method_count\":" + std::to_string(BridgeGetMethodCount()) + ",";
    out += "\"ai_ready\":" + std::string(
        (BridgeIsMethodRegistered(LQ_METHOD_AI_CHAT_COMPLETE) &&
         BridgeIsMethodRegistered(LQ_METHOD_AI_POLL_RESULT)) ? "true" : "false") + ",";
    out += "\"transport\":\"" + std::string(BridgeGetTransportName()) + "\",";
    out += "\"hooks\":{";
    out += "\"create_browser\":" + std::string(hs.createBrowser ? "true" : "false") + ",";
    out += "\"load_handler\":" + std::string(hs.loadHandler ? "true" : "false") + ",";
    out += "\"display_handler\":" + std::string(hs.displayHandler ? "true" : "false") + ",";
    out += "\"life_span_handler\":" + std::string(hs.lifeSpanHandler ? "true" : "false") + ",";
    out += "\"v8\":" + std::string(hs.v8 ? "true" : "false");
    out += "}}";
    return out;
}

/* ---- runtime.getCapabilities ---- */
static std::string HandleRuntimeGetCapabilities(const BridgeRequest& req) {
    (void)req;
    return "{\"methods\":" + BridgeGetCapabilitiesJson() + "}";
}

/* ---- runtime.getConfig ---- */
static std::string HandleRuntimeGetConfig(const BridgeRequest& req) {
    (void)req;

    /* Read config from one-shot secret channel (never env secrets) */
    const LqSecretBundle& secrets = LqSecretsCached();
    const bool hasApiKey = !secrets.apiKey.empty();

    std::string out = "{";
    out += "\"model\":\"" + BridgeJsonEscape(secrets.model) + "\",";
    out += "\"baseUrl\":\"" + BridgeJsonEscape(secrets.baseUrl) + "\",";
    out += "\"hasApiKey\":" + std::string(hasApiKey ? "true" : "false");
    out += "}";
    return out;
}

/* ---- Registration ---- */
void RegisterRuntimeCapabilities() {
    BridgeRegisterMethod(LQ_METHOD_RUNTIME_PING, HandleRuntimePing);
    BridgeRegisterMethod(LQ_METHOD_RUNTIME_GET_STATUS, HandleRuntimeGetStatus);
    BridgeRegisterMethod(LQ_METHOD_RUNTIME_GET_CAPS, HandleRuntimeGetCapabilities);
    BridgeRegisterMethod(LQ_METHOD_RUNTIME_GET_CONFIG, HandleRuntimeGetConfig);
}
