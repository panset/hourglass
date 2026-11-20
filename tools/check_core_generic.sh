#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Fails when a core package stops being generic (INTENT: "The core must stay generic").
# Core = internal/, pkg/hourglass/, pkg/hourglasstest/.
#   rule1: core .go files (tests included) must not import extension packages
#          (github.com/panset/hourglass/{providers,pressure,policies,hooks,examples}) or vmware/govmomi.
#   rule2: core files of any type must not mention vendor, size or provider terms (case-insensitive).
#   rule3: vmware/govmomi may be imported only by providers/vsphere/ and deploy/ or e2e/
#          (non-core files; core is already covered by rule1).
#   rule4: core files of any type must not carry a resource naming rule (D40): the reference
#          scenario's pool suffix `_RP` and the removed `resource_name_template` setting
#          (case-insensitive). Only the migration that deletes the stored setting and its
#          upgrade test may name it.
# Output: path:line: rule: text. Exit 1 on any violation, 0 otherwise.
set -euo pipefail
cd "$(dirname "$0")/.."

CORE_DIRS="internal pkg/hourglass pkg/hourglasstest"
EXT_IMPORT_RE='^github\.com/panset/hourglass/(providers|pressure|policies|hooks|examples)(/|$)'
GOVMOMI='vmware/govmomi'
TEXT_RE='vcenter|vsphere|govmomi|vcsim|resource pool|resourcepool|extralarge|size=|keycloak'
NAMING_RE='_RP|resource_name_template'
NAMING_EXEMPT='internal/store/migrations/0004_drop_resource_name_override.sql internal/store/migrate0004_test.go'

violations=0
report() {
  printf '%s:%s: %s: %s\n' "$1" "$2" "$3" "$4"
  violations=$((violations + 1))
}

is_core() {
  case "$1" in
    internal/*|pkg/hourglass/*|pkg/hourglasstest/*) return 0 ;;
  esac
  return 1
}

# All tracked-or-not Go files in the repo (bazel-* symlinks, .git and .orchestrator skipped).
go_files=$(find . \( -name .git -o -name .orchestrator -o -name 'bazel-*' \) -prune -o \
  -type f -name '*.go' -print | sed 's|^\./||' | sort)

# Core files of any type.
core_files=""
for d in $CORE_DIRS; do
  [ -d "$d" ] || continue
  core_files="$core_files$(find "$d" -type f | sort)
"
done
core_files=$(printf '%s' "$core_files" | sed '/^$/d')

# Import extraction via the go tool (handles every import form: blank, dot, aliased, one-line, grouped).
# "file<TAB>line<TAB>importpath": the package's Imports, TestImports and XTestImports come from
# `go list`; file:line is then located by searching the package's Go files for the quoted path.
imports=""
if [ -n "$go_files" ]; then
  pkgs=$(go list -e -f '{{.Dir}}{{"\t"}}{{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... 2>/dev/null || true)
  root=$(pwd -P)
  while IFS=$'\t' read -r dir plist; do
    [ -n "$dir" ] || continue
    rel=${dir#"$root"}; rel=${rel#/}; [ -n "$rel" ] || rel="."
    for ip in $plist; do
      [[ $ip == *.* ]] || continue # stdlib has no dot in first element
      if [[ $ip =~ $EXT_IMPORT_RE ]] || [[ $ip == *"$GOVMOMI"* ]]; then
        found=$(grep -H -n -F "\"$ip\"" "$rel"/*.go 2>/dev/null | grep -v -E '^[^:]*:[0-9]+:[[:space:]]*//' || true)
        if [ -n "$found" ]; then
          while IFS= read -r g; do
            f=${g%%:*}; r=${g#*:}
            imports="$imports$f"$'\t'"${r%%:*}"$'\t'"$ip"$'\n'
          done <<< "$found"
        else
          imports="$imports$rel"$'\t'"0"$'\t'"$ip"$'\n'
        fi
      fi
    done
  done <<< "$pkgs"
fi

while IFS=$'\t' read -r file line path; do
  [ -n "$path" ] || continue
  if is_core "$file"; then
    if [[ $path =~ $EXT_IMPORT_RE ]] || [[ $path == *"$GOVMOMI"* ]]; then
      report "$file" "$line" "rule1" "import \"$path\""
    fi
  else
    case "$file" in
      providers/vsphere/*|deploy/*|e2e/*) ;;
      *)
        if [[ $path == *"$GOVMOMI"* ]]; then
          report "$file" "$line" "rule3" "import \"$path\""
        fi
        ;;
    esac
  fi
done <<< "$imports"

core_count=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  core_count=$((core_count + 1))
  hits=$(grep -n -i -I -E "$TEXT_RE" "$f" || true)
  [ -n "$hits" ] || continue
  while IFS= read -r hit; do
    report "$f" "${hit%%:*}" "rule2" "${hit#*:}"
  done <<< "$hits"
done <<< "$core_files"

while IFS= read -r f; do
  [ -n "$f" ] || continue
  case " $NAMING_EXEMPT " in *" $f "*) continue ;; esac
  hits=$(grep -n -i -I -E "$NAMING_RE" "$f" || true)
  [ -n "$hits" ] || continue
  while IFS= read -r hit; do
    report "$f" "${hit%%:*}" "rule4" "${hit#*:}"
  done <<< "$hits"
done <<< "$core_files"

go_count=$(printf '%s\n' "$go_files" | sed '/^$/d' | wc -l | tr -d ' ')

if [ "$violations" -gt 0 ]; then
  echo "core-genericity: $violations violation(s) found" >&2
  exit 1
fi
echo "core-genericity: OK ($core_count core files, $go_count Go files checked, no violations)"
