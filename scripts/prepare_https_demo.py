#!/usr/bin/env python3
"""Prepare private certificates and exact HTTPS realm redirects in an isolated CI checkout."""

import json
import os
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[1]


def prepare():
    if os.environ.get("CI") != "true":
        raise ValueError("HTTPS demo preparation is restricted to isolated CI checkouts.")
    private = ROOT / ".local"
    private.mkdir(mode=0o700, exist_ok=True)
    private.chmod(0o700)
    directory = private / "https-demo/tls"
    directory.mkdir(parents=True, mode=0o700, exist_ok=True)
    if (directory / "ca.key").exists():
        raise ValueError("Existing HTTPS demo certificates were preserved; use a fresh CI checkout.")

    def openssl(*arguments):
        subprocess.run(["openssl", *arguments], cwd=directory, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)

    previous = os.umask(0o077)
    try:
        openssl("req", "-x509", "-newkey", "rsa:3072", "-nodes", "-keyout", "ca.key", "-out", "ca.crt", "-days", "2", "-subj", "/CN=WAD isolated CI root", "-addext", "basicConstraints=critical,CA:TRUE")
        openssl("req", "-newkey", "rsa:2048", "-nodes", "-keyout", "server.key", "-out", "server.csr", "-subj", "/CN=localhost")
        (directory / "extensions.cnf").write_text("subjectAltName=DNS:localhost,IP:127.0.0.1\nextendedKeyUsage=serverAuth\nkeyUsage=digitalSignature,keyEncipherment\nbasicConstraints=CA:FALSE\n")
        openssl("x509", "-req", "-in", "server.csr", "-CA", "ca.crt", "-CAkey", "ca.key", "-CAcreateserial", "-out", "server.crt", "-days", "2", "-extfile", "extensions.cnf")
        shutil.copyfile(directory / "server.key", directory / "keycloak.key")
        (directory / "keycloak.key").chmod(0o600)
    finally:
        os.umask(previous)
    # Keycloak reads only its separate key copy; Go keeps the runner-owned original.
    subprocess.run(["sudo", "chown", "1000:1000", directory / "keycloak.key"], check=True)
    (directory / "server.crt").chmod(0o644)
    template = ROOT / "infrastructure/exams-realm.json"
    realm = json.loads(template.read_text())
    realm["sslRequired"] = "all"
    client = next(c for c in realm["clients"] if c["clientId"] == "exam-web")
    client["redirectUris"] = ["https://localhost:8443/"]
    client["webOrigins"] = ["https://localhost:8443"]
    client["attributes"]["post.logout.redirect.uris"] = "https://localhost:8443/"
    template.write_text(json.dumps(realm))
    print("Prepared isolated HTTPS origins: app https://localhost:8443; identity https://localhost:8444.")


if __name__ == "__main__":
    prepare()
