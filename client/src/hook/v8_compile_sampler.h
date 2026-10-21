#pragma once

#include <windows.h>

#ifdef __cplusplus
extern "C" {
#endif

#define V8_COMPILE_SAMPLE_CAPACITY 64

typedef struct V8CompileSampleRecord {
    DWORD seq;
    void* arg0;
    void* arg1;
    void* arg2;
    void* arg3;
    void* arg4;
    void* sourceHandle;
} V8CompileSampleRecord;

typedef struct V8CompileSamplerSnapshot {
    LONG totalHits;
    V8CompileSampleRecord latest;
} V8CompileSamplerSnapshot;

extern volatile LONG g_v8CompileSampleWriteIndex;
extern V8CompileSampleRecord g_v8CompileSamples[V8_COMPILE_SAMPLE_CAPACITY];

void V8CompileSamplerRecordForTest(void* arg0, void* arg1, void* arg2,
                                   void* arg3, void* arg4,
                                   void* sourceHandle);
void V8CompileSamplerGetSnapshot(V8CompileSamplerSnapshot* snapshot);
void V8CompileSamplerResetForTest(void);
void V8CompileSamplerStartLogger(void);
void V8CompileSamplerStopLogger(void);

#ifdef __cplusplus
}
#endif
