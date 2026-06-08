#include "pattern_scan.h"

#include <cstdio>
#include <cstdlib>
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
    if (!data || pattern.bytes.empty() || pattern.bytes.size() != pattern.mask.size()) {
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

static bool IsExecutableSection(DWORD characteristics) {
    return (characteristics & IMAGE_SCN_MEM_EXECUTE) != 0;
}

std::vector<PatternMatch> FindPatternInModule(HMODULE module, const PatternBytes& pattern, size_t maxMatches) {
    std::vector<PatternMatch> matches;
    if (!module || pattern.bytes.empty() || pattern.bytes.size() != pattern.mask.size() || maxMatches == 0) {
        return matches;
    }

    const auto* base = reinterpret_cast<const unsigned char*>(module);
    const auto* dos = reinterpret_cast<const IMAGE_DOS_HEADER*>(base);
    if (dos->e_magic != IMAGE_DOS_SIGNATURE) {
        return matches;
    }

    const auto* nt = reinterpret_cast<const IMAGE_NT_HEADERS*>(base + dos->e_lfanew);
    if (nt->Signature != IMAGE_NT_SIGNATURE) {
        return matches;
    }

    const IMAGE_SECTION_HEADER* section = IMAGE_FIRST_SECTION(nt);
    for (WORD i = 0; i < nt->FileHeader.NumberOfSections; ++i, ++section) {
        if (!IsExecutableSection(section->Characteristics)) {
            continue;
        }

        const DWORD rva = section->VirtualAddress;
        const DWORD size = section->Misc.VirtualSize ? section->Misc.VirtualSize : section->SizeOfRawData;
        if (size < pattern.bytes.size()) {
            continue;
        }

        const unsigned char* begin = base + rva;
        const size_t limit = static_cast<size_t>(size) - pattern.bytes.size();
        for (size_t offset = 0; offset <= limit; ++offset) {
            if (MatchPatternAt(begin + offset, pattern.bytes.size(), pattern)) {
                matches.push_back(PatternMatch{begin + offset, rva + static_cast<DWORD>(offset)});
                if (matches.size() >= maxMatches) {
                    return matches;
                }
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
