#pragma once
// ============================================================================
// HTTP Client — WinHTTP wrapper with HMAC signing and cert pinning
// ============================================================================
#include <windows.h>
#include <winhttp.h>
#include <functional>
#include <QString>
#include <QByteArray>
#include "crypto.h"
#include "config.h"
#include "strcrypt.h"

struct HttpResponse { int statusCode; QByteArray body; QString error; };

// RAII wrapper for WinHTTP handles
class WinHttpHandle {
    HINTERNET h_ = nullptr;
public:
    WinHttpHandle() = default;
    explicit WinHttpHandle(HINTERNET h) : h_(h) {}
    ~WinHttpHandle() { if (h_) WinHttpCloseHandle(h_); }
    WinHttpHandle(const WinHttpHandle&) = delete;
    WinHttpHandle& operator=(const WinHttpHandle&) = delete;
    WinHttpHandle(WinHttpHandle&& o) noexcept : h_(o.h_) { o.h_ = nullptr; }
    WinHttpHandle& operator=(WinHttpHandle&& o) noexcept {
        if (this != &o) { if (h_) WinHttpCloseHandle(h_); h_ = o.h_; o.h_ = nullptr; }
        return *this;
    }
    HINTERNET get() const { return h_; }
    explicit operator bool() const { return h_ != nullptr; }
    HINTERNET release() { auto t = h_; h_ = nullptr; return t; }
};

static HttpResponse HttpPostJson(const wchar_t* host, int port,
                                  const wchar_t* path, const QByteArray& body)
{
    HttpResponse result = {0, QByteArray()};
    // Anti-replay: timestamp + nonce included in HMAC signature
    char tsBuf[32]; _i64toa(GetUnixTimestamp(), tsBuf, 10);
    char nonceHex[33]; GenerateNonce(nonceHex);
    BYTE sig[32]; DWORD sigLen = 0;
    HmacSha256Signed((const char*)HMAC_KEY, 32,
                     tsBuf, nonceHex, body.constData(), (DWORD)body.size(), sig, &sigLen);
    char sigHex[65]; ByteToHex(sig, sigLen, sigHex);
    wchar_t headers[1024];
    swprintf_s(headers, sizeof(headers)/sizeof(wchar_t),
        L"Content-Type: application/json\r\n"
        L"X-Client-ID: %s\r\n"
        L"X-HMAC-Signature: %S\r\n"
        L"X-Timestamp: %S\r\n"
        L"X-Nonce: %S\r\n", CLIENT_ID, sigHex, tsBuf, nonceHex);
    HINTERNET hSession = WinHttpOpen(_WS(L"CefBridge/2.0"),
        WINHTTP_ACCESS_TYPE_DEFAULT_PROXY, NULL, NULL, 0);
    if (!hSession) { result.error = QString::fromUtf8(_S("无法初始化网络 (错误: %1)")).arg(GetLastError()); return result; }
    HINTERNET hConnect = WinHttpConnect(hSession, host, (INTERNET_PORT)port, 0);
    if (!hConnect) { DWORD e = GetLastError(); WinHttpCloseHandle(hSession);
        result.error = (e == ERROR_WINHTTP_NAME_NOT_RESOLVED) ? QString::fromUtf8(_S("DNS 解析失败"))
            : QString::fromUtf8(_S("连接服务器失败 (错误: %1)")).arg(e); return result; }
    HINTERNET hRequest = WinHttpOpenRequest(hConnect, L"POST", path, NULL, NULL, NULL, WINHTTP_FLAG_SECURE);
    if (!hRequest) { WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        result.error = QString::fromUtf8(_S("创建请求失败")); return result; }
    DWORD redirectPolicy = WINHTTP_OPTION_REDIRECT_POLICY_DISALLOW_HTTPS_TO_HTTP;
    WinHttpSetOption(hRequest, WINHTTP_OPTION_REDIRECT_POLICY, &redirectPolicy, sizeof(redirectPolicy));
    // Allow self-signed certs for TLS handshake; we verify via fingerprint pinning below
    DWORD secFlags = SECURITY_FLAG_IGNORE_UNKNOWN_CA
                   | SECURITY_FLAG_IGNORE_CERT_CN_INVALID
                   | SECURITY_FLAG_IGNORE_CERT_DATE_INVALID;
    WinHttpSetOption(hRequest, WINHTTP_OPTION_SECURITY_FLAGS, &secFlags, sizeof(secFlags));
    WinHttpSetTimeouts(hRequest, 5000, 5000, 5000, 10000);
    if (!WinHttpSendRequest(hRequest, headers, (DWORD)wcslen(headers),
        (LPVOID)body.constData(), (DWORD)body.size(), (DWORD)body.size(), 0)) {
        DWORD e = GetLastError();
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        result.error = (e == ERROR_WINHTTP_TIMEOUT) ? QString::fromUtf8(_S("发送请求超时"))
            : QString::fromUtf8(_S("发送请求失败 (错误: %1)")).arg(e); return result;
    }
    if (!WinHttpReceiveResponse(hRequest, NULL)) {
        DWORD e = GetLastError();
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        result.error = (e == ERROR_WINHTTP_TIMEOUT) ? QString::fromUtf8(_S("接收响应超时"))
            : QString::fromUtf8(_S("接收响应失败 (错误: %1)")).arg(e); return result;
    }
    // Certificate fingerprint pinning — block MITM (callback did early check, this is fallback)
    if (!VerifyServerCert(hRequest)) {
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        result.error = QString::fromUtf8(_S("证书验证失败 — 可能遭受中间人攻击")); return result;
    }
    DWORD statusCode = 0, statusCodeSize = sizeof(statusCode);
    WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
        NULL, &statusCode, &statusCodeSize, NULL);
    result.statusCode = (int)statusCode;
    char buf[4096]; DWORD bytesRead = 0;
    while (WinHttpReadData(hRequest, buf, sizeof(buf) - 1, &bytesRead) && bytesRead > 0) {
        buf[bytesRead] = 0;
        result.body.append(buf, (int)bytesRead);
    }
    WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
    return result;
}

