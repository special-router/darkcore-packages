# darkcore-packages

OpenWrt/FriendlyWrt package feed с **backend-компонентами** прошивки
**Special Router** (бывш. Darkcore). Категория в menuconfig — `DarkCore`.

Клонируется на лету при сборке из основного репозитория **darkcorewrt**
(`/home/trytoca7ch/Projects/darkcore/darkcorewrt`):
`scripts/add_packages.sh` делает
`git clone https://github.com/special-router/darkcore-packages.git darkcore --depth 1 -b main`
в `friendlywrt/package/darkcore` и включает
`CONFIG_PACKAGE_{darkcore-singbox,darkcore-main,geoupdate,dcvpnupd}=y`.

Здесь только UCI-конфиги, init-скрипты и Go-бинарники. **LuCI-приложения
/ темы здесь нет** — оно живёт отдельным пакетом в
`darkcorewrt/packages/luci-app-darkcore` (страница ввода кода активации +
статус sing-box + брендинг).

---

## Пакеты

| пакет | версия | роль |
|---|---|---|
| `darkcore-main` | 1.2.0 | UCI-каркас `/etc/config/darkcore` (`activation_code`, `device_token`, `config_url`, `api_base`) |
| `darkcore-singbox` | 1.12.4-1 | сам прокси (sing-box) + routing + nft + config-валидация + fail-open watchdog |
| `dcvpnupd` | 0.3.1 | cron `*/5`: меняет одноразовый код на device_token, тянет sing-box конфиг с backend, рестартит сервис |
| `geoupdate` | 0.0.5 | cron `15 0`: освежает `geoip.dat`/`geosite.dat` (задел под xray, сейчас мёртвый груз — см. ниже) |

`darkcore-provision` **удалён** (2026-09-04) — он никогда не подключался в
образ (`add_packages.sh` не ставил `CONFIG_PACKAGE_darkcore-provision`),
саморегистрация на первом бутe не работала. Код активации заводится
вручную через LuCI-страницу или `uci` (см. ниже, «2026-09: миграция с
UUID на код активации»).

---

## `darkcore-main`

Config-only (`Build/Compile = true`). Ставит `/etc/config/darkcore`:

```
config darkcore 'main'
	option activation_code ''
	option device_token ''
	option config_url ''
	option api_base 'https://special-wifi.link'
```

| ключ | читает |
|---|---|
| `darkcore.main.activation_code` | `dcvpnupd` (`ensureActivated`, одноразовый — сам же и чистит), LuCI-страница |
| `darkcore.main.device_token` | `dcvpnupd` (`ensureActivated`/`fetchConfig`, `Bearer`), argon-баннер «не сконфигурирован» |
| `darkcore.main.config_url` | `dcvpnupd` (`fetchConfig`) |
| `darkcore.main.api_base` | `dcvpnupd` (`getAPIBase`); пусто → вкомпилированный дефолт `defaultAPIBase` |

Смена backend для партии плат = `uci set darkcore.main.api_base=...; uci commit darkcore`
(без пересборки; `dcvpnupd` подхватит на следующем прогоне крона).

---

## `darkcore-singbox`

**2026-09: заменяет `darkcore-xray` полностью** (не сосуществуют).
Причина — не выбор, а необходимость: бэкенд отдаёт конфиг в нативной
схеме sing-box (`"type":"vless"`, `"server"`/`"server_port"`,
`"tls":{"reality":{...},"utls":{...}}`, группы `"type":"urltest"`/
`"type":"selector"`), а не в xray/v2ray-схеме
(`"protocol":"vless"`+`"streamSettings"`) — и обратной конвертации без
потерь не существует (вложенные `selector`-группы по странам не имеют
аналога в плоской модели `routing.balancers` у xray). Разбор вариантов —
`~/.claude/plans/greedy-waddling-papert.md` (План A против Плана B).

