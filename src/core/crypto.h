#pragma once
// ============================================================================
// Cryptographic utilities — HMAC-SHA256, hex encoding, TLS cert pinning
// ============================================================================
#include <windows.h>
#include "strcrypt.h"
#include <winhttp.h>
#include <bcrypt.h>

#ifndef NT_SUCCESS
#define NT_SUCCESS(status) ((status) >= 0)
#endif

static bool HmacSha256(const char* key, DWORD keyLen,
                       const char* data, DWORD dataLen,
                       BYTE* output, DWORD* outputLen)
{
    BCRYPT_ALG_HANDLE hAlg = NULL;
    NTSTATUS status = BCryptOpenAlgorithmProvider(&hAlg, BCRYPT_SHA256_ALGORITHM,
        NULL, BCRYPT_ALG_HANDLE_HMAC_FLAG);
    if (!NT_SUCCESS(status)) return false;
    DWORD hashObjSize = 0, resultLen = 0;
    BCryptGetProperty(hAlg, BCRYPT_OBJECT_LENGTH, (PBYTE)&hashObjSize, sizeof(hashObjSize), &resultLen, 0);
    DWORD hashLen = 0;
    BCryptGetProperty(hAlg, BCRYPT_HASH_LENGTH, (PBYTE)&hashLen, sizeof(hashLen), &resultLen, 0);
    BYTE* hashObj = (BYTE*)HeapAlloc(GetProcessHeap(), 0, hashObjSize);
    if (!hashObj) { BCryptCloseAlgorithmProvider(hAlg, 0); return false; }
    BCRYPT_HASH_HANDLE hHash = NULL;
    status = BCryptCreateHash(hAlg, &hHash, hashObj, hashObjSize, (PBYTE)key, keyLen, 0);
    if (NT_SUCCESS(status)) {
        BCryptHashData(hHash, (PBYTE)data, dataLen, 0);
        BCryptFinishHash(hHash, output, hashLen, 0);
        *outputLen = hashLen;
        BCryptDestroyHash(hHash);
    }
    HeapFree(GetProcessHeap(), 0, hashObj);
    BCryptCloseAlgorithmProvider(hAlg, 0);
    return NT_SUCCESS(status);
}

static void ByteToHex(const BYTE* input, DWORD inputLen, char* output) {
    static const char hex[] = "0123456789abcdef";
    for (DWORD i = 0; i < inputLen; i++) {
        output[i * 2]     = hex[(input[i] >> 4) & 0xF];
        output[i * 2 + 1] = hex[input[i] & 0xF];
    }
    output[inputLen * 2] = 0;
}

// Generate a random nonce (16 bytes → 32 hex chars)
static void GenerateNonce(char* output) {
    BYTE buf[16];
    if (BCryptGenRandom(NULL, buf, sizeof(buf), BCRYPT_USE_SYSTEM_PREFERRED_RNG) != 0) {
        HCRYPTPROV hProv = 0;
        if (CryptAcquireContextW(&hProv, NULL, NULL, PROV_RSA_FULL, CRYPT_VERIFYCONTEXT)) {
            CryptGenRandom(hProv, sizeof(buf), buf);
            CryptReleaseContext(hProv, 0);
        } else {
            memset(buf, 0, sizeof(buf));
        }
    }
    ByteToHex(buf, 16, output);
}

// Get current Unix timestamp in seconds
static __int64 GetUnixTimestamp() {
    FILETIME ft;
    GetSystemTimeAsFileTime(&ft);
    __int64 t = ((__int64)ft.dwHighDateTime << 32) | ft.dwLowDateTime;
    return (t - 116444736000000000LL) / 10000000LL;
}

// Build HMAC signed data: "timestamp|nonce|body" for anti-replay
static bool HmacSha256Signed(const char* key, DWORD keyLen,
                              const char* timestamp, const char* nonce,
                              const char* body, DWORD bodyLen,
                              BYTE* output, DWORD* outputLen)
{
    // Concatenate: timestamp + "|" + nonce + "|" + body
    DWORD tsLen = (DWORD)strlen(timestamp);
    DWORD nonceLen = (DWORD)strlen(nonce);
    DWORD totalLen = tsLen + 1 + nonceLen + 1 + bodyLen;
    char* buf = (char*)HeapAlloc(GetProcessHeap(), 0, totalLen);
    if (!buf) return false;
    memcpy(buf, timestamp, tsLen);
    buf[tsLen] = '|';
    memcpy(buf + tsLen + 1, nonce, nonceLen);
    buf[tsLen + 1 + nonceLen] = '|';
    memcpy(buf + tsLen + 1 + nonceLen + 1, body, bodyLen);
    bool ok = HmacSha256(key, keyLen, buf, totalLen, output, outputLen);
    HeapFree(GetProcessHeap(), 0, buf);
    return ok;
}

