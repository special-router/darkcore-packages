# darkcore-packages

OpenWrt/FriendlyWrt package feed с **backend-компонентами** прошивки
**Special Router** (бывш. Darkcore). Категория в menuconfig — `DarkCore`
(`CATEGORY:=DarkCore` во всех Makefile).

Клонируется на лету при сборке из основного репозитория **darkcorewrt**
(`/home/trytoca7ch/Projects/darkcore/darkcorewrt`):
`scripts/add_packages.sh` делает
`git clone https://github.com/special-router/darkcore-packages.git darkcore --depth 1 -b main`
в `friendlywrt/package/darkcore` и включает
`CONFIG_PACKAGE_{darkcore-xray,darkcore-main,geoupdate,dcvpnupd}=y`.
**`darkcore-provision` в сборку прошивки не включается** (см. ниже).

Здесь только UCI-конфиги, init-скрипты и Go-бинарники. **LuCI-приложения
/ темы здесь нет** — оно живёт отдельным пакетом в
`darkcorewrt/packages/luci-app-darkcore` (страница ввода UUID + статус
xray + брендинг).

---

## Быстрый вердикт

| пакет | в прошивке | вердикт |
|---|---|---|
| `darkcore-main` | да | **нужен** как каркас `/etc/config/darkcore`; бо́льшая часть опций мёртвая |
| `darkcore-xray` | да | **ядро** (сам прокси + routing + nft), удалять нельзя |
| `dcvpnupd` | да (cron `*/5`) | **нужен** — единственный источник `proxy`-outbound'а; половина кода (liveness, routing-fetch) — под снос |
| `geoupdate` | да (cron `15 0`) | полезен, не критичен; самый безопасный кандидат на снос |
| `darkcore-provision` | **НЕТ** | в образ не попадает, саморегистрация не работает → **под удаление** |

Планы доработок (спеки) — в `darkcorewrt`
`~/.claude/plans/robust-squishing-scone.md`: смена backend-адреса, чистка
мёртвого кода, удаление `darkcore-provision`, статический routing +
fail-open.

---

## `darkcore-main`

- `PKG_VERSION:=1.0.1`. Config-only (`Build/Compile = true`, без Go, без
  скачивания). Ставит **один** файл → `/etc/config/darkcore`.
- Схема `darkcore.*` и кто реально читает:

  | ключ | дефолт | пишет | читает |
  |---|---|---|---|
  | `darkcore.main.uuid` | `''` | `provision.sh`, LuCI-страница, оператор вручную | `dcvpnupd` (`main.go` `getUuid`), `provision.sh` (idempotency-гейт), LuCI, argon-баннер «не сконфигурирован» |
  | `darkcore.main.version` | `'1.0.0'` | никто | **никто** — мёртвая опция |
  | `darkcore.main.telemetry_enabled` | нет в файле | никто (ни `build.sh`, ни LuCI) | только `dcvpnupd/liveness.go` — гейт всей ветки liveness |
  | `darkcore.provision.token` | `''` | `darkcorewrt/build.sh` при сборке образа (из `$DARKCORE_PROVISION_TOKEN` / `~/.darkcore-provision-token`) | `provision.sh`, `dcvpnupd/liveness.go` |
  | `darkcore.provision.endpoint` | `http://201.34.132.118:3000/api/connections` | никто в рантайме | только `provision.sh` (иначе — идентичный хардкод `ENDPOINT_DEFAULT`) |

- **Вердикт:** секции нужны (без них `dcvpnupd` падает, LuCI-пакет не
  собирается), но `version` мёртвая, `provision.*` — под снос вместе с
  пакетом `darkcore-provision`, `endpoint` дублирует хардкод в
  `provision.sh`.

---

## `darkcore-xray`

- `PKG_VERSION:=1.0.4`, `PKG_RELEASE:=2`, `XRAY_VERSION:=25.1.30`.
  Go-пакет: собирает `github.com/xtls/xray-core` v25.1.30 (тарбол с
  codeload) → `/usr/bin/xray`. `DEPENDS:=$(GO_ARCH_DEPENDS) +ca-bundle`.
  Сборка = стоковый рецепт `feeds/packages/net/xray-core` (без
  `go mod vendor` — см. коммит `9303e3e`).
