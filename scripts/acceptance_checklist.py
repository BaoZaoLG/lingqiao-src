"""
Manual / scripted acceptance checklist for Lingqiao client + hook.

Run after deploying CX_LingQiao.exe + CefHook.dll as a pair.

Usage:
  python scripts/acceptance_checklist.py
"""
from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
EXE = ROOT / "client" / "build" / "src" / "Release" / "CX_LingQiao.exe"
DLL = ROOT / "client" / "build" / "src" / "Release" / "CefHook.dll"
AUTOEXAM = ROOT / "AutoExam.js"

CHECKS = [
    "1. Build artifacts exist (CX_LingQiao.exe + CefHook.dll) and have similar timestamps",
    "2. Deploy BOTH files together (old DLL + new EXE will fail secret-channel load)",
    "3. Launch CX_LingQiao, activate a card",
    "4. Confirm status shows core-component ready (DLL download OK)",
    "5. Select target .exe, click 启动注入",
    "6. Injector should EXIT after successful inject (not stay in tray)",
    "7. In target app page with .questionLi, press Tab (enable) if needed",
    "8. Middle-click once → cursor wait → second middle-click fills answer",
    "9. Press F3 to toggle cursor feedback; Tab toggles AutoExam",
    "10. page.reportQuestionState should appear in DebugView as [BRIDGE:PAGE]",
    "11. After a few answers, diagnostics.getHookState should show exam_uploaded > 0",
    "12. Server exam_records table / admin exam page should show new rows",
    "13. API key must NOT appear in page: window.__AUTOEXAM_AI_CONFIG has no apiKey",
    "14. Right-click model combo / chat view → light theme menu (not black)",
]


def main() -> int:
    missing = []
    if not EXE.is_file():
        missing.append(str(EXE))
    if not DLL.is_file():
        missing.append(str(DLL))
    if not AUTOEXAM.is_file():
        missing.append(str(AUTOEXAM))
    if missing:
        print("ARTIFACTS MISSING")
        for m in missing:
            print(" ", m)
        return 1

    print("ARTIFACTS OK")
    print("Manual acceptance steps:")
    for c in CHECKS:
        print(" ", c)
    print("\nAutomated checks:")
    rc = 0
    for script in (
        ROOT / "scripts" / "check_admin_api_paths.py",
        ROOT / "scripts" / "check_autoexam_package.py",
    ):
        import subprocess

        r = subprocess.run([sys.executable, str(script)], cwd=ROOT)
        if r.returncode != 0:
            rc = 1
    return rc


if __name__ == "__main__":
    sys.exit(main())
