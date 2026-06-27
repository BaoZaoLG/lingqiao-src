# V8 JavaScript Engine Signature Research Report

## 1. Overview

This report documents the process and findings of identifying V8 JavaScript engine internal functions within `libcef.dll` (Chromium Embedded Framework) through **byte-pattern scanning** (signature scanning). All patterns have been validated with a **100% hit rate** across the target environment.

The purpose of this research is to establish a reliable, version-independent method of locating V8 engine entry points at runtime, enabling future work such as memory inspection, script execution, and internal data structure analysis.

## 2. Target Environment

| Item | Detail |
| :--- | :--- |
| **Platform** | Windows x86 (32-bit) |
| **Target Module** | `libcef.dll` (Chromium Embedded Framework) |
| **V8 Version** | Dynamically determined at runtime (varies by CEF/Chromium build) |
| **ASLR** | Enabled — all addresses are randomized per process launch |
| **Toolchain** | MSVC (Visual Studio), MinHook, x64dbg |

## 3. What Is a V8 Signature?

V8 is an open-source JavaScript engine developed by the Chromium Project. It compiles JavaScript to native machine code via multiple tiers (Ignition interpreter → TurboFan/Maglev optimizing compilers). When embedded inside a binary like `libcef.dll`, V8's internal symbols are stripped and addresses change with every build.

A **byte-pattern signature** is a sequence of machine-code bytes extracted from the `.text` section of the binary, where:

- **Fixed bytes** represent stable instruction opcodes that rarely change across compiler updates.
- **Wildcard bytes (`?`)** mask out dynamically varying operands such as:
  - Stack canary values
  - Relative offsets (RVA/JMP targets)
  - Global variable pointers (Isolate, Heap, etc.)

By scanning the target module's memory at runtime for these patterns, we can locate the function entry points regardless of ASLR or binary version.

## 4. Identified V8 Signatures

The following five signatures target core V8 execution pipeline functions:

### 4.1 `v8::internal::ScriptCompiler::CompileScript`

```
55 89 E5 53 57 56 83 E4 ? 81 EC ? ? ? ? A1 ? ? ? ? 31 E8 89 84 24 ? ? ? ? A1 ? ? ? ? 85 C0 75 ? E8
```

**Role**: This is the top-level entry point for compiling JavaScript source code into an internal `Script` object. It handles:
- Source string parsing
- Bytecode generation (Ignition)
- Lazy compilation gating

**Hook Potential**: Intercepting this function allows capturing the raw source code of every script before it is compiled.

---

### 4.2 `v8::internal::Script::Run` (cross-reference pattern)

```
68 ? ? ? ? E8 ? ? ? ? 8B 87 ? ? ? ? C7 87 ? ? ? ? ? ? ? ? 8B 97
```

**Role**: This pattern captures a call-site reference to the script execution path. It is part of V8's execution pipeline that takes a compiled `Script` object and runs it within the current `Context`.

**Hook Potential**: Useful for detecting when a script begins execution, and for injecting logic at the boundary between compilation and runtime.

---

### 4.3 `v8::internal::FunctionTemplateInfo::SetCallHandler`

```
55 89 E5 53 57 56 83 EC ? A1 ? ? ? ? 89 CF 31 E8 89 45 ? 8B 01 8B 48 ? F6 C1
```

**Role**: Binds a C++ callback (invocation callback) to a `FunctionTemplate`. This is the mechanism by which the embedding application (e.g., CEF) exposes native functions to JavaScript.

**Hook Potential**: Understanding this function is critical for:
- Tracing how C++ APIs are registered into the JS runtime
- Injecting custom native functions into the V8 context

---

### 4.4 `v8::internal::FunctionTemplateInfo::New`

```
55 89 E5 53 57 56 83 E4 ? 83 EC ? A1 ? ? ? ? 8B 75 ? 8D 54 24 ? 31 E8 89 44 24 ? C7 04 24 ? ? ? ? C7 44 24 ? ? ? ? ? C7 44 24 ? ? ? ? ? C7 44 24 ? ? ? ? ? C7 44 24 ? ? ? ? ? C7 44 24 ? ? ? ? ? C7 44 24 ? ? ? ? ? A1 ? ? ? ? 85 C0 75 ? 8B 8E ? ? ? ? 8B 7D
```

**Role**: Allocates and initializes a new `FunctionTemplateInfo` object on the V8 heap. `FunctionTemplate` is the fundamental building block for creating JavaScript functions backed by C++ code.

**Hook Potential**: Monitoring template creation reveals the full set of native APIs the embedding application registers.

