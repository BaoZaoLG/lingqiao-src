# V8 Signature Diagnostics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only V8 signature diagnostics layer to `CefHook.dll` that scans `libcef.dll` and logs known V8 anchor matches without installing new hooks.

**Architecture:** Add a focused pattern scanner module for IDA-style byte patterns, a V8 signature catalog/diagnostics module, and one call from the hook worker after existing CEF hooks initialize. The new code only reads loaded module memory and emits `OutputDebugStringA` diagnostics.

**Tech Stack:** C++17, Win32 PE headers, CMake, MinHook remains unchanged.

---

## File Map

- Create `src/pattern_scan.h`: pattern parser and module scan API.
- Create `src/pattern_scan.cpp`: IDA pattern parsing, PE executable section scan, result formatting.
- Create `src/v8_signatures.h`: public diagnostics entry point.
- Create `src/v8_signatures.cpp`: V8 signature catalog and logging.
- Create `src/pattern_scan_test.cpp`: small console-style test binary for parser/matcher behavior.
- Modify `src/thread.cpp`: call V8 diagnostics after existing hook install attempt.
- Modify `src/CMakeLists.txt`: compile new DLL files and the test executable.

## Task 1: Pattern Scanner Test

- [ ] Add `src/pattern_scan_test.cpp` with tests for `?` wildcard parsing and masked matching.
- [ ] Add a test target to `src/CMakeLists.txt`.
- [ ] Run the test and verify it fails before implementation.

## Task 2: Pattern Scanner Implementation

- [ ] Add `src/pattern_scan.h/.cpp`.
- [ ] Implement `ParseIdaPattern`, `MatchPatternAt`, and executable section scan helpers.
- [ ] Run the pattern scanner test and verify it passes.

## Task 3: V8 Diagnostics

- [ ] Add `src/v8_signatures.h/.cpp`.
- [ ] Register the V8 signatures from the IDA research report.
- [ ] Log missing module, no match, single match, and multiple match cases.

## Task 4: Integration

- [ ] Include `v8_signatures.h` in `src/thread.cpp`.
- [ ] Call `RunV8SignatureDiagnostics()` once after `InstallHook()` completes.
- [ ] Add new files to the `CefHook` target in `src/CMakeLists.txt`.

## Task 5: Verification

- [ ] Build `CefHook`.
- [ ] Run the pattern scanner test.
- [ ] Confirm the diagnostics code does not alter existing hook pointers or detour behavior.