Go-пакет: `sing-box` v1.12.4 (тарбол с codeload,
`github.com/sagernet/sing-box`) → `/usr/bin/sing-box`. Сборка — тот же
паттерн, что у `darkcore-xray` (golang-package.mk, без вендоринга, без
зависимости на апстримный `feeds/packages/net/sing-box` — собираем сами,
чтобы `GO_PKG_TAGS` (`with_utls,with_quic,with_clash_api`) были явно в
своём Makefile, а не в Kconfig-меню апстрима, которое
`scripts/add_packages.sh` не умеет выставлять). `with_utls` обязателен —
без него vless+Reality-outbound'ы бэкенда не устанавливают TLS.
`DEPENDS: $(GO_ARCH_DEPENDS) +ca-bundle +kmod-inet-diag`.

Ставит:
- `/etc/config/sing-box` (`files/sing-box.conf`) — `enabled '1'`,
  `user 'root'` (TPROXY + policy routing требуют root, как и у xray),
  `confdir '/etc/sing-box/conf.d'`, `workdir '/usr/share/sing-box'`.
- `/etc/init.d/sing-box` (`files/sing-box.init`) — procd, `START=99`
  (**было `START=00` - критичный баг, найден 2026-09-29 на реальном
  железе**: при `00` скрипт запускался ПЕРВЫМ вообще из всех
  `/etc/rc.d/S*` - раньше `fstab`(S11), `rpcd`(S12), `dnsmasq`/
  `dropbear`(S19), `network`(S20). `start_service` делает `ip rule`/
  `ip route`/`nft -f`/`procd_open_instance` - последнее общается с procd
  через ubus, который в это время ещё не факт что готов принимать
  запросы (rpcd/ubus только на S12). Результат - весь boot-sequence
  вставал колом на этом шаге (OpenWrt гоняет `/etc/rc.d/S*` строго
  последовательно), `dropbear`/`uhttpd` не поднимались НИКОГДА - роутер
  пинговался (сеть через отдельный hotplug-путь), но SSH/LuCI 100%
  недоступны. Судя по `sing-box-watchdog`'у на `START=98` "на 1 раньше
  sing-box" - `99` для sing-box и был исходно задуман, просто опечатка
  при написании init-скрипта, которую не поймали раньше, т.к. до этого
  тестировали живыми правками поверх уже загруженной системы, а не
  полным циклом сборка→прошивка→загрузка с нуля.),
  инстанс `sing-box`. Та же policy routing, что у `xray.init` (`ip rule
  fwmark 1 → table 100`, `local 0.0.0.0/0 dev lo`,
  `default via 192.168.2.1`, best-effort `wait_for_gateway`), тот же
  `nft -f /usr/share/sing-box/nftables.rulesv46`. **Плюс валидация**:
  `start_service`/`restart_service` гоняют `sing-box check -C "$confdir"`
  перед запуском/рестартом; если конфиг битый — не трогают уже
  работающий процесс и откатывают `$confdir` на снимок
  `/etc/sing-box/.lastgood` (обновляется при каждой успешной проверке),
  так что даже ребут после сломанного фетча поднимается с последним
  рабочим конфигом, а не с блэкхолом. Запуск: `sing-box run -C
  "$confdir" -D "$workdir"`, `respawn`. `stop`: `nft delete table inet
  sing-box` (только своя таблица, см. ниже) + откат routing.
- `/etc/init.d/sing-box-watchdog` (`files/sing-box-watchdog.init` +
  `files/sing-box-watchdog.sh`) — отдельный procd-сервис, `START=98`
  (на 1 раньше sing-box). См. «fail-open» ниже.
