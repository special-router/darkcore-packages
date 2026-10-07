include $(TOPDIR)/rules.mk

PKG_NAME:=darkcore-singbox
PKG_VERSION:=1.11.15
PKG_RELEASE:=7

# ВАЖНО: не поднимать выше 1.11.x без проверки. С 1.12.0 sing-box перешёл
# на форк github.com/metacubex/utls, который через common/badtls тянет
# github.com/sagernet/sing@v0.9.6 + github.com/klauspost/compress@v1.20.1 -
# оба требуют go >= 1.25, а этот путь импорта реально достижим ТОЛЬКО при
# сборке под GOARCH=arm64 (наша цель), не под amd64/host - поэтому проблему
# не видно при обычной проверке "собралось на своей машине". У нас
# вкомпилированный Go host-тулчейна - 1.23.12 (feeds/packages/lang/golang).
# 1.11.15 всё ещё на старом github.com/sagernet/utls (без этой цепочки) и
# уже имеет полноценный Reality-клиент (common/tls/reality_client.go,
# гейтится тем же тегом with_utls).
#
# Шипованные go.mod/go.sum у 1.11.15 сами по себе НЕ tidy (отсутствуют
# github.com/josharian/native, github.com/sagernet/nftables/internal/
# parseexprfunc, github.com/sagernet/sing-shadowsocks2 в графе) - readonly-
# сборка падает с "no required module provides". Обычный `go mod tidy`
# это чинит, но заодно МВС поднимает зависимые версии до несовместимых:
# github.com/sagernet/nftables до v0.3.0-beta.4 (там отсутствует нужный
# internal/parseexprfunc в ожидаемом виде) и github.com/sagernet/sing до
# v0.7.6 (сигнатуры socks.HandleConnectionEx/bufio.CopyWithCounters у
# v0.7.6 не совпадают с тем, что вызывает код sing-box 1.11.15 - "not
# enough arguments in call to ..."). Оба зафиксированы обратно через
# replace на версии, под которые реально писан код 1.11.15:
#   replace github.com/sagernet/nftables v0.3.0-beta.4 => .../nftables v0.2.3
#   replace github.com/sagernet/sing v0.7.6 => github.com/sagernet/sing v0.6.10
# Итоговые go.mod/go.sum лежат в files/go.mod.fixed и files/go.sum.fixed и
# накладываются поверх распакованного тарбола в Hooks/Prepare/Post ниже -
# без этого лечения `go mod tidy`/сеть на сборочной машине не нужны.
#
# Проверено вручную: `go build -tags with_utls -o sing-box ./cmd/sing-box`
# под GOOS=linux GOARCH=arm64 CGO_ENABLED=0 GOFLAGS=-mod=readonly
# GOPROXY=off (то есть полностью офлайн, тем же режимом, каким собирает
# сам OpenWrt) - собирает рабочий ARM64 ELF без единой ошибки.
SINGBOX_VERSION:=1.11.15

PKG_SOURCE:=$(PKG_NAME)-$(PKG_VERSION).tar.gz
PKG_SOURCE_URL:=https://codeload.github.com/SagerNet/sing-box/tar.gz/v$(SINGBOX_VERSION)?
PKG_HASH:=97d58dd873d7cf9b5e4b4aca5516568f3b2e6f5c3dbc93241c82fff5e4a609fd

PKG_MAINTAINER:=DarkCore <lalkaboss33@gmail.com>
PKG_LICENSE:=GPL-3.0-or-later
PKG_LICENSE_FILES:=LICENSE

PKG_BUILD_DIR:=$(BUILD_DIR)/sing-box-$(SINGBOX_VERSION)
PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1
PKG_BUILD_FLAGS:=no-mips16

