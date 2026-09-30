#!/bin/sh
# Reads sing-box's Clash API (127.0.0.1-only, see experimental.clash_api
# in 00-base.json) for the current manual-selector state ("MANUAL", see
# sing-box-sync-manual-selector.sh) plus a live latency probe of every
# server GLOBAL AUTO knows about, in one round trip via the Clash API's
# group-delay endpoint (tests every member in parallel server-side,
# so this stays cheap even with ~90 servers - no per-server calls).
#
# Called from the LuCI page via ubus file.exec (see acl.d) - Clash API is
# not reachable from the browser directly.
#
# Output - one line of JSON:
#   {"available":false}
#   {"available":true,"current":"<tag>","auto":<bool>,"delays":{"<tag>":<ms>,...}}
# "auto" is true when the manual selector is currently pointed at
# "GLOBAL AUTO" itself (automatic latency-based selection), false when a
# specific server has been manually pinned.

API="http://127.0.0.1:9090"
GROUP_ENC="GLOBAL%20AUTO"
MANUAL_ENC="MANUAL"
PROBE_URL_ENC="https%3A%2F%2Fcp.cloudflare.com%2Fgenerate_204"

fail() {
	echo '{"available":false}'
	exit 0
}

json_escape() {
	printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

manual_json="$(curl -s -m 3 "$API/proxies/$MANUAL_ENC")"
[ -n "$manual_json" ] || fail

current="$(echo "$manual_json" | jsonfilter -e '@.now' 2>/dev/null)"
[ -n "$current" ] || fail

delays_json="$(curl -s -m 10 "$API/group/$GROUP_ENC/delay?url=$PROBE_URL_ENC&timeout=3000")"
[ -n "$delays_json" ] || delays_json='{}'

if [ "$current" = "GLOBAL AUTO" ]; then
	auto=true
else
	auto=false
fi

printf '{"available":true,"current":"%s","auto":%s,"delays":%s}\n' \
	"$(json_escape "$current")" "$auto" "$delays_json"
