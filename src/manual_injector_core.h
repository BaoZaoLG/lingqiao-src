#pragma once

#include <windows.h>

#include <string>
#include <vector>

enum class ManualInjectResultCode {
    Success = 0,
    InvalidPid,
    InvalidDllPath,
    DllNotFound,
    OpenProcessFailed,
    VirtualAllocFailed,
    WriteMemoryFailed,
    Kernel32NotFound,
    LoadLibraryNotFound,
    CreateRemoteThreadFailed,
    RemoteThreadTimeout,
    GetExitCodeFailed,
    RemoteLoadLibraryFailed,
    InvalidExePath,
    ExeNotFound,
    CreateProcessFailed,
};

struct ManualInjectResult {
    ManualInjectResultCode code = ManualInjectResultCode::Success;
    DWORD win32Error = 0;
    DWORD remoteModule = 0;
};

struct ManualProcessInfo {
    DWORD pid = 0;
    DWORD parentPid = 0;
    std::wstring exeName;
    std::wstring exePath;
};

DWORD ManualInjectorParsePid(const wchar_t* text);
bool ManualInjectorIsAbsolutePath(const wchar_t* path);
bool ManualInjectorFileExists(const wchar_t* path);
std::wstring ManualInjectorDefaultWorkingDirectory(const wchar_t* exePath);
std::vector<ManualProcessInfo> ManualInjectorListProcesses();
const wchar_t* ManualInjectorResultCodeName(ManualInjectResultCode code);
std::wstring ManualInjectorFormatResult(const ManualInjectResult& result);
ManualInjectResult ManualInjectDll(DWORD pid, const wchar_t* dllPath);
ManualInjectResult ManualLaunchAndInject(
    const wchar_t* exePath,
    const wchar_t* arguments,
    const wchar_t* workingDirectory,
    const wchar_t* dllPath,
    DWORD injectDelayMs,
    DWORD* launchedPid);
