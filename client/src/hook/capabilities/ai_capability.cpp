/* ============================================================================
 * ai_capability.cpp — ai.chatComplete bridge method
 *
 * Runs the OpenAI-compatible chat completion inside the injected DLL so the
 * page never sees the API key. AutoExam.js only sends { messages, max_tokens }.
 * ========================================================================= */

#include "pch.h"

#include <string>

#include "bridge_dispatcher.h"
#include "bridge_protocol.h"
#include "bridge_json.h"
#include "secret_channel.h"
#include "script_loader.h" /* LqUtf8ToWide, HookVerifyServerCert */

#include <winhttp.h>
#pragma comment(lib, "winhttp.lib")

namespace {

std::string BuildChatUrl(const std::string& baseUrl) {
    std::string u = baseUrl;
    while (!u.empty() && (u.back() == '/' || u.back() == '\\')) u.pop_back();
    if (u.empty()) return {};
    /* If already ends with /chat/completions keep it */
    const char* suffix = "/chat/completions";
    size_t sl = strlen(suffix);
    if (u.size() >= sl) {
        std::string tail = u.substr(u.size() - sl);
        for (auto& c : tail) c = (char)tolower((unsigned char)c);
        if (tail == suffix) return u;
    }
    return u + suffix;
}

/* Minimal extract: "max_tokens": N */
int ExtractIntParam(const std::string& params, const char* key, int fallback) {
    std::string needle = std::string("\"") + key + "\":";
    size_t p = params.find(needle);
    if (p == std::string::npos) return fallback;
    p += needle.size();
    while (p < params.size() && (params[p] == ' ' || params[p] == '\t')) ++p;
    bool neg = false;
    if (p < params.size() && params[p] == '-') { neg = true; ++p; }
    long v = 0;
    bool any = false;
    while (p < params.size() && params[p] >= '0' && params[p] <= '9') {
        v = v * 10 + (params[p] - '0');
        ++p;
        any = true;
    }
    if (!any) return fallback;
    return (int)(neg ? -v : v);
}

double ExtractDoubleParam(const std::string& params, const char* key, double fallback) {
    std::string needle = std::string("\"") + key + "\":";
    size_t p = params.find(needle);
    if (p == std::string::npos) return fallback;
    p += needle.size();
    while (p < params.size() && (params[p] == ' ' || params[p] == '\t')) ++p;
    size_t start = p;
    while (p < params.size() && ((params[p] >= '0' && params[p] <= '9') || params[p] == '.' || params[p] == '-')) ++p;
    if (p == start) return fallback;
    return atof(params.substr(start, p - start).c_str());
}

} // namespace

