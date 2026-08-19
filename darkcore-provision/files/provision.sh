#!/bin/sh
# Саморегистрация устройства при первой загрузке.
#
# До этого идентификатор платы вписывал человек: партия из тридцати штук
# означала тридцать ручных настроек, и каждая — возможность опечататься в UUID,
# после чего плата молча не получает конфигурацию.
#
# Плата регистрируется выключенной: сервер заводит клиента с enable=false, так
# что саморегистрация не раздаёт доступ. Включает её отдельное действие — то же,
# что и раньше, только уже по существующему устройству.
#
# Скрипт идемпотентен: при заполненном uuid он ничего не делает, поэтому его
# безопасно вызывать при каждой загрузке и повторять по таймеру.

. /lib/functions.sh

ENDPOINT_DEFAULT="http://201.34.132.118:3000/api/connections"
RETRY_DELAY=30
MAX_ATTEMPTS=10

log() {
	logger -t darkcore-provision "$1"
}

existing=$(uci -q get darkcore.main.uuid)

if [ -n "$existing" ]; then
	log "uuid already set, nothing to do"
	exit 0
fi

token=$(uci -q get darkcore.provision.token)

if [ -z "$token" ]; then
	log "no provisioning token configured, refusing to register"
	exit 1
fi

endpoint=$(uci -q get darkcore.provision.endpoint)
[ -n "$endpoint" ] || endpoint="$ENDPOINT_DEFAULT"

new_uuid=$(cat /proc/sys/kernel/random/uuid)

attempt=1
while [ "$attempt" -le "$MAX_ATTEMPTS" ]; do
	code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 \
		-X POST "$endpoint" \
		-H 'Content-Type: application/json' \
		-H "X-Darkcore-Token: $token" \
		-d "{\"uuid\":\"$new_uuid\"}")

	case "$code" in
	200 | 201)
		# uuid сохраняется только после подтверждения сервером: иначе плата
		# считала бы себя зарегистрированной, а на сервере её бы не было, и
		# повторить регистрацию было бы уже нечем — условие выхода выполнено.
		uci set darkcore.main.uuid="$new_uuid"
		uci commit darkcore
		log "registered on attempt $attempt"
		exit 0
		;;
	401 | 403)
		log "provisioning token rejected, giving up"
		exit 1
		;;
	esac

	log "registration attempt $attempt failed with code ${code:-none}"
	attempt=$((attempt + 1))
	sleep "$RETRY_DELAY"
done

log "registration failed after $MAX_ATTEMPTS attempts"
exit 1
