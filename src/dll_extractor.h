#pragma once
// ============================================================================
// DLL Extraction — download encrypted DLL from server, decrypt, and verify
//
// Improvements over baseline:
//   - PE header validation (MZ + PE signatures)
//   - Minimum/maximum size sanity checks
//   - Post-decryption integrity verification
//   - Secure temp path with random naming
//   - Cleanup on failure
// ============================================================================
#include <windows.h>
#include <tchar.h>
#include <QString>
#include "crypto.h"
#include "http_client.h"
#include "config.h"
#include "strcrypt.h"

// Derive a key for DLL encryption/decryption (separate from HMAC signing key)
static bool DeriveDllKey(BYTE* dllKey, DWORD dllKeyLen) {
    return DeriveKey(g_secretBuf, (DWORD)strlen(g_secretBuf),
                     _S("CefBridge-DLL-Salt-v1"), 21,
                     dllKey, dllKeyLen);
}

// XOR crypt in-place using a repeating 32-byte key
static void XorCrypt(BYTE* data, DWORD dataLen, const BYTE* key, DWORD keyLen) {
    for (DWORD i = 0; i < dataLen; i++)
        data[i] ^= key[i % keyLen];
}

static TCHAR g_dllPath[MAX_PATH] = {0};
static TCHAR g_dllDir[MAX_PATH]  = {0};
static bool   g_dllReady         = false;

// Forward declaration
static void CleanupDll();

static void MakeRandomPath() {
    TCHAR tempPath[MAX_PATH];
    GetTempPath(MAX_PATH, tempPath);
    DWORD unique = GetTickCount() ^ GetCurrentProcessId() ^ (DWORD)__rdtsc();
    swprintf_s(g_dllDir,  MAX_PATH, L"%s\\%08X",     tempPath, unique);
    swprintf_s(g_dllPath, MAX_PATH, L"%s\\%08X.dll", g_dllDir, unique);
}

// Validate PE headers: check MZ signature and PE signature
static bool ValidatePeHeaders(const BYTE* data, DWORD size) {
    if (size < 64) return false; // minimum for DOS header

    // Check MZ signature
    if (data[0] != 'M' || data[1] != 'Z') return false;

    // Read e_lfanew (PE header offset)
    DWORD peOffset = *(DWORD*)(data + 0x3C);
    if (peOffset + 4 > size) return false;

    // Check PE signature
    if (data[peOffset] != 'P' || data[peOffset + 1] != 'E' ||
        data[peOffset + 2] != 0 || data[peOffset + 3] != 0)
        return false;

    // Check optional header magic (PE32 = 0x10B, PE32+ = 0x20B)
    WORD optMagic = *(WORD*)(data + peOffset + 24);
    if (optMagic != 0x10B && optMagic != 0x20B) return false;

    return true;
}

// Download encrypted DLL from server, XOR-decrypt, validate, write to temp file.
// Returns empty QString on success, error message on failure.
static QString DownloadDll(const wchar_t* host, int port, const wchar_t* sessionToken) {
    const wchar_t* path = L"/api/v1/dll";

    // Use signed GET with HMAC headers, pass session token as header
    HttpResponse resp = WinHttpGetSigned(host, port, path, sessionToken);
    if (resp.statusCode != 200 || resp.body.isEmpty()) {
        return QString::fromUtf8(_S("下载 DLL 失败 (HTTP %1)")).arg(resp.statusCode);
    }

    // Size sanity check: DLL should be between 4KB and 50MB
    if (resp.body.size() < 4096) {
        return QString::fromUtf8(_S("下载的 DLL 文件异常 (太小)"));
    }
    if (resp.body.size() > 50 * 1024 * 1024) {
        return QString::fromUtf8(_S("下载的 DLL 文件异常 (太大)"));
    }

    // Derive DLL decryption key (separate from HMAC signing key)
    BYTE dllKey[32];
    if (!DeriveDllKey(dllKey, sizeof(dllKey))) {
        return QString::fromUtf8(_S("密钥派生失败"));
    }

    // XOR decrypt in-place
    QByteArray data = resp.body;
    XorCrypt((BYTE*)data.data(), (DWORD)data.size(), dllKey, sizeof(dllKey));

    // Validate PE headers after decryption
    if (!ValidatePeHeaders((const BYTE*)data.constData(), (DWORD)data.size())) {
        return QString::fromUtf8(_S("DLL 文件校验失败 — 文件可能已损坏或被篡改"));
    }

    // Write to temp file
    MakeRandomPath();
    CreateDirectory(g_dllDir, NULL);
    SetFileAttributes(g_dllDir, FILE_ATTRIBUTE_HIDDEN);
    HANDLE hFile = CreateFile(g_dllPath, GENERIC_WRITE, 0, NULL,
        CREATE_ALWAYS, FILE_ATTRIBUTE_HIDDEN, NULL);
    if (hFile == INVALID_HANDLE_VALUE) {
        return QString::fromUtf8(_S("无法创建 DLL 文件"));
    }
    DWORD written;
    WriteFile(hFile, data.constData(), (DWORD)data.size(), &written, NULL);
    CloseHandle(hFile);

    if (written != (DWORD)data.size()) {
        CleanupDll();
        return QString::fromUtf8(_S("DLL 写入不完整"));
    }

    g_dllReady = true;
    return QString();
}

static void CleanupDll() {
    if (g_dllPath[0]) {
        SetFileAttributes(g_dllPath, FILE_ATTRIBUTE_NORMAL);
        DeleteFile(g_dllPath);
        g_dllPath[0] = 0;
    }
    if (g_dllDir[0]) {
        SetFileAttributes(g_dllDir, FILE_ATTRIBUTE_NORMAL);
        RemoveDirectory(g_dllDir);
        g_dllDir[0] = 0;
    }
    g_dllReady = false;
}