static HttpResponse WinHttpGetRaw(const wchar_t* host, int port, const wchar_t* path)
{
    HttpResponse result = {0, QByteArray()};
    HINTERNET hSession = WinHttpOpen(_WS(L"CefBridge/2.0"),
        WINHTTP_ACCESS_TYPE_DEFAULT_PROXY, NULL, NULL, 0);
    if (!hSession) return result;
    HINTERNET hConnect = WinHttpConnect(hSession, host, (INTERNET_PORT)port, 0);
    if (!hConnect) { WinHttpCloseHandle(hSession); return result; }
    DWORD reqFlags = WINHTTP_FLAG_SECURE;
    HINTERNET hRequest = WinHttpOpenRequest(hConnect, L"GET", path, NULL, NULL, NULL, reqFlags);
    if (!hRequest) { WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession); return result; }
    DWORD secFlags = SECURITY_FLAG_IGNORE_UNKNOWN_CA
                   | SECURITY_FLAG_IGNORE_CERT_CN_INVALID
                   | SECURITY_FLAG_IGNORE_CERT_DATE_INVALID;
    WinHttpSetOption(hRequest, WINHTTP_OPTION_SECURITY_FLAGS, &secFlags, sizeof(secFlags));
    WinHttpSetTimeouts(hRequest, 5000, 5000, 5000, 10000);
    if (!WinHttpSendRequest(hRequest, NULL, 0, NULL, 0, 0, 0)) {
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        return result;
    }
    if (!WinHttpReceiveResponse(hRequest, NULL)) {
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        return result;
    }
    if (!VerifyServerCert(hRequest)) {
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        return result;
    }
    DWORD statusCode = 0, statusCodeSize = sizeof(statusCode);
    WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
        NULL, &statusCode, &statusCodeSize, NULL);
    result.statusCode = (int)statusCode;
    char buf[4096]; DWORD bytesRead = 0;
    while (WinHttpReadData(hRequest, buf, sizeof(buf) - 1, &bytesRead) && bytesRead > 0) {
        buf[bytesRead] = 0;
        result.body.append(buf, (int)bytesRead);
    }
    WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
    return result;
}

