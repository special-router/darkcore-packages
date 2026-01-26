include $(TOPDIR)/rules.mk

PKG_NAME:=darkcore-xray
PKG_VERSION:=1.0.3
PKG_RELEASE:=1

XRAY_VERSION:=25.1.30

PKG_SOURCE:=$(PKG_NAME)-$(PKG_VERSION).tar.gz
PKG_SOURCE_URL:=https://codeload.github.com/XTLS/Xray-core/tar.gz/v$(XRAY_VERSION)?
PKG_HASH:=983ee395f085ed1b7fbe0152cb56a5b605a6f70a5645d427c7186c476f14894e

PKG_MAINTAINER:=Tianling Shen <cnsztl@immortalwrt.org>
PKG_LICENSE:=MPL-2.0
PKG_LICENSE_FILES:=LICENSE

PKG_BUILD_DIR:=$(BUILD_DIR)/Xray-core-$(XRAY_VERSION)
PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1
PKG_BUILD_FLAGS:=no-mips16

GO_PKG:=github.com/xtls/xray-core
GO_PKG_BUILD_PKG:=github.com/xtls/xray-core/main
GO_PKG_LDFLAGS_X:= \
	$(GO_PKG)/core.build=OpenWrt \
	$(GO_PKG)/core.version=$(XRAY_VERSION)

include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

define Package/darkcore-xray
  TITLE:=A platform for building proxies to bypass network restrictions
  SECTION:=net
  CATEGORY:=DarkCore
  URL:=https://xtls.github.io
	DEPENDS:=$(GO_ARCH_DEPENDS) +ca-bundle
endef

define Package/darkcore-xray/description
    Xray, Penetrates Everything. It helps you to build your own computer network.
    It secures your network connections and thus protects your privacy.
endef

define Package/darkcore-xray/conffiles
/etc/xray/
/etc/config/xray
endef

define Package/darkcore-xray/install
	$(call GoPackage/Package/Install/Bin,$(PKG_INSTALL_DIR))
	$(INSTALL_DIR) $(1)/usr/bin/
	$(INSTALL_BIN) $(PKG_INSTALL_DIR)/usr/bin/main $(1)/usr/bin/xray

	$(INSTALL_DIR) $(1)/etc/config/
	$(INSTALL_CONF) $(CURDIR)/files/xray.conf $(1)/etc/config/xray
	$(INSTALL_DIR) $(1)/etc/init.d/
	$(INSTALL_BIN) $(CURDIR)/files/xray.init $(1)/etc/init.d/xray

	$(INSTALL_DIR) $(1)/etc/init.d/
	$(INSTALL_BIN) $(CURDIR)/files/xrayrestart.init $(1)/etc/init.d/xrayrestart
	$(INSTALL_DIR) $(1)/root/
	$(INSTALL_BIN) $(CURDIR)/files/xrayrestart.sh $(1)/root/xrayrestart.sh

	$(INSTALL_DIR) $(1)/etc/xray/
	$(INSTALL_DATA) $(CURDIR)/files/configs/dns.json $(1)/etc/xray/
	$(INSTALL_DIR) $(1)/etc/xray/
	$(INSTALL_DATA) $(CURDIR)/files/configs/inbounds.json $(1)/etc/xray/
	$(INSTALL_DIR) $(1)/etc/xray/
	$(INSTALL_DATA) $(CURDIR)/files/configs/routing.json $(1)/etc/xray/
	$(INSTALL_DIR) $(1)/etc/xray/
	$(INSTALL_DATA) $(CURDIR)/files/configs/tail_outbounds.json $(1)/etc/xray/

	$(INSTALL_DIR) $(1)/usr/share/xray/
	$(INSTALL_DATA) $(CURDIR)/files/nftables.rulesv46 $(1)/usr/share/xray/
endef

$(eval $(call BuildPackage,darkcore-xray))