// PBKDF2-HMAC-SHA256 key derivation — derives HMAC key from master secret
// so the raw secret never appears as a signing key directly.
static bool DeriveKey(const char* masterSecret, DWORD secretLen,
                      const char* salt, DWORD saltLen,
                      BYTE* derivedKey, DWORD derivedKeyLen)
{
    BCRYPT_ALG_HANDLE hAlg = NULL;
    NTSTATUS status = BCryptOpenAlgorithmProvider(&hAlg, BCRYPT_SHA256_ALGORITHM,
        NULL, BCRYPT_ALG_HANDLE_HMAC_FLAG);
    if (!NT_SUCCESS(status)) return false;

    status = BCryptDeriveKeyPBKDF2(hAlg,
        (PUCHAR)masterSecret, secretLen,
        (PUCHAR)salt, saltLen,
        100000, // iterations
        derivedKey, derivedKeyLen,
        0);
    BCryptCloseAlgorithmProvider(hAlg, 0);
    return NT_SUCCESS(status);
}

// SHA-256 fingerprint of the server's self-signed TLS certificate.
// Regenerate when certs/server.crt is renewed:
//   openssl x509 -in certs/server.crt -outform DER | sha256sum
static const char* CERT_PIN_SHA256 = LQ_SP("9f3435586bdb2528c1b1b460748782a96e2670ff4690b388796c85241b306458");

// ============================================================================
// AES-256-GCM encryption/decryption (via BCrypt)
// More secure than XOR for DLL encryption.
// ============================================================================

// AES-256-GCM encrypt. Returns true on success.
// output must have room for dataLen + 16 bytes (tag).
// *outputLen = dataLen + 16 (tag appended).
static bool AesGcmEncrypt(const BYTE* key, DWORD keyLen,
                          const BYTE* iv, DWORD ivLen,
                          const BYTE* data, DWORD dataLen,
                          BYTE* output, DWORD* outputLen)
{
    BCRYPT_ALG_HANDLE hAlg = NULL;
    BCRYPT_KEY_HANDLE hKey = NULL;
    NTSTATUS status;

    status = BCryptOpenAlgorithmProvider(&hAlg, BCRYPT_AES_ALGORITHM, NULL, 0);
    if (!NT_SUCCESS(status)) return false;

    // Set GCM mode
    status = BCryptSetProperty(hAlg, BCRYPT_CHAINING_MODE,
        (PBYTE)BCRYPT_CHAIN_MODE_GCM, sizeof(BCRYPT_CHAIN_MODE_GCM), 0);
    if (!NT_SUCCESS(status)) { BCryptCloseAlgorithmProvider(hAlg, 0); return false; }

    // Import key
    status = BCryptGenerateSymmetricKey(hAlg, &hKey, NULL, 0, (PBYTE)key, keyLen, 0);
    if (!NT_SUCCESS(status)) { BCryptCloseAlgorithmProvider(hAlg, 0); return false; }

    // GCM auth info
    BCRYPT_AUTHENTICATED_CIPHER_MODE_INFO authInfo;
    BCRYPT_INIT_AUTH_MODE_INFO(authInfo);
    authInfo.pbNonce = (PBYTE)iv;
    authInfo.cbNonce = ivLen;
    authInfo.pbTag = output + dataLen;
    authInfo.cbTag = 16; // GCM tag is 16 bytes

    DWORD resultLen = 0;
    status = BCryptEncrypt(hKey, (PBYTE)data, dataLen, &authInfo,
        NULL, 0, output, dataLen, &resultLen, 0);

    *outputLen = dataLen + 16;

    BCryptDestroyKey(hKey);
    BCryptCloseAlgorithmProvider(hAlg, 0);
    return NT_SUCCESS(status);
}

