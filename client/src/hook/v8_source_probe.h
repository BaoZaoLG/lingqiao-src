#pragma once

#include <stddef.h>

struct V8SourceProbeResult {
    bool readable;
    int candidateIndex;
    void* sourceHandle;
    size_t previewLen;
    char preview[256];
};

V8SourceProbeResult ProbeV8CompileSource(void* compileArg2);
