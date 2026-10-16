#!/usr/bin/env bash
# Public-API compatibility of package genesismesh against the latest release
# tag, with golang.org/x/exp/cmd/apidiff (CI installs a pinned version).
#
#   scripts/api_compat.sh BASE HEAD
#
# Prints the API changes from the latest v* tag to HEAD. An incompatible change
# HEAD adds (one the merge base of BASE and HEAD does not already have, since
# that one was accepted when it merged) fails unless:
#   - BREAKING is "yes" (the pull request carries the label `breaking`; CI
#     sets "n/a" on a push to main, where the label was checked before merge),
#   - and the change adds or modifies a changelog fragment
#     (changelog.d/<version>/<name>.md) with an "### Upgrading" section.
#
# Environment: APIDIFF (default: apidiff on PATH), BREAKING (yes, no or n/a).
set -euo pipefail

if [ $# -ne 2 ]; then
  echo "usage: $0 BASE HEAD" >&2
  exit 2
fi
apidiff="${APIDIFF:-apidiff}"
breaking="${BREAKING:-no}"
head="$(git rev-parse --verify "$2^{commit}")"
base="$(git merge-base "$1" "$head")"
tag="$(git tag --list 'v[0-9]*' --sort=-v:refname | head -n 1)"
if [ -z "$tag" ]; then
  echo "::error::No release tag v* to compare with (fetch the tags: fetch-depth 0)."
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
export_api() { # <revision> <name>
  mkdir -p "$work/$2"
  git archive "$1" | tar -x -C "$work/$2"
  (cd "$work/$2" && "$apidiff" -w "$work/$2.export" ./genesismesh)
}
incompatible() { # <name>: the incompatible changes from the tag, one per line
  "$apidiff" -incompatible "$work/tag.export" "$work/$1.export" | grep '^- ' | LC_ALL=C sort -u || true
}
export_api "$tag" tag
export_api "$base" base
export_api "$head" head

echo "API changes from $tag to ${head:0:12}:"
report="$("$apidiff" "$work/tag.export" "$work/head.export")"
echo "${report:-  none}" | sed 's/^/  /'

new="$(LC_ALL=C comm -13 <(incompatible base) <(incompatible head))"
if [ -z "$new" ]; then
  echo "No incompatible change over the merge base ${base:0:12}."
  exit 0
fi
echo "Incompatible changes against $tag that this change adds:"
echo "$new" | sed 's/^/  /'

notes=""
while IFS= read -r f; do
  [ -n "$f" ] || continue
  if git show "$head:$f" | grep -q '^### Upgrading'; then notes="$notes $f"; fi
done < <(git diff --no-renames --name-only --diff-filter=AM "$base" "$head" -- 'changelog.d/*/*.md')

failed=0
if [ -z "$notes" ]; then
  echo "::error::An incompatible API change needs a changelog fragment (changelog.d/<version>/<name>.md) with an \"### Upgrading\" section telling callers what to do."
  failed=1
else
  echo "Upgrading notes:$notes"
fi
case "$breaking" in
  yes) echo "The pull request carries the label breaking." ;;
  n/a) echo "Not a pull request: the label breaking was required before the merge." ;;
  *)
    echo "::error::An incompatible API change needs the label breaking, which the Maintainer adds after deciding the break is worth it (a Go major version needs a new module path)."
    failed=1
    ;;
esac
exit "$failed"
