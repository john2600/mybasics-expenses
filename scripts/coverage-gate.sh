#!/usr/bin/env bash
#
# Per-package coverage gate.
#
# Reads a Go coverage profile and fails if any package listed in the thresholds
# file is below its minimum. Packages that are not listed are not measured — the
# list is an allow-list, so adding a package under the gate is a deliberate act
# and a new package never breaks CI on the day it is created.
#
# Usage: scripts/coverage-gate.sh [profile] [thresholds]

set -euo pipefail

PROFILE="${1:-coverage.out}"
THRESHOLDS="${2:-coverage-thresholds.txt}"

[[ -f "$PROFILE" ]]    || { echo "coverage profile not found: $PROFILE" >&2; exit 2; }
[[ -f "$THRESHOLDS" ]] || { echo "thresholds file not found: $THRESHOLDS" >&2; exit 2; }

MODULE="$(awk '/^module /{print $2; exit}' go.mod)"

# Aggregate the profile into "<package> <percent>" lines. Profile rows look like
#   <file>:<start>,<end> <numStatements> <timesExecuted>
# so statements are summed per directory, and counted as covered when executed
# at least once.
coverage_by_package() {
  awk -v module="$MODULE" '
    /^mode:/ { next }
    {
      split($1, loc, ":")
      n = split(loc[1], parts, "/")
      dir = parts[1]
      for (i = 2; i < n; i++) dir = dir "/" parts[i]
      sub("^" module "/", "", dir)

      total[dir] += $2
      if ($3 > 0) covered[dir] += $2
    }
    END {
      for (d in total) printf "%s %.1f\n", d, total[d] ? covered[d] * 100 / total[d] : 0
    }
  ' "$PROFILE"
}

MEASURED="$(coverage_by_package)"
failed=0

printf '%-40s %8s %8s   %s\n' PACKAGE ACTUAL MINIMUM RESULT
printf '%-40s %8s %8s   %s\n' "----------------------------------------" "------" "-------" "------"

while read -r pkg min; do
  [[ -z "${pkg:-}" || "$pkg" == \#* ]] && continue

  actual="$(awk -v p="$pkg" '$1 == p { print $2 }' <<<"$MEASURED")"

  # A listed package missing from the profile means it was never compiled into
  # the test run — treat that as a failure rather than silently passing.
  if [[ -z "$actual" ]]; then
    printf '%-40s %8s %8s   %s\n' "$pkg" "n/a" "$min%" "FAIL (not in profile)"
    failed=1
    continue
  fi

  if awk -v a="$actual" -v m="$min" 'BEGIN { exit !(a + 0 < m + 0) }'; then
    printf '%-40s %7s%% %7s%%   %s\n' "$pkg" "$actual" "$min" "FAIL"
    failed=1
  else
    printf '%-40s %7s%% %7s%%   %s\n' "$pkg" "$actual" "$min" "ok"
  fi
done < "$THRESHOLDS"

if (( failed )); then
  echo
  echo "Coverage gate failed. Add tests, or — only if the threshold was wrong to"
  echo "begin with — change it in $THRESHOLDS in the same commit, with a reason."
  exit 1
fi

echo
echo "Coverage gate passed."
