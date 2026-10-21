/* ============================================================================
 * v8_intercept.cpp — 参考解答: V8 拦截引擎 + Bridge 加解密（完全修复版）
 * ========================================================================= */

#include "v8_intercept.h"
#include "bridge_protocol.h"

#include <cstring>
#include <cstdlib>
#include <cstdio>
#include <mutex>
#include <unordered_map>
#include <string>
#include <vector>
#include <atomic>

static std::mutex g_interceptLock;
static std::atomic<bool> g_initialized{false};

struct InterceptRule {
    std::string urlPattern;
    std::string replacement;
    bool blockCompile;
};

static std::vector<InterceptRule> g_rules;

struct PendingRequest {
    std::string id;
    uint64_t timestamp;
    int retryCount;
};
static std::unordered_map<std::string, PendingRequest> g_pendingRequests;
static std::mutex g_pendingLock;

void V8InterceptInit(void) {
    g_initialized.store(true);
}

void V8InterceptShutdown(void) {
    std::lock_guard<std::mutex> lock(g_interceptLock);
    g_rules.clear();
    g_initialized.store(false);
}

int V8InterceptCompileScript(void* esp_frame) {
    if (!esp_frame || !g_initialized.load()) {
        return 0;
    }

    const char** sourcePtr = (const char**)esp_frame;
    if (!sourcePtr || !*sourcePtr) {
        return 0;
    }

    const char* source = *sourcePtr;
    size_t sourceLen = strlen(source);
    (void)sourceLen;

    std::lock_guard<std::mutex> lock(g_interceptLock);
    for (const auto& rule : g_rules) {
        if (strstr(source, rule.urlPattern.c_str()) != nullptr) {
            if (rule.blockCompile) {
                return 1;
            }

            // 修复: 安全的源码替换
            // 先分配新缓冲区，失败时保持原始状态
            char* newSource = (char*)malloc(rule.replacement.size() + 1);
            if (!newSource) {
                // 修复: malloc 失败时不崩溃，保持原始源码
                return 0;
            }
            memcpy(newSource, rule.replacement.c_str(), rule.replacement.size() + 1);
            *sourcePtr = newSource;
            return 0;
        }
    }

    return 0;
}

void* V8InterceptSetCallHandler(void* esp_frame) {
    if (!esp_frame) return nullptr;
    void** args = (void**)esp_frame;
    return args[2];
}

int V8InterceptScriptRun(void* esp_frame) {
    (void)esp_frame;
    return 0;
}

/* ============================================================================
 * Bridge IPC 加密 — 完整实现
 * ========================================================================= */

#define AES_GCM_NONCE_SIZE  12
#define AES_GCM_TAG_SIZE    16
#define HEADER_SIZE         4

int BridgeEncrypt(
    const unsigned char* plaintext, size_t plainLen,
    const unsigned char* key,
    unsigned char* outBuf, size_t outBufSize, size_t* outLen)
{
    // 参数验证
    if (!plaintext || !key || !outBuf || !outLen) {
        return -1;
    }

    // 计算输出长度
    size_t totalSize = HEADER_SIZE + AES_GCM_NONCE_SIZE + plainLen + AES_GCM_TAG_SIZE;
    if (outBufSize < totalSize) {
        return -2; // 缓冲区太小
    }

    // 生成随机 nonce
    unsigned char nonce[AES_GCM_NONCE_SIZE];
    if (pal_random_bytes(nonce, AES_GCM_NONCE_SIZE) != 0) {
        return -3;
    }

    // 写入长度头 (big-endian)
    outBuf[0] = (unsigned char)((plainLen >> 24) & 0xFF);
    outBuf[1] = (unsigned char)((plainLen >> 16) & 0xFF);
    outBuf[2] = (unsigned char)((plainLen >> 8) & 0xFF);
    outBuf[3] = (unsigned char)(plainLen & 0xFF);

    // 写入 nonce
    memcpy(outBuf + HEADER_SIZE, nonce, AES_GCM_NONCE_SIZE);

    // 加密
    unsigned char* ciphertext = outBuf + HEADER_SIZE + AES_GCM_NONCE_SIZE;
    unsigned char tag[AES_GCM_TAG_SIZE];

    if (pal_aes_gcm_encrypt(key, nonce, AES_GCM_NONCE_SIZE,
                            plaintext, plainLen, ciphertext, tag) != 0) {
        return -3;
    }

    // 追加 tag
    memcpy(ciphertext + plainLen, tag, AES_GCM_TAG_SIZE);

    *outLen = totalSize;
    return 0;
}

int BridgeDecrypt(
    const unsigned char* ciphertext, size_t cipherLen,
    const unsigned char* key,
    unsigned char* outBuf, size_t outBufSize, size_t* outLen)
{
    // 参数验证
    if (!ciphertext || !key || !outBuf || !outLen) {
        return -1;
    }

    // 最小长度检查
    size_t minLen = HEADER_SIZE + AES_GCM_NONCE_SIZE + AES_GCM_TAG_SIZE;
    if (cipherLen < minLen) {
        return -2; // 密文太短
    }

    // 读取长度头
    size_t payloadLen = ((size_t)ciphertext[0] << 24) |
                        ((size_t)ciphertext[1] << 16) |
                        ((size_t)ciphertext[2] << 8) |
                        ((size_t)ciphertext[3]);

    // 验证长度一致性
    size_t expectedTotal = HEADER_SIZE + AES_GCM_NONCE_SIZE + payloadLen + AES_GCM_TAG_SIZE;
    if (cipherLen != expectedTotal) {
        return -3; // 长度不匹配
    }

    // 输出缓冲区检查
    if (outBufSize < payloadLen) {
        return -5;
    }

    // 提取 nonce
    const unsigned char* nonce = ciphertext + HEADER_SIZE;

    // 提取密文和 tag
    const unsigned char* encData = ciphertext + HEADER_SIZE + AES_GCM_NONCE_SIZE;
    const unsigned char* tag = encData + payloadLen;

    // 解密
    if (pal_aes_gcm_decrypt(key, nonce, AES_GCM_NONCE_SIZE,
                            encData, payloadLen, tag, outBuf) != 0) {
        return -4; // 认证失败
    }

    *outLen = payloadLen;
    return 0;
}
