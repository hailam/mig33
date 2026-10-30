#!/bin/sh
set -eu

# Keep the scanner version shared by local runs and CI. The vulnerability
# database remains live; findings fail this command rather than being waived.
cd "$(dirname "$0")/.."
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 "$@" ./...
