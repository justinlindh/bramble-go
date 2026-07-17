#!/usr/bin/env bash
#
# Fail if any tracked file leaks internal infrastructure into this public repo.
# Run locally or in CI; exits non-zero if anything is found. Scans ALL tracked
# files including markdown (public docs must never carry these either).
#
# The only private-range address that is allowed is the Bramble node SoftAP
# default gateway 192.168.4.x, which is a product constant, not a real host.
set -euo pipefail

cd "$(dirname "$0")/.."

SELF='scripts/check-no-internal-refs.sh'
fail=0

# Internal hostnames, the fleet secrets dir, and personal absolute paths.
word_hits="$(git grep -nIE 'example|justinlindh|host|bramble-meta|/home/justin' -- . ":!$SELF" 2>/dev/null || true)"
if [ -n "$word_hits" ]; then
  echo "ERROR: internal infrastructure references found:" >&2
  echo "$word_hits" >&2
  fail=1
fi

# Any private 192.168.x.x address except the ESP32 SoftAP range 192.168.4.x.
ip_hits="$(git grep -nIE '192\.168\.[0-9]+\.[0-9]+' -- . ":!$SELF" 2>/dev/null | grep -vE '192\.168\.4\.' || true)"
if [ -n "$ip_hits" ]; then
  echo "ERROR: private LAN address (outside the ESP32 AP range) found:" >&2
  echo "$ip_hits" >&2
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "check-no-internal-refs: clean"
