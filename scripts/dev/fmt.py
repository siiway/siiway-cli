#!/usr/bin/env python3
import shutil
import subprocess
import sys
from pathlib import Path


def get_go_files() -> list[str]:
    if shutil.which("git"):
        try:
            out = subprocess.check_output(["git", "ls-files", "*.go"], text=True)
            files = [line.strip() for line in out.splitlines() if line.strip()]
            if files:
                return files
        except subprocess.SubprocessError:
            pass

    return [str(p) for p in Path(".").rglob("*.go") if p.is_file()]


def main() -> int:
    if shutil.which("gofmt") is None:
        print("gofmt not found in PATH", file=sys.stderr)
        return 1

    files = get_go_files()
    if not files:
        print("No Go files found.")
        return 0

    subprocess.check_call(["gofmt", "-w", *files])
    print("Formatting complete.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
