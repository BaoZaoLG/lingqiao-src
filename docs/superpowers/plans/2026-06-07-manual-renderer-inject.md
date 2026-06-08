# Manual Renderer Injection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add manual running-process selection and injection to the existing GUI injector.

**Architecture:** Put process enumeration in `manual_injector_core` so it is testable and reusable. Keep GUI logic thin: refresh process rows, filter them, and call the existing `ManualInjectDll(pid, dllPath)`.

**Tech Stack:** C++17/Win32 Toolhelp32 APIs, Qt5 Widgets, existing CMake targets.

---

### Task 1: Process Enumeration Core

**Files:**
- Modify: `src/manual_injector_core.h`
- Modify: `src/manual_injector_core.cpp`
- Modify: `src/manual_injector_core_test.cpp`

- [ ] Add `ManualProcessInfo` and `ManualInjectorListProcesses`.
- [ ] Add a failing test that the current process appears in the process list.
- [ ] Implement enumeration with `CreateToolhelp32Snapshot`, `Process32FirstW`, `Process32NextW`, and best-effort `QueryFullProcessImageNameW`.
- [ ] Run `ManualInjectorCoreTest` and confirm pass.

### Task 2: GUI Running Process Injection

**Files:**
- Modify: `src/manual_dll_injector_gui.cpp`
- Modify: `src/CMakeLists.txt`

- [ ] Add Qt table/filter controls for running processes.
- [ ] Add refresh behavior backed by `ManualInjectorListProcesses`.
- [ ] Add selected PID injection using `ManualInjectDll`.
- [ ] Link any required Windows libraries.
- [ ] Build `ManualDllInjectorGui`.
