/* ============================================================================
 * diagnostics_capability.cpp — diagnostics.* bridge methods
 *
 * diagnostics.log            — write a log entry to DebugView
 * diagnostics.getHookState   — get detailed hook installation state
 * ========================================================================= */

#include "pch.h"
#include "bridge_dispatcher.h"
#include "bridge_runtime.h"
#include "bridge_protocol.h"
#include "bridge_security.h"

/* ---- diagnostics.log ---- */
static std::string HandleDiagnosticsLog(const BridgeRequest& req) {
    /* Extract level, scope, message from params */
    std::string level = BridgeJsonGetString(req.params, "level");
    std::string scope = BridgeJsonGetString(req.params, "scope");
    std::string message = BridgeJsonGetString(req.params, "message");

    level = BridgeSanitizeLogField(level, 16);
    scope = BridgeSanitizeLogField(scope, 32);
    message = BridgeRedactSensitiveLogField(message, 512);
    if (level != "debug" && level != "info" && level != "warn" && level != "error") {
        level = "info";
    }
    if (scope.empty()) scope = "js";

    /* Write to DebugView */
    std::string logLine = "[BRIDGE:" + level + ":" + scope + "] " + message + "\n";
    OutputDebugStringA(logLine.c_str());

    return "{\"logged\":true}";
}

/* ---- diagnostics.getHookState ---- */
extern void ExamCapabilityGetStats(long* uploaded, long* failed, unsigned* buffered, int* failStreak);

static std::string HandleDiagnosticsGetHookState(const BridgeRequest& req) {
    (void)req;
    BridgeHookState& hs = BridgeGetHookState();

    long uploaded = 0, failed = 0;
    unsigned buffered = 0;
    int failStreak = 0;
    ExamCapabilityGetStats(&uploaded, &failed, &buffered, &failStreak);

    std::string out = "{";
    out += "\"create_browser\":" + std::string(hs.createBrowser ? "true" : "false") + ",";
    out += "\"load_handler\":" + std::string(hs.loadHandler ? "true" : "false") + ",";
    out += "\"display_handler\":" + std::string(hs.displayHandler ? "true" : "false") + ",";
    out += "\"life_span_handler\":" + std::string(hs.lifeSpanHandler ? "true" : "false") + ",";
    out += "\"v8\":" + std::string(hs.v8 ? "true" : "false") + ",";
    out += "\"transport\":\"" + std::string(BridgeGetTransportName()) + "\",";
    out += "\"dll_loaded\":" + std::string(BridgeIsDllLoaded() ? "true" : "false") + ",";
    out += "\"script_injected\":" + std::string(BridgeIsScriptInjected() ? "true" : "false") + ",";
    out += "\"exam_uploaded\":" + std::to_string(uploaded) + ",";
    out += "\"exam_failed\":" + std::to_string(failed) + ",";
    out += "\"exam_buffered\":" + std::to_string(buffered) + ",";
    out += "\"exam_fail_streak\":" + std::to_string(failStreak);
    out += "}";
    return out;
}

/* ---- Registration ---- */
void RegisterDiagnosticsCapabilities() {
    BridgeRegisterMethod(LQ_METHOD_DIAG_LOG, HandleDiagnosticsLog);
    BridgeRegisterMethod(LQ_METHOD_DIAG_GET_HOOK_STATE, HandleDiagnosticsGetHookState);
}