static HttpResponse WinHttpGet(const wchar_t* host, int port, const wchar_t* path)
{
    HttpResponse result = {0, QByteArray()};
    HINTERNET hSession = WinHttpOpen(_WS(L"CefBridge/2.0"),
        WINHTTP_ACCESS_TYPE_DEFAULT_PROXY, NULL, NULL, 0);
    if (!hSession) return result;
    HINTERNET hConnect = WinHttpConnect(hSession, host, (INTERNET_PORT)port, 0);
    if (!hConnect) { WinHttpCloseHandle(hSession); return result; }
    DWORD reqFlags = WINHTTP_FLAG_SECURE;
    HINTERNET hRequest = WinHttpOpenRequest(hConnect, L"GET", path, NULL, NULL, NULL, reqFlags);
    if (!hRequest) { WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession); return result; }
    DWORD secFlags = SECURITY_FLAG_IGNORE_UNKNOWN_CA
                   | SECURITY_FLAG_IGNORE_CERT_CN_INVALID
                   | SECURITY_FLAG_IGNORE_CERT_DATE_INVALID;
    WinHttpSetOption(hRequest, WINHTTP_OPTION_SECURITY_FLAGS, &secFlags, sizeof(secFlags));
    WinHttpSetTimeouts(hRequest, 5000, 5000, 5000, 10000);
    if (!WinHttpSendRequest(hRequest, NULL, 0, NULL, 0, 0, 0)) {
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        return result;
    }
    if (!WinHttpReceiveResponse(hRequest, NULL)) {
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        return result;
    }
    if (!VerifyServerCert(hRequest)) {
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        return result;
    }
    DWORD statusCode = 0, statusCodeSize = sizeof(statusCode);
    WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
        NULL, &statusCode, &statusCodeSize, NULL);
    result.statusCode = (int)statusCode;
    char buf[4096]; DWORD bytesRead = 0;
    while (WinHttpReadData(hRequest, buf, sizeof(buf) - 1, &bytesRead) && bytesRead > 0) {
        buf[bytesRead] = 0;
        result.body.append(buf, (int)bytesRead);
    }
    WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
    return result;
}

