#pragma once
// Process creation + remote LoadLibrary injection. UI-independent.

#include <QString>
#include <windows.h>

struct InjectOutcome {
    enum Kind { Success, NeedElevation, Fail } kind = Fail;
    QString message;
    DWORD win32Error = 0;
};

// Runs on a worker thread. Uses global g_dllPath / CleanupDll().
InjectOutcome PerformInjection(const QString& targetPath);

// Clears INJECTOR_* secrets from the current process environment.
void ClearInjectedSecrets();

// Sets INJECTOR_* env vars so the child inherits them at CreateProcess.
void SetInjectionEnv(
    const QString& model,
    const QString& baseUrl,
    const QString& apiKey,
    const QString& sessionToken,
    const QString& machineId,
    const QString& cardCode);
