#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
python3 scripts/export-sqlc-schema.py

# SQLC_BIN can point at a prebuilt binary for offline generation. Verify its
# version so generated code stays reproducible across developer machines.
if [ -n "${SQLC_BIN:-}" ]; then
  if [ "$("$SQLC_BIN" version)" != "v1.31.1" ]; then
    echo "SQLC_BIN must be sqlc v1.31.1" >&2
    exit 1
  fi
  "$SQLC_BIN" generate
else
  go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
fi
