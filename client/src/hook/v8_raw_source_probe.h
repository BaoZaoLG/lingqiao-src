#pragma once

#include <windows.h>

#ifdef __cplusplus
extern "C" {
#endif

#define V8_RAW_SOURCE_KEYWORD_DEBUGGER        0x00000001u
#define V8_RAW_SOURCE_KEYWORD_FUNCTION        0x00000002u
#define V8_RAW_SOURCE_KEYWORD_USE_STRICT      0x00000004u
#define V8_RAW_SOURCE_KEYWORD_CLIENT_FORM_SIGN 0x00000008u

typedef struct V8TaggedWordProbeResult {
    int readable;
    void* raw;
    void* object;
    DWORD protect;
    void* words[4];
    int asciiOffset;
    char asciiPreview[96];
    unsigned int keywordMask;
} V8TaggedWordProbeResult;

typedef struct V8RawSourceProbeResult {
    int readable;
    void* base;
    void* allocationBase;
    DWORD protect;
    SIZE_T regionSize;
    void* words[4];
    V8TaggedWordProbeResult tagged[4];
} V8RawSourceProbeResult;

void V8RawSourceProbe(void* sourceHandle, V8RawSourceProbeResult* result);
int V8RawSourceNeutralizeDebugger(V8TaggedWordProbeResult* tagged);

#ifdef __cplusplus
}
#endif
