include $(TOPDIR)/rules.mk

PKG_NAME:=darkcore-provision
PKG_VERSION:=1.0.0
PKG_RELEASE:=1

include $(INCLUDE_DIR)/package.mk

define Package/darkcore-provision
	SECTION:=utils
	CATEGORY:=DarkCore
	TITLE:=Darkcore first-boot self-registration
	DEPENDS:=+curl
endef

define Build/Compile
  true
endef

define Package/darkcore-provision/install
	$(INSTALL_DIR) $(1)/usr/bin/
	$(INSTALL_BIN) $(CURDIR)/files/provision.sh $(1)/usr/bin/darkcore-provision
	$(INSTALL_DIR) $(1)/etc/init.d/
	$(INSTALL_BIN) $(CURDIR)/files/provision.init $(1)/etc/init.d/darkcore-provision
endef

$(eval $(call BuildPackage,darkcore-provision))
