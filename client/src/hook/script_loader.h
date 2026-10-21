#pragma once

#include <windows.h>
#include <winhttp.h>
#include <wincrypt.h>
#include <bcrypt.h>

#include <string>

#include "obfuscate.h"
#include "encrypted_data.h"
#include "secret_channel.h"

#ifndef NT_SUCCESS
#define NT_SUCCESS(status) ((status) >= 0)
#endif

// Certificate pin: use the single source of truth from crypto.h
#include "crypto.h"
static const char* HOOK_CERT_PIN_SHA256 = LQ_SP(LQ_CERT_PIN_SHA256_LITERAL);

// Thin aliases to core/crypto.h — do not reimplement HMAC/hex here.
static void HookByteToHex(const BYTE* input, DWORD inputLen, char* output) {
    ByteToHex(input, inputLen, output);
}

static bool HookSha256Hex(const char* data, DWORD dataLen, char* outputHex) {
    BCRYPT_ALG_HANDLE hAlg = NULL;
    BCRYPT_HASH_HANDLE hHash = NULL;
    bool ok = false;
    if (BCryptOpenAlgorithmProvider(&hAlg, BCRYPT_SHA256_ALGORITHM, NULL, 0) == 0) {
        if (BCryptCreateHash(hAlg, &hHash, NULL, 0, NULL, 0, 0) == 0) {
            BYTE hash[32];
            if (BCryptHashData(hHash, (PUCHAR)data, dataLen, 0) == 0 &&
                BCryptFinishHash(hHash, hash, sizeof(hash), 0) == 0) {
                ByteToHex(hash, sizeof(hash), outputHex);
                ok = true;
            }
            BCryptDestroyHash(hHash);
        }
        BCryptCloseAlgorithmProvider(hAlg, 0);
    }
    return ok;
}

static bool HookDeriveKey(const char* secret, BYTE* key, DWORD keyLen) {
    const char* salt = "CefBridge-HMAC-Salt-v2";
    return DeriveKey(secret, (DWORD)strlen(secret),
                     salt, (DWORD)strlen(salt), key, keyLen);
}

static bool HookHmacSha256(const BYTE* key, DWORD keyLen, const char* data, DWORD dataLen, BYTE* output, DWORD* outputLen) {
    return HmacSha256((const char*)key, keyLen, data, dataLen, output, outputLen);
}

static __int64 HookUnixTimestamp() {
    return GetUnixTimestamp();
}

static void HookGenerateNonce(char* output) {
    if (!GenerateNonce(output)) {
        memset(output, 0, 33);
    }
}

static bool HookBuildSignature(const BYTE* key, const wchar_t* path, char* sigHex, char* tsBuf, char* nonceHex) {
    _i64toa_s(GetUnixTimestamp(), tsBuf, 32, 10);
    if (!GenerateNonce(nonceHex)) return false;

    int pathLen = WideCharToMultiByte(CP_UTF8, 0, path, -1, NULL, 0, NULL, NULL);
    if (pathLen <= 1) return false;
    std::string pathUtf8(pathLen, '\0');
    WideCharToMultiByte(CP_UTF8, 0, path, -1, &pathUtf8[0], pathLen, NULL, NULL);
    pathUtf8.resize(pathLen - 1);

    BYTE sig[32];
    DWORD sigLen = 0;
    if (!HmacSha256Signed((const char*)key, 32, tsBuf, nonceHex,
                          pathUtf8.data(), (DWORD)pathUtf8.size(), sig, &sigLen)) {
        return false;
    }
    ByteToHex(sig, sigLen, sigHex);
    return true;
}

