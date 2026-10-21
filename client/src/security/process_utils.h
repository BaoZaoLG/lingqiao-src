#pragma once
// Process inspection and temp-file hygiene (UI-independent).

#include <QString>
#include <QFileInfo>
#include <windows.h>
#include <tlhelp32.h>
#include <psapi.h>

inline bool IsTargetAlreadyInjected(const QString& targetPath) {
    QFileInfo fi(targetPath);
    QString procName = fi.fileName();

    HANDLE snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snap == INVALID_HANDLE_VALUE) return false;

    PROCESSENTRY32W pe = {0};
    pe.dwSize = sizeof(pe);
    bool found = false;

    if (Process32FirstW(snap, &pe)) {
        do {
            if (_wcsicmp(pe.szExeFile, (LPCWSTR)procName.utf16()) != 0) continue;
            HANDLE hProc = OpenProcess(PROCESS_QUERY_INFORMATION | PROCESS_VM_READ, FALSE, pe.th32ProcessID);
            if (!hProc) continue;
            HMODULE mods[1024];
            DWORD cbNeeded = 0;
            if (EnumProcessModules(hProc, mods, sizeof(mods), &cbNeeded)) {
                for (DWORD i = 0; i < cbNeeded / sizeof(HMODULE); i++) {
                    WCHAR modName[MAX_PATH];
                    if (GetModuleFileNameExW(hProc, mods[i], modName, MAX_PATH)) {
                        if (wcsstr(modName, L"CefHook") || wcsstr(modName, L"cefhook")) {
                            found = true;
                            break;
                        }
                    }
                }
            }
            CloseHandle(hProc);
            if (found) break;
        } while (Process32NextW(snap, &pe));
    }
    CloseHandle(snap);
    return found;
}

inline void CleanupOldUpdateTempFiles() {
    WCHAR tempPath[MAX_PATH];
    if (!GetTempPathW(MAX_PATH, tempPath)) return;

    WIN32_FIND_DATAW fd;
    WCHAR pattern[MAX_PATH];
    swprintf_s(pattern, MAX_PATH, L"%s\\*.*", tempPath);
    HANDLE hFind = FindFirstFileW(pattern, &fd);
    if (hFind == INVALID_HANDLE_VALUE) return;

    do {
        if (fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) continue;
        if (!(wcsstr(fd.cFileName, L"update_v") && wcsstr(fd.cFileName, L".exe"))) continue;

        WCHAR fullPath[MAX_PATH];
        swprintf_s(fullPath, MAX_PATH, L"%s\\%s", tempPath, fd.cFileName);

        FILETIME ft;
        SYSTEMTIME st;
        GetSystemTime(&st);
        SystemTimeToFileTime(&st, &ft);
        ULARGE_INTEGER ul1, ul2;
        ul1.LowPart = ft.dwLowDateTime;
        ul1.HighPart = ft.dwHighDateTime;
        ul2.LowPart = fd.ftLastWriteTime.dwLowDateTime;
        ul2.HighPart = fd.ftLastWriteTime.dwHighDateTime;
        // 1 hour in 100ns units
        if ((ul1.QuadPart - ul2.QuadPart) > 36000000000ULL) {
            DeleteFileW(fullPath);
        }
    } while (FindNextFileW(hFind, &fd));
    FindClose(hFind);
}
