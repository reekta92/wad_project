#!/usr/bin/env python3
"""Run project commands with the private local environment, without a shell."""

import argparse
import os
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[1]
COMMANDS = {
    "server": ("backend", ["go", "run", "./cmd/server"]),
    "migrate": ("backend", ["go", "run", "./cmd/migrate"]),
    "seed": ("backend", ["go", "run", "./cmd/seed"]),
    "provision": ("backend", ["go", "run", "./cmd/provision"]),
    "test": ("backend", ["go", "test", "-race", "-count=1", "./..."]),
    "web": ("frontend", ["npm", "run", "dev", "--", "--host", "localhost"]),
}


def environment(root: Path, inherited: dict) -> dict:
    result = inherited.copy()
    source = root / ".env"
    if not source.exists():
        raise ValueError("Missing .env; run python3 scripts/setup.py first.")
    for line in source.read_text().splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        name, separator, value = line.partition("=")
        if not separator or not re.fullmatch(r"[A-Z][A-Z0-9_]*", name):
            raise ValueError("Invalid .env assignment; use literal NAME=value lines.")
        result.setdefault(name, value)
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=COMMANDS)
    args = parser.parse_args()
    directory, command = COMMANDS[args.command]
    try:
        env = environment(ROOT, os.environ)
        raise SystemExit(subprocess.call(command, cwd=ROOT / directory, env=env))
    except (ValueError, FileNotFoundError) as error:
        parser.exit(1, f"{error}\n")
