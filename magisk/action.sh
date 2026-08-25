#!/system/bin/sh
# KernelSU Action script

echo "=========================================="
echo "    Netclient Status & Diagnostics"
echo "=========================================="

if [ -f /data/adb/netclient/netclient.pid ]; then
    PID=$(cat /data/adb/netclient/netclient.pid 2>/dev/null)
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
        echo "[✓] Daemon is RUNNING (PID: $PID)"
    else
        echo "[!] Daemon is NOT running (stale PID: $PID)"
    fi
else
    echo "[!] Daemon is NOT running (no PID file)"
fi

echo ""
echo "--- Netclient Configuration ---"
if command -v netclient >/dev/null 2>&1; then
    netclient list 2>&1 || true
else
    echo "netclient command not found in PATH"
fi

echo ""
echo "--- Interface Status ---"
if command -v wg >/dev/null 2>&1; then
    wg show 2>&1 || true
else
    ip addr show dev netmaker 2>&1 || echo "Interface netmaker not up"
fi

echo ""
echo "--- Policy Rules (pref 90 & 99) ---"
ip rule show 2>/dev/null | grep -E "(90:|99:|1000)" || echo "No active Netclient PBR rules found"
ip -6 rule show 2>/dev/null | grep -E "(90:|99:|1000)" || echo "No active IPv6 Netclient PBR rules found"

echo "=========================================="
