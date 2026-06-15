#include "process_identity.h"

#include <cstdio>

static int Fail(const char* msg) {
    std::printf("FAIL: %s\n", msg);
    return 1;
}

int main() {
    if (InferCefRoleFromCommandLine(L"CXexam.exe --type=renderer --lang=zh-CN") != L"renderer") {
        return Fail("renderer role should be inferred from command line");
    }
    if (InferCefRoleFromCommandLine(L"CXexam.exe --type=gpu-process") != L"gpu-process") {
        return Fail("gpu role should be inferred from command line");
    }
    if (InferCefRoleFromCommandLine(L"CXexam.exe --flag") != L"browser") {
        return Fail("missing --type should be treated as browser");
    }

    ProcessIdentity identity{};
    identity.pid = 1234;
    identity.parentPid = 5678;
    identity.exePath = L"C:\\Apps\\CXexam.exe";
    identity.commandLine = L"\"C:\\Apps\\CXexam.exe\" --type=renderer";
    identity.cefRole = InferCefRoleFromCommandLine(identity.commandLine);

    const std::wstring line = FormatProcessIdentityLog(identity);
    if (line.find(L"[HOOK] process pid=1234 ppid=5678 role=renderer") == std::wstring::npos) {
        return Fail("formatted log should include pid, ppid, and role");
    }
    if (line.find(identity.exePath) == std::wstring::npos ||
        line.find(identity.commandLine) == std::wstring::npos) {
        return Fail("formatted log should include exe path and command line");
    }

    const ProcessIdentity current = GetCurrentProcessIdentity();
    if (current.pid == 0 || current.exePath.empty() || current.commandLine.empty()) {
        return Fail("current process identity should include pid, exe path, and command line");
    }

    std::printf("PASS: process identity tests\n");
    return 0;
}