static bool HookVerifyServerCert(HINTERNET hRequest) {
    CERT_CONTEXT* pCert = nullptr;
    DWORD certSize = sizeof(pCert);
    if (!WinHttpQueryOption(hRequest, WINHTTP_OPTION_SERVER_CERT_CONTEXT, &pCert, &certSize) || !pCert)
        return false;

    BCRYPT_ALG_HANDLE hAlg = nullptr;
    BCRYPT_HASH_HANDLE hHash = nullptr;
    bool ok = false;
    if (BCryptOpenAlgorithmProvider(&hAlg, BCRYPT_SHA256_ALGORITHM, nullptr, 0) == 0) {
        if (BCryptCreateHash(hAlg, &hHash, nullptr, 0, nullptr, 0, 0) == 0) {
            BYTE hash[32];
            BCryptHashData(hHash, (PUCHAR)pCert->pbCertEncoded, pCert->cbCertEncoded, 0);
            BCryptFinishHash(hHash, hash, 32, 0);
            char hex[65];
            HookByteToHex(hash, 32, hex);
            ok = CertPinMatchesList(hex, HOOK_CERT_PIN_SHA256);
            BCryptDestroyHash(hHash);
        }
        BCryptCloseAlgorithmProvider(hAlg, 0);
    }
    CertFreeCertificateContext(pCert);
    return ok;
}

static bool HookJsonExtractString(const std::string& json, const char* key, std::string* out) {
    std::string marker = std::string("\"") + key + "\"";
    size_t p = json.find(marker);
    if (p == std::string::npos) return false;
    p = json.find(':', p + marker.size());
    if (p == std::string::npos) return false;
    p = json.find('"', p + 1);
    if (p == std::string::npos) return false;
    p++;

    std::string value;
    for (; p < json.size(); p++) {
        char c = json[p];
        if (c == '"') {
            *out = value;
            return true;
        }
        if (c == '\\' && p + 1 < json.size()) {
            char e = json[++p];
            switch (e) {
            case '"': value.push_back('"'); break;
            case '\\': value.push_back('\\'); break;
            case '/': value.push_back('/'); break;
            case 'b': value.push_back('\b'); break;
            case 'f': value.push_back('\f'); break;
            case 'n': value.push_back('\n'); break;
            case 'r': value.push_back('\r'); break;
            case 't': value.push_back('\t'); break;
            case 'u':
                if (p + 4 < json.size()) {
                    unsigned int cp = 0;
                    for (int i = 0; i < 4; i++) {
                        char h = json[p + 1 + i];
                        cp <<= 4;
                        if (h >= '0' && h <= '9') cp |= (unsigned int)(h - '0');
                        else if (h >= 'a' && h <= 'f') cp |= (unsigned int)(h - 'a' + 10);
                        else if (h >= 'A' && h <= 'F') cp |= (unsigned int)(h - 'A' + 10);
                    }
                    p += 4;
                    if (cp < 0x80) {
                        value.push_back((char)cp);
                    } else if (cp < 0x800) {
                        value.push_back((char)(0xC0 | (cp >> 6)));
                        value.push_back((char)(0x80 | (cp & 0x3F)));
                    } else {
                        value.push_back((char)(0xE0 | (cp >> 12)));
                        value.push_back((char)(0x80 | ((cp >> 6) & 0x3F)));
                        value.push_back((char)(0x80 | (cp & 0x3F)));
                    }
                }
                break;
            default: value.push_back(e); break;
            }
        } else {
            value.push_back(c);
        }
    }
    return false;
}

static bool HookQueryHeaderString(HINTERNET hRequest, const wchar_t* headerName, std::string* out) {
    DWORD size = 0;
    WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_CUSTOM, headerName, NULL, &size, WINHTTP_NO_HEADER_INDEX);
    if (GetLastError() != ERROR_INSUFFICIENT_BUFFER || size == 0) {
        return false;
    }

    std::wstring value;
    value.resize(size / sizeof(wchar_t));
    if (!WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_CUSTOM, headerName, &value[0], &size, WINHTTP_NO_HEADER_INDEX)) {
        return false;
    }
    if (!value.empty() && value.back() == L'\0') {
        value.pop_back();
    }

    int len = WideCharToMultiByte(CP_UTF8, 0, value.c_str(), -1, NULL, 0, NULL, NULL);
    if (len <= 1) {
        return false;
    }
    std::string utf8(len, '\0');
    WideCharToMultiByte(CP_UTF8, 0, value.c_str(), -1, &utf8[0], len, NULL, NULL);
    utf8.resize(len - 1);
    *out = utf8;
    return true;
}

