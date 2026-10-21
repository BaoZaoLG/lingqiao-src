#include "hook/v8_fast_neutralize.h"

#include <cstdio>
#include <cstring>

static int Expect(bool condition, const char* message) {
    if (!condition) {
        std::printf("FAIL %s\n", message);
        return 1;
    }
    return 0;
}

int main() {
    /* Simulate a V8 tagged-pointer layout:
     *   sourceHandle[0..3]  →  4 dwords, some have low bit set (tagged)
     *   tagged pointer - 1   →  object containing JS source text
     *
     * The fast neutralizer follows each tagged pointer to the object,
     * scans 1024 bytes for "debugger", and replaces it with spaces. */

    unsigned char* object = (unsigned char*)VirtualAlloc(
        NULL, 4096, MEM_COMMIT, PAGE_READWRITE);
    if (!object) {
        std::printf("FAIL VirtualAlloc failed\n");
        return 1;
    }

    const char* js =
        "(function anonymous() {\ndebugger\n})";
    memcpy(object + 100, js, strlen(js));

    /* Build the source handle region: second slot = tagged pointer */
    void* sourceRegion[4] = {};
    sourceRegion[1] = (void*)((uintptr_t)object | 1u);

    int patched = V8FastNeutralizeFromSourceHandle(sourceRegion);

    int failures = 0;
    failures += Expect(patched == 1, "one debugger statement is patched");
    failures += Expect(strstr((char*)(object + 100), "debugger") == NULL,
                       "debugger text is removed from object memory");
    failures += Expect(strstr((char*)(object + 100), "        ") != NULL,
                       "debugger text is replaced with same-length spaces");

    /* Verify surrounding JS source is intact */
    failures += Expect(strstr((char*)(object + 100), "function anonymous") != NULL,
                       "surrounding JS source is preserved");

    /* Null sourceHandle should be safe */
    int nullResult = V8FastNeutralizeFromSourceHandle(NULL);
    failures += Expect(nullResult == 0, "null sourceHandle returns 0");

    /* Non-tagged pointer should be skipped safely */
    void* plainRegion[4] = { (void*)0x12340000, NULL, NULL, NULL };
    int plainResult = V8FastNeutralizeFromSourceHandle(plainRegion);
    failures += Expect(plainResult == 0,
                       "non-tagged pointer is skipped without crash");

    VirtualFree(object, 0, MEM_RELEASE);

    if (failures) return 1;
    std::printf("PASS v8 fast neutralizer tests\n");
    return 0;
}
