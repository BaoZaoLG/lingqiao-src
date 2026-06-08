#pragma once

#include <windows.h>
#include <winhttp.h>
#include <wincrypt.h>
#include <bcrypt.h>

#include <string>

#include "obfuscate.h"
#include "encrypted_data.h"

#ifndef NT_SUCCESS
#define NT_SUCCESS(status) ((status) >= 0)
#endif

static const char* HOOK_CERT_PIN_SHA256 = "9f3435586bdb2528c1b1b460748782a96e2670ff4690b388796c85241b306458";

static void HookByteToHex(const BYTE* input, DWORD inputLen, char* output) {
    static const char hex[] = "0123456789abcdef";
    for (DWORD i = 0; i < inputLen; i++) {
        output[i * 2] = hex[(input[i] >> 4) & 0xF];
        output[i * 2 + 1] = hex[input[i] & 0xF];
    }
    output[inputLen * 2] = 0;
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
                HookByteToHex(hash, sizeof(hash), outputHex);
                ok = true;
            }
            BCryptDestroyHash(hHash);
        }
        BCryptCloseAlgorithmProvider(hAlg, 0);
    }
    return ok;
}

static bool HookDeriveKey(const char* secret, BYTE* key, DWORD keyLen) {
    BCRYPT_ALG_HANDLE hAlg = NULL;
    NTSTATUS status = BCryptOpenAlgorithmProvider(&hAlg, BCRYPT_SHA256_ALGORITHM, NULL, BCRYPT_ALG_HANDLE_HMAC_FLAG);
    if (!NT_SUCCESS(status)) return false;
    const char* salt = "CefBridge-HMAC-Salt-v2";
    status = BCryptDeriveKeyPBKDF2(hAlg,
        (PUCHAR)secret, (ULONG)strlen(secret),
        (PUCHAR)salt, (ULONG)strlen(salt),
        100000,
        key, keyLen,
        0);
    BCryptCloseAlgorithmProvider(hAlg, 0);
    return NT_SUCCESS(status);
}

static bool HookHmacSha256(const BYTE* key, DWORD keyLen, const char* data, DWORD dataLen, BYTE* output, DWORD* outputLen) {
    BCRYPT_ALG_HANDLE hAlg = NULL;
    BCRYPT_HASH_HANDLE hHash = NULL;
    NTSTATUS status = BCryptOpenAlgorithmProvider(&hAlg, BCRYPT_SHA256_ALGORITHM, NULL, BCRYPT_ALG_HANDLE_HMAC_FLAG);
    if (!NT_SUCCESS(status)) return false;

    DWORD hashObjSize = 0, resultLen = 0, hashLen = 0;
    BCryptGetProperty(hAlg, BCRYPT_OBJECT_LENGTH, (PBYTE)&hashObjSize, sizeof(hashObjSize), &resultLen, 0);
    BCryptGetProperty(hAlg, BCRYPT_HASH_LENGTH, (PBYTE)&hashLen, sizeof(hashLen), &resultLen, 0);
    BYTE* hashObj = (BYTE*)HeapAlloc(GetProcessHeap(), 0, hashObjSize);
    if (!hashObj) {
        BCryptCloseAlgorithmProvider(hAlg, 0);
        return false;
    }

    status = BCryptCreateHash(hAlg, &hHash, hashObj, hashObjSize, (PBYTE)key, keyLen, 0);
    if (NT_SUCCESS(status)) {
        status = BCryptHashData(hHash, (PBYTE)data, dataLen, 0);
    }
    if (NT_SUCCESS(status)) {
        status = BCryptFinishHash(hHash, output, hashLen, 0);
        *outputLen = hashLen;
    }
    if (hHash) BCryptDestroyHash(hHash);
    HeapFree(GetProcessHeap(), 0, hashObj);
    BCryptCloseAlgorithmProvider(hAlg, 0);
    return NT_SUCCESS(status);
}

static __int64 HookUnixTimestamp() {
    FILETIME ft;
    GetSystemTimeAsFileTime(&ft);
    __int64 t = ((__int64)ft.dwHighDateTime << 32) | ft.dwLowDateTime;
    return (t - 116444736000000000LL) / 10000000LL;
}

static void HookGenerateNonce(char* output) {
    BYTE buf[16];
    if (BCryptGenRandom(NULL, buf, sizeof(buf), BCRYPT_USE_SYSTEM_PREFERRED_RNG) != 0) {
        memset(buf, 0, sizeof(buf));
    }
    HookByteToHex(buf, sizeof(buf), output);
}