- Ставит:
  - `/etc/config/xray` (`files/xray.conf`) — `enabled '1'`,
    `confdir '/etc/xray'`, `datadir '/usr/share/xray'`, `dialer ''`,
    `format 'json'`.
  - `/etc/init.d/xray` (`files/xray.init`) — procd, `START=00`, инстанс
    `xray`. Гейт `xray.enabled.enabled=1`. `wait_for_gateway 192.168.2.1`
    (до 30×1 c, дальше «continuing anyway»). Policy routing: `ip rule
    fwmark 1 → table 100`, `local 0.0.0.0/0 dev lo`, `default via
    192.168.2.1`. `nft -f /usr/share/xray/nftables.rulesv46`. Запуск
    `xray run -confdir /etc/xray -format json`, `respawn`,
    `XRAY_LOCATION_ASSET=/usr/share/xray`. `stop`: `nft flush ruleset` +
    откат routing. `service_triggers`: reload на `uci commit xray`.
  - `/etc/init.d/xrayrestart` + `/root/xrayrestart.sh` — **вестигиальное**:
    `START=99`, `sleep 20`, один раз `service xray restart`. Не
    health-check, перекрыто `wait_for_gateway`/`respawn` и рестартом из
    `dcvpnupd`.
  - `/etc/xray/{dns,inbounds,routing,tail_outbounds}.json` — xray сливает
    все `*.json` из `-confdir`.
  - `/usr/share/xray/nftables.rulesv46`.
- Конфиги xray:
  - `dns.json` — резолвер xray: `223.5.5.5`, DoH `1.1.1.1`, DoH
    `dns.google`, `localhost`; host-override `dns.google → 8.8.8.8`.
  - `inbounds.json` — `all-in` (`dokodemo-door`, tproxy, порт `12345`,
    `tcp,udp`, sniffing) + SOCKS `noauth` порт `10808`.
  - `routing.json` — `domainMatcher mph`, `domainStrategy IPIfNonMatch`.
    Балансер `proxy-balancer` (`selector ["proxy"]`, `strategy leastPing`
    — **но observatory-блока в конфигах НЕТ**, health-данных нет).
    Правила по порядку: `geosite:category-ads-all`→`block`; udp/53 от
    `all-in`→`dns-out`; `1.1.1.1`/`8.8.8.8`→`proxy-balancer`;
    `195.66.213.120`→`direct` (IP из /24 старого backend);
    `regexp:\.ru$`→`direct`; `geoip:ru`→`direct`; default `tcp,udp`
    →`proxy-balancer`.
  - `tail_outbounds.json` — **только** `direct` (`freedom`,
    `sockopt.mark 255`), `block` (`blackhole`), `dns-out`.
    **`proxy`-outbound'а тут НЕТ** — его приносит `dcvpnupd` в
    `/etc/xray/proxy.json`. Пока `dcvpnupd` не отработал успешно хотя бы
    раз, у балансера нет членов.
  - `nftables.rulesv46` — `table inet xray`: чейны `prerouting`
    (tproxy `→127.0.0.1:12345` / `[::1]:12345` с mark `0x1`, bypass
    для `127/8`, `192.168/16`, `::1`, `fe80::/10`, `fd00::/8`,
    спец-кейс udp/53, `return` при mark `0xff`), `output` (mark `0x1`
    для локально-исходящего), `divert` (TPROXY established-socket).
- **Вердикт:** обязателен. Вестигиальное внутри: `xrayrestart.*`,
  `option dialer ''` (только для `XRAY_BROWSER_DIALER`).

---

## `dcvpnupd`

- `PKG_VERSION:=0.1.0`. Go, `PKG_SOURCE_PROTO:=local` (из `src/`).
  `DEPENDS:=$(GO_ARCH_DEPENDS)` (шеллит `uci` и `service`). Ставит
  `/usr/bin/dcvpnupd`. Cron `*/5 * * * * dcvpnupd` добавляет
  `darkcorewrt/build.sh` (`add_scripts`).
