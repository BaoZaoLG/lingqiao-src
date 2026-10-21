/* ============================================================================
 * bridge_dispatcher.cpp — RPC method whitelist and dispatch engine
 * ========================================================================= */

#include "pch.h"
#include "bridge_dispatcher.h"
#include "bridge_protocol.h"
#include <map>
#include <vector>

/* ---- Method registry ---- */

static std::map<std::string, BridgeMethodHandler>& GetMethodMap() {
    static std::map<std::string, BridgeMethodHandler> methods;
    return methods;
}

void BridgeRegisterMethod(const char* name, BridgeMethodHandler handler) {
    if (!name || !handler) return;
    GetMethodMap()[name] = handler;
    char dbg[128];
    sprintf_s(dbg, "[BRIDGE] Registered method: %s\n", name);
    OutputDebugStringA(dbg);
}

bool BridgeIsMethodRegistered(const char* name) {
    if (!name) return false;
    auto& m = GetMethodMap();
    return m.find(name) != m.end();
}

std::string BridgeGetCapabilitiesJson() {
    auto& m = GetMethodMap();
    std::string out = "[";
    bool first = true;
    for (auto& kv : m) {
        if (!first) out += ",";
        out += "\"" + BridgeJsonEscape(kv.first) + "\"";
        first = false;
    }
    out += "]";
    return out;
}

int BridgeGetMethodCount() {
    return (int)GetMethodMap().size();
}

const char* BridgeGetBuildStamp() {
    return __DATE__ " " __TIME__;
}

const char* BridgeGetBuildArch() {
    return sizeof(void*) == 8 ? "x64" : "x86";
}

/* ---- Forward declarations for capability registration ---- */
extern void RegisterRuntimeCapabilities();
extern void RegisterScriptCapabilities();
extern void RegisterDiagnosticsCapabilities();
extern void RegisterPageCapabilities();
extern void RegisterExamCapabilities();
extern void RegisterAiCapabilities();
extern void RegisterNativeInputCapabilities();

void BridgeInitDispatcher() {
    {
        char banner[256];
        sprintf_s(banner, "[BRIDGE] ===== CefHook build %s (%s) =====\n",
                  BridgeGetBuildStamp(), BridgeGetBuildArch());
        OutputDebugStringA(banner);
    }
    OutputDebugStringA("[BRIDGE] Initializing dispatcher...\n");
    RegisterRuntimeCapabilities();
    RegisterScriptCapabilities();
    RegisterDiagnosticsCapabilities();
    RegisterPageCapabilities();
    RegisterExamCapabilities();
    RegisterAiCapabilities();
    RegisterNativeInputCapabilities();
    char dbg[192];
    sprintf_s(dbg, "[BRIDGE] Dispatcher ready with %d methods\n", BridgeGetMethodCount());
    OutputDebugStringA(dbg);

    /* A DLL that silently lacks a method is the hardest class of bug to chase,
     * so state the AI surface explicitly at startup. If any of these is 0 while
     * the page is running the new script, the wrong DLL is loaded. */
    sprintf_s(dbg, "[BRIDGE] AI surface: chatComplete=%d pollResult=%d pollTrigger=%d\n",
              BridgeIsMethodRegistered(LQ_METHOD_AI_CHAT_COMPLETE) ? 1 : 0,
              BridgeIsMethodRegistered(LQ_METHOD_AI_POLL_RESULT) ? 1 : 0,
              BridgeIsMethodRegistered(LQ_METHOD_NATIVE_POLL_TRIGGER) ? 1 : 0);
    OutputDebugStringA(dbg);
}

/* ---- Dispatch ---- */

/* Error sentinel prefix used by handlers to signal errors */
static const char* ERROR_PREFIX = "__ERROR__:";

std::string BridgeDispatch(const std::string& rawJson) {
    /* Parse the request */
    BridgeRequest req;
    std::string parseError = BridgeParseRequest(rawJson, &req);
    if (!parseError.empty()) {
        /* Try to extract id so the JS pending request can reject instead of hang. */
        std::string id = BridgeJsonGetString(rawJson, "id");
        char dbg[192];
        sprintf_s(dbg, "[BRIDGE] Parse failed code=%s size=%zu\n",
                  parseError.c_str(), rawJson.size());
        OutputDebugStringA(dbg);
        return BridgeMakeError(id, parseError.c_str(),
                               "Failed to parse bridge request");
    }

    /* Whitelist check */
    auto& methods = GetMethodMap();
    auto it = methods.find(req.method);
    if (it == methods.end()) {
        char dbg[160];
        sprintf_s(dbg, "[BRIDGE] Unknown method: %s\n", req.method.c_str());
        OutputDebugStringA(dbg);
        return BridgeMakeError(req.id, LQ_ERR_METHOD_NOT_FOUND,
                               "Unknown method: " + req.method);
    }

    if (req.method == "ai.chatComplete") {
        char dbg[128];
        sprintf_s(dbg, "[BRIDGE] ai.chatComplete params_size=%zu\n", req.params.size());
        OutputDebugStringA(dbg);
    }

    /* Call the handler — handlers must not throw/AV by design */
    std::string result = it->second(req);

    /* Check if the handler returned an error */
    if (result.compare(0, strlen(ERROR_PREFIX), ERROR_PREFIX) == 0) {
        /* Parse error format: __ERROR__:code|message */
        std::string errBody = result.substr(strlen(ERROR_PREFIX));
        size_t sep = errBody.find('|');
        if (sep != std::string::npos) {
            return BridgeMakeError(req.id, errBody.substr(0, sep).c_str(),
                                   errBody.substr(sep + 1));
        }
        return BridgeMakeError(req.id, LQ_ERR_METHOD_FAILED, errBody);
    }

    return BridgeMakeSuccess(req.id, result);
}