// Signed HTTPS GET with HMAC headers (for authenticated downloads like DLL)
static HttpResponse WinHttpGetSigned(const wchar_t* host, int port, const wchar_t* path,
                                     const wchar_t* sessionToken = nullptr)
{
    HttpResponse result = {0, QByteArray()};
    HINTERNET hSession = WinHttpOpen(_WS(L"CefBridge/2.0"),
        WINHTTP_ACCESS_TYPE_DEFAULT_PROXY, NULL, NULL, 0);
    if (!hSession) { result.error = QString::fromUtf8(_S("无法初始化网络")); return result; }
    HINTERNET hConnect = WinHttpConnect(hSession, host, (INTERNET_PORT)port, 0);
    if (!hConnect) { WinHttpCloseHandle(hSession); result.error = QString::fromUtf8(_S("连接服务器失败")); return result; }
    HINTERNET hRequest = WinHttpOpenRequest(hConnect, L"GET", path, NULL, NULL, NULL, WINHTTP_FLAG_SECURE);
    if (!hRequest) { WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession); result.error = QString::fromUtf8(_S("创建请求失败")); return result; }

    // HMAC sign the path for authenticated GET
    char tsBuf[32]; _i64toa(GetUnixTimestamp(), tsBuf, 10);
    char nonceHex[33]; GenerateNonce(nonceHex);
    BYTE sig[32]; DWORD sigLen = 0;
    char pathUtf8[256];
    WideCharToMultiByte(CP_UTF8, 0, path, -1, pathUtf8, sizeof(pathUtf8), NULL, NULL);
    HmacSha256Signed((const char*)HMAC_KEY, 32,
                     tsBuf, nonceHex, pathUtf8, (DWORD)strlen(pathUtf8), sig, &sigLen);
    char sigHex[65]; ByteToHex(sig, sigLen, sigHex);
    wchar_t authHeaders[640];
    int off = swprintf_s(authHeaders, sizeof(authHeaders)/sizeof(wchar_t),
        L"X-Client-ID: %s\r\nX-HMAC-Signature: %S\r\nX-Timestamp: %S\r\nX-Nonce: %S\r\n",
        CLIENT_ID, sigHex, tsBuf, nonceHex);
    if (sessionToken && sessionToken[0]) {
        swprintf_s(authHeaders + off, (sizeof(authHeaders)/sizeof(wchar_t)) - off,
            L"X-Session-Token: %s\r\n", sessionToken);
    }
    WinHttpAddRequestHeaders(hRequest, authHeaders, (DWORD)wcslen(authHeaders), WINHTTP_ADDREQ_FLAG_ADD);

    DWORD secFlags = SECURITY_FLAG_IGNORE_UNKNOWN_CA
                   | SECURITY_FLAG_IGNORE_CERT_CN_INVALID
                   | SECURITY_FLAG_IGNORE_CERT_DATE_INVALID;
    WinHttpSetOption(hRequest, WINHTTP_OPTION_SECURITY_FLAGS, &secFlags, sizeof(secFlags));
    WinHttpSetTimeouts(hRequest, 15000, 15000, 30000, 120000);
    if (!WinHttpSendRequest(hRequest, NULL, 0, NULL, 0, 0, 0)) {
        DWORD e = GetLastError();
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        result.error = (e == ERROR_WINHTTP_TIMEOUT) ? QString::fromUtf8(_S("发送请求超时"))
            : QString::fromUtf8(_S("发送请求失败 (错误: %1)")).arg(e); return result;
    }
    if (!WinHttpReceiveResponse(hRequest, NULL)) {
        DWORD e = GetLastError();
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        result.error = (e == ERROR_WINHTTP_TIMEOUT) ? QString::fromUtf8(_S("接收响应超时"))
            : QString::fromUtf8(_S("接收响应失败 (错误: %1)")).arg(e); return result;
    }
    if (!VerifyServerCert(hRequest)) {
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        result.error = QString::fromUtf8(_S("证书验证失败")); return result;
    }
    DWORD statusCode = 0, statusCodeSize = sizeof(statusCode);
    WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
        NULL, &statusCode, &statusCodeSize, NULL);
    result.statusCode = (int)statusCode;
    char buf[4096]; DWORD bytesRead = 0;
    while (WinHttpReadData(hRequest, buf, sizeof(buf) - 1, &bytesRead) && bytesRead > 0) {
        buf[bytesRead] = 0;
        result.body.append(buf, (int)bytesRead);
    }
    WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
    return result;
}

