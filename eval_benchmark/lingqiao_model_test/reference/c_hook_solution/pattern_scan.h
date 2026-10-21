#pragma once
/* ============================================================================
 * pattern_scan.h — 模式扫描引擎 API
 *
 * 提供 IDA 风格的特征码解析和模块内存搜索能力。
 * ========================================================================= */

#include "platform_compat.h"

#include <cstddef>
#include <string>
#include <vector>

struct PatternBytes {
    std::vector<unsigned char> bytes;
    std::vector<unsigned char> mask;  // 1 = fixed byte, 0 = wildcard
};

struct PatternMatch {
    const unsigned char* address;
    uint32_t rva;
};

/// 解析 IDA 风格的特征码字符串 (如 "48 8B ?? 55 ?? 89")
/// @return true 解析成功, false 格式错误
bool ParseIdaPattern(const char* pattern, PatternBytes* out);

/// 在给定数据块中匹配模式
/// @param data 数据起始地址
/// @param dataSize 数据块大小
/// @param pattern 解析后的模式
/// @return true 匹配成功
bool MatchPatternAt(const unsigned char* data, size_t dataSize, const PatternBytes& pattern);

/// 在指定模块的可执行节区中搜索特征码
/// @param moduleBase 模块基址
/// @param moduleSize 模块大小（字节）
/// @param pattern 解析后的模式
/// @param maxMatches 最大匹配数
/// @return 所有匹配位置的列表
std::vector<PatternMatch> FindPatternInModule(
    const unsigned char* moduleBase,
    size_t moduleSize,
    const PatternBytes& pattern,
    size_t maxMatches);

/// 格式化地址为十六进制字符串
std::string FormatAddress(const void* address);
