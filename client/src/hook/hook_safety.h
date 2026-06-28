#pragma once

#include <stddef.h>
#include <stdint.h>
#include <windows.h>

#include "include/capi/cef_client_capi.h"

inline size_t CefClientHandlerTableOffset(void) {
    return offsetof(cef_client_t, get_audio_handler);
}

inline bool IsInvalidCodePointer(PVOID value) {
    UINT_PTR raw = (UINT_PTR)value;
    return raw == 0 || raw == (UINT_PTR)-1;
}

inline bool DllPlaintextLengthFromEncryptedSize(DWORD encryptedLen, DWORD* plainLen) {
    const DWORD kIvLen = 12;
    const DWORD kTagLen = 16;
    if (!plainLen || encryptedLen < kIvLen + kTagLen) return false;
    *plainLen = encryptedLen - kIvLen - kTagLen;
    return true;
}
