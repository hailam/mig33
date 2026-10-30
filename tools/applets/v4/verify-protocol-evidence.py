#!/usr/bin/env python3
"""Reproduce original v4 disassembly and serializer evidence without networking.

Only the verified original ay/bj classes are executed. No MIDlet is started and
no analysis-only transformed JAR is used. Outputs stay under ignored directories.
This verifies evidence provenance and fixtures; it does not prove every catalog
interpretation or historical server behavior.
"""

import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
from zipfile import ZipFile


ROOT = Path(__file__).resolve().parents[3]
V4 = ROOT / "research/applets/v4"
ORIGINAL_SHA256 = "dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c"


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def relative(path):
    return str(path.relative_to(ROOT))


def java_tool(name, java_home):
    roots = [java_home, os.environ.get("JAVA_HOME"), "/opt/homebrew/opt/openjdk@21"]
    for directory in roots:
        if directory:
            candidate = Path(directory) / "bin" / name
            if candidate.is_file():
                return str(candidate)
    candidate = shutil.which(name)
    if candidate:
        return candidate
    raise RuntimeError(f"{name} not found; pass --java-home for an installed JDK")


def run(command, **kwargs):
    return subprocess.run(command, cwd=ROOT, capture_output=True, check=True, **kwargs)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--java-home", help="Installed JDK directory")
    parser.add_argument("--skip-disassembly", action="store_true", help="Only reproduce serializer fixtures")
    parser.add_argument("--output", type=Path, help="JSON result path; defaults to ignored data/protocol-audit/reproduction.json")
    args = parser.parse_args()

    jar = V4 / "artifacts/original/mig33-4.62.jar"
    if sha256(jar) != ORIGINAL_SHA256:
        raise RuntimeError("Original JAR checksum does not match the preserved 4.62 build")
    java = java_tool("java", args.java_home)
    javac = java_tool("javac", args.java_home)
    javap = java_tool("javap", args.java_home)
    output = ROOT / "data/protocol-audit"
    output.mkdir(parents=True, exist_ok=True)

    report = {
        "scope": "Original JAR provenance, whole-JAR disassembly, original serializer fixtures and offline codec checks; no MIDlet/network or server-semantic validation",
        "original_jar": {"path": relative(jar), "sha256": sha256(jar), "bytes": jar.stat().st_size},
        "java_version": run([java, "-version"], text=True).stderr.strip(),
        "javac_version": run([javac, "-version"], text=True).stdout.strip(),
        "javap_version": run([javap, "-version"], text=True).stdout.strip(),
        "catalog_inputs": [],
    }
    for path in sorted((V4 / "docs/data").glob("protocol-*.json")):
        if path.name.startswith("protocol-audit"):
            continue
        report["catalog_inputs"].append({"path": relative(path), "sha256": sha256(path)})

    if not args.skip_disassembly:
        destination = V4 / "artifacts/bytecode"
        destination.mkdir(parents=True, exist_ok=True)
        with ZipFile(jar) as archive:
            classes = sorted(entry[:-6].replace("/", ".") for entry in archive.namelist() if entry.endswith(".class"))

        def disassemble(name):
            path = destination / (name + ".txt")
            result = run([javap, "-classpath", str(jar), "-c", "-p", "-s", name])
            path.write_bytes(result.stdout)
            return {"class": name, "path": relative(path), "sha256": sha256(path), "bytes": path.stat().st_size}

        with ThreadPoolExecutor(max_workers=4) as executor:
            report["disassembly"] = list(executor.map(disassemble, classes))
        report["disassembly_arguments"] = ["javap", "-classpath", relative(jar), "-c", "-p", "-s", "<class>"]

        endpoint_rows = json.loads((V4 / "docs/data/protocol-endpoints.json").read_text())
        references = []
        for row in endpoint_rows:
            for reference in row["references"]:
                lines = (ROOT / reference["path"]).read_text().splitlines()
                number = reference["line"]
                references.append({
                    "path": reference["path"], "line": number,
                    "literal": row["literal"],
                    "matches": 0 < number <= len(lines) and row["literal"] in lines[number - 1],
                })
        report["endpoint_line_references"] = {
            "checked": len(references),
            "matched": sum(item["matches"] for item in references),
            "mismatches": [item for item in references if not item["matches"]],
        }

    outgoing = json.loads((V4 / "docs/data/protocol-outgoing.json").read_text())["operations"]
    protocol_notes = (V4 / "docs/protocol.md").read_text()
    outgoing_section = protocol_notes.split("## Outgoing operation catalog", 1)[1].split("### Messages500", 1)[0]
    markdown_rows = {
        int(match[1]): {"purpose": match[2].strip(), "schema_and_flow": match[3].strip()}
        for match in re.finditer(r"^\| (\d+) \| ([^|]*) \| ([^|]*) \|$", outgoing_section, re.MULTILINE)
    }
    differences = []
    for row in outgoing:
        expected_row = {key: row[key] for key in ("purpose", "schema_and_flow")}
        if markdown_rows.get(row["opcode"]) != expected_row:
            differences.append({"opcode": row["opcode"], "json": expected_row, "markdown": markdown_rows.get(row["opcode"])})
    report["outgoing_markdown_consistency"] = {
        "json_rows": len(outgoing), "markdown_rows": len(markdown_rows),
        "opcode_sets_equal": set(markdown_rows) == {row["opcode"] for row in outgoing},
        "differences": differences,
    }

    v3 = {row["opcode"]: row for row in json.loads((ROOT / "research/applets/v3/docs/data/protocol-incoming.json").read_text())}
    v4 = {row["opcode"]: row for row in json.loads((V4 / "docs/data/protocol-incoming.json").read_text())}
    recorded_delta = json.loads((V4 / "docs/data/protocol-version-differences.json").read_text())
    changed = []

    def fields(values):
        return [{"tag": tag, "type": kind} for tag, kind in sorted(values)]

    for opcode in sorted(v3.keys() & v4.keys()):
        before = {(item["tag"], item["type"]) for item in v3[opcode]["direct_field_reads"]}
        after = {(item["tag"], item["type"]) for item in v4[opcode]["direct_fields"]}
        if before != after:
            changed.append({"opcode": opcode, "v3_direct_fields": fields(before), "v4_direct_fields": fields(after), "added_reads": fields(after - before), "removed_reads": fields(before - after)})
    computed_delta = {
        "v3_count": len(v3), "v4_count": len(v4),
        "v4_added_incoming": sorted(v4.keys() - v3.keys()),
        "v4_removed_incoming": sorted(v3.keys() - v4.keys()),
        "shared_count": len(v3.keys() & v4.keys()), "changed_direct_reads": changed,
    }
    report["version_comparison_consistency"] = {
        "scope": "Comparison against the existing v3/v4 catalogs, not a fresh original-v3 source audit",
        "matches": all(recorded_delta[key] == value for key, value in computed_delta.items()),
        "shared_opcodes": computed_delta["shared_count"], "changed_field_sets": len(changed),
    }

    emulator = ROOT / "tools/applets/bin/microemulator-2.0.4.jar"
    manifest = json.loads((ROOT / "tools/applets/bin/manifest.json").read_text())
    expected = next(tool for tool in manifest["tools"] if tool["file"] == emulator.name)
    if sha256(emulator) != expected["sha256"] or emulator.stat().st_size != expected["bytes"]:
        raise RuntimeError("MicroEmulator checksum/length does not match the pinned tool manifest")
    report["microemulator"] = {"path": relative(emulator), "sha256": sha256(emulator)}
    classes = output / "fixture-classes"
    classes.mkdir(parents=True, exist_ok=True)
    harness = ROOT / "tools/applets/v4/protocol-fixture-harness.java"
    classpath = os.pathsep.join(map(str, [jar, emulator]))
    run([javac, "-cp", classpath, "-d", str(classes), str(harness)])
    fixture_output = run([
        java, "-Djava.awt.headless=true", "-cp",
        os.pathsep.join([str(classes), classpath]), "ProtocolV4FixtureHarness",
    ], text=True).stdout
    (output / "original-fixtures.txt").write_text(fixture_output)
    actual = {}
    for line in fixture_output.splitlines():
        name, separator, value = line.partition("=")
        if not separator or name in actual:
            raise RuntimeError(f"Unexpected or duplicate fixture output: {line!r}")
        actual[name] = value

    corpus = V4 / "docs/data/protocol-fixtures.json"
    expected_packets = json.loads(corpus.read_text())["packets"]
    if set(actual) != {packet["name"] for packet in expected_packets}:
        raise RuntimeError("Original harness and recorded fixture names differ")
    module_spec = importlib.util.spec_from_file_location("v4_codec", ROOT / "tools/applets/v4/protocol-codec.py")
    codec = importlib.util.module_from_spec(module_spec)
    module_spec.loader.exec_module(codec)
    report["serializer_fixtures"] = []
    for packet in expected_packets:
        generated = bytes.fromhex(actual[packet["name"]])
        recorded = bytes.fromhex(packet["original_java_hex"])
        report["serializer_fixtures"].append({
            "name": packet["name"], "opcode": packet["opcode"],
            "byte_exact": generated == recorded,
            "decoded_equal": codec.decode_many(generated) == codec.decode_many(recorded),
        })
    go_copy = ROOT / "internal/protocol/testdata/original_v4_packets.json"
    report["fixture_copies"] = {
        "research_sha256": sha256(corpus), "go_sha256": sha256(go_copy),
        "byte_identical": corpus.read_bytes() == go_copy.read_bytes(),
    }
    report["harness_sha256"] = sha256(harness)
    report["python_codec_self_test"] = run([
        sys.executable, str(ROOT / "tools/applets/v4/protocol-codec.py"), "--self-test",
    ], text=True).stdout.strip()
    report["passed"] = (
        all(item["byte_exact"] and item["decoded_equal"] for item in report["serializer_fixtures"])
        and report["fixture_copies"]["byte_identical"]
        and report["outgoing_markdown_consistency"]["opcode_sets_equal"]
        and not differences
        and report["version_comparison_consistency"]["matches"]
        and not report.get("endpoint_line_references", {}).get("mismatches", [])
    )
    result = args.output or output / "reproduction.json"
    if not result.is_absolute():
        result = ROOT / result
    result.parent.mkdir(parents=True, exist_ok=True)
    result.write_text(json.dumps(report, indent=2) + "\n")
    print(f"Verified original JAR; {len(report.get('disassembly', []))} classes disassembled")
    print(f"Reproduced {len(expected_packets)} original serializer fixtures; exact matches: {sum(item['byte_exact'] for item in report['serializer_fixtures'])}")
    print(report["python_codec_self_test"])
    print(f"Outgoing Markdown/JSON description differences: {len(differences)}")
    print(f"Report: {relative(result)}")
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
