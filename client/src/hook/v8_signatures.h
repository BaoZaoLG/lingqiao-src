#pragma once

#include <windows.h>

#include <cstddef>

static const size_t V8_SIGNATURE_COUNT = 5;

struct V8ResolvedSignature {
    const char* name = nullptr;
    PVOID address = nullptr;
    DWORD rva = 0;
    size_t matchCount = 0;
    bool validPattern = false;
};

const char* GetV8SignatureName(size_t index);
bool ResolveV8Signatures(V8ResolvedSignature* out, size_t outCount);
void RunV8SignatureDiagnostics(void);