- `/etc/sing-box/conf.d/{00-base,90-proxy}.json` — sing-box мерджит все
  `*.json` из `-C confdir` по алфавиту. **Проверено вживую на реальном
  железе (2026-09-29): для скаляров побеждает ПЕРВЫЙ (более ранний по
  алфавиту) файл, а не последний.** Причина — реализация в
  `sing/common/json/badjson/merge.go` (`mergeJSON`): для каждого файла
  вызывается `MergeJSON(source=текущий_файл, destination=накопленное)`,
  и в объектах для уже существующего в `destination` ключа делается
  рекурсивный merge, где скалярный `default:`-кейс просто возвращает
  `destination` (старое значение), source отбрасывается целиком. Массивы
  (`outbounds`) - другое дело, там `destination = append(destination,
  source...)`, конкатенация работает как ожидалось. Из-за этого
  `route.final`, once set in `00-base.json`, был НЕ переопределим из
  `90-proxy.json` - весь трафик уходил `direct`, несмотря на валидный
  активированный конфиг с реальными VLESS-серверами. Фикс: `00-base.json`
  **не должен** задавать `route.final` вообще - тогда ключ у него
  просто отсутствует в объекте `route`, и `90-proxy.json` добавляет его
  как новый (не конфликтующий) ключ. Без активации (`90-proxy.json={}`,
  единственный outbound - `direct`) `outbound.Manager` при пустом
  `defaultTag` сам берёт первый объявленный outbound по умолчанию (см.
  `adapter/outbound/manager.go`) - то есть `direct`, тот же безопасный
  дефолт, но без явного `route.final` в базовом конфиге.
- `/usr/share/sing-box/nftables.rulesv46`.
- `/usr/libexec/darkcore-singbox-profile-status` (`files/
  darkcore-singbox-profile-status.sh`) — для LuCI-страницы
  (`darkcorewrt/packages/luci-app-darkcore`): какой VLESS-профиль сейчас
  реально выбран группой `GLOBAL AUTO` и живой пинг до него. Читает
  Clash API sing-box'а (`experimental.clash_api.external_controller:
  127.0.0.1:9090` в `00-base.json`, тег сборки `with_clash_api` -
  **проверено 2026-09-29, собирается чисто на уже имеющемся
  go.sum.fixed**, новых модулей не нужно). Запрашивает саму группу
  `GLOBAL AUTO`, не конкретный тег сервера (`GET /proxies/GLOBAL%20AUTO`
  → `.now` для имени, `GET /proxies/GLOBAL%20AUTO/delay?...` для пинга) -
  у group-outbound'ов `DialContext` делегирует текущему выбранному члену,
  так что percent-encode нужен только для пробела в "GLOBAL AUTO", а не
  для юникода/эмодзи в тегах отдельных серверов (было бы больно кодировать
  побайтово в POSIX shell). Вызывается со страницы через `ubus file.exec`
  (Clash API слушает только 127.0.0.1, из браузера не достать напрямую).
  Если группы/профиля нет (например конфиг ещё не активирован) - отдаёт
  `{"available":false}`, страница показывает «Нет доступных профилей».

Конфиги sing-box:
- `00-base.json` — статика, которой нет в конфиге бэкенда: `dns`
  (`223.5.5.5` + DoH `1.1.1.1`, **оба с `"detour": "direct"`** - иначе
  после фикса `route.final` DNS-резолвинг сам пытается пойти через
  `GLOBAL AUTO`, а чтобы законнектиться к outbound'у, нужно сперва
  зарезолвить его хост через тот же DNS - замкнутый круг, sing-box падает
  с `DNS query loopback in transport[resolver]`. Проверено вживую
  2026-09-29 сразу после фикса `route.final` из пункта выше. `detour`
  форсит резолвинг всегда direct, не завязываясь на `route.final`), `inbounds` (`tproxy` на `12345` — прямой
  аналог xray'евского `dokodemo-door`+tproxy; `mixed` на `127.0.0.1:10808`
  — аналог голого SOCKS-инбаунда xray, и путь пробника вотчдога),
  outbound `direct` (`routing_mark: 255` — аналог xray'евского
  `sockopt.mark 255` на `direct`/`dns-out`, чтобы дозвон наружу не
  зацикливался через TPROXY), **`route.final` сознательно НЕ задан**
  (см. разбор мерджа выше - если задать здесь, `90-proxy.json` не сможет
  его переопределить) + одно правило `hijack-dns` для DNS с
  tproxy-инбаунда.
- `90-proxy.json` — то, что приносит `dcvpnupd` (сырые байты от бэкенда,
  без изменений). На заводской прошивке — плейсхолдер `{}` (валидный
  пустой фрагмент; `direct`-outbound и так есть в `00-base.json`, так что
  неактивированный роутер работает в режиме «всё напрямую» без отдельного
  плейсхолдерного `direct`).
- **Никаких geoip/geosite/ad-block правил** — сознательно не перенесены с
  xray в v1 (нужен `.srs`-формат sing-box + отдельный шаг в `build.sh`,
  отдельная задача на будущее).
- `nftables.rulesv46` — `table inet sing-box`, один в один правила
  `darkcore-xray` (тот же TPROXY-порт `12345`, тот же локальный
  `10808`, те же bypass'ы) — коллизий нет, xray полностью убран.

### fail-open — как работает
У sing-box `urltest` при недоступности всех участников использует
**первый outbound из своего списка**, а не переключается на `direct`
автоматически; `selector` — вообще ручное переключение (через Clash
API). Значит нет xray-подобного `fallbackTag`, на который можно было бы
положиться внутри самого прокси — фейл-опен сделан **снаружи**,
вотчдогом:

`sing-box-watchdog` каждые 15с делает запрос на
`https://cp.cloudflare.com/generate_204` через локальный `mixed`-инбаунд
sing-box (`127.0.0.1:10808` — тот же probe URL, что использует
`urltest` бэкенда, чтобы вотчдог и sing-box не расходились в оценке
«жив/мёртв»). После 3 подряд неудач (~45с) — `nft delete table inet sing-box`:
TPROXY-перехват полностью снимается, трафик LAN идёт в интернет
напрямую через NAT `fw4`. При восстановлении — `nft -f
nftables.rulesv46` поднимает правила обратно.

