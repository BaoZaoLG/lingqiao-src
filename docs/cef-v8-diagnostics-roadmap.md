# CEF / V8 Diagnostics Research Roadmap

This document summarizes the current research direction for building a
diagnostic-oriented CEF/V8 runtime analysis tool. The scope is limited to
authorized local debugging, process-role identification, signature validation,
hook health checks, and stability testing.

The project intentionally separates diagnostic capabilities from content
collection or bypass behavior. The primary goal is to understand where V8 runs
inside a CEF multi-process application and how to verify that internal V8
anchors are correctly located and reached.

## 1. Background

CEF applications normally run as multiple processes. A single executable may
spawn different process roles, such as:

- `browser`: the main process, usually without a `--type=` argument.
- `renderer`: the process that hosts web pages and runs V8 JavaScript.
- `gpu-process`: the process responsible for GPU / ANGLE / graphics work.
- `utility`: helper process used for services such as networking.

For V8 diagnostics, the important target is the renderer process:

```text
--type=renderer
```

Installing V8 hooks in the browser, GPU, or utility process can succeed at the
binary level, but those hooks will not necessarily receive JavaScript-related
calls because the page's V8 engine is hosted by the renderer.

## 2. Current Findings

The research confirmed the following chain:

```text
Find V8 signatures in libcef.dll
-> resolve unique runtime addresses
-> inject diagnostic DLL into renderer process
-> attach V8 hook stubs
-> observe V8 hook hits
```

The strongest observed V8 anchors are:

```text
v8.script_compiler.compile_script.func
v8.script_run.xref
v8.function_template.set_call_handler.func
v8.function_template.new.func
v8.function_template.new.alt_func
```

The most important compile-path anchor is:

```text
v8.script_compiler.compile_script.func
```

This anchor is associated with V8 script compilation behavior and is the best
candidate for later non-invasive parameter diagnostics.

## 3. Diagnostic Scope

Recommended diagnostic features:

- Identify current process role from the command line.
- Log process identity at DLL startup.
- Scan `libcef.dll` executable sections for known V8 byte signatures.
- Report signature match counts.
- Attach hook stubs only when every required signature is unique.
- Count hook hits per V8 anchor.
- Limit per-hook logging to avoid noisy output.
- Record only pointer-level argument snapshots during early research.

Out of scope:

- Capturing private page content.
- Extracting protected data.
- Modifying JavaScript before compilation.
- Bypassing application, platform, or exam restrictions.
- Hiding modules, privilege escalation, or stealth injection.

## 4. Process Role Identification

The first diagnostic step is to log which CEF process received the DLL.

Example output:

```text
[CefHook] process pid=1234 ppid=5678 role=renderer exe="C:\App\App.exe" cmd="C:\App\App.exe --type=renderer ..."
```

Only the renderer process should be used for V8 page-runtime diagnostics.

### Role Inference Template

```cpp
std::wstring InferCefRoleFromCommandLine(const std::wstring& commandLine) {
    const std::wstring marker = L"--type=";
    size_t pos = commandLine.find(marker);
    if (pos == std::wstring::npos) {
        return L"browser";
    }

    pos += marker.size();
    size_t end = commandLine.find_first_of(L" \t\r\n\"", pos);
    if (end == std::wstring::npos) {
        end = commandLine.size();
    }

    if (end <= pos) {
        return L"other";
    }

    return commandLine.substr(pos, end - pos);
}
```

### Process Identity Log Template

```cpp
void LogCurrentProcessIdentity() {
    DWORD pid = GetCurrentProcessId();

    wchar_t exePath[MAX_PATH * 4]{};
    GetModuleFileNameW(nullptr, exePath, _countof(exePath));

    std::wstring commandLine = GetCommandLineW();
    std::wstring role = InferCefRoleFromCommandLine(commandLine);

    wchar_t log[4096]{};
    swprintf_s(
        log,
        L"[CefHook] process pid=%lu role=%s exe=\"%s\" cmd=\"%s\"\n",
        pid,
        role.c_str(),
        exePath,
        commandLine.c_str());

    OutputDebugStringW(log);
}
```

## 5. Signature Scanning Strategy

V8 internal functions vary across CEF / Chromium builds. Static addresses should
not be hard-coded. Instead, use byte signatures with wildcarded operands.

Recommended scanning behavior:

- Locate `libcef.dll` in the current process.
- Parse the PE headers.
- Scan only executable sections.
- Support IDA-style wildcard patterns.
- Report zero, one, or multiple matches.
- Attach hooks only when the intended signature has exactly one match.

Example diagnostic output:

