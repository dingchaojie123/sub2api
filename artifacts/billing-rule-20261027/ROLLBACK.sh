#!/bin/sh
set -eu

if [ "${1:-}" = "--fixture" ]; then
  fixture=${2:?fixture path is required}
  tmp="${fixture}.rollback.tmp"
  jq '.channels = [.channels[] | select(.id != 4 and .id != 5)]
      | .groups["34"] = 1.5
      | .groups["62"] = 0.7
      | .accounts["38"] = 1.0
      | .accounts["67"] = 1.0' "$fixture" > "$tmp"
  mv "$tmp" "$fixture"
  printf '%s\n' 'ROLLBACK fixture restored: channels removed; groups 34=1.5, 62=0.7; accounts 38=1.0, 67=1.0'
  exit 0
fi

: "${SUB2API_BASE_URL:?SUB2API_BASE_URL is required}"
: "${SUB2API_JWT:?SUB2API_JWT is required}"

auth="Authorization: Bearer ${SUB2API_JWT}"
curl -fSs -X DELETE "${SUB2API_BASE_URL}/api/v1/admin/channels/4" -H "$auth" >/dev/null
curl -fSs -X DELETE "${SUB2API_BASE_URL}/api/v1/admin/channels/5" -H "$auth" >/dev/null
curl -fSs -X PUT "${SUB2API_BASE_URL}/api/v1/admin/groups/34" \
  -H "$auth" -H 'Content-Type: application/json' \
  --data '{"rate_multiplier":1.5}' >/dev/null
printf '%s\n' 'ROLLBACK live restored: channels 4/5 removed; group 34 rate_multiplier=1.5'
