/* ============================================================================
 * page_capability.cpp — page.* bridge methods
 *
 * page.reportQuestionState  — receive question state from JS (AutoExam)
 * ========================================================================= */

#include "pch.h"
#include "bridge_dispatcher.h"
#include "bridge_protocol.h"
#include "bridge_security.h"

/* ---- page.reportQuestionState ---- */
static std::string HandlePageReportQuestionState(const BridgeRequest& req) {
    std::string qId = BridgeSanitizeLogField(
        BridgeJsonGetString(req.params, "questionId"), 128);
    std::string qType = BridgeSanitizeLogField(
        BridgeJsonGetString(req.params, "type"), 64);
    std::string state = BridgeSanitizeLogField(
        BridgeJsonGetString(req.params, "state"), 64);

    char dbg[512];
    sprintf_s(dbg, "[BRIDGE:PAGE] Question state: id=%s type=%s state=%s\n",
              qId.c_str(), qType.c_str(), state.c_str());
    OutputDebugStringA(dbg);

    return "{\"received\":true}";
}

/* ---- Registration ---- */
void RegisterPageCapabilities() {
    BridgeRegisterMethod(LQ_METHOD_PAGE_REPORT_Q_STATE, HandlePageReportQuestionState);
}