**2026-10-07: было `nft flush ruleset` - и это ломало fail-open для LAN
(и не только).** `flush ruleset` сносит ВСЕ таблицы, включая `inet fw4`
штатного firewall4 (fw4 сам трогает только свою таблицу - `flush table
inet fw4` в `ruleset.uc`). Последствия: (1) `nftables.rulesv46`
начинался с `flush ruleset`, т.е. на КАЖДОМ старте sing-box (START=99,
после firewall) роутер оставался вообще без firewall - ни reject на
WAN input, ни masquerade, ни forward-правил; LAN при этом работал
только потому, что весь его трафик уходил в TPROXY локально; (2) на
`stop`/фейл-опене вотчдога LAN-клиенты оставались без NAT → без
интернета, т.е. fail-CLOSED. Вотчдог проверяли только с самого роутера
(тому NAT не нужен), поэтому не заметили. Поймано на железе с
Windows-клиентом за роутером: после «Остановить» в LuCI - «Невозможно
соединиться с удалённым сервером». Теперь все три места (rules-файл,
`stop_service`, вотчдог) трогают только `table inet sing-box`;
rules-файл идемпотентен (`table` + `delete table` в начале), проверено
двойным `nft -f`. Отдельный от sing-box
процесс специально: рестарт sing-box каждые 5 минут (`dcvpnupd`) не
должен сбивать состояние вотчдога.

**Проверено вживую (2026-09-29):** директорийный мердж sing-box (`-C
confdir`) - конкатенация `outbounds` работает как ожидалось, но
override скаляров вроде `route.final` работает НАОБОРОТ (побеждает
первый файл, не последний) - см. разбор в начале секции. Fallback на
`jq`-мердж не понадобился - фикс через "просто не задавать `final` в
`00-base.json`" оказался достаточным и проще.

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

**`configPath`/`targetService`**: `/etc/sing-box/conf.d/90-proxy.json` /
`sing-box` — теперь настоящие пути, не провизорные (пакет
`darkcore-singbox` заведён, см. выше). `configPath` указывает внутрь
confdir-директории `darkcore-singbox`'а, а не на отдельный файл: имя с
префиксом `90-` гарантирует, что этот фрагмент мерджится sing-box'ом
после статического `00-base.json` (алфавитный порядок для конкатенации
`outbounds`). Про `route.final` — см. разбор в разделе про конфиги выше:
`00-base.json` его не задаёт специально, чтобы `90-proxy.json` мог
добавить свой `final` как новый ключ, а не проигрывать в конфликте
скаляров. `dcvpnupd` по-прежнему просто пишет байты как есть, без
сборки/мержа на своей стороне — мердж делает сам sing-box через `-C`.

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
`defaultAPIBase` в `dcvpnupd/src/main/main.go`, и теперь тем же значением
шипается в `darkcore-main/files/darkcore.conf`, так что оба места
согласованы). Переопределяется через `uci set darkcore.main.api_base=...`
без пересборки.

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
- init `/etc/init.d/sing-box` и `/etc/init.d/sing-box-watchdog` включаются
  на финализации образа (без явного `enable` в Makefile — дефолт
  OpenWrt).
