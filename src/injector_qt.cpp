// ============================================================================
// Injector — Qt5 GUI entry point
//
// Architecture: modular headers handle distinct responsibilities.
//   config.h          — XOR-encrypted secrets, server config
//   crypto.h          — HMAC-SHA256, hex encoding, TLS cert pinning
//   http_client.h     — WinHTTP wrapper with signing and pinning
//   machine_fp.h      — hardware-unique machine fingerprint
//   dll_extractor.h   — server-downloaded DLL staging and cleanup
//   workers.h         — background threads (activate, heartbeat)
//   ui/theme.h        — color palette and stylesheet
//   ui/title_bar.h    — frameless window title bar
//   ui/inline_status.h — status indicator widget
//   ui/main_window.h  — primary application window
// ============================================================================

#include <windows.h>
#include <commctrl.h>
#include <dbghelp.h>

#include <QApplication>

// Core modules
#include "antidebug.h"
#include "config.h"
#include "crypto.h"
#include "http_client.h"
#include "machine_fp.h"
#include "dll_extractor.h"
#include "workers.h"

// Binary padding for obfuscation (auto-generated)
#include "padding_data.h"

// UI modules
#include "ui/theme.h"
#include "ui/title_bar.h"
#include "ui/inline_status.h"
#include "ui/main_window.h"

#pragma comment(lib, "winhttp.lib")
#pragma comment(lib, "bcrypt.lib")
#pragma comment(lib, "iphlpapi.lib")
#pragma comment(lib, "comctl32.lib")
#pragma comment(lib, "dwmapi.lib")
#pragma comment(lib, "psapi.lib")
#pragma comment(lib, "crypt32.lib")
#pragma comment(lib, "user32.lib")
#pragma comment(lib, "shell32.lib")
#pragma comment(lib, "gdi32.lib")
#pragma comment(lib, "dbghelp.lib")

