# darkcore-packages

OpenWrt/FriendlyWrt package feed с **backend-компонентами** прошивки
**Special Router** (бывш. Darkcore). Категория в menuconfig — `DarkCore`.

Клонируется на лету при сборке из основного репозитория **darkcorewrt**
(`/home/trytoca7ch/Projects/darkcore/darkcorewrt`):
`scripts/add_packages.sh` делает
`git clone https://github.com/special-router/darkcore-packages.git darkcore --depth 1 -b main`
в `friendlywrt/package/darkcore` и включает
`CONFIG_PACKAGE_{darkcore-xray,darkcore-main,geoupdate,dcvpnupd}=y`.

Здесь только UCI-конфиги, init-скрипты и Go-бинарники. **LuCI-приложения
/ темы здесь нет** — оно живёт отдельным пакетом в
`darkcorewrt/packages/luci-app-darkcore` (страница ввода UUID + статус
xray + брендинг).

---

## Пакеты

| пакет | версия | роль |
|---|---|---|
| `darkcore-main` | 1.1.0 | UCI-каркас `/etc/config/darkcore` (`uuid`, `api_base`) |
| `darkcore-xray` | 1.0.4-3 | сам прокси (xray-core) + routing + nft + fail-open |
| `dcvpnupd` | 0.2.0 | cron `*/5`: тянет per-uuid `proxy.json` с backend, рестартит xray |
| `geoupdate` | 0.0.5 | cron `15 0`: освежает `geoip.dat`/`geosite.dat` |

`darkcore-provision` **удалён** (2026-09-04) — он никогда не подключался в
образ (`add_packages.sh` не ставил `CONFIG_PACKAGE_darkcore-provision`),
саморегистрация на первом бутe не работала. UUID заводится вручную через
LuCI-страницу или `uci`.

---

## `darkcore-main`

Config-only (`Build/Compile = true`). Ставит `/etc/config/darkcore`:

```
config darkcore 'main'
	option uuid ''
	option api_base 'https://sub.special-wifi.ru'
```

| ключ | читает |
|---|---|
| `darkcore.main.uuid` | `dcvpnupd` (`getUuid`), LuCI-страница, argon-баннер «не сконфигурирован» |
| `darkcore.main.api_base` | `dcvpnupd` (`getAPIBase`); пусто → вкомпилированный дефолт `defaultAPIBase` |

Смена backend для партии плат = `uci set darkcore.main.api_base=...; uci commit darkcore`
(без пересборки; `dcvpnupd` подхватит на следующем прогоне крона).

---

## `darkcore-xray`

Go-пакет: `xray-core` v25.1.30 (тарбол с codeload) → `/usr/bin/xray`.
Сборка = стоковый рецепт `feeds/packages/net/xray-core` без `go mod vendor`
(коммит `9303e3e`; если сборка падает на модулях — чинить сеть/токен, не
возвращать вендоринг). `DEPENDS: $(GO_ARCH_DEPENDS) +ca-bundle`.

Ставит:
- `/etc/config/xray` (`files/xray.conf`) — `enabled '1'`,
  `confdir '/etc/xray'`, `datadir '/usr/share/xray'`, `format 'json'`.
- `/etc/init.d/xray` (`files/xray.init`) — procd, `START=00`, инстанс
  `xray`. Гейт `xray.enabled.enabled=1`. `wait_for_gateway 192.168.2.1`
  (30×1 c, дальше «continuing anyway»). Policy routing: `ip rule
  fwmark 1 → table 100`, `local 0.0.0.0/0 dev lo`,
  `default via 192.168.2.1`. `nft -f /usr/share/xray/nftables.rulesv46`.
  Запуск `xray run -confdir /etc/xray -format json`, `respawn`. `stop`:
  `nft flush ruleset` + откат routing. `service_triggers`: reload на
  `uci commit xray`.
- `/etc/xray/{dns,inbounds,routing,observatory,tail_outbounds}.json` —
  xray сливает все `*.json` из `-confdir`.
- `/usr/share/xray/nftables.rulesv46`.

Конфиги xray:
- `dns.json` — резолвер xray: `223.5.5.5`, DoH `1.1.1.1`, DoH `dns.google`,
  `localhost`; host-override `dns.google → 8.8.8.8`.
- `inbounds.json` — `all-in` (`dokodemo-door`, tproxy, порт `12345`) +
  SOCKS `noauth` порт `10808`.
