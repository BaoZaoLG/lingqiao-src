#pragma once
// One-shot secret handoff from injector to CefHook via a DPAPI-protected temp file.
// Env carries only the file path (INJECTOR_SECRET_FILE), never the secrets.
//
// File format (UTF-8 lines):
//   LQSEC1
//   base64(apiKey)
//   base64(baseUrl)
//   base64(model)
//   base64(sessionToken)
//   base64(machineId)
//   base64(cardCode)
//
// Usable from both Qt (injector) and native C++ (hook). No Qt dependency.

#include <windows.h>
#include <wincrypt.h>
#include <bcrypt.h>
#include <string>
#include <vector>

#pragma comment(lib, "crypt32.lib")
#pragma comment(lib, "bcrypt.lib")

#ifndef LQ_SECRET_FILE_ENV
#define LQ_SECRET_FILE_ENV L"INJECTOR_SECRET_FILE"
#endif

struct LqSecretBundle {
    std::string apiKey;
    std::string baseUrl;
    std::string model;
    std::string sessionToken;
    std::string machineId;
    std::string cardCode;

    void clear() {
        SecureZeroMemory(apiKey.empty() ? (void*)"" : (void*)apiKey.data(), apiKey.size());
        SecureZeroMemory(baseUrl.empty() ? (void*)"" : (void*)baseUrl.data(), baseUrl.size());
        SecureZeroMemory(model.empty() ? (void*)"" : (void*)model.data(), model.size());
        SecureZeroMemory(sessionToken.empty() ? (void*)"" : (void*)sessionToken.data(), sessionToken.size());
        SecureZeroMemory(machineId.empty() ? (void*)"" : (void*)machineId.data(), machineId.size());
        SecureZeroMemory(cardCode.empty() ? (void*)"" : (void*)cardCode.data(), cardCode.size());
        apiKey.clear();
        baseUrl.clear();
        model.clear();
        sessionToken.clear();
        machineId.clear();
        cardCode.clear();
    }
};

namespace lq_secret_detail {

inline std::string B64Encode(const std::string& in) {
    DWORD need = 0;
    if (!CryptBinaryToStringA((const BYTE*)in.data(), (DWORD)in.size(),
                              CRYPT_STRING_BASE64 | CRYPT_STRING_NOCRLF, NULL, &need)) {
        return {};
    }
    std::string out(need, '\0');
    if (!CryptBinaryToStringA((const BYTE*)in.data(), (DWORD)in.size(),
                              CRYPT_STRING_BASE64 | CRYPT_STRING_NOCRLF, &out[0], &need)) {
        return {};
    }
    out.resize(need);
    return out;
}

inline bool B64Decode(const std::string& in, std::string* out) {
    DWORD need = 0;
    if (!CryptStringToBinaryA(in.c_str(), (DWORD)in.size(), CRYPT_STRING_BASE64,
                              NULL, &need, NULL, NULL)) {
        return false;
    }
    std::vector<BYTE> buf(need ? need : 1);
    if (!CryptStringToBinaryA(in.c_str(), (DWORD)in.size(), CRYPT_STRING_BASE64,
                              buf.data(), &need, NULL, NULL)) {
        return false;
    }
    out->assign((const char*)buf.data(), need);
    return true;
}

inline bool DpapiProtect(const std::string& plain, std::vector<BYTE>* cipher) {
    DATA_BLOB in, out;
    in.pbData = (BYTE*)plain.data();
    in.cbData = (DWORD)plain.size();
    if (!CryptProtectData(&in, L"LingQiaoSecret", nullptr, nullptr, nullptr,
                          CRYPTPROTECT_UI_FORBIDDEN, &out)) {
        return false;
    }
    cipher->assign(out.pbData, out.pbData + out.cbData);
    LocalFree(out.pbData);
    return true;
}

inline bool DpapiUnprotect(const std::vector<BYTE>& cipher, std::string* plain) {
    DATA_BLOB in, out;
    in.pbData = (BYTE*)cipher.data();
    in.cbData = (DWORD)cipher.size();
    if (!CryptUnprotectData(&in, nullptr, nullptr, nullptr, nullptr,
                            CRYPTPROTECT_UI_FORBIDDEN, &out)) {
        return false;
    }
    plain->assign((const char*)out.pbData, out.cbData);
    LocalFree(out.pbData);
    return true;
}

inline std::string SerializeBundle(const LqSecretBundle& b) {
    std::string s = "LQSEC1\n";
    s += B64Encode(b.apiKey) + "\n";
    s += B64Encode(b.baseUrl) + "\n";
    s += B64Encode(b.model) + "\n";
    s += B64Encode(b.sessionToken) + "\n";
    s += B64Encode(b.machineId) + "\n";
    s += B64Encode(b.cardCode) + "\n";
    return s;
}

inline bool ParseBundle(const std::string& raw, LqSecretBundle* out) {
    std::vector<std::string> lines;
    size_t pos = 0;
    while (pos <= raw.size()) {
        size_t nl = raw.find('\n', pos);
        if (nl == std::string::npos) {
            if (pos < raw.size()) lines.push_back(raw.substr(pos));
            break;
        }
        lines.push_back(raw.substr(pos, nl - pos));
        pos = nl + 1;
    }
    if (lines.empty() || lines[0] != "LQSEC1") return false;
    if (lines.size() < 7) return false;
    return B64Decode(lines[1], &out->apiKey)
        && B64Decode(lines[2], &out->baseUrl)
        && B64Decode(lines[3], &out->model)
        && B64Decode(lines[4], &out->sessionToken)
        && B64Decode(lines[5], &out->machineId)
        && B64Decode(lines[6], &out->cardCode);
}

} // namespace lq_secret_detail

