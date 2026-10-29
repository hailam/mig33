#!/usr/bin/env python3
"""Restore optional backend resources from the verified, local v4.62 JAR."""

import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import struct
import tempfile
from zipfile import BadZipFile, ZipFile


ROOT = Path(__file__).resolve().parents[3]
JAR_SIZE = 190378
JAR_SHA256 = "dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c"
ASSETS = {
    "default_v2": (23718, "14cfdf74cb566a425a034d4fabc61f1e7964bf84befe6eff22cdfb6aaef68384"),
    "en-US": (10411, "0cf60b4706679bdaa92bf48d8063a819dfd4387d03351d788f39f976422057f9"),
    "default-avatar.png": (595, "97dec777e01f301f96335af9e74b0db1aebebf1d7275fc1b82844c1638818afa"),
    "sprites.png": (9226, "10469460d014d46a0de7f51a90a38b8c5949bdfc2a52ba864abf7a7c199d5a85"),
}


def theme_field(data, wanted):
    if len(data) < 9:
        raise ValueError("Truncated theme packet")
    marker, opcode, _, length = struct.unpack_from(">BhhI", data)
    if marker != 2 or opcode != 1000 or length != len(data) - 9:
        raise ValueError("Invalid theme packet header")
    offset, fields = 9, {}
    while offset < len(data):
        if offset + 6 > len(data):
            raise ValueError("Truncated theme field header")
        tag, size = struct.unpack_from(">hI", data, offset)
        offset += 6
        if offset + size > len(data) or tag in fields:
            raise ValueError("Invalid or duplicate theme field")
        fields[tag] = data[offset:offset + size]
        offset += size
    if wanted not in fields:
        raise ValueError(f"Theme field {wanted} is missing")
    return fields[wanted]


def extract(jar_path):
    try:
        jar_data = jar_path.read_bytes()
    except FileNotFoundError as exc:
        raise ValueError(
            f"Missing applet: {jar_path}. Restore the verified v4.62 JAR using "
            "tools/applets/SETUP.md, then rerun this command."
        ) from exc
    if len(jar_data) != JAR_SIZE or hashlib.sha256(jar_data).hexdigest() != JAR_SHA256:
        raise ValueError(f"Verification failed for {jar_path}; expected the documented v4.62 publisher JAR")
    with ZipFile(io.BytesIO(jar_data)) as jar:
        assets = {name: jar.read(name) for name in ASSETS if name != "sprites.png"}
    assets["sprites.png"] = theme_field(assets["default_v2"], 129)
    for name, data in assets.items():
        size, digest = ASSETS[name]
        if len(data) != size or hashlib.sha256(data).hexdigest() != digest:
            raise ValueError(f"Verification failed for extracted {name}")
    return assets


def install(assets, directory):
    # Verify every input before creating output. Replace each destination only
    # after its complete contents have been written to a temporary sibling.
    directory.mkdir(parents=True, exist_ok=True)
    for name, data in assets.items():
        temporary = None
        try:
            with tempfile.NamedTemporaryFile(dir=directory, prefix=f".{name}.", delete=False) as stream:
                temporary = Path(stream.name)
                stream.write(data)
                os.fchmod(stream.fileno(), 0o644)
            temporary.replace(directory / name)
        finally:
            if temporary is not None:
                temporary.unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--jar", type=Path, default=ROOT / "research/applets/v4/artifacts/original/mig33-4.62.jar")
    parser.add_argument("--output", type=Path, default=ROOT / "data/applets/resources/v4")
    args = parser.parse_args()
    try:
        assets = extract(args.jar)
        install(assets, args.output)
    except (OSError, ValueError, KeyError, BadZipFile) as exc:
        parser.exit(1, f"{exc}\n")
    print(json.dumps({
        "jar": str(args.jar),
        "jar_sha256": JAR_SHA256,
        "output": str(args.output),
        "assets": [{"name": name, "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}
                   for name, data in assets.items()],
    }, indent=2))


if __name__ == "__main__":
    main()
