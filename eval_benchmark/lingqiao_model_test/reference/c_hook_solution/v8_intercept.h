#pragma once
/* ============================================================================
 * v8_intercept.h — V8 拦截引擎 API
 *
 * 将被动的 V8 观测 Hook 转变为主动拦截:
 *   1. 脚本拦截 — 在 V8 编译前读取/修改 JS 源码
 *   2. Handler 劫持 — 替换注册的 Native 函数处理器
 *   3. 执行控制 — 按脚本实施断点、跳过、重放
 *
 * 所有入口点为 extern "C" __cdecl，可从底层 detour 调用。
 * ========================================================================= */

#include "platform_compat.h"

#ifdef __cplusplus
extern "C" {
#endif

/* ---- 1. 脚本拦截 (compile_script hook) ----
 * 在 V8 编译脚本前被调用。
 * 返回:  0 = 放行（可能使用修改后的源码）
 *         1 = 阻止脚本编译 */
int V8InterceptCompileScript(void* esp_frame);

/* ---- 2. Handler 劫持 (set_call_handler hook) ----
 * 在应用注册 Native 函数 handler 时被调用。
 * 返回要安装的回调指针（原始或替换的）。 */
void* V8InterceptSetCallHandler(void* esp_frame);

/* ---- 3. 执行控制 (script_run hook) ----
 * 在已编译的脚本执行前被调用。
 * 返回:  0 = 允许执行
 *         1 = 跳过执行 */
int V8InterceptScriptRun(void* esp_frame);

/* ---- 初始化 / 关闭 ---- */
void V8InterceptInit(void);
void V8InterceptShutdown(void);

/* ============================================================================
 * Bridge IPC 加密通信
 * ========================================================================= */

/* 加密一条 Bridge 消息
 * @param plaintext  明文 JSON 字节
 * @param plainLen   明文长度
 * @param key        256-bit AES 密钥 (32 字节)
 * @param outBuf     输出缓冲区（调用者分配）
 * @param outBufSize 输出缓冲区大小
 * @param outLen     实际写入的字节数
 * @return 0 成功, 非零 错误码 */
int BridgeEncrypt(
    const unsigned char* plaintext, size_t plainLen,
    const unsigned char* key,
    unsigned char* outBuf, size_t outBufSize, size_t* outLen);

/* 解密一条 Bridge 消息
 * @param ciphertext 密文数据（包含 nonce + 密文 + tag）
 * @param cipherLen  密文总长度
 * @param key        256-bit AES 密钥 (32 字节)
 * @param outBuf     输出缓冲区（调用者分配）
 * @param outBufSize 输出缓冲区大小
 * @param outLen     实际写入的明文字节数
 * @return 0 成功, 非零 错误码 */
int BridgeDecrypt(
    const unsigned char* ciphertext, size_t cipherLen,
    const unsigned char* key,
    unsigned char* outBuf, size_t outBufSize, size_t* outLen);

#ifdef __cplusplus
}
#endif
