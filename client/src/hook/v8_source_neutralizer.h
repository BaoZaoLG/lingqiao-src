#pragma once

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

int V8SourceContainsDebuggerStatement(const char* source, size_t sourceLen);
int V8SourceContainsDebuggerStatementForTest(const char* source);

#ifdef __cplusplus
}
#endif
