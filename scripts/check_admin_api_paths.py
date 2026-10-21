"""
Path parity check: admin/agent frontend API paths vs Java @RequestMapping bases.

Run:  python scripts/check_admin_api_paths.py
Exit 0 = no missing prefixes. Non-zero = mismatch (print details).
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def collect_frontend_paths(ts_path: Path, prefix: str) -> set[str]:
    text = ts_path.read_text(encoding="utf-8")
    return set(re.findall(r"['\`](%s[a-zA-Z0-9_/${}.:-]+)" % re.escape(prefix), text))


def collect_java_bases() -> set[str]:
    bases: set[str] = set()
    java_root = ROOT / "server-java" / "src" / "main" / "java"
    for f in java_root.rglob("*.java"):
        text = f.read_text(encoding="utf-8", errors="ignore")
        for m in re.finditer(r"@RequestMapping\(\s*(\{[^}]+\}|\"[^\"]+\")", text):
            bases.update(re.findall(r"\"([^\"]+)\"", m.group(1)))
    return bases


def frontend_to_java(path: str) -> str:
    if path.startswith("/admin/api/"):
        return "/java/admin/api/" + path[len("/admin/api/") :]
    if path.startswith("/api/"):
        return "/java/agent/api/" + path[len("/api/") :]
    return path


def normalize(path: str) -> str:
    return re.sub(r"\$\{[^}]+\}", "*", path)


def covered(java_path: str, bases: set[str]) -> bool:
    for b in bases:
        if java_path == b or java_path.startswith(b + "/") or b.startswith(java_path):
            return True
        if java_path.startswith(b):
            return True
    return False


def main() -> int:
    admin = collect_frontend_paths(ROOT / "server" / "web" / "admin" / "src" / "main.ts", "/admin/api/")
    agent = collect_frontend_paths(ROOT / "server" / "web" / "agent" / "src" / "main.ts", "/api/")
    bases = collect_java_bases()

    missing: list[tuple[str, str, str]] = []
    for label, paths in (("admin", admin), ("agent", agent)):
        for p in sorted(paths):
            jp = frontend_to_java(normalize(p))
            if not covered(jp, bases):
                missing.append((label, p, jp))

    # Critical aliases that MUST stay dual-mapped (nginx rewrites keep singular form)
    required_java = {
        "/java/admin/api/card",
        "/java/admin/api/cards",
    }
    for req in required_java:
        if req not in bases:
            missing.append(("required-base", req, req))

    if missing:
        print("PATH PARITY FAIL")
        for label, front, java in missing:
            print(f"  [{label}] {front} -> {java}")
        return 1

    print(f"PATH PARITY OK (admin={len(admin)} agent={len(agent)} java_bases={len(bases)})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
