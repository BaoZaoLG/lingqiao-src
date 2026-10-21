#include "v8_raw_source_probe.h"

#include <windows.h>
#include <cstring>

static int IsReadableProtect(DWORD protect) {
    if (protect & PAGE_GUARD) return 0;
    if (protect & PAGE_NOACCESS) return 0;

    DWORD p = protect & 0xff;
    return p == PAGE_READONLY ||
           p == PAGE_READWRITE ||
           p == PAGE_WRITECOPY ||
           p == PAGE_EXECUTE_READ ||
           p == PAGE_EXECUTE_READWRITE ||
           p == PAGE_EXECUTE_WRITECOPY;
}

static int IsAsciiPreviewChar(unsigned char c) {
    return c == '\t' || c == '\n' || c == '\r' || (c >= 0x20 && c <= 0x7e);
}

static int IsUsefulAsciiChar(unsigned char c) {
    return c >= 0x20 && c <= 0x7e;
}

static int BytesEqualAt(const unsigned char* bytes, int offset,
                        const char* needle, int needleLen) {
    for (int i = 0; i < needleLen; ++i) {
        if (bytes[offset + i] != (unsigned char)needle[i]) return 0;
    }
    return 1;
}

static void UpdateKeywordMask(const unsigned char* bytes, int limit,
                              V8TaggedWordProbeResult* result) {
    struct Keyword {
        const char* text;
        int len;
        unsigned int bit;
    };
    static const Keyword keywords[] = {
        {"debugger", 8, V8_RAW_SOURCE_KEYWORD_DEBUGGER},
        {"function", 8, V8_RAW_SOURCE_KEYWORD_FUNCTION},
        {"use strict", 10, V8_RAW_SOURCE_KEYWORD_USE_STRICT},
        {"CLIENT_FORM_SIGN", 16, V8_RAW_SOURCE_KEYWORD_CLIENT_FORM_SIGN},
    };

    for (int k = 0; k < (int)(sizeof(keywords) / sizeof(keywords[0])); ++k) {
        for (int i = 0; i <= limit - keywords[k].len; ++i) {
            if (BytesEqualAt(bytes, i, keywords[k].text, keywords[k].len)) {
                result->keywordMask |= keywords[k].bit;
                break;
            }
        }
    }
}

static void CaptureAsciiPreview(void* object, V8TaggedWordProbeResult* result) {
    result->asciiOffset = -1;
    result->asciiPreview[0] = '\0';

    __try {
        unsigned char* bytes = (unsigned char*)object;
        int bestStart = -1;
        int bestLen = 0;

        const int limit = 1024;
        UpdateKeywordMask(bytes, limit, result);

        for (int i = 0; i < limit; ++i) {
            if (!IsUsefulAsciiChar(bytes[i])) continue;

            int j = i;
            while (j < limit && IsAsciiPreviewChar(bytes[j])) ++j;

            int len = j - i;
            if (len > bestLen) {
                bestStart = i;
                bestLen = len;
            }
            i = j;
        }

        if (bestStart < 0 || bestLen < 8) return;

        int copyLen = bestLen;
        if (copyLen > (int)sizeof(result->asciiPreview) - 1) {
            copyLen = (int)sizeof(result->asciiPreview) - 1;
        }

        for (int i = 0; i < copyLen; ++i) {
            unsigned char c = bytes[bestStart + i];
            result->asciiPreview[i] = IsAsciiPreviewChar(c) ? (char)c : '.';
        }
        result->asciiPreview[copyLen] = '\0';
        result->asciiOffset = bestStart;
    }
    __except (EXCEPTION_EXECUTE_HANDLER) {
        result->asciiOffset = -1;
        result->asciiPreview[0] = '\0';
    }
}

static void ProbeTaggedWord(void* raw, V8TaggedWordProbeResult* result) {
    if (!result) return;
    result->raw = raw;

    ULONG_PTR value = (ULONG_PTR)raw;
    if ((value & 1u) == 0) return;

    void* object = (void*)(value - 1u);
    result->object = object;

    MEMORY_BASIC_INFORMATION mbi = {};
    if (!VirtualQuery(object, &mbi, sizeof(mbi))) return;
    result->protect = mbi.Protect;
    if (mbi.State != MEM_COMMIT || !IsReadableProtect(mbi.Protect)) return;

    __try {
        void** p = (void**)object;
        result->words[0] = p[0];
        result->words[1] = p[1];
        result->words[2] = p[2];
        result->words[3] = p[3];
        result->readable = 1;
        CaptureAsciiPreview(object, result);
    }
    __except (EXCEPTION_EXECUTE_HANDLER) {
        result->readable = 0;
    }
}

extern "C" void V8RawSourceProbe(void* sourceHandle,
                                 V8RawSourceProbeResult* result) {
    if (!result) return;
    ZeroMemory(result, sizeof(*result));
    result->base = sourceHandle;

    if (!sourceHandle) return;

    MEMORY_BASIC_INFORMATION mbi = {};
    if (!VirtualQuery(sourceHandle, &mbi, sizeof(mbi))) return;

    result->allocationBase = mbi.AllocationBase;
    result->protect = mbi.Protect;
    result->regionSize = mbi.RegionSize;

    if (mbi.State != MEM_COMMIT || !IsReadableProtect(mbi.Protect)) return;

    __try {
        void** p = (void**)sourceHandle;
        result->words[0] = p[0];
        result->words[1] = p[1];
        result->words[2] = p[2];
        result->words[3] = p[3];
        result->readable = 1;
        ProbeTaggedWord(result->words[0], &result->tagged[0]);
        ProbeTaggedWord(result->words[1], &result->tagged[1]);
        ProbeTaggedWord(result->words[2], &result->tagged[2]);
        ProbeTaggedWord(result->words[3], &result->tagged[3]);
    }
    __except (EXCEPTION_EXECUTE_HANDLER) {
        result->readable = 0;
    }
}

extern "C" int V8RawSourceNeutralizeDebugger(V8TaggedWordProbeResult* tagged) {
    if (!tagged || !tagged->readable || !tagged->object) return 0;
    const unsigned int required = V8_RAW_SOURCE_KEYWORD_DEBUGGER |
                                  V8_RAW_SOURCE_KEYWORD_FUNCTION;
    if ((tagged->keywordMask & required) != required) return 0;

    unsigned char* bytes = (unsigned char*)tagged->object;
    const char needle[] = "debugger";
    const int needleLen = 8;
    const int limit = 1024;
    int patched = 0;

    DWORD oldProtect = 0;
    if (!VirtualProtect(tagged->object, limit, PAGE_READWRITE, &oldProtect)) {
        oldProtect = 0;
    }

    __try {
        for (int i = 0; i <= limit - needleLen; ++i) {
            if (!BytesEqualAt(bytes, i, needle, needleLen)) continue;
            for (int j = 0; j < needleLen; ++j) {
                bytes[i + j] = ' ';
            }
            patched++;
        }
    }
    __except (EXCEPTION_EXECUTE_HANDLER) {
        patched = 0;
    }

    if (oldProtect) {
        DWORD ignored = 0;
        VirtualProtect(tagged->object, limit, oldProtect, &ignored);
    }

    return patched;
}
