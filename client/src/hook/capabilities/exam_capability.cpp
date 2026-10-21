/* ============================================================================
 * exam_capability.cpp — page.reportExamRecord bridge method
 *
 * Receives exam answer records from AutoExam.js via the bridge, buffers them
 * in a thread-safe queue, and periodically flushes them to the Go server via
 * WinHttp POST with HMAC-SHA256 authentication.
 *
 * The upload runs on a dedicated background thread to avoid blocking the
 * renderer process or bridge dispatch path.
 * ========================================================================= */

#include "pch.h"
#include "bridge_dispatcher.h"
#include "bridge_protocol.h"
#include "script_loader.h"
#include "secret_channel.h"

#include <winhttp.h>
#include <vector>
#include <mutex>
#include <thread>
#include <atomic>

#pragma comment(lib, "winhttp.lib")

/* ---- Configuration ---- */
#define EXAM_FLUSH_INTERVAL_MS   30000   /* base interval between flush attempts */
#define EXAM_FLUSH_MAX_BACKOFF   300000  /* cap backoff at 5 minutes */
#define EXAM_FLUSH_THRESHOLD     5       /* flush immediately at this many records */
#define EXAM_MAX_BUFFER_SIZE     500     /* discard oldest if buffer exceeds this */

/* ---- Thread-safe record buffer ---- */
static std::mutex           g_examMutex;
static std::vector<std::string> g_examBuffer;
static std::atomic<bool>    g_examThreadStarted{false};
static std::atomic<bool>    g_examThreadStop{false};
static std::atomic<int>     g_examFailStreak{0};

/* Upload outcome for backoff / permanent-drop decisions */
enum class ExamUploadResult {
    Ok,
    Retryable,   /* network, 5xx, cert pin — keep batch, back off */
    Permanent    /* 401/403 — drop batch, do not spin */
};

/* ---- Upload result tracking (for diagnostics) ---- */
static std::atomic<long>    g_examTotalUploaded{0};
static std::atomic<long>    g_examTotalFailed{0};

/* ---- Forward declarations ---- */
static void ExamFlushThread();
static ExamUploadResult ExamUploadBatch(const std::vector<std::string>& records);
static bool HookBuildSignatureForBody(const BYTE* key, const std::string& body,
                                       char* sigHex, char* tsBuf, char* nonceHex);

/* =========================================================================
 * Bridge handler — called from BridgeDispatch when JS calls
 * page.reportExamRecord({ records: [...] })
 * ========================================================================= */

static std::string HandlePageReportExamRecord(const BridgeRequest& req) {
    /* Extract the "records" array from params.
     * BridgeJsonGetString with ExtractValue handles arrays/objects as raw JSON. */
    std::string recordsJson = BridgeJsonGetString(req.params, "records");
    if (recordsJson.empty() || recordsJson[0] != '[') {
        /* Single record mode — wrap params in an array */
        recordsJson = "[" + req.params + "]";
    }

    /* Count individual records by counting '{' at depth 1 */
    int count = 0;
    {
        int depth = 0;
        bool inStr = false;
        for (size_t i = 0; i < recordsJson.size(); ++i) {
            char c = recordsJson[i];
            if (inStr) {
                if (c == '\\' && i + 1 < recordsJson.size()) { ++i; continue; }
                if (c == '"') inStr = false;
            } else {
                if (c == '"') inStr = true;
                else if (c == '{') { if (depth == 1) ++count; ++depth; }
                else if (c == '}') --depth;
            }
        }
    }

    /* Add to buffer under lock */
    {
        std::lock_guard<std::mutex> lock(g_examMutex);
        g_examBuffer.push_back(recordsJson);

        /* Safety: cap buffer size */
        if (g_examBuffer.size() > EXAM_MAX_BUFFER_SIZE) {
            size_t discard = g_examBuffer.size() - EXAM_MAX_BUFFER_SIZE;
            g_examBuffer.erase(g_examBuffer.begin(),
                               g_examBuffer.begin() + (long)discard);
        }
    }

    /* Start flush thread on first call */
    if (!g_examThreadStarted.exchange(true)) {
        g_examThreadStop = false;
        std::thread(ExamFlushThread).detach();
        OutputDebugStringA("[EXAM] Flush thread started\n");
    }

    char dbg[128];
    sprintf_s(dbg, "[EXAM] Buffered %d record(s), buffer=%u\n",
              count, (unsigned)g_examBuffer.size());
    OutputDebugStringA(dbg);

    char result[64];
    sprintf_s(result, "{\"queued\":%d,\"buffered\":%u}",
              count, (unsigned)g_examBuffer.size());
    return std::string(result);
}

