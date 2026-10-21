#include "v8_source_neutralizer.h"

#include <ctype.h>
#include <string.h>

static int IsJsIdentifierChar(unsigned char c) {
    return isalnum(c) || c == '_' || c == '$';
}

int V8SourceContainsDebuggerStatement(const char* source, size_t sourceLen) {
    if (!source || sourceLen == 0) return 0;

    static const char kDebugger[] = "debugger";
    const size_t tokenLen = sizeof(kDebugger) - 1;
    if (sourceLen < tokenLen) return 0;

    for (size_t i = 0; i + tokenLen <= sourceLen; ++i) {
        if (memcmp(source + i, kDebugger, tokenLen) != 0) continue;

        unsigned char before = (i == 0) ? 0 : (unsigned char)source[i - 1];
        unsigned char after = (i + tokenLen >= sourceLen)
                                  ? 0
                                  : (unsigned char)source[i + tokenLen];
        if (!IsJsIdentifierChar(before) && !IsJsIdentifierChar(after)) {
            return 1;
        }
    }

    return 0;
}

int V8SourceContainsDebuggerStatementForTest(const char* source) {
    return V8SourceContainsDebuggerStatement(
        source, source ? strlen(source) : 0);
}