// Write bundle to a random temp file; returns full path or empty on failure.
inline std::wstring WriteSecretFile(const LqSecretBundle& bundle) {
    using namespace lq_secret_detail;

    std::string plain = SerializeBundle(bundle);
    std::vector<BYTE> cipher;
    if (!DpapiProtect(plain, &cipher)) {
        SecureZeroMemory(&plain[0], plain.size());
        return {};
    }
    SecureZeroMemory(&plain[0], plain.size());

    WCHAR tempDir[MAX_PATH] = {0};
    if (!GetTempPathW(MAX_PATH, tempDir)) return {};

    BYTE rnd[8] = {0};
    if (!BCryptGenRandom(nullptr, rnd, sizeof(rnd), BCRYPT_USE_SYSTEM_PREFERRED_RNG)) {
        // Fallback if bcrypt RNG unavailable
        for (int i = 0; i < 8; i++) rnd[i] = (BYTE)(GetTickCount() >> (i * 3));
    }
    WCHAR name[64] = {0};
    swprintf_s(name, L"LQ_%02X%02X%02X%02X%02X%02X%02X%02X.sec",
               rnd[0], rnd[1], rnd[2], rnd[3], rnd[4], rnd[5], rnd[6], rnd[7]);

    std::wstring path = std::wstring(tempDir) + name;
    // Not DELETE_ON_CLOSE: CEF has multiple processes (browser/renderer).
    // Each process opens independently; injector deletes after injection.
    HANDLE h = CreateFileW(path.c_str(), GENERIC_WRITE, 0, nullptr, CREATE_NEW,
                           FILE_ATTRIBUTE_TEMPORARY, nullptr);
    if (h == INVALID_HANDLE_VALUE) return {};
    DWORD written = 0;
    BOOL ok = WriteFile(h, cipher.data(), (DWORD)cipher.size(), &written, nullptr);
    FlushFileBuffers(h);
    CloseHandle(h);
    if (!ok || written != cipher.size()) {
        DeleteFileW(path.c_str());
        return {};
    }
    return path;
}

