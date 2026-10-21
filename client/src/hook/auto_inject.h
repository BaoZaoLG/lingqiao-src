#pragma once
/* ============================================================================
 * auto_inject.h — Automatic renderer process injection
 *
 * When CefHook.dll is loaded in the CEF browser process, this module hooks
 * CreateProcessW to detect child renderer process creation and automatically
 * inject CefHook.dll into the new renderer via remote LoadLibraryW.
 *
 * Usage:
 *   - Call InstallAutoInjectHook() from the browser process ThreadProc
 *   - The hook persists for the lifetime of the browser process
 *   - Call UninstallAutoInjectHook() during CefHookShutdown() before FreeLibrary
 * ========================================================================= */

/* Install CreateProcessW hook for auto-injection into renderer processes.
 * Must be called from the browser process only.
 * Returns true on success. */
bool InstallAutoInjectHook();

/* Remove the CreateProcessW hook. Safe to call even if not installed. */
void UninstallAutoInjectHook();
