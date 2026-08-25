#!/system/bin/sh
# Netclient service startup script for KernelSU / Magisk

MODDIR=${0%/*}

# Wait until Android has fully booted
until [ "$(getprop sys.boot_completed)" = "1" ]; do
    sleep 2
done

# Give network stack a few seconds to settle
for i in {1..30}; do ping -c 1 -W 1 1.0.0.1 >/dev/null 2>&1 && break || sleep 1; done

# Ensure WireGuard kernel module is loaded
modprobe wireguard 2>/dev/null || true

# Prepare persistence directory
mkdir -p /data/adb/netclient
chmod 0755 /data/adb/netclient

# Clean up any leftover DNS DNAT rules at boot
iptables -t nat -D OUTPUT -p udp --dport 53 -m mark ! --mark 0x10067 ! -d 127.0.0.1 -j DNAT --to-destination 127.0.0.1:5300 2>/dev/null || true
iptables -t nat -D OUTPUT -p udp --dport 53 ! -d 127.0.0.1 -j DNAT --to-destination 127.0.0.1:5300 2>/dev/null || true
ip6tables -t nat -D OUTPUT -p udp --dport 53 -m mark ! --mark 0x10067 ! -d ::1 -j DNAT --to-destination [::1]:5300 2>/dev/null || true
ip6tables -t nat -D OUTPUT -p udp --dport 53 ! -d ::1 -j DNAT --to-destination [::1]:5300 2>/dev/null || true

# Check if daemon binary exists
if [ -x "$MODDIR/system/bin/netclient" ]; then
    DAEMON_BIN="$MODDIR/system/bin/netclient"
elif [ -x "/system/bin/netclient" ]; then
    DAEMON_BIN="/system/bin/netclient"
elif [ -x "/data/adb/netclient/netclient" ]; then
    DAEMON_BIN="/data/adb/netclient/netclient"
else
    echo "[$(date)] netclient binary not found" >> /data/adb/netclient/netclient.log
    exit 1
fi

# Kill any existing stale daemon instance
if [ -f /data/adb/netclient/netclient.pid ]; then
    OLD_PID=$(cat /data/adb/netclient/netclient.pid 2>/dev/null)
    if [ -n "$OLD_PID" ] && kill -0 "$OLD_PID" 2>/dev/null; then
        kill -9 "$OLD_PID" 2>/dev/null || true
    fi
    rm -f /data/adb/netclient/netclient.pid
fi

# Start netclient daemon in background
echo "[$(date)] Starting netclient daemon..." >> /data/adb/netclient/netclient.log
nohup "$DAEMON_BIN" daemon >> /data/adb/netclient/netclient.log 2>&1 &
