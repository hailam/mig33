#!/bin/sh
set -eu

# Run from any directory. Tool versions are intentionally pinned independently
# of runtime dependencies, and go run @version does not edit the app module.
cd "$(dirname "$0")/.."
go run github.com/getkin/kin-openapi/cmd/validate@v0.142.0 -- api/openapi.yaml
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.1 \
  -config api/oapi-codegen.yaml api/openapi.yaml
