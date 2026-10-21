#include "pattern_scan.h"

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <sstream>

static bool IsHexByte(const std::string& token) {
    if (token.size() != 2) return false;
    for (char c : token) {
        const bool digit = c >= '0' && c <= '9';
        const bool upper = c >= 'A' && c <= 'F';
        const bool lower = c >= 'a' && c <= 'f';
        if (!digit && !upper && !lower) return false;
    }
    return true;
}

bool ParseIdaPattern(const char* pattern, PatternBytes* out) {
    // BUG: 缺少对 pattern 长度的限制，超长模式串可能导致内存耗尽
    // TODO: 添加 pattern 长度校验（上限 1024 个字符）
    if (!pattern || !out) return false;

    PatternBytes parsed;
    std::istringstream stream(pattern);
    std::string token;
    while (stream >> token) {
        if (token == "?" || token == "??") {
            parsed.bytes.push_back(0);
            parsed.mask.push_back(0);
            continue;
        }

        if (!IsHexByte(token)) {
            return false;
        }

        char* end = nullptr;
        unsigned long value = std::strtoul(token.c_str(), &end, 16);
        if (!end || *end != '\0' || value > 0xFF) {
            return false;
        }
        parsed.bytes.push_back(static_cast<unsigned char>(value));
        parsed.mask.push_back(1);
    }

    if (parsed.bytes.empty()) return false;
    *out = parsed;
    return true;
}

bool MatchPatternAt(const unsigned char* data, size_t dataSize, const PatternBytes& pattern) {
    // BUG: 空指针检查被注释掉了，当 data 为 NULL 时直接解引用会崩溃
    // if (!data) return false;

    if (pattern.bytes.empty() || pattern.bytes.size() != pattern.mask.size()) {
        return false;
    }
    if (dataSize < pattern.bytes.size()) {
        return false;
    }

    for (size_t i = 0; i < pattern.bytes.size(); ++i) {
        if (pattern.mask[i] && data[i] != pattern.bytes[i]) {
            return false;
        }
    }
    return true;
}

/// PE 节区头结构（跨平台模拟）
struct MockSectionHeader {
    uint32_t virtualAddress;
    uint32_t virtualSize;
    uint32_t characteristics;
};

#define MOCK_SCN_MEM_EXECUTE 0x20000000

std::vector<PatternMatch> FindPatternInModule(
    const unsigned char* moduleBase,
    size_t moduleSize,
    const PatternBytes& pattern,
    size_t maxMatches)
{
    std::vector<PatternMatch> matches;
    if (!moduleBase || pattern.bytes.empty() || pattern.bytes.size() != pattern.mask.size() || maxMatches == 0) {
        return matches;
    }

    // BUG: 没有校验 moduleSize 是否比 pattern 长，当 moduleSize < pattern.size() 时
    //       下面的 limit 计算会产生整数下溢（size_t 是无符号的），导致循环越界读取
    // TODO: 在计算 limit 之前添加 moduleSize >= pattern.bytes.size() 的检查

    const size_t limit = moduleSize - pattern.bytes.size();
    for (size_t offset = 0; offset <= limit; ++offset) {
        if (MatchPatternAt(moduleBase + offset, pattern.bytes.size(), pattern)) {
            PatternMatch m;
            m.address = moduleBase + offset;
            m.rva = static_cast<uint32_t>(offset);
            matches.push_back(m);
            if (matches.size() >= maxMatches) {
                return matches;
            }
        }
    }

    return matches;
}

std::string FormatAddress(const void* address) {
    char buf[32] = {0};
    std::snprintf(buf, sizeof(buf), "0x%p", address);
    return std::string(buf);
}