- `routing.json` — `domainMatcher mph`, `domainStrategy IPIfNonMatch`.
  Балансер `proxy-balancer` (`selector ["proxy"]`, `strategy leastPing`,
  **`fallbackTag: "direct"`**). Правила по порядку:
  `geosite:category-ads-all`→`block`; udp/53 от `all-in`→`dns-out`;
  **`full:sub.special-wifi.ru`→`direct`** (backend должен быть доступен
  мимо прокси, иначе `dcvpnupd` не восстановится при упавшем VPN);
  `1.1.1.1`/`8.8.8.8`→`proxy-balancer`; `regexp:\.ru$`→`direct`;
  `geoip:ru`→`direct`; default `tcp,udp`→`proxy-balancer`.
- `observatory.json` — `burstObservatory` (`subjectSelector ["proxy"]`,
  проба `https://www.gstatic.com/generate_204`, `interval 30s`,
  `sampling 3`, `timeout 10s`). Даёт health-данные, которые нужны и
  `fallbackTag`, и `leastPing`. Открывает **не** gRPC-API — просто
  внутренний health-check.
- `tail_outbounds.json` — только `direct` (`freedom`, `sockopt.mark 255`),
  `block` (`blackhole`), `dns-out`. **`proxy`-outbound приносит
  `dcvpnupd`** в `/etc/xray/proxy.json`. Пока `dcvpnupd` не отработал
  успешно, у балансера нет членов → всё уходит в `fallbackTag: direct`.
- `nftables.rulesv46` — `table inet xray`: `prerouting` (tproxy
  `→127.0.0.1:12345` / `[::1]:12345`, bypass `127/8`, `192.168/16`,
  `::1`, `fe80::/10`, `fd00::/8`, спец-кейс udp/53, `return` при mark
  `0xff`), `output` (mark `0x1` для локально-исходящего), `divert`
  (TPROXY established-socket).

### fail-open — как работает
Провайдер режет VLESS-эндпоинт → `burstObservatory` за ~30-90 c помечает
все `proxy`-outbound'ы мёртвыми → `proxy-balancer` отдаёт трафик в
`fallbackTag: direct` (чистый интернет без VLESS), **соединение не
рвётся**. Когда прокси снова отвечает — балансер сам возвращает трафик на
него.

**Ещё не сделано (план task 4c):** если xray вообще не поднялся (backend
отдал битый `proxy.json`), nft-правила TPROXY остаются загруженными →
блэкхол. Нужны: `xray run -test` в `dcvpnupd` перед заменой файла +
watchdog в `xray.init`, снимающий nft-правила если xray не запущен.

---

## `dcvpnupd`

Go, `PKG_SOURCE_PROTO:=local` (из `src/`), **stdlib-only** (после
удаления liveness/grpc — коммит `5758155`). `DEPENDS: $(GO_ARCH_DEPENDS)
+ca-bundle`. Ставит `/usr/bin/dcvpnupd`. Cron `*/5 * * * * dcvpnupd`
добавляет `darkcorewrt/build.sh` (`add_scripts`).

`src/main/main.go` — весь control flow:
1. `getUuid()` = `uci get darkcore.main.uuid`, `TrimSpace`. Пусто/ошибка
   → `os.Exit(1)`.
2. `getAPIBase()` = `uci -q get darkcore.main.api_base`, иначе
   `defaultAPIBase` (`https://sub.special-wifi.ru`).
3. `fetchConfig(base, uuid)` — `GET <base>/api/v1/vpn/box/<uuid>/config/`
   (без auth-заголовка, как и раньше). Не 200 → `APIError`, лог, выход.
4. `writeIfChanged("/etc/xray/proxy.json", body)` — пустой ответ
   отбрасывает; при изменении файла — `os.WriteFile` +
   `service xray restart`.

`ucitrack`: `darkcorewrt/packages/luci-app-darkcore` регистрирует
`ucitrack.@darkcore[-1].exec='/usr/bin/dcvpnupd'`, чтобы «Save & Apply»
на LuCI-странице прогонял `dcvpnupd` сразу.

**Что убрано (`5758155`):** `fetchRouting()` / `routingUrl` /
`routingPath` (routing теперь статикой в `darkcore-xray`); `liveness.go` +
`src/xray/observatory/**` + grpc/protobuf из `go.mod`/`go.sum` (ветка была
инертна — `telemetry_enabled` не ставился, observatory gRPC не поднимался).

---

## `geoupdate`

