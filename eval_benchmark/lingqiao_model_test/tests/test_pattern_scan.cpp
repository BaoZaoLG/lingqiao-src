/* ============================================================================
 * test_pattern_scan.cpp — 特征码扫描模块单元测试
 * ========================================================================= */

#include "../starter/c_hook/pattern_scan.h"
#include <cassert>
#include <cstdio>
#include <cstring>
#include <vector>

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

// ============================================================================
// 测试用例
// ============================================================================

TEST(test_parse_basic_pattern) {
    PatternBytes pb;
    bool ok = ParseIdaPattern("48 8B 05 ?? ?? ?? ?? 55", &pb);
    ASSERT(ok);
    ASSERT(pb.bytes.size() == 8);
    ASSERT(pb.mask[0] == 1);  // 48
    ASSERT(pb.mask[3] == 0);  // ??
    ASSERT(pb.bytes[0] == 0x48);
    ASSERT(pb.bytes[1] == 0x8B);
}

TEST(test_parse_null_pattern) {
    PatternBytes pb;
    ASSERT(!ParseIdaPattern(nullptr, &pb));
    ASSERT(!ParseIdaPattern("48 8B", nullptr));
}

TEST(test_parse_invalid_hex) {
    PatternBytes pb;
    ASSERT(!ParseIdaPattern("GG HH", &pb));
    ASSERT(!ParseIdaPattern("123", &pb));
}

TEST(test_parse_overlong_pattern) {
    // 超过 1024 字符的模式串应该被拒绝
    std::string longPattern;
    for (int i = 0; i < 400; i++) {
        longPattern += "48 ";
    }
    // 至少 1200 字符
    PatternBytes pb;
    bool ok = ParseIdaPattern(longPattern.c_str(), &pb);
    // 修复后应返回 false（超长模式被拒绝）
    ASSERT(!ok);
}

TEST(test_match_null_data) {
    PatternBytes pb;
    ParseIdaPattern("48 8B", &pb);
    // 修复后: data 为 NULL 时应返回 false 而非崩溃
    bool result = MatchPatternAt(nullptr, 100, pb);
    ASSERT(!result);
}

TEST(test_match_basic) {
    unsigned char data[] = {0x48, 0x8B, 0x05, 0xAA, 0xBB};
    PatternBytes pb;
    ParseIdaPattern("48 8B 05 ?? ??", &pb);
    ASSERT(MatchPatternAt(data, sizeof(data), pb));
}

TEST(test_match_insufficient_data) {
    unsigned char data[] = {0x48, 0x8B};
    PatternBytes pb;
    ParseIdaPattern("48 8B 05 ??", &pb);
    ASSERT(!MatchPatternAt(data, sizeof(data), pb));
}

TEST(test_find_in_module_null) {
    PatternBytes pb;
    ParseIdaPattern("48 8B", &pb);
    auto matches = FindPatternInModule(nullptr, 100, pb, 10);
    ASSERT(matches.empty());
}

TEST(test_find_in_module_size_less_than_pattern) {
    // 修复后: moduleSize < pattern.size() 不应导致整数下溢崩溃
    unsigned char data[] = {0x48};
    PatternBytes pb;
    ParseIdaPattern("48 8B 05 ?? ??", &pb);
    auto matches = FindPatternInModule(data, 1, pb, 10);
    ASSERT(matches.empty());
}

TEST(test_find_in_module_basic) {
    unsigned char module[] = {
        0x00, 0x00, 0x48, 0x8B, 0x05, 0xCC, 0xDD, 0x00,
        0x48, 0x8B, 0x05, 0xEE, 0xFF, 0x00, 0x00, 0x00
    };
    PatternBytes pb;
    ParseIdaPattern("48 8B 05 ?? ??", &pb);
    auto matches = FindPatternInModule(module, sizeof(module), pb, 10);
    ASSERT(matches.size() == 2);
    ASSERT(matches[0].rva == 2);
    ASSERT(matches[1].rva == 8);
}

TEST(test_find_in_module_max_matches) {
    unsigned char module[] = {
        0xAA, 0xBB, 0xAA, 0xBB, 0xAA, 0xBB, 0xAA, 0xBB
    };
    PatternBytes pb;
    ParseIdaPattern("AA BB", &pb);
    auto matches = FindPatternInModule(module, sizeof(module), pb, 2);
    ASSERT(matches.size() == 2);
}

// ============================================================================
// 主函数
// ============================================================================

int main() {
    printf("[SUITE] test_pattern_scan\n");

    RUN(test_parse_basic_pattern);
    RUN(test_parse_null_pattern);
    RUN(test_parse_invalid_hex);
    RUN(test_parse_overlong_pattern);
    RUN(test_match_null_data);
    RUN(test_match_basic);
    RUN(test_match_insufficient_data);
    RUN(test_find_in_module_null);
    RUN(test_find_in_module_size_less_than_pattern);
    RUN(test_find_in_module_basic);

    printf("\n[RESULT] %d passed, %d failed\n", g_passed, g_failed);
    return g_failed > 0 ? 1 : 0;
}