// AES-256-GCM decrypt. Returns true on success.
// data must include the 16-byte GCM tag at the end.
// *outputLen = dataLen - 16.
static bool AesGcmDecrypt(const BYTE* key, DWORD keyLen,
                          const BYTE* iv, DWORD ivLen,
                          const BYTE* data, DWORD dataLen,
                          BYTE* output, DWORD* outputLen)
{
    if (dataLen < 16) return false;

    BCRYPT_ALG_HANDLE hAlg = NULL;
    BCRYPT_KEY_HANDLE hKey = NULL;
    NTSTATUS status;

    status = BCryptOpenAlgorithmProvider(&hAlg, BCRYPT_AES_ALGORITHM, NULL, 0);
    if (!NT_SUCCESS(status)) return false;

    status = BCryptSetProperty(hAlg, BCRYPT_CHAINING_MODE,
        (PBYTE)BCRYPT_CHAIN_MODE_GCM, sizeof(BCRYPT_CHAIN_MODE_GCM), 0);
    if (!NT_SUCCESS(status)) { BCryptCloseAlgorithmProvider(hAlg, 0); return false; }

    status = BCryptGenerateSymmetricKey(hAlg, &hKey, NULL, 0, (PBYTE)key, keyLen, 0);
    if (!NT_SUCCESS(status)) { BCryptCloseAlgorithmProvider(hAlg, 0); return false; }

    DWORD cipherLen = dataLen - 16;
    BCRYPT_AUTHENTICATED_CIPHER_MODE_INFO authInfo;
    BCRYPT_INIT_AUTH_MODE_INFO(authInfo);
    authInfo.pbNonce = (PBYTE)iv;
    authInfo.cbNonce = ivLen;
    authInfo.pbTag = (PBYTE)(data + cipherLen);
    authInfo.cbTag = 16;

    DWORD resultLen = 0;
    status = BCryptDecrypt(hKey, (PBYTE)data, cipherLen, &authInfo,
        NULL, 0, output, cipherLen, &resultLen, 0);

    *outputLen = resultLen;

    BCryptDestroyKey(hKey);
    BCryptCloseAlgorithmProvider(hAlg, 0);
    return NT_SUCCESS(status);
}

static bool VerifyServerCert(HINTERNET hRequest) {
    CERT_CONTEXT* pCert = nullptr;
    DWORD certSize = sizeof(pCert);
    if (!WinHttpQueryOption(hRequest, WINHTTP_OPTION_SERVER_CERT_CONTEXT, &pCert, &certSize) || !pCert)
        return false;

    BCRYPT_ALG_HANDLE hAlg = nullptr;
    BCRYPT_HASH_HANDLE hHash = nullptr;
    bool ok = false;

    if (BCryptOpenAlgorithmProvider(&hAlg, BCRYPT_SHA256_ALGORITHM, nullptr, 0) == 0) {
        if (BCryptCreateHash(hAlg, &hHash, nullptr, 0, nullptr, 0, 0) == 0) {
            BCryptHashData(hHash, (PUCHAR)pCert->pbCertEncoded, pCert->cbCertEncoded, 0);
            BYTE hash[32];
            BCryptFinishHash(hHash, hash, 32, 0);
            char hex[65];
            ByteToHex(hash, 32, hex);
            ok = (_stricmp(hex, CERT_PIN_SHA256) == 0);
            BCryptDestroyHash(hHash);
        }
        BCryptCloseAlgorithmProvider(hAlg, 0);
    }
    CertFreeCertificateContext(pCert);
    return ok;
}

// ============================================================================
// DPAPI — Data Protection API for local secret storage
// ============================================================================
#include <wincrypt.h>
#pragma comment(lib, "crypt32.lib")

// Encrypt plaintext with DPAPI (user-scoped). Returns Base64 string.
static QString DpapiProtect(const QByteArray& plaintext) {
    DATA_BLOB in, out;
    in.pbData = (BYTE*)plaintext.constData();
    in.cbData = (DWORD)plaintext.size();
    if (!CryptProtectData(&in, L"LingQiao", nullptr, nullptr, nullptr, CRYPTPROTECT_UI_FORBIDDEN, &out))
        return {};
    QByteArray cipher((const char*)out.pbData, (int)out.cbData);
    LocalFree(out.pbData);
    return cipher.toBase64();
}

// Decrypt Base64 DPAPI ciphertext. Returns plaintext bytes.
static QByteArray DpapiUnprotect(const QString& base64Cipher) {
    QByteArray cipher = QByteArray::fromBase64(base64Cipher.toUtf8());
    DATA_BLOB in, out;
    in.pbData = (BYTE*)cipher.constData();
    in.cbData = (DWORD)cipher.size();
    if (!CryptUnprotectData(&in, nullptr, nullptr, nullptr, nullptr, CRYPTPROTECT_UI_FORBIDDEN, &out))
        return {};
    QByteArray plain((const char*)out.pbData, (int)out.cbData);
    LocalFree(out.pbData);
    return plain;
}