// Generic HTTPS GET with Bearer token auth (for external APIs like DeepSeek)
static HttpResponse HttpGetBearer(const wchar_t* host, const wchar_t* path, const QString& bearerToken)
{
    HttpResponse result = {0, QByteArray()};
    WinHttpHandle hSession(WinHttpOpen(_WS(L"CefBridge/2.0"),
        WINHTTP_ACCESS_TYPE_DEFAULT_PROXY, NULL, NULL, 0));
    if (!hSession) { result.error = QString::fromUtf8(_S("无法初始化网络")); return result; }
    WinHttpHandle hConnect(WinHttpConnect(hSession.get(), host, INTERNET_DEFAULT_HTTPS_PORT, 0));
    if (!hConnect) { result.error = QString::fromUtf8(_S("连接服务器失败")); return result; }
    WinHttpHandle hRequest(WinHttpOpenRequest(hConnect.get(), L"GET", path, NULL, NULL, NULL, WINHTTP_FLAG_SECURE));
    if (!hRequest) { result.error = QString::fromUtf8(_S("创建请求失败")); return result; }
    wchar_t authHeader[512];
    swprintf_s(authHeader, sizeof(authHeader)/sizeof(wchar_t),
        L"Authorization: Bearer %s\r\nAccept: application/json", (const wchar_t*)bearerToken.utf16());
    WinHttpAddRequestHeaders(hRequest.get(), authHeader, (DWORD)wcslen(authHeader), WINHTTP_ADDREQ_FLAG_ADD);
    // External API: use system CA store for certificate validation (no IGNORE flags)
    WinHttpSetTimeouts(hRequest.get(), 5000, 5000, 5000, 10000);
    if (!WinHttpSendRequest(hRequest.get(), NULL, 0, NULL, 0, 0, 0)) {
        DWORD e = GetLastError();
        result.error = (e == ERROR_WINHTTP_TIMEOUT) ? QString::fromUtf8(_S("请求超时"))
            : QString::fromUtf8(_S("请求失败 (错误: %1)")).arg(e); return result;
    }
    if (!WinHttpReceiveResponse(hRequest.get(), NULL)) {
        DWORD e = GetLastError();
        result.error = (e == ERROR_WINHTTP_TIMEOUT) ? QString::fromUtf8(_S("响应超时"))
            : QString::fromUtf8(_S("接收响应失败")); return result;
    }
    DWORD statusCode = 0, statusCodeSize = sizeof(statusCode);
    WinHttpQueryHeaders(hRequest.get(), WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
        NULL, &statusCode, &statusCodeSize, NULL);
    result.statusCode = (int)statusCode;
    char buf[4096]; DWORD bytesRead = 0;
    while (WinHttpReadData(hRequest.get(), buf, sizeof(buf) - 1, &bytesRead) && bytesRead > 0) {
        buf[bytesRead] = 0;
        result.body.append(buf, (int)bytesRead);
    }
    return result;
}

