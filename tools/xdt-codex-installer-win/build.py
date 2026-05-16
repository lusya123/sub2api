#!/usr/bin/env python3
import base64
import gzip
import hashlib
import os
import pathlib
import subprocess


ROOT = pathlib.Path(__file__).resolve().parents[2]
PAYLOAD = ROOT / "tools/xdt-codex-installer-win/payload/install-codex-win.ps1"
MAIN = ROOT / "tools/xdt-codex-installer-win/main.go"
OUT_DIR = ROOT / "frontend/public/downloads/codex"
OUT_EXE = OUT_DIR / "XueDingToken-Codex-Installer-Windows-x64.exe"
OUT_SHA = OUT_DIR / "XueDingToken-Codex-Installer-Windows-x64.exe.sha256"


def main() -> None:
    script = PAYLOAD.read_bytes()
    payload_sha = hashlib.sha256(script).hexdigest()
    payload_b64 = base64.b64encode(gzip.compress(script, compresslevel=9)).decode("ascii")

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env["GO111MODULE"] = "off"
    env["GOOS"] = "windows"
    env["GOARCH"] = "amd64"
    subprocess.run(
        [
            "go",
            "build",
            "-trimpath",
            "-ldflags",
            f"-s -w -X main.embeddedScriptB64={payload_b64} -X main.payloadSHA256={payload_sha}",
            "-o",
            str(OUT_EXE),
            str(MAIN),
        ],
        cwd=ROOT,
        env=env,
        check=True,
    )
    exe_sha = hashlib.sha256(OUT_EXE.read_bytes()).hexdigest()
    OUT_SHA.write_text(exe_sha + "\n", encoding="ascii")
    print(f"payload_sha256={payload_sha}")
    print(f"exe_sha256={exe_sha}")


if __name__ == "__main__":
    main()
