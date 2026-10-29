#!/usr/bin/env python3
"""Run original-JAR suites against isolated backend processes and SQLite files.

The publisher JAR is never modified. Isolated harness copies change loopback
health URLs, two oversized room-name fixtures, and a presence assertion's contact
correlation. Every correction is recorded. Evidence includes the exact binary,
its production source manifest, original frames, and screenshots.
"""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import sqlite3
import subprocess
import sys
import tempfile
import time
import urllib.request
import zipfile

REPO = Path(__file__).resolve().parents[4]
SUITES = [
    # name, runner output, expected clients, use actual listener as page origin
    ("pair", "out", 2, False),
    ("group", "group-out", 4, False),
    ("room", "room-out", 3, False),
    ("room-moderation", "room-moderation-out", 3, False),
    ("block", "block-out", 3, False),
    ("contacts", "contact-out", 2, False),
    ("vote-report", "vote-report-out", 5, False),
    ("room-discovery", "room-discovery-out", 3, False),
    ("friend-moderation", "friend-moderation-out", 3, False),
    ("slash", "slash-out", 4, False),
    ("identity", "identity-out", 2, False),
    ("pages", "pages-out", 2, False),
    ("catalog", "catalog-out", 2, True),
    ("formatting", "formatting-out", 2, True),
    ("media", "media-out", 2, True),
    ("mirrors", "mirror-out", 3, False),
    ("reconnect", "reconnect-out", 3, True),
]


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def save(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")


def build_inputs(env):
    output = subprocess.check_output(["go", "list", "-deps", "-json", "./cmd/omni"], cwd=REPO, env=env, text=True)
    decoder, files = json.JSONDecoder(), {REPO / "go.mod", REPO / "go.sum"}
    while output.strip():
        package, end = decoder.raw_decode(output.lstrip())
        output = output.lstrip()[end:]
        directory = Path(package.get("Dir", "/"))
        if not directory.is_relative_to(REPO):
            continue
        for key in ("GoFiles", "CgoFiles", "CFiles", "HFiles", "SFiles", "SysoFiles", "EmbedFiles"):
            files.update(directory / name for name in package.get(key, []))
    return {str(path.relative_to(REPO)): sha(path) for path in sorted(files)}


def prepare_harness(root):
    source = REPO / "tools/applets/v4/integration"
    target = root / "tools/applets/v4/integration"
    target.mkdir(parents=True)
    for pattern in ("*.java", "*.sh"):
        for path in source.glob(pattern):
            shutil.copy2(path, target / path.name)
    shutil.copytree(source / "shims", target / "shims")
    for name in ("tools/applets/v4/shims", "research/applets/v4/artifacts/original", "research/applets/v4/artifacts/extracted", "research/applets/v4/docs/data", "tools/applets/bin"):
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.symlink_to(REPO / name, target_is_directory=True)
    shutil.copytree(REPO / "configs", root / "configs")
    corrections = []
    # Only the external health probe changes; raw 922 URL fields stay intact.
    for path in target.glob("run-*.sh"):
        path.write_text(path.read_text().replace("http://127.0.0.1:8080/healthz", '"http://127.0.0.1:$MIG33_TEST_HTTP_PORT/healthz"'))
    # These older fixtures predate the recovered 15-character creation limit.
    # Keep their space-containing names and uniqueness without changing any
    # Java assertion or weakening production validation.
    for name, old_prefix, prefix in (("room-discovery", "Discovery ", "D "), ("reconnect", "Reconnect ", "R ")):
        path = target / ("run-" + name + ".sh")
        before = "'" + old_prefix + "'+datetime.now(timezone.utc).strftime('%Y%m%d%H%M%S')"
        after = "'" + prefix + "'+datetime.now(timezone.utc).strftime('%y%m%d%H%M%S')"
        text = path.read_text()
        assert text.count(before) == 1, "Room fixture changed; review correction: " + name
        path.write_text(text.replace(before, after))
        corrections.append({"path": str(path.relative_to(root)), "original_sha256": sha(source / path.name), "isolated_sha256": sha(path), "before": before, "after": after, "reason": "Retain space-containing unique test names within unchanged historical 15-character limit; resulting fixture names are 14 characters."})
    path = target / "V4SlashIntegration.java"
    before = '''            if (Connector.count("RX", 404) == 0) return false;
            Object p = packet("RX", 404);
            Integer contact = (Integer) method(p.getClass(), "d", Integer.class, short.class).invoke(p, (short) 1);
            Byte presence = (Byte) method(p.getClass(), "b", Byte.class, short.class).invoke(p, (short) 2);
            return contact != null && contact == id && presence != null && presence == 99;'''
    after = '''            List<String> files = new ArrayList<>();
            synchronized (Connector.events) {
                for (String event : Connector.events) {
                    String[] parts = event.split("\\t");
                    if (parts.length >= 5 && parts[0].equals("RX") && parts[1].equals("404")) files.add(parts[4]);
                }
            }
            Collections.reverse(files);
            Class<?> ay = Class.forName("ay");
            for (String file : files) {
                Object[] packets = (Object[]) method(ay, "a", java.lang.reflect.Array.newInstance(ay, 0).getClass(), byte[].class).invoke(null, Files.readAllBytes(out.resolve(file)));
                for (Object p : packets) {
                    Integer contact = (Integer) method(p.getClass(), "d", Integer.class, short.class).invoke(p, (short) 1);
                    if (contact == null || contact != id) continue;
                    Byte presence = (Byte) method(p.getClass(), "b", Byte.class, short.class).invoke(p, (short) 2);
                    return presence != null && presence == 99;
                }
            }
            return false;'''
    text = path.read_text()
    assert text.count(before) == 1, "Slash presence helper changed; review correction"
    path.write_text(text.replace(before, after))
    corrections.append({"path": str(path.relative_to(root)), "original_sha256": sha(source / path.name), "isolated_sha256": sha(path), "before": before, "after": after, "reason": "Observe latest404 for the requested contact ID rather than globally latest404; preserve logged-out marker and exact tag1 ID/tag2 presence99 requirement."})
    manifest = {str(path.relative_to(root)): sha(path) for path in sorted(target.rglob("*")) if path.is_file()}
    return manifest, corrections


def prepare_resources(root):
    script = REPO / "tools/applets/scripts/prepare_runtime_assets.py"
    output = subprocess.check_output([
        sys.executable, str(script),
        "--jar", str(REPO / "research/applets/v4/artifacts/original/mig33-4.62.jar"),
        "--output", str(root / "data/applets/resources/v4"),
    ], cwd=REPO, text=True)
    extraction = json.loads(output)
    manifest = {str(path.relative_to(root)): sha(path)
                for path in sorted((root / "data/applets/resources/v4").iterdir())}
    return manifest, extraction


def available(port):
    if port in (8080, 9119) or not 1024 <= port <= 65535:
        raise ValueError("Refusing live/default or invalid port: " + str(port))
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", port))


def stop(process, group=False):
    if process is None or process.poll() is not None:
        return
    if group:
        os.killpg(process.pid, signal.SIGTERM)
    else:
        process.terminate()
    try:
        process.wait(timeout=10)
    except subprocess.TimeoutExpired:
        if group:
            os.killpg(process.pid, signal.SIGKILL)
        else:
            process.kill()
        process.wait()


def inspect_clients(folder, expected):
    clients = {}
    for directory in sorted(folder.iterdir()):
        result_file = directory / "result.txt"
        if not directory.is_dir() or not result_file.exists():
            continue
        result = result_file.read_text().strip()
        assert result.startswith("PASS"), result
        events = [line.split("\t") for line in (directory / "events.tsv").read_text().splitlines()]
        logout = [event for event in events if event[:2] == ["TX", "300"]][-1]
        assert any(event[:3] == ["RX", "1", logout[2]] for event in events), "Missing correlated logout: " + str(directory)
        clients[directory.name] = {
            "result": result,
            "frames": len(list(directory.glob("TX-*.bin"))) + len(list(directory.glob("RX-*.bin"))),
            "screenshots": len([path for path in directory.glob("*.png") if path.name != "background-fixture.png"]),
            "logout_ack": True,
        }
    assert len(clients) == expected, (len(clients), expected)
    return clients


def run_suite(binary, root, evidence, env, suite, index):
    name, folder, count, listener_origin = suite
    http_port, tcp_port = 60080 + index, 61080 + index
    available(http_port)
    available(tcp_port)
    origin = "http://127.0.0.1:" + str(http_port if listener_origin else 8080)
    db = root / (name + ".sqlite")
    server_log, runner_log = evidence / (name + "-server.log"), evidence / (name + "-runner.log")
    result = {"binary_sha256": sha(binary), "http_listener": "127.0.0.1:" + str(http_port), "socket_listener": "127.0.0.1:" + str(tcp_port), "advertised_origin": origin, "database": str(db), "started_at": datetime.datetime.now(datetime.timezone.utc).isoformat()}
    server = runner = None
    started = time.monotonic()
    try:
        with server_log.open("w") as log:
            server = subprocess.Popen([str(binary), "-config-dir", str(root / "configs"), "-env", "", "-http", result["http_listener"], "-tcp", result["socket_listener"], "-public-url", origin, "-db", str(db), "-operators", ""], cwd=root, env=env, stdout=log, stderr=subprocess.STDOUT)
        deadline = time.monotonic() + 20
        while True:
            if server.poll() is not None:
                raise RuntimeError("Server exited: " + server_log.read_text())
            try:
                with urllib.request.urlopen("http://" + result["http_listener"] + "/healthz", timeout=1) as response:
                    assert response.status == 200
                break
            except OSError:
                if time.monotonic() > deadline:
                    raise
                time.sleep(.1)
        with runner_log.open("w") as log:
            runner = subprocess.Popen(["sh", "tools/applets/v4/integration/run-" + name + ".sh"], cwd=root, env={**env, "MIG33_TEST_HTTP_PORT": str(http_port), "MIG33_TEST_TCP_PORT": str(tcp_port), "MIG33_TEST_DB": str(db)}, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
            result["exit_code"] = runner.wait(timeout=600)
    except Exception as error:
        result["error"] = str(error)
    finally:
        stop(runner, group=True)
        stop(server)
        output = root / "data/applets/integration" / folder
        if output.exists():
            shutil.copytree(output, evidence / name)
        if db.exists():
            shutil.copy2(db, evidence / (name + ".sqlite"))
    result["duration_seconds"] = round(time.monotonic() - started, 3)
    result["passed"] = False
    if result.get("exit_code") != 0 or "error" in result:
        return result
    try:
        result["clients"] = inspect_clients(evidence / name, count)
        with sqlite3.connect((evidence / (name + ".sqlite")).as_uri() + "?mode=ro", uri=True) as connection:
            assert connection.execute("pragma integrity_check").fetchall() == [("ok",)]
            assert connection.execute("pragma foreign_key_check").fetchall() == []
            result["schema"] = connection.execute("pragma user_version").fetchone()[0]
        result.update(passed=True, integrity="ok", foreign_key_violations=0)
    except Exception as error:
        result["error"] = str(error)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--evidence", type=Path, required=True)
    parser.add_argument("--suites", default="all", help="Comma-separated suite names, or all")
    parser.add_argument("--resume", action="store_true", help="Continue failed/unrun suites with the retained binary and fresh isolated fixtures")
    args = parser.parse_args()
    selected = {suite[0] for suite in SUITES} if args.suites == "all" else set(args.suites.split(","))
    assert selected and selected <= {suite[0] for suite in SUITES}, "Unknown suite"
    assert {"run-" + suite[0] + ".sh" for suite in SUITES} == {path.name for path in (REPO / "tools/applets/v4/integration").glob("run-*.sh")}, "Suite inventory changed; update isolated runner"
    evidence = args.evidence.resolve()
    env = {key: value for key, value in os.environ.items() if not key.startswith("MIG33_")}
    env.setdefault("GOCACHE", "/tmp/mig33-go-build")
    binary = evidence / "omni"
    jar = REPO / "research/applets/v4/artifacts/original/mig33-4.62.jar"
    assert sha(jar) == "dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c"
    if args.resume:
        summary = json.loads((evidence / "evidence-summary.json").read_text())
        source = json.loads((evidence / "source-manifest.json").read_text())
        assert sha(binary) == summary["binary_sha256"] and source == build_inputs(env), "Frozen binary/source changed; start a new run"
        assert sha(jar) == summary["jar_sha256"]
    else:
        evidence.mkdir(parents=True, exist_ok=False)
        source = build_inputs(env)
        with (evidence / "build.log").open("w") as log:
            subprocess.run(["go", "build", "-o", str(binary), "./cmd/omni"], cwd=REPO, env=env, stdout=log, stderr=subprocess.STDOUT, check=True)
        if source != build_inputs(env):
            raise RuntimeError("Production source changed during build; retain evidence and restart after source freeze")
        save(evidence / "source-manifest.json", source)
        with zipfile.ZipFile(evidence / "production-source.zip", "w", zipfile.ZIP_DEFLATED) as archive:
            for name in source:
                archive.write(REPO / name, name)
        (evidence / "binary-metadata.txt").write_text(subprocess.check_output(["go", "version", "-m", str(binary)], env=env, text=True))
        summary = {"started_at": datetime.datetime.now(datetime.timezone.utc).isoformat(), "binary_sha256": sha(binary), "jar_sha256": sha(jar), "source_manifest_sha256": sha(evidence / "source-manifest.json"), "suite_count": len(selected), "suites": {}, "complete": False, "source_unchanged": True}
    root = Path(tempfile.mkdtemp(prefix="mig33-hexagonal-native-"))
    manifest, corrections = prepare_harness(root)
    resources, extraction = prepare_resources(root)
    attempt = len(summary.setdefault("harness_attempts", [])) + 1
    # Preserve the first attempt's manifests, including runs made by older
    # versions of this driver before resumable evidence was introduced.
    while (evidence / ("harness-manifest" + ("" if attempt == 1 else "-" + str(attempt)) + ".json")).exists():
        attempt += 1
    suffix = "" if attempt == 1 else "-" + str(attempt)
    save(evidence / ("harness-manifest" + suffix + ".json"), manifest)
    save(evidence / ("config-manifest" + suffix + ".json"), {str(path.relative_to(root)): sha(path) for path in sorted((root / "configs").rglob("*")) if path.is_file()})
    save(evidence / ("fixture-corrections" + suffix + ".json"), corrections)
    save(evidence / ("resource-manifest" + suffix + ".json"), {"files": resources, "extraction": extraction})
    preparer_name = "resource-preparer" + suffix + ".py"
    shutil.copy2(REPO / "tools/applets/scripts/prepare_runtime_assets.py", evidence / preparer_name)
    runner_name = "acceptance-runner" + suffix + ".py"
    shutil.copy2(Path(__file__), evidence / runner_name)
    summary["harness_attempts"].append({"number": attempt, "temporary_harness": str(root), "runner_sha256": sha(Path(__file__)), "runner_source": runner_name, "manifest": "harness-manifest" + suffix + ".json", "corrections": "fixture-corrections" + suffix + ".json", "resources": "resource-manifest" + suffix + ".json", "resource_preparer": preparer_name, "resource_preparer_sha256": sha(evidence / preparer_name), "started_at": datetime.datetime.now(datetime.timezone.utc).isoformat()})
    summary.setdefault("temporary_harness", str(root))
    save(evidence / "evidence-summary.json", summary)
    for index, suite in enumerate(SUITES):
        if suite[0] not in selected:
            continue
        previous = summary["suites"].get(suite[0])
        if previous and previous.get("passed"):
            continue
        if previous:
            archive = evidence / "failed-attempts" / (suite[0] + "-" + str(attempt - 1))
            archive.mkdir(parents=True, exist_ok=False)
            for name in (suite[0], suite[0] + "-server.log", suite[0] + "-runner.log", suite[0] + ".sqlite"):
                path = evidence / name
                if path.exists():
                    path.rename(archive / name)
            save(archive / "result.json", previous)
        print("START " + suite[0], flush=True)
        result = run_suite(binary, root, evidence, env, suite, index)
        assert result["binary_sha256"] == summary["binary_sha256"], "Frozen binary changed"
        result["harness_attempt"] = attempt
        if previous:
            result["earlier_attempt_evidence"] = str(archive.relative_to(evidence))
        summary["suites"][suite[0]] = result
        save(evidence / "evidence-summary.json", summary)
        print(("PASS " if result["passed"] else "FAIL ") + suite[0] + " " + str(result["duration_seconds"]) + "s", flush=True)
        if not result["passed"]:
            print((evidence / (suite[0] + "-runner.log")).read_text()[-12000:], flush=True)
            return 1
    summary["source_unchanged"] = source == build_inputs(env)
    summary["resources_unchanged"] = resources == {name: sha(root / name) for name in resources}
    if not summary["resources_unchanged"]:
        save(evidence / "evidence-summary.json", summary)
        raise RuntimeError("Runtime resource bytes changed during acceptance")
    summary["complete"] = True
    summary["totals"] = {key: sum(client[key] for suite in summary["suites"].values() for client in suite["clients"].values()) for key in ("frames", "screenshots")}
    summary["totals"]["clients"] = sum(len(suite["clients"]) for suite in summary["suites"].values())
    summary["finished_at"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    save(evidence / "evidence-summary.json", summary)
    print("ALL PASS " + json.dumps(summary["totals"]), flush=True)
    if not summary["source_unchanged"]:
        print("Production source changed after build; binary results remain valid for retained source manifest.", flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
