#pragma once
// Update package magic checks and file SHA-256 (UI-independent).

#include <QString>
#include <windows.h>
#include <bcrypt.h>

#pragma comment(lib, "bcrypt.lib")

inline bool ReadFilePrefix(const wchar_t* path, char* out, DWORD outLen, DWORD* fileSizeOut) {
    HANDLE h = CreateFileW(path, GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING, 0, NULL);
    if (h == INVALID_HANDLE_VALUE) return false;
    DWORD rd = 0;
    if (out && outLen) {
        if (!ReadFile(h, out, outLen, &rd, NULL) || rd < outLen) {
            CloseHandle(h);
            return false;
        }
    }
    if (fileSizeOut) *fileSizeOut = GetFileSize(h, NULL);
    CloseHandle(h);
    return true;
}

inline bool FileLooksLikeZip(const wchar_t* path) {
    char magic[2] = {0};
    if (!ReadFilePrefix(path, magic, 2, nullptr)) return false;
    return magic[0] == 'P' && magic[1] == 'K';
}

inline bool FileLooksLikeMsi(const wchar_t* path) {
    char magic[2] = {0};
    if (!ReadFilePrefix(path, magic, 2, nullptr)) return false;
    return magic[0] == (char)0xD0 && magic[1] == (char)0xCF;
}

inline bool FileLooksLikeExe(const wchar_t* path) {
    char magic[2] = {0};
    if (!ReadFilePrefix(path, magic, 2, nullptr)) return false;
    return magic[0] == 'M' && magic[1] == 'Z';
}

// Returns lowercase hex SHA-256 of the file, or empty string on failure.
inline QString Sha256FileHex(const wchar_t* path) {
    HANDLE h = CreateFileW(path, GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING, 0, NULL);
    if (h == INVALID_HANDLE_VALUE) return QString();

    BCRYPT_ALG_HANDLE hAlg = nullptr;
    BCRYPT_HASH_HANDLE hHash = nullptr;
    char fileHashHex[65] = {0};
    bool ok = false;

    if (BCryptOpenAlgorithmProvider(&hAlg, BCRYPT_SHA256_ALGORITHM, nullptr, 0) == 0) {
        if (BCryptCreateHash(hAlg, &hHash, nullptr, 0, nullptr, 0, 0) == 0) {
            char buf[65536];
            DWORD bytesRead = 0;
            ok = true;
            while (ReadFile(h, buf, sizeof(buf), &bytesRead, NULL) && bytesRead > 0) {
                BCryptHashData(hHash, (PUCHAR)buf, bytesRead, 0);
            }
            BYTE hash[32];
            BCryptFinishHash(hHash, hash, 32, 0);
            static const char* hexd = "0123456789abcdef";
            for (int i = 0; i < 32; i++) {
                fileHashHex[i * 2] = hexd[(hash[i] >> 4) & 0xF];
                fileHashHex[i * 2 + 1] = hexd[hash[i] & 0xF];
            }
            BCryptDestroyHash(hHash);
        }
        BCryptCloseAlgorithmProvider(hAlg, 0);
    }
    CloseHandle(h);
    return ok ? QString::fromLatin1(fileHashHex) : QString();
}

inline bool Sha256Matches(const QString& actualHex, const QString& expectedHex) {
    if (expectedHex.isEmpty()) return true;
    return actualHex.compare(expectedHex, Qt::CaseInsensitive) == 0;
}

// packageKind: "msi" or anything else treated as exe/bundle.
inline bool UpdatePackageMagicOk(const wchar_t* path, const QString& packageKind, DWORD minSize = 4096) {
    DWORD fileSize = 0;
    if (!ReadFilePrefix(path, nullptr, 0, &fileSize)) return false;
    if (fileSize < minSize) return false;
    const bool expectsMsi = packageKind.compare("msi", Qt::CaseInsensitive) == 0;
    if (expectsMsi) return FileLooksLikeMsi(path);
    return FileLooksLikeExe(path);
}