GO_PKG:=github.com/sagernet/sing-box
GO_PKG_BUILD_PKG:=$(GO_PKG)/cmd/sing-box
GO_PKG_LDFLAGS_X:=$(GO_PKG)/constant.Version=$(SINGBOX_VERSION)
# with_utls обязателен: vless-outbound'ы, которые отдаёт бэкенд, используют
# tls.utls.enabled + tls.reality.enabled - без тега TLS-хендшейк не пройдёт.
#
# НЕ добавлять with_quic "про запас": у sing-box ленивый граф модулей
# (lazy module graph), go.sum считает хэши только для того, что реально
# импортируется при активных тегах. with_quic тянет github.com/sagernet/
# quic-go, github.com/quic-go/qpack и т.д. - их граф не досчитан в
# шипованном go.sum, сборка падает с "no required module provides
# package X" по всему дереву. В текущем конфиге бэкенда нет ни одного
# QUIC/Hysteria-транспорта - добавлять тег, только когда он реально
# понадобится, и сразу проверять сборку.
#
# with_clash_api, наоборот, ПРОВЕРЕН (2026-09-29): собирается чисто на
# уже имеющемся go.sum.fixed, новых модулей не требует (видимо,
# go-chi/chi/v5 и остальное, что ему нужно, уже и так в графе через
# with_utls). Нужен для страницы статуса в LuCI - список профилей с
# пингом (из urltest-истории GLOBAL AUTO) + ручной выбор (см. darkcore-singbox-profile-list,
# darkcore-singbox-select-profile, sing-box-sync-manual-selector в
# files/, experimental.clash_api/cache_file в files/conf.d/00-base.json).
GO_PKG_TAGS:=with_utls with_clash_api

include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

# Накладываем исправленные go.mod/go.sum (см. комментарий у SINGBOX_VERSION
# выше) поверх того, что распаковалось из PKG_SOURCE - до Build/Configure,
# который уже читает $(PKG_BUILD_DIR)/go.mod для golang-build.sh.
define OverridePinnedGoSum
	cp $(CURDIR)/files/go.mod.fixed $(PKG_BUILD_DIR)/go.mod
	cp $(CURDIR)/files/go.sum.fixed $(PKG_BUILD_DIR)/go.sum
endef
Hooks/Prepare/Post += OverridePinnedGoSum

define Package/darkcore-singbox
  TITLE:=The universal proxy platform (Special Router build)
  SECTION:=net
  CATEGORY:=DarkCore
  URL:=https://sing-box.sagernet.org
  DEPENDS:=$(GO_ARCH_DEPENDS) +ca-bundle +kmod-inet-diag +curl
endef

define Package/darkcore-singbox/description
  sing-box, собранный под прокси-движок Special Router (замена
  darkcore-xray). Ставит UCI-схему, procd-init с валидацией конфига перед
  применением, TPROXY-правила nftables и watchdog для автоматического
  фейл-опена на прямой интернет, если все proxy-outbound'ы недоступны.
endef

define Package/darkcore-singbox/conffiles
/etc/sing-box/
/etc/config/sing-box
endef

define Package/darkcore-singbox/install
	$(INSTALL_DIR) $(1)/usr/bin/
	$(INSTALL_BIN) $(GO_PKG_BUILD_BIN_DIR)/sing-box $(1)/usr/bin/sing-box
	$(INSTALL_BIN) $(CURDIR)/files/sing-box-watchdog.sh $(1)/usr/bin/sing-box-watchdog

	$(INSTALL_DIR) $(1)/usr/libexec/
	$(INSTALL_BIN) $(CURDIR)/files/darkcore-singbox-profile-list.sh $(1)/usr/libexec/darkcore-singbox-profile-list
	$(INSTALL_BIN) $(CURDIR)/files/darkcore-singbox-select-profile.sh $(1)/usr/libexec/darkcore-singbox-select-profile
	$(INSTALL_BIN) $(CURDIR)/files/sing-box-sync-manual-selector.sh $(1)/usr/libexec/darkcore-singbox-sync-manual-selector

	$(INSTALL_DIR) $(1)/etc/config/
	$(INSTALL_CONF) $(CURDIR)/files/sing-box.conf $(1)/etc/config/sing-box
	$(INSTALL_DIR) $(1)/etc/init.d/
	$(INSTALL_BIN) $(CURDIR)/files/sing-box.init $(1)/etc/init.d/sing-box
	$(INSTALL_BIN) $(CURDIR)/files/sing-box-watchdog.init $(1)/etc/init.d/sing-box-watchdog

	$(INSTALL_DIR) $(1)/etc/sing-box/conf.d/
	$(INSTALL_DATA) $(CURDIR)/files/conf.d/00-base.json $(1)/etc/sing-box/conf.d/
	$(INSTALL_DATA) $(CURDIR)/files/conf.d/90-proxy.json $(1)/etc/sing-box/conf.d/

	$(INSTALL_DIR) $(1)/usr/share/sing-box/
	$(INSTALL_DATA) $(CURDIR)/files/nftables.rulesv46 $(1)/usr/share/sing-box/
endef

$(eval $(call BuildPackage,darkcore-singbox))
