#include "v8_fast_neutralize.h"
#include "v8_raw_source_probe.h"

#include <windows.h>

extern "C" int V8FastNeutralizeFromSourceHandle(void* sourceHandle) {
    if (!sourceHandle) return 0;

    void* words[4];

    __try {
        void** p = (void**)sourceHandle;
        words[0] = p[0];
        words[1] = p[1];
        words[2] = p[2];
        words[3] = p[3];
    } __except (EXCEPTION_EXECUTE_HANDLER) {
        return 0;
    }

    int totalPatched = 0;

    for (int i = 0; i < 4; i++) {
        ULONG_PTR val = (ULONG_PTR)words[i];
        if ((val & 1u) == 0) continue;

        void* obj = (void*)(val - 1u);

        MEMORY_BASIC_INFORMATION mbi = {};
        if (!VirtualQuery(obj, &mbi, sizeof(mbi))) continue;
        if (mbi.State != MEM_COMMIT) continue;
        DWORD p = mbi.Protect & 0xff;
        if (p == PAGE_NOACCESS) continue;
        if (mbi.Protect & PAGE_GUARD) continue;

        DWORD oldProtect = 0;
        if (!VirtualProtect(obj, 1024, PAGE_READWRITE, &oldProtect)) continue;

        __try {
            unsigned char* bytes = (unsigned char*)obj;
            for (int j = 0; j <= 1024 - 8; j++) {
                if (bytes[j]     == 'd' &&
                    bytes[j + 1] == 'e' &&
                    bytes[j + 2] == 'b' &&
                    bytes[j + 3] == 'u' &&
                    bytes[j + 4] == 'g' &&
                    bytes[j + 5] == 'g' &&
                    bytes[j + 6] == 'e' &&
                    bytes[j + 7] == 'r') {
                    bytes[j]     = ' ';
                    bytes[j + 1] = ' ';
                    bytes[j + 2] = ' ';
                    bytes[j + 3] = ' ';
                    bytes[j + 4] = ' ';
                    bytes[j + 5] = ' ';
                    bytes[j + 6] = ' ';
                    bytes[j + 7] = ' ';
                    totalPatched++;
                }
            }
        } __except (EXCEPTION_EXECUTE_HANDLER) {
            /* skip this slot */
        }

        if (oldProtect) {
            DWORD tmp;
            VirtualProtect(obj, 1024, oldProtect, &tmp);
        }
    }

    return totalPatched;
}
