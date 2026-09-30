#!/bin/sh
# Regenerates /etc/sing-box/conf.d/80-manual.json - a "selector" outbound
# ("MANUAL") mirroring GLOBAL AUTO's member list, with route.final pointed
# at it. GLOBAL AUTO itself is a urltest group and sing-box refuses manual
# selection on those (Clash API: "Must be a Selector") - MANUAL exists
# purely so the LuCI page can let a human pin a specific server, with
# "GLOBAL AUTO" itself as one of its choices (the default) for staying on
# automatic latency-based selection.
#
# 00-base.json (this file's own conf.d neighbour) loads before 90-proxy.json
# alphabetically, and for scalars in a sing-box -C directory merge the
# EARLIER file wins - but this generated file is named 80-*, which is also
# earlier than 90-proxy.json, so ITS route.final="MANUAL" wins over
# 90-proxy.json's own route.final="GLOBAL AUTO" the same way. Run before
# `sing-box check` on every start/restart (see sing-box.init) so it is
# always part of what gets validated and started.
#
# Pre-activation (90-proxy.json does not exist yet, or has no GLOBAL AUTO
# group) there is nothing to mirror - remove any stale 80-manual.json so
# route.final falls back to sing-box's own default (first declared
# outbound, i.e. "direct") exactly like before this feature existed.

PROXY_CONF="/etc/sing-box/conf.d/90-proxy.json"
OUT_CONF="/etc/sing-box/conf.d/80-manual.json"

if [ ! -f "$PROXY_CONF" ]; then
	rm -f "$OUT_CONF"
	exit 0
fi

members="$(jsonfilter -i "$PROXY_CONF" -e '@.outbounds[@.tag="GLOBAL AUTO"].outbounds[*]' 2>/dev/null)"
if [ -z "$members" ]; then
	rm -f "$OUT_CONF"
	exit 0
fi

{
	printf '{"outbounds":[{"type":"selector","tag":"MANUAL","outbounds":["GLOBAL AUTO"'
	echo "$members" | while IFS= read -r tag; do
		[ -n "$tag" ] || continue
		esc=$(printf '%s' "$tag" | sed 's/\\/\\\\/g; s/"/\\"/g')
		printf ',"%s"' "$esc"
	done
	printf '],"default":"GLOBAL AUTO"}],"route":{"final":"MANUAL"}}'
} > "${OUT_CONF}.tmp" && mv "${OUT_CONF}.tmp" "$OUT_CONF"
