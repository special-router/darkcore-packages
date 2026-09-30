include $(TOPDIR)/rules.mk

PKG_NAME:=darkcore-main
PKG_VERSION:=1.2.0
PKG_RELEASE:=2

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

	# TEMP: boot-trace checkpoints for diagnosing the 2026-09-30 cold-boot
	# hang (SSH/LuCI unreachable, only port 53 up). Each just timestamps
	# /root/boot-trace.log + sync, so it survives a power-cut and can be
	# read by pulling the SD card. Remove once root cause is found.
	$(INSTALL_DIR) $(1)/etc/init.d/
	$(INSTALL_BIN) $(CURDIR)/files/checkpoint05.init $(1)/etc/init.d/checkpoint05
	$(INSTALL_BIN) $(CURDIR)/files/checkpoint15.init $(1)/etc/init.d/checkpoint15
	$(INSTALL_BIN) $(CURDIR)/files/checkpoint55.init $(1)/etc/init.d/checkpoint55
	$(INSTALL_BIN) $(CURDIR)/files/checkpoint89.init $(1)/etc/init.d/checkpoint89
endef

define Package/darkcore-main/postinst
#!/bin/sh
[ -n "$${IPKG_INSTROOT}" ] && exit 0
/etc/init.d/checkpoint05 enable
/etc/init.d/checkpoint15 enable
/etc/init.d/checkpoint55 enable
/etc/init.d/checkpoint89 enable
exit 0
endef

$(eval $(call BuildPackage,darkcore-main))