- `src/main/main.go`:
  - `getUuid()` = `uci get darkcore.main.uuid`. Проверка
    `err != nil || uuid == ""` → `exit 1`; но при пустом `uuid` `uci`
    отдаёт `"\n"` с кодом 0 → проверка проходит, дальше в URL уходит
    пустой uuid (**латентный баг**).
  - `fetchRouting()` (`routingUrl`, без uuid) и `fetchData(uuid)`
    (`urlTemplate`) — ошибки держатся в разных переменных намеренно
    (`f622045`: раньше вторая перетирала первую, пустой routing затирал
    рабочий).
  - `writeIfChanged`: пустой ответ отбрасывает; при изменении файла —
    `os.WriteFile` + `service xray restart`. За один прогон может
    рестартнуть xray **до 2 раз** (отдельно для `proxy.json` и
    `routing.json`).
  - Затем `reportLiveness(uuid)`. Явного `os.Exit` нет → **exit 0 даже
    если обе загрузки упали** (единственный ненулевой выход — пустой
    uuid).
- `src/main/liveness.go` — **инертен**: `telemetry_enabled` нигде не
  ставится (`telemetryEnabled()` → false → мгновенный return), и ни
  один xray-конфиг не поднимает observatory / gRPC-API на
  `127.0.0.1:8081`, так что `collectObservations` всё равно не
  подключился бы. Тянет за собой `grpc`/`protobuf` в `src/go.mod` (пин
  `48bc349`) и `src/xray/observatory/**` (3 сгенерённых файла:
  `command_grpc.pb.go`, `command.pb.go`, `config.pb.go`). POST на
  `.../liveness` с `X-Darkcore-Token`, принимает 200/201/204 (`00b5a26`).
- `ucitrack`: `darkcorewrt/packages/luci-app-darkcore` регистрирует
  `ucitrack.@darkcore[-1].exec='/usr/bin/dcvpnupd'`, чтобы «Save & Apply»
  на LuCI-странице прогонял `dcvpnupd` сразу, не дожидаясь крона.
- **Вердикт:** обязателен ради `proxy.json` (per-uuid VLESS-outbound —
  единственное реально динамическое, per-device). Под снос:
  - `fetchRouting()` / `routingUrl` / `routingPath` — `routingconfig`
    тянется без uuid, т.е. fleet-global; политику маршрутизации
    правильнее возить обновлением пакета `darkcore-xray`
    (git/версии/ревью), а не HTTP-блобом каждые 5 мин. Churn IP/доменов
    закрывает `geoupdate`.
  - вся ветка `liveness.go` + `src/xray/observatory/**` + `require`
    grpc/protobuf — мёртвый груз и источник боли со сборкой protobuf на
    golang-feed 24.10.
  - После обеих чисток `main.go` = `getUuid → fetch config →
    writeIfChanged → restart`, и у backend остаётся один endpoint.

---

## `geoupdate`

- `PKG_VERSION:=0.0.5`. Go, `PKG_SOURCE_PROTO:=local`, zero deps
  (`go.mod` без `require`). Ставит `/usr/bin/geoupdate`. Cron
  `15 0 * * * geoupdate` (`darkcorewrt/build.sh`).
- `src/main/main.go`: качает `geoip.dat` / `geosite.dat` с
  `raw.githubusercontent.com/runetfreedom/russia-v2ray-rules-dat/release/`
  в `/tmp/geo-xray`, копирует в `/usr/share/xray`, чистит tmp. **xray не
  перезапускает** — новые `.dat` подхватываются на следующем рестарте
  (рестарт из `dcvpnupd` / boot `xrayrestart` / reboot). Любая ошибка →
  `os.Exit(1)`.
- `darkcorewrt/build.sh` тем же `wget` кладёт эти файлы в образ при
  сборке, так что `geoupdate` только освежает.
- **Вердикт:** не критичен (правила `geoip:ru` / `geosite:*` работают со
  снапшота в образе, просто устаревают). Ничто не `DEPENDS`. Мелкий баг:
  финальный `copyFile` не atomic (нет temp+rename) — прерванная копия
  может обрезать `/usr/share/xray/geoip.dat`.

