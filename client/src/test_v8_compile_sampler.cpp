#include "hook/v8_compile_sampler.h"

#include <cstdio>

static int Expect(bool condition, const char* message) {
    if (!condition) {
        std::printf("FAIL %s\n", message);
        return 1;
    }
    return 0;
}

int main() {
    V8CompileSamplerResetForTest();

    V8CompileSamplerRecordForTest(
        (void*)0x11111111, (void*)0x22222222, (void*)0x33333333,
        (void*)0x44444444, (void*)0x55555555, (void*)0x66666666);
    V8CompileSamplerRecordForTest(
        (void*)0xaaaaaaaa, (void*)0xbbbbbbbb, (void*)0xcccccccc,
        (void*)0xdddddddd, (void*)0xeeeeeeee, (void*)0xffffffff);

    V8CompileSamplerSnapshot snapshot = {};
    V8CompileSamplerGetSnapshot(&snapshot);

    int failures = 0;
    failures += Expect(snapshot.totalHits == 2, "total hit count is tracked");
    failures += Expect(snapshot.latest.seq == 2, "latest sequence is tracked");
    failures += Expect(snapshot.latest.arg0 == (void*)0xaaaaaaaa, "latest arg0 is tracked");
    failures += Expect(snapshot.latest.arg4 == (void*)0xeeeeeeee, "latest arg4 is tracked");
    failures += Expect(snapshot.latest.sourceHandle == (void*)0xffffffff,
                       "latest source handle candidate is tracked");

    if (failures) return 1;
    std::printf("PASS v8 compile sampler tests\n");
    return 0;
}
