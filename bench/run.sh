#!/usr/bin/env bash
# Measure gm on a generated corpus of SIZE notes: the time of each command in
# the full and the pruned copy, then how often search finds the answer.
# Usage: bench/run.sh SIZE [GM]   (GM defaults to the gm on PATH)
set -euo pipefail
shopt -s nullglob
size=$1
gm=$(realpath "${2:-$(command -v gm)}")
here=$(dirname "$(realpath "$0")")
dir=$(mktemp -d)
python3 -I "$here/gen.py" "$size" "$dir"
python3 -I "$here/build.py" "$dir" "$gm"
for copy in full pruned; do
  cd "$dir/$copy/nw"
  files=(.memory/notes/*.md)
  id=$(basename "${files[0]}" | cut -c1-23)
  for cmd in "search what does the billing api use as its message queue" "search what replaced auth0 --all" "show $id" "list -n 5" "lint"; do
    TIMEFORMAT="$copy %Rs  gm $cmd"
    time ("$gm" $cmd >/dev/null || true)
  done
done
cd "$here"
python3 -I "$here/retrieval.py" "$dir" "$gm" 40
echo "corpus kept in $dir"
