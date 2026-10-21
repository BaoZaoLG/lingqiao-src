#include "hook/v8_raw_source_probe.h"

#include <cstdio>

static int Expect(bool condition, const char* message) {
    if (!condition) {
        std::printf("FAIL %s\n", message);
        return 1;
    }
    return 0;
}

int main() {
    unsigned int words[4] = {0x11111111u, 0x22222222u, 0x33333333u, 0x44444444u};
    unsigned char taggedObject[64] = {};
    unsigned int* taggedWords = (unsigned int*)taggedObject;
    taggedWords[0] = 0xa1a1a1a1u;
    taggedWords[1] = 0xb2b2b2b2u;
    taggedWords[2] = 0xc3c3c3c3u;
    taggedWords[3] = 0xd4d4d4d4u;
    memcpy(taggedObject + 16, "function anonymous(){debugger}", 30);
    words[0] = ((unsigned int)(uintptr_t)taggedObject) | 1u;

    V8RawSourceProbeResult result = {};
    V8RawSourceProbe(words, &result);

    int failures = 0;
    failures += Expect(result.readable != 0, "stack buffer is readable");
    failures += Expect(result.base == words, "probe stores original address");
    failures += Expect(result.words[0] == (void*)(((unsigned int)(uintptr_t)taggedObject) | 1u),
                       "first dword is captured");
    failures += Expect(result.words[3] == (void*)0x44444444u, "fourth dword is captured");
    failures += Expect(result.tagged[0].readable != 0, "tagged pointer target is readable");
    failures += Expect(result.tagged[0].object == taggedObject,
                       "tagged pointer target subtracts low tag bit");
    failures += Expect(result.tagged[0].words[0] == (void*)0xa1a1a1a1u,
                       "tagged pointer target first word is captured");
    failures += Expect(result.tagged[0].asciiOffset == 16,
                       "tagged pointer ascii offset is captured");
    failures += Expect(strstr(result.tagged[0].asciiPreview, "debugger") != NULL,
                       "tagged pointer ascii preview is captured");
    failures += Expect((result.tagged[0].keywordMask & V8_RAW_SOURCE_KEYWORD_DEBUGGER) != 0,
                       "debugger keyword is detected");
    failures += Expect((result.tagged[0].keywordMask & V8_RAW_SOURCE_KEYWORD_FUNCTION) != 0,
                       "function keyword is detected");

    int patched = V8RawSourceNeutralizeDebugger(&result.tagged[0]);
    failures += Expect(patched == 1, "debugger statement is neutralized once");
    failures += Expect(strstr((char*)taggedObject + 16, "debugger") == NULL,
                       "debugger text is removed from object memory");
    failures += Expect(strstr((char*)taggedObject + 16, "        ") != NULL,
                       "debugger text is replaced with same-length spaces");

    V8RawSourceProbe(NULL, &result);
    failures += Expect(result.readable == 0, "null pointer is not readable");

    if (failures) return 1;
    std::printf("PASS v8 raw source probe tests\n");
    return 0;
}
