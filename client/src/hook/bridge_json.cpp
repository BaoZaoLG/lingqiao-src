/* ============================================================================
 * bridge_json.cpp — Lightweight JSON helpers for Bridge v2 protocol
 * ========================================================================= */

#include "pch.h"
#include "bridge_json.h"
#include <sstream>
#include <cstdio>
#include <cctype>

/* ---- Timestamp ---- */

long long BridgeTimestampMs() {
    FILETIME ft;
    GetSystemTimeAsFileTime(&ft);
    ULARGE_INTEGER ul;
    ul.LowPart = ft.dwLowDateTime;
    ul.HighPart = ft.dwHighDateTime;
    /* FILETIME is 100ns intervals since 1601-01-01.
     * Convert to ms since 1970-01-01 (epoch diff = 11644473600 seconds). */
    return (long long)(ul.QuadPart / 10000ULL - 11644473600000ULL);
}

/* ---- JSON escape ---- */

std::string BridgeJsonEscape(const char* s) {
    if (!s) return "";
    std::string out;
    out.reserve(strlen(s) + 16);
    for (; *s; ++s) {
        switch (*s) {
        case '\\': out += "\\\\"; break;
        case '"':  out += "\\\""; break;
        case '\b': out += "\\b";  break;
        case '\f': out += "\\f";  break;
        case '\n': out += "\\n";  break;
        case '\r': out += "\\r";  break;
        case '\t': out += "\\t";  break;
        default:
            if ((unsigned char)*s < 0x20) {
                char buf[8];
                sprintf_s(buf, "\\u%04x", (unsigned char)*s);
                out += buf;
            } else {
                out += *s;
            }
        }
    }
    return out;
}

std::string BridgeJsonEscape(const std::string& s) {
    return BridgeJsonEscape(s.c_str());
}

/* ---- Minimal JSON string value extractor ---- */

/* Skip whitespace */
static size_t SkipWs(const std::string& s, size_t pos) {
    while (pos < s.size() && (s[pos] == ' ' || s[pos] == '\t' || s[pos] == '\r' || s[pos] == '\n'))
        ++pos;
    return pos;
}

/* Find a key in a JSON object at the top level.
 * Returns position after the colon, or std::string::npos if not found. */
static size_t FindKey(const std::string& json, const char* key, size_t start = 0) {
    std::string searchKey = std::string("\"") + key + "\"";
    size_t pos = start;
    int depth = 0;
    bool inString = false;

    while (pos < json.size()) {
        char c = json[pos];

        if (inString) {
            if (c == '\\' && pos + 1 < json.size()) {
                pos += 2;
                continue;
            }
            if (c == '"') inString = false;
            ++pos;
            continue;
        }

        if (c == '"') {
            /* Check if this string is our key */
            if (depth == 1 && json.compare(pos, searchKey.size(), searchKey) == 0) {
                pos += searchKey.size();
                pos = SkipWs(json, pos);
                if (pos < json.size() && json[pos] == ':') {
                    pos = SkipWs(json, pos + 1);
                    return pos;
                }
            }
            inString = true;
            ++pos;
            continue;
        }

        if (c == '{' || c == '[') ++depth;
        else if (c == '}' || c == ']') --depth;

        ++pos;
    }
    return std::string::npos;
}

/* Extract a JSON value starting at position pos as a raw string.
 * Handles strings, objects, arrays, numbers, booleans, null. */
static std::string ExtractValue(const std::string& json, size_t pos) {
    if (pos >= json.size()) return "";

    char c = json[pos];

    /* String value */
    if (c == '"') {
        std::string result = "\"";
        ++pos;
        while (pos < json.size()) {
            char ch = json[pos];
            result += ch;
            if (ch == '\\' && pos + 1 < json.size()) {
                result += json[pos + 1];
                pos += 2;
                continue;
            }
            if (ch == '"') return result;
            ++pos;
        }
        return result; /* unterminated string */
    }

    /* Object or array */
    if (c == '{' || c == '[') {
        char open = c, close = (c == '{') ? '}' : ']';
        int depth = 0;
        bool inStr = false;
        size_t start = pos;
        while (pos < json.size()) {
            char ch = json[pos];
            if (inStr) {
                if (ch == '\\' && pos + 1 < json.size()) { pos += 2; continue; }
                if (ch == '"') inStr = false;
            } else {
                if (ch == '"') inStr = true;
                else if (ch == open) ++depth;
                else if (ch == close) {
                    --depth;
                    if (depth == 0) return json.substr(start, pos - start + 1);
                }
            }
            ++pos;
        }
        return json.substr(start);
    }

    /* Number, boolean, null — read until delimiter */
    size_t start = pos;
    while (pos < json.size() && json[pos] != ',' && json[pos] != '}' && json[pos] != ']' &&
           json[pos] != ' ' && json[pos] != '\t' && json[pos] != '\r' && json[pos] != '\n') {
        ++pos;
    }
    return json.substr(start, pos - start);
}

