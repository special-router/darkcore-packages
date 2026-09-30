#!/bin/sh
# Switches the "MANUAL" selector (see sing-box-sync-manual-selector.sh) to
# a specific outbound tag via sing-box's Clash API PUT /proxies/MANUAL.
# Pass "GLOBAL AUTO" as the target to go back to automatic latency-based
# selection. Persists across restarts via experimental.cache_file's
# store_selected (00-base.json) - without it every config refresh from
# dcvpnupd would silently revert a manual pin back to the default.
#
# $1 - target outbound tag, exactly as it appears in the profile list
#      returned by darkcore-singbox-profile-list.
#
# Called from the LuCI page via ubus file.exec (see acl.d).
#
# Output - one line of JSON: {"ok":true} or {"ok":false,"error":"..."}

API="http://127.0.0.1:9090"
TARGET="$1"
RESP="/tmp/darkcore-select-profile-resp.json"

json_escape() {
	printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

if [ -z "$TARGET" ]; then
	echo '{"ok":false,"error":"no target"}'
	exit 0
fi

body="{\"name\":\"$(json_escape "$TARGET")\"}"

http_code="$(curl -s -o "$RESP" -w '%{http_code}' -m 5 \
	-X PUT "$API/proxies/MANUAL" \
	-H 'Content-Type: application/json' \
	--data-raw "$body")"

if [ "$http_code" = "204" ] || [ "$http_code" = "200" ]; then
	echo '{"ok":true}'
else
	err="$(cat "$RESP" 2>/dev/null)"
	printf '{"ok":false,"error":"%s"}\n' "$(json_escape "$err")"
fi

rm -f "$RESP"
