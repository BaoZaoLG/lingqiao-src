#pragma once
// ============================================================================
// Configuration — XOR-encrypted secrets decrypted at startup
// ============================================================================
#include <windows.h>
#include "obfuscate.h"
#include "crypto.h"  // DeriveKey for PBKDF2 key derivation
#include "strcrypt.h"

#if __has_include("encrypted_data.h")
#include "encrypted_data.h"
#else
static const EncryptedBlob g_enc_kHost         = { nullptr, 0, 0 };
static const EncryptedBlob g_enc_kClientId     = { nullptr, 0, 0 };
static const EncryptedBlob g_enc_kSecret       = { nullptr, 0, 0 };
static const EncryptedBlob g_enc_kPathActivate = { nullptr, 0, 0 };
static const EncryptedBlob g_enc_kPathHeartbeat = { nullptr, 0, 0 };
static const EncryptedBlob g_enc_kPathDeact    = { nullptr, 0, 0 };
static const EncryptedBlob g_enc_kPathAnnounce = { nullptr, 0, 0 };
#endif

static wchar_t g_hostBuf[64];
static wchar_t g_clientIdBuf[32];
static char    g_secretBuf[128];
static wchar_t g_pathAct[64];
static wchar_t g_pathHb[64];
static wchar_t g_pathDeact[64];
static wchar_t g_pathAnn[64];
static BYTE    g_derivedKey[32]; // PBKDF2-derived HMAC signing key

static const wchar_t* SERVER_HOST   = nullptr;
static const int      SERVER_PORT   = 443;
static const wchar_t* CLIENT_ID     = nullptr;
static const char*    CLIENT_SECRET = nullptr;
static const BYTE*    HMAC_KEY      = nullptr; // points to g_derivedKey

// Version injected from CMake via APP_VERSION compile definition
#ifdef APP_VERSION
#define CLIENT_VERSION APP_VERSION
#else
#define CLIENT_VERSION "0.0.0"
#endif

#ifndef UPDATE_MANIFEST_PUBLIC_KEY_HEX
#define UPDATE_MANIFEST_PUBLIC_KEY_HEX ""
#endif

// Read version from PE resource (VS_FIXEDFILEINFO) at runtime
inline QString GetExeVersion() {
    WCHAR path[MAX_PATH] = {0};
    GetModuleFileNameW(NULL, path, MAX_PATH);
    DWORD handle = 0;
    DWORD size = GetFileVersionInfoSizeW(path, &handle);
    if (size == 0) return QStringLiteral(CLIENT_VERSION);
    QByteArray buf(size, 0);
    if (!GetFileVersionInfoW(path, handle, size, buf.data()))
        return QStringLiteral(CLIENT_VERSION);
    VS_FIXEDFILEINFO* fi = nullptr;
    UINT fiLen = 0;
    if (!VerQueryValueW(buf.data(), L"\\", (LPVOID*)&fi, &fiLen) || !fi)
        return QStringLiteral(CLIENT_VERSION);
    return QString::fromUtf8("%1.%2.%3")
        .arg(HIWORD(fi->dwProductVersionMS))
        .arg(LOWORD(fi->dwProductVersionMS))
        .arg(HIWORD(fi->dwProductVersionLS));
}

// Get reported client version: always from PE resource (authoritative)
inline QString GetClientVersion() {
    return GetExeVersion();
}

inline void InitSecrets() {
    SecBuf<WCHAR> host(g_enc_kHost);
    wcscpy_s(g_hostBuf, host.c_str());
    SERVER_HOST = g_hostBuf;

    SecBuf<WCHAR> cid(g_enc_kClientId);
    wcscpy_s(g_clientIdBuf, cid.c_str());
    CLIENT_ID = g_clientIdBuf;

    SecBuf<char> sec(g_enc_kSecret);
    strcpy_s(g_secretBuf, sec.c_str());
    CLIENT_SECRET = g_secretBuf;

    // Derive HMAC signing key via PBKDF2 so raw secret never used directly
    const char* salt = LQ_SP("CefBridge-HMAC-Salt-v2");
    DeriveKey(g_secretBuf, (DWORD)strlen(g_secretBuf),
              salt, (DWORD)strlen(salt),
              g_derivedKey, sizeof(g_derivedKey));
    HMAC_KEY = g_derivedKey;

    SecBuf<WCHAR> a(g_enc_kPathActivate);
    wcscpy_s(g_pathAct, a.c_str());
    SecBuf<WCHAR> h(g_enc_kPathHeartbeat);
    wcscpy_s(g_pathHb, h.c_str());
    SecBuf<WCHAR> d(g_enc_kPathDeact);
    wcscpy_s(g_pathDeact, d.c_str());
    SecBuf<WCHAR> n(g_enc_kPathAnnounce);
    wcscpy_s(g_pathAnn, n.c_str());
}

inline constexpr int WINDOW_WIDTH   = 480;
inline constexpr int WINDOW_HEIGHT  = 560;
inline constexpr int TITLE_BAR_H    = 40;
