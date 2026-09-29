#!/bin/sh
#
# Fail-open watchdog for darkcore-singbox.
#
# sing-box's own outbound-selection primitives don't give an automatic
# "fall back to direct internet" when every proxy outbound is dead:
#   - urltest falls back to the first outbound in its list, not to a
#     named "direct" outbound;
#   - selector is a manual (Clash-API-driven) switch, not automatic.
# So the actual fail-open is done here, one layer below sing-box: probe
# connectivity through sing-box's own local inbound, and toggle the
# TPROXY nftables interception on/off - mirroring darkcore-xray's
# stop_service's "nft flush ruleset" / "nft -f nftables.rulesv46" pair.

PROBE_URL="https://cp.cloudflare.com/generate_204"
SOCKS="127.0.0.1:10808"
NFT_RULES="/usr/share/sing-box/nftables.rulesv46"
STATE="/tmp/sing-box-watchdog.state"
FAIL_THRESHOLD=3
INTERVAL=15

fails=0
while true; do
	if curl -s -o /dev/null -m 5 --socks5-hostname "$SOCKS" "$PROBE_URL"; then
		fails=0
		if [ "$(cat "$STATE" 2>/dev/null)" = "down" ]; then
			if nft -f "$NFT_RULES"; then
				echo up > "$STATE"
				logger -t sing-box-watchdog "proxy reachable again, TPROXY interception resumed"
			fi
		fi
	else
		fails=$((fails + 1))
		if [ "$fails" -ge "$FAIL_THRESHOLD" ] && [ "$(cat "$STATE" 2>/dev/null)" != "down" ]; then
			if nft flush ruleset; then
				echo down > "$STATE"
				logger -t sing-box-watchdog "proxy unreachable ($fails checks failed), failing open to direct internet"
			fi
		fi
	fi
	sleep "$INTERVAL"
done