// Download a file from the server, save to localPath. Returns empty string on success, error message on failure.
static QString HttpDownloadFile(const wchar_t* host, int port, const wchar_t* path,
                                const wchar_t* localPath, std::function<void(qint64 bytesRead, qint64 total)> progressCb)
{
    HINTERNET hSession = WinHttpOpen(_WS(L"CefBridge/2.0"),
        WINHTTP_ACCESS_TYPE_DEFAULT_PROXY, NULL, NULL, 0);
    if (!hSession) return QString::fromUtf8(_S("无法初始化网络"));

    HINTERNET hConnect = WinHttpConnect(hSession, host, (INTERNET_PORT)port, 0);
    if (!hConnect) { WinHttpCloseHandle(hSession); return QString::fromUtf8(_S("连接服务器失败")); }

    HINTERNET hRequest = WinHttpOpenRequest(hConnect, L"GET", path, NULL, NULL, NULL, WINHTTP_FLAG_SECURE);
    if (!hRequest) { WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession); return QString::fromUtf8(_S("创建请求失败")); }

    // Add HMAC headers for authentication (sign timestamp+nonce+path for GET requests)
    char tsBuf[32]; _i64toa(GetUnixTimestamp(), tsBuf, 10);
    char nonceHex[33]; GenerateNonce(nonceHex);
    BYTE sig[32]; DWORD sigLen = 0;
    char pathUtf8[256];
    WideCharToMultiByte(CP_UTF8, 0, path, -1, pathUtf8, sizeof(pathUtf8), NULL, NULL);
    HmacSha256Signed((const char*)HMAC_KEY, 32,
                     tsBuf, nonceHex, pathUtf8, (DWORD)strlen(pathUtf8), sig, &sigLen);
    char sigHex[65]; ByteToHex(sig, sigLen, sigHex);
    wchar_t authHeaders[512];
    swprintf_s(authHeaders, sizeof(authHeaders)/sizeof(wchar_t),
        L"X-Client-ID: %s\r\nX-HMAC-Signature: %S\r\nX-Timestamp: %S\r\nX-Nonce: %S\r\n",
        CLIENT_ID, sigHex, tsBuf, nonceHex);
    WinHttpAddRequestHeaders(hRequest, authHeaders, (DWORD)wcslen(authHeaders), WINHTTP_ADDREQ_FLAG_ADD);

    DWORD secFlags = SECURITY_FLAG_IGNORE_UNKNOWN_CA
                   | SECURITY_FLAG_IGNORE_CERT_CN_INVALID
                   | SECURITY_FLAG_IGNORE_CERT_DATE_INVALID;
    WinHttpSetOption(hRequest, WINHTTP_OPTION_SECURITY_FLAGS, &secFlags, sizeof(secFlags));
    WinHttpSetTimeouts(hRequest, 30000, 30000, 60000, 300000); // 5min total timeout for large files

    if (!WinHttpSendRequest(hRequest, NULL, 0, NULL, 0, 0, 0)) {
        DWORD e = GetLastError();
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        return (e == ERROR_WINHTTP_TIMEOUT) ? QString::fromUtf8(_S("发送请求超时"))
            : QString::fromUtf8(_S("发送请求失败 (错误: %1)")).arg(e);
    }
    if (!WinHttpReceiveResponse(hRequest, NULL)) {
        DWORD e = GetLastError();
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        return (e == ERROR_WINHTTP_TIMEOUT) ? QString::fromUtf8(_S("接收响应超时"))
            : QString::fromUtf8(_S("接收响应失败 (错误: %1)")).arg(e);
    }
    if (!VerifyServerCert(hRequest)) {
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        return QString::fromUtf8(_S("证书验证失败"));
    }

    DWORD statusCode = 0, statusCodeSize = sizeof(statusCode);
    WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
        NULL, &statusCode, &statusCodeSize, NULL);
    if (statusCode != 200) {
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        return QString::fromUtf8(_S("服务器返回错误 (HTTP %1)")).arg(statusCode);
    }

    // Get content length
    qint64 totalBytes = 0;
    DWORD contentLengthSize = sizeof(totalBytes);
    WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_CONTENT_LENGTH | WINHTTP_QUERY_FLAG_NUMBER,
        NULL, &totalBytes, &contentLengthSize, NULL);

    HANDLE hFile = CreateFileW(localPath, GENERIC_WRITE, 0, NULL,
        CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (hFile == INVALID_HANDLE_VALUE) {
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        return QString::fromUtf8(_S("无法创建本地文件"));
    }

    char buf[65536]; // 64KB buffer for faster download
    qint64 totalRead = 0;
    DWORD bytesRead = 0;
    bool writeError = false;
    while (WinHttpReadData(hRequest, buf, sizeof(buf), &bytesRead) && bytesRead > 0) {
        DWORD written = 0;
        if (!WriteFile(hFile, buf, bytesRead, &written, NULL) || written != bytesRead) {
            writeError = true;
            break;
        }
        totalRead += bytesRead;
        if (progressCb) progressCb(totalRead, totalBytes);
    }
    CloseHandle(hFile);
    WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);

    if (writeError) {
        DeleteFileW(localPath);
        return QString::fromUtf8(_S("写入文件失败"));
    }
    if (totalRead == 0) {
        DeleteFileW(localPath);
        return QString::fromUtf8(_S("下载失败：文件为空"));
    }
    return QString(); // success
}
