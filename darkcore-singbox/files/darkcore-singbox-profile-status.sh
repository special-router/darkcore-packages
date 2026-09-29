#!/bin/sh
# Читает через Clash API sing-box'а (см. experimental.clash_api в
# 00-base.json, забинден только на 127.0.0.1 - наружу не торчит), какой
# outbound сейчас реально выбран группой "GLOBAL AUTO", и меряет пинг до
# него живым запросом. Дергаем именно саму группу (не конкретный тег
# сервера) и на GET, и на /delay - у group-outbound'ов DialContext сам
# делегирует текущему выбранному члену, так что тестируется реальный
# активный путь, а percent-encode для delay-запроса нужен только для
# "GLOBAL AUTO" (пробел), не для имени сервера, у которого в теге бывают
# эмодзи - их пришлось бы percent-encode побайтово в POSIX shell, а так
# необходимости в этом нет. Проверено вручную (2026-09-29) реальными
# запросами к работающему sing-box с этим же tag/URL форматом.
#
# Вызывается со страницы LuCI через ubus `file.exec` (Clash API слушает
# только 127.0.0.1, из браузера напрямую не достать) - см. acl.d.
#
# Вывод - одна строка JSON:
#   {"available":false}
#   {"available":true,"profile":"<тег текущего сервера>","delay_ms":<N>}

API="http://127.0.0.1:9090"
GROUP_ENC="GLOBAL%20AUTO"
PROBE_URL_ENC="https%3A%2F%2Fcp.cloudflare.com%2Fgenerate_204"

fail() {
	echo '{"available":false}'
	exit 0
}

# экранирует только то, что обязательно для валидности JSON-строки
# (backslash, кавычка) - эмодзи/юникод в теге профиля идут как есть,
# JSON-строки UTF-8 нативно, экранировать их не нужно
json_escape() {
	printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

group_json="$(curl -s -m 3 "$API/proxies/$GROUP_ENC")"
[ -n "$group_json" ] || fail

now="$(echo "$group_json" | jsonfilter -e '@.now' 2>/dev/null)"
[ -n "$now" ] || fail

delay_json="$(curl -s -m 5 "$API/proxies/$GROUP_ENC/delay?url=$PROBE_URL_ENC&timeout=3000")"
[ -n "$delay_json" ] || fail

delay="$(echo "$delay_json" | jsonfilter -e '@.delay' 2>/dev/null)"
[ -n "$delay" ] || fail

printf '{"available":true,"profile":"%s","delay_ms":%s}\n' "$(json_escape "$now")" "$delay"
