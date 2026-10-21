#pragma once
/* ============================================================================
 * bridge_json.h — Lightweight JSON helpers for Bridge v2 protocol
 *
 * Provides minimal JSON parsing and generation for the bridge RPC envelope.
 * Not a general-purpose JSON library — only handles the bridge protocol format.
 * ========================================================================= */

#include <string>
#include "bridge_protocol.h"

/* Parsed bridge request (from JS -> DLL) */
struct BridgeRequest {
    std::string id;         /* RPC id (e.g. "rpc_1720000000000_1") */
    std::string method;     /* Method name (e.g. "runtime.ping") */
    std::string params;     /* Raw JSON params object (may be "{}") */
    std::string type;       /* "request" */
};

/* ---- Parsing ---- */

/* Parse a raw JSON string into a BridgeRequest.
 * Returns empty string on success, or an error code (LQ_ERR_*) on failure. */
std::string BridgeParseRequest(const std::string& raw, BridgeRequest* req);

/* ---- Response generation ---- */

/* Build a success response envelope JSON string.
 * resultJson is the raw JSON value for the "result" field. */
std::string BridgeMakeSuccess(const std::string& id, const std::string& resultJson);

/* Build an error response envelope JSON string. */
std::string BridgeMakeError(const std::string& id, const char* code, const std::string& message);

/* Build an event envelope JSON string (DLL -> JS push). */
std::string BridgeMakeEvent(const std::string& eventName, const std::string& payloadJson);

/* ---- Utilities ---- */

/* JSON-escape a string value (handles \, ", control chars, Unicode). */
std::string BridgeJsonEscape(const char* s);
std::string BridgeJsonEscape(const std::string& s);

/* Extract a string value for a given key from a flat JSON object.
 * Returns empty string if not found. Handles simple cases only. */
std::string BridgeJsonGetString(const std::string& json, const char* key);

/* Get current timestamp in milliseconds. */
long long BridgeTimestampMs();
