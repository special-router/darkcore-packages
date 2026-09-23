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
`darkcorewrt/packages/luci-app-darkcore` (страница ввода кода активации +
статус xray + брендинг).

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
удаления liveness/grpc — коммит `5758155`; JSON-обмен для активации —
`encoding/json`, тоже stdlib). `DEPENDS: $(GO_ARCH_DEPENDS) +ca-bundle`.
Ставит `/usr/bin/dcvpnupd`. Cron `*/5 * * * * dcvpnupd` добавляет
`darkcorewrt/build.sh` (`add_scripts`).

**2026-09: миграция с UUID на код активации** (backend сменился на
`special-wifi.link`, целевой сервис — sing-box вместо xray). UUID больше
не используется. `src/main/main.go` — весь control flow:

1. `ensureActivated()` — сперва пробует `darkcore.main.device_token` +
   `darkcore.main.config_url` из UCI (`uci -q get`, так что отсутствие
   ключа — не ошибка). Если оба есть — отдаёт их как есть (обычный путь
   при каждом прогоне крона).
2. Если токена нет — читает `darkcore.main.activation_code`. Пусто →
   лог «не активировано», `os.Exit(1)`. Есть → `activate(base, code)`:
   `POST <base>/api/v1/vpn/router/activate/` с `{"code": "..."}`, ответ
   `{"device_token": "...", "config_url": "..."}`. Не 200 или пустые
   поля → ошибка, выход.
3. После успешной активации — `uci set` для `device_token`/`config_url`,
   `uci set darkcore.main.activation_code=''` (код одноразовый, чтобы не
   переактивироваться на следующем прогоне крона) и `uci commit
   darkcore`.
4. `fetchConfig(configURL, deviceToken)` — `GET <config_url>` с
   `Authorization: Bearer <device_token>`. Не 200 → `APIError`, лог,
   выход.
5. `writeIfChanged(configPath, body)` — пустой ответ отбрасывает; при
   изменении файла — `os.WriteFile` + `service <targetService> restart`.

`getAPIBase()` не изменился по форме: `darkcore.main.api_base`, иначе
вкомпилированный дефолт — теперь `https://special-wifi.link`.

**`configPath`/`targetService` — провизорные константы**
(`/etc/sing-box/proxy.json`, `sing-box`): в `darkcore-packages` пока нет
пакета sing-box (нет confdir, нет init-скрипта, нет `/etc/config/sing-box`
— это отдельная задача, аналог заведения `darkcore-xray`). `config_url`
уже отдаёт готовый JSON (`log`+`outbounds`+`route`, без `inbounds`/`dns`
— по всей видимости sing-box, как и xray, будет мержить несколько файлов
через `-C confdir`), так что `dcvpnupd` по-прежнему просто пишет байты
как есть, без сборки/мержа на своей стороне.

**Что не обрабатывается:** протухший/невалидный `device_token` (401 от
`config_url`) не триггерит повторную активацию — `activation_code` к
этому моменту уже очищен и нового взять неоткуда без участия
пользователя. Если это окажется реальным сценарием — понадобится
отдельный сигнал «токен отозван» с backend или ручной ввод нового кода
через LuCI.

`ucitrack`: `darkcorewrt/packages/luci-app-darkcore` регистрирует
`ucitrack.@darkcore[-1].exec='/usr/bin/dcvpnupd'`, чтобы «Save & Apply»
на LuCI-странице (ввод кода активации) прогонял `dcvpnupd` сразу — тем
самым активация происходит немедленно, а не ждёт следующего тика крона.
UCI-схема (`activation_code` и то, что её вводит LuCI) заводится в
`darkcorewrt` — вне `dcvpnupd`.

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

**2026-09: старый UUID/`sub.special-wifi.ru`-flow заменён на активацию
по одноразовому коду** (см. `dcvpnupd` выше). Два эндпоинта:

| метод | путь | назначение |
|---|---|---|
| `POST` | `<api_base>/api/v1/vpn/router/activate/` | тело `{"code": "<одноразовый код>"}` → `{"device_token": "...", "config_url": "..."}` |
| `GET` | `<config_url>` (из ответа выше), заголовок `Authorization: Bearer <device_token>` | → `configPath` в `dcvpnupd` (готовый sing-box JSON: `log`+`outbounds`+`route`) |

`api_base` по умолчанию — `https://special-wifi.link` (вкомпилирован как
`defaultAPIBase` в `dcvpnupd/src/main/main.go`; в `darkcore-main` не
проверялось — вне скоупа этой правки). Переопределяется через
`uci set darkcore.main.api_base=...` без пересборки.

`activate/` — без авторизации (код в теле — секрет и признак
активации); `config_url` — с `Bearer`-токеном, полученным от `activate/`.
`device_token`/`config_url` сохраняются в UCI после первой активации,
`activation_code` одноразовый и чистится сразу после использования.

История адресов: `195.66.213.74:3000` → `201.34.132.118:3000/api/connections`
→ `https://sub.special-wifi.ru/api/v1/vpn/box/<uuid>/config/` (UUID-flow,
заменён) → `https://special-wifi.link/api/v1/vpn/router/activate/` +
per-device `config_url` (текущий, код-активации flow).

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