```text
[V8Sig] scanning libcef.dll base=0x50000000 signatures=5
[V8Sig] hit name=v8.script_compiler.compile_script.func count=1 rva=0x00DF2150 va=0x50DF2150
```

## 6. Hook Health Checks

Initial V8 hooks should be attach-only and diagnostic-only. The first milestone
is proving that the hook is reached and returns safely.

Recommended output:

```text
[V8Hook] attached name=v8.script_compiler.compile_script.func target=0x50DF2150 trampoline=0x12340F80
[V8Hook] hit name=v8.script_compiler.compile_script.func count=1
```

### Hit Counter Template

```cpp
static LONG g_v8HookHits[5] = {};
static const LONG kMaxLoggedHitsPerHook = 20;

extern "C" void __cdecl LogV8HookHit(int index) {
    if (index < 0 || index >= 5) {
        return;
    }

    LONG count = InterlockedIncrement(&g_v8HookHits[index]);
    if (count > kMaxLoggedHitsPerHook) {
        return;
    }

    char log[256]{};
    sprintf_s(
        log,
        "[V8Hook] hit index=%d count=%ld",
        index,
        count);

    OutputDebugStringA(log);
    OutputDebugStringA("\n");
}
```

## 7. Compile-Path Argument Diagnostics

After `v8.script_compiler.compile_script.func` is confirmed to hit in the
renderer process, the next safe research step is pointer-level argument logging.

Do not deeply dereference V8 internal handles until the argument ownership and
object layout are understood in IDA.

### Pointer Snapshot Template

```cpp
extern "C" void __cdecl LogV8CompileArgs(
    void* a1,
    void* a2,
    void* a3,
    void* a4,
    void* a5) {
    char log[512]{};
    sprintf_s(
        log,
        "[V8Compile] a1=0x%p a2=0x%p a3=0x%p a4=0x%p a5=0x%p",
        a1,
        a2,
        a3,
        a4,
        a5);

    OutputDebugStringA(log);
    OutputDebugStringA("\n");
}
```

The argument snapshot should then be compared against the decompiled compile
function in IDA. The purpose is to identify which values are handles, context
objects, source containers, or metadata objects.

## 8. Recommended Tooling Improvements

### Renderer Selection UI

The injector UI should support:

- Process list refresh.
- Text filtering.
- PID, parent PID, process name, and path display.
- CEF role display.
- Renderer highlighting.
- Optional display of `renderer-client-id`.

The UI should make it difficult to accidentally choose `browser`,
`gpu-process`, or `utility` when the goal is V8 renderer diagnostics.

### Hook Status Panel

Useful fields:

```text
process pid
process role
libcef base
signature name
match count
target VA
trampoline VA
attach status
hit count
```

### Stability Tests

Recommended manual checks:

- Start application.
- Enter a page that creates a renderer process.
- Inject into renderer only.
- Confirm V8 hook attach logs.
- Refresh the page several times.
- Confirm hit counts increase.
- Confirm no renderer crash.
- Repeat with a newly created renderer process.

## 9. Suggested Roadmap

### Phase 1: Reliable Diagnostics

- Add process identity logging.
- Add renderer process filtering in the injector UI.
- Add V8 signature scan logging.
- Add attach and hit counters.

### Phase 2: Safer Runtime Observation

- Add compile-path argument pointer snapshots.
- Correlate pointer snapshots with IDA parameter usage.
- Add crash-safe guardrails around logging.
- Limit output volume.

### Phase 3: Signature Maintenance

- Store signatures in a small catalog.
- Record CEF / libcef version metadata.
- Add match-count regression tests where possible.
- Keep function-entry signatures and reference-site signatures separate.

### Phase 4: Developer UX

- Add a renderer-focused injector view.
- Add status summaries.
- Export diagnostic logs.
- Add troubleshooting hints for common mistakes:
  - wrong process role
  - stale DLL build
  - duplicate signature match
  - hook attach failure

## 10. Key Lessons

- CEF process role matters more than executable name.
- V8 page-runtime hooks must be installed in the renderer process.
- A successful signature scan does not prove the hook is on a hot path.
- A successful hook attach does not prove the function is called.
- A hook hit in `FunctionTemplate` paths proves V8 API activity, but the compile
  path should be validated separately.
- `v8.script_compiler.compile_script.func` is currently the most valuable
  compile-path diagnostic anchor.

## 11. Current Status

Current validated state:

```text
renderer process identified: yes
V8 signatures unique: yes
V8 hooks attached: yes
V8 hooks hit: yes
compile_script hit observed: yes
content extraction implemented: no
JavaScript rewriting implemented: no
```

This is a solid base for authorized CEF/V8 runtime diagnostics and further
reverse-engineering research focused on stability, observability, and signature
maintenance.