---

## `darkcore-provision` — НЕ активен в прошивке

- `PKG_VERSION:=1.0.0`, `DEPENDS:=+curl`. Config/script-only. Ставит
  `/usr/bin/darkcore-provision` (`provision.sh`) + `/etc/init.d/darkcore-provision`
  (procd, `START=99`, `respawn 60 30 5`).
- `provision.sh`: идемпотентно — если `darkcore.main.uuid` не пуст,
  `exit 0`. Иначе: токен из `uci get darkcore.provision.token` (пуст →
  `exit 1`); `endpoint` из UCI или `ENDPOINT_DEFAULT`
  (`http://201.34.132.118:3000/api/connections`); `new_uuid` из
  `/proc/sys/kernel/random/uuid`; до 10 попыток
  `POST $endpoint` `{"uuid": "..."}` с `X-Darkcore-Token`; на 200/201 —
  `uci set darkcore.main.uuid` + commit + `exit 0`; на 401/403 — сдаётся.
  Сервер заводит клиента выключенным (`enable=false`).
- **Почему не работает:** `darkcorewrt/scripts/add_packages.sh` **не**
  ставит `CONFIG_PACKAGE_darkcore-provision=y` → ни бинарь, ни init в
  образ не попадают. `darkcorewrt/build.sh` `add_provision_token()` кладёт
  в `/etc/uci-defaults/99-darkcore-provision` только
  `darkcore.provision.token`, но не сам скрипт. Ничто на пакет не
  `DEPENDS`. То есть саморегистрация на первом бутe в шипнутых образах
  **не существует** — UUID заводится только вручную (LuCI-страница или
  `uci`).
- CI (`.github/workflows/build-packages.yml`) пакет всё же собирает в
  opkg-feed (ветка `feed`), так что через `opkg` он ставится.
- **Вердикт:** удалить пакет (решение 2026-09-04). Ничто из работающего
  сегодня не сломается.

---

## Карта API backend

Текущий (в коде): `http://201.34.132.118:3000`. Заголовок авторизации
везде: `X-Darkcore-Token: <token>`.

| метод | путь | кто зовёт | назначение | судьба |
|---|---|---|---|---|
| `GET` | `/api/connections/{uuid}/config/` | `dcvpnupd` | → `/etc/xray/proxy.json` | **остаётся** |
| `GET` | `/api/connections/routingconfig` | `dcvpnupd` | → `/etc/xray/routing.json` | под удаление (routing → статикой в `darkcore-xray`) |
| `POST` | `/api/connections/{uuid}/liveness` | `dcvpnupd/liveness.go` (инертно) | наблюдения observatory | под удаление |
| `POST` | `/api/connections` | `provision.sh` (в образе не работает) | саморегистрация | под удаление (с пакетом) |

**Новый backend (решение 2026-09-04):** `https://sub.special-wifi.ru`,
per-uuid конфиг — `https://sub.special-wifi.ru/api/v1/vpn/box/<UUID>/config/`
(сменились схема HTTP→HTTPS, хост IP→домен **и структура путей**).

Точки, где адрес зашит:
- `darkcore-main/files/darkcore.conf` — `darkcore.provision.endpoint`
  (UCI, меняется без пересборки).
- `darkcore-provision/files/provision.sh` — `ENDPOINT_DEFAULT` (shell,
  без пересборки; пакет под удаление).
- `dcvpnupd/src/main/main.go` — `const urlTemplate`, `const routingUrl`
  (**зашиты в бинарь**, нужна пересборка).
- `dcvpnupd/src/main/liveness.go` — `const livenessURLTemplate` (в
  бинарь).
- `dcvpnupd` **не читает** `darkcore.provision.endpoint` из UCI — только
  `provision.sh` читает. На будущее решено: `dcvpnupd` должен читать
  базовый адрес из UCI (`darkcore.main.api_base`), как `provision.sh`.

Не backend, но рядом: `darkcore-xray/files/configs/routing.json` правило
`ip ["195.66.213.120"] → direct` — тот же /24, что старый backend; при
переезде на домен «direct по IP» неприменимо, правило под пересмотр.

---

