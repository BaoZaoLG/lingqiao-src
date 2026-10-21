/* ============================================================================
 * v8_intercept.cpp — V8 拦截引擎实现
 *
 * 实现脚本拦截、Handler 劫持和 Bridge IPC 加密通信。
 *
 * 已知缺陷:
 *   - 脚本替换失败时未恢复原始内存
 *   - 缺少异常保护（SEH/signal handler）
 *   - Bridge 加解密函数未实现
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

/* ============================================================================
 * 内部状态
 * ========================================================================= */

static std::mutex g_interceptLock;
static std::atomic<bool> g_initialized{false};

/* 脚本拦截规则表 */
struct InterceptRule {
    std::string urlPattern;
    std::string replacement;
    bool blockCompile;
};

static std::vector<InterceptRule> g_rules;

/* 请求-响应状态机 */
struct PendingRequest {
    std::string id;
    uint64_t timestamp;
    int retryCount;
};
static std::unordered_map<std::string, PendingRequest> g_pendingRequests;
static std::mutex g_pendingLock;

/* ============================================================================
 * V8 拦截实现
 * ========================================================================= */

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
        return 0;  /* pass-through */
    }

    /* 模拟从栈帧中读取脚本源码指针
     * 在真实实现中，这里通过 esp_frame 的偏移量读取 V8 参数 */
    const char** sourcePtr = (const char**)esp_frame;
    if (!sourcePtr || !*sourcePtr) {
        return 0;
    }

    const char* source = *sourcePtr;
    size_t sourceLen = strlen(source);

    /* 查找匹配的拦截规则 */
    std::lock_guard<std::mutex> lock(g_interceptLock);
    for (const auto& rule : g_rules) {
        if (strstr(source, rule.urlPattern.c_str()) != nullptr) {
            if (rule.blockCompile) {
                return 1;  /* 阻止编译 */
            }

            /* BUG: 源码替换 — 如果替换失败（如内存不足），原始内存已被部分覆盖
             * 正确做法：先分配新缓冲区，成功后再替换指针
             * TODO: 实现安全的源码替换，失败时恢复原始内存 */
            char* newSource = (char*)malloc(rule.replacement.size() + 1);
            // BUG: 没有检查 malloc 返回值
            // 如果 malloc 失败，下面的 memcpy 会写入 NULL 指针导致崩溃
            memcpy(newSource, rule.replacement.c_str(), rule.replacement.size() + 1);

            /* 直接覆盖原始指针 — 危险！旧内存泄漏且无法恢复 */
            *sourcePtr = newSource;
            return 0;
        }
    }

    return 0;
}

void* V8InterceptSetCallHandler(void* esp_frame) {
    if (!esp_frame) return nullptr;
    /* 简化实现：直接返回原始 handler */
    void** args = (void**)esp_frame;
    return args[2];  /* 第三个参数是 handler 指针 */
}

int V8InterceptScriptRun(void* esp_frame) {
    (void)esp_frame;
    return 0;  /* 默认允许执行 */
}

/* ============================================================================
 * Bridge IPC 加密实现
 *
 * 协议格式:
 *   [4 bytes: payload length (big-endian)]
 *   [12 bytes: AES-GCM nonce]
 *   [N bytes: AES-GCM encrypted JSON]
 *   [16 bytes: GCM authentication tag]
 * ========================================================================= */

#define AES_GCM_NONCE_SIZE  12
#define AES_GCM_TAG_SIZE    16
#define HEADER_SIZE         4

int BridgeEncrypt(
    const unsigned char* plaintext, size_t plainLen,
    const unsigned char* key,
    unsigned char* outBuf, size_t outBufSize, size_t* outLen)
{
    /* TODO: 实现 AES-256-GCM 加密
     *
     * 步骤:
     * 1. 验证输入参数（plaintext, key, outBuf 均不为 NULL）
     * 2. 计算输出长度: HEADER_SIZE + AES_GCM_NONCE_SIZE + plainLen + AES_GCM_TAG_SIZE
     * 3. 检查 outBufSize 是否足够
     * 4. 生成随机 12 字节 nonce
     * 5. 写入 4 字节大端长度头（payload length = plainLen）
     * 6. 写入 nonce
     * 7. 执行 AES-256-GCM 加密
     * 8. 追加 16 字节 authentication tag
     * 9. 设置 *outLen 为实际写入字节数
     *
     * 提示: 可使用 platform_compat.h 中的 pal_aes_gcm_encrypt()
     *
     * 错误码:
     *   -1 = 无效参数
     *   -2 = 输出缓冲区太小
     *   -3 = 加密失败
     */

    (void)plaintext; (void)plainLen; (void)key;
    (void)outBuf; (void)outBufSize; (void)outLen;
    return -99;  /* NOT IMPLEMENTED */
}

int BridgeDecrypt(
    const unsigned char* ciphertext, size_t cipherLen,
    const unsigned char* key,
    unsigned char* outBuf, size_t outBufSize, size_t* outLen)
{
    /* TODO: 实现 AES-256-GCM 解密
     *
     * 步骤:
     * 1. 验证输入参数
     * 2. 验证 cipherLen >= HEADER_SIZE + AES_GCM_NONCE_SIZE + AES_GCM_TAG_SIZE
     * 3. 读取 4 字节大端长度头获取 payload length
     * 4. 验证 cipherLen 与声明的 payload length 一致
     * 5. 提取 nonce (12 字节)
     * 6. 提取密文和 tag
     * 7. 执行 AES-256-GCM 解密并验证 tag
     * 8. 将明文写入 outBuf，设置 *outLen
     *
     * 错误码:
     *   -1 = 无效参数
     *   -2 = 密文太短
     *   -3 = 长度头不匹配
     *   -4 = 解密失败（认证失败）
     *   -5 = 输出缓冲区太小
     */

    (void)ciphertext; (void)cipherLen; (void)key;
    (void)outBuf; (void)outBufSize; (void)outLen;
    return -99;  /* NOT IMPLEMENTED */
}
