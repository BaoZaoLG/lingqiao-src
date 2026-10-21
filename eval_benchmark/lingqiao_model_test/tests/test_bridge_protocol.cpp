/* test_bridge_protocol.cpp - Bridge IPC encrypt/decrypt tests */

#include "../starter/c_hook/v8_intercept.h"
#include "../starter/c_hook/bridge_protocol.h"
#include <cassert>
#include <cstdio>
#include <cstring>

static int g_passed = 0;
static int g_failed = 0;

#define TEST(name) void name()
#define RUN(name) do { \
    printf("  [TEST] %s ... ", #name); \
    name(); \
    printf("PASSED\n"); \
    g_passed++; \
} while(0)

#define ASSERT(cond) do { \
    if (!(cond)) { \
        printf("FAILED at %s:%d: %s\n", __FILE__, __LINE__, #cond); \
        g_failed++; \
        return; \
    } \
} while(0)

static const unsigned char TEST_KEY[32] = {
    0x00,0x01,0x02,0x03,0x04,0x05,0x06,0x07,
    0x08,0x09,0x0A,0x0B,0x0C,0x0D,0x0E,0x0F,
    0x10,0x11,0x12,0x13,0x14,0x15,0x16,0x17,
    0x18,0x19,0x1A,0x1B,0x1C,0x1D,0x1E,0x1F
};

TEST(test_encrypt_null_params) {
    unsigned char buf[1024];
    size_t outLen = 0;
    const char* msg = "test";
    ASSERT(BridgeEncrypt(nullptr, 4, TEST_KEY, buf, sizeof(buf), &outLen) == -1);
    ASSERT(BridgeEncrypt((const unsigned char*)msg, 4, nullptr, buf, sizeof(buf), &outLen) == -1);
    ASSERT(BridgeEncrypt((const unsigned char*)msg, 4, TEST_KEY, nullptr, sizeof(buf), &outLen) == -1);
    ASSERT(BridgeEncrypt((const unsigned char*)msg, 4, TEST_KEY, buf, sizeof(buf), nullptr) == -1);
}

TEST(test_encrypt_buffer_too_small) {
    unsigned char buf[8];
    size_t outLen = 0;
    const char* msg = "hello world this is a long message";
    int result = BridgeEncrypt((const unsigned char*)msg, strlen(msg), TEST_KEY, buf, sizeof(buf), &outLen);
    ASSERT(result == -2);
}

TEST(test_decrypt_null_params) {
    unsigned char buf[1024];
    size_t outLen = 0;
    unsigned char data[64] = {0};
    ASSERT(BridgeDecrypt(nullptr, 64, TEST_KEY, buf, sizeof(buf), &outLen) == -1);
    ASSERT(BridgeDecrypt(data, 64, nullptr, buf, sizeof(buf), &outLen) == -1);
    ASSERT(BridgeDecrypt(data, 64, TEST_KEY, nullptr, sizeof(buf), &outLen) == -1);
    ASSERT(BridgeDecrypt(data, 64, TEST_KEY, buf, sizeof(buf), nullptr) == -1);
}

TEST(test_decrypt_too_short) {
    unsigned char data[10] = {0};
    unsigned char buf[1024];
    size_t outLen = 0;
    int result = BridgeDecrypt(data, sizeof(data), TEST_KEY, buf, sizeof(buf), &outLen);
    ASSERT(result == -2);
}

TEST(test_decrypt_length_mismatch) {
    // header says 100 bytes payload but data is much shorter
    unsigned char data[40] = {0};
    data[0] = 0; data[1] = 0; data[2] = 0; data[3] = 100; // payload = 100
    unsigned char buf[1024];
    size_t outLen = 0;
    int result = BridgeDecrypt(data, sizeof(data), TEST_KEY, buf, sizeof(buf), &outLen);
    ASSERT(result == -3);
}

TEST(test_encrypt_decrypt_roundtrip) {
    const char* message = "{\"version\":2,\"type\":\"request\",\"id\":\"abc123\",\"method\":\"runtime.ping\"}";
    size_t msgLen = strlen(message);

    unsigned char encrypted[1024];
    size_t encLen = 0;
    int r1 = BridgeEncrypt((const unsigned char*)message, msgLen, TEST_KEY, encrypted, sizeof(encrypted), &encLen);
    ASSERT(r1 == 0);
    ASSERT(encLen == 4 + 12 + msgLen + 16);

    unsigned char decrypted[1024];
    size_t decLen = 0;
    int r2 = BridgeDecrypt(encrypted, encLen, TEST_KEY, decrypted, sizeof(decrypted), &decLen);
    ASSERT(r2 == 0);
    ASSERT(decLen == msgLen);
    ASSERT(memcmp(decrypted, message, msgLen) == 0);
}

TEST(test_encrypt_decrypt_empty_like) {
    const char* message = "x";
    unsigned char encrypted[1024];
    size_t encLen = 0;
    int r1 = BridgeEncrypt((const unsigned char*)message, 1, TEST_KEY, encrypted, sizeof(encrypted), &encLen);
    ASSERT(r1 == 0);

    unsigned char decrypted[1024];
    size_t decLen = 0;
    int r2 = BridgeDecrypt(encrypted, encLen, TEST_KEY, decrypted, sizeof(decrypted), &decLen);
    ASSERT(r2 == 0);
    ASSERT(decLen == 1);
    ASSERT(decrypted[0] == 'x');
}

TEST(test_tampered_ciphertext) {
    const char* message = "secret data";
    unsigned char encrypted[1024];
    size_t encLen = 0;
    BridgeEncrypt((const unsigned char*)message, strlen(message), TEST_KEY, encrypted, sizeof(encrypted), &encLen);

    // Tamper with ciphertext
    encrypted[20] ^= 0xFF;

    unsigned char decrypted[1024];
    size_t decLen = 0;
    int result = BridgeDecrypt(encrypted, encLen, TEST_KEY, decrypted, sizeof(decrypted), &decLen);
    ASSERT(result == -4); // authentication failure
}

TEST(test_wrong_key_decrypt) {
    const char* message = "hello";
    unsigned char encrypted[1024];
    size_t encLen = 0;
    BridgeEncrypt((const unsigned char*)message, strlen(message), TEST_KEY, encrypted, sizeof(encrypted), &encLen);

    unsigned char wrongKey[32];
    memset(wrongKey, 0xFF, 32);

    unsigned char decrypted[1024];
    size_t decLen = 0;
    int result = BridgeDecrypt(encrypted, encLen, wrongKey, decrypted, sizeof(decrypted), &decLen);
    ASSERT(result == -4); // authentication failure with wrong key
}

int main() {
    printf("[SUITE] test_bridge_protocol\n");

    RUN(test_encrypt_null_params);
    RUN(test_encrypt_buffer_too_small);
    RUN(test_decrypt_null_params);
    RUN(test_decrypt_too_short);
    RUN(test_decrypt_length_mismatch);
    RUN(test_encrypt_decrypt_roundtrip);
    RUN(test_encrypt_decrypt_empty_like);
    RUN(test_tampered_ciphertext);
    RUN(test_wrong_key_decrypt);

    printf("\n[RESULT] %d passed, %d failed\n", g_passed, g_failed);
    return g_failed > 0 ? 1 : 0;
}