---

### 4.5 `v8::internal::FunctionTemplateInfo::New` (alternate path)

```
55 89 E5 53 57 56 83 E4 ? 83 EC ? A1 ? ? ? ? 89 CB 8B 4D ? 89 D7
```

**Role**: An alternative code path for `FunctionTemplateInfo::New`, likely reached via a different calling convention or optimization tier. Both patterns should resolve to the same logical function.

**Hook Potential**: Same as 4.4; both patterns should be collected for completeness.

## 5. Technical Analysis

### 5.1 Wildcard Byte Categories

| Category | Example | Reason |
| :--- | :--- | :--- |
| **Stack Canary** | `A1 ? ? ? ?` | Load of GS-segment security cookie; value changes per build |
| **Relative Offset** | `E8 ? ? ? ?` | CALL target; changes with every link |
| **Global Pointer** | `8B 87 ? ? ? ?` | Access to Isolate/Heap globals; offsets shift between versions |
| **Immediate Data** | `C7 04 24 ? ? ? ?` | MOV [ESP], imm32; constant data that may vary |

### 5.2 Why x86 (32-bit)?

V8's x86 codegen produces distinct instruction sequences compared to x64. The patterns above are specifically tailored for **32-bit V8 builds**. For x64 targets, a separate set of patterns using REX-prefixed instructions and different calling conventions would be required.

### 5.3 Signature Stability

These patterns were derived from the **function prologues and core logic**, which tend to be more stable than epilogues or hot-loop code. The heavy use of wildcards on operand bytes provides resilience against:
- Compiler flag changes (e.g., `/O1` vs `/O2`)
- Minor source-level refactors in the V8 codebase
- Linker layout changes

However, **major V8 version bumps** (e.g., V8 11.x → 12.x) may alter the instruction sequences enough to require pattern updates.

## 6. Verification Results

All patterns were validated against a live `libcef.dll` process:

| Pattern | Matches | Status |
| :--- | :--- | :--- |
| `v8.script_compiler.compile_script` | 1 | ✅ Hit (Unique) |
| `v8.script_run.xref` | 1 | ✅ Hit (Unique) |
| `v8.function_template.set_call_handler` | 1 | ✅ Hit (Unique) |
| `v8.function_template.new` | 1 | ✅ Hit (Unique) |
| `v8.function_template.new.alt` | 1 | ✅ Hit (Unique) |

- **Hit Rate**: 5/5 (100%)
- **Uniqueness**: All matches returned exactly 1 result (no false positives)
- **Memory Permissions**: All located in `.text` section (Read + Execute)

## 7. Future Research Directions

### 7.1 V8 Heap Object Layout Analysis

Using the located function addresses as anchor points, the next step is to reverse-engineer V8's internal object model:
- **`JSObject`** property storage (in-object properties vs. backing store)
- **`String`** representation (SeqOneByteString vs. SeqTwoByteString vs. ConsString)
- **`FixedArray`** layout for function arguments and closures

### 7.2 Direct V8 API Invocation

By resolving additional V8 API functions (`v8::Isolate::GetCurrent`, `v8::Context::GetCurrent`, `v8::String::NewFromUtf8`), it becomes possible to compile and execute JavaScript directly through V8's C++ API, bypassing higher-level embedding interfaces entirely.

### 7.3 Isolate and Context Discovery

The most critical next step is locating the **`v8::Isolate*`** and **`v8::Context*`** pointers. Strategies include:
- Scanning known global variable slots referenced by the signature functions
- Hooking `v8::Isolate::GetCurrent` if its pattern can be identified
- Tracing the call chain from `compile_script` back to the Isolate argument

### 7.4 Cross-Version Compatibility Module

A robust version-adaptation system should:
1. Read the CEF/Chromium version string from `libcef.dll` resources or exports
2. Select the appropriate pattern set for that version range
3. Validate matches and fall back to heuristic scanning if needed
4. Cache resolved addresses for the session lifetime

### 7.5 x64 Pattern Development

A parallel effort is needed to develop signatures for 64-bit targets, which dominate modern deployments. The x64 instruction encoding differs significantly (REX prefixes, different calling convention with register arguments in RCX/RDX/R8/R9).

## 8. Disclaimer

This research is conducted solely for the purposes of **security research**, **reverse engineering education**, and **authorized software testing**. All techniques described herein are intended to be used in compliance with applicable laws and regulations. Unauthorized use against systems without explicit permission is strictly prohibited.

---

**Date**: 2026-05-30
**Version**: 1.0