static std::wstring LqUtf8ToWide(const std::string& s) {
    if (s.empty()) return std::wstring();
    int n = MultiByteToWideChar(CP_UTF8, 0, s.c_str(), (int)s.size(), nullptr, 0);
    if (n <= 0) return std::wstring();
    std::wstring w((size_t)n, L'\0');
    MultiByteToWideChar(CP_UTF8, 0, s.c_str(), (int)s.size(), &w[0], n);
    return w;
}

static bool FetchVersionedScript(std::string* scriptOut, std::string* versionOut) {
    OutputDebugStringA("[HOOK] FetchVersionedScript: entry\n");
    const LqSecretBundle& secrets = LqSecretsCached();
    if (secrets.sessionToken.empty()) {
        OutputDebugStringA("[HOOK] Script fetch skipped: missing session token\n");
        return false;
    }
    std::wstring sessionToken = LqUtf8ToWide(secrets.sessionToken);
    std::wstring machineId = LqUtf8ToWide(secrets.machineId);
    std::wstring cardCode = LqUtf8ToWide(secrets.cardCode);

    SecBuf<WCHAR> host(g_enc_kHost);
    SecBuf<WCHAR> clientId(g_enc_kClientId);
    SecBuf<char> secret(g_enc_kSecret);
    BYTE key[32];
    if (!HookDeriveKey(secret.c_str(), key, sizeof(key))) {
        OutputDebugStringA("[HOOK] Script fetch skipped: key derivation failed\n");
        return false;
    }
    {
        char hostUtf8[128] = {0};
        WideCharToMultiByte(CP_UTF8, 0, host.c_str(), -1, hostUtf8, sizeof(hostUtf8), NULL, NULL);
        char dbg[256];
        sprintf_s(dbg, "[HOOK] Script fetch: host=%s token_len=%zu mid=%d cc=%d\n",
                  hostUtf8, sessionToken.size(), (int)machineId.size(), (int)cardCode.size());
        OutputDebugStringA(dbg);
    }

    const wchar_t* path = L"/api/v1/script";
    char sigHex[65] = {0};
    char tsBuf[32] = {0};
    char nonceHex[33] = {0};
    if (!HookBuildSignature(key, path, sigHex, tsBuf, nonceHex)) {
        OutputDebugStringA("[HOOK] Script fetch skipped: signature failed\n");
        return false;
    }

    HINTERNET hSession = WinHttpOpen(L"HOOK/1.0", WINHTTP_ACCESS_TYPE_DEFAULT_PROXY, NULL, NULL, 0);
    if (!hSession) return false;
    HINTERNET hConnect = WinHttpConnect(hSession, host.c_str(), 443, 0);
    if (!hConnect) {
        WinHttpCloseHandle(hSession);
        return false;
    }
    HINTERNET hRequest = WinHttpOpenRequest(hConnect, L"GET", path, NULL, NULL, NULL, WINHTTP_FLAG_SECURE);
    if (!hRequest) {
        WinHttpCloseHandle(hConnect);
        WinHttpCloseHandle(hSession);
        return false;
    }

    wchar_t headers[2048];
    int off = swprintf_s(headers, _countof(headers),
        L"X-Client-ID: %s\r\nX-HMAC-Signature: %S\r\nX-Timestamp: %S\r\nX-Nonce: %S\r\nX-Session-Token: %s\r\n",
        clientId.c_str(), sigHex, tsBuf, nonceHex, sessionToken.c_str());
    if (!machineId.empty()) {
        off += swprintf_s(headers + off, _countof(headers) - off,
            L"X-Machine-ID: %s\r\n", machineId.c_str());
    }
    if (!cardCode.empty()) {
        swprintf_s(headers + off, _countof(headers) - off,
            L"X-Card-Code: %s\r\n", cardCode.c_str());
    }
    WinHttpAddRequestHeaders(hRequest, headers, (DWORD)-1, WINHTTP_ADDREQ_FLAG_ADD);

    DWORD secFlags = SECURITY_FLAG_IGNORE_UNKNOWN_CA;
    WinHttpSetOption(hRequest, WINHTTP_OPTION_SECURITY_FLAGS, &secFlags, sizeof(secFlags));
    WinHttpSetTimeouts(hRequest, 5000, 5000, 5000, 10000);

    bool ok = false;
    std::string body;
    bool sendOk = WinHttpSendRequest(hRequest, NULL, 0, NULL, 0, 0, 0);
    if (!sendOk) {
        char dbg[128];
        sprintf_s(dbg, "[HOOK] Script fetch: WinHttpSendRequest failed err=%lu\n", GetLastError());
        OutputDebugStringA(dbg);
    }
    bool recvOk = sendOk && WinHttpReceiveResponse(hRequest, NULL);
    if (!recvOk && sendOk) {
        char dbg[128];
        sprintf_s(dbg, "[HOOK] Script fetch: WinHttpReceiveResponse failed err=%lu\n", GetLastError());
        OutputDebugStringA(dbg);
    }
    bool certOk = recvOk && HookVerifyServerCert(hRequest);
    if (!certOk && recvOk) {
        OutputDebugStringA("[HOOK] Script fetch: certificate verification failed\n");
    }
    if (sendOk && recvOk && certOk) {
        DWORD status = 0, statusSize = sizeof(status);
        WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER, NULL, &status, &statusSize, NULL);
        char dbg[128];
        sprintf_s(dbg, "[HOOK] Script fetch: HTTP %lu\n", (unsigned long)status);
        OutputDebugStringA(dbg);
        if (status == 200) {
            char buf[4096];
            DWORD bytesRead = 0;
            while (WinHttpReadData(hRequest, buf, sizeof(buf), &bytesRead) && bytesRead > 0) {
                body.append(buf, bytesRead);
                if (body.size() > (2 << 20)) {
                    body.clear();
                    break;
                }
            }
            if (!body.empty()) {
                char dbg[128];
                sprintf_s(dbg, "[HOOK] Script fetch: body size=%zu\n", body.size());
                OutputDebugStringA(dbg);
                std::string sha;
                std::string version;
                char actualSha[65] = {0};
                if (HookQueryHeaderString(hRequest, L"X-Script-SHA256", &sha)) {
                    HookSha256Hex(body.data(), (DWORD)body.size(), actualSha);
                    if (_stricmp(actualSha, sha.c_str()) == 0) {
                        HookQueryHeaderString(hRequest, L"X-Script-Version", &version);
                        *scriptOut = body;
                        *versionOut = version;
                        ok = true;
                    } else {
                        char dbg2[256];
                        sprintf_s(dbg2, "[HOOK] Script fetch: SHA256 mismatch expected=%.64s got=%.64s\n",
                                  sha.c_str(), actualSha);
                        OutputDebugStringA(dbg2);
                    }
                } else {
                    std::string content;
                    if (HookJsonExtractString(body, "content", &content) &&
                        HookJsonExtractString(body, "sha256", &sha) &&
                        HookSha256Hex(content.data(), (DWORD)content.size(), actualSha) &&
                        _stricmp(actualSha, sha.c_str()) == 0) {
                        HookJsonExtractString(body, "version", &version);
                        *scriptOut = content;
                        *versionOut = version;
                        ok = true;
                    } else {
                        OutputDebugStringA("[HOOK] Script fetch: SHA256 verification failed (no header + JSON fallback failed)\n");
                    }
                }
            } else {
                OutputDebugStringA("[HOOK] Script fetch: empty response body\n");
            }
        } else {
            char buf[512];
            DWORD bytesRead = 0;
            while (WinHttpReadData(hRequest, buf, sizeof(buf) - 1, &bytesRead) && bytesRead > 0) {
                buf[bytesRead] = 0;
                char dbg[640];
                sprintf_s(dbg, "[HOOK] Script fetch: non-200 body=%.400s\n", buf);
                OutputDebugStringA(dbg);
            }
        }
    }

    WinHttpCloseHandle(hRequest);
    WinHttpCloseHandle(hConnect);
    WinHttpCloseHandle(hSession);
    SecureZeroMemory(key, sizeof(key));

    if (ok) {
        char dbg[160];
        sprintf_s(dbg, "[HOOK] Script fetched version=%s size=%zu\n", versionOut->c_str(), scriptOut->size());
        OutputDebugStringA(dbg);
    } else {
        OutputDebugStringA("[HOOK] Script fetch failed\n");
    }
    return ok;
}
