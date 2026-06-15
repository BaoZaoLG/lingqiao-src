#include "manual_injector_core.h"

#include <cstdio>
#include <windows.h>

static int Fail(const char* msg) {
    std::printf("FAIL: %s\n", msg);
    return 1;
}

int main() {
    if (ManualInjectorParsePid(L"") != 0) {
        return Fail("empty PID should be invalid");
    }
    if (ManualInjectorParsePid(L"abc") != 0) {
        return Fail("non-numeric PID should be invalid");
    }
    if (ManualInjectorParsePid(L"1234") != 1234) {
        return Fail("numeric PID was not parsed");
    }
    if (ManualInjectorIsAbsolutePath(L"relative.dll")) {
        return Fail("relative path should be rejected");
    }
    if (!ManualInjectorIsAbsolutePath(L"C:\\Temp\\CefHook.dll")) {
        return Fail("drive absolute path should be accepted");
    }
    if (!ManualInjectorIsAbsolutePath(L"\\\\server\\share\\CefHook.dll")) {
        return Fail("UNC absolute path should be accepted");
    }
    if (ManualInjectorDefaultWorkingDirectory(L"C:\\Tools\\App\\target.exe") != L"C:\\Tools\\App") {
        return Fail("default working directory should be executable directory");
    }
    if (ManualInjectorDefaultWorkingDirectory(L"C:\\target.exe") != L"C:\\") {
        return Fail("root executable working directory should keep trailing slash");
    }

    ManualInjectResult result{};
    result.code = ManualInjectResultCode::InvalidDllPath;
    result.win32Error = 5;
    result.remoteModule = 0x1234;
    const std::wstring text = ManualInjectorFormatResult(result);
    if (text.find(L"InvalidDllPath") == std::wstring::npos || text.find(L"5") == std::wstring::npos) {
        return Fail("formatted result should include code and win32 error");
    }

    const DWORD selfPid = GetCurrentProcessId();
    bool foundSelf = false;
    for (const ManualProcessInfo& process : ManualInjectorListProcesses()) {
        if (process.pid == selfPid && !process.exeName.empty()) {
            foundSelf = true;
            break;
        }
    }
    if (!foundSelf) {
        return Fail("process list should include the current process with a name");
    }

    std::printf("PASS: manual injector core tests\n");
    return 0;
}
