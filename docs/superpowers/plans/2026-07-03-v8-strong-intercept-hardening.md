# V8 Strong Intercept Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the V8 native anti-debugger path stable enough to neutralize `debugger` without causing white screens.

**Architecture:** Keep this V8-only. Reduce the hook surface to `ScriptCompiler::CompileScript`, validate stack arguments and V8 string layout before mutating anything, then patch only standalone `debugger` tokens in-place. If validation fails, pass through and auto-disable strong mode instead of blocking scripts or touching unknown memory.

**Tech Stack:** Win32/x86 C++, MinHook, CEF/libcef V8 internals, SEH guarded probes, DebugView logging, CMake/MSBuild.

---

### Task 1: Reduce V8 Hook Surface to Compile-Only

**Files:**
- Modify: `client/src/hook/v8_hooks.h`
- Modify: `client/src/hook/v8_hooks.cpp`
- Modify: `client/src/hook/thread.cpp`

- [ ] **Step 1: Add compile-only attach API**

In `client/src/hook/v8_hooks.h`, add:

```cpp
bool AttachV8CompileHookOnly(void);
```

- [ ] **Step 2: Implement compile-only attach**

In `client/src/hook/v8_hooks.cpp`, add a new attach function that only uses signature index `0`:

```cpp
bool AttachV8CompileHookOnly(void) {
#if defined(_M_IX86)
    if (InterlockedCompareExchange(&g_v8HooksAttached, 1, 0) != 0) {
        LogLine("[HOOK] V8 already attached");
        return true;
    }

    V8InterceptInit();

    V8ResolvedSignature resolved[V8_SIGNATURE_COUNT];
    if (!ResolveV8Signatures(resolved, V8_SIGNATURE_COUNT)) {
        LogLine("[HOOK] V8 compile-only skipped: signatures are not all unique");
        InterlockedExchange(&g_v8HooksAttached, 0);
        return false;
    }

    const size_t i = 0;
    g_v8HookTrampolines[i] = resolved[i].address;
    LONG r = HookAttach(&g_v8HookTrampolines[i], reinterpret_cast<PVOID>(V8Detour0));
    if (r != ERROR_SUCCESS) {
        char msg[256];
        sprintf_s(msg, sizeof(msg), "[HOOK] V8 compile-only attach failed error=%ld", r);
        LogLine(msg);
        DetachV8SignatureHooks();
        return false;
    }

    char msg[256];
    sprintf_s(msg, sizeof(msg), "[HOOK] V8 compile-only attached target=0x%p trampoline=0x%p",
              resolved[i].address, g_v8HookTrampolines[i]);
    LogLine(msg);
    return true;
#else
    OutputDebugStringA("[HOOK] V8 compile-only skipped: x86 only\n");
    return false;
#endif
}
```

- [ ] **Step 3: Use compile-only API from renderer**

In `client/src/hook/thread.cpp`, replace `AttachV8SignatureHooks()` in the renderer V8 branch with:

```cpp
if (AttachV8CompileHookOnly()) {
    OutputDebugStringA("[HOOK] V8 strong anti-debugger compile hook active\n");
} else {
    OutputDebugStringA("[HOOK] V8 strong anti-debugger compile hook unavailable\n");
}
```

Do not call `BridgeInstallV8Transport()` or `InstallV8ContextHook()` from this strong anti-debugger path.

- [ ] **Step 4: Build x86**

Run:

```powershell
cmake --build build --target CefHook --config Release
```

Expected: build succeeds and `C:\Users\Li\Downloads\Lingqiao_src\build\src\Release\CefHook.dll` is produced.

### Task 2: Correct Compile Detour Argument Capture

**Files:**
- Modify: `client/src/hook/v8_hooks.cpp`
- Modify: `client/src/hook/v8_intercept.cpp`

- [ ] **Step 1: Stop relying on ambiguous pushad offsets**

Replace `V8Detour0` with explicit argument capture like the handler detours:

