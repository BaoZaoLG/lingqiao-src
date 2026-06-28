#pragma once

#include <windows.h>

#include <string>

struct ProcessIdentity {
    DWORD pid = 0;
    DWORD parentPid = 0;
    std::wstring exePath;
    std::wstring commandLine;
    std::wstring cefRole;
};

std::wstring InferCefRoleFromCommandLine(const std::wstring& commandLine);
std::wstring FormatProcessIdentityLog(const ProcessIdentity& identity);
ProcessIdentity GetCurrentProcessIdentity();
void LogCurrentProcessIdentity();
