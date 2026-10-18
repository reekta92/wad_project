#!/usr/bin/env python3
"""CI proofs for downloaded XLSX workbooks and isolated application restores."""

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import uuid
import xml.etree.ElementTree as ET
import zipfile

ROOT = Path(__file__).resolve().parents[1]
NS = {"s": "http://schemas.openxmlformats.org/spreadsheetml/2006/main"}


def workbook(path, exams):
    with zipfile.ZipFile(path) as archive:
        book = ET.fromstring(archive.read("xl/workbook.xml"))
        assert [s.attrib["name"] for s in book.findall("s:sheets/s:sheet", NS)] == ["Calendar", "List"]
        sheets = [ET.fromstring(archive.read(f"xl/worksheets/sheet{i}.xml")) for i in (1, 2)]
    rows = []
    for row in sheets[1].findall("s:sheetData/s:row", NS):
        assert not row.findall(".//s:f", NS), "Formula cell in exported data"
        rows.append(["".join(c.itertext()) for c in row.findall("s:c", NS)])
    expected = [[str(e["id"]), str(e["course_id"]), e["course_code"], e["course_name"], str(e["term"]), str(e["department_id"]), e["department"], str(e["session_id"]), e["date"], e["start_time"], e["end_time"], ", ".join(f'{r["name"]} (floor {r["floor"]})' for r in e["rooms"])] for e in exams]
    assert rows[1:] == expected, "Workbook list differs from filtered API/database values"
    calendar_rows = sheets[0].findall("s:sheetData/s:row", NS)
    columns = {"".join(c.itertext()): c.attrib["r"].rstrip("0123456789") for c in calendar_rows[0].findall("s:c", NS)}
    slots = {"".join(r.find("s:c", NS).itertext()): r for r in calendar_rows[1:]}
    for exam in exams:
        row = slots[exam["start_time"] + "–" + exam["end_time"]]
        cell = row.find(f's:c[@r="{columns[exam["date"]]}{row.attrib["r"]}"]', NS)
        assert cell is not None and exam["course_code"] in "".join(cell.itertext())
        assert exam["course_name"] in "".join(cell.itertext())
    print(f"XLSX proof: both sheets agree with {len(exams)} filtered exams.")


def restore(path):
    if os.environ.get("CI") != "true":
        raise ValueError("Restore proof is restricted to isolated CI services; see README for operator restore instructions.")
    database = "wad_restore_" + uuid.uuid4().hex
    base = ["docker", "compose", "exec", "-T", "postgres"]

    def run(args, data=None):
        return subprocess.run(base + args, input=data, cwd=ROOT, capture_output=True, check=True).stdout

    tables = ("departments", "courses", "classrooms", "sessions", "personnel", "app_users", "unavailability_requests", "exams", "exam_rooms", "invigilator_assignments", "audit_logs", "schedule_state", "schema_migrations", "seed_runs")
    pairs = []
    for table in tables:
        order = "name" if table in ("schema_migrations", "seed_runs") else "id"
        pairs.append(f"'{table}', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY {order}), '[]'::jsonb) FROM public.{table} t)")
    pairs.append("'sequences', (SELECT jsonb_agg(jsonb_build_array(sequencename,last_value) ORDER BY sequencename) FROM pg_sequences WHERE schemaname='public')")
    query = "SELECT md5(jsonb_build_object(" + ",".join(pairs) + ")::text)"

    def digest(target):
        return run(["psql", "-U", "postgres", "-d", target, "-At", "-v", "ON_ERROR_STOP=1", "-c", query]).strip()

    before = digest("exams")
    created = False
    try:
        run(["createdb", "-U", "postgres", "--template=template0", "--owner=exam_owner", database])
        created = True
        run(["pg_restore", "-U", "postgres", "--dbname=" + database, "--single-transaction", "--exit-on-error", "--no-owner"], path.read_bytes())
        assert digest(database) == before, "Restored data, sequences, or migration/seed markers differ"
        assert digest("exams") == before, "Restore proof changed its source database"
        print("Restore proof: all 14 application tables, sequences, and constrained relationships match in a new database; source unchanged.")
    finally:
        if created:
            run(["dropdb", "-U", "postgres", "--force", database])


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("kind", choices=("xlsx", "backup"))
    parser.add_argument("path", type=Path)
    args = parser.parse_args()
    if args.kind == "xlsx":
        workbook(args.path, json.load(sys.stdin))
    else:
        restore(args.path)
