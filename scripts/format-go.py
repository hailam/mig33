#!/usr/bin/env python3
"""Format handwritten Go source with pinned goimports and gofumpt tools."""

import argparse
import os
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[1]
SOURCE_ROOTS = ("cmd", "configs", "db", "internal", "tools/contracts")
TOOLS = {
    "goimports": ("golang.org/x/tools", "golang.org/x/tools/cmd/goimports", "v0.52.0"),
    "gofumpt": ("mvdan.cc/gofumpt", "mvdan.cc/gofumpt", "v0.12.0"),
}
GENERATED = re.compile(rb"(?m)^// Code generated .* DO NOT EDIT\.$")


def tool_path(name):
    return ROOT / "data/tools" / name / TOOLS[name][2] / name


def verify_tool(name):
    binary = tool_path(name)
    if not binary.is_file():
        raise ValueError(f"Missing {name}; run sh scripts/format-go.sh --install first")
    metadata = subprocess.check_output(["go", "version", "-m", str(binary)], text=True)
    module, _, version = TOOLS[name]
    if not any(line.split()[:3] == ["mod", module, version] for line in metadata.splitlines()):
        raise ValueError(f"Expected {name} {version} at {binary}; remove it and rerun --install")
    return binary


def install_tools():
    for name, (_, package, version) in TOOLS.items():
        binary = tool_path(name)
        if not binary.is_file():
            binary.parent.mkdir(parents=True, exist_ok=True)
            subprocess.run(
                ["go", "install", f"{package}@{version}"],
                cwd=ROOT,
                env={**os.environ, "GOBIN": str(binary.parent)},
                check=True,
            )
        verify_tool(name)
        print(f"Ready: {name} {version}")


def source_files(root):
    files, generated = [], 0
    for name in SOURCE_ROOTS:
        directory = root / name
        if directory.is_symlink():
            continue
        for current, directories, names in os.walk(directory, followlinks=False):
            directories[:] = sorted(d for d in directories if d != "node_modules" and not (Path(current) / d).is_symlink())
            for name in sorted(names):
                path = Path(current) / name
                if path.suffix != ".go" or path.is_symlink():
                    continue
                data = path.read_bytes()
                package = re.search(rb"(?m)^package[ \t]", data)
                header = data[:package.start()] if package else data
                if GENERATED.search(header):
                    generated += 1
                    continue
                files.append((path, data))
    return files, generated


def format_sources(check):
    goimports, gofumpt = verify_tool("goimports"), verify_tool("gofumpt")
    module_file = (ROOT / "go.mod").read_text()
    module = re.search(r"(?m)^module\s+(\S+)", module_file).group(1)
    language = "go" + re.search(r"(?m)^go\s+(\d+\.\d+)", module_file).group(1)
    files, generated = source_files(ROOT)
    changed = []
    # Formatting uses installed tools and already available packages only.
    # Installation is an explicit separate step and never edits the app module.
    environment = {**os.environ, "GOPROXY": "off", "GOTOOLCHAIN": "local"}
    for path, original in files:
        imports = subprocess.check_output(
            [str(goimports), "-local", module, "-srcdir", str(path)],
            input=original, cwd=ROOT, env=environment,
        )
        formatted = subprocess.check_output(
            [str(gofumpt), "-lang", language, "-modpath", module],
            input=imports, cwd=ROOT, env=environment,
        )
        if formatted != original:
            changed.append((path, original, formatted))
    if not check:
        # Refuse to overwrite an edit made while formatting was running.
        for path, original, _ in changed:
            if path.read_bytes() != original:
                raise ValueError(f"Source changed during formatting: {path}; rerun the command")
        for path, _, formatted in changed:
            path.write_bytes(formatted)
    for path, _, _ in changed:
        print(path.relative_to(ROOT))
    action = "need formatting" if check else "formatted"
    print(f"{len(files)} handwritten Go files checked; {generated} generated files skipped; {len(changed)} {action}.")
    if check and changed:
        print("Run sh scripts/format-go.sh --write to apply these changes.", file=sys.stderr)
        return 1
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--write", action="store_true", help="Format handwritten source (default)")
    mode.add_argument("--check", action="store_true", help="Report differences without writing source")
    mode.add_argument("--install", action="store_true", help="Install or verify pinned tools only")
    args = parser.parse_args()
    try:
        if args.install:
            install_tools()
            return 0
        return format_sources(args.check)
    except (OSError, ValueError, subprocess.CalledProcessError) as exc:
        print(f"Formatting failed: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
