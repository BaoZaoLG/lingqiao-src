#include "v8_signatures.h"

#include "pattern_scan.h"

#include <windows.h>

#include <cstdio>

struct V8Signature {
    const char* name;
    const char* pattern;
};

static const V8Signature kV8Signatures[] = {
    {
        "v8.script_compiler.compile_script.func",
        "55 89 E5 53 57 56 83 E4 ? 81 EC ? ? ? ? A1 ? ? ? ? 31 E8 89 84 24 ? ? ? ? A1 ? ? ? ? 85 C0 75 ? E8",
    },
    {
        "v8.script_run.xref",
        "68 ? ? ? ? E8 ? ? ? ? 8B 87 ? ? ? ? C7 87 ? ? ? ? ? ? ? ? 8B 97",
    },
    {
        "v8.function_template.set_call_handler.func",
        "55 89 E5 53 57 56 83 EC ? A1 ? ? ? ? 89 CF 31 E8 89 45 ? 8B 01 8B 48 ? F6 C1",
    },
    {
        "v8.function_template.new.func",
        "55 89 E5 53 57 56 83 E4 ? 83 EC ? A1 ? ? ? ? 8B 75 ? 8D 54 24 ? 31 E8 89 44 24 ? C7 04 24 ? ? ? ? C7 44 24 ? ? ? ? ? C7 44 24 ? ? ? ? ? C7 44 24 ? ? ? ? ? C7 44 24 ? ? ? ? ? C7 44 24 ? ? ? ? ? C7 44 24 ? ? ? ? ? A1 ? ? ? ? 85 C0 75 ? 8B 8E ? ? ? ? 8B 7D",
    },
    {
        "v8.function_template.new.alt_func",
        "55 89 E5 53 57 56 83 E4 ? 83 EC ? A1 ? ? ? ? 89 CB 8B 4D ? 89 D7",
    },
};

static void LogLine(const char* text) {
    OutputDebugStringA(text);
    OutputDebugStringA("\n");
}

const char* GetV8SignatureName(size_t index) {
    if (index >= V8_SIGNATURE_COUNT) return "unknown";
    return kV8Signatures[index].name;
}

bool ResolveV8Signatures(V8ResolvedSignature* out, size_t outCount) {
    if (!out || outCount < V8_SIGNATURE_COUNT) return false;
    for (size_t i = 0; i < V8_SIGNATURE_COUNT; ++i) {
        out[i] = V8ResolvedSignature{};
        out[i].name = kV8Signatures[i].name;
    }

    HMODULE libcef = GetModuleHandleA("libcef.dll");
    if (!libcef) {
        return false;
    }

    bool allUnique = true;
    for (size_t i = 0; i < V8_SIGNATURE_COUNT; ++i) {
        const V8Signature& sig = kV8Signatures[i];
        PatternBytes pattern;
        if (!ParseIdaPattern(sig.pattern, &pattern)) {
            allUnique = false;
            continue;
        }
        out[i].validPattern = true;

        const std::vector<PatternMatch> matches = FindPatternInModule(libcef, pattern, 8);
        out[i].matchCount = matches.size();
        if (!matches.empty()) {
            out[i].address = const_cast<unsigned char*>(matches[0].address);
            out[i].rva = matches[0].rva;
        }
        if (matches.size() != 1) {
            allUnique = false;
        }
    }
    return allUnique;
}

void RunV8SignatureDiagnostics(void) {
    HMODULE libcef = GetModuleHandleA("libcef.dll");
    if (!libcef) {
        LogLine("[HOOK] V8 libcef.dll not loaded; skipping signature diagnostics");
        return;
    }

    char header[160];
    sprintf_s(header, sizeof(header), "[HOOK] V8 scanning libcef.dll base=0x%p signatures=%zu",
              libcef, V8_SIGNATURE_COUNT);
    LogLine(header);

    V8ResolvedSignature resolved[V8_SIGNATURE_COUNT];
    ResolveV8Signatures(resolved, V8_SIGNATURE_COUNT);

    for (size_t i = 0; i < V8_SIGNATURE_COUNT; ++i) {
        if (!resolved[i].validPattern) {
            char msg[192];
            sprintf_s(msg, sizeof(msg), "[HOOK] V8 invalid pattern name=%s", resolved[i].name);
            LogLine(msg);
            continue;
        }
        if (resolved[i].matchCount == 0) {
            char msg[192];
            sprintf_s(msg, sizeof(msg), "[HOOK] V8 miss name=%s", resolved[i].name);
            LogLine(msg);
            continue;
        }

        const char* status = resolved[i].matchCount == 1 ? "hit" : "multi";
        char msg[256];
        sprintf_s(msg, sizeof(msg), "[HOOK] V8 %s name=%s count=%zu rva=0x%08lX va=0x%p",
                  status, resolved[i].name, resolved[i].matchCount,
                  static_cast<unsigned long>(resolved[i].rva),
                  resolved[i].address);
        LogLine(msg);
    }
}
