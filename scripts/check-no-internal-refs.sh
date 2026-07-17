#!/usr/bin/env bash
#
# Fail if any tracked file leaks internal infrastructure references into this
# public repository. Run locally or in CI; exits non-zero on the first hit.
#
# Notes on what is deliberately NOT matched:
#   - 192.168.4.1 is the Bramble node SoftAP default gateway (product fact).
#   - 192.168.1.x in tests is mock sample data, not a real host.
# Only the internal LAN subnet used by the private Gitea host is flagged.
set -euo pipefail

cd "$(dirname "$0")/.."

# Extended-regex patterns that must never appear in a public checkout.
patterns=(
  'example'
  'justinlindh'
  '192\.168\.100\.'
  '/home/user'
  'host'
)

pattern="$(IFS='|'; echo "${patterns[*]}")"

# Search tracked files only; exclude this script (it names the patterns) and
# the frozen .gitea mirror config, which retains Gitea-side references by design.
if git grep -nIE "$pattern" -- . \
    ':!scripts/check-no-internal-refs.sh' \
    ':!.gitea/' \
    > /tmp/internal-refs-hits.txt 2>/dev/null; then
  echo "ERROR: internal infrastructure references found in tracked files:" >&2
  cat /tmp/internal-refs-hits.txt >&2
  exit 1
fi

echo "check-no-internal-refs: clean"
