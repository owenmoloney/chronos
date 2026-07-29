#!/usr/bin/env bash
set -euo pipefail

DATABASE_URL="${DATABASE_URL:-postgres://chronos:chronos@localhost:5432/chronos?sslmode=disable}"
MIGRATIONS_PATH="./migrations"

migrate\
    -path  "$MIGRATIONS_PATH"\
    -database "$DATABASE_URL"\
    "$@"