//go:build linux
// +build linux

package wireguard

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/vishvananda/netlink"
	"golang.org/x/exp/slog"
	"golang.org/x/sys/unix"
)

const (
	BypassPref = 90
	BypassFile = "/data/adb/netclient/bypass.json"
)

// BypassConfig defines the home SSIDs and subnets to bypass WireGuard when connected to home Wi-Fi.
type BypassConfig struct {
	HomeSSIDs     []string `json:"home_ssids"`
	BypassSubnets []string `json:"bypass_subnets"`
}

var bypassMu sync.Mutex

// LoadBypassConfig loads bypass configuration from disk.
func LoadBypassConfig() (*BypassConfig, error) {
	data, err := os.ReadFile(BypassFile)
	if err != nil {
		return nil, err
	}
	var cfg BypassConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveBypassConfig writes bypass configuration to disk.
func SaveBypassConfig(cfg *BypassConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(BypassFile, data, 0644)
}

// GetActiveSSID detects the currently connected Wi-Fi SSID on Android.
func GetActiveSSID() string {
	// Method 1: cmd wifi status
	if out, err := exec.Command("cmd", "wifi", "status").Output(); err == nil {
		s := string(out)
		if idx := strings.Index(s, "connected to \""); idx != -1 {
			rest := s[idx+len("connected to \""):]
			if end := strings.Index(rest, "\""); end != -1 {
				return rest[:end]
			}
		}
		if idx := strings.Index(s, "SSID: \""); idx != -1 {
			rest := s[idx+len("SSID: \""):]
			if end := strings.Index(rest, "\""); end != -1 {
				return rest[:end]
			}
		}
	}

	// Method 2: dumpsys wifi
	if out, err := exec.Command("dumpsys", "wifi").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "mWifiInfo") && strings.Contains(line, "SSID: ") {
				idx := strings.Index(line, "SSID: ")
				rest := line[idx+len("SSID: "):]
				rest = strings.Trim(strings.Split(rest, ",")[0], "\" ")
				if rest != "" && rest != "<unknown ssid>" && rest != "0x" {
					return rest
				}
			}
		}
	}

	// Method 3: iw dev wlan0 link
	if out, err := exec.Command("iw", "dev", "wlan0", "link").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "SSID: ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "SSID: "))
			}
		}
	}

	return ""
}

func flushRouteCache() {
	_ = os.WriteFile("/proc/sys/net/ipv4/route/flush", []byte("1"), 0644)
	_ = os.WriteFile("/proc/sys/net/ipv6/route/flush", []byte("1"), 0644)
}

func getWlanGateway() net.IP {
	if wlanLink, err := netlink.LinkByName("wlan0"); err == nil && wlanLink != nil {
		allRoutes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{LinkIndex: wlanLink.Attrs().Index}, netlink.RT_FILTER_OIF)
		if err == nil {
			for _, r := range allRoutes {
				if r.Gw != nil && !r.Gw.IsUnspecified() {
					return r.Gw
				}
			}
		}
		// Check all routes across all routing tables for a default route via wlan0
		routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
		if err == nil {
			for _, r := range routes {
				if r.LinkIndex == wlanLink.Attrs().Index && r.Gw != nil && !r.Gw.IsUnspecified() {
					return r.Gw
				}
			}
		}
	}
	for _, prop := range []string{"net.wlan0.gw", "dhcp.wlan0.gateway", "net.wlan0.gateway", "net.dns1"} {
		if out, err := exec.Command("getprop", prop).Output(); err == nil {
			gwStr := strings.TrimSpace(string(out))
			if gwStr != "" {
				if ip := net.ParseIP(gwStr); ip != nil && ip.To4() != nil {
					return ip
				}
			}
		}
	}
	// Intelligent subnet fallback: default to .1 of the wlan0 subnet
	if wlanLink, err := netlink.LinkByName("wlan0"); err == nil && wlanLink != nil {
		addrs, err := netlink.AddrList(wlanLink, netlink.FAMILY_V4)
		if err == nil {
			for _, addr := range addrs {
				if addr.IP != nil && addr.IP.To4() != nil {
					ip4 := addr.IP.To4()
					return net.IPv4(ip4[0], ip4[1], ip4[2], 1)
				}
			}
		}
	}
	return nil
}

