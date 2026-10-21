#!/usr/bin/env python3
"""check_java_artifact.py — verify a staged sidecar JAR actually serves the
Windows client's request contract.

Why this exists
---------------
The client sends ``client_id`` in every body it posts (client/src/ui/impl_chat.h
``buildChatAuthRequest`` and impl_update.h). Go's encoding/json ignores unknown
fields, so those requests always worked against Go. The sidecar used a stock
Jackson mapper, whose ``FAIL_ON_UNKNOWN_PROPERTIES`` default is true, so the same
payload was rejected and the generic catch reported it as INVALID_JSON.

That fix lives in source. ``cutover-to-java-production.sh`` records the JAR's
sha256 but never compares it against an expected value, so an older artifact
deploys silently — and after the cutover there is no Go fallback, so chat and
update checks simply break in production.

This check extracts the real classes from the real JAR and deserialises the
client's exact bodies against them. It tests the artifact, not the source tree.

Usage:
    python3 scripts/check_java_artifact.py path/to/lingqiao-server-java-*.jar

Exit code 0 = the artifact accepts the client contract.
"""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path

RUNNER = """
import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.chat.ClientChatController;
import com.lingqiao.server.client.ClientUpdateController;

public class ArtifactContractCheck {
    private static int failed = 0;

    public static void main(String[] args) {
        ObjectMapper mapper = new ObjectMapper();
        check(mapper, "chat/presence body",
              "{\\"client_id\\":\\"lq-client\\",\\"session_token\\":\\"s\\",\\"machine_id\\":\\"m\\",\\"card\\":\\"c\\"}",
              ClientChatController.ChatRequest.class);
        check(mapper, "update/check body",
              "{\\"client_id\\":\\"lq-client\\",\\"session_token\\":\\"s\\",\\"machine_id\\":\\"m\\",\\"client_version\\":\\"1.0.0\\",\\"channel\\":\\"stable\\"}",
              ClientUpdateController.UpdateCheckRequest.class);
        if (failed > 0) {
            System.out.println("ARTIFACT_CONTRACT_FAILED");
            System.exit(1);
        }
        System.out.println("ARTIFACT_CONTRACT_OK");
    }

    private static void check(ObjectMapper mapper, String label, String body, Class<?> type) {
        try {
            mapper.readValue(body, type);
            System.out.println("PASS  " + label);
        } catch (Exception ex) {
            failed++;
            System.out.println("FAIL  " + label + " -> " + ex.getClass().getSimpleName()
                + ": " + ex.getMessage());
        }
    }
}
"""


def tool(name: str) -> str:
    found = shutil.which(name)
    if not found:
        raise SystemExit("required command is missing: %s" % name)
    return found


def main() -> int:
    if len(sys.argv) < 2:
        print(__doc__)
        return 2
    jar = Path(sys.argv[1])
    if not jar.is_file():
        raise SystemExit("artifact not readable: %s" % jar)

    javac = tool("javac")
    java = tool("java")

    with tempfile.TemporaryDirectory(prefix="lq-jar-check-") as tmp:
        work = Path(tmp)
        with zipfile.ZipFile(jar) as zf:
            names = zf.namelist()
            if not any(n.startswith("BOOT-INF/classes/") for n in names):
                raise SystemExit("not an executable Spring Boot jar: %s" % jar)
            zf.extractall(work)

        classes_dir = work / "BOOT-INF" / "classes"
        lib_dir = work / "BOOT-INF" / "lib"
        if not classes_dir.is_dir():
            raise SystemExit("BOOT-INF/classes is missing from %s" % jar)

        for required in (
            "com/lingqiao/server/chat/ClientChatController$ChatRequest.class",
            "com/lingqiao/server/client/ClientUpdateController$UpdateCheckRequest.class",
        ):
            if not (classes_dir / required).is_file():
                raise SystemExit("artifact does not contain %s" % required)

        # Build a classpath from the fat jar's own dependencies.
        entries = [str(classes_dir)]
        if lib_dir.is_dir():
            entries += sorted(str(p) for p in lib_dir.glob("*.jar"))
        separator = ";" if os.name == "nt" else ":"
        classpath = separator.join(entries)

        runner_src = work / "ArtifactContractCheck.java"
        runner_src.write_text(RUNNER, encoding="utf-8")

        compile_result = subprocess.run(
            [javac, "-nowarn", "-proc:none", "-encoding", "UTF-8",
             "-cp", classpath, "-d", str(work), str(runner_src)],
            capture_output=True, text=True,
        )
        if compile_result.returncode != 0:
            print(compile_result.stdout)
            print(compile_result.stderr, file=sys.stderr)
            raise SystemExit("failed to compile the contract runner")

        run_result = subprocess.run(
            [java, "-cp", classpath + separator + str(work), "ArtifactContractCheck"],
            capture_output=True, text=True,
        )
        sys.stdout.write(run_result.stdout)
        if run_result.stderr.strip():
            sys.stderr.write(run_result.stderr)

        if run_result.returncode != 0:
            print("\nThis artifact predates the unknown-field tolerance fix. Rebuild it "
                  "from the current source before cutting over: an older JAR will make "
                  "chat and update checks fail in production with no Go fallback.",
                  file=sys.stderr)
            return 1

    print("\nJava artifact serves the client contract.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