/* Unescape a JSON string value (strip quotes and handle escapes) */
static std::string UnescapeJsonString(const std::string& s) {
    if (s.size() < 2 || s[0] != '"' || s[s.size() - 1] != '"') return "";
    std::string out;
    out.reserve(s.size());
    for (size_t i = 1; i < s.size() - 1; ++i) {
        if (s[i] == '\\' && i + 1 < s.size() - 1) {
            ++i;
            switch (s[i]) {
            case '"':  out += '"';  break;
            case '\\': out += '\\'; break;
            case '/':  out += '/';  break;
            case 'b':  out += '\b'; break;
            case 'f':  out += '\f'; break;
            case 'n':  out += '\n'; break;
            case 'r':  out += '\r'; break;
            case 't':  out += '\t'; break;
            case 'u':
                /* Simplified: keep as-is for non-ASCII */
                if (i + 4 < s.size() - 1) {
                    out += s.substr(i - 1, 6);
                    i += 4;
                }
                break;
            default: out += s[i]; break;
            }
        } else {
            out += s[i];
        }
    }
    return out;
}

static bool IsSafeIdentifier(const std::string& value, size_t maxLength) {
    if (value.empty() || value.size() > maxLength) return false;
    for (unsigned char ch : value) {
        if (!(std::isalnum(ch) || ch == '.' || ch == '_' || ch == '-' || ch == ':')) {
            return false;
        }
    }
    return true;
}

std::string BridgeJsonGetString(const std::string& json, const char* key) {
    size_t pos = FindKey(json, key);
    if (pos == std::string::npos) return "";
    std::string val = ExtractValue(json, pos);
    if (!val.empty() && val[0] == '"') {
        return UnescapeJsonString(val);
    }
    return val; /* raw value for non-strings (numbers, booleans, etc.) */
}

/* ---- Parse bridge request ---- */

std::string BridgeParseRequest(const std::string& raw, BridgeRequest* req) {
    if (!req) return LQ_ERR_INTERNAL;

    /* Size check */
    if (raw.size() > LQ_BRIDGE_MAX_MESSAGE_BYTES) {
        return LQ_ERR_PAYLOAD_TOO_LARGE;
    }

    /* Basic JSON validation — must start with '{' */
    size_t start = SkipWs(raw, 0);
    if (start >= raw.size() || raw[start] != '{') {
        return LQ_ERR_INVALID_JSON;
    }

    /* Extract required fields */
    std::string id = BridgeJsonGetString(raw, "id");
    std::string method = BridgeJsonGetString(raw, "method");
    std::string type = BridgeJsonGetString(raw, "type");

    if (id.empty() || method.empty()) {
        return LQ_ERR_INVALID_REQUEST;
    }
    if (!IsSafeIdentifier(id, 128) || !IsSafeIdentifier(method, 128)) {
        return LQ_ERR_INVALID_REQUEST;
    }
    if (type != LQ_TYPE_REQUEST) {
        return LQ_ERR_INVALID_REQUEST;
    }

    req->id = id;
    req->method = method;
    req->type = type;

    /* Extract params as raw JSON (keep the raw object for the handler) */
    size_t paramsPos = FindKey(raw, "params");
    if (paramsPos != std::string::npos) {
        req->params = ExtractValue(raw, paramsPos);
    } else {
        req->params = "{}";
    }
    const size_t paramsStart = SkipWs(req->params, 0);
    if (paramsStart >= req->params.size() || req->params[paramsStart] != '{') {
        return LQ_ERR_INVALID_REQUEST;
    }

    return ""; /* success */
}

/* ---- Response builders ---- */

std::string BridgeMakeSuccess(const std::string& id, const std::string& resultJson) {
    std::string out;
    out.reserve(256);
    out += "{";
    out += "\"v\":" + std::to_string(LQ_BRIDGE_VERSION) + ",";
    out += "\"id\":\"" + BridgeJsonEscape(id) + "\",";
    out += "\"type\":\"" LQ_TYPE_RESPONSE "\",";
    out += "\"ok\":true,";
    out += "\"result\":" + (resultJson.empty() ? "null" : resultJson) + ",";
    out += "\"ts\":" + std::to_string(BridgeTimestampMs());
    out += "}";
    return out;
}

std::string BridgeMakeError(const std::string& id, const char* code, const std::string& message) {
    std::string out;
    out.reserve(256);
    out += "{";
    out += "\"v\":" + std::to_string(LQ_BRIDGE_VERSION) + ",";
    out += "\"id\":\"" + BridgeJsonEscape(id) + "\",";
    out += "\"type\":\"" LQ_TYPE_RESPONSE "\",";
    out += "\"ok\":false,";
    out += "\"error\":{\"code\":\"" + BridgeJsonEscape(code) + "\",\"message\":\"" + BridgeJsonEscape(message) + "\"},";
    out += "\"ts\":" + std::to_string(BridgeTimestampMs());
    out += "}";
    return out;
}

std::string BridgeMakeEvent(const std::string& eventName, const std::string& payloadJson) {
    std::string out;
    out.reserve(256);
    out += "{";
    out += "\"v\":" + std::to_string(LQ_BRIDGE_VERSION) + ",";
    out += "\"type\":\"" LQ_TYPE_EVENT "\",";
    out += "\"method\":\"" + BridgeJsonEscape(eventName) + "\",";
    out += "\"params\":" + (payloadJson.empty() ? "{}" : payloadJson) + ",";
    out += "\"ts\":" + std::to_string(BridgeTimestampMs());
    out += "}";
    return out;
}
