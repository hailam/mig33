#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
exec uv run --no-project python scripts/format-go.py "$@"
