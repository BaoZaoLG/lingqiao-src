#include "process_identity.h"

#include <tlhelp32.h>

#include <sstream>
#include <vector>

static DWORD GetParentProcessId(DWORD pid) {
    HANDLE snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snapshot == INVALID_HANDLE_VALUE) {
        return 0;
    }

    PROCESSENTRY32W entry{};
    entry.dwSize = sizeof(entry);
    if (!Process32FirstW(snapshot, &entry)) {
        CloseHandle(snapshot);
        return 0;
    }

    DWORD parentPid = 0;
    do {
        if (entry.th32ProcessID == pid) {
            parentPid = entry.th32ParentProcessID;
            break;
        }
    } while (Process32NextW(snapshot, &entry));

    CloseHandle(snapshot);
    return parentPid;
}

std::wstring InferCefRoleFromCommandLine(const std::wstring& commandLine) {
    const std::wstring marker = L"--type=";
    size_t pos = commandLine.find(marker);
    if (pos == std::wstring::npos) {
        return L"browser";
    }

    pos += marker.size();
    size_t end = commandLine.find_first_of(L" \t\r\n\"", pos);
    if (end == std::wstring::npos) {
        end = commandLine.size();
    }
    if (end <= pos) {
        return L"other";
    }
    return commandLine.substr(pos, end - pos);
}

std::wstring FormatProcessIdentityLog(const ProcessIdentity& identity) {
    std::wstringstream ss;
    ss << L"[HOOK] process pid=" << identity.pid
       << L" ppid=" << identity.parentPid
       << L" role=" << identity.cefRole
       << L" exe=\"" << identity.exePath
       << L"\" cmd=\"" << identity.commandLine << L"\"";
    return ss.str();
}

ProcessIdentity GetCurrentProcessIdentity() {
    ProcessIdentity identity{};
    identity.pid = GetCurrentProcessId();
    identity.parentPid = GetParentProcessId(identity.pid);

    wchar_t exePath[MAX_PATH * 4]{};
    DWORD exeChars = GetModuleFileNameW(nullptr, exePath, static_cast<DWORD>(sizeof(exePath) / sizeof(exePath[0])));
    if (exeChars > 0) {
        identity.exePath.assign(exePath, exeChars);
    }

    LPWSTR commandLine = GetCommandLineW();
    if (commandLine) {
        identity.commandLine = commandLine;
    }
    identity.cefRole = InferCefRoleFromCommandLine(identity.commandLine);
    return identity;
}

void LogCurrentProcessIdentity() {
    const std::wstring line = FormatProcessIdentityLog(GetCurrentProcessIdentity());
    OutputDebugStringW(line.c_str());
    OutputDebugStringW(L"\n");
}
