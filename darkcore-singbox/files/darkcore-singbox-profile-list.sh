#!/bin/sh
# Reads sing-box's Clash API (127.0.0.1-only, see experimental.clash_api
# in 00-base.json) for the current manual-selector state ("MANUAL", see
# sing-box-sync-manual-selector.sh), its full server list, and each
# server's last measured latency - all from one GET /proxies.
#
# Latency is the last result of GLOBAL AUTO's own periodic urltest (kept
# by sing-box in the shared URL-test history, exposed per proxy as
# "history"), NOT a fresh probe. A fresh probe (GET /group/.../delay) was
# tried first and does not work here: with ~90 servers at sing-box's
# internal urltest concurrency of 10 it takes far longer than any sane
# LuCI rpc timeout, so it always came back empty and the dropdown showed
# nothing but "Авто" (verified on hardware 2026-10-07: 10147ms, delays {}).
# It would also hit every VPN server at once on every page load.
#
# Called from the LuCI page via ubus file.exec (see acl.d) - Clash API is
# not reachable from the browser directly.
#
# Output - one line of JSON:
#   {"available":false}
#   {"available":true,"current":"<tag>","auto":<bool>,
#    "servers":["<tag>",...],"delays":{"<tag>":<ms>,...}}
# "auto" is true when the manual selector is currently pointed at
# "GLOBAL AUTO" itself (automatic latency-based selection), false when a
# specific server has been manually pinned. "servers" is MANUAL's member
# list minus "GLOBAL AUTO"; "delays" only has servers whose last check
# succeeded (sing-box drops the history entry of a failed one).

API="http://127.0.0.1:9090"
TMP="/tmp/darkcore-profile-list.$$.json"

trap 'rm -f "$TMP"' EXIT

fail() {
	echo '{"available":false}'
	exit 0
}

json_escape() {
	printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

curl -s -f -m 3 -o "$TMP" "$API/proxies" || fail

current="$(jsonfilter -i "$TMP" -e '@.proxies.MANUAL.now' 2>/dev/null)"
[ -n "$current" ] || fail

servers=''
delays=''
while IFS= read -r tag; do
	[ -n "$tag" ] || continue
	[ "$tag" = "GLOBAL AUTO" ] && continue
	esc="$(json_escape "$tag")"
	servers="${servers:+$servers,}\"$esc\""

	# a tag with a double quote can't be put in a jsonfilter expression -
	# it just gets no ping instead of breaking the whole list
	case "$tag" in *'"'*) continue ;; esac
	d="$(jsonfilter -i "$TMP" -e "@.proxies[\"$tag\"].history[0].delay" 2>/dev/null)"
	case "$d" in
		''|*[!0-9]*) ;;
		*) delays="${delays:+$delays,}\"$esc\":$d" ;;
	esac
done <<EOF
$(jsonfilter -i "$TMP" -e '@.proxies.MANUAL.all[*]' 2>/dev/null)
EOF

if [ "$current" = "GLOBAL AUTO" ]; then
	auto=true
else
	auto=false
fi

printf '{"available":true,"current":"%s","auto":%s,"servers":[%s],"delays":{%s}}\n' \
	"$(json_escape "$current")" "$auto" "$servers" "$delays"
