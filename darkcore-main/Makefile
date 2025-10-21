include $(TOPDIR)/rules.mk

PKG_NAME:=darkcore-main
PKG_VERSION:=1.0.0
PKG_RELEASE:=1

include $(INCLUDE_DIR)/package.mk

define Package/darkcore-main
	SECTION:=utils
	CATEGORY:=DarkCore
	TITLE:=Darkcore Main
endef

define Build/Compile
  true
endef

define Package/darkcore-main/install
	$(INSTALL_DIR) $(1)/etc/config/
	$(INSTALL_CONF) $(CURDIR)/files/darkcore.conf $(1)/etc/config/darkcore
endef

$(eval $(call BuildPackage,darkcore-main))