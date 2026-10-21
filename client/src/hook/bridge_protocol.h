#pragma once
/* ============================================================================
 * bridge_protocol.h — Bridge v2 protocol constants, error codes, method names
 *
 * Shared by all bridge transport layers (console, V8, process message).
 * ========================================================================= */

/* Protocol version */
#define LQ_BRIDGE_VERSION           2
#define LQ_BRIDGE_VERSION_TEXT      "2.0"

/* Console transport prefix — JS sends messages via console.info with this prefix */
#define LQ_BRIDGE_CONSOLE_PREFIX    "__LQ_BRIDGE_V2__"
#define LQ_BRIDGE_CONSOLE_PREFIX_LEN 16

/* Maximum single message size (2 MB — vision payloads can be large) */
#define LQ_BRIDGE_MAX_MESSAGE_BYTES 2097152

/* ---- Error codes ---- */
#define LQ_ERR_INVALID_JSON         "INVALID_JSON"
#define LQ_ERR_INVALID_REQUEST      "INVALID_REQUEST"
#define LQ_ERR_METHOD_NOT_FOUND     "METHOD_NOT_FOUND"
#define LQ_ERR_METHOD_FAILED        "METHOD_FAILED"
#define LQ_ERR_PAYLOAD_TOO_LARGE    "PAYLOAD_TOO_LARGE"
#define LQ_ERR_TIMEOUT              "TIMEOUT"
#define LQ_ERR_INTERNAL             "INTERNAL_ERROR"

/* ---- Method names (whitelist) ---- */
#define LQ_METHOD_RUNTIME_PING          "runtime.ping"
#define LQ_METHOD_RUNTIME_GET_STATUS    "runtime.getStatus"
#define LQ_METHOD_RUNTIME_GET_CAPS      "runtime.getCapabilities"
#define LQ_METHOD_RUNTIME_GET_CONFIG    "runtime.getConfig"
#define LQ_METHOD_SCRIPT_GET_VERSION    "script.getVersion"
#define LQ_METHOD_DIAG_LOG              "diagnostics.log"
#define LQ_METHOD_DIAG_GET_HOOK_STATE   "diagnostics.getHookState"
#define LQ_METHOD_PAGE_REPORT_Q_STATE   "page.reportQuestionState"
#define LQ_METHOD_PAGE_REPORT_EXAM      "page.reportExamRecord"
#define LQ_METHOD_AI_CHAT_COMPLETE      "ai.chatComplete"
#define LQ_METHOD_AI_POLL_RESULT        "ai.pollResult"
#define LQ_METHOD_NATIVE_POLL_TRIGGER   "native.pollTrigger"

/* ---- Message types ---- */
#define LQ_TYPE_REQUEST     "request"
#define LQ_TYPE_RESPONSE    "response"
#define LQ_TYPE_EVENT       "event"

/* ---- CEF Process Message names ---- */
#define LQ_PM_REQUEST   "LingQiaoBridge.Request"
#define LQ_PM_RESPONSE  "LingQiaoBridge.Response"