- `build.sh`'s `wget` `geoip.dat`/`geosite.dat` в `usr/share/xray/`
  **убран** вместе с `darkcore-xray` (v1 sing-box не использует
  geoip-правила). `geoupdate` продолжает качать эти же файлы по крону —
  теперь их никто не читает, дохлый груз (см. `darkcorewrt/TODO.md`).

## Сборка пакетов

- Go-пакеты — через `feeds/packages/lang/golang/golang-package.mk`. У
  `dcvpnupd`/`geoupdate` `PKG_SOURCE_PROTO:=local`, у `darkcore-singbox` —
  тарбол sing-box с codeload, `PKG_HASH` — настоящий sha256 (проставлен,
  сверен с независимой повторной загрузкой того же тега).
- CI `.github/workflows/build-packages.yml` (`workflow_dispatch`): список
  пакетов в `PKGS` (`geoupdate darkcore-singbox darkcore-main dcvpnupd`),
  OpenWrt SDK 24.10.4 rockchip/armv8, `make package/<pkg>/compile`,
  публикация подписанного opkg-feed в ветку `feed`.
- Ручная сборка через `darkcorewrt/build.sh` идёт в свежем
  `friendlywrt24-<dev>/` с новым `dl/go-mod-cache`; прерванный прогон
  оставляет частично распакованные модули → `import lookup disabled by
  -mod=vendor` / `pattern ... no matching files found`. Лечение (если
  всё же понадобится вручную): снести распакованные деревья в
  `dl/go-mod-cache` (оставив `cache/`) либо весь `dl/go-mod-cache`, не
  прерывать прогон.
  **2026-09-29, разобрано на реальном железе — три РАЗНЫХ источника
  этой боли, все теперь фиксятся автоматически в `darkcorewrt/build.sh`:**
  (1) `make -j$(nproc)` на 15GB/8-core машине ловил OOM посреди записи
  файла (и в `toolchain/gcc/initial`, и в Go-компиляции) — `build.sh`
  теперь считает безопасный `-j` из RAM и nproc; (2) несколько
  Go-пакетов (`darkcore-singbox`/`dcvpnupd`/`geoupdate` + чужие feeds)
  параллельно бьют по общему `dl/go-mod-cache` — `build.sh` оборачивает
  шаг `build` в `golang-build.sh` в `flock`; (3) главная причина -
  `mk-friendlywrt.sh`'s `find dl -size -1024c -exec rm -f {} \;` (чистка
  недокачанных архивов) без `-maxdepth 1` рекурсивно косила тысячи
  легитимных Go-исходников <1KB внутри `dl/go-mod-cache` при каждом
  прогоне - `build.sh` теперь патчит это на `-maxdepth 1`. Подробности и
  обоснование каждого фикса - комментарии в `darkcorewrt/build.sh`.

## Проверка активации (на живой плате)

- ввести код активации через LuCI-страницу (или `uci set
  darkcore.main.activation_code=...; uci commit darkcore`) и нажать
  «Save & Apply» — `ucitrack` сразу прогоняет `dcvpnupd`, ждать крона не
  нужно;
- `logread | grep dcvpnupd` — «Устройство активировано», без `x509` /
  `no such host` / `connection refused` / `HTTP 4xx`;
- `uci get darkcore.main.device_token` / `.config_url` — заполнены,
  `.activation_code` снова пуст (одноразовый, чистится сразу);
- `curl -sS -H "Authorization: Bearer $(uci get darkcore.main.device_token)" "$(uci get darkcore.main.config_url)"` с
  платы → `200` + валидный JSON;
- `/etc/sing-box/conf.d/90-proxy.json` обновился, в логе — успешная
  `sing-box check` и рестарт (не «keeping old config running»);
