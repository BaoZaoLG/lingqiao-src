#pragma once

#include <windows.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Called from V8Detour0 BEFORE returning to the trampoline.
 * Decodes tagged pointers from sourceHandle, scans each target
 * object for the ASCII "debugger" keyword, and replaces it with
 * same-length spaces.  Uses VirtualProtect + SEH only — no V8
 * API calls, no heap allocation, no locks.
 *
 * Returns the number of "debugger" occurrences patched. */
int V8FastNeutralizeFromSourceHandle(void* sourceHandle);

#ifdef __cplusplus
}
#endif