// EvaluateBypassRules evaluates current Wi-Fi SSID and applies or removes pref 90 bypass rules.
func EvaluateBypassRules() {
	bypassMu.Lock()
	defer bypassMu.Unlock()

	cfg, err := LoadBypassConfig()
	if err != nil || cfg == nil || len(cfg.HomeSSIDs) == 0 || len(cfg.BypassSubnets) == 0 {
		FlushBypassRules()
		return
	}

	currentSSID := GetActiveSSID()
	isHome := false
	if currentSSID != "" {
		for _, ssid := range cfg.HomeSSIDs {
			if strings.EqualFold(strings.TrimSpace(ssid), currentSSID) {
				isHome = true
				break
			}
		}
	}

	wlanLink, _ := netlink.LinkByName("wlan0")

	if isHome && wlanLink != nil {
		slog.Info("connected to home Wi-Fi, applying LAN bypass rules", "ssid", currentSSID)
		wlanGw := getWlanGateway()
		for _, subnet := range cfg.BypassSubnets {
			_, ipnet, err := net.ParseCIDR(strings.TrimSpace(subnet))
			if err != nil {
				continue
			}
			family := unix.AF_INET
			if strings.Contains(subnet, ":") {
				family = unix.AF_INET6
			}

			// 1. Add route in table main pointing to wlan0 / wlan0 gateway
			route := &netlink.Route{
				LinkIndex: wlanLink.Attrs().Index,
				Dst:       ipnet,
				Table:     unix.RT_TABLE_MAIN,
				Gw:        wlanGw,
			}
			if wlanGw == nil {
				route.Scope = netlink.SCOPE_LINK
			}
			_ = netlink.RouteReplace(route)

			// 2. Add rule with pref 90 looking up table main
			rule := netlink.NewRule()
			rule.Family = family
			rule.Dst = ipnet
			rule.Table = unix.RT_TABLE_MAIN
			rule.Priority = BypassPref
			_ = netlink.RuleDel(rule)
			if err := netlink.RuleAdd(rule); err != nil && !os.IsExist(err) {
				slog.Warn("failed to add bypass rule", "subnet", subnet, "error", err)
			}
		}
		flushRouteCache()
	} else {
		FlushBypassRules()
	}
}

// FlushBypassRules flushes all pref 90 bypass rules and table main bypass routes.
func FlushBypassRules() {
	rules, err := netlink.RuleList(netlink.FAMILY_ALL)
	if err == nil {
		for _, rule := range rules {
			if rule.Priority == BypassPref {
				_ = netlink.RuleDel(&rule)
			}
		}
	}

	cfg, _ := LoadBypassConfig()
	if cfg != nil {
		wlanLink, _ := netlink.LinkByName("wlan0")
		linkIndex := 0
		if wlanLink != nil {
			linkIndex = wlanLink.Attrs().Index
		}
		for _, subnet := range cfg.BypassSubnets {
			_, ipnet, err := net.ParseCIDR(strings.TrimSpace(subnet))
			if err != nil {
				continue
			}
			route := &netlink.Route{
				LinkIndex: linkIndex,
				Dst:       ipnet,
				Table:     unix.RT_TABLE_MAIN,
			}
			_ = netlink.RouteDel(route)
		}
	}
	flushRouteCache()
}

// StartBypassMonitor starts an event-driven Netlink listener for network state changes (0% idle CPU).
func StartBypassMonitor(ctx context.Context) {
	// Initial evaluation at startup
	EvaluateBypassRules()

	linkCh := make(chan netlink.LinkUpdate)
	linkDone := make(chan struct{})
	if err := netlink.LinkSubscribe(linkCh, linkDone); err != nil {
		slog.Warn("failed to subscribe to netlink link updates", "error", err)
		return
	}
	defer close(linkDone)

	addrCh := make(chan netlink.AddrUpdate)
	addrDone := make(chan struct{})
	if err := netlink.AddrSubscribe(addrCh, addrDone); err != nil {
		slog.Warn("failed to subscribe to netlink addr updates", "error", err)
		return
	}
	defer close(addrDone)

	routeCh := make(chan netlink.RouteUpdate)
	routeDone := make(chan struct{})
	if err := netlink.RouteSubscribe(routeCh, routeDone); err == nil {
		defer close(routeDone)
	}

	slog.Info("started event-driven Wi-Fi bypass monitor")

	for {
		select {
		case <-ctx.Done():
			FlushBypassRules()
			return
		case linkUpdate := <-linkCh:
			if linkUpdate.Link != nil && linkUpdate.Link.Attrs() != nil && strings.HasPrefix(linkUpdate.Link.Attrs().Name, "wlan") {
				EvaluateBypassRules()
			}
		case addrUpdate := <-addrCh:
			if link, err := netlink.LinkByIndex(addrUpdate.LinkIndex); err == nil && link != nil && strings.HasPrefix(link.Attrs().Name, "wlan") {
				EvaluateBypassRules()
			}
		case routeUpdate := <-routeCh:
			if routeUpdate.Route.LinkIndex != 0 {
				if link, err := netlink.LinkByIndex(routeUpdate.Route.LinkIndex); err == nil && link != nil && strings.HasPrefix(link.Attrs().Name, "wlan") {
					EvaluateBypassRules()
				}
			}
		}
	}
}
