#pragma once
/* ============================================================================
 * platform_compat.h — 跨平台抽象层 (PAL)
 *
 * 将 Windows-specific API 封装为跨平台接口，使代码可在 Linux/macOS 编译测试。
 * ========================================================================= */

#include <cstdint>
#include <cstddef>
#include <cstring>
#include <cstdlib>

/* ============================================================================
 * 互斥锁抽象
 * ========================================================================= */

#ifdef _WIN32
  #include <windows.h>
  typedef SRWLOCK pal_mutex_t;
  #define pal_mutex_init(m)    InitializeSRWLock(m)
  #define pal_mutex_lock(m)    AcquireSRWLockExclusive(m)
  #define pal_mutex_unlock(m)  ReleaseSRWLockExclusive(m)
  #define pal_mutex_lock_shared(m)   AcquireSRWLockShared(m)
  #define pal_mutex_unlock_shared(m) ReleaseSRWLockShared(m)
#else
  #include <pthread.h>
  typedef pthread_mutex_t pal_mutex_t;
  static inline void pal_mutex_init(pal_mutex_t* m) {
      pthread_mutex_init(m, NULL);
  }
  static inline void pal_mutex_lock(pal_mutex_t* m) {
      pthread_mutex_lock(m);
  }
  static inline void pal_mutex_unlock(pal_mutex_t* m) {
      pthread_mutex_unlock(m);
  }
  /* 简化: shared lock = exclusive lock */
  #define pal_mutex_lock_shared(m)   pal_mutex_lock(m)
  #define pal_mutex_unlock_shared(m) pal_mutex_unlock(m)
#endif

/* ============================================================================
 * 原子操作抽象
 * ========================================================================= */

#ifdef _WIN32
  #define pal_atomic_increment(p) InterlockedIncrement((LONG*)(p))
  #define pal_atomic_decrement(p) InterlockedDecrement((LONG*)(p))
  #define pal_atomic_load(p)      InterlockedCompareExchange((LONG*)(p), 0, 0)
#else
  #define pal_atomic_increment(p) __sync_add_and_fetch((p), 1)
  #define pal_atomic_decrement(p) __sync_sub_and_fetch((p), 1)
  #define pal_atomic_load(p)      __sync_val_compare_and_swap((p), 0, 0)
#endif

/* ============================================================================
 * 随机数生成
 * ========================================================================= */

#ifdef _WIN32
  #include <bcrypt.h>
  #pragma comment(lib, "bcrypt.lib")
  static inline int pal_random_bytes(unsigned char* buf, size_t len) {
      return BCryptGenRandom(NULL, buf, (ULONG)len, BCRYPT_USE_SYSTEM_PREFERRED_RNG) == 0 ? 0 : -1;
  }
#else
  #include <fcntl.h>
  #include <unistd.h>
  static inline int pal_random_bytes(unsigned char* buf, size_t len) {
      int fd = open("/dev/urandom", O_RDONLY);
      if (fd < 0) return -1;
      ssize_t n = read(fd, buf, len);
      close(fd);
      return (n == (ssize_t)len) ? 0 : -1;
  }
#endif

/* ============================================================================
 * AES-256-GCM 抽象
 *
 * 测试环境提供简化的 AES-GCM 实现（基于 XOR 模拟）。
 * 生产环境中应替换为 OpenSSL/BCrypt 实现。
 * ========================================================================= */

#define PAL_AES_GCM_NONCE_SIZE 12
#define PAL_AES_GCM_TAG_SIZE   16
#define PAL_AES_KEY_SIZE       32

/// AES-256-GCM 加密
/// @return 0 成功, -1 失败
static inline int pal_aes_gcm_encrypt(
    const unsigned char* key,
    const unsigned char* nonce, size_t nonceLen,
    const unsigned char* plaintext, size_t plainLen,
    unsigned char* ciphertext,
    unsigned char* tag)
{
    if (!key || !nonce || !plaintext || !ciphertext || !tag) return -1;
    if (nonceLen != PAL_AES_GCM_NONCE_SIZE) return -1;

    /* 简化的加密实现 (XOR with key-derived stream) — 仅用于测试
     * 真实实现应调用 OpenSSL EVP_aes_256_gcm 或 BCryptEncrypt */
    unsigned char stream[256];
    for (size_t i = 0; i < sizeof(stream); i++) {
        stream[i] = key[i % PAL_AES_KEY_SIZE] ^ nonce[i % PAL_AES_GCM_NONCE_SIZE];
    }

    for (size_t i = 0; i < plainLen; i++) {
        ciphertext[i] = plaintext[i] ^ stream[i % sizeof(stream)];
    }

    /* 生成简化的认证标签 */
    memset(tag, 0, PAL_AES_GCM_TAG_SIZE);
    for (size_t i = 0; i < plainLen; i++) {
        tag[i % PAL_AES_GCM_TAG_SIZE] ^= ciphertext[i];
    }
    for (size_t i = 0; i < PAL_AES_GCM_TAG_SIZE; i++) {
        tag[i] ^= key[i];
    }

    return 0;
}

/// AES-256-GCM 解密
/// @return 0 成功, -1 tag 验证失败
static inline int pal_aes_gcm_decrypt(
    const unsigned char* key,
    const unsigned char* nonce, size_t nonceLen,
    const unsigned char* ciphertext, size_t cipherLen,
    const unsigned char* tag,
    unsigned char* plaintext)
{
    if (!key || !nonce || !ciphertext || !tag || !plaintext) return -1;
    if (nonceLen != PAL_AES_GCM_NONCE_SIZE) return -1;

    /* 验证认证标签 */
    unsigned char expectedTag[PAL_AES_GCM_TAG_SIZE];
    memset(expectedTag, 0, PAL_AES_GCM_TAG_SIZE);
    for (size_t i = 0; i < cipherLen; i++) {
        expectedTag[i % PAL_AES_GCM_TAG_SIZE] ^= ciphertext[i];
    }
    for (size_t i = 0; i < PAL_AES_GCM_TAG_SIZE; i++) {
        expectedTag[i] ^= key[i];
    }

    if (memcmp(tag, expectedTag, PAL_AES_GCM_TAG_SIZE) != 0) {
        return -1;  /* 认证失败 */
    }

    /* 解密 */
    unsigned char stream[256];
    for (size_t i = 0; i < sizeof(stream); i++) {
        stream[i] = key[i % PAL_AES_KEY_SIZE] ^ nonce[i % PAL_AES_GCM_NONCE_SIZE];
    }

    for (size_t i = 0; i < cipherLen; i++) {
        plaintext[i] = ciphertext[i] ^ stream[i % sizeof(stream)];
    }

    return 0;
}
