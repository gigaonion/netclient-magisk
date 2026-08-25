#!/system/bin/sh
SKIPUNZIP=0

ui_print "**********************************************"
ui_print "*    Netclient (Netmaker) for Android        *"
ui_print "**********************************************"

# Verify architecture
if [ "$ARCH" != "arm64" ]; then
    abort "! Unsupported architecture: $ARCH (Only arm64 is supported)"
fi

ui_print "- Creating persistence directory /data/adb/netclient..."
mkdir -p /data/adb/netclient
chmod 0755 /data/adb/netclient

# Set executable permissions
ui_print "- Setting binary permissions..."
set_perm "$MODPATH/service.sh" 0 0 0755
[ -f "$MODPATH/action.sh" ] && set_perm "$MODPATH/action.sh" 0 0 0755
[ -f "$MODPATH/system/bin/netclient" ] && set_perm "$MODPATH/system/bin/netclient" 0 0 0755
[ -f "$MODPATH/system/bin/wg" ] && set_perm "$MODPATH/system/bin/wg" 0 0 0755

ui_print "- Checking WireGuard kernel support..."
if grep -qw "wireguard" /proc/modules 2>/dev/null || [ -d "/sys/module/wireguard" ]; then
    ui_print "  [✓] WireGuard kernel module detected"
else
    # Try loading module if available
    modprobe wireguard 2>/dev/null || true
    if [ -d "/sys/module/wireguard" ]; then
        ui_print "  [✓] WireGuard kernel module loaded successfully"
    else
        ui_print "  [!] Note: WireGuard kernel module not yet loaded (will be checked at boot)"
    fi
fi

ui_print ""
ui_print "- Installation completed successfully!"
ui_print "  To register to a network, run from root shell:"
ui_print "  # su"
ui_print "  # netclient register -t <enrollment_token>"
ui_print "**********************************************"
