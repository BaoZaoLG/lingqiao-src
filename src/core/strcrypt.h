#pragma once
// ============================================================================
// Compile-time string encryption — XOR-based, per-string random key
// Strings are encrypted at compile time, decrypted at runtime on the stack.
// Uses __TIME__ seed so recompilation changes all keys.
// ============================================================================
#include <windows.h>
#include <mutex>

namespace strcrypt {

// Compile-time PRNG seeded by __TIME__
constexpr unsigned int g_seed = __TIME__[0] * 3600 + __TIME__[1] * 360
                              + __TIME__[3] * 60  + __TIME__[4] * 10
                              + __TIME__[6] * 10  + __TIME__[7];

constexpr unsigned int Rand(unsigned int seed) {
    seed ^= seed << 13;
    seed ^= seed >> 17;
    seed ^= seed << 5;
    return seed;
}

template <typename CharT, int N, unsigned int Key>
class EncryptedString {
    CharT m_data[N];
public:
    constexpr EncryptedString(const CharT (&str)[N]) : m_data{} {
        constexpr unsigned int k = Rand(Key);
        for (int i = 0; i < N; i++) {
            m_data[i] = static_cast<CharT>(
                str[i] ^ static_cast<CharT>((k >> ((i & 3) * 8)) & 0xFF)
                        ^ static_cast<CharT>((i * 0x9D) & 0xFF));
        }
    }

    __declspec(noinline) void decrypt(CharT* out) const {
        unsigned int k = Rand(Key);
        for (int i = 0; i < N; i++) {
            out[i] = static_cast<CharT>(
                m_data[i] ^ static_cast<CharT>((k >> ((i & 3) * 8)) & 0xFF)
                          ^ static_cast<CharT>((i * 0x9D) & 0xFF));
        }
    }
};

// RAII wrapper for char strings
template <int N, unsigned int Key>
struct SecureStr {
    char buf[N];
    __declspec(noinline) SecureStr(const EncryptedString<char, N, Key>& enc) {
        enc.decrypt(buf);
    }
    __declspec(noinline) ~SecureStr() {
        SecureZeroMemory(buf, N);
    }
    const char* c_str() const { return buf; }
    operator const char*() const { return buf; }
};

// RAII wrapper for wchar_t strings
template <int N, unsigned int Key>
struct SecureWStr {
    wchar_t buf[N];
    __declspec(noinline) SecureWStr(const EncryptedString<wchar_t, N, Key>& enc) {
        enc.decrypt(buf);
    }
    __declspec(noinline) ~SecureWStr() {
        SecureZeroMemory(buf, N * sizeof(wchar_t));
    }
    const wchar_t* c_str() const { return buf; }
    operator const wchar_t*() const { return buf; }
};

} // namespace strcrypt

// ============================================================================
// Macros — auto-generate unique key from __LINE__ + __COUNTER__
// Usage:
//   auto s = _S("hello");        // persistent static buffer
//   auto w = _WS(L"hello");      // persistent static buffer
//   const char* p = LQ_SP("hello"); // persistent static buffer
//   const wchar_t* wp = _WSP(L"hello"); // persistent static buffer
// ============================================================================

// char string — persistent static buffer, cleared on process teardown
#define _S(str) ([&]() -> const char* { \
    constexpr auto _enc = strcrypt::EncryptedString<char, sizeof(str), \
        strcrypt::g_seed ^ __LINE__ ^ (__COUNTER__ * 0x1337CAFE)>(str); \
    static strcrypt::SecureStr<sizeof(str), \
        strcrypt::g_seed ^ __LINE__ ^ ((__COUNTER__ - 1) * 0x1337CAFE)> _dec(_enc); \
    return _dec.c_str(); \
}())

// wchar_t string — persistent static buffer, cleared on process teardown
#define _WS(str) ([&]() -> const wchar_t* { \
    constexpr auto _enc = strcrypt::EncryptedString<wchar_t, sizeof(str)/sizeof(wchar_t), \
        strcrypt::g_seed ^ __LINE__ ^ (__COUNTER__ * 0x1337CAFE)>(str); \
    static strcrypt::SecureWStr<sizeof(str)/sizeof(wchar_t), \
        strcrypt::g_seed ^ __LINE__ ^ ((__COUNTER__ - 1) * 0x1337CAFE)> _dec(_enc); \
    return _dec.c_str(); \
}())

// Persistent char* (for static/const assignments — no auto-cleanup)
#define LQ_SP(str) ([&]() -> const char* { \
    constexpr auto _enc = strcrypt::EncryptedString<char, sizeof(str), \
        strcrypt::g_seed ^ __LINE__ ^ (__COUNTER__ * 0x1337CAFE)>(str); \
    static char _buf[sizeof(str)]; \
    static std::once_flag _once; \
    std::call_once(_once, [&]() { _enc.decrypt(_buf); }); \
    return _buf; \
}())

// Persistent wchar_t*
#define _WSP(str) ([&]() -> const wchar_t* { \
    constexpr auto _enc = strcrypt::EncryptedString<wchar_t, sizeof(str)/sizeof(wchar_t), \
        strcrypt::g_seed ^ __LINE__ ^ (__COUNTER__ * 0x1337CAFE)>(str); \
    static wchar_t _buf[sizeof(str)/sizeof(wchar_t)]; \
    static std::once_flag _once; \
    std::call_once(_once, [&]() { _enc.decrypt(_buf); }); \
    return _buf; \
}())

// ============================================================================
// Code bloat padding — inflate binary to 5-10 MB range
// Inserts dummy code blocks that are never executed but defeat static analysis
// ============================================================================
#define _OBFUSCATE_PAD() do { \
    volatile int _pad = __LINE__; \
    if (_pad == 0xDEAD) { \
        char _junk[4096]; \
        for (int _i = 0; _i < 4096; _i++) _junk[_i] = (char)(_i * 0x5A); \
        volatile char _x = _junk[0]; (void)_x; \
    } \
} while(0)
