include $(TOPDIR)/rules.mk

PKG_NAME:=geoupdate
PKG_VERSION:=0.0.5
PKG_RELEASE:=1

PKG_SOURCE_PROTO:=local
PKG_SOURCE_VERSION:=$(PKG_VERSION)

PKG_BUILD_DIR:=$(BUILD_DIR)$(PKG_NAME)
PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1
PKG_BUILD_FLAGS:=no-mips16

GO_PKG:=github.com/special-router/dcvpnupd
GO_PKG_BUILD_PKG:=$(GO_PKG)/main

include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

define Package/geoupdate
	SECTION:=utils
	CATEGORY:=DarkCore
	TITLE:=Vpn connection updater
	DEPENDS:=$(GO_ARCH_DEPENDS)
endef

define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
	$(CP) ./src/* $(PKG_BUILD_DIR)/
endef

define Package/geoupdate/install
	$(call GoPackage/Package/Install/Bin,$(PKG_INSTALL_DIR))
	$(INSTALL_DIR) $(1)/usr/bin/
	$(INSTALL_BIN) $(PKG_INSTALL_DIR)/usr/bin/main $(1)/usr/bin/dcvpnupd
endef

$(eval $(call GoBinPackage,$(PKG_NAME)))
$(eval $(call BuildPackage,$(PKG_NAME)))