/* =========================================================================
 * Background flush thread
 * ========================================================================= */

static void ExamFlushThread() {
    while (!g_examThreadStop) {
        int failStreak = g_examFailStreak.load();
        DWORD waitMs = EXAM_FLUSH_INTERVAL_MS;
        if (failStreak > 0) {
            /* Exponential backoff: 30s, 60s, 120s, 240s, capped at 5min */
            DWORD shift = (DWORD)((failStreak > 4) ? 4 : failStreak);
            waitMs = EXAM_FLUSH_INTERVAL_MS << shift;
            if (waitMs > EXAM_FLUSH_MAX_BACKOFF) waitMs = EXAM_FLUSH_MAX_BACKOFF;
        }
        for (DWORD elapsed = 0; elapsed < waitMs && !g_examThreadStop; elapsed += 100) {
            Sleep(100);
        }
        if (g_examThreadStop) break;

        std::vector<std::string> batch;
        {
            std::lock_guard<std::mutex> lock(g_examMutex);
            if (g_examBuffer.empty()) continue;
            batch.swap(g_examBuffer);
        }

        ExamUploadResult result = ExamUploadBatch(batch);
        if (result == ExamUploadResult::Ok) {
            g_examFailStreak = 0;
            long total = 0;
            for (auto& r : batch) {
                for (size_t i = 0; i < r.size(); ++i)
                    if (r[i] == '{') ++total;
            }
            g_examTotalUploaded += total;
            char dbg[128];
            sprintf_s(dbg, "[EXAM] Uploaded batch (%u entries, ~%ld records)\n",
                      (unsigned)batch.size(), total);
            OutputDebugStringA(dbg);
        } else if (result == ExamUploadResult::Permanent) {
            /* Auth rejected — do not requeue forever */
            g_examFailStreak = 0;
            ++g_examTotalFailed;
            char dbg[160];
            sprintf_s(dbg, "[EXAM] Upload permanently rejected (auth), dropping %u entries\n",
                      (unsigned)batch.size());
            OutputDebugStringA(dbg);
        } else {
            {
                std::lock_guard<std::mutex> lock(g_examMutex);
                g_examBuffer.insert(g_examBuffer.begin(), batch.begin(), batch.end());
                if (g_examBuffer.size() > EXAM_MAX_BUFFER_SIZE) {
                    size_t discard = g_examBuffer.size() - EXAM_MAX_BUFFER_SIZE;
                    g_examBuffer.erase(g_examBuffer.begin(),
                                       g_examBuffer.begin() + (long)discard);
                }
            }
            int streak = ++g_examFailStreak;
            ++g_examTotalFailed;
            char dbg[128];
            sprintf_s(dbg, "[EXAM] Upload failed (retryable), streak=%d, batch re-queued\n", streak);
            OutputDebugStringA(dbg);
        }
    }
}

/* =========================================================================
 * WinHttp upload — POST /api/v1/exam/report with HMAC auth
 *
 * Builds the JSON body, signs it with PBKDF2-derived HMAC key, and
 * sends via WinHttp with cert pin verification.
 * ========================================================================= */

