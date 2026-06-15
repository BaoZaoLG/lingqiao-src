#include "hook_safety.h"

#include "include/capi/cef_client_capi.h"

#include <cstdio>
#include <cstddef>
#include <windows.h>

static int Fail(const char* msg) {
    std::printf("FAIL: %s\n", msg);
    return 1;
}

int main() {
    if (CefClientHandlerTableOffset() != offsetof(cef_client_t, get_audio_handler)) {
        return Fail("CEF client handler table offset should match generated C API layout");
    }

    if (!IsInvalidCodePointer(nullptr)) {
        return Fail("null pointer should be invalid");
    }
    if (!IsInvalidCodePointer((PVOID)(UINT_PTR)-1)) {
        return Fail("all-bits-one pointer should be invalid on this platform");
    }
    if (IsInvalidCodePointer((PVOID)(UINT_PTR)0x10000)) {
        return Fail("ordinary non-null pointer value should not be rejected by sentinel check");
    }

    DWORD plainLen = 0;
    if (!DllPlaintextLengthFromEncryptedSize(12 + 4096 + 16, &plainLen) || plainLen != 4096) {
        return Fail("AES-GCM DLL envelope should report ciphertext length as plaintext length");
    }
    if (DllPlaintextLengthFromEncryptedSize(27, &plainLen)) {
        return Fail("AES-GCM DLL envelope shorter than IV plus tag should be rejected");
    }

    std::printf("PASS: security regression tests\n");
    return 0;
}
