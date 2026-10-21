#include "pch.h"
#include "v8_source_probe.h"

#include <windows.h>
#include <cstdio>
#include <cstring>

static bool SafeReadPtr(void* base, int offset, void** out) {
    __try {
        if (!base || IsBadReadPtr(base, offset + (int)sizeof(void*))) return false;
        *out = *(void**)((char*)base + offset);
        return *out != NULL;
    }
    __except (EXCEPTION_EXECUTE_HANDLER) {
        return false;
    }
}

V8SourceProbeResult ProbeV8CompileSource(void* compileArg2) {
    V8SourceProbeResult r = {};
    const int offsets[] = { 0, 4, 8, 12, 16, 20, 24, 28, 32 };

    for (int i = 0; i < (int)(sizeof(offsets) / sizeof(offsets[0])); ++i) {
        void* candidate = NULL;
        if (!SafeReadPtr(compileArg2, offsets[i], &candidate)) continue;

        r.sourceHandle = candidate;
        r.candidateIndex = offsets[i];
        r.readable = true;
        sprintf_s(r.preview, sizeof(r.preview), "candidate_handle=%p", candidate);
        r.previewLen = strlen(r.preview);
        return r;
    }

    r.candidateIndex = -1;
    return r;
}