static bool HookBuildSignature(const BYTE* key, const wchar_t* path, char* sigHex, char* tsBuf, char* nonceHex) {
    _i64toa_s(HookUnixTimestamp(), tsBuf, 32, 10);
    HookGenerateNonce(nonceHex);

    int pathLen = WideCharToMultiByte(CP_UTF8, 0, path, -1, NULL, 0, NULL, NULL);
    if (pathLen <= 1) return false;
    std::string pathUtf8(pathLen, '\0');
    WideCharToMultiByte(CP_UTF8, 0, path, -1, &pathUtf8[0], pathLen, NULL, NULL);
    pathUtf8.resize(pathLen - 1);

    std::string signedBody = std::string(tsBuf) + "|" + nonceHex + "|" + pathUtf8;
    BYTE sig[32];
    DWORD sigLen = 0;
    if (!HookHmacSha256(key, 32, signedBody.data(), (DWORD)signedBody.size(), sig, &sigLen)) return false;
    HookByteToHex(sig, sigLen, sigHex);
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
            ok = (_stricmp(hex, HOOK_CERT_PIN_SHA256) == 0);
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

static bool FetchVersionedScript(std::string* scriptOut, std::string* versionOut) {
    WCHAR sessionToken[256] = {0};
    GetEnvironmentVariableW(L"INJECTOR_SESSION_TOKEN", sessionToken, _countof(sessionToken));
    if (!sessionToken[0]) {
        OutputDebugStringA("[HOOK] Script fetch skipped: missing session token\n");
        return false;
    }

    SecBuf<WCHAR> host(g_enc_kHost);
    SecBuf<WCHAR> clientId(g_enc_kClientId);
    SecBuf<char> secret(g_enc_kSecret);
    BYTE key[32];
    if (!HookDeriveKey(secret.c_str(), key, sizeof(key))) {
        OutputDebugStringA("[HOOK] Script fetch skipped: key derivation failed\n");
        return false;
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

    wchar_t headers[1024];
    swprintf_s(headers, _countof(headers),
        L"X-Client-ID: %s\r\nX-HMAC-Signature: %S\r\nX-Timestamp: %S\r\nX-Nonce: %S\r\nX-Session-Token: %s\r\n",
        clientId.c_str(), sigHex, tsBuf, nonceHex, sessionToken);
    WinHttpAddRequestHeaders(hRequest, headers, (DWORD)wcslen(headers), WINHTTP_ADDREQ_FLAG_ADD);

    DWORD secFlags = SECURITY_FLAG_IGNORE_UNKNOWN_CA | SECURITY_FLAG_IGNORE_CERT_CN_INVALID | SECURITY_FLAG_IGNORE_CERT_DATE_INVALID;
    WinHttpSetOption(hRequest, WINHTTP_OPTION_SECURITY_FLAGS, &secFlags, sizeof(secFlags));
    WinHttpSetTimeouts(hRequest, 5000, 5000, 5000, 10000);

    bool ok = false;
    std::string body;
    if (WinHttpSendRequest(hRequest, NULL, 0, NULL, 0, 0, 0) &&
        WinHttpReceiveResponse(hRequest, NULL) &&
        HookVerifyServerCert(hRequest)) {
        DWORD status = 0, statusSize = sizeof(status);
        WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER, NULL, &status, &statusSize, NULL);
        if (status == 200) {
            char buf[4096];
            DWORD bytesRead = 0;
            while (WinHttpReadData(hRequest, buf, sizeof(buf), &bytesRead) && bytesRead > 0) {
                body.append(buf, bytesRead);
                if (body.size() > (2 << 20)) break;
            }
            std::string content;
            std::string sha;
            std::string version;
            char actualSha[65] = {0};
            if (HookJsonExtractString(body, "content", &content) &&
                HookJsonExtractString(body, "sha256", &sha) &&
                HookSha256Hex(content.data(), (DWORD)content.size(), actualSha) &&
                _stricmp(actualSha, sha.c_str()) == 0) {
                HookJsonExtractString(body, "version", &version);
                *scriptOut = content;
                *versionOut = version;
                ok = true;
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
        OutputDebugStringA("[HOOK] Script fetch failed; using embedded fallback\n");
    }
    return ok;
}
