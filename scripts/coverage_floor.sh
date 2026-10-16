#!/usr/bin/env bash
# Coverage ratchet: fails when statement coverage falls below its floor.
#
#   FLOOR_TOTAL=87 FLOOR_VERIFIER=88 scripts/coverage_floor.sh coverage.out
#
# coverage.out comes from `go test -coverprofile`. The floors (whole percents,
# set in .github/workflows/ci.yml) are checked for the whole package and for
# the offline verifier (the json, registry and verifiers layers of
# docs/architecture.md). The script prints every file's coverage.
set -euo pipefail

profile="${1:?usage: $0 coverage.out}"
: "${FLOOR_TOTAL:?FLOOR_TOTAL is not set}"
: "${FLOOR_VERIFIER:?FLOOR_VERIFIER is not set}"
verifier_files="strict_json.go canonical.go strict.go verify.go key_history.go grants.go"

awk -v floor_total="$FLOOR_TOTAL" -v floor_verifier="$FLOOR_VERIFIER" -v verifier="$verifier_files" '
  BEGIN { n = split(verifier, v, " "); for (i = 1; i <= n; i++) is_verifier[v[i]] = 1 }
  NR == 1 { next }  # mode: ...
  {
    # path/file.go:start,end statements count
    split($1, loc, ":"); file = loc[1]; sub(/.*\//, "", file)
    key = $1  # a block can appear more than once; it is covered if any run covered it
    if (!(key in stmts)) { stmts[key] = $2; fileof[key] = file }
    if ($3 > 0) covered[key] = 1
  }
  END {
    for (k in stmts) {
      f = fileof[k]; fs[f] += stmts[k]; total += stmts[k]
      if (k in covered) { fc[f] += stmts[k]; total_c += stmts[k] }
      if (f in is_verifier) { vs += stmts[k]; if (k in covered) vc += stmts[k] }
    }
    if (total == 0 || vs == 0) { print "::error::The coverage profile has no statements for the package or the verifier"; exit 1 }
    for (f in fs) printf "  %-22s %6.1f%%  (%d statements)\n", f, 100 * fc[f] / fs[f], fs[f] | "sort"
    close("sort")
    t = 100 * total_c / total; ver = 100 * vc / vs
    printf "Package:          %.1f%% of %d statements (floor %d%%)\n", t, total, floor_total
    printf "Offline verifier: %.1f%% of %d statements (floor %d%%)\n", ver, vs, floor_verifier
    failed = 0
    if (t < floor_total) { printf "::error::Coverage %.1f%% is below the floor %d%%: add tests for the code you changed.\n", t, floor_total; failed = 1 }
    if (ver < floor_verifier) { printf "::error::Offline verifier coverage %.1f%% is below the floor %d%%: add tests for the verifier code you changed.\n", ver, floor_verifier; failed = 1 }
    exit failed
  }
' "$profile"
