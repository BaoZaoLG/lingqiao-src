#include "injection_runner.h"

#include <QString>
#include <QDir>
#include <QFileInfo>
#include <string>

#include "dll_extractor.h"
#include "security/secret_channel.h"

static std::wstring g_lastSecretFile;

/* Delete LQ_*.sec files older than 2 hours (injector may exit long before the
 * target finishes loading pages — we must NOT delete the live file). */
static void CleanupStaleSecretFiles() {
    WCHAR tempPath[MAX_PATH];
    if (!GetTempPathW(MAX_PATH, tempPath)) return;
    WIN32_FIND_DATAW fd;
    WCHAR pattern[MAX_PATH];
    swprintf_s(pattern, MAX_PATH, L"%sLQ_*.sec", tempPath);
    HANDLE hFind = FindFirstFileW(pattern, &fd);
    if (hFind == INVALID_HANDLE_VALUE) return;
    FILETIME ftNow;
    GetSystemTimeAsFileTime(&ftNow);
    ULARGE_INTEGER now;
    now.LowPart = ftNow.dwLowDateTime;
    now.HighPart = ftNow.dwHighDateTime;
    do {
        if (fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) continue;
        ULARGE_INTEGER ft;
        ft.LowPart = fd.ftLastWriteTime.dwLowDateTime;
        ft.HighPart = fd.ftLastWriteTime.dwHighDateTime;
        // 2 hours in 100ns units
        if (now.QuadPart > ft.QuadPart && (now.QuadPart - ft.QuadPart) > 72000000000ULL) {
            WCHAR full[MAX_PATH];
            swprintf_s(full, MAX_PATH, L"%s%s", tempPath, fd.cFileName);
            SetFileAttributesW(full, FILE_ATTRIBUTE_NORMAL);
            DeleteFileW(full);
        }
    } while (FindNextFileW(hFind, &fd));
    FindClose(hFind);
}

void ClearInjectedSecrets() {
    // Only clear OUR process env. The child already inherited INJECTOR_SECRET_FILE
    // and still needs the on-disk file until page scripts load (often after we exit).
    LqSecretsClearEnvAndLegacy();
    SetEnvironmentVariableW(LQ_SECRET_FILE_ENV, NULL);
    // Keep g_lastSecretFile for optional diagnostics; do not DeleteFileW here.
}

void SetInjectionEnv(
    const QString& model,
    const QString& baseUrl,
    const QString& apiKey,
    const QString& sessionToken,
    const QString& machineId,
    const QString& cardCode) {
    // Drop any legacy plaintext env first.
    LqSecretsClearEnvAndLegacy();
    CleanupStaleSecretFiles();

    LqSecretBundle bundle;
    bundle.apiKey = apiKey.toStdString();
    bundle.baseUrl = baseUrl.toStdString();
    bundle.model = model.toStdString();
    bundle.sessionToken = sessionToken.toStdString();
    bundle.machineId = machineId.toStdString();
    bundle.cardCode = cardCode.toStdString();

    std::wstring path = WriteSecretFile(bundle);
    bundle.clear();
    if (path.empty()) {
        SetEnvironmentVariableW(LQ_SECRET_FILE_ENV, NULL);
        return;
    }
    g_lastSecretFile = path;
    SetEnvironmentVariableW(LQ_SECRET_FILE_ENV, path.c_str());
}

