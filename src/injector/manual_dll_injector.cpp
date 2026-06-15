#include "manual_injector_core.h"

#include <cstdio>

static void PrintUsage() {
    std::fwprintf(stderr, L"Usage: ManualDllInjector.exe <pid> <absolute-dll-path>\n");
}

static int ExitCodeForResult(ManualInjectResultCode code) {
    return code == ManualInjectResultCode::Success ? 0 : 10 + static_cast<int>(code);
}

int wmain(int argc, wchar_t** argv) {
    if (argc != 3) {
        PrintUsage();
        return 2;
    }

    DWORD pid = ManualInjectorParsePid(argv[1]);
    if (!pid) {
        std::fwprintf(stderr, L"Invalid PID: %s\n", argv[1]);
        PrintUsage();
        return 3;
    }

    ManualInjectResult result = ManualInjectDll(pid, argv[2]);
    std::wstring text = ManualInjectorFormatResult(result);
    if (result.code == ManualInjectResultCode::Success) {
        std::fwprintf(stdout, L"Injected %s into PID %lu: %s\n", argv[2], pid, text.c_str());
    } else {
        std::fwprintf(stderr, L"Injection failed: %s\n", text.c_str());
    }
    return ExitCodeForResult(result.code);
}
