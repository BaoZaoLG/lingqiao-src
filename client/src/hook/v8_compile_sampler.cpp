#include "v8_compile_sampler.h"
#include "v8_raw_source_probe.h"

#include <windows.h>
#include <cstdio>

extern "C" volatile LONG g_v8CompileSampleWriteIndex = 0;
extern "C" V8CompileSampleRecord g_v8CompileSamples[V8_COMPILE_SAMPLE_CAPACITY] = {};

static HANDLE g_samplerLoggerThread = NULL;
static volatile LONG g_samplerLoggerStop = 0;
static volatile LONG g_samplerLoggerStarted = 0;

static void StoreSample(DWORD seq, void* arg0, void* arg1, void* arg2,
                        void* arg3, void* arg4, void* sourceHandle) {
    LONG slot = (LONG)((seq - 1) & (V8_COMPILE_SAMPLE_CAPACITY - 1));
    g_v8CompileSamples[slot].arg0 = arg0;
    g_v8CompileSamples[slot].arg1 = arg1;
    g_v8CompileSamples[slot].arg2 = arg2;
    g_v8CompileSamples[slot].arg3 = arg3;
    g_v8CompileSamples[slot].arg4 = arg4;
    g_v8CompileSamples[slot].sourceHandle = sourceHandle;
    g_v8CompileSamples[slot].seq = seq;
}

extern "C" void V8CompileSamplerRecordForTest(void* arg0, void* arg1, void* arg2,
                                              void* arg3, void* arg4,
                                              void* sourceHandle) {
    LONG seq = InterlockedIncrement(&g_v8CompileSampleWriteIndex);
    StoreSample((DWORD)seq, arg0, arg1, arg2, arg3, arg4, sourceHandle);
}

extern "C" void V8CompileSamplerGetSnapshot(V8CompileSamplerSnapshot* snapshot) {
    if (!snapshot) return;

    LONG total = InterlockedCompareExchange(&g_v8CompileSampleWriteIndex, 0, 0);
    snapshot->totalHits = total;
    snapshot->latest = {};

    if (total <= 0) return;

    LONG slot = (total - 1) & (V8_COMPILE_SAMPLE_CAPACITY - 1);
    snapshot->latest = g_v8CompileSamples[slot];
}

extern "C" void V8CompileSamplerResetForTest(void) {
    InterlockedExchange(&g_v8CompileSampleWriteIndex, 0);
    ZeroMemory(g_v8CompileSamples, sizeof(g_v8CompileSamples));
}

static DWORD WINAPI V8CompileSamplerLoggerProc(LPVOID) {
    LONG last = 0;

    while (InterlockedCompareExchange(&g_samplerLoggerStop, 0, 0) == 0) {
        Sleep(5000);

        V8CompileSamplerSnapshot snapshot = {};
        V8CompileSamplerGetSnapshot(&snapshot);
        if (snapshot.totalHits <= 0 || snapshot.totalHits == last) continue;

        V8RawSourceProbeResult probe = {};
        V8RawSourceProbe(snapshot.latest.sourceHandle, &probe);

        char msg[256];
        sprintf_s(msg, sizeof(msg),
                  "[V8ICPT] compile sampler hits=%ld latest_seq=%lu "
                  "arg0=%p arg1=%p arg2=%p arg3=%p arg4=%p source_handle=%p",
                  snapshot.totalHits,
                  snapshot.latest.seq,
                  snapshot.latest.arg0,
                  snapshot.latest.arg1,
                  snapshot.latest.arg2,
                  snapshot.latest.arg3,
                  snapshot.latest.arg4,
                  snapshot.latest.sourceHandle);
        OutputDebugStringA(msg);
        OutputDebugStringA("\n");

        sprintf_s(msg, sizeof(msg),
                  "[V8ICPT] source raw readable=%d protect=0x%08lx "
                  "region=%zu words=%p,%p,%p,%p",
                  probe.readable,
                  probe.protect,
                  probe.regionSize,
                  probe.words[0],
                  probe.words[1],
                  probe.words[2],
                  probe.words[3]);
        OutputDebugStringA(msg);
        OutputDebugStringA("\n");

        for (int i = 0; i < 4; ++i) {
            if (!probe.tagged[i].readable) continue;
            if (probe.tagged[i].asciiOffset < 0 && probe.tagged[i].keywordMask == 0) continue;

            int patched = 0;
            if ((probe.tagged[i].keywordMask & V8_RAW_SOURCE_KEYWORD_DEBUGGER) &&
                (probe.tagged[i].keywordMask & V8_RAW_SOURCE_KEYWORD_FUNCTION)) {
                patched = V8RawSourceNeutralizeDebugger(&probe.tagged[i]);
            }

            sprintf_s(msg, sizeof(msg),
                      "[V8ICPT] source tagged%d raw=%p obj=%p readable=%d "
                      "kw=0x%08x patched=%d words=%p,%p,%p,%p ascii_off=%d ascii=%s",
                      i,
                      probe.tagged[i].raw,
                      probe.tagged[i].object,
                      probe.tagged[i].readable,
                      probe.tagged[i].keywordMask,
                      patched,
                      probe.tagged[i].words[0],
                      probe.tagged[i].words[1],
                      probe.tagged[i].words[2],
                      probe.tagged[i].words[3],
                      probe.tagged[i].asciiOffset,
                      probe.tagged[i].asciiPreview);
            OutputDebugStringA(msg);
            OutputDebugStringA("\n");
        }
        last = snapshot.totalHits;
    }

    return 0;
}

extern "C" void V8CompileSamplerStartLogger(void) {
    if (InterlockedCompareExchange(&g_samplerLoggerStarted, 1, 0) != 0) return;

    InterlockedExchange(&g_samplerLoggerStop, 0);
    g_samplerLoggerThread = CreateThread(NULL, 0, V8CompileSamplerLoggerProc,
                                         NULL, 0, NULL);
    if (!g_samplerLoggerThread) {
        InterlockedExchange(&g_samplerLoggerStarted, 0);
        OutputDebugStringA("[V8ICPT] compile sampler logger failed to start\n");
        return;
    }

    OutputDebugStringA("[V8ICPT] compile sampler logger started\n");
}

extern "C" void V8CompileSamplerStopLogger(void) {
    if (InterlockedCompareExchange(&g_samplerLoggerStarted, 0, 1) != 1) return;

    InterlockedExchange(&g_samplerLoggerStop, 1);
    if (g_samplerLoggerThread) {
        WaitForSingleObject(g_samplerLoggerThread, 1000);
        CloseHandle(g_samplerLoggerThread);
        g_samplerLoggerThread = NULL;
    }
}
