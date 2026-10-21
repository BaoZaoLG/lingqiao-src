"""
Verify AutoExam package single-source consistency.

Source of truth: packages/autoexam/src/autoexam.js
Packaged copies: packages/autoexam/src/autoexam.js, dist/autoexam.bundle.js
Manifest: packages/autoexam/dist/autoexam.manifest.json (sha256, size)

Run:  python scripts/check_autoexam_package.py
"""
from __future__ import annotations

import hashlib
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "packages" / "autoexam" / "src" / "autoexam.js"
SRC_COPY = ROOT / "packages" / "autoexam" / "src" / "autoexam.js"
DIST = ROOT / "packages" / "autoexam" / "dist" / "autoexam.bundle.js"
MANIFEST = ROOT / "packages" / "autoexam" / "dist" / "autoexam.manifest.json"


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def main() -> int:
    errors: list[str] = []

    if not SOURCE.is_file():
        print(f"FAIL: missing source {SOURCE}")
        return 1

    source_bytes = SOURCE.read_bytes()
    source_hash = sha256_bytes(source_bytes)

    for path in (SRC_COPY, DIST):
        if not path.is_file():
            errors.append(f"missing {path.relative_to(ROOT)}")
            continue
        if path.read_bytes() != source_bytes:
            errors.append(f"content mismatch {path.relative_to(ROOT)}")

    if not MANIFEST.is_file():
        errors.append(f"missing {MANIFEST.relative_to(ROOT)}")
    else:
        try:
            manifest = json.loads(MANIFEST.read_text(encoding="utf-8-sig"))
        except json.JSONDecodeError as exc:
            errors.append(f"invalid manifest json: {exc}")
        else:
            if manifest.get("sha256", "").lower() != source_hash:
                errors.append(
                    f"manifest sha256 {manifest.get('sha256')} != source {source_hash}"
                )
            if int(manifest.get("size", -1)) != len(source_bytes):
                errors.append(
                    f"manifest size {manifest.get('size')} != source {len(source_bytes)}"
                )

    if errors:
        print("AUTOEXAM PACKAGE FAIL")
        for e in errors:
            print(" ", e)
        print("Fix: powershell -File packages/autoexam/build.ps1")
        return 1

    print(f"AUTOEXAM PACKAGE OK sha256={source_hash} size={len(source_bytes)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
