#!/usr/bin/env python3
"""check_unwired_hook_entries.py — find hook-subsystem entry points that are
declared but never called.

Why this exists
---------------
Twice in one session a feature was fully implemented, compiled, and shipped yet
did nothing, because the one function that starts it was never invoked:

  * BridgeInstallV8Transport()  — resolves the V8 C API. Never called, so the
    whole synchronous JS->C++ transport was dead and every bridge call fell back
    to the console path.
  * AttachV8SignatureHooks()    — attaches all V8 signature hooks. Never called,
    so the script_run detour was missing and BridgeTryInstallFromV8Hook() could
    never install window.__LingQiaoNative.

Neither shows up in a build, a unit test, or a log: the code is simply inert.
This scan lists functions declared in the hook headers whose name appears in
exactly one .cpp — i.e. only at its own definition.

It is a REVIEW REPORT, not a hard gate: a function may legitimately be called
through a pointer, or be part of a deliberate public surface. It exits 0 always
and is meant to be read, so that "declared but never wired" is noticed at review
time rather than in production logs.

Usage:
    python3 scripts/check_unwired_hook_entries.py [--quiet]
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
HOOK_DIR = ROOT / "client" / "src" / "hook"

# Functions that are intentionally not called from this tree.
ALLOWLIST = {
    "BridgeIsV8TransportActive",   # public accessor, currently unused
    "UninstallV8ContextHook",      # teardown hook, no caller yet
    "DetachV8SignatureHooks",      # called internally by the attach helpers
    "V8CompileSamplerStopLogger",
    "HookFindFunction",
    "HookAttach",
    "HookDetach",
}

DECL_RE = re.compile(
    r"^\s*(?!static|inline|typedef|template|return|if|for|while|switch)"
    r"(?:extern\s+)?(?:const\s+)?"
    r"(?:unsigned\s+|signed\s+|struct\s+|class\s+)?"
    r"[A-Za-z_][\w:<>,\s\*&]*?"
    r"\b([A-Z][A-Za-z0-9_]*)\s*\([^;{)]*\)\s*;\s*$"
)

# A call site such as "    return Foo();" would otherwise look like a definition
# ("type then name"), which silently turns a reference into a definition and
# produces a false positive.
STATEMENT_PREFIX = re.compile(
    r"^\s*(return|if|for|while|switch|else|do|case|break|continue|goto)\b")


def declared_functions(header: Path) -> list[str]:
    names = []
    for line in header.read_text(encoding="utf-8", errors="replace").splitlines():
        stripped = line.strip()
        if stripped.startswith("//") or stripped.startswith("*"):
            continue
        m = DECL_RE.match(line)
        if m:
            names.append(m.group(1))
    return names


def main() -> int:
    quiet = "--quiet" in sys.argv
    if not HOOK_DIR.is_dir():
        print("hook directory not found: %s" % HOOK_DIR)
        return 0

    # Look at every .cpp AND .c under client/src, not just hook/: callers live in
    # the test drivers and the Qt sources too, a hook detour is often passed as a
    # bare function pointer (no parentheses), and hook_engine.c is a C file that
    # defines the hook engine itself. Matching only .cpp made the scan blind to it.
    client_src = ROOT / "client" / "src"
    sources = sorted(list(client_src.rglob("*.cpp")) + list(client_src.rglob("*.c")))
    cpp_sources = {
        p: p.read_text(encoding="utf-8", errors="replace")
        for p in sources
        if "vendor" not in p.parts
    }

    findings = []
    for header in sorted(HOOK_DIR.rglob("*.h")):
        for name in declared_functions(header):
            if name in ALLOWLIST:
                continue
            pattern = re.compile(r"\b" + re.escape(name) + r"\b")
            # A "definition line" starts with a return type followed by the name.
            # Calls never look like that (e.g. "if (Foo(x))" or "x = Foo(y);"),
            # and counting definitions separately means an #if/#else pair of
            # definitions does not masquerade as a reference.
            definition = re.compile(
                r"^\s*[A-Za-z_][\w:<>,\s\*&]*\b" + re.escape(name) + r"\s*\(")
            total = 0
            definitions = 0
            first_file = None
            for path, text in cpp_sources.items():
                n = len(pattern.findall(text))
                if not n:
                    continue
                if first_file is None:
                    first_file = path
                total += n
                for line in text.splitlines():
                    if STATEMENT_PREFIX.match(line):
                        continue
                    if definition.match(line):
                        definitions += 1
            if total - definitions == 0:
                findings.append((header.relative_to(ROOT), name,
                                 first_file.relative_to(ROOT)))

    if findings:
        print("Hook entry points declared but never called "
              "(%d) — review each one:" % len(findings))
        for header, name, definition in findings:
            print("  %-38s declared in %s" % (name, header))
            print("  %-38s only definition in %s" % ("", definition))
        print()
        print("A function that must run for a feature to work, but is never")
        print("called, fails silently — no build error, no test failure, no log.")
    elif not quiet:
        print("No unwired hook entry points found.")

    return 0


if __name__ == "__main__":
    sys.exit(main())