Go, `PKG_SOURCE_PROTO:=local`, zero deps. Ставит `/usr/bin/geoupdate`.
Cron `15 0 * * * geoupdate` (`darkcorewrt/build.sh`). Качает `geoip.dat` /
`geosite.dat` с
`raw.githubusercontent.com/runetfreedom/russia-v2ray-rules-dat/release/`
в `/tmp/geo-xray`, копирует в `/usr/share/xray`, чистит tmp. **xray не
перезапускает** — новые `.dat` подхватываются на следующем рестарте.
`darkcorewrt/build.sh` тем же `wget` кладёт эти файлы в образ при сборке,
так что `geoupdate` только освежает. Мелкий баг: финальный `copyFile` не
atomic (нет temp+rename).

---

## Backend API

Один used endpoint:

| метод | путь | назначение |
|---|---|---|
| `GET` | `<api_base>/api/v1/vpn/box/<uuid>/config/` | → `/etc/xray/proxy.json` (per-uuid VLESS-outbound) |

`api_base` по умолчанию — `https://sub.special-wifi.ru` (в двух местах:
`darkcore-main/files/darkcore.conf` и `defaultAPIBase` в
`dcvpnupd/src/main/main.go`). Переопределяется через
`uci set darkcore.main.api_base=...` без пересборки.

Заголовок авторизации не нужен: сам UUID в пути — секрет и признак
реального устройства. `fetchConfig` шлёт голый `http.Get` (так же, как
старый endpoint).

История адресов: `195.66.213.74:3000` → `201.34.132.118:3000/api/connections`
→ `https://sub.special-wifi.ru/api/v1/vpn/box/<uuid>/config/`.

---

## Триггеры и установка (со стороны `darkcorewrt`)

- `scripts/add_packages.sh` — `git clone` этого репо в
  `friendlywrt/package/darkcore` + `CONFIG_PACKAGE_*` для четырёх пакетов.
- `build.sh` `add_scripts()` дописывает в `/etc/crontabs/root`:
  - `0 0 * * * curl -fsSL ".../special-router/darkcore-updater/main/update.sh" | sh`
  - `15 0 * * * geoupdate`
  - `*/5 * * * * dcvpnupd`
- `build.sh` — `wget` `geoip.dat` / `geosite.dat` в
  `${ROOTFS}/usr/share/xray/` при сборке образа.
- init `/etc/init.d/xray` включается на финализации образа (без явного
  `enable` в Makefile — дефолт OpenWrt).

## Сборка пакетов

- Go-пакеты — через `feeds/packages/lang/golang/golang-package.mk`. У
  `dcvpnupd`/`geoupdate` `PKG_SOURCE_PROTO:=local`, у `darkcore-xray` —
  тарбол xray-core с codeload.
- CI `.github/workflows/build-packages.yml` (`workflow_dispatch`): матрица
  из 4 пакетов, OpenWrt SDK 24.10.4 rockchip/armv8, `make
  package/<pkg>/compile`, публикация подписанного opkg-feed в ветку
  `feed`.
- Ручная сборка через `darkcorewrt/build.sh` идёт в свежем
  `friendlywrt24-<dev>/` с новым `dl/go-mod-cache`; прерванный прогон
  оставляет частично распакованные модули → `import lookup disabled by
  -mod=vendor` / `pattern ... no matching files found`. Лечение: снести
  распакованные деревья в `dl/go-mod-cache` (оставив `cache/`) либо весь
  `dl/go-mod-cache`, не прерывать прогон. (После `5758155` `dcvpnupd`
  stdlib-only — эта боль остаётся только у `darkcore-xray`.)

## Проверка после смены адреса (на живой плате)

- `logread | grep dcvpnupd` — без `x509` / `no such host` /
  `connection refused`;
- `curl -sS https://sub.special-wifi.ru/api/v1/vpn/box/<uuid>/config/` с
  платы → `200` + валидный JSON;
- `/etc/xray/proxy.json` обновился, `service xray restart` в логе, xray
  поднялся (`ubus call service list '{"name":"xray"}'`);
- fail-open: заблокировать VLESS-сервер → через ~1-2 мин внешний IP
  клиента становится IP роутера (пошёл `direct`), соединение живо; снять
  блок → трафик вернулся на прокси.

## Связанные задачи в `darkcorewrt`

- LuCI-страница «Special Router» — `darkcorewrt/packages/luci-app-darkcore`
  (ручной ввод UUID, статус/управление xray, брендинг).
- Периодический health-check xray (`TODO.md` 3b) — частично закрыт
  fail-open выше; полностью — с task 4c (watchdog).
- Авто-DNS LAN-клиентам по DHCP (`TODO.md` 3c) — сделано в
  `darkcorewrt/build.sh` (`add_lan_dhcp_dns`), не здесь.
