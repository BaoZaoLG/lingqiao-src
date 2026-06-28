#pragma once

#include <windows.h>

#include <cstddef>
#include <string>
#include <vector>

struct PatternBytes {
    std::vector<unsigned char> bytes;
    std::vector<unsigned char> mask;  // 1 = fixed byte, 0 = wildcard
};

struct PatternMatch {
    const unsigned char* address;
    DWORD rva;
};

bool ParseIdaPattern(const char* pattern, PatternBytes* out);
bool MatchPatternAt(const unsigned char* data, size_t dataSize, const PatternBytes& pattern);
std::vector<PatternMatch> FindPatternInModule(HMODULE module, const PatternBytes& pattern, size_t maxMatches);
std::string FormatAddress(const void* address);