// ============================================================================
// Crash handler — log crash info to temp file before exit
// ============================================================================
static LONG WINAPI CrashHandler(EXCEPTION_POINTERS* exInfo) {
    WCHAR logPath[MAX_PATH];
    GetTempPathW(MAX_PATH, logPath);
    wcscat_s(logPath, L"LingQiao_crash.log");

    HANDLE hFile = CreateFileW(logPath, GENERIC_WRITE, 0, NULL,
        CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (hFile != INVALID_HANDLE_VALUE) {
        char buf[4096];
        SYSTEMTIME st;
        GetLocalTime(&st);
        int len = sprintf_s(buf, sizeof(buf),
            "=== LingQiao Crash Report ===\r\n"
            "Time: %04d-%02d-%02d %02d:%02d:%02d\r\n"
            "Exception: 0x%08X\r\n"
            "Address: 0x%p\r\n"
            "Module: ",
            st.wYear, st.wMonth, st.wDay, st.wHour, st.wMinute, st.wSecond,
            exInfo->ExceptionRecord->ExceptionCode,
            exInfo->ExceptionRecord->ExceptionAddress);

        // Try to find which module the crash address belongs to
        HMODULE hMods[1024]; DWORD cbNeeded;
        if (EnumProcessModules(GetCurrentProcess(), hMods, sizeof(hMods), &cbNeeded)) {
            for (DWORD i = 0; i < cbNeeded / sizeof(HMODULE); i++) {
                MODULEINFO mi;
                if (GetModuleInformation(GetCurrentProcess(), hMods[i], &mi, sizeof(mi))) {
                    if (exInfo->ExceptionRecord->ExceptionAddress >= mi.lpBaseOfDll &&
                        exInfo->ExceptionRecord->ExceptionAddress < (BYTE*)mi.lpBaseOfDll + mi.SizeOfImage) {
                        WCHAR modName[MAX_PATH];
                        if (GetModuleFileNameExW(GetCurrentProcess(), hMods[i], modName, MAX_PATH)) {
                            len += sprintf_s(buf + len, sizeof(buf) - len, "%S", modName);
                        }
                        break;
                    }
                }
            }
        }

        len += sprintf_s(buf + len, sizeof(buf) - len,
            "\r\nException Record:\r\n"
            "  Code:   0x%08X\r\n"
            "  Flags:  0x%08X\r\n"
            "  Addr:   0x%p\r\n",
            exInfo->ExceptionRecord->ExceptionCode,
            exInfo->ExceptionRecord->ExceptionFlags,
            exInfo->ExceptionRecord->ExceptionAddress);

        // Write register values
#ifdef _M_AMD64
        len += sprintf_s(buf + len, sizeof(buf) - len,
            "Registers (x64):\r\n"
            "  RAX: 0x%016llX  RBX: 0x%016llX\r\n"
            "  RCX: 0x%016llX  RDX: 0x%016llX\r\n"
            "  RSI: 0x%016llX  RDI: 0x%016llX\r\n"
            "  RIP: 0x%016llX  RSP: 0x%016llX\r\n"
            "  RBP: 0x%016llX\r\n",
            exInfo->ContextRecord->Rax, exInfo->ContextRecord->Rbx,
            exInfo->ContextRecord->Rcx, exInfo->ContextRecord->Rdx,
            exInfo->ContextRecord->Rsi, exInfo->ContextRecord->Rdi,
            exInfo->ContextRecord->Rip, exInfo->ContextRecord->Rsp,
            exInfo->ContextRecord->Rbp);
#else
        len += sprintf_s(buf + len, sizeof(buf) - len,
            "Registers (x86):\r\n"
            "  EAX: 0x%08X  EBX: 0x%08X\r\n"
            "  ECX: 0x%08X  EDX: 0x%08X\r\n"
            "  ESI: 0x%08X  EDI: 0x%08X\r\n"
            "  EIP: 0x%08X  ESP: 0x%08X\r\n"
            "  EBP: 0x%08X\r\n",
            exInfo->ContextRecord->Eax, exInfo->ContextRecord->Ebx,
            exInfo->ContextRecord->Ecx, exInfo->ContextRecord->Edx,
            exInfo->ContextRecord->Esi, exInfo->ContextRecord->Edi,
            exInfo->ContextRecord->Eip, exInfo->ContextRecord->Esp,
            exInfo->ContextRecord->Ebp);
#endif

        DWORD written;
        WriteFile(hFile, buf, (DWORD)strlen(buf), &written, NULL);
        CloseHandle(hFile);
    }

    return EXCEPTION_EXECUTE_HANDLER;
}

static void LogStartupSecurityExit(const char* reason) {
    WCHAR logPath[MAX_PATH];
    GetTempPathW(MAX_PATH, logPath);
    wcscat_s(logPath, L"LingQiao_injector.log");

    HANDLE hFile = CreateFileW(logPath, FILE_APPEND_DATA, FILE_SHARE_READ, NULL,
        OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (hFile == INVALID_HANDLE_VALUE) return;

    char buf[512];
    SYSTEMTIME st;
    GetLocalTime(&st);
    int len = sprintf_s(buf, sizeof(buf),
        "%04d-%02d-%02d %02d:%02d:%02d [SECURITY] %s\r\n",
        st.wYear, st.wMonth, st.wDay, st.wHour, st.wMinute, st.wSecond, reason);
    DWORD written = 0;
    WriteFile(hFile, buf, (DWORD)len, &written, NULL);
    CloseHandle(hFile);
}

// ============================================================================
// Entry point
// ============================================================================
int WINAPI WinMain(HINSTANCE, HINSTANCE, LPSTR, int) {
    // Anti-debug diagnostics are logged only. Do not block the UI on false positives.
    if (IsBeingDebugged()) {
        LogStartupSecurityExit("Anti-debug check triggered during startup; continuing for stability");
    }

    SetProcessDPIAware();
    QCoreApplication::setAttribute(Qt::AA_DisableHighDpiScaling);
    QCoreApplication::setAttribute(Qt::AA_UseHighDpiPixmaps);
    QCoreApplication::setAttribute(Qt::AA_DisableWindowContextHelpButton);

    InitSecrets();

    QApplication app(__argc, __argv);
    app.setApplicationName(QString::fromUtf8("\xe7\x81\xb5\xe6\xa1\xa5"));
    app.setApplicationVersion(GetClientVersion());

    // Install crash handler AFTER Qt init (handler uses QDateTime)
    SetUnhandledExceptionFilter(CrashHandler);

    InitCommonControls();
    padding::TouchPadding();

    // Parse --reinject --target "path" for UAC elevation retry
    QString reinjectTarget;
    QStringList args = app.arguments();
    int reinjectIdx = args.indexOf("--reinject");
    if (reinjectIdx >= 0) {
        int targetIdx = args.indexOf("--target");
        if (targetIdx >= 0 && targetIdx + 1 < args.size())
            reinjectTarget = args.at(targetIdx + 1);
    }

    MainWindow w;
    if (!reinjectTarget.isEmpty()) {
        w.setTargetAndInject(reinjectTarget);
    }
    w.show();
    return app.exec();
}