```cpp
extern "C" __declspec(naked) void V8Detour0() {
    __asm {
        pushfd
        pushad
        mov eax, dword ptr [esp + 48]
        push eax
        mov eax, dword ptr [esp + 48]
        push eax
        mov eax, dword ptr [esp + 48]
        push eax
        mov eax, dword ptr [esp + 48]
        push eax
        mov eax, esp
        push eax
        call V8InterceptCompileScript
        add esp, 20
        test eax, eax
        popad
        popfd
        jnz skip_compile
        jmp dword ptr [g_v8HookTrampolines + 0]
    skip_compile:
        xor eax, eax
        ret
    }
}
```

- [ ] **Step 2: Read compile args from frame[0..3]**

In `V8InterceptCompileScript`, replace:

```cpp
void** frame = (void**)esp_frame;
void* arg2 = frame[9];
```

with:

```cpp
void** frame = (void**)esp_frame;
void* arg1 = frame[0];
void* arg2 = frame[1];
void* arg3 = frame[2];
void* arg4 = frame[3];
```

Log all four values for the first 30 calls:

```cpp
if (count <= 30) {
    LogF("[V8ICPT] compile #%ld a1=%p a2=%p a3=%p a4=%p",
         count, arg1, arg2, arg3, arg4);
}
```

- [ ] **Step 3: Build and collect DebugView**

Run:

```powershell
cmake --build build --target CefHook --config Release
```

Expected DebugView after launching target: `compile #N a1=... a2=...` entries appear and the page does not white-screen before any source mutation is enabled.

### Task 3: Add Read-Only V8 Source Layout Probe

**Files:**
- Create: `client/src/hook/v8_source_probe.h`
- Create: `client/src/hook/v8_source_probe.cpp`
- Modify: `client/src/CMakeLists.txt`
- Modify: `client/src/hook/v8_intercept.cpp`

- [ ] **Step 1: Add probe API**

Create `client/src/hook/v8_source_probe.h`:

```cpp
#pragma once
#include <stddef.h>

struct V8SourceProbeResult {
    bool readable;
    int candidateIndex;
    void* sourceHandle;
    size_t previewLen;
    char preview[256];
};

V8SourceProbeResult ProbeV8CompileSource(void* compileArg2);
```

- [ ] **Step 2: Implement candidate offset probe**

Create `client/src/hook/v8_source_probe.cpp` with SEH guarded reads:

```cpp
#include "pch.h"
#include "v8_source_probe.h"

#include <windows.h>
#include <string.h>

static bool SafeReadPtr(void* base, int offset, void** out) {
    __try {
        if (!base || IsBadReadPtr(base, offset + (int)sizeof(void*))) return false;
        *out = *(void**)((char*)base + offset);
        return *out != NULL;
    } __except (EXCEPTION_EXECUTE_HANDLER) {
        return false;
    }
}

V8SourceProbeResult ProbeV8CompileSource(void* compileArg2) {
    V8SourceProbeResult r = {};
    const int offsets[] = { 0, 4, 8, 12, 16, 20, 24, 28, 32 };

    for (int i = 0; i < (int)(sizeof(offsets) / sizeof(offsets[0])); ++i) {
        void* candidate = NULL;
        if (!SafeReadPtr(compileArg2, offsets[i], &candidate)) continue;

        r.sourceHandle = candidate;
        r.candidateIndex = offsets[i];
        r.readable = true;
        sprintf_s(r.preview, sizeof(r.preview), "candidate_handle=%p", candidate);
        r.previewLen = strlen(r.preview);
        return r;
    }

    r.candidateIndex = -1;
    return r;
}
```

This task is intentionally read-only. Do not patch source yet.

- [ ] **Step 3: Add file to CMake**

Add `hook/v8_source_probe.cpp` and `hook/v8_source_probe.h` to the `CefHook` target.

- [ ] **Step 4: Log probe result only**

In `V8InterceptCompileScript`, call:

```cpp
V8SourceProbeResult probe = ProbeV8CompileSource(arg2);
if (count <= 30) {
    LogF("[V8ICPT] source-probe readable=%d offset=%d handle=%p preview=%s",
         probe.readable ? 1 : 0,
         probe.candidateIndex,
         probe.sourceHandle,
         probe.preview);
}
return 0;
```

- [ ] **Step 5: Run target and inspect logs**

Expected: page remains usable. Logs show stable `arg2` and candidate handles. If page white-screens here, the detour ABI is still wrong and patching must not proceed.