static ExamUploadResult ExamUploadBatch(const std::vector<std::string>& recordArrays) {
    if (recordArrays.empty()) return ExamUploadResult::Ok;

    /* Get session token from secret channel */
    const LqSecretBundle& secrets = LqSecretsCached();
    if (secrets.sessionToken.empty()) {
        OutputDebugStringA("[EXAM] Upload skipped: no session token\n");
        return ExamUploadResult::Retryable;
    }
    std::string sessionTokenUtf8 = secrets.sessionToken;

    /* Decrypt host / client ID / secret */
    SecBuf<WCHAR> host(g_enc_kHost);
    SecBuf<WCHAR> clientId(g_enc_kClientId);
    SecBuf<char> secret(g_enc_kSecret);

    BYTE key[32];
    if (!HookDeriveKey(secret.c_str(), key, sizeof(key))) {
        OutputDebugStringA("[EXAM] Upload skipped: key derivation failed\n");
        return ExamUploadResult::Retryable;
    }

    /* Build JSON body:
     * { "session_token": "...", "machine_id": "...", "card": "...", "questions": [...] }
     *
     * We merge all buffered record arrays into a single questions array. */
    std::string questionsJson = "[";
    bool first = true;
    for (auto& arr : recordArrays) {
        /* arr is like '[{...},{...}]' or '{...}'
         * Strip outer brackets to get individual record objects */
        std::string inner = arr;
        if (inner.size() >= 2 && inner[0] == '[' && inner.back() == ']') {
            inner = inner.substr(1, inner.size() - 2);
        }
        if (inner.empty()) continue;
        if (!first) questionsJson += ",";
        questionsJson += inner;
        first = false;
    }
    questionsJson += "]";

    /* Escape session token for JSON embedding */
    std::string body;
    body.reserve(questionsJson.size() + 256);
    body += "{\"session_token\":\"";
    body += BridgeJsonEscape(sessionTokenUtf8);
    body += "\",\"questions\":";
    body += questionsJson;
    body += "}";

    /* Sign the body */
    const wchar_t* path = L"/api/v1/exam/report";
    char sigHex[65] = {0};
    char tsBuf[32] = {0};
    char nonceHex[33] = {0};
    if (!HookBuildSignatureForBody(key, body, sigHex, tsBuf, nonceHex)) {
        OutputDebugStringA("[EXAM] Upload skipped: signature failed\n");
        SecureZeroMemory(key, sizeof(key));
        return ExamUploadResult::Retryable;
    }

    /* WinHttp session */
    HINTERNET hSession = WinHttpOpen(L"HOOK-EXAM/1.0", WINHTTP_ACCESS_TYPE_DEFAULT_PROXY, NULL, NULL, 0);
    if (!hSession) { SecureZeroMemory(key, sizeof(key)); return ExamUploadResult::Retryable; }

    HINTERNET hConnect = WinHttpConnect(hSession, host.c_str(), 443, 0);
    if (!hConnect) {
        WinHttpCloseHandle(hSession);
        SecureZeroMemory(key, sizeof(key));
        return ExamUploadResult::Retryable;
    }

    HINTERNET hRequest = WinHttpOpenRequest(hConnect, L"POST", path, NULL, NULL, NULL, WINHTTP_FLAG_SECURE);
    if (!hRequest) {
        WinHttpCloseHandle(hConnect);
        WinHttpCloseHandle(hSession);
        SecureZeroMemory(key, sizeof(key));
        return ExamUploadResult::Retryable;
    }

    /* Headers: HMAC + Content-Type */
    std::wstring sessionTokenWide = LqUtf8ToWide(secrets.sessionToken);
    wchar_t headers[2048];
    swprintf_s(headers, _countof(headers),
        L"Content-Type: application/json\r\n"
        L"X-Client-ID: %s\r\n"
        L"X-HMAC-Signature: %S\r\n"
        L"X-Timestamp: %S\r\n"
        L"X-Nonce: %S\r\n"
        L"X-Session-Token: %s\r\n",
        clientId.c_str(), sigHex, tsBuf, nonceHex, sessionTokenWide.c_str());
    WinHttpAddRequestHeaders(hRequest, headers, (DWORD)wcslen(headers), WINHTTP_ADDREQ_FLAG_ADD);

    /* Ignore self-signed CA */
    DWORD secFlags = SECURITY_FLAG_IGNORE_UNKNOWN_CA;
    WinHttpSetOption(hRequest, WINHTTP_OPTION_SECURITY_FLAGS, &secFlags, sizeof(secFlags));
    WinHttpSetTimeouts(hRequest, 5000, 5000, 10000, 15000);

    /* Send */
    ExamUploadResult outcome = ExamUploadResult::Retryable;
    if (WinHttpSendRequest(hRequest, WINHTTP_NO_ADDITIONAL_HEADERS, 0,
                           body.data(), (DWORD)body.size(), (DWORD)body.size(), 0) &&
        WinHttpReceiveResponse(hRequest, NULL)) {
        if (!HookVerifyServerCert(hRequest)) {
            OutputDebugStringA("[EXAM] Upload failed: server cert pin mismatch\n");
        } else {
            DWORD status = 0, statusSize = sizeof(status);
            WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
                                NULL, &status, &statusSize, NULL);
            if (status == 200) {
                outcome = ExamUploadResult::Ok;
            } else if (status == 401 || status == 403) {
                char dbg[128];
                sprintf_s(dbg, "[EXAM] Upload auth rejected HTTP %lu\n", status);
                OutputDebugStringA(dbg);
                outcome = ExamUploadResult::Permanent;
            } else {
                char dbg[160];
                DWORD bodyLen = 0;
                WinHttpQueryDataAvailable(hRequest, &bodyLen);
                sprintf_s(dbg, "[EXAM] Upload returned HTTP %lu (body_avail=%lu, payload=%zu)\n",
                          status, bodyLen, body.size());
                OutputDebugStringA(dbg);
                outcome = ExamUploadResult::Retryable;
            }
        }
    } else {
        DWORD err = GetLastError();
        char dbg[128];
        sprintf_s(dbg, "[EXAM] Upload network error win32=%lu\n", err);
        OutputDebugStringA(dbg);
        outcome = ExamUploadResult::Retryable;
    }

    WinHttpCloseHandle(hRequest);
    WinHttpCloseHandle(hConnect);
    WinHttpCloseHandle(hSession);
    SecureZeroMemory(key, sizeof(key));
    return outcome;
}

