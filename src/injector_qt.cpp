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
        int len = sprintf_s(buf, sizeof(buf),
            "=== LingQiao Crash Report ===\r\n"
            "Time: %s\r\n"
            "Exception: 0x%08X\r\n"
            "Address: 0x%p\r\n"
            "Module: ",
            QDateTime::currentDateTime().toString("yyyy-MM-dd hh:mm:ss").toUtf8().constData(),
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

// ============================================================================
// Entry point
// ============================================================================
int WINAPI WinMain(HINSTANCE, HINSTANCE, LPSTR, int) {
    // Install crash handler for diagnostics
    SetUnhandledExceptionFilter(CrashHandler);

    // Anti-debug: exit silently if debugger detected
    if (IsBeingDebugged()) ExitProcess(0);

    QCoreApplication::setAttribute(Qt::AA_EnableHighDpiScaling);
    QCoreApplication::setAttribute(Qt::AA_UseHighDpiPixmaps);
    QCoreApplication::setAttribute(Qt::AA_DisableWindowContextHelpButton);

    InitSecrets();

    QApplication app(__argc, __argv);
    app.setApplicationName(QString::fromUtf8("\xe7\x81\xb5\xe6\xa1\xa5"));
    app.setApplicationVersion(GetClientVersion());

    InitCommonControls();
    padding::TouchPadding(); // prevent dead-code elimination of padding data
    MainWindow w;
    w.show();
    return app.exec();
}
