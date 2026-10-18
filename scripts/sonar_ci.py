#!/usr/bin/env python3
"""Run a private, disposable SonarQube analysis in isolated CI and save safe evidence."""

import base64
import zlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
HOST = "http://127.0.0.1:9000"
PROJECT = "wad_project"
SERVER_IMAGE = "sonarqube@sha256:0b3408443fb6a311253de27941b10977c9b977fd53be2265947bfe14161308b8"
SCANNER_IMAGE = "sonarsource/sonar-scanner-cli@sha256:a3f4215076706c95a17a68c19322ee916e40a3acd081a8c1a1e839e0194afa57"


def run():
    if os.environ.get("CI") != "true":
        raise ValueError("This disposable SonarQube runner is restricted to isolated CI.")
    evidence = ROOT / ".local/sonar-evidence"
    evidence.mkdir(parents=True, exist_ok=True)
    password = "admin"

    def api(path, data=None, authenticated=True):
        payload = None if data is None else urllib.parse.urlencode(data).encode()
        request = urllib.request.Request(HOST + path, data=payload)
        if authenticated:
            credential = base64.b64encode(f"admin:{password}".encode()).decode()
            request.add_header("Authorization", "Basic " + credential)
        try:
            with urllib.request.urlopen(request, timeout=20) as response:
                body = response.read()
                return json.loads(body) if body else {}
        except urllib.error.HTTPError as error:
            detail = error.read().decode(errors="replace") if path != "/api/users/change_password" else "credential change failed"
            raise RuntimeError(f"Sonar API {path.split('?')[0]} returned {error.code}: {detail}") from None

    def command(args, **kwargs):
        return subprocess.run(args, cwd=ROOT, check=True, **kwargs)

    try:
        command(["docker", "run", "--detach", "--name", "wad-sonar", "--publish", "127.0.0.1:9000:9000", "--memory", "4g", "--ulimit", "nofile=131072:131072", "--ulimit", "nproc=8192:8192", "--env", "SONAR_ES_BOOTSTRAP_CHECKS_DISABLE=true", SERVER_IMAGE])
        for _ in range(180):
            try:
                if api("/api/system/status", authenticated=False).get("status") == "UP":
                    break
            except (OSError, json.JSONDecodeError):
                pass
            time.sleep(2)
        else:
            raise RuntimeError("Disposable SonarQube did not become ready")
        replacement = "Aa1!" + secrets.token_urlsafe(40)
        api("/api/users/change_password", {"login": "admin", "previousPassword": password, "password": replacement})
        password = replacement
        api("/api/projects/create", {"project": PROJECT, "name": "Faculty Exam Management", "visibility": "private"})
        token = api("/api/user_tokens/generate", {"name": "isolated-ci-analysis", "type": "PROJECT_ANALYSIS_TOKEN", "projectKey": PROJECT})["token"]
        environment = {**os.environ, "SONAR_TOKEN": token}
        # The analysis token is passed by environment, never as a command argument or artifact.
        scan = subprocess.run(["docker", "run", "--rm", "--network", "host", "--user", f"{os.getuid()}:{os.getgid()}", "--volume", f"{ROOT}:/usr/src", "--env", "SONAR_TOKEN", "--env", "SONAR_USER_HOME=/usr/src/.local/sonar-cache", "--env", "SONAR_HOST_URL=" + HOST, SCANNER_IMAGE, "-Dsonar.working.directory=/usr/src/.scannerwork", "-Dsonar.qualitygate.wait=true", "-Dsonar.qualitygate.timeout=300"], env=environment, cwd=ROOT, check=False)
        task = dict(line.split("=", 1) for line in (ROOT / ".scannerwork/report-task.txt").read_text().splitlines() if "=" in line)
        completed = api("/api/ce/task?id=" + urllib.parse.quote(task["ceTaskId"]))["task"]
        analysis = completed["analysisId"]
        report = {
            "commit": command(["git", "rev-parse", "HEAD"], capture_output=True, text=True).stdout.strip(),
            "server": api("/api/system/status", authenticated=False),
            "analysis_id": analysis,
            "quality_gate": api("/api/qualitygates/project_status?analysisId=" + urllib.parse.quote(analysis)),
            "measures": api("/api/measures/component?component=" + PROJECT + "&metricKeys=bugs,vulnerabilities,code_smells,security_hotspots,coverage,duplicated_lines_density,ncloc"),
            "issues": api("/api/issues/search?componentKeys=" + PROJECT + "&ps=500"),
            "hotspots": api("/api/hotspots/search?project=" + PROJECT + "&ps=100"),
        }
        (evidence / "analysis.json").write_text(json.dumps(report, indent=2) + "\n")
        print("WAD_SONAR_REVIEW:" + base64.b64encode(zlib.compress(json.dumps(report).encode())).decode())
        for image in (SERVER_IMAGE, SCANNER_IMAGE):
            command(["docker", "image", "inspect", image, "--format", "{{json .RepoDigests}}"])
        print("SonarQube analysis completed; safe evidence saved.")
        if scan.returncode != 0:
            raise RuntimeError("SonarQube quality gate failed; inspect the saved analysis evidence.")
    finally:
        subprocess.run(["docker", "rm", "--force", "wad-sonar"], cwd=ROOT, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


if __name__ == "__main__":
    run()
