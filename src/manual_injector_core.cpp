#include "manual_injector_core.h"

#include <cwchar>
#include <sstream>
#include <tlhelp32.h>
#include <vector>

DWORD ManualInjectorParsePid(const wchar_t* text) {
    if (!text || !text[0]) return 0;
    wchar_t* end = nullptr;
    unsigned long value = std::wcstoul(text, &end, 10);
    if (!end || *end != L'\0' || value == 0 || value > 0xFFFFFFFFUL) {
        return 0;
    }
    return static_cast<DWORD>(value);
}

bool ManualInjectorIsAbsolutePath(const wchar_t* path) {
    if (!path || !path[0]) return false;
    const size_t len = std::wcslen(path);
    if (len >= 3 && path[1] == L':' && (path[2] == L'\\' || path[2] == L'/')) {
        return true;
    }
    return len >= 3 && path[0] == L'\\' && path[1] == L'\\';
}

bool ManualInjectorFileExists(const wchar_t* path) {
    DWORD attrs = GetFileAttributesW(path);
    return attrs != INVALID_FILE_ATTRIBUTES && (attrs & FILE_ATTRIBUTE_DIRECTORY) == 0;
}

std::wstring ManualInjectorDefaultWorkingDirectory(const wchar_t* exePath) {
    if (!exePath || !exePath[0]) return L"";
    std::wstring path(exePath);
    size_t pos = path.find_last_of(L"\\/");
    if (pos == std::wstring::npos) return L"";
    if (pos == 2 && path.size() >= 3 && path[1] == L':') {
        return path.substr(0, 3);
    }
    return path.substr(0, pos);
}

std::vector<ManualProcessInfo> ManualInjectorListProcesses() {
    std::vector<ManualProcessInfo> processes;
    HANDLE snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snapshot == INVALID_HANDLE_VALUE) {
        return processes;
    }

    PROCESSENTRY32W entry{};
    entry.dwSize = sizeof(entry);
    if (!Process32FirstW(snapshot, &entry)) {
        CloseHandle(snapshot);
        return processes;
    }

    do {
        ManualProcessInfo info{};
        info.pid = entry.th32ProcessID;
        info.parentPid = entry.th32ParentProcessID;
        info.exeName = entry.szExeFile;

        HANDLE process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, info.pid);
        if (process) {
            wchar_t path[MAX_PATH * 4]{};
            DWORD pathChars = static_cast<DWORD>(sizeof(path) / sizeof(path[0]));
            if (QueryFullProcessImageNameW(process, 0, path, &pathChars)) {
                info.exePath.assign(path, pathChars);
            }
            CloseHandle(process);
        }

        processes.push_back(info);
    } while (Process32NextW(snapshot, &entry));

    CloseHandle(snapshot);
    return processes;
}

const wchar_t* ManualInjectorResultCodeName(ManualInjectResultCode code) {
    switch (code) {
    case ManualInjectResultCode::Success: return L"Success";
    case ManualInjectResultCode::InvalidPid: return L"InvalidPid";
    case ManualInjectResultCode::InvalidDllPath: return L"InvalidDllPath";
    case ManualInjectResultCode::DllNotFound: return L"DllNotFound";
    case ManualInjectResultCode::OpenProcessFailed: return L"OpenProcessFailed";
    case ManualInjectResultCode::VirtualAllocFailed: return L"VirtualAllocFailed";
    case ManualInjectResultCode::WriteMemoryFailed: return L"WriteMemoryFailed";
    case ManualInjectResultCode::Kernel32NotFound: return L"Kernel32NotFound";
    case ManualInjectResultCode::LoadLibraryNotFound: return L"LoadLibraryNotFound";
    case ManualInjectResultCode::CreateRemoteThreadFailed: return L"CreateRemoteThreadFailed";
    case ManualInjectResultCode::RemoteThreadTimeout: return L"RemoteThreadTimeout";
    case ManualInjectResultCode::GetExitCodeFailed: return L"GetExitCodeFailed";
    case ManualInjectResultCode::RemoteLoadLibraryFailed: return L"RemoteLoadLibraryFailed";
    case ManualInjectResultCode::InvalidExePath: return L"InvalidExePath";
    case ManualInjectResultCode::ExeNotFound: return L"ExeNotFound";
    case ManualInjectResultCode::CreateProcessFailed: return L"CreateProcessFailed";
    default: return L"Unknown";
    }
}

std::wstring ManualInjectorFormatResult(const ManualInjectResult& result) {
    std::wstringstream ss;
    ss << ManualInjectorResultCodeName(result.code)
       << L" win32=" << result.win32Error
       << L" remoteModule=0x" << std::hex << result.remoteModule;
    return ss.str();
}