InjectOutcome PerformInjection(const QString& targetPath) {
    InjectOutcome out;
    PROCESS_INFORMATION pi = {0};
    STARTUPINFOW si = {0};
    si.cb = sizeof(si);

    QString workDir = QFileInfo(targetPath).absolutePath();
    LPCWSTR workDirPtr = workDir.isEmpty() ? NULL : (LPCWSTR)workDir.utf16();

    if (!CreateProcessW((LPCWSTR)targetPath.utf16(), NULL, NULL, NULL, FALSE,
                        CREATE_SUSPENDED, NULL, workDirPtr, &si, &pi)) {
        DWORD err = GetLastError();
        if (err == ERROR_ELEVATION_REQUIRED) {
            out.kind = InjectOutcome::NeedElevation;
            out.win32Error = err;
            return out;
        }
        out.kind = InjectOutcome::Fail;
        out.win32Error = err;
        out.message = QString::fromUtf8("无法启动目标程序 (错误: %1)").arg(err);
        return out;
    }

    size_t cb = (wcslen(g_dllPath) + 1) * sizeof(WCHAR);
    PVOID pRemote = VirtualAllocEx(pi.hProcess, NULL, cb, MEM_COMMIT, PAGE_READWRITE);
    if (!pRemote) {
        TerminateProcess(pi.hProcess, 1);
        CloseHandle(pi.hThread);
        CloseHandle(pi.hProcess);
        out.kind = InjectOutcome::Fail;
        out.message = QString::fromUtf8("内存分配失败");
        return out;
    }
    if (!WriteProcessMemory(pi.hProcess, pRemote, g_dllPath, cb, NULL)) {
        VirtualFreeEx(pi.hProcess, pRemote, 0, MEM_RELEASE);
        TerminateProcess(pi.hProcess, 1);
        CloseHandle(pi.hThread);
        CloseHandle(pi.hProcess);
        out.kind = InjectOutcome::Fail;
        out.win32Error = GetLastError();
        out.message = QString::fromUtf8("内存写入失败 (错误: %1)").arg(out.win32Error);
        return out;
    }

    HMODULE hK32 = GetModuleHandleW(L"kernel32.dll");
    LPTHREAD_START_ROUTINE pLoadLib =
        (LPTHREAD_START_ROUTINE)GetProcAddress(hK32, "LoadLibraryW");
    HANDLE hThread = NULL;

    typedef NTSTATUS(NTAPI * pNtCreateThreadEx)(
        PHANDLE, ACCESS_MASK, PVOID, HANDLE, LPTHREAD_START_ROUTINE,
        PVOID, ULONG, SIZE_T, SIZE_T, SIZE_T, PVOID);
    HMODULE hNtdll = GetModuleHandleW(L"ntdll.dll");
    auto NtCreateThreadEx = hNtdll
        ? (pNtCreateThreadEx)GetProcAddress(hNtdll, "NtCreateThreadEx")
        : NULL;

    if (NtCreateThreadEx) {
        NTSTATUS st = NtCreateThreadEx(
            &hThread, THREAD_ALL_ACCESS, NULL, pi.hProcess, pLoadLib, pRemote,
            0, 0, 0, 0, NULL);
        if (st < 0 || !hThread) {
            hThread = NULL;
        }
    }

    if (!hThread) {
        hThread = CreateRemoteThread(pi.hProcess, NULL, 0, pLoadLib, pRemote, 0, NULL);
    }

    if (!hThread) {
        VirtualFreeEx(pi.hProcess, pRemote, 0, MEM_RELEASE);
        TerminateProcess(pi.hProcess, 1);
        CloseHandle(pi.hThread);
        CloseHandle(pi.hProcess);
        out.kind = InjectOutcome::Fail;
        out.message = QString::fromUtf8("注入失败 — 目标程序可能不兼容");
        return out;
    }

    DWORD waitResult = WaitForSingleObject(hThread, 15000);
    DWORD result = 0;
    if (waitResult == WAIT_OBJECT_0) {
        GetExitCodeThread(hThread, &result);
        VirtualFreeEx(pi.hProcess, pRemote, 0, MEM_RELEASE);
    }
    CloseHandle(hThread);
    ResumeThread(pi.hThread);
    CloseHandle(pi.hThread);
    CloseHandle(pi.hProcess);
    CleanupDll();

    if (result) {
        out.kind = InjectOutcome::Success;
        out.message = QString::fromUtf8("注入成功");
    } else {
        out.kind = InjectOutcome::Fail;
        out.message = QString::fromUtf8("注入失败 — DLL 加载被目标程序拒绝");
    }
    return out;
}
