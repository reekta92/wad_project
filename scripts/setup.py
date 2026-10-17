#!/usr/bin/env python3
"""Generate local credentials once without overwriting an existing environment."""

from pathlib import Path
import os
import secrets

ROOT = Path(__file__).resolve().parents[1]
SECRET_NAMES = {
    "POSTGRES_PASSWORD", "EXAM_OWNER_PASSWORD", "EXAM_ADMIN_PASSWORD",
    "EXAM_INVIGILATOR_PASSWORD", "EXAM_READER_PASSWORD",
    "KEYCLOAK_DB_PASSWORD", "KEYCLOAK_ADMIN_PASSWORD",
}


def create_environment(root: Path) -> bool:
    target = root / ".env"
    lines = []
    for line in (root / ".env.example").read_text().splitlines():
        name, separator, value = line.partition("=")
        if separator and name in SECRET_NAMES:
            line = f"{name}={secrets.token_urlsafe(32)}"
        lines.append(line)
    try:
        descriptor = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    except FileExistsError:
        return False
    with os.fdopen(descriptor, "w") as stream:
        stream.write("\n".join(lines) + "\n")
    return True


if __name__ == "__main__":
    print("Created private .env." if create_environment(ROOT) else "Kept existing .env unchanged.")