### Task 4: Implement Patch Only After Proven Read/Write

**Files:**
- Modify: `client/src/hook/v8_source_probe.cpp`
- Modify: `client/src/hook/v8_intercept.cpp`
- Test: `client/src/test_v8_source_neutralizer.cpp`

- [ ] **Step 1: Keep existing token tests**

Run:

```powershell
build\src\Release\V8SourceNeutralizerTests.exe
```

Expected: `PASS v8 source neutralizer tests`.

- [ ] **Step 2: Replace block behavior with patch-only behavior**

In `V8InterceptCompileScript`, remove any path that returns `1` for `debugger`. Strong mode must never block a script. It must return `0` on success, failure, unreadable source, or no match.

- [ ] **Step 3: Add mutation only for verified source layout**

Only call `ApplyScriptPatches(sourceHandle)` when the probe has proven source readability. If source is unreadable, log and pass through:

```cpp
if (!probe.readable || !probe.sourceHandle) {
    InterlockedIncrement(&g_statsBypassed);
    return 0;
}
```

- [ ] **Step 4: Build x86**

Run:

```powershell
cmake --build build --target CefHook --config Release
```

Expected: build succeeds.

### Task 5: Add Auto-Disable Fuse

**Files:**
- Modify: `client/src/hook/v8_intercept.cpp`
- Modify: `client/src/hook/v8_hooks.h`
- Modify: `client/src/hook/v8_hooks.cpp`

- [ ] **Step 1: Add counters**

In `v8_intercept.cpp`, add:

```cpp
static volatile LONG g_statsUnreadable = 0;
static volatile LONG g_strongModeDisabled = 0;
static const LONG kMaxUnreadableBeforeDisable = 10;
```

- [ ] **Step 2: Add query API**

In `v8_intercept.h`, add:

```cpp
int V8InterceptStrongModeDisabled(void);
```

Implement:

```cpp
int V8InterceptStrongModeDisabled(void) {
    return InterlockedCompareExchange(&g_strongModeDisabled, 0, 0) != 0;
}
```

- [ ] **Step 3: Fuse on repeated unreadable source**

When source is unreadable:

```cpp
LONG unreadable = InterlockedIncrement(&g_statsUnreadable);
if (unreadable >= kMaxUnreadableBeforeDisable) {
    InterlockedExchange(&g_strongModeDisabled, 1);
    OutputDebugStringA("[V8ICPT] strong mode auto-disabled: source unreadable threshold reached\n");
}
return 0;
```

At the top of `V8InterceptCompileScript`:

```cpp
if (V8InterceptStrongModeDisabled()) return 0;
```

- [ ] **Step 4: Build and runtime verify**

Run:

```powershell
cmake --build build --target CefHook --config Release
```

Expected: if source remains unreadable, logs show auto-disable and the page continues instead of white-screening.

### Task 6: Acceptance Criteria

**Files:**
- No new files required.

- [ ] **Step 1: Safe-disabled baseline**

Run target without `LQ_ENABLE_V8_INTERCEPT=1`.

Expected log:

```text
[HOOK] V8 intercept disabled (set LQ_ENABLE_V8_INTERCEPT=1 to enable)
```

Expected behavior: page loads normally.

- [ ] **Step 2: Strong mode attach**

Run target with `LQ_ENABLE_V8_INTERCEPT=1`.

Expected log:

```text
[HOOK] V8 compile-only attached
```

Expected behavior: page does not white-screen.

- [ ] **Step 3: Debugger handling**

Open DevTools and trigger the target anti-debugger path.

Expected: no repeated pause at:

```js
(function anonymous() { debugger })
```

- [ ] **Step 4: Fuse behavior**

If V8 source layout is not readable on this CEF version, expected log:

```text
[V8ICPT] strong mode auto-disabled: source unreadable threshold reached
```

Expected behavior: page remains usable.

---

## Self-Review

Spec coverage: This plan focuses only on the V8 native strong path and excludes JS bootstrap/document-start strategies.

No placeholders: Each task names exact files, exact code snippets, commands, and expected output.

Type consistency: New APIs are named consistently: `AttachV8CompileHookOnly`, `ProbeV8CompileSource`, and `V8InterceptStrongModeDisabled`.