- fail-open (`sing-box-watchdog`, не встроенный в sing-box механизм):
  заблокировать все VLESS-сервера → через ~45-60с `nft list ruleset`
  пуст, внешний IP клиента становится IP роутера, соединение живо; снять
  блок → через ~15-30с `nft -f` возвращает TPROXY-правила, трафик
  вернулся на прокси.

## Связанные задачи в `darkcorewrt`

- LuCI-страница «Special Router» — `darkcorewrt/packages/luci-app-darkcore`
  (ввод кода активации, статус/управление sing-box, брендинг).
- Периодический health-check прокси (`TODO.md` 3b) — сделано полностью:
  валидация конфига перед рестартом + `sing-box-watchdog` (см.
  «fail-open» выше) закрывают то, что для xray оставалось задачей 4c.
- Авто-DNS LAN-клиентам по DHCP (`TODO.md` 3c) — сделано в
  `darkcorewrt/build.sh` (`add_lan_dhcp_dns`), не здесь.

## 2026-09-30: ложная тревога с "роутер не грузится" - виновата SD-карта, не код

После фикса `sing-box.init` `START=00`→`99` (см. ниже) router несколько
раз подряд не отвечал после честной холодной перепрошивки (`nmap`
показывал только порт 53/dnsmasq через hotplug, SSH/LuCI - connection
refused). Подозревали сам `START=99`-фикс, потом ещё один баг
(`add_packages.sh` клонировал `darkcore` в уже существующую директорию
и молча падал без `set -e`, так что старые фиксы не попадали в
пересборку - тоже реальный баг, исправлен, см. `wrt_repo/CLAUDE.md`).
Оба фикса подтверждены байт-в-байт через `debugfs` прямо в собранном
`.img` (`rc.d`-порядок правильный, `S19dropbear`/`S50uhttpd` задолго до
`S99sing-box`) - и всё равно роутер не отвечал.

Решающий тест: собрали ПОЛНОСТЬЮ стоковый образ (`./build.sh
friendlywrt` + `./build.sh sd-img` напрямую в `WRT_DIR`, в обход
`add_packages.sh` - ни одного darkcore-пакета) и прошили той же SD-
картой - тот же симптом (порт 53 only). Значит, дело было не в коде
вообще. Прошили ту же прошивку на **другую** SD-карту - заработало.
Прошили уже реальную сборку с darkcore-пакетами (образ от 2026-09-29
23:30, с обоими фиксами) на ту же новую карту - тоже заработало.

**Вывод**: карта деградировала/повредилась от множества `dd`-перезаписей
за один день (десяток+ циклов прошивки за сессию). `sync; sleep 3;
sync` после `dd` не спасает от этого - помогает только не убивать одну
карту раз за разом в течение дня, или иметь запасную для быстрой
проверки именно этой гипотезы. Раньше в этой же сессии уже ловили
похожий симптом (расхождения по блокам при сравнении `sda` и `.img`
без `sync`) - это разные проявления одной и той же ненадёжности много-
кратно переписываемой карты, а не баг в прошивке.

**На будущее**: если после фикса, подтверждённого байт-в-байт в
собранном образе, роутер всё равно не грузится - не тратить лишний час
на догадки про boot-порядок/procd/DNS-петли. Первым делом собрать и
прошить **чисто стоковый** образ (без `add_packages.sh`) на ту же
карту; если он тоже не грузится - дело в карте/железе, а не в коде, и
дальше искать нужно там (другая карта, другой картридер).

## 2026-09-30: ручной выбор профиля (не только авто по пингу)

