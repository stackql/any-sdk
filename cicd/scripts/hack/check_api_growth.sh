#!/usr/bin/env bash
set -euo pipefail

echo "Checking for API sprawl..."

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "TMP_DIR=$TMP_DIR"

echo "Changing to root dir: $ROOT_DIR"

cd "$ROOT_DIR"

# Regenerate current API snapshots
grep -Rho 'func [A-Z][A-Za-z0-9_]*' --include='*.go' --exclude-dir={vendor,testdata} . \
  | sort -u > "$TMP_DIR/exported_funcs.txt"

grep -Rnoh '^type [A-Z][A-Za-z0-9_]* interface' --include='*.go' --exclude-dir={vendor,testdata} . \
  | sed 's/^[^:]*://g' | sort -u > "$TMP_DIR/exported_interfaces.txt"

grep -Rnoh '^type [A-Z][A-Za-z0-9_]* struct' --include='*.go' --exclude-dir={vendor,testdata} . \
  | sed 's/^[^:]*://g' | sort -u > "$TMP_DIR/exported_structs.txt"

fail=0

check_file() {
  local baseline="$1"
  local current="$2"
  local label="$3"

  if [[ ! -f "$baseline" ]]; then
    echo "Baseline file missing: $baseline"
    fail=1
    return
  fi

  # Show lines present in current but not in baseline (i.e. additions).
  local added
  added="$(comm -13 <(sort "$baseline") <(sort "$current") || true)"

  if [[ -n "$added" ]]; then
    echo "API growth detected in $label:"
    echo "$added"
    echo
    fail=1
  fi
}


check_file "cicd/tools/api/exported_funcs.txt"       "$TMP_DIR/exported_funcs.txt"       "exported functions"
check_file "cicd/tools/api/exported_interfaces.txt"  "$TMP_DIR/exported_interfaces.txt"  "exported interfaces"
check_file "cicd/tools/api/exported_structs.txt"     "$TMP_DIR/exported_structs.txt"     "exported structs"

if [[ "$fail" -ne 0 ]]; then
  echo "API sprawl check FAILED: new exported symbols were introduced."
  echo "If this is intentional, update tools/api/*.txt and FUTURE_API.md in the same PR."
  exit 1
fi

echo "API sprawl check PASSED: no new exported symbols."