## Мёртвое / вестигиальное (сводка)

- `darkcore.main.version` — не читает никто.
- `dcvpnupd/liveness.go` + `src/xray/observatory/**` + `require`
  grpc/protobuf в `src/go.mod` — инертны без `telemetry_enabled=1` **и**
  xray observatory gRPC на `127.0.0.1:8081`.
- `dcvpnupd` `fetchRouting()` / `routingUrl` — fleet-global HTTP-блоб
  каждые 5 мин; правильнее статикой в `darkcore-xray`.
- `darkcore-xray/files/xrayrestart.init` + `xrayrestart.sh` — слепой
  одноразовый рестарт, перекрыт `wait_for_gateway` / `respawn` / рестартом
  из `dcvpnupd`; не health-check.
- `darkcore-xray` `option dialer ''` — только для `XRAY_BROWSER_DIALER`.
- `darkcore-provision` — целиком (в образе не активен).
- `darkcore.provision.endpoint` — дублирует хардкод `ENDPOINT_DEFAULT`.

---

## Триггеры и установка (со стороны `darkcorewrt`)

- `scripts/add_packages.sh` — `git clone` этого репо в
  `friendlywrt/package/darkcore` + `CONFIG_PACKAGE_*` для четырёх
  пакетов (без `darkcore-provision`).
- `build.sh` `add_scripts()` дописывает в `/etc/crontabs/root`:
  - `0 0 * * * curl -fsSL ".../special-router/darkcore-updater/main/update.sh" | sh`
  - `15 0 * * * geoupdate`
  - `*/5 * * * * dcvpnupd`
- `build.sh` `add_provision_token()` → `/etc/uci-defaults/99-darkcore-provision`
  (только `darkcore.provision.token`, если задан
  `$DARKCORE_PROVISION_TOKEN` / `~/.darkcore-provision-token`).
- `build.sh` — `wget` `geoip.dat` / `geosite.dat` в
  `${ROOTFS}/usr/share/xray/` при сборке образа.
- init `/etc/init.d/xray` и `/etc/init.d/xrayrestart` включаются на
  финализации образа (без явного `enable` в Makefile — дефолт OpenWrt).

## Сборка пакетов

- Go-пакеты (`dcvpnupd`, `geoupdate`, `darkcore-xray`) — через
  `feeds/packages/lang/golang/golang-package.mk`. У `dcvpnupd`/`geoupdate`
  `PKG_SOURCE_PROTO:=local` (исходники из `src/`), у `darkcore-xray` —
  тарбол xray-core с codeload.
- CI `.github/workflows/build-packages.yml` (`workflow_dispatch`): матрица
  из 5 пакетов, OpenWrt SDK 24.10.4 rockchip/armv8, `make
  package/<pkg>/compile`, публикация подписанного opkg-feed в ветку
  `feed`.
- Ручная сборка через `darkcorewrt/build.sh` каждый раз идёт в свежем
  `friendlywrt24-<dev>/` с новым `dl/go-mod-cache`; прерванный прогон
  оставляет частично распакованные модули (например пустой
  `protobuf@.../internal/editiondefaults/`) → `import lookup disabled by
  -mod=vendor` / `pattern ... no matching files found`. Лечение: снести
  распакованные деревья в `dl/go-mod-cache` (оставив `cache/`) либо весь
  `dl/go-mod-cache`, не прерывать прогон.

## Связанные задачи в `darkcorewrt`

- LuCI-страница «Special Router» — `darkcorewrt/packages/luci-app-darkcore`
  (ручной ввод UUID, статус/управление xray, брендинг). Фразы «авто-
  регистрация делается `darkcore-provision`» в старых заметках **неверны**.
- Периодический health-check xray (`TODO.md` 3b) — не реализован;
  частично закрывается планируемым fail-open (`fallbackTag: direct` +
  `burstObservatory`) в `darkcore-xray`.
- Авто-DNS LAN-клиентам по DHCP (`TODO.md` 3c) — сделано в
  `darkcorewrt/build.sh` (`add_lan_dhcp_dns`, `dhcp.lan.dhcp_option='6,...'`),
  не здесь.