`route.final` = `"GLOBAL AUTO"`, это `urltest`-группа - sing-box
категорически отказывается переключать её вручную через Clash API
(`PUT /proxies/GLOBAL%20AUTO` → `{"message":"Must be a Selector"}`,
проверено эмпирически на живом роутере). Вложенные `COUNTRY <страна>`
outbound'ы в конфиге от бэкенда - это `selector` (ручной выбор
поддерживают), но никуда не подключены в роутинге (`route.final` идёт
прямо на `GLOBAL AUTO`, они просто висят в списке outbound'ов).

**Решение** — `sing-box-sync-manual-selector` (новый
`/usr/libexec/darkcore-singbox-sync-manual-selector`, вызывается из
`sing-box.init` в начале `start_service()`/`restart_service()`, ДО
`check_config`):
- зеркалит список серверов из `outbounds[tag="GLOBAL AUTO"].outbounds`
  (в `90-proxy.json`, живом файле от бэкенда) в свою `selector`-группу
  `MANUAL` с добавленным первым пунктом `"GLOBAL AUTO"` (автомат);
- пишет `confdir/80-manual.json` с `{"route":{"final":"MANUAL"}}`.
  Имя файла НЕ случайное: `00-base.json` (наш) < `80-manual.json`
  (генерируемый) < `90-proxy.json` (бэкенд) по алфавиту, а при мердже
  `sing-box -C` для скаляров побеждает более РАННИЙ файл (см. выше про
  `route.final`-баг) - значит `80-manual.json`'s `final:"MANUAL"`
  побеждает над `90-proxy.json`'s собственным `final:"GLOBAL AUTO"`,
  и переключение реально управляет трафиком, а не просто отображением.
- если `90-proxy.json` ещё нет (пре-активация) или в нём нет `GLOBAL
  AUTO` - скрипт удаляет стухший `80-manual.json` и ничего не делает,
  `route.final` остаётся неустановленным (как было до этой фичи,
  дефолт - первый outbound, `direct`).

**Персистентность выбора** — `experimental.cache_file` в `00-base.json`
(`{"enabled":true,"path":"/etc/sing-box/cache.db"}`). Без него выбор
`MANUAL`-селектора сбрасывался бы на дефолт (`GLOBAL AUTO`) при КАЖДОМ
рестарте sing-box, включая рестарт, который `dcvpnupd` триггерит сам
при каждом обновлении `90-proxy.json` с бэкенда. Поле
`store_selected`, которое я сначала добавил по памяти - не существует
в схеме этой версии sing-box (`unknown field "store_selected"`,
поймано на реальной валидации), персистентность выбора работает
просто от `cache_file.enabled`, отдельного флага не требуется.

**LuCI-скрипты** (Clash API 127.0.0.1-only, дергаются через `fs.exec`
из `setup.js`, см. `luci-app-darkcore/CLAUDE.md`):
- `darkcore-singbox-profile-list` — один `GET /proxies`: `MANUAL`'s
  `now` (текущий выбор), `MANUAL`'s `all` (полный список серверов →
  поле `servers`) и `history[0].delay` каждого сервера (последний
  результат периодического urltest самой `GLOBAL AUTO` - группа пишет
  в общий `HistoryStorage` Clash-сервера, см. `protocol/group/urltest.go`
  в 1.11.15) → поле `delays`;
- **2026-10-07: живой `/group/GLOBAL%20AUTO/delay` убран - на железе
  он не работает.** Первая версия брала и пинг, и сам список серверов
  из него. Проверено на R2S: запрос шёл 10147мс и упирался в
  `curl -m 10` → `delays:{}` → в dropdown только «Авто». Причина в
  sing-box: urltest-группа гоняет пробу с concurrency 10, на ~90
  серверах это заметно дольше любого разумного rpc-таймаута LuCI; плюс
  если в этот момент идёт её собственная фоновая проверка
  (`checking.Swap(true)`), отдаётся сразу пустой `{}`. К тому же
  failed-серверы в ответ не попадают вообще. Поэтому список серверов
  теперь берётся только из `MANUAL.all`, а пинг - из истории (сервер
  без успешной последней проверки просто без пинга, «нет ответа»);
- `darkcore-singbox-select-profile <tag>` — `PUT /proxies/MANUAL`.
- Заменили ими старый `darkcore-singbox-profile-status` (был
  read-only, показывал только текущий профиль+пинг) - `profile-list`
  строгий суперсет его функциональности.

**Проверено вживую end-to-end** (2026-09-30): сгенерированный
`80-manual.json` проходит `sing-box check`, `PUT` реально меняет
исходящий IP (подтверждено `curl ifconfig.me` через SOCKS до и после),
выбор пережил `service sing-box restart` благодаря `cache_file`.
