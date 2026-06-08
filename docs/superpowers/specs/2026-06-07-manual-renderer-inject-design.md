# Manual Renderer Injection Design

Goal: add an explicit running-process injection path to the manual GUI injector so V8 hooks can be loaded into the CEF renderer process instead of only the browser process.

Design:
- Keep the existing "launch and inject" flow unchanged.
- Add a process enumeration helper in `manual_injector_core` using Toolhelp32 snapshots.
- Show PID, executable name, and best-effort executable path in the GUI.
- Let the user refresh the list, filter by text, select a row, and inject the selected PID with the chosen local DLL.
- Do not add automatic child-process injection, privilege escalation, hiding, or bypass behavior.

Testing:
- Extend `ManualInjectorCoreTest` with a process enumeration assertion that the current test process appears in the returned list.
- Build `ManualDllInjectorGui` and `ManualInjectorCoreTest`.