/* =========================================================================
 * HMAC signature for POST body
 * Signs: timestamp|nonce|body (where body is the full JSON payload)
 * ========================================================================= */

static bool HookBuildSignatureForBody(const BYTE* key, const std::string& body,
                                       char* sigHex, char* tsBuf, char* nonceHex) {
    _i64toa_s(GetUnixTimestamp(), tsBuf, 32, 10);
    if (!GenerateNonce(nonceHex)) return false;

    BYTE sig[32];
    DWORD sigLen = 0;
    if (!HmacSha256Signed((const char*)key, 32, tsBuf, nonceHex,
                          body.data(), (DWORD)body.size(), sig, &sigLen)) {
        return false;
    }
    ByteToHex(sig, sigLen, sigHex);
    return true;
}

/* =========================================================================
 * Shutdown hook — call from DLL cleanup to stop the flush thread
 * ========================================================================= */

void ExamCapabilityShutdown() {
    g_examThreadStop = true;
    /* Thread will exit within EXAM_FLUSH_INTERVAL_MS / 10 iterations */
}

/* ---- Registration ---- */
void RegisterExamCapabilities() {
    BridgeRegisterMethod(LQ_METHOD_PAGE_REPORT_EXAM, HandlePageReportExamRecord);
}

/* Exposed for diagnostics.getHookState */
void ExamCapabilityGetStats(long* uploaded, long* failed, unsigned* buffered, int* failStreak) {
    if (uploaded) *uploaded = g_examTotalUploaded.load();
    if (failed) *failed = g_examTotalFailed.load();
    if (buffered) {
        std::lock_guard<std::mutex> lock(g_examMutex);
        *buffered = (unsigned)g_examBuffer.size();
    }
    if (failStreak) *failStreak = g_examFailStreak.load();
}
