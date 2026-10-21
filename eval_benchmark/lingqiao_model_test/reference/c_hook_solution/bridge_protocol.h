#pragma once
/* ============================================================================
 * bridge_protocol.h — Bridge v2 协议常量、错误码、方法名
 *
 * 所有 Bridge 传输层（console, V8, IPC）共享的常量定义。
 * ========================================================================= */

/* 协议版本 */
#define LQ_BRIDGE_VERSION           2
#define LQ_BRIDGE_VERSION_TEXT      "2.0"

/* 最大单条消息大小 (64 KB) */
#define LQ_BRIDGE_MAX_MESSAGE_BYTES 65536

/* ---- 错误码 ---- */
#define LQ_ERR_INVALID_JSON         "INVALID_JSON"
#define LQ_ERR_INVALID_REQUEST      "INVALID_REQUEST"
#define LQ_ERR_METHOD_NOT_FOUND     "METHOD_NOT_FOUND"
#define LQ_ERR_METHOD_FAILED        "METHOD_FAILED"
#define LQ_ERR_PAYLOAD_TOO_LARGE    "PAYLOAD_TOO_LARGE"
#define LQ_ERR_TIMEOUT              "TIMEOUT"
#define LQ_ERR_INTERNAL             "INTERNAL_ERROR"
#define LQ_ERR_DECRYPT_FAILED       "DECRYPT_FAILED"
#define LQ_ERR_ENCRYPT_FAILED       "ENCRYPT_FAILED"

/* ---- 方法名（白名单） ---- */
#define LQ_METHOD_RUNTIME_PING          "runtime.ping"
#define LQ_METHOD_RUNTIME_GET_STATUS    "runtime.getStatus"
#define LQ_METHOD_RUNTIME_GET_CAPS      "runtime.getCapabilities"
#define LQ_METHOD_RUNTIME_GET_CONFIG    "runtime.getConfig"
#define LQ_METHOD_SCRIPT_GET_VERSION    "script.getVersion"
#define LQ_METHOD_DIAG_LOG              "diagnostics.log"
#define LQ_METHOD_DIAG_GET_HOOK_STATE   "diagnostics.getHookState"
#define LQ_METHOD_PAGE_REPORT_STATE     "page.reportState"

/* ---- 消息类型 ---- */
#define LQ_TYPE_REQUEST     "request"
#define LQ_TYPE_RESPONSE    "response"
#define LQ_TYPE_EVENT       "event"
