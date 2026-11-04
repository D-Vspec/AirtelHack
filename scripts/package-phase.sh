#!/usr/bin/env bash
# Zips the repo as the output of one phase: ./scripts/package-phase.sh <n>
set -euo pipefail
N="${1:?usage: package-phase.sh <phase number>}"
cd "$(dirname "$0")/.."
SLUG=$(python3 -c "import json,sys; print(next(p['slug'] for p in json.load(open('scripts/phases.json')) if p['n']==int(sys.argv[1])))" "$N")
mkdir -p dist
OUT="dist/phase${N}-${SLUG}-output.zip"
rm -f "$OUT"
zip -rq "$OUT" . \
  -x "node_modules/*" "*/node_modules/*" "*/.next/*" "dist/*" ".git/*" ".env" "*/.expo/*" "*.tsbuildinfo" "*/__pycache__/*" "ml/.venv/*"
echo "wrote $OUT ($(du -h "$OUT" | cut -f1))"