static std::string HandleAiChatComplete(const BridgeRequest& req) {
    OutputDebugStringA("[AI] chatComplete enter\n");
    const LqSecretBundle& secrets = LqSecretsCached();
    {
        char dbg[160];
        sprintf_s(dbg, "[AI] chatComplete secrets apiKey=%d baseUrl=%d model=%d id=%s\n",
                  secrets.apiKey.empty() ? 0 : 1,
                  secrets.baseUrl.empty() ? 0 : 1,
                  secrets.model.empty() ? 0 : 1,
                  req.id.c_str());
        OutputDebugStringA(dbg);
    }
    if (secrets.apiKey.empty() || secrets.baseUrl.empty()) {
        OutputDebugStringA("[AI] chatComplete: secrets not loaded\n");
        return std::string("__ERROR__:") + LQ_ERR_METHOD_FAILED + "|AI secret not loaded";
    }

    std::string messagesJson = BridgeJsonGetString(req.params, "messages");
    if (messagesJson.empty() || messagesJson[0] != '[') {
        char dbg[96];
        sprintf_s(dbg, "[AI] chatComplete: bad messages head=%c size=%zu\n",
                  messagesJson.empty() ? '?' : messagesJson[0], messagesJson.size());
        OutputDebugStringA(dbg);
        return std::string("__ERROR__:") + LQ_ERR_INVALID_REQUEST + "|messages array required";
    }
    {
        char dbg[128];
        sprintf_s(dbg, "[AI] chatComplete: messages_len=%zu model=%d\n",
                  messagesJson.size(), secrets.model.empty() ? 0 : 1);
        OutputDebugStringA(dbg);
    }

    int maxTokens = ExtractIntParam(req.params, "max_tokens", 1024);
    if (maxTokens < 1) maxTokens = 1024;
    if (maxTokens > 16384) maxTokens = 16384;
    double temperature = ExtractDoubleParam(req.params, "temperature", 0.1);

    std::string model = secrets.model;
    if (model.empty()) {
        return std::string("__ERROR__:") + LQ_ERR_METHOD_FAILED + "|model not configured";
    }

    std::string body;
    body.reserve(messagesJson.size() + 256);
    body += "{\"model\":\"";
    for (char c : model) {
        if (c == '"' || c == '\\') body += '\\';
        body += c;
    }
    body += "\",\"messages\":";
    body += messagesJson;
    body += ",\"temperature\":";
    char num[32];
    sprintf_s(num, "%.2f", temperature);
    body += num;
    body += ",\"max_tokens\":";
    sprintf_s(num, "%d", maxTokens);
    body += num;
    body += ",\"stream\":false}";

    std::wstring urlWide = LqUtf8ToWide(BuildChatUrl(secrets.baseUrl));
    if (urlWide.empty()) {
        return std::string("__ERROR__:") + LQ_ERR_INTERNAL + "|bad base url";
    }
    {
        char hostUtf8[256] = {0};
        WideCharToMultiByte(CP_UTF8, 0, urlWide.c_str(), -1, hostUtf8, sizeof(hostUtf8), NULL, NULL);
        char dbg[320];
        sprintf_s(dbg, "[AI] chatComplete target url=%s body_len=%zu\n", hostUtf8, body.size());
        OutputDebugStringA(dbg);
    }

    URL_COMPONENTSW uc = {0};
    uc.dwStructSize = sizeof(uc);
    wchar_t host[256] = {0};
    wchar_t path[1024] = {0};
    uc.lpszHostName = host;
    uc.dwHostNameLength = _countof(host);
    uc.lpszUrlPath = path;
    uc.dwUrlPathLength = _countof(path);
    if (!WinHttpCrackUrl(urlWide.c_str(), 0, 0, &uc)) {
        return std::string("__ERROR__:") + LQ_ERR_INTERNAL + "|url parse failed";
    }
    bool secure = (uc.nScheme == INTERNET_SCHEME_HTTPS);
    INTERNET_PORT port = uc.nPort ? uc.nPort : (secure ? INTERNET_DEFAULT_HTTPS_PORT : INTERNET_DEFAULT_HTTP_PORT);
    /* path may include extra query — keep as-is */

    HINTERNET hSession = WinHttpOpen(L"LQ-AI/1.0", WINHTTP_ACCESS_TYPE_DEFAULT_PROXY, NULL, NULL, 0);
    if (!hSession) return std::string("__ERROR__:") + LQ_ERR_INTERNAL + "|winhttp open";
    WinHttpSetTimeouts(hSession, 10000, 10000, 90000, 120000);

    HINTERNET hConnect = WinHttpConnect(hSession, host, port, 0);
    if (!hConnect) {
        WinHttpCloseHandle(hSession);
        return std::string("__ERROR__:") + LQ_ERR_INTERNAL + "|winhttp connect";
    }

    HINTERNET hRequest = WinHttpOpenRequest(
        hConnect, L"POST", path, NULL, WINHTTP_NO_REFERER,
        WINHTTP_DEFAULT_ACCEPT_TYPES, secure ? WINHTTP_FLAG_SECURE : 0);
    if (!hRequest) {
        WinHttpCloseHandle(hConnect);
        WinHttpCloseHandle(hSession);
        return std::string("__ERROR__:") + LQ_ERR_INTERNAL + "|winhttp request";
    }

    std::wstring apiKeyWide = LqUtf8ToWide(secrets.apiKey);
    std::wstring auth = L"Authorization: Bearer " + apiKeyWide;
    std::wstring headers =
        L"Content-Type: application/json\r\n" + auth + L"\r\n";
    WinHttpAddRequestHeaders(hRequest, headers.c_str(), (DWORD)-1, WINHTTP_ADDREQ_FLAG_ADD);

    if (secure) {
        DWORD secFlags = SECURITY_FLAG_IGNORE_UNKNOWN_CA;
        WinHttpSetOption(hRequest, WINHTTP_OPTION_SECURITY_FLAGS, &secFlags, sizeof(secFlags));
    }

    std::string result;
    bool sent = WinHttpSendRequest(hRequest, WINHTTP_NO_ADDITIONAL_HEADERS, 0,
                                   (LPVOID)body.data(), (DWORD)body.size(),
                                   (DWORD)body.size(), 0) != FALSE;
    {
        char dbg[128];
        sprintf_s(dbg, "[AI] chatComplete WinHttpSendRequest ok=%d err=%lu\n",
                  sent ? 1 : 0, sent ? 0UL : GetLastError());
        OutputDebugStringA(dbg);
    }
    bool recvd = sent && WinHttpReceiveResponse(hRequest, NULL) != FALSE;
    {
        char dbg[128];
        sprintf_s(dbg, "[AI] chatComplete WinHttpReceiveResponse ok=%d err=%lu\n",
                  recvd ? 1 : 0, recvd ? 0UL : GetLastError());
        OutputDebugStringA(dbg);
    }

    if (recvd) {
        DWORD status = 0, statusSize = sizeof(status);
        WinHttpQueryHeaders(hRequest,
                            WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
                            NULL, &status, &statusSize, NULL);
        {
            char dbg[96];
            sprintf_s(dbg, "[AI] chatComplete HTTP status=%lu\n", (unsigned long)status);
            OutputDebugStringA(dbg);
        }

        std::string respBody;
        for (;;) {
            DWORD avail = 0;
            if (!WinHttpQueryDataAvailable(hRequest, &avail) || avail == 0) break;
            std::string chunk(avail, '\0');
            DWORD rd = 0;
            if (!WinHttpReadData(hRequest, &chunk[0], avail, &rd) || rd == 0) break;
            chunk.resize(rd);
            respBody += chunk;
        }

        if (status == 200) {
            /* Extract first choice message content + finish_reason */
            std::string content;
            std::string finish = "stop";
            size_t choices = respBody.find("\"choices\"");
            if (choices != std::string::npos) {
                size_t msg = respBody.find("\"content\"", choices);
                if (msg != std::string::npos) {
                    size_t colon = respBody.find(':', msg);
                    size_t q1 = respBody.find('"', colon + 1);
                    if (q1 != std::string::npos) {
                        size_t i = q1 + 1;
                        while (i < respBody.size()) {
                            if (respBody[i] == '\\' && i + 1 < respBody.size()) {
                                char n = respBody[i + 1];
                                if (n == 'n') content += '\n';
                                else if (n == 't') content += '\t';
                                else if (n == 'r') content += '\r';
                                else if (n == '"' || n == '\\' || n == '/') content += n;
                                else if (n == 'u' && i + 5 < respBody.size()) {
                                    /* keep as-is for simplicity */
                                    content += 'u';
                                } else content += n;
                                i += 2;
                                continue;
                            }
                            if (respBody[i] == '"') break;
                            content += respBody[i];
                            ++i;
                        }
                    }
                }
                size_t fr = respBody.find("\"finish_reason\"", choices);
                if (fr != std::string::npos) {
                    size_t q1 = respBody.find('"', respBody.find(':', fr) + 1);
                    size_t q2 = respBody.find('"', q1 + 1);
                    if (q1 != std::string::npos && q2 != std::string::npos)
                        finish = respBody.substr(q1 + 1, q2 - q1 - 1);
                }
            }

            /* Build JSON response for JS */
            std::string escaped;
            escaped.reserve(content.size() + 16);
            for (unsigned char c : content) {
                switch (c) {
                case '\\': escaped += "\\\\"; break;
                case '"':  escaped += "\\\""; break;
                case '\n': escaped += "\\n"; break;
                case '\r': escaped += "\\r"; break;
                case '\t': escaped += "\\t"; break;
                default:
                    if (c < 0x20) {
                        char b[8];
                        sprintf_s(b, "\\u%04x", c);
                        escaped += b;
                    } else escaped += (char)c;
                }
            }
            result = "{\"content\":\"" + escaped + "\",\"finish_reason\":\"" + finish + "\"}";
        } else if (status == 401 || status == 403) {
            result = std::string("__ERROR__:HTTP ") + std::to_string(status) + "|auth rejected";
        } else if (status == 429) {
            result = std::string("__ERROR__:HTTP 429:rate limited");
        } else if (status >= 500) {
            result = std::string("__ERROR__:HTTP ") + std::to_string(status) + "|upstream";
        } else {
            /* include truncated body for debug */
            std::string brief = respBody.substr(0, 180);
            for (char& c : brief) if ((unsigned char)c < 0x20) c = ' ';
            result = std::string("__ERROR__:HTTP ") + std::to_string(status) + "|" + brief;
        }
    } else {
        result = std::string("__ERROR__:") + LQ_ERR_INTERNAL + "|network";
    }

    WinHttpCloseHandle(hRequest);
    WinHttpCloseHandle(hConnect);
    WinHttpCloseHandle(hSession);
    return result;
}

void RegisterAiCapabilities() {
    BridgeRegisterMethod(LQ_METHOD_AI_CHAT_COMPLETE, HandleAiChatComplete);
}
