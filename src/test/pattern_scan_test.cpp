#include "pattern_scan.h"

#include <cstdio>
#include <vector>

static int Fail(const char* msg) {
    std::printf("FAIL: %s\n", msg);
    return 1;
}

int main() {
    PatternBytes pat;
    if (!ParseIdaPattern("55 89 E5 ? ? A1", &pat)) {
        return Fail("ParseIdaPattern rejected a valid pattern");
    }
    if (pat.bytes.size() != 6 || pat.mask.size() != 6) {
        return Fail("parsed pattern length is wrong");
    }
    if (!pat.mask[0] || !pat.mask[1] || !pat.mask[2] || pat.mask[3] || pat.mask[4] || !pat.mask[5]) {
        return Fail("wildcard mask is wrong");
    }

    const unsigned char ok[] = {0x55, 0x89, 0xE5, 0x11, 0x22, 0xA1};
    if (!MatchPatternAt(ok, sizeof(ok), pat)) {
        return Fail("MatchPatternAt did not honor wildcards");
    }

    const unsigned char bad[] = {0x55, 0x89, 0xE4, 0x11, 0x22, 0xA1};
    if (MatchPatternAt(bad, sizeof(bad), pat)) {
        return Fail("MatchPatternAt ignored a fixed-byte mismatch");
    }

    PatternBytes invalid;
    if (ParseIdaPattern("55 ZZ", &invalid)) {
        return Fail("ParseIdaPattern accepted invalid hex");
    }

    std::printf("PASS: pattern scanner tests\n");
    return 0;
}