ManualInjectResult ManualInjectDll(DWORD pid, const wchar_t* dllPath) {
    if (pid == 0) return {ManualInjectResultCode::InvalidPid, 0, 0};
    if (!ManualInjectorIsAbsolutePath(dllPath)) return {ManualInjectResultCode::InvalidDllPath, 0, 0};
    if (!ManualInjectorFileExists(dllPath)) return {ManualInjectResultCode::DllNotFound, GetLastError(), 0};

    const size_t bytes = (std::wcslen(dllPath) + 1) * sizeof(wchar_t);
    HANDLE process = OpenProcess(
        PROCESS_CREATE_THREAD | PROCESS_QUERY_INFORMATION | PROCESS_VM_OPERATION |
            PROCESS_VM_WRITE | PROCESS_VM_READ,
        FALSE,
        pid);
    if (!process) return {ManualInjectResultCode::OpenProcessFailed, GetLastError(), 0};

    LPVOID remotePath = VirtualAllocEx(process, nullptr, bytes, MEM_COMMIT | MEM_RESERVE, PAGE_READWRITE);
    if (!remotePath) {
        DWORD err = GetLastError();
        CloseHandle(process);
        return {ManualInjectResultCode::VirtualAllocFailed, err, 0};
    }

    if (!WriteProcessMemory(process, remotePath, dllPath, bytes, nullptr)) {
        DWORD err = GetLastError();
        VirtualFreeEx(process, remotePath, 0, MEM_RELEASE);
        CloseHandle(process);
        return {ManualInjectResultCode::WriteMemoryFailed, err, 0};
    }

    HMODULE kernel32 = GetModuleHandleW(L"kernel32.dll");
    if (!kernel32) {
        DWORD err = GetLastError();
        VirtualFreeEx(process, remotePath, 0, MEM_RELEASE);
        CloseHandle(process);
        return {ManualInjectResultCode::Kernel32NotFound, err, 0};
    }

    FARPROC loadLibrary = GetProcAddress(kernel32, "LoadLibraryW");
    if (!loadLibrary) {
        DWORD err = GetLastError();
        VirtualFreeEx(process, remotePath, 0, MEM_RELEASE);
        CloseHandle(process);
        return {ManualInjectResultCode::LoadLibraryNotFound, err, 0};
    }

    HANDLE thread = CreateRemoteThread(
        process,
        nullptr,
        0,
        reinterpret_cast<LPTHREAD_START_ROUTINE>(loadLibrary),
        remotePath,
        0,
        nullptr);
    if (!thread) {
        DWORD err = GetLastError();
        VirtualFreeEx(process, remotePath, 0, MEM_RELEASE);
        CloseHandle(process);
        return {ManualInjectResultCode::CreateRemoteThreadFailed, err, 0};
    }

    DWORD wait = WaitForSingleObject(thread, 15000);
    if (wait != WAIT_OBJECT_0) {
        CloseHandle(thread);
        VirtualFreeEx(process, remotePath, 0, MEM_RELEASE);
        CloseHandle(process);
        return {ManualInjectResultCode::RemoteThreadTimeout, wait, 0};
    }

    DWORD remoteResult = 0;
    if (!GetExitCodeThread(thread, &remoteResult)) {
        DWORD err = GetLastError();
        CloseHandle(thread);
        VirtualFreeEx(process, remotePath, 0, MEM_RELEASE);
        CloseHandle(process);
        return {ManualInjectResultCode::GetExitCodeFailed, err, 0};
    }

    CloseHandle(thread);
    VirtualFreeEx(process, remotePath, 0, MEM_RELEASE);
    CloseHandle(process);

    if (remoteResult == 0) {
        return {ManualInjectResultCode::RemoteLoadLibraryFailed, 0, 0};
    }
    return {ManualInjectResultCode::Success, 0, remoteResult};
}

static std::wstring QuoteForCommandLine(const wchar_t* value) {
    std::wstring out = L"\"";
    for (const wchar_t* p = value; p && *p; ++p) {
        if (*p == L'"') out += L'\\';
        out += *p;
    }
    out += L"\"";
    return out;
}

ManualInjectResult ManualLaunchAndInject(
    const wchar_t* exePath,
    const wchar_t* arguments,
    const wchar_t* workingDirectory,
    const wchar_t* dllPath,
    DWORD injectDelayMs,
    DWORD* launchedPid) {
    if (launchedPid) *launchedPid = 0;
    if (!ManualInjectorIsAbsolutePath(exePath)) return {ManualInjectResultCode::InvalidExePath, 0, 0};
    if (!ManualInjectorFileExists(exePath)) return {ManualInjectResultCode::ExeNotFound, GetLastError(), 0};
    if (!ManualInjectorIsAbsolutePath(dllPath)) return {ManualInjectResultCode::InvalidDllPath, 0, 0};
    if (!ManualInjectorFileExists(dllPath)) return {ManualInjectResultCode::DllNotFound, GetLastError(), 0};

    std::wstring cwd;
    if (workingDirectory && workingDirectory[0]) {
        cwd = workingDirectory;
    } else {
        cwd = ManualInjectorDefaultWorkingDirectory(exePath);
    }

    std::wstring commandLine = QuoteForCommandLine(exePath);
    if (arguments && arguments[0]) {
        commandLine += L" ";
        commandLine += arguments;
    }
    std::vector<wchar_t> mutableCommandLine(commandLine.begin(), commandLine.end());
    mutableCommandLine.push_back(L'\0');

    STARTUPINFOW si{};
    si.cb = sizeof(si);
    PROCESS_INFORMATION pi{};
    BOOL ok = CreateProcessW(
        exePath,
        mutableCommandLine.data(),
        nullptr,
        nullptr,
        FALSE,
        0,
        nullptr,
        cwd.empty() ? nullptr : cwd.c_str(),
        &si,
        &pi);
    if (!ok) {
        return {ManualInjectResultCode::CreateProcessFailed, GetLastError(), 0};
    }

    if (launchedPid) *launchedPid = pi.dwProcessId;
    if (injectDelayMs > 0) {
        DWORD idle = WaitForInputIdle(pi.hProcess, injectDelayMs);
        if (idle != WAIT_OBJECT_0) {
            Sleep(injectDelayMs);
        }
    }

    ManualInjectResult result = ManualInjectDll(pi.dwProcessId, dllPath);
    CloseHandle(pi.hThread);
    CloseHandle(pi.hProcess);
    return result;
}