// Read and consume the secret file named by INJECTOR_SECRET_FILE (or pathOverride).
inline bool ReadSecretFile(LqSecretBundle* out, const wchar_t* pathOverride = nullptr) {
    using namespace lq_secret_detail;

    WCHAR path[MAX_PATH] = {0};
    if (pathOverride && pathOverride[0]) {
        wcsncpy_s(path, pathOverride, _TRUNCATE);
    } else {
        DWORD n = GetEnvironmentVariableW(LQ_SECRET_FILE_ENV, path, MAX_PATH);
        if (n == 0 || n >= MAX_PATH) return false;
    }

    HANDLE h = CreateFileW(path, GENERIC_READ, FILE_SHARE_READ, nullptr,
                           OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr);
    if (h == INVALID_HANDLE_VALUE) return false;
    LARGE_INTEGER sz{};
    if (!GetFileSizeEx(h, &sz) || sz.QuadPart <= 0 || sz.QuadPart > 1 << 20) {
        CloseHandle(h);
        return false;
    }
    std::vector<BYTE> cipher((size_t)sz.QuadPart);
    DWORD rd = 0;
    BOOL ok = ReadFile(h, cipher.data(), (DWORD)cipher.size(), &rd, nullptr);
    CloseHandle(h);
    if (!ok || rd != cipher.size()) return false;

    std::string plain;
    if (!DpapiUnprotect(cipher, &plain)) return false;
    bool parsed = ParseBundle(plain, out);
    SecureZeroMemory(&plain[0], plain.size());
    // Do not delete here: CEF browser + renderer each load secrets.
    // Injector deletes the file after CreateProcess/inject completes.
    return parsed;
}

// Process-wide cached load (thread-safe enough via InitOnce-style Interlocked).
//
// A failed load must NOT be permanent. The injector deletes the one-shot secret
// file once injection completes, so a process that happens to call this before
// the file lands loses the race — and with a sticky failure the AI path
// (ai.chatComplete), the script fetch and the exam upload would all stay dead
// for the whole process lifetime with no way to recover.
inline const LqSecretBundle& LqSecretsCached() {
    static LqSecretBundle cache;
    static volatile LONG state = 0;        // 0=idle 1=loading 2=ready 3=failed
    static volatile LONG lastAttempt = 0;  // GetTickCount of the last read attempt
    static volatile LONG attempts = 0;     // total read attempts made

    const DWORD kRetryIntervalMs = 2000;
    const LONG  kMaxAttempts = 30;         // ~60s of retrying, then stop

    LONG s = InterlockedCompareExchange(&state, 1, 0);
    if (s == 0) {
        InterlockedExchange(&lastAttempt, (LONG)GetTickCount());
        InterlockedIncrement(&attempts);
        LqSecretBundle tmp;
        if (ReadSecretFile(&tmp)) {
            cache = tmp;
            tmp.clear();
            InterlockedExchange(&state, 2);
        } else {
            InterlockedExchange(&state, 3);
        }
    } else if (s == 1) {
        // Another thread is loading. Wait, but never forever.
        for (int i = 0; i < 5000; ++i) {
            LONG cur = InterlockedCompareExchange(&state, 0, 0);
            if (cur == 2 || cur == 3) break;
            Sleep(1);
        }
    } else if (s == 3) {
        DWORD now = GetTickCount();
        DWORD prev = (DWORD)InterlockedCompareExchange(&lastAttempt, 0, 0);
        if ((now - prev) >= kRetryIntervalMs &&
            InterlockedCompareExchange(&attempts, 0, 0) < kMaxAttempts) {
            InterlockedExchange(&lastAttempt, (LONG)now);
            LONG n = InterlockedIncrement(&attempts);
            InterlockedExchange(&state, 1);
            LqSecretBundle tmp;
            if (ReadSecretFile(&tmp)) {
                cache = tmp;
                tmp.clear();
                InterlockedExchange(&state, 2);
                OutputDebugStringA("[SECRET] secret bundle loaded on retry\n");
            } else {
                InterlockedExchange(&state, 3);
                if (n >= kMaxAttempts) {
                    OutputDebugStringA("[SECRET] giving up on secret bundle after max attempts "
                                       "— AI/script/exam features will stay unavailable\n");
                }
            }
        }
    }
    return cache;
}

inline void LqSecretsClearEnvAndLegacy() {
    SetEnvironmentVariableW(L"INJECTOR_AI_API_KEY", NULL);
    SetEnvironmentVariableW(L"INJECTOR_AI_MODEL", NULL);
    SetEnvironmentVariableW(L"INJECTOR_AI_BASE_URL", NULL);
    SetEnvironmentVariableW(L"INJECTOR_SESSION_TOKEN", NULL);
    SetEnvironmentVariableW(L"INJECTOR_MACHINE_ID", NULL);
    SetEnvironmentVariableW(L"INJECTOR_CARD_CODE", NULL);
}